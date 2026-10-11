package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	rt "github.com/aiarsenal/runtime-go"

	"aiarsenal/config"
	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/auth"
	"aiarsenal/infrastructure/crypto"
)

// AgentInstanceService Agent 运行实例：通过 runtime-go Adapter 真起/停容器，
// 元数据落 aa_agent_instance。镜像按 os/agent/version 查 system_image_setting 解析。
type AgentInstanceService struct {
	repo             repository.IAgentInstanceRepository
	imageRepo        repository.IAgentSystemImageSettingRepository
	cmdRepo          repository.IAgentInstanceCommandRepository // 下发 config_push/restart 给容器内 manager
	llmKeySvc        *LLMKeyService                             // 签发实例专属 LLM 虚拟密钥（agent 接 LLM 网关用）
	llmModelRepo     repository.ILLMModelRepository             // aa_llm_model 富化下发模型参数（name/cost/contextWindow/maxTokens）
	adapter          rt.Adapter
	configChangeRepo repository.IAgentInstanceConfigChangeRepository // agent 配置下发即落 dispatch 记录，供配置变更列表 + runtime applied/reverted 对比
	mcpSvc           *AgentMcpService                                // MCP 分组绑定后的 key/配置收敛 + 删实例清理（main.go 装配后回指，防构造循环）
	channelNotice    *ChannelNoticeService                           // 数字员工创建双通道通知（main.go 回指，防构造循环）
	volumeSvc        *AgentVolumeService                             // 持久卷绑定（main.go 装配后回指，防构造循环）：创建实例时按 volume_ids 挂载
}

// SetVolumeService main.go 装配完 AgentVolumeService 后回指（同 SetMcpService 模式）。
func (s *AgentInstanceService) SetVolumeService(v *AgentVolumeService) { s.volumeSvc = v }

// SetMcpService main.go 装配完 AgentMcpService 后回指（AgentMcpService 构造需要本服务，不能互传）。
func (s *AgentInstanceService) SetMcpService(m *AgentMcpService) { s.mcpSvc = m }

// SetChannelNotice main.go 装配后回指：创建成功通知归属人（站内铃铛 + 名下 hermes 数字人转微信）。
func (s *AgentInstanceService) SetChannelNotice(c *ChannelNoticeService) { s.channelNotice = c }

func NewAgentInstanceService(
	r repository.IAgentInstanceRepository,
	ir repository.IAgentSystemImageSettingRepository,
	cr repository.IAgentInstanceCommandRepository,
	lk *LLMKeyService,
	lm repository.ILLMModelRepository,
	a rt.Adapter,
	ccr repository.IAgentInstanceConfigChangeRepository,
) *AgentInstanceService {
	return &AgentInstanceService{repo: r, imageRepo: ir, cmdRepo: cr, llmKeySvc: lk, llmModelRepo: lm, adapter: a, configChangeRepo: ccr}
}

// Create 建库记录 + 调 adapter 真起容器。
// llmBaseURL 非空时，同时为该实例签发一把 LLM 虚拟密钥（记录 tokenHash），
// 并在容器起来后把 agent 运行配置（含密钥）下发进去——agent 据此接 aiarsenal LLM 网关。
// volumeIds 可选：创建时一并挂载的持久卷（aa_agent_persistent_volume.persistent_volume_id）。
// 容器起来前绑定（provision 内 MountsForInstance 校验+落 instance_id），拼进 spec.Volumes 追加挂载。
func (s *AgentInstanceService) Create(ctx context.Context, e *entity.AgentInstance, ports []rt.PortMap, llmBaseURL string, volumeIds []string) (string, error) {
	// 解析镜像：优先用 e.ImageTag，否则查 system_image_setting。
	// 云 provider（tencent 等）未配镜像时留空，由 provider 用其默认镜像（config 里配）。
	image := e.ImageTag
	if image == "" {
		imgs, err := s.imageRepo.List(ctx, e.Agent, e.Version)
		if err != nil {
			return "", err
		}
		if len(imgs) > 0 {
			image = imgs[0].Image
			e.ImageTag = image
		}
	}

	// 无可用模型 → 创建失败：这样的实例配置没法下发（下发即假模型），起了也必然运行期 404。
	// 预检在建记录之前——不落库、不签 key，错误直接冒到创建表单（与「刷新密钥」的拉取失败同口径）。
	if llmBaseURL != "" && agentDeliversConfig(e.Agent) {
		if ok, merr := HasDeployableLLMModel(ctx, s.llmModelRepo); merr != nil {
			return "", fmt.Errorf("检查可用模型失败: %w", merr)
		} else if !ok {
			return "", fmt.Errorf("无可用模型，请先在【LLM 管理】配置模型部署")
		}
	}

	// 先落库（status=creating），拿到 instanceId 作容器名。
	e.Status = 0 // creating
	id, err := s.repo.Add(ctx, e)
	if err != nil {
		// 同一 user 下实例名唯一：把裸 SQL 唯一约束冲突翻译成可读提示。
		if isDuplicateInstanceName(err) {
			return "", fmt.Errorf("实例名称「%s」已存在，请换一个", e.Name)
		}
		return "", err
	}

	// Phase3：签发 instance token（容器内 agent 控制面鉴权），注入环境变量。
	instToken, _ := auth.GenerateForInstance(e.UserId, id)
	_ = s.repo.UpdateInstanceToken(ctx, id, instToken)

	// 同时签发实例专属 LLM 虚拟密钥，记录 tokenHash（id）到实例。
	// 明文仅此一次拿到，透传给 provision 下发进 agent 运行配置；不落库（仅存哈希）。
	// 签发失败不阻断建实例：记日志，容器仍正常起，后续可手动补发配置。
	var llmPlainKey string
	if s.llmKeySvc != nil && llmBaseURL != "" {
		alias := fmt.Sprintf("instance:%s:%s", id, agentAlias(e.Agent))
		tokenHash, plain, kerr := s.llmKeySvc.Create(ctx, &entity.LLMVirtualKey{
			KeyAlias:  alias,
			UserId:    e.UserId,
			CreatedBy: e.CreatedBy,
		})
		if kerr != nil {
			log.Printf("[instance:%s] 签发 LLM key 失败: %v（继续建实例，跳过配置下发）", id, kerr)
		} else {
			_ = s.repo.UpdateLLMKeyHash(ctx, id, tokenHash)
			llmPlainKey = plain
		}
	}

	// 起容器较慢（拉镜像/启动）：丢后台协程，立即返回 instanceId。
	// 状态由协程异步更新（creating→running/error），前端轮询列表看进度。
	// 注意：不能用请求 ctx（请求返回即取消），provision 内部用 background ctx。
	go s.provision(id, e, image, instToken, llmBaseURL, llmPlainKey, ports, volumeIds)

	// 创建受理即双通道通知归属人（容器还在异步启动，通知先行）。
	if s.channelNotice != nil {
		s.channelNotice.Push(ctx, e.UserId, "数字员工已创建",
			fmt.Sprintf("数字员工「%s」（%s）已创建，容器启动中。", e.Name, e.Agent),
			"/agent/instance/list", "info", "agent-create")
	}

	return id, nil
}

// provision 后台真正起容器并回写状态。Create 落库后异步调用，不阻塞 HTTP 响应。
// llmPlainKey 非空时（且该 agent 由 api 下发配置），容器起来后把含密钥的运行配置经 config_push 下发并重启生效。
func (s *AgentInstanceService) provision(id string, e *entity.AgentInstance, image, instToken, llmBaseURL, llmPlainKey string, ports []rt.PortMap, volumeIds []string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[instance:%s] provision panic: %v", id, r)
			_ = s.repo.UpdateStatus(context.Background(), id, 3, "", "", 0) // error
		}
	}()
	ctx := context.Background()

	spec := rt.InstanceSpec{
		Name:     id, // 容器名用 instanceId（localdocker 约定 aiarsenal-<id>）
		Image:    image,
		CPU:      e.CpuCores,
		MemoryMB: e.MemoryGb * 1024,
		DiskGB:   e.DiskGb,
		Ports:    ports,
		// 业务对（agent/version）不作为命名字段下沉到部署层——
		// runtime-go 只认 Image，业务解析（agent+version→image，查 system_image_setting）留在 api。
		// 这里仅以不透明标签透传，供运维侧按 agent/version 检索。
		Labels: map[string]string{"userId": e.UserId, "agent": e.Agent, "version": e.Version},
		// /aa 持久卷：镜像把 agent 安装与配置烤在 /aa（见 image/Dockerfile/*）。
		// 卷 id 复用 instance id（localdocker 约定 volID==instanceID）。首次挂载 docker 把镜像
		// /aa 内容 seed 进空卷；存量实例重启走卷内已 seed 内容（镜像更新不自动触达，需删卷重建）。
		Volumes: []rt.VolumeMount{{VolumeID: id, MountPath: "/aa"}},
	}
	// 追加持久卷挂载（v2）：创建请求带 volume_ids 时，把对应 aa_agent_persistent_volume 行
	// 绑定到本实例并追加挂载。绑定失败不阻断建实例（记日志，卷保持未绑定可手动重试）；
	// 校验+落 instance_id 在 MountsForInstance 内完成（见 agent_volume_service.go）。
	if len(volumeIds) > 0 && s.volumeSvc != nil {
		if mounts, verr := s.volumeSvc.MountsForInstance(ctx, volumeIds, id); verr != nil {
			log.Printf("[instance:%s] 持久卷绑定失败（实例照常创建）: %v", id, verr)
		} else if len(mounts) > 0 {
			spec.Volumes = append(spec.Volumes, mounts...)
		}
	}
	if e.EnvironmentOverridesJson != "" && e.EnvironmentOverridesJson != "{}" {
		env := map[string]string{}
		if err := json.Unmarshal([]byte(e.EnvironmentOverridesJson), &env); err == nil {
			spec.Env = env
		}
	}
	// 注入控制面 env：agent 据此回连 api + 鉴权。BASE_URL 走公共可达地址。
	if spec.Env == nil {
		spec.Env = map[string]string{}
	}
	spec.Env["AIARSENAL_BASE_URL"] = config.Get().AgentBaseURL()
	spec.Env["AIARSENAL_INSTANCE_TOKEN"] = instToken
	spec.Env["AIARSENAL_INSTANCE_ID"] = id

	log.Printf("[instance:%s] provision start: image=%s agent=%s", id, image, e.Agent)
	// 自动绑定归属人有权限的 MCP 分组（best-effort，只记日志）：必须先于 deliverAgentConfig——
	// 绑定会让 SyncInstance 签实例 key（McpKeyPlain），初始配置的 mcp 段依赖它；管理员代建按归属人权限算。
	if s.mcpSvc != nil {
		s.mcpSvc.AutoBindPermitted(ctx, id, e.UserId)
	}
	// 先下发 agent 配置（config_push 命令落库），再起容器：保证容器一起起来、manager 第一次心跳
	// ClaimNext 就能认领到，不会因「命令还没插库」空轮询一轮。原来这步在 adapter.Create 之后，存在
	// 竞态——manager 被 s6 拉起后第一次心跳可能赶在 deliverAgentConfig 之前 → commands=[]。
	// reload=false：agent 此时未运行，config_push 写盘后由 executor 顺带 Start 拉起（先配置后启动）。
	if llmPlainKey != "" && llmBaseURL != "" && agentDeliversConfig(e.Agent) {
		if derr := s.deliverAgentConfig(ctx, e.Agent, id, e.CreatedBy, "dispatch:create", llmBaseURL, llmPlainKey, false); derr != nil {
			// 不阻断建实例（容器照常起）：记日志留痕，待模型配好后「刷新密钥」补发。
			log.Printf("[instance:%s] 跳过配置下发: %v", id, derr)
		}
	}
	if _, err := s.adapter.Create(ctx, spec); err != nil {
		// 起容器失败：标记 error，保留记录便于排查。
		log.Printf("[instance:%s] adapter.Create 失败: %v", id, err)
		_ = s.repo.UpdateStatus(ctx, id, 3, "", "", 0) // error
		return
	}

	// 读回容器状态落库。
	st, err := s.adapter.Status(ctx, id)
	if err == nil {
		hostPort := 0
		desktopHostPort := 0
		if len(st.Ports) > 0 {
			hostPort = st.Ports[0].HostPort
		}
		// 桌面实例：找 3001(noVNC/KasmVNC) 映射的宿主端口。
		// 走 3001 而非 3000(Selkies)：Selkies 浏览器解码 H.264 要 AVX2(无 AVX2 报 "platform not supported")；
		// noVNC 在 canvas 解 Tight/JPEG 无此要求。3001 是自签 HTTPS，由 api DesktopProxy 终结 TLS(InsecureSkipVerify)，iframe 不感知证书。
		for _, p := range st.Ports {
			if p.ContainerPort == 3001 && p.HostPort > 0 {
				desktopHostPort = p.HostPort
			}
		}
		// 桌面实例 HostPort 用 3001 的宿主端口（DesktopProxy 走它）；非桌面用首个端口。
		effectiveHostPort := hostPort
		if desktopHostPort > 0 {
			effectiveHostPort = desktopHostPort
		}
		_ = s.repo.UpdateStatus(ctx, id, int(st.State), st.ContainerID, "aiarsenal-"+id, effectiveHostPort)
		if desktopHostPort > 0 {
			_ = s.repo.UpdateAccessUrl(ctx, id, fmt.Sprintf("https://%s:%d", config.Get().HostPublicIP(), desktopHostPort))
		}
		log.Printf("[instance:%s] provision done: state=%d hostPort=%d", id, st.State, hostPort)
	} else {
		_ = s.repo.UpdateStatus(ctx, id, 1, "", "aiarsenal-"+id, 0) // running
		log.Printf("[instance:%s] provision done (status 读取失败，兜底 running)", id)
	}
}

// isDuplicateInstanceName 判断是否命中 (user_id, name) 唯一约束。
// gorm 透传 pg 的 unique violation，按错误串里的约束名 / SQLSTATE 23505 识别。
func isDuplicateInstanceName(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "idx_aa_agent_instance_user_name") ||
		(strings.Contains(msg, "duplicate key") && strings.Contains(msg, "23505"))
}

func (s *AgentInstanceService) List(ctx context.Context, userId, name string, status int, pageNum, pageSize int) ([]*entity.AgentInstance, int64, error) {
	return s.repo.List(ctx, userId, name, status, pageNum, pageSize)
}

func (s *AgentInstanceService) ListAll(ctx context.Context, name string, status int, pageNum, pageSize int) ([]*entity.AgentInstance, int64, error) {
	return s.repo.ListAll(ctx, name, status, pageNum, pageSize)
}

func (s *AgentInstanceService) Get(ctx context.Context, id string) (*entity.AgentInstance, error) {
	return s.repo.Get(ctx, id)
}

// Start 启动已停止的容器。
// 用 UpdateStateOnly：docker start 保持同一容器 id 与端口映射，只改状态，不清 host_port。
func (s *AgentInstanceService) Start(ctx context.Context, id string) error {
	if err := s.adapter.Start(ctx, id); err != nil {
		return err
	}
	return s.repo.UpdateStateOnly(ctx, id, 1)
}

// Stop 停止容器。
func (s *AgentInstanceService) Stop(ctx context.Context, id string) error {
	if err := s.adapter.Stop(ctx, id); err != nil {
		return err
	}
	return s.repo.UpdateStateOnly(ctx, id, 2)
}

// Restart 重启容器。
func (s *AgentInstanceService) Restart(ctx context.Context, id string) error {
	if err := s.adapter.Restart(ctx, id); err != nil {
		return err
	}
	return s.repo.UpdateStateOnly(ctx, id, 1)
}

// Remove 软删库记录 + 清理实例专属 LLM/MCP 密钥（防泄漏）；容器/卷清理由后台异步完成。
// docker volume rm 在 colima 上删 hermes 的 /aa 卷实测 30s+（海量小文件），同步等它会把
// 接口拖过前端 10s 超时——页面报错但删除实际还在跑。先落软删立刻返回，后台再清容器+卷。
func (s *AgentInstanceService) Remove(ctx context.Context, ids []string) error {
	// 先取出各实例记录的 LLM key，软删库后即查不到。
	var hashes []string
	if s.llmKeySvc != nil {
		for _, id := range ids {
			if ins, err := s.repo.Get(ctx, id); err == nil && ins != nil && ins.LLMKeyHash != "" {
				hashes = append(hashes, ins.LLMKeyHash)
			}
		}
	}
	// 同理：MCP key + 直绑分组行也要在软删前清理。
	if s.mcpSvc != nil {
		s.mcpSvc.OnInstancesRemoved(ctx, ids)
	}
	if len(hashes) > 0 {
		_ = s.llmKeySvc.Delete(ctx, hashes)
	}
	if err := s.repo.Delete(ctx, ids); err != nil {
		return err
	}
	// 容器/卷清理放后台：请求 ctx 随响应取消，须换独立 ctx。失败只记日志——
	// 容器/卷都以 instanceId 命名，重删幂等，且原实现本来就是 `_ =` 吞错不回滚软删。
	go func() {
		bctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		for _, id := range ids {
			if err := s.adapter.Remove(bctx, id); err != nil { // 容器不存在也幂等
				log.Printf("[instance-remove] 删容器 %s 失败: %v", id, err)
			}
			if err := s.adapter.RemoveVolume(bctx, id); err != nil { // 清 /aa 持久卷（volID==instanceID），防泄漏
				log.Printf("[instance-remove] 删卷 %s 失败: %v", id, err)
			}
		}
	}()
	return nil
}

// Status 实时容器状态（直读 adapter，不落库）。
// 容器暂不可查（异步创建中 / 短暂窗口）时回退用 DB 的 status，不再硬报 not found。
func (s *AgentInstanceService) Status(ctx context.Context, id string) (*rt.Status, error) {
	st, err := s.adapter.Status(ctx, id)
	if err != nil {
		var nf *rt.ErrNotFound
		if errors.As(err, &nf) {
			if ins, e := s.repo.Get(ctx, id); e == nil && ins != nil {
				return &rt.Status{InstanceID: id, State: rt.State(ins.Status)}, nil
			}
		}
		return nil, err
	}
	st.InstanceID = id
	return st, nil
}

// Exec 在容器内执行命令。
func (s *AgentInstanceService) Exec(ctx context.Context, id string, cmd []string) ([]byte, error) {
	return s.adapter.Exec(ctx, id, rt.ExecOpts{Cmd: cmd})
}

// Logs 返回容器日志流。
func (s *AgentInstanceService) Logs(ctx context.Context, id string, follow bool, tail string) (io.ReadCloser, error) {
	return s.adapter.Logs(ctx, id, rt.LogOpts{Follow: follow, Tail: tail})
}

// resolveHostPort 工具：从 instance 拿访问端口（用于 access_url 拼接）。
func (s *AgentInstanceService) resolveHostPort(ins *entity.AgentInstance) int {
	if ins.HostPort > 0 {
		return ins.HostPort
	}
	return 0
}

// 各 agent 的配置构造/下发按 agent 分派（deliverAgentConfig），实现抽到各自的
// agent_<name>_service.go（openclaw→openclaw.json、hermes→config.yaml）；
// 通用模型拉取在 llm_models.go（FetchLLMModels / FirstModelID），与具体 agent 无关。

// RefreshLLMKey 轮换实例专属 LLM 密钥并重发 agent 运行配置（零停机）。
// 用 alias 反查当前 tokenHash（兼容密钥已被外部轮换过的情况）→ Rotate 出新明文 →
// 把实例记录的 tokenHash 刷成新哈希 → 含新明文的运行配置经 config_push+restart 下发。
// 全程在 Rotate 的 5 分钟宽限期内完成：旧 key 仍有效，agent 重启切到新 key，无感切换。
func (s *AgentInstanceService) RefreshLLMKey(ctx context.Context, instanceId, llmBaseURL string) error {
	if s.llmKeySvc == nil {
		return fmt.Errorf("LLM 密钥服务未启用")
	}
	ins, err := s.repo.Get(ctx, instanceId)
	if err != nil || ins == nil {
		return fmt.Errorf("实例不存在")
	}
	// alias 反查当前 tokenHash：即使密钥在别处轮换过（实例记录的 hash 已过期），也能拿到现行 hash。
	alias := fmt.Sprintf("instance:%s:%s", instanceId, agentAlias(ins.Agent))
	cur, err := s.llmKeySvc.GetByAlias(ctx, alias)
	if err != nil || cur == nil || cur.TokenHash == "" {
		return fmt.Errorf("实例未绑定 LLM 密钥")
	}
	issuedBy := ins.UpdatedBy
	if issuedBy == "" {
		issuedBy = ins.CreatedBy
	}
	plain, err := s.llmKeySvc.Rotate(ctx, cur.TokenHash, issuedBy)
	if err != nil {
		return fmt.Errorf("密钥轮换失败: %w", err)
	}
	// 刷新实例记录为新哈希（保持绑定，删实例时仍能清理；也兼容后续再次刷新）。
	_ = s.repo.UpdateLLMKeyHash(ctx, instanceId, crypto.HashToken(plain))
	// 由 api 下发配置的 agent 实例：重发配置（config_push+restart）；其它 agent 仅轮换密钥。
	// 下发失败（如网关无可用模型）→ error 冒到 UI：比下发一个含假模型的配置、运行期才 404 好排查。
	// 注意此时密钥已轮换（旧 key 5 分钟宽限后失效）——修好模型配置后须再次「刷新密钥」。
	if llmBaseURL != "" && agentDeliversConfig(ins.Agent) {
		if derr := s.deliverAgentConfig(ctx, ins.Agent, instanceId, issuedBy, "dispatch:key-rotate", llmBaseURL, plain, true); derr != nil {
			return derr
		}
	}
	log.Printf("[instance:%s] 已刷新 LLM 密钥并重发 %s 配置", instanceId, agentAlias(ins.Agent))
	return nil
}

// GetBoundInstanceByLLMKeyHash 按 tokenHash 反查绑定的实例（封禁/解封通知用：取 owner + 实例名）。
func (s *AgentInstanceService) GetBoundInstanceByLLMKeyHash(ctx context.Context, tokenHash string) (*entity.AgentInstance, error) {
	return s.repo.GetByLLMKeyHash(ctx, tokenHash)
}

// RefreshByLLMKeyHashIfBound 虚拟 key 编辑/轮换后按 tokenHash 反查绑定的 agent 实例，命中则重推其配置。
// best-effort：反查不到（非 instance key，如团队/个人 key）、不下发配置的实例、或重推失败都不影响 key 操作本身，仅记日志。
// （重推走 RefreshLLMKey：轮换取明文 + 重拉 /v1/models + config_push+restart，5 分钟宽限内无感切换。）
func (s *AgentInstanceService) RefreshByLLMKeyHashIfBound(ctx context.Context, tokenHash, llmBaseURL, issuedBy string) {
	if tokenHash == "" || llmBaseURL == "" {
		return
	}
	inst, err := s.repo.GetByLLMKeyHash(ctx, tokenHash)
	if err != nil || inst == nil {
		return // 该 tokenHash 未绑定实例（非 instance key），无需重推
	}
	if !agentDeliversConfig(inst.Agent) {
		return // 不下发运行配置的 agent 直接返回
	}
	if err := s.RefreshLLMKey(ctx, inst.InstanceId, llmBaseURL); err != nil {
		log.Printf("[llm-key] 虚拟 key 变更后重推实例 %s 失败: %v", inst.InstanceId, err)
	} else {
		log.Printf("[llm-key] 虚拟 key 变更后已重推实例 %s", inst.InstanceId)
	}
}

// NotifyModelUnavailable 模型禁用/删除后，找出 allowed_models 让该模型可见的 key（其 /v1/models 会变），
// 逐个从其绑定实例的运行配置里删掉该模型条目（ak 不变，读历史 config_push 改后重推）。
// best-effort：无绑定实例/不下发配置的实例/无历史配置都不阻断，仅记日志。
func (s *AgentInstanceService) NotifyModelUnavailable(ctx context.Context, group, modelName, issuedBy string) {
	if s.llmKeySvc == nil || group == "" || modelName == "" {
		return
	}
	composite := group + "/" + modelName
	keys, err := s.llmKeySvc.ListAffectedKeys(ctx, group, modelName)
	if err != nil {
		log.Printf("[llm-model] 模型 %s 下线：查受影响 key 失败: %v", composite, err)
		return
	}
	if len(keys) == 0 {
		log.Printf("[llm-model] 模型 %s 下线：无受影响 key", composite)
		return
	}
	notified := 0
	for _, k := range keys {
		inst, err := s.repo.GetByLLMKeyHash(ctx, k.TokenHash)
		if err != nil || inst == nil {
			continue // 该 key 未绑定实例（如团队/个人 key）
		}
		if !agentDeliversConfig(inst.Agent) {
			continue // 不下发运行配置的 agent 直接返回
		}
		switch agentAlias(inst.Agent) {
		case "hermes":
			// hermes 单 default：重新选 default 重推（/v1/models 已不含下线模型）。
			s.redeliverHermesConfigFromStored(ctx, inst.InstanceId, issuedBy, "dispatch:model-offline")
		default:
			s.RemoveModelFromInstanceConfig(ctx, inst.InstanceId, composite, issuedBy)
		}
		notified++
	}
	log.Printf("[llm-model] 模型 %s 下线：已通知 %d/%d 个受影响实例移除该模型", composite, notified, len(keys))
}

// NotifyModelConfigRefresh 模型参数编辑（max_input/output_tokens 等影响下发配置的字段）后，
// 找出 allowed_models 含该模型的 key，逐个重新下发完整运行配置（ak 不变，读历史 config_push 取 ak，
// 用最新 DB 重富化 maxTokens/contextWindow）。best-effort：无绑定实例/不下发配置的实例/无历史配置都不阻断。
func (s *AgentInstanceService) NotifyModelConfigRefresh(ctx context.Context, group, modelName, issuedBy string) {
	if s.llmKeySvc == nil || group == "" || modelName == "" {
		return
	}
	keys, err := s.llmKeySvc.ListAffectedKeys(ctx, group, modelName)
	if err != nil {
		log.Printf("[llm-model] 模型 %s/%s 编辑：查受影响 key 失败: %v", group, modelName, err)
		return
	}
	if len(keys) == 0 {
		log.Printf("[llm-model] 模型 %s/%s 编辑：无受影响 key（实例 key 的 allowed_models 不含它）", group, modelName)
		return
	}
	notified := 0
	for _, k := range keys {
		inst, err := s.repo.GetByLLMKeyHash(ctx, k.TokenHash)
		if err != nil || inst == nil {
			continue
		}
		if !agentDeliversConfig(inst.Agent) {
			continue
		}
		switch agentAlias(inst.Agent) {
		case "hermes":
			s.redeliverHermesConfigFromStored(ctx, inst.InstanceId, issuedBy, "dispatch:model-edit")
		default:
			s.RedeliverInstanceOpenclawConfig(ctx, inst.InstanceId, issuedBy)
		}
		notified++
	}
	log.Printf("[llm-model] 模型 %s/%s 编辑：已重推 %d/%d 个受影响实例", group, modelName, notified, len(keys))
}
