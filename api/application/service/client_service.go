package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/auth"
	"aiarsenal/infrastructure/db"
)

// ClientService 桌面客户端支持（数据存独立 client 库）：
//   - device-code 登录中转（bind / exchange）：避免 JWT 直接出现在 aiarsenal:// 回调 URL
//   - 取可安装 agent 列表 + 各 agent 的版本（aa_client_agent / aa_client_agent_version）
//   - 上报运行情况（aa_client_agent_status）
type ClientService struct {
	rdb              redis.UniversalClient
	agentRepo        repository.IClientAgentRepository
	versionRepo      repository.IClientAgentVersionRepository
	statusRepo       repository.IClientAgentStatusRepository
	deviceRepo       repository.IClientDeviceRepository
	userRepo         repository.IUserRepository
	packageRepo      repository.IClientPackageRepository
	commandRepo      repository.IClientCommandRepository
	logRepo          repository.IClientLogRepository
	configChangeRepo repository.IClientConfigChangeRepository
	appRepo          repository.IClientAppRepository
	appPermRepo      repository.IClientAppPermRepository
	skillRepo        repository.ISkillRepository
	skillSecRepo     repository.ISkillSecurityRepository // 技能安装门禁：版本最新审计须为「安全」
	llmKeySvc        *LLMKeyService
	llmModelRepo     repository.ILLMModelRepository
	packageDir       string
	mcpSvc           *ClientMcpService // MCP 分组绑定后的配置注入 + 删设备清理（main.go 装配后回指，防构造循环）
}

// SetMcpService main.go 装配完 ClientMcpService 后回指（其构造需要本服务，不能互传）。
func (s *ClientService) SetMcpService(m *ClientMcpService) { s.mcpSvc = m }

func NewClientService(
	agentRepo repository.IClientAgentRepository,
	versionRepo repository.IClientAgentVersionRepository,
	statusRepo repository.IClientAgentStatusRepository,
	deviceRepo repository.IClientDeviceRepository,
	userRepo repository.IUserRepository,
	packageRepo repository.IClientPackageRepository,
	commandRepo repository.IClientCommandRepository,
	logRepo repository.IClientLogRepository,
	configChangeRepo repository.IClientConfigChangeRepository,
	appRepo repository.IClientAppRepository,
	appPermRepo repository.IClientAppPermRepository,
	skillRepo repository.ISkillRepository,
	skillSecRepo repository.ISkillSecurityRepository,
	llmKeySvc *LLMKeyService,
	llmModelRepo repository.ILLMModelRepository,
	packageDir string,
) *ClientService {
	return &ClientService{rdb: db.Redis(), agentRepo: agentRepo, versionRepo: versionRepo, statusRepo: statusRepo, deviceRepo: deviceRepo, userRepo: userRepo, packageRepo: packageRepo, commandRepo: commandRepo, logRepo: logRepo, configChangeRepo: configChangeRepo, appRepo: appRepo, appPermRepo: appPermRepo, skillRepo: skillRepo, skillSecRepo: skillSecRepo, llmKeySvc: llmKeySvc, llmModelRepo: llmModelRepo, packageDir: packageDir}
}

const (
	clientDcKeyPrefix     = "client:dc:"
	clientDcTTL           = 180 * time.Second
	clientDcUsedPrefix    = "client:dc:used:" // exchange 消费后留的「已用」标记，供浏览器刷新时识别
	clientDcUsedTTL       = 30 * time.Minute
	clientDevStatusPrefix = "client:dev:status:"
	clientDevStatusTTL    = 60 * time.Second
	// openclawDesktopNodeName client 桌面端向网关注册的 node displayName
	// （client/mac/src-tauri/src/platform/desktop_node/protocol.rs 注册）。下发的 openclaw.json
	// 用它把 tools.exec 路由进 client（host=node），敏感本地操作不再落网关裸 node 二进制。
	openclawDesktopNodeName = "AIArsenal Client"
)

// DeviceInfo App 上报的设备信息（exchange 时携带，落 aa_client_device）。
type DeviceInfo struct {
	DeviceId   string   `json:"deviceId"`
	Name       string   `json:"name"`
	Platform   string   `json:"platform"`
	Os         string   `json:"os"`
	Arch       string   `json:"arch"`
	AppVersion string   `json:"appVersion"`
	MacAddress string   `json:"macAddress"`
	MacExtra   []string `json:"macExtra"`
	Ip         string   `json:"ip"`
}

// ClientAgentVersionVO 单个版本规格（App 与管理后台共用；jsonb 字段解析为对象）。
type ClientAgentVersionVO struct {
	VersionId      string             `json:"versionId"`
	AgentId        string             `json:"agentId"`
	Version        string             `json:"version"`
	IsDefault      bool               `json:"isDefault"`
	Enabled        bool               `json:"enabled"`
	DownloadUrl    string             `json:"downloadUrl"`
	ContainerImage string             `json:"containerImage"`
	Channel        string             `json:"channel"`
	ConfigTemplate interface{}        `json:"configTemplate"`
	EnvVars        interface{}        `json:"envVars"`
	Extra          interface{}        `json:"extra"`
	Packages       []ClientPackageRef `json:"packages"` // 已缓存的安装包（按平台）
}

// ClientPackageRef 版本下已缓存安装包的引用（App 据此构造下载 URL：{baseUrl}/api/v1/system/client/packages/{agent}/{version}/{platform}）。
type ClientPackageRef struct {
	Platform string `json:"platform"`
	Filename string `json:"filename"`
	FileSize int64  `json:"fileSize"`
	Sha256   string `json:"sha256"`
}

// ClientAgentVO agent + 其版本列表（App 首页 / 管理后台列表共用）。
type ClientAgentVO struct {
	AgentId  string                 `json:"agentId"`
	Key      string                 `json:"key"` // agent key（= agent 列）
	Name     string                 `json:"name"`
	Intro    string                 `json:"intro"`
	Icon     string                 `json:"icon"`
	Logo     string                 `json:"logo"`
	Enabled  bool                   `json:"enabled"`
	Weigh    int                    `json:"weigh"`
	Versions []ClientAgentVersionVO `json:"versions"`
}

// dcPayload device-code 绑定时存入 redis 的载荷
type dcPayload struct {
	UserId     string `json:"userId"`
	UserName   string `json:"userName"`
	IsAdmin    int    `json:"isAdmin"`
	Name       string `json:"name"`       // 真实姓名（aa_user.name）
	EmployeeNo string `json:"employeeNo"` // 工号（aa_user.employee_no）
}

// Bind 浏览器登录成功后调用：把 deviceCode 绑定到当前登录用户。
// 同时回查 aa_user 取真实姓名 + 工号，供 App 右上角展示（非昵称）。
// 返回 expired=true 表示该 deviceCode 已被 App 端 exchange 消费（一次性），不可再绑定。
func (s *ClientService) Bind(ctx context.Context, userId, userName string, isAdmin int, deviceCode string) (expired bool, err error) {
	if s.rdb == nil {
		return false, errors.New("缓存不可用")
	}
	// 一次性：已被 App 换过 token 的 dc 直接判失效，不重置 key
	if used, e := s.rdb.Exists(ctx, clientDcUsedPrefix+deviceCode).Result(); e == nil && used > 0 {
		return true, nil
	}
	p := dcPayload{UserId: userId, UserName: userName, IsAdmin: isAdmin}
	// 回查 aa_user 取 name + employee_no（JWT claims 只有 userId/userName/isAdmin）
	if s.userRepo != nil {
		if u, err := s.userRepo.GetUserById(ctx, userId); err == nil && u != nil {
			p.Name = u.Name
			p.EmployeeNo = u.EmployeeNo
			if u.UserName != "" {
				p.UserName = u.UserName
			}
		}
	}
	b, _ := json.Marshal(p)
	if e := s.rdb.Set(ctx, clientDcKeyPrefix+deviceCode, string(b), clientDcTTL).Err(); e != nil {
		return false, e
	}
	return false, nil
}

// Exchange App 端用 deviceCode 换 JWT；一次性（成功后删除）。
// 第二个返回值 ok=true 表示已绑定并签发；false 表示尚未绑定（pending）。
// deviceInfo 由 App 采集上报，落 aa_client_device；deviceId 绑入 JWT Claims。
// 返回 name + employeeNo（来自 bind 时回查的 aa_user）供 App 展示。
func (s *ClientService) Exchange(ctx context.Context, deviceCode string, info *DeviceInfo, agentIP, llmBaseURL string) (token, userId, userName, name, employeeNo string, isAdmin int, ok bool, err error) {
	if s.rdb == nil {
		err = errors.New("缓存不可用")
		return
	}
	// deviceId 提前取（pending 分支也要用：落 pending 设备记录）
	deviceId := ""
	if info != nil {
		deviceId = info.DeviceId
	}
	key := clientDcKeyPrefix + deviceCode
	v, e := s.rdb.Get(ctx, key).Result()
	if e == redis.Nil {
		// 尚未绑定：先把设备记录落库为 pending（App 打开登录页即落库，
		// admin 能立刻看到「登录中」设备），再返回 pending。已存在则不动。
		if s.deviceRepo != nil && deviceId != "" && info != nil {
			macExtraJSON := "[]"
			if len(info.MacExtra) > 0 {
				if b, mErr := json.Marshal(info.MacExtra); mErr == nil {
					macExtraJSON = string(b)
				}
			}
			_ = s.deviceRepo.InsertPending(ctx, &entity.ClientDevice{
				DeviceId:   deviceId,
				Name:       info.Name,
				Platform:   info.Platform,
				Os:         info.Os,
				Arch:       info.Arch,
				AppVersion: info.AppVersion,
				MacAddress: info.MacAddress,
				MacExtra:   macExtraJSON,
				Ip:         info.Ip,
			})
		}
		return "", "", "", "", "", 0, false, nil
	}
	if e != nil {
		err = e
		return
	}
	var p dcPayload
	if e := json.Unmarshal([]byte(v), &p); e != nil {
		err = fmt.Errorf("deviceCode 载荷损坏: %w", e)
		return
	}
	// 设备记录：若该设备已存在且被封禁(blocked=1)，拒绝签发 token。
	// 快照同时留作首登判定（Upsert 之后 existing 已被覆盖，须用登录前的行）。
	var firstLogin bool // 从未见过 / pending 待批 / 未挂用户 → 本次登录即首登
	if deviceId != "" && s.deviceRepo != nil {
		existing, lookupErr := s.deviceRepo.GetById(ctx, deviceId)
		firstLogin = lookupErr != nil || existing == nil || existing.Status == "pending" || existing.UserId == ""
		if lookupErr == nil && existing.Blocked == 1 {
			err = errors.New("设备已被封禁，请联系管理员")
			return
		}
	}
	// 签发 JWT（带 deviceId，复用 auth.GenerateForDevice）；一次性使用，删 key
	deptId, postIds, employeeNo, _ := s.userRepo.GetDeptAndPosts(ctx, p.UserId)
	t, e := auth.GenerateForDevice(p.UserId, p.UserName, p.IsAdmin, deviceId, deptId, postIds, employeeNo)
	if e != nil {
		err = fmt.Errorf("签发 token 失败: %w", e)
		return
	}
	// 落库设备记录（status=active，保留 blocked）
	if deviceId != "" && s.deviceRepo != nil && info != nil {
		macExtraJSON := "[]"
		if len(info.MacExtra) > 0 {
			if b, mErr := json.Marshal(info.MacExtra); mErr == nil {
				macExtraJSON = string(b)
			}
		}
		_ = s.deviceRepo.Upsert(ctx, &entity.ClientDevice{
			DeviceId:    deviceId,
			UserId:      p.UserId,
			UserName:    p.UserName,
			Name:        info.Name,
			Platform:    info.Platform,
			Os:          info.Os,
			Arch:        info.Arch,
			AppVersion:  info.AppVersion,
			MacAddress:  info.MacAddress,
			MacExtra:    macExtraJSON,
			Ip:          info.Ip,
			Status:      "active",
			LastLoginIp: agentIP,
		})
		_ = s.invalidateDeviceCache(ctx, deviceId)
		// 首登自动绑定归属人有权限的 MCP 分组（同步 best-effort，失败只记日志，绝不影响登录）。
		// 已 active 的存量设备重登不重放；其既有绑定不动。
		if firstLogin && s.mcpSvc != nil {
			s.mcpSvc.AutoBindPermitted(ctx, deviceId, p.UserId, llmBaseURL)
		}
	}
	_ = s.rdb.Del(ctx, key).Err()
	// 留「已用」标记：浏览器刷新命中后判 dc 失效，避免重复绑定/误导
	_ = s.rdb.Set(ctx, clientDcUsedPrefix+deviceCode, "1", clientDcUsedTTL).Err()
	return t, p.UserId, p.UserName, p.Name, p.EmployeeNo, p.IsAdmin, true, nil
}

// IsDeviceActive 校验设备登录态是否有效（active 且未封禁）。
// redis 缓存 60s；未命中查 DB 并回填。deviceId 为空直接放行（web token）。
func (s *ClientService) IsDeviceActive(ctx context.Context, deviceId string) (bool, error) {
	if deviceId == "" {
		return true, nil
	}
	if s.rdb != nil {
		if v, e := s.rdb.Get(ctx, clientDevStatusPrefix+deviceId).Result(); e == nil {
			return v == "active", nil
		}
	}
	if s.deviceRepo == nil {
		return true, nil
	}
	dev, err := s.deviceRepo.GetById(ctx, deviceId)
	if err != nil {
		// 设备记录不存在（旧 token / 数据被删）：视为失效
		return false, nil
	}
	active := dev.Status == "active" && dev.Blocked != 1
	statusVal := "active"
	if !active {
		statusVal = "revoked"
	}
	if s.rdb != nil {
		_ = s.rdb.Set(ctx, clientDevStatusPrefix+deviceId, statusVal, clientDevStatusTTL).Err()
	}
	return active, nil
}

// invalidateDeviceCache 删除设备状态缓存（撤销/封禁/解封/exchange 后调用，使下次请求即时生效）。
func (s *ClientService) invalidateDeviceCache(ctx context.Context, deviceId string) error {
	if s.rdb == nil || deviceId == "" {
		return nil
	}
	return s.rdb.Del(ctx, clientDevStatusPrefix+deviceId).Err()
}

// ListAgents 取 App 首页可安装 agent 列表：仅 enabled agent + enabled version。
// 返回 []ClientAgentVO（Versions 已填充，jsonb 解析为对象，camelCase）。
func (s *ClientService) ListAgents(ctx context.Context) ([]ClientAgentVO, error) {
	return s.listAgents(ctx, false)
}

// AdminListAgents 管理后台用：全量 agent + 全量版本（含 disabled）。
func (s *ClientService) AdminListAgents(ctx context.Context) ([]ClientAgentVO, error) {
	return s.listAgents(ctx, true)
}

// listAgents 公共组装逻辑。includeDisabled=true 时返回全部，否则只返回 enabled。
func (s *ClientService) listAgents(ctx context.Context, includeDisabled bool) ([]ClientAgentVO, error) {
	agents, err := s.agentRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	agentIds := make([]string, 0, len(agents))
	for _, a := range agents {
		agentIds = append(agentIds, a.AgentId)
	}
	verMap, err := s.versionRepo.ListAllByAgents(ctx, agentIds)
	if err != nil {
		return nil, err
	}
	// 取全部已缓存安装包，按 (agent, version) 分组，挂到对应版本下供 App 构造下载 URL
	pkgIdx := map[string][]ClientPackageRef{}
	if s.packageRepo != nil {
		if pkgs, perr := s.packageRepo.ListAll(ctx); perr == nil {
			for _, p := range pkgs {
				if p.Status != "ready" {
					continue
				}
				key := p.Agent + "\x00" + p.Version
				pkgIdx[key] = append(pkgIdx[key], ClientPackageRef{
					Platform: p.Platform,
					Filename: p.Filename,
					FileSize: p.FileSize,
					Sha256:   p.Sha256,
				})
			}
		}
	}
	out := make([]ClientAgentVO, 0, len(agents))
	for _, a := range agents {
		if !includeDisabled && a.Enabled != 1 {
			continue
		}
		vo := ClientAgentVO{
			AgentId:  a.AgentId,
			Key:      a.Agent,
			Name:     a.Name,
			Intro:    a.Intro,
			Icon:     a.Icon,
			Logo:     a.Logo,
			Enabled:  a.Enabled == 1,
			Weigh:    a.Weigh,
			Versions: []ClientAgentVersionVO{},
		}
		for _, v := range verMap[a.AgentId] {
			if !includeDisabled && v.Enabled != 1 {
				continue
			}
			vvo := buildAgentVersionVO(v)
			vvo.Packages = pkgIdx[a.Agent+"\x00"+v.Version]
			if vvo.Packages == nil {
				vvo.Packages = []ClientPackageRef{}
			}
			vo.Versions = append(vo.Versions, vvo)
		}
		out = append(out, vo)
	}
	return out, nil
}

// buildAgentVersionVO 把版本实体转为前端可用的 VO（jsonb 解析为对象，camelCase）。
func buildAgentVersionVO(v *entity.ClientAgentVersion) ClientAgentVersionVO {
	return ClientAgentVersionVO{
		VersionId:      v.VersionId,
		AgentId:        v.AgentId,
		Version:        v.Version,
		IsDefault:      v.IsDefault == 1,
		Enabled:        v.Enabled == 1,
		DownloadUrl:    v.DownloadUrl,
		ContainerImage: v.ContainerImage,
		Channel:        v.Channel,
		ConfigTemplate: parseJSON(v.ConfigTemplate),
		EnvVars:        parseJSON(v.EnvVars),
		Extra:          parseJSON(v.Extra),
	}
}

// parseJSON 把 jsonb 字符串解析为对象；失败返回原字符串。
func parseJSON(s string) interface{} {
	if s == "" {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		return v
	}
	return s
}

// AdminListStatus 列出全部用户的客户端运行状态。
// uptime 读时计算：running 且最近上报过（活跃）的行 = now − created_at；
// created_at 是行插入时间（合并刷新 reported_at 不动 created_at），即后端首次见到该 running 状态的时刻。
// 客户端从不真实上报 uptime，旧 uptime_seconds 列已 drop，故此处统一回填。
func (s *ClientService) AdminListStatus(ctx context.Context) ([]*entity.ClientAgentStatus, error) {
	list, err := s.statusRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for _, st := range list {
		if st.Status != "running" || st.CreatedAt == nil || st.ReportedAt == nil {
			continue
		}
		// 仅当前活跃行（最近 2 分钟上报过）才算 uptime：
		// 避免时间线里陈旧 running 行（agent 已停但末行残留 running）虚高，以及历史行误导。
		if now.Sub(time.Time(*st.ReportedAt)) <= 2*time.Minute {
			st.UptimeSeconds = int64(now.Sub(time.Time(*st.CreatedAt)).Seconds())
		}
	}
	return list, nil
}

// AdminListStatusByDevice 某设备全部客户端的状态历史（时间线，多条/客户端）。
// 设备页 App 上下线时间线用：前端按 agent=client 过滤。
func (s *ClientService) AdminListStatusByDevice(ctx context.Context, deviceId string) ([]*entity.ClientAgentStatus, error) {
	return s.statusRepo.ListByDevice(ctx, deviceId)
}

// ===== 管理后台：agent / version CRUD =====

// AdminSaveAgent 保存 agent（插入或更新）。agent key 不做白名单限制，管理员可自由新增类型。
func (s *ClientService) AdminSaveAgent(ctx context.Context, e *entity.ClientAgent) error {
	if e.Agent == "" {
		return fmt.Errorf("agent(agent key) 不能为空")
	}
	if e.Name == "" {
		return fmt.Errorf("name 不能为空")
	}
	return s.agentRepo.Upsert(ctx, e)
}

// AdminDeleteAgent 删除 agent 及其全部版本。
func (s *ClientService) AdminDeleteAgent(ctx context.Context, agentId string) error {
	if agentId == "" {
		return fmt.Errorf("agentId 不能为空")
	}
	if err := s.versionRepo.DeleteByAgent(ctx, agentId); err != nil {
		return err
	}
	return s.agentRepo.Delete(ctx, agentId)
}

// AdminSaveVersion 保存版本。若标记为默认，先清除该 agent 内其它版本的默认标记。
func (s *ClientService) AdminSaveVersion(ctx context.Context, e *entity.ClientAgentVersion) error {
	if e.AgentId == "" {
		return fmt.Errorf("agentId 不能为空")
	}
	if e.Version == "" {
		return fmt.Errorf("version 不能为空")
	}
	if e.IsDefault == 1 {
		// 先落库，再用 versionId 清除其它默认
		if err := s.versionRepo.Upsert(ctx, e); err != nil {
			return err
		}
		return s.versionRepo.ClearDefault(ctx, e.AgentId, e.VersionId)
	}
	return s.versionRepo.Upsert(ctx, e)
}

// AdminDeleteVersion 删除单个版本。
func (s *ClientService) AdminDeleteVersion(ctx context.Context, versionId string) error {
	if versionId == "" {
		return fmt.Errorf("versionId 不能为空")
	}
	return s.versionRepo.Delete(ctx, versionId)
}

// ===== 管理后台：设备管理 =====

// AdminListDevices 列出全部设备（按最近登录倒序）。
func (s *ClientService) AdminListDevices(ctx context.Context) ([]*entity.ClientDevice, error) {
	return s.deviceRepo.ListAll(ctx)
}

// AdminGetDevice 取单个设备详情。
func (s *ClientService) AdminGetDevice(ctx context.Context, deviceId string) (*entity.ClientDevice, error) {
	if deviceId == "" {
		return nil, fmt.Errorf("deviceId 不能为空")
	}
	return s.deviceRepo.GetById(ctx, deviceId)
}

// AdminRevokeDevice 撤销设备登录态（status=revoked），其 token 在下次桌面请求时 401。
func (s *ClientService) AdminRevokeDevice(ctx context.Context, deviceId string) error {
	if deviceId == "" {
		return fmt.Errorf("deviceId 不能为空")
	}
	if err := s.deviceRepo.UpdateStatus(ctx, deviceId, "revoked"); err != nil {
		return err
	}
	return s.invalidateDeviceCache(ctx, deviceId)
}

// AdminSetDeviceBlocked 设置设备封禁状态。blocked=1 禁止再登录；0 解封。
func (s *ClientService) AdminSetDeviceBlocked(ctx context.Context, deviceId string, blocked uint) error {
	if deviceId == "" {
		return fmt.Errorf("deviceId 不能为空")
	}
	if err := s.deviceRepo.UpdateBlocked(ctx, deviceId, blocked); err != nil {
		return err
	}
	return s.invalidateDeviceCache(ctx, deviceId)
}

// AdminDeleteDevice 删除设备记录。
func (s *ClientService) AdminDeleteDevice(ctx context.Context, deviceId string) error {
	if deviceId == "" {
		return fmt.Errorf("deviceId 不能为空")
	}
	// 删行前清设备专属 MCP key 与绑定（best-effort）。
	if s.mcpSvc != nil {
		s.mcpSvc.OnDeviceRemoved(ctx, deviceId)
	}
	if err := s.deviceRepo.Delete(ctx, deviceId); err != nil {
		return err
	}
	return s.invalidateDeviceCache(ctx, deviceId)
}

// ===== 管理后台：安装包缓存 =====

// PackageDir 返回存储根目录。
func (s *ClientService) PackageDir() string { return s.packageDir }

// AdminListPackages 列出全部已缓存安装包。
func (s *ClientService) AdminListPackages(ctx context.Context) ([]*entity.ClientPackage, error) {
	list, err := s.packageRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	// 回填 agent logo（join aa_client_agent.logo），供管理后台列表在 agent 列展示图标。
	if agents, aerr := s.agentRepo.ListAll(ctx); aerr == nil {
		logos := make(map[string]string, len(agents))
		for _, a := range agents {
			logos[a.Agent] = a.Logo
		}
		for _, p := range list {
			p.Logo = logos[p.Agent]
		}
	}
	return list, nil
}

// GetPackageByCVP 取某 agent+version+platform 的包（用户下载用）。
func (s *ClientService) GetPackageByCVP(ctx context.Context, agent, version, platform string) (*entity.ClientPackage, error) {
	return s.packageRepo.GetByCVP(ctx, agent, version, platform)
}

// PackageLocalPath 返回包的绝对路径（供 controller c.File 流式下发）。
func (s *ClientService) PackageLocalPath(p *entity.ClientPackage) string {
	return filepath.Join(s.packageDir, p.LocalPath)
}

// FetchPackage 从 officialUrl 下载安装包到本地并落库（必须从官网拉取，保证来源）。
// 落盘路径：<packageDir>/<agent>/<version>/<platform>/<filename>。
func (s *ClientService) FetchPackage(ctx context.Context, agent, version, platform, officialUrl, fetchedBy string) (*entity.ClientPackage, error) {
	if agent == "" || version == "" || platform == "" || officialUrl == "" {
		return nil, fmt.Errorf("agent/version/platform/officialUrl 不能为空")
	}
	if s.packageDir == "" {
		return nil, fmt.Errorf("packageDir 未配置")
	}
	relDir := filepath.Join(agent, version, platform)
	absDir := filepath.Join(s.packageDir, relDir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建目录失败: %w", err)
	}
	filename := filenameFromURL(officialUrl)
	if filename == "" {
		filename = fmt.Sprintf("%s-%s-%s.pkg", agent, version, platform)
	}
	absPath := filepath.Join(absDir, filename)
	relPath := filepath.Join(relDir, filename)

	// 已存在同 (agent,version,platform) 包：覆盖前删旧文件
	if existing, err := s.packageRepo.GetByCVP(ctx, agent, version, platform); err == nil && existing != nil && existing.LocalPath != "" {
		_ = os.Remove(filepath.Join(s.packageDir, existing.LocalPath))
	}

	// HTTP 拉取（带超时、UA，跟随重定向），流式写盘 + sha256 + 大小
	httpClient := &http.Client{Timeout: 10 * time.Minute}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, officialUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "aiarsenal-client-fetch/1.0")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("官方源返回 HTTP %d", resp.StatusCode)
	}
	// 若 Content-Disposition 给了文件名，优先用
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if fn := parseContentDispositionFilename(cd); fn != "" {
			filename = fn
			absPath = filepath.Join(absDir, filename)
			relPath = filepath.Join(relDir, filename)
		}
	}
	tmpPath := absPath + ".part"
	f, err := os.Create(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	h := sha256.New()
	mw := io.MultiWriter(f, h)
	n, err := io.Copy(mw, resp.Body)
	f.Close()
	if err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("写入失败: %w", err)
	}
	if err := os.Rename(tmpPath, absPath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("落盘失败: %w", err)
	}
	pkg := &entity.ClientPackage{
		Agent:       agent,
		Version:     version,
		Platform:    platform,
		Filename:    filename,
		FileSize:    n,
		Sha256:      hex.EncodeToString(h.Sum(nil)),
		OfficialUrl: officialUrl,
		LocalPath:   relPath,
		Status:      "ready",
		FetchedBy:   fetchedBy,
	}
	if err := s.packageRepo.Upsert(ctx, pkg); err != nil {
		_ = os.Remove(absPath)
		return nil, fmt.Errorf("落库失败: %w", err)
	}
	return pkg, nil
}

// AdminDeletePackage 删除安装包记录 + 本地文件。
func (s *ClientService) AdminDeletePackage(ctx context.Context, packageId string) error {
	if packageId == "" {
		return fmt.Errorf("packageId 不能为空")
	}
	var pkg entity.ClientPackage
	// 先查出 local_path 再删文件
	list, err := s.packageRepo.ListAll(ctx)
	if err != nil {
		return err
	}
	for _, p := range list {
		if p.PackageId == packageId {
			if p.LocalPath != "" {
				_ = os.Remove(filepath.Join(s.packageDir, p.LocalPath))
			}
			break
		}
	}
	_ = pkg
	return s.packageRepo.Delete(ctx, packageId)
}

// filenameFromURL 从 URL 提取最后一段作为文件名。
func filenameFromURL(u string) string {
	// 去掉 query/fragment
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	u = strings.TrimRight(u, "/")
	if i := strings.LastIndex(u, "/"); i >= 0 {
		u = u[i+1:]
	}
	return u
}

// parseContentDispositionFilename 从 Content-Disposition 头取 filename。
func parseContentDispositionFilename(cd string) string {
	// 形如: attachment; filename="Obsidian-1.12.7.dmg"
	for _, part := range strings.Split(cd, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), "filename=") {
			v := strings.TrimPrefix(part, "filename=")
			v = strings.Trim(v, "\"")
			return v
		}
	}
	return ""
}

// ===== 远程指令队列 =====

// AdminEnqueueCommand 管理员下发指令（pending）。action 合法性校验。
// openclaw/hermes 特殊：install/restart/start 自动签发新 LLM key，把配置内容塞进单条命令的
// payload.configContent，App 执行时先写配置再装/启/重启——单条命令完成，无命令链回传竞态。
// llmBaseURL 用于签发 LLM key 并构造配置。签发后严格校验：key 无可用模型 → 拒绝入队并报错。
// commandTimeout 每个 action 的执行限时。install/plugin_install/channel_login 较长
// （装包/扫码需要），restart/stop/start/shell/config_push 中等，其它短。超时由 cleanupStaleRunning 兜底。
func commandTimeout(action string) time.Duration {
	switch action {
	case "install":
		return 20 * time.Minute
	case "skill_install":
		return 10 * time.Minute
	case "plugin_install", "channel_login":
		return 5 * time.Minute
	case "skill_uninstall", "restart", "stop", "start", "shell", "config_push":
		return 60 * time.Second
	default:
		return 30 * time.Second
	}
}

// timeoutOf 命令超时：timeout_seconds>0 则覆盖 action 默认（per-command 可配）。
func timeoutOf(c *entity.ClientCommand) time.Duration {
	if c.TimeoutSeconds > 0 {
		return time.Duration(c.TimeoutSeconds) * time.Second
	}
	return commandTimeout(c.Action)
}

// cleanupStaleRunning idle 超时兜底：基准为 last_progress_at（进度最近变化时间，认领时=started_at）。
// 进度连续无变化超过 timeout 才标 failed——是「无进度时长」非「命令总时长」。App 一直上报新进度
// （UpdateProgress 检测到变化会刷新 last_progress_at）就不会触发；仅兜底真正卡死 / 无输出的 running 命令。
func (s *ClientService) cleanupStaleRunning(ctx context.Context, deviceId string) {
	running, err := s.commandRepo.ListRunningByDevice(ctx, deviceId)
	if err != nil || len(running) == 0 {
		return
	}
	now := time.Now()
	for _, c := range running {
		base := c.LastProgressAt
		if base == nil {
			base = c.StartedAt
		}
		if base == nil {
			continue
		}
		timeout := timeoutOf(c)
		if now.Sub(time.Time(*base)) > timeout {
			_ = s.commandRepo.UpdateResult(ctx, c.CommandId, "failed", "{}", "执行超时（连续 "+timeout.String()+" 无进度变化）")
			log.Printf("[command] 超时清理(idle): id=%s action=%s last_progress=%s",
				c.CommandId, c.Action, time.Time(*base).Format(time.RFC3339))
		}
	}
}

func (s *ClientService) AdminEnqueueCommand(ctx context.Context, deviceId, action, agent, version string, payload json.RawMessage, createdBy, llmBaseURL string) (string, error) {
	if deviceId == "" {
		return "", fmt.Errorf("deviceId 不能为空")
	}
	switch action {
	case "install", "uninstall", "start", "stop", "restart", "config_push", "collect_data",
		"channel_login", "shell", "appperm_reset", "skill_install", "skill_uninstall":
	default:
		return "", fmt.Errorf("未知 action: %s", action)
	}
	dev, err := s.deviceRepo.GetById(ctx, deviceId)
	if err != nil {
		return "", fmt.Errorf("设备不存在: %w", err)
	}
	// 串行：设备有 running 命令（上一条没执行完）→ 拒绝下发，避免堆积。
	// pending 不全拦（一键启用会连发 plugin_install+restart，需排队串行执行），
	// 但同 (agent, action) 的 pending 要拦——防快速双击在第一条还没被客户端认领
	// （仍 pending）时又入队一条相同动作：曾出现连续 2 条 restart，第二条撞第一条
	// 重启 gateway 的冷启动窗口而失败。
	if s.commandRepo != nil {
		if running, e := s.commandRepo.ListRunningByDevice(ctx, deviceId); e == nil && len(running) > 0 {
			return "", fmt.Errorf("有命令执行中（%s），请等待完成或超时后再下发", running[0].Action)
		}
		if pending, e := s.commandRepo.ListPendingByDevice(ctx, deviceId); e == nil {
			for _, c := range pending {
				if c.Agent != agent || c.Action != action {
					continue
				}
				// skill_install 例外：MCP 配对补装会一次排队多个「不同技能」的同 action 命令，
				// 只有 payload 完全相同（同 skillId+versionId，双击重复）才拦。
				if action == "skill_install" && string(c.Payload) != string(payload) {
					continue
				}
				return "", fmt.Errorf("已有相同的「%s」指令在排队，请等待执行完成", action)
			}
		}
	}
	if len(payload) == 0 {
		payload = json.RawMessage("{}")
	}

	// openclaw/hermes install/restart/start：签发新 LLM key，把配置塞进 payload.configContent
	// （openclaw→openclaw.json，hermes→config.yaml）。App 执行时先写配置再装/启。
	needConfig := (agent == "openclaw" || agent == "hermes") && s.llmKeySvc != nil && llmBaseURL != "" &&
		(action == "install" || action == "restart" || action == "start")
	if needConfig {
		var cfgJSON string
		var cerr error
		if agent == "hermes" {
			cfgJSON, cerr = s.issueHermesConfig(ctx, dev, llmBaseURL, createdBy)
		} else {
			cfgJSON, cerr = s.issueOpenclawConfig(ctx, dev, llmBaseURL, createdBy)
		}
		if cerr != nil {
			return "", cerr
		}
		// 把 configContent 合并进原 payload（保留原有字段如 install 的 version）
		var p map[string]interface{}
		if json.Unmarshal(payload, &p) != nil {
			p = map[string]interface{}{}
		}
		p["configContent"] = cfgJSON
		b, merr := json.Marshal(p)
		if merr != nil {
			return "", merr
		}
		payload = b
	}

	// per-command 超时：install 单独给 20 分钟（hermes/openclaw 安装较久，且只要持续有进度就不超时）；
	// skill_install 10 分钟（下载 zip + 解压 + openclaw skills install）。
	// 其它命令 timeout_seconds=0 → 走 commandTimeout(action) 默认。
	timeoutSec := 0
	if action == "install" {
		timeoutSec = 1200
	} else if action == "skill_install" {
		timeoutSec = 600
	}
	cmd := &entity.ClientCommand{
		DeviceId:       deviceId,
		Action:         action,
		Agent:          agent,
		Version:        version,
		Payload:        payload,
		Status:         "pending",
		CreatedBy:      createdBy,
		TimeoutSeconds: timeoutSec,
	}
	// 源头兜底：把目标 agent 补进 selectedAgents，避免心跳过滤吞掉命令结果。
	// best-effort（即便没补上，客户端主修也保证当前命令的 agent 始终上报）。
	s.ensureAgentSelected(ctx, deviceId, agent)
	if err := s.commandRepo.Enqueue(ctx, cmd); err != nil {
		return "", err
	}
	return cmd.CommandId, nil
}

// signDeviceKey 签发/轮换设备某 agent 的 LLM key（对齐 instance：alias=client:<deviceId>:<agent>）。
// 首次（alias 无 key）→ Create；已有 → Rotate（同 instance RefreshLLMKey，5 分钟宽限零停机）。
// 返回明文 key（内联进配置下发）+ tokenHash（存 device.extra.llm_key_hash 供溯源/吊销）。
func (s *ClientService) signDeviceKey(ctx context.Context, dev *entity.ClientDevice, agent, createdBy string) (string, string, error) {
	alias := fmt.Sprintf("client:%s:%s", dev.DeviceId, agent)
	if existing, e := s.llmKeySvc.GetByAlias(ctx, alias); e == nil && existing != nil && existing.TokenHash != "" {
		// 已有 → 轮换（不 delete+create，保留 alias 身份，对齐 instance RefreshLLMKey）
		plain, rerr := s.llmKeySvc.Rotate(ctx, existing.TokenHash, createdBy)
		if rerr != nil {
			return "", "", fmt.Errorf("轮换 LLM key 失败: %w", rerr)
		}
		hash := existing.TokenHash
		if k, e2 := s.llmKeySvc.GetByAlias(ctx, alias); e2 == nil && k != nil {
			hash = k.TokenHash
		}
		return plain, hash, nil
	}
	// 首次 → 创建
	hash, plain, cerr := s.llmKeySvc.Create(ctx, &entity.LLMVirtualKey{
		KeyAlias:  alias,
		UserId:    dev.UserId,
		CreatedBy: createdBy,
	})
	if cerr != nil {
		return "", "", fmt.Errorf("签发 LLM key 失败: %w", cerr)
	}
	return plain, hash, nil
}

// setDeviceKeyHash 把某 agent 的 LLM key hash 合并存进 device.extra.llm_key_hash（per-agent map），
// 保留 extra 其它字段。best-effort（失败仅记日志，不阻断下发）。对齐 instance 的 llm_key_hash 落行。
func (s *ClientService) setDeviceKeyHash(ctx context.Context, deviceId, agent, hash string) {
	if hash == "" {
		return
	}
	dev, err := s.deviceRepo.GetById(ctx, deviceId)
	if err != nil {
		return
	}
	var m map[string]interface{}
	if dev.Extra == "" || json.Unmarshal([]byte(dev.Extra), &m) != nil {
		m = map[string]interface{}{}
	}
	h, _ := m["llm_key_hash"].(map[string]interface{})
	if h == nil {
		h = map[string]interface{}{}
	}
	h[agent] = hash
	m["llm_key_hash"] = h
	b, _ := json.Marshal(m)
	_ = s.deviceRepo.UpdateExtra(ctx, deviceId, string(b))
}

// issueOpenclawConfig 签发新 LLM key 并构造完整合法的 openclaw.json 内容。
// 复用/替换：按 alias 查旧 key，有则删除（llm 库不累积废弃 key），再签新的。
// 配置结构遵循 openclaw 官方 schema（gateway.mode=local 等），覆盖写——设备由后台统一管理。
func (s *ClientService) issueOpenclawConfig(ctx context.Context, dev *entity.ClientDevice, llmBaseURL, createdBy string) (string, error) {
	plainKey, hash, kerr := s.signDeviceKey(ctx, dev, "openclaw", createdBy)
	if kerr != nil {
		return "", kerr
	}
	s.setDeviceKeyHash(ctx, dev.DeviceId, "openclaw", hash)
	// channels：设备已启用的渠道合并进 openclaw.json（plugins.entries.<channel>.enabled + channels.<channel>）
	channelsCfg := map[string]interface{}{}
	pluginsEntries := map[string]interface{}{}
	for _, ch := range parseDeviceChannels(dev.Channels) {
		if !ch.Enabled {
			continue
		}
		pluginsEntries[ch.Channel] = map[string]interface{}{"enabled": true}
		channelsCfg[ch.Channel] = map[string]interface{}{
			"dmPolicy":    ch.DmPolicy,
			"groupPolicy": ch.GroupPolicy,
		}
	}
	// openclaw.json 走共享构造（agent_openclaw.go BuildOpenclawConfig）：模型列表 + gateway + channels/plugins。
	// 模型严格校验：刚签的 key 拉不到可用模型 → 下发直接失败报错（不回退假 default——配置发了也用不了，
	// 拖到运行期才暴露更难排查）。
	models, merr := FetchLLMModels(ctx, s.llmModelRepo, llmBaseURL, plainKey)
	if merr != nil {
		return "", fmt.Errorf("该设备的 LLM 密钥无可用模型，已中止下发：%w。请先在「LLM 管理」配置可用的凭据/模型后重试", merr)
	}
	opts := OpenclawConfigOpts{
		LLMBaseURL:     llmBaseURL,
		APIKey:         plainKey,
		API:            OpenclawProviderAPI(ctx, s.llmModelRepo),
		Models:         models,
		Workspace:      "~/.openclaw/workspace",
		Channels:       channelsCfg, // 非 nil → 写出 channels 对象（openclaw 要求 channels 是对象）
		PluginsEntries: pluginsEntries,
		// exec 强制路由进 client node：微信等渠道驱动 exec 时默认 host=auto 会落本机网关直跑
		// shell（osascript 通讯录/screencapture 的 TCC 授权对象变成网关 node 二进制）。
		ExecNode: openclawDesktopNodeName,
	}
	// 已绑 MCP 分组 → mcp.servers（无绑定 nil 不写段；best-effort 不阻断配置下发）。
	opts.McpServers = s.deviceMcpServers(ctx, dev)
	cfgJSON := BuildOpenclawConfig(opts)
	return cfgJSON, nil
}

// issueHermesConfig 签发新 LLM key 并构造 hermes config.yaml 内容（model 指向 aiarsenal LLM 网关）。
// 复用/替换：按 alias 查旧 key 删，再签新的。配置只含 model（provider=custom + base_url + 内联 key），
// 不含 platforms.weixin——hermes 见 .env WEIXIN_* 自动加载，桌面端 writeEffectiveConfig 按绑定态叠加 weixin 块。
func (s *ClientService) issueHermesConfig(ctx context.Context, dev *entity.ClientDevice, llmBaseURL, createdBy string) (string, error) {
	plainKey, hash, kerr := s.signDeviceKey(ctx, dev, "hermes", createdBy)
	if kerr != nil {
		return "", kerr
	}
	s.setDeviceKeyHash(ctx, dev.DeviceId, "hermes", hash)
	// config.yaml 走共享构造（agent_hermes.go BuildHermesConfig）：单 default 模型 + provider=custom + 内联 key。
	// 模型严格校验（与 issueOpenclawConfig 同理）：无可用模型 → 下发失败报错。
	defaultModel, provider, merr := FetchHermesDefaultModel(ctx, s.llmModelRepo, llmBaseURL, plainKey)
	if merr != nil {
		return "", fmt.Errorf("该设备的 LLM 密钥无可用模型，已中止下发：%w。请先在「LLM 管理」配置可用的凭据/模型后重试", merr)
	}
	// MCP 工具 = 设备已绑 MCP 分组 + 客户端本地桌面控制 server（aa-desktop，loopback）。
	mcpServers := s.deviceMcpServers(ctx, dev)
	if mcpServers == nil {
		mcpServers = map[string]interface{}{}
	}
	// aa-desktop：客户端 desktop_node 内置的环回 MCP server（127.0.0.1:43250，无鉴权——
	// 仅本机可达）。hermes 经它获得 desktop_run/desktop_act/desktop_snapshot/desktop_notify
	// 四个桌面工具，全部走客户端 dispatch 闸门（App 权限清单 + exec 审批），与 openclaw
	// node 协议同一套权限边界。端口与客户端 mcp_server.rs DESKTOP_MCP_PORT 编译期约定，
	// 改须两边同步。无条件注入（未绑 MCP 分组也注入）：闸门在客户端 fail-closed，空 App
	// 清单时桌面命令会被拒，无安全敞口。
	mcpServers["aa-desktop"] = map[string]interface{}{"url": deviceDesktopMcpURL}
	cfgYAML := BuildHermesConfig(HermesConfigOpts{
		LLMBaseURL:   llmBaseURL,
		APIKey:       plainKey,
		APIMode:      hermesAPIModeForProvider(provider), // anthropic 族须走 /v1/messages
		DefaultModel: defaultModel,
		// 已绑 MCP 分组 → mcp_servers（与 issueOpenclawConfig 同理：无绑定 nil 不写段，
		// best-effort 不阻断配置下发）。缺这段时设备 hermes 无 MCP 工具，微信里问员工信息
		// agent 只能空答（tool_turns=0）——设备侧 v1 漏配，2026-09-19 补齐。
		McpServers: mcpServers,
	})
	return cfgYAML, nil
}

// deviceDesktopMcpURL 客户端本地桌面控制 MCP server 的环回地址。仅 hermes 下发
// （openclaw 有原生 node 协议路径，不需要）；端口与客户端 mcp_server.rs 约定一致。
const deviceDesktopMcpURL = "http://127.0.0.1:43250/mcp"

// deviceMcpServers 设备已绑 MCP 分组的 mcp 段（openclaw/hermes 配置共用）。mcpSvc 未装配/无绑定返回 nil。
func (s *ClientService) deviceMcpServers(ctx context.Context, dev *entity.ClientDevice) map[string]interface{} {
	if s.mcpSvc == nil {
		return nil
	}
	return s.mcpSvc.DeviceMcpServers(ctx, dev)
}

// builtinSkillIdBySlug 技能库按 slug 查技能 id，跨命名空间同 slug 优先内置源（Source=builtin）。
// 未配置技能仓库/查不到返回空。MCP 配对补装（client_mcp_service.pairDomainSkills）用。
func (s *ClientService) builtinSkillIdBySlug(ctx context.Context, slug string) string {
	if s.skillRepo == nil {
		return ""
	}
	list, err := s.skillRepo.ListBySlug(ctx, slug)
	if err != nil || len(list) == 0 {
		return ""
	}
	sk := list[0]
	for _, cand := range list {
		if cand.Source == "builtin" {
			sk = cand
			break
		}
	}
	return sk.SkillId
}

// parseDeviceChannels 解析设备 channels jsonb 为状态数组；空/非法返回 nil。
func parseDeviceChannels(channelsJSON string) []entity.ClientChannelState {
	if channelsJSON == "" || channelsJSON == "[]" {
		return nil
	}
	var list []entity.ClientChannelState
	if json.Unmarshal([]byte(channelsJSON), &list) != nil {
		return nil
	}
	return list
}

// parseDeviceSkills 解析设备 skills jsonb；空/坏数据一律回 nil（列表展示为空）。
func parseDeviceSkills(skillsJSON string) []entity.ClientSkillState {
	if skillsJSON == "" || skillsJSON == "[]" {
		return nil
	}
	var list []entity.ClientSkillState
	if json.Unmarshal([]byte(skillsJSON), &list) != nil {
		return nil
	}
	return list
}

// AdminListCommands 某设备指令历史。
func (s *ClientService) AdminListCommands(ctx context.Context, deviceId string, limit int) ([]*entity.ClientCommand, error) {
	return s.commandRepo.ListByDevice(ctx, deviceId, limit)
}

// selectedAgentsFromExtra 从 device.extra jsonb 解析 selectedAgents 数组。
// extra 形如 {"selectedAgents":["openclaw","hermes"]}。损坏/无则返回 nil。
func selectedAgentsFromExtra(extra string) []string {
	if extra == "" || extra == "{}" {
		return nil
	}
	var m map[string]interface{}
	if json.Unmarshal([]byte(extra), &m) != nil {
		return nil
	}
	arr, _ := m["selectedAgents"].([]interface{})
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// selectedAgentsSet 同上，返回 map[string]bool 便于心跳过滤用。
func selectedAgentsSet(extra string) map[string]bool {
	list := selectedAgentsFromExtra(extra)
	m := make(map[string]bool, len(list))
	for _, c := range list {
		m[c] = true
	}
	return m
}

// AdminSetSelectedAgents 设置某设备选中的 agent（存 device.extra.selectedAgents）。
// App 心跳据此上报对应 agent；下发指令 tab 也只显示这些 agent。
func (s *ClientService) AdminSetSelectedAgents(ctx context.Context, deviceId string, agents []string) error {
	dev, err := s.deviceRepo.GetById(ctx, deviceId)
	if err != nil {
		return fmt.Errorf("设备不存在: %w", err)
	}
	// 读出现有 extra，合并改 selectedAgents（保留 extra 里其它字段）
	var m map[string]interface{}
	if dev.Extra == "" || json.Unmarshal([]byte(dev.Extra), &m) != nil {
		m = map[string]interface{}{}
	}
	// 去重去空
	seen := make(map[string]bool, len(agents))
	clean := make([]string, 0, len(agents))
	for _, a := range agents {
		if a == "" || seen[a] {
			continue
		}
		seen[a] = true
		clean = append(clean, a)
	}
	m["selectedAgents"] = clean
	b, _ := json.Marshal(m)
	return s.deviceRepo.UpdateExtra(ctx, deviceId, string(b))
}

// ensureAgentSelected 确保目标 agent 在设备 selectedAgents 里。
// 下发某 agent 的命令时调用：若该 agent 不在选中列表，客户端心跳会过滤掉它的 module，
// 导致命令的 done/failed 永远上报不到（只能 20min 后超时标 failed，实际安装已成功）。
// 这里从源头补上，best-effort：失败仅记日志，不阻断命令下发。
func (s *ClientService) ensureAgentSelected(ctx context.Context, deviceId, agent string) {
	if agent == "" || agent == "client" {
		return
	}
	dev, err := s.deviceRepo.GetById(ctx, deviceId)
	if err != nil {
		return
	}
	list := selectedAgentsFromExtra(dev.Extra)
	for _, a := range list {
		if a == agent {
			return // 已选中，无需补
		}
	}
	list = append(list, agent)
	if err := s.AdminSetSelectedAgents(ctx, deviceId, list); err != nil {
		log.Printf("[command] ensureAgentSelected 写 selectedAgents 失败: dev=%s agent=%s err=%v", deviceId, agent, err)
	}
}

// mergeChannelReports 合并 App 心跳上报的渠道状态进设备 channels jsonb。
// 仅更新已启用渠道的登录态/QR/昵称/消息（enabled/dmPolicy/groupPolicy 由后台管理，不被心跳覆盖）；
// connected 时清空 qrDataUrl。读-改-写整列。
func (s *ClientService) mergeChannelReports(ctx context.Context, deviceId string, reports []ChannelStateReport) {
	dev, err := s.deviceRepo.GetById(ctx, deviceId)
	if err != nil {
		return
	}
	list := parseDeviceChannels(dev.Channels)
	if len(list) == 0 {
		return
	}
	byChannel := make(map[string]int, len(list))
	for i := range list {
		byChannel[list[i].Channel] = i
	}
	changed := false
	for _, r := range reports {
		idx, ok := byChannel[r.Channel]
		if !ok {
			continue // 设备未启用该渠道，忽略
		}
		cur := &list[idx]
		if r.LoginStatus != "" && r.LoginStatus != cur.LoginStatus {
			cur.LoginStatus = r.LoginStatus
			changed = true
		}
		qr := r.QrDataUrl
		if r.LoginStatus == "connected" {
			qr = "" // 登录成功清 QR
		}
		if qr != cur.QrDataUrl {
			cur.QrDataUrl = qr
			changed = true
		}
		if r.AccountAlias != "" && r.AccountAlias != cur.AccountAlias {
			cur.AccountAlias = r.AccountAlias
			changed = true
		}
		if r.Message != "" && r.Message != cur.LastMessage {
			cur.LastMessage = r.Message
			changed = true
		}
	}
	if !changed {
		return
	}
	b, _ := json.Marshal(list)
	_ = s.deviceRepo.UpdateChannels(ctx, deviceId, string(b))
}

// AdminListChannels 返回某设备的渠道绑定状态数组。
func (s *ClientService) AdminListChannels(ctx context.Context, deviceId string) ([]entity.ClientChannelState, error) {
	dev, err := s.deviceRepo.GetById(ctx, deviceId)
	if err != nil {
		return nil, fmt.Errorf("设备不存在: %w", err)
	}
	return parseDeviceChannels(dev.Channels), nil
}

// deviceSkillPackagePath 设备用自身 device token 拉取 skill 整包 ZIP 的端点（相对路径，
// App 持有 api baseURL + Bearer token，拼接即得完整链接）。与 DownloadVersion 同源
// （复用 pkgSvc.DownloadVersion），仅鉴权方式不同：本端点挂 clientAuth（设备 token）。
const deviceSkillPackagePath = "/api/v1/system/client/skill/package"

// resolveSkillVersionId 解析技能「最新版本」id：优先 skill.latest_version_id；未发布
// （仅回填于 PublishVersion）则回退最新非 yanked 版本（草稿也可装，DownloadVersion 不卡
// status）。install 与已装列表的「库最新」列共用，保证两处口径一致。
// 安全口径：候选版本还须最新安全审计为「安全」（不安全/扫描中/失败/无记录跳过），
// 即「最新」=最新的安全版本。
func (s *ClientService) resolveSkillVersionId(ctx context.Context, sk *entity.Skill) (string, error) {
	if sk.LatestVersionId != "" {
		if v, err := s.skillRepo.GetVersion(ctx, sk.LatestVersionId); err == nil && v != nil &&
			v.Status != 2 && ensureSkillVersionSafe(ctx, s.skillSecRepo, v.VersionId) == nil {
			return v.VersionId, nil
		}
	}
	versions, err := s.skillRepo.ListVersions(ctx, sk.SkillId)
	if err != nil {
		return "", err
	}
	for _, v := range versions { // ListVersions 按 id desc，最新优先
		if v.Status == 2 { // 跳过已 yanked
			continue
		}
		if ensureSkillVersionSafe(ctx, s.skillSecRepo, v.VersionId) != nil { // 跳过非「安全」
			continue
		}
		return v.VersionId, nil
	}
	return "", nil
}

// AdminListDeviceSkills 设备已装技能列表（aa_client_device.skills 快照，客户端心跳上报）。
// 按 slug 回查技能库附「库最新」版本信息：已装版本 ≠ 库最新时前端出「升级」按钮
// （版本是自由文本，不做大小推断）。库内无此 slug（手工装）对照字段留空。
func (s *ClientService) AdminListDeviceSkills(ctx context.Context, deviceId string) ([]entity.DeviceSkillView, error) {
	dev, err := s.deviceRepo.GetById(ctx, deviceId)
	if err != nil {
		return nil, fmt.Errorf("设备不存在: %w", err)
	}
	states := parseDeviceSkills(dev.Skills)
	views := make([]entity.DeviceSkillView, 0, len(states))
	resolved := make(map[string]*entity.DeviceSkillView, len(states)) // slug→view 缓存（同 slug 多行只查一次库）
	for i := range states {
		v := entity.DeviceSkillView{ClientSkillState: states[i]}
		views = append(views, v)
		if states[i].Slug == "" {
			continue
		}
		if prev, ok := resolved[states[i].Slug]; ok {
			if prev != nil { // 命中缓存：复制对照字段
				views[i].SkillId = prev.SkillId
				views[i].LatestVersionId = prev.LatestVersionId
				views[i].LatestVersion = prev.LatestVersion
			}
			continue
		}
		resolved[states[i].Slug] = nil // 先占位：库查不到时后续同 slug 不再查
		if s.skillRepo == nil {
			continue
		}
		cands, cerr := s.skillRepo.ListBySlug(ctx, states[i].Slug)
		if cerr != nil || len(cands) == 0 {
			continue
		}
		sk := cands[0] // slug 非全局唯一：优先启用且未隐藏的；都不满足取最新一条
		for _, c := range cands {
			if c.Status == 1 && c.Hidden == 0 {
				sk = c
				break
			}
		}
		verId, verr := s.resolveSkillVersionId(ctx, sk)
		if verr != nil || verId == "" {
			continue
		}
		ver, gerr := s.skillRepo.GetVersion(ctx, verId)
		if gerr != nil || ver == nil {
			continue
		}
		views[i].SkillId = sk.SkillId
		views[i].LatestVersionId = verId
		views[i].LatestVersion = ver.Version
		resolved[states[i].Slug] = &views[i]
	}
	return views, nil
}

// resolveSkillInstallAgent 解析技能装/卸的目标 agent：显式传 hermes/openclaw 原样用；
// 空则按设备已启用 agent 推断——只启用 hermes → hermes，其余（只 openclaw / 空 / 双开）
// → openclaw（保持既有行为；双开设备要装到 hermes 须显式指定 agent）。
func resolveSkillInstallAgent(extra, agent string) (string, error) {
	switch agent {
	case "hermes", "openclaw":
		return agent, nil
	case "":
		hasOpenclaw, hasHermes := false, false
		for _, a := range selectedAgentsFromExtra(extra) {
			switch a {
			case "openclaw":
				hasOpenclaw = true
			case "hermes":
				hasHermes = true
			}
		}
		if hasHermes && !hasOpenclaw {
			return "hermes", nil
		}
		return "openclaw", nil
	default:
		return "", fmt.Errorf("不支持的 agent: %s（仅 openclaw/hermes）", agent)
	}
}

// AdminInstallDeviceSkill 向设备下发 skill_install：payload 携带取包相对路径 + slug，
// App 收到后自行下载 ZIP、解压，按 agent 落地——openclaw 走 skills install --global，
// hermes 直接落盘 $HERMES_HOME/skills（扫盘即生效）。
// 版本解析与容器侧 install_skill 一致：优先 skill.latest_version_id；未发布（仅回填于
// PublishVersion）则回退最新非 yanked 版本（草稿也可装，DownloadVersion 不卡 status）。
// 安全门禁：显式指定/解析出的版本最新审计须为「安全」，否则拒绝下发（取包端点同拦）。
func (s *ClientService) AdminInstallDeviceSkill(ctx context.Context, deviceId, skillId, versionId, agent, createdBy string) (string, error) {
	dev, err := s.deviceRepo.GetById(ctx, deviceId)
	if err != nil {
		return "", fmt.Errorf("设备不存在: %w", err)
	}
	if s.skillRepo == nil {
		return "", fmt.Errorf("技能仓库未配置")
	}
	sk, err := s.skillRepo.Get(ctx, skillId)
	if err != nil || sk == nil {
		return "", fmt.Errorf("技能不存在: %s", skillId)
	}
	if versionId == "" {
		versionId, err = s.resolveSkillVersionId(ctx, sk)
		if err != nil {
			return "", fmt.Errorf("查询版本失败: %w", err)
		}
	}
	if versionId == "" {
		return "", fmt.Errorf("技能无可用版本: %s", skillId)
	}
	if err := ensureSkillVersionSafe(ctx, s.skillSecRepo, versionId); err != nil {
		return "", err
	}
	payload, _ := json.Marshal(map[string]any{
		"skillId":     skillId,
		"versionId":   versionId,
		"slug":        sk.Slug,
		"name":        sk.DisplayName,
		"packagePath": fmt.Sprintf("%s?versionId=%s", deviceSkillPackagePath, versionId),
	})
	targetAgent, err := resolveSkillInstallAgent(dev.Extra, agent)
	if err != nil {
		return "", err
	}
	// agent 决定 App 侧落地目录：openclaw 全局托管目录（~/.openclaw/skills），
	// hermes 直接落盘 $HERMES_HOME/skills；命令结果随对应 agent 模块上报
	// （ensureAgentSelected 会补 selectedAgents）。
	return s.AdminEnqueueCommand(ctx, deviceId, "skill_install", targetAgent, "", payload, createdBy, "")
}

// AdminUninstallDeviceSkill 向设备下发 skill_uninstall：payload 携带 slug，App 据此定位
// 目标 agent 的技能目录删除（openclaw 托管目录与 hermes $HERMES_HOME/skills
// 均为删目录即卸载，扫描/监听自动生效；幂等）。
func (s *ClientService) AdminUninstallDeviceSkill(ctx context.Context, deviceId, slug, agent, createdBy string) (string, error) {
	dev, err := s.deviceRepo.GetById(ctx, deviceId)
	if err != nil {
		return "", fmt.Errorf("设备不存在: %w", err)
	}
	if slug == "" {
		return "", fmt.Errorf("slug 不能为空")
	}
	targetAgent, err := resolveSkillInstallAgent(dev.Extra, agent)
	if err != nil {
		return "", err
	}
	payload, _ := json.Marshal(map[string]any{"slug": slug})
	return s.AdminEnqueueCommand(ctx, deviceId, "skill_uninstall", targetAgent, "", payload, createdBy, "")
}

// AdminSetChannel 启用/禁用某渠道（微信等）。enabled=true 时若该渠道未在列表则追加。
// 仅写 channels jsonb；装插件+生效配置由调用方再下发 plugin_install + restart。
func (s *ClientService) AdminSetChannel(ctx context.Context, deviceId, channel string, enabled bool, dmPolicy, groupPolicy string) error {
	dev, err := s.deviceRepo.GetById(ctx, deviceId)
	if err != nil {
		return fmt.Errorf("设备不存在: %w", err)
	}
	if dmPolicy == "" {
		dmPolicy = "pairing"
	}
	if groupPolicy == "" {
		groupPolicy = "disabled"
	}
	list := parseDeviceChannels(dev.Channels)
	idx := -1
	for i := range list {
		if list[i].Channel == channel {
			idx = i
			break
		}
	}
	if idx < 0 {
		// 新增：默认 idle
		list = append(list, entity.ClientChannelState{
			Channel:     channel,
			AccountId:   "default",
			Enabled:     enabled,
			DmPolicy:    dmPolicy,
			GroupPolicy: groupPolicy,
			LoginStatus: "idle",
		})
	} else {
		list[idx].Enabled = enabled
		list[idx].DmPolicy = dmPolicy
		list[idx].GroupPolicy = groupPolicy
		if !enabled {
			// 禁用时清登录态
			list[idx].LoginStatus = "idle"
			list[idx].QrDataUrl = ""
			list[idx].AccountAlias = ""
		}
	}
	b, _ := json.Marshal(list)
	return s.deviceRepo.UpdateChannels(ctx, deviceId, string(b))
}

// AdminGetChannelQr 取某渠道当前 QR/登录态（管理后台扫码用）。
func (s *ClientService) AdminGetChannelQr(ctx context.Context, deviceId, channel string) (*entity.ClientChannelState, error) {
	list, err := s.AdminListChannels(ctx, deviceId)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].Channel == channel {
			return &list[i], nil
		}
	}
	return nil, fmt.Errorf("渠道未配置: %s", channel)
}

// agentOf 渠道名前缀即 agent key（openclaw-weixin→openclaw、hermes-weixin→hermes）；无前缀则原样返回。
func agentOf(channel string) string {
	if i := strings.IndexByte(channel, '-'); i > 0 {
		return channel[:i]
	}
	return channel
}

// AdminStartChannelLogin 下发 channel_login 命令。openclaw 走 web.login RPC/CLI；hermes 走 Python qr_login shim
// （桌面 App 按 agent 分派）。agent 由渠道名前缀推导。
func (s *ClientService) AdminStartChannelLogin(ctx context.Context, deviceId, channel, createdBy string) (string, error) {
	// 重置该渠道登录态：清掉上次 failed/二维码残留。否则前端轮询会立刻读到历史
	// loginStatus=failed → message.error('登录失败') + stopQrPoll，新命令还没执行就被误判失败。
	// 客户端执行 channel_login 后通过心跳上报本次真实状态（qr_pending/qrDataUrl/connected/failed）覆盖。
	s.resetChannelLogin(ctx, deviceId, channel)
	agent := agentOf(channel)
	payload := map[string]interface{}{"channel": channel}
	// openclaw 外部插件渠道（微信等）：channels.* 配置只在 gateway 启动时加载（openclaw 文档
	// channels/wechat.md「Restart the Gateway after install」+ AGENTS.md「Gateway/plugin metadata is
	// process-stable, changes require restart」）。把 dmPolicy/groupPolicy 带给客户端，channel_login 执行时
	// 据此 `openclaw config set` 写渠道配置 + `gateway restart` 让插件加载——自包含确保就绪，不依赖
	// 「启用渠道那次 restart 恰好成功」（曾因 restart 失败致 channels 配置始终未加载、扫码必失败）。
	if agent == "openclaw" {
		if dev, err := s.deviceRepo.GetById(ctx, deviceId); err == nil && dev != nil {
			for _, ch := range parseDeviceChannels(dev.Channels) {
				if ch.Channel == channel {
					if ch.DmPolicy != "" {
						payload["dmPolicy"] = ch.DmPolicy
					}
					if ch.GroupPolicy != "" {
						payload["groupPolicy"] = ch.GroupPolicy
					}
					break
				}
			}
		}
	}
	b, _ := json.Marshal(payload)
	return s.AdminEnqueueCommand(ctx, deviceId, "channel_login", agent, "", b, createdBy, "")
}

// resetChannelLogin 清某渠道登录态（loginStatus 置空、清 qrDataUrl/lastMessage）。best-effort。
func (s *ClientService) resetChannelLogin(ctx context.Context, deviceId, channel string) {
	dev, err := s.deviceRepo.GetById(ctx, deviceId)
	if err != nil {
		return
	}
	list := parseDeviceChannels(dev.Channels)
	changed := false
	for i := range list {
		if list[i].Channel == channel {
			c := &list[i]
			if c.LoginStatus != "" || c.QrDataUrl != "" || c.LastMessage != "" || c.AccountAlias != "" {
				c.LoginStatus = ""
				c.QrDataUrl = ""
				c.LastMessage = ""
				c.AccountAlias = "" // 旧账号昵称一并清（解绑后不残留）
				changed = true
			}
		}
	}
	if !changed {
		return
	}
	b, _ := json.Marshal(list)
	_ = s.deviceRepo.UpdateChannels(ctx, deviceId, string(b))
}

// AdminChannelLogout 下发登出命令。hermes 走 channel_logout action（桌面端清 .env + 重生成 config + restart）；
// openclaw 走 shell 跑 channels logout——但 openclaw-weixin 插件无 logout 能力（CLI 报
// "does not support logout"），对它登出=清账号，走与解绑同一套 stop→rm→start。
func (s *ClientService) AdminChannelLogout(ctx context.Context, deviceId, channel, createdBy, llmBaseURL string) (string, error) {
	if agentOf(channel) == "hermes" {
		return s.AdminEnqueueCommand(ctx, deviceId, "channel_logout", "hermes", "", json.RawMessage("{}"), createdBy, "")
	}
	if channel == "openclaw-weixin" {
		// 账号清了登录态也要清（与解绑同构）：漏掉的话 channels.loginStatus 停留在
		// 旧 connected，前端「扫码登录」按钮 disabled（loginStatus===connected）点不了，
		// 死锁在假"已连接"（实踩：登出后无法重新扫码，只能去点解绑）。
		s.resetChannelLogin(ctx, deviceId, channel)
		return s.enqueueOpenclawAccountClear(ctx, deviceId, channel, createdBy, llmBaseURL)
	}
	payload := fmt.Sprintf(`{"program":"openclaw","args":["channels","logout","--channel",%q]}`, channel)
	return s.AdminEnqueueCommand(ctx, deviceId, "shell", "openclaw", "", json.RawMessage(payload), createdBy, "")
}

// enqueueOpenclawAccountClear openclaw 插件渠道清当前账号：stop → rm 账号文件 → start（设备串行认领执行）。
// openclaw-weixin 等外部插件不支持 channels logout，账号数据在插件状态目录 ~/.openclaw/<channel>/accounts*。
// stop 先停网关（launchd bootout，不会 KeepAlive 重拉），防插件退出时把内存账号回写盘；start 带
// llmBaseURL 重推含渠道的新配置（渠道 enabled 不动），重启后无账号即解绑，可直接扫码绑其它账号。
// 返回首条（stop）命令 id 供前端轮询。
func (s *ClientService) enqueueOpenclawAccountClear(ctx context.Context, deviceId, channel, createdBy, llmBaseURL string) (string, error) {
	agent := agentOf(channel)
	stopId, err := s.AdminEnqueueCommand(ctx, deviceId, "stop", agent, "", json.RawMessage("{}"), createdBy, "")
	if err != nil {
		return "", err
	}
	rmPayload := fmt.Sprintf(
		`{"program":"/bin/sh","args":["-c","rm -rf \"$HOME/.openclaw/%s/accounts.json\" \"$HOME/.openclaw/%s/accounts\""]}`,
		channel, channel)
	if _, err := s.AdminEnqueueCommand(ctx, deviceId, "shell", agent, "", json.RawMessage(rmPayload), createdBy, ""); err != nil {
		return stopId, err
	}
	_, err = s.AdminEnqueueCommand(ctx, deviceId, "start", agent, "", json.RawMessage("{}"), createdBy, llmBaseURL)
	return stopId, err
}

// AdminInstallPlugin 下发通用 shell 命令装 openclaw 渠道插件（如 @tencent-weixin/openclaw-weixin）。
func (s *ClientService) AdminInstallPlugin(ctx context.Context, deviceId, spec, createdBy string) (string, error) {
	payload := fmt.Sprintf(`{"program":"openclaw","args":["plugins","install",%q]}`, spec)
	return s.AdminEnqueueCommand(ctx, deviceId, "shell", "openclaw", "", json.RawMessage(payload), createdBy, "")
}

// AdminUnbindChannel 解绑渠道：只清当前账号绑定，渠道功能保留（enabled 不动，解绑后可直接
// 重新扫码绑其它账号——「解绑微信」不是「下线微信功能」）。openclaw：enqueueOpenclawAccountClear
// （stop→rm 账号文件→start）；hermes：channel_logout（清 .env 凭据 + 重生成 config + restart，自包含）。
// 登录态同步清（昵称/QR/状态），命令入队由设备串行认领执行。
func (s *ClientService) AdminUnbindChannel(ctx context.Context, deviceId, channel, createdBy, llmBaseURL string) error {
	// 1. 清登录态（enabled 保留）
	s.resetChannelLogin(ctx, deviceId, channel)
	// 2. 清账号数据
	if agentOf(channel) == "hermes" {
		_, err := s.AdminEnqueueCommand(ctx, deviceId, "channel_logout", "hermes", "", json.RawMessage("{}"), createdBy, "")
		return err
	}
	_, err := s.enqueueOpenclawAccountClear(ctx, deviceId, channel, createdBy, llmBaseURL)
	return err
}

// AdminGetDeviceDetail 设备详情：设备信息 + 各 agent 最新运行状态 + 该设备选中的 agent 列表。
func (s *ClientService) AdminGetDeviceDetail(ctx context.Context, deviceId string) (*entity.ClientDevice, []*entity.ClientAgentStatus, []string, error) {
	dev, err := s.deviceRepo.GetById(ctx, deviceId)
	if err != nil {
		return nil, nil, nil, err
	}
	statuses, err := s.statusRepo.ListLatestByDevice(ctx, deviceId)
	if err != nil {
		return nil, nil, nil, err
	}
	// 选中 agent（替代旧的 commandedAgents）：用户在设备控制弹窗多选框选的
	selected := selectedAgentsFromExtra(dev.Extra)
	return dev, statuses, selected, nil
}

// ===== 心跳通道（状态+日志+配置变更+命令进度合一） =====

// LogEntry 单条日志（客户端增量上报）。
type LogEntry struct {
	Line  string `json:"line"`
	Level string `json:"level"`
	Ts    string `json:"ts"` // 客户端时间，ISO 字符串
}

// RunningCommandReport 当前执行中命令的进度/结果。
type RunningCommandReport struct {
	CommandId string `json:"commandId"`
	Action    string `json:"action"`   // install/uninstall/start/stop/restart/config_push
	Status    string `json:"status"`   // running/done/failed
	Progress  string `json:"progress"` // 进度 json（step/percent/lastLine）
	Percent   int    `json:"percent"`
	LastLine  string `json:"lastLine"`
	Result    string `json:"result"`
	Error     string `json:"error"`
}

// ModuleReport 单客户端模块上报。
type ModuleReport struct {
	Status         string                `json:"status"`
	Version        string                `json:"version"`
	LastError      string                `json:"lastError"`
	Logs           []LogEntry            `json:"logs"`
	ConfigHash     string                `json:"configHash"`
	ConfigSize     int64                 `json:"configSize"`
	ConfigMtime    string                `json:"configMtime"`
	ConfigContent  string                `json:"configContent"` // hash 变化时才带全文
	RunningCommand *RunningCommandReport `json:"runningCommand"`
	// Env 环境探测（当前仅 openclaw）：本机已有安装的多来源版本 + 是否需版本对齐，
	// 合进状态行 extra.env 供后台展示「待对齐」。client 侧不带时为 nil。
	Env map[string]any `json:"env"`
}

// ChannelStateReport 渠道登录/连接状态上报（App 通过 openclaw channels status / web.login 拿到）。
type ChannelStateReport struct {
	Channel      string `json:"channel"`      // openclaw-weixin
	AccountId    string `json:"accountId"`    // 默认 default
	LoginStatus  string `json:"loginStatus"`  // idle/qr_pending/connected/disconnected/failed
	QrDataUrl    string `json:"qrDataUrl"`    // qr_pending 时的 QR 图片 data URL（登录成功后空）
	AccountAlias string `json:"accountAlias"` // 登录后微信昵称
	Message      string `json:"message"`      // 最近消息
}

// HeartbeatReq 心跳请求。
type HeartbeatReq struct {
	DeviceId  string                  `json:"deviceId"`
	Modules   map[string]ModuleReport `json:"modules"`
	Channels  []ChannelStateReport    `json:"channels"`   // 该设备各渠道登录/连接状态（含扫码期 QR）
	PermState *HeartbeatPermState     `json:"perm_state"` // App 授权状态快照（变更时/启动首跳才带；nil=无上报）
	// Skills 已装技能快照（客户端扫 ~/.openclaw/skills）：nil=旧版本客户端未上报（保留存量），
	// 非 nil（含空数组）=设备真实状态，整体覆盖（卸载到空即清空）。
	Skills []entity.ClientSkillState `json:"skills"`
}

// HeartbeatPermState 客户端上报的 App 授权状态（aa.json granted_perms/denied_perms 镜像）。
type HeartbeatPermState struct {
	GrantedPerms []string `json:"granted_perms"` // 已授权 bundle id 集
	DeniedPerms  []string `json:"denied_perms"`  // 已拒绝 bundle id 集（TCC 缓存拒绝 -1743）
}

// HeartbeatRes 心跳响应。
type HeartbeatRes struct {
	Stored          bool                    `json:"stored"`
	Commands        []*entity.ClientCommand `json:"commands"`
	ActivatedAgents []string                `json:"activatedAgents"` // 该设备选中的 agent，App 据此决定上报哪些
	// 该设备配置的 macOS 自动化授权 App 清单（App 启动/变更时逐个请求 AppleEvents 授权；
	// 空=未配置，客户端仅内置 System Events 授权）。
	// 注意：不做 camelCase 双发垫片——客户端 serde rename+alias 遇双键会 duplicate field
	// 解析失败，且 api 与 App 始终同步重编重起，无长窗口混跑。
	AppPerms []AppPerm `json:"app_perms"`
}

// AppPerm 心跳下发的授权 App（bundle_id 喂 osascript application id，app_name 仅供日志展示）。
type AppPerm struct {
	BundleId string `json:"bundle_id"`
	AppName  string `json:"app_name"`
}

// Heartbeat 处理一次心跳：存状态+追加日志+落配置变更+更新命令进度/结果+认领下一条命令。
func (s *ClientService) Heartbeat(ctx context.Context, userId string, req *HeartbeatReq) (*HeartbeatRes, error) {
	if req.DeviceId == "" {
		return nil, fmt.Errorf("deviceId 不能为空")
	}
	// 诊断日志：记录每次心跳各 agent 上报的状态 + 是否带执行中命令
	for agent, m := range req.Modules {
		rc := ""
		if m.RunningCommand != nil {
			rc = fmt.Sprintf(" cmd=%s:%s", m.RunningCommand.CommandId, m.RunningCommand.Status)
		}
		log.Printf("[heartbeat] dev=%s agent=%s status=%s ver=%q%s",
			req.DeviceId, agent, m.Status, m.Version, rc)
	}
	// 只处理「该设备选中」的 agent：用户在设备控制弹窗多选框选哪些 agent，就落哪些的状态/日志/配置。
	// 替代旧的「下过命令才落库」——这样装过但没下过命令的 agent（如 openclaw）选了也能采上来。
	// 「client」是 App 自身（启动/退出上报在线/离线），始终放行。
	var selected map[string]bool
	if dev, e := s.deviceRepo.GetById(ctx, req.DeviceId); e == nil {
		selected = selectedAgentsSet(dev.Extra)
	}
	for agent, m := range req.Modules {
		if agent != "client" && !selected[agent] {
			continue
		}
		// 1. 状态 upsert（kind：client=App 自身心跳，其余=agent 上报）
		kind := "agent"
		if agent == "client" {
			kind = "app"
		}
		// 命令执行中（该 agent 有 running_command，无论 running/done/failed）→ 强制插新行，
		// 保留 start/每秒 during/end 各自一行在时间线上，不被合并抹平。
		forceInsert := m.RunningCommand != nil
		// 把执行中命令写进 Extra：状态行才能在时间线上标出「卸载中/重启中…」及最后一步日志、
		// 结果/错误。否则上报行 extra={}，和普通心跳无差别，命令过程在 /client/status 不可见。
		// env（openclaw 环境探测：多来源版本/待对齐）同样合进 Extra，随状态行持久化。
		extra := "{}"
		{
			em := map[string]any{}
			if m.RunningCommand != nil {
				rc := m.RunningCommand
				em["commandId"] = rc.CommandId
				em["action"] = rc.Action // install/uninstall/start/stop/restart/...
				em["status"] = rc.Status // running/done/failed
				em["lastLine"] = rc.LastLine
				em["result"] = rc.Result
				em["error"] = rc.Error
				if rc.Percent > 0 {
					em["percent"] = rc.Percent
				}
			}
			if len(m.Env) > 0 {
				em["env"] = m.Env
			}
			if len(em) > 0 {
				if b, e := json.Marshal(em); e == nil {
					extra = string(b)
				}
			}
		}
		_ = s.statusRepo.Upsert(ctx, &entity.ClientAgentStatus{
			DeviceId:  req.DeviceId,
			UserId:    userId,
			Agent:     agent,
			Kind:      kind,
			Status:    m.Status,
			Version:   m.Version,
			LastError: m.LastError,
			Extra:     extra,
		}, forceInsert)
		// 2. 日志批插
		if len(m.Logs) > 0 && s.logRepo != nil {
			cmdId := ""
			if m.RunningCommand != nil {
				cmdId = m.RunningCommand.CommandId
			}
			logs := make([]*entity.ClientLog, 0, len(m.Logs))
			for _, l := range m.Logs {
				var ts *types.Time
				if l.Ts != "" {
					if t, err := time.Parse(time.RFC3339, l.Ts); err == nil {
						tt := types.Time(t)
						ts = &tt
					}
				}
				logs = append(logs, &entity.ClientLog{
					DeviceId:  req.DeviceId,
					Agent:     agent,
					CommandId: cmdId,
					Level:     l.Level,
					Line:      l.Line,
					Ts:        ts,
				})
			}
			_ = s.logRepo.AppendLogs(ctx, logs)
		}
		// 3. 配置变更：hash 变化才落库。基线取「客户端上一次真实上报的快照」
		//（GetLatestReported，跳过后台 app_perm 审计行）——审计行不是客户端文件
		// 状态，当基线用会把保存后首条上报（内容还是旧文件）误记成重复变更。
		if m.ConfigHash != "" && s.configChangeRepo != nil {
			var prev *entity.ClientConfigChange
			if latest, err := s.configChangeRepo.GetLatestReported(ctx, req.DeviceId, agent); err != nil || latest.ConfigHash != m.ConfigHash {
				if err == nil {
					prev = latest // 旧内容用于来源归因的路径差异
				}
				var mtime *types.Time
				if m.ConfigMtime != "" {
					if t, err := time.Parse(time.RFC3339, m.ConfigMtime); err == nil {
						tt := types.Time(t)
						mtime = &tt
					}
				}
				source, sourceDetail := s.attributeConfigChange(ctx, req.DeviceId, agent, prev, m.ConfigContent)
				_ = s.configChangeRepo.Insert(ctx, &entity.ClientConfigChange{
					DeviceId:     req.DeviceId,
					Agent:        agent,
					ConfigHash:   m.ConfigHash,
					ConfigSize:   m.ConfigSize,
					ConfigMtime:  mtime,
					Content:      m.ConfigContent,
					Source:       source,
					SourceDetail: sourceDetail,
				})
			}
		}
		// 4. 命令进度/结果
		if m.RunningCommand != nil && m.RunningCommand.CommandId != "" && s.commandRepo != nil {
			rc := m.RunningCommand
			if rc.Status == "done" || rc.Status == "failed" {
				_ = s.commandRepo.UpdateResult(ctx, rc.CommandId, rc.Status, rc.Result, rc.Error)
				// 链中某步失败 → 取消后续 pending（install→config→start 依次执行，失败即止）
				if rc.Status == "failed" {
					if c, e := s.commandRepo.GetById(ctx, rc.CommandId); e == nil && c.ChainId != "" {
						_ = s.commandRepo.CancelChainPending(ctx, c.ChainId)
					}
				}
			} else {
				// interim（status=running）：把当前步骤行写入 progress，使分步执行在 UI 可见。
				// Rust 执行期每 1s 带 last_line（如「停止: openclaw 已停止（成功）」），但 progress
				// 字段恒空——这里优先用 progress，回退 last_line，构造 jsonb 落库。
				step := rc.Progress
				if step == "" {
					step = rc.LastLine
				}
				if step != "" {
					prog := map[string]any{"step": step}
					if rc.Percent > 0 {
						prog["percent"] = rc.Percent
					}
					if b, err := json.Marshal(prog); err == nil {
						_ = s.commandRepo.UpdateProgress(ctx, rc.CommandId, string(b))
					}
				}
			}
		}
	}
	// 5. 渠道状态上报：合并进设备 channels jsonb（登录态/QR/昵称），落库供管理后台扫码/状态展示。
	if len(req.Channels) > 0 {
		log.Printf("[heartbeat] channels received: dev=%s count=%d", req.DeviceId, len(req.Channels))
		for _, ch := range req.Channels {
			log.Printf("[heartbeat] channel=%s status=%s qrLen=%d", ch.Channel, ch.LoginStatus, len(ch.QrDataUrl))
		}
		if s.deviceRepo != nil {
			s.mergeChannelReports(ctx, req.DeviceId, req.Channels)
		}
	}
	// 6. 技能快照上报：客户端扫描 ~/.openclaw/skills 的真实状态，整体覆盖设备 skills jsonb。
	// nil（旧客户端未带该字段）→ 不动存量；非 nil（含空数组）→ 覆盖。内容没变不写。
	if req.Skills != nil && s.deviceRepo != nil {
		if b, err := json.Marshal(req.Skills); err == nil {
			if dev, e := s.deviceRepo.GetById(ctx, req.DeviceId); e == nil && dev != nil && dev.Skills != string(b) {
				_ = s.deviceRepo.UpdateSkills(ctx, req.DeviceId, string(b))
			}
		}
	}
	// 7. 超时清理：设备有 running 命令超过该 action 的限时 → 标 failed（孤儿/卡死兜底，
	// 释放队列让下一条能认领）。App 非阻塞 channel_login 不受影响（瞬间 done）。
	if s.commandRepo != nil {
		s.cleanupStaleRunning(ctx, req.DeviceId)
	}
	// 8. 认领下一条 pending 命令。设备已有 running 命令时不认领：ClaimNext 返回即标 running，
	// 若上一条还在执行就返回新命令，客户端忙检查会跳过它 → 命令成「已认领未执行」的孤儿，
	// 只能等超时清理判 failed（2026-08-15 解绑双命令实锤：shell 执行中 restart 被认领后从未执行）。
	// 本心跳第 4 步已先落上一条的终态，done/failed 后这里即可认领下一条，流水线不卡。
	var cmd *entity.ClientCommand
	if s.commandRepo != nil {
		if running, e := s.commandRepo.ListRunningByDevice(ctx, req.DeviceId); e == nil && len(running) == 0 {
			cmd, _ = s.commandRepo.ClaimNext(ctx, req.DeviceId)
		}
	}
	commands := []*entity.ClientCommand{}
	if cmd != nil {
		commands = append(commands, cmd)
	}
	// activatedAgents 返回该设备选中的 agent（替代旧的「下过命令的」），App 据此上报
	activated := make([]string, 0, len(selected))
	for k := range selected {
		activated = append(activated, k)
	}
	// App 授权状态快照上报：客户端变更时才带，落 aa_client_app_perm.granted/denied
	//（失败不阻断心跳，客户端下轮会重发——只有状态再变才会带，故这里尽力即可）。
	if req.PermState != nil && s.appPermRepo != nil {
		if err := s.appPermRepo.UpdateStatesByDevice(ctx, req.DeviceId, req.PermState.GrantedPerms, req.PermState.DeniedPerms); err != nil {
			log.Printf("[heartbeat] app_perm state update failed: dev=%s err=%v", req.DeviceId, err)
		}
	}
	// 授权 App 清单：按设备配置下发（每次心跳直查，(device_id,bundle_id) 索引小表）。
	appPerms := s.appPermsForDevice(ctx, req.DeviceId)
	return &HeartbeatRes{
		Stored: true, Commands: commands, ActivatedAgents: activated,
		AppPerms: appPerms,
	}, nil
}

// appPermsForDevice 查设备权限清单并 join 目录取展示名；失败不阻断心跳（返回空清单）。
func (s *ClientService) appPermsForDevice(ctx context.Context, deviceId string) []AppPerm {
	apps := make([]AppPerm, 0)
	if s.appPermRepo == nil || deviceId == "" {
		return apps
	}
	rows, err := s.appPermRepo.ListByDevice(ctx, deviceId)
	if err != nil {
		log.Printf("[heartbeat] app_perm list failed: dev=%s err=%v", deviceId, err)
		return apps
	}
	names := map[string]string{}
	if s.appRepo != nil {
		if catalog, err := s.appRepo.ListAll(ctx); err == nil {
			for _, a := range catalog {
				names[a.BundleId] = a.AppName
			}
		}
	}
	for _, r := range rows {
		apps = append(apps, AppPerm{BundleId: r.BundleId, AppName: names[r.BundleId]})
	}
	return apps
}

// AdminListApps 管理后台：App 目录全量（供勾选与目录管理）。
func (s *ClientService) AdminListApps(ctx context.Context) ([]*entity.ClientApp, error) {
	return s.appRepo.ListAll(ctx)
}

// AdminSaveApp 管理后台：保存目录 App（bundle_id 查重排除自身）。
func (s *ClientService) AdminSaveApp(ctx context.Context, e *entity.ClientApp) error {
	if exist, err := s.appRepo.GetByBundleId(ctx, e.BundleId); err == nil && exist.AppId != e.AppId {
		return fmt.Errorf("bundle id 已存在: %s（%s）", e.BundleId, exist.AppName)
	}
	return s.appRepo.Upsert(ctx, e)
}

// AdminDeleteApp 管理后台：删除目录 App。被设备 App 权限配置引用时拦截（先从设备移除）。
func (s *ClientService) AdminDeleteApp(ctx context.Context, appId string) error {
	list, err := s.appRepo.ListAll(ctx)
	if err != nil {
		return err
	}
	var target *entity.ClientApp
	for _, a := range list {
		if a.AppId == appId {
			target = a
			break
		}
	}
	if target == nil {
		return fmt.Errorf("App 不存在")
	}
	if n, err := s.appRepo.CountAppPermRefs(ctx, target.BundleId); err == nil && n > 0 {
		return fmt.Errorf("该 App 被 %d 台设备的 App 权限配置引用，请先在设备 App 权限配置中移除", n)
	}
	return s.appRepo.Delete(ctx, appId)
}

// AppSuggestItem App 目录录入联想候选（本机已装 App 优先 + Apple iTunes Search 查 Mac App Store）。
type AppSuggestItem struct {
	BundleId   string `json:"bundleId"`
	TrackName  string `json:"trackName"`  // 展示名（本机=App 名；MAS=上架名，供比对确认）
	ArtistName string `json:"artistName"` // 开发者（仅 MAS 候选有）
	ArtworkUrl string `json:"artworkUrl"` // 图标（仅 MAS 候选有；前端候选下拉展示）
	Source     string `json:"source"`     // local=本机已安装 mas=Mac App Store
}

// itunesSearchResult iTunes Search 单条结果（只取用到的字段）。
type itunesSearchResult struct {
	BundleId   string `json:"bundleId"`
	TrackName  string `json:"trackName"`
	ArtistName string `json:"artistName"`
	Artwork512 string `json:"artworkUrl512"`
	Artwork100 string `json:"artworkUrl100"`
}

// itunesSearchResp iTunes Search /search 响应。
type itunesSearchResp struct {
	ResultCount int                  `json:"resultCount"`
	Results     []itunesSearchResult `json:"results"`
}

// AdminSuggestApps 管理后台：按 App 名/关键词联想 Bundle ID（目录录入辅助）。
// 双源合并：① 本机已装 App 扫描（Info.plist 实测值，官网分发不上架 MAS 的 App——Cursor/Warp 等
// 只有这里能查到；api 跑在使用者 Mac 上时命中，prod Linux 自动跳过）；
// ② Apple iTunes Search 公开接口（免 key）查 Mac App Store（entity=macSoftware）。
// 未上架/无网/超时一律降级返回可得部分，不向上抛错。
func (s *ClientService) AdminSuggestApps(ctx context.Context, keyword string) []AppSuggestItem {
	q := strings.TrimSpace(keyword)
	if q == "" {
		return []AppSuggestItem{}
	}
	list := []AppSuggestItem{}
	seen := map[string]bool{}

	// ① 本机已装 App（名字或 bundleId 子串命中，大小写不敏感）
	for _, a := range matchLocalApps(q) {
		if !seen[a.bundleId] {
			seen[a.bundleId] = true
			list = append(list, AppSuggestItem{BundleId: a.bundleId, TrackName: a.name, Source: "local"})
		}
	}

	// ② Mac App Store（iTunes Search）
	itunes := s.searchItunes(ctx, q)
	for _, it := range itunes {
		if it.BundleId == "" || seen[it.BundleId] {
			continue
		}
		seen[it.BundleId] = true
		artwork := it.Artwork512
		if artwork == "" {
			artwork = it.Artwork100
		}
		list = append(list, AppSuggestItem{
			BundleId:   it.BundleId,
			TrackName:  it.TrackName,
			ArtistName: it.ArtistName,
			ArtworkUrl: artwork,
			Source:     "mas",
		})
	}
	return list
}

// searchItunes 调 iTunes Search 查 Mac App Store；失败打日志返回空（静默降级）。
func (s *ClientService) searchItunes(ctx context.Context, q string) []itunesSearchResult {
	u := "https://itunes.apple.com/search?term=" + url.QueryEscape(q) + "&entity=macSoftware&limit=8"
	httpClient := &http.Client{Timeout: 4 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "aiarsenal-app-suggest/1.0")
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("[client] app suggest 请求失败 keyword=%s: %v", q, err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("[client] app suggest HTTP %d keyword=%s", resp.StatusCode, q)
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil
	}
	var r itunesSearchResp
	if err := json.Unmarshal(body, &r); err != nil {
		log.Printf("[client] app suggest 解析失败 keyword=%s: %v", q, err)
		return nil
	}
	return r.Results
}

// ===== 本机已装 App 扫描（仅 darwin；结果缓存 10 分钟） =====

type localAppInfo struct {
	bundleId string
	name     string
}

var localAppsCache struct {
	sync.Mutex
	apps []localAppInfo
	at   time.Time
}

// scanLocalApps 枚举常见安装位置的全部 .app，plutil 读 Info.plist 取 bundleId + App 名。
// 结果缓存 10 分钟；扫描在锁外 16 路并发（~200 个 .app 各 fork 一次 plutil，串行要数秒），
// 并发首次调用可能重复扫（幂等无害）。目录缺失/读取失败的单项跳过。
func scanLocalApps() []localAppInfo {
	if runtime.GOOS != "darwin" {
		return nil
	}
	localAppsCache.Lock()
	if localAppsCache.apps != nil && time.Since(localAppsCache.at) < 10*time.Minute {
		apps := localAppsCache.apps
		localAppsCache.Unlock()
		return apps
	}
	localAppsCache.Unlock()

	dirs := []string{
		"/Applications", "/Applications/Utilities",
		"/System/Applications", "/System/Applications/Utilities",
		"/System/Library/CoreServices",
		filepath.Join(os.Getenv("HOME"), "Applications"),
	}
	var paths []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".app") {
				paths = append(paths, filepath.Join(dir, e.Name()))
			}
		}
	}
	sem := make(chan struct{}, 16)
	var mu sync.Mutex
	var wg sync.WaitGroup
	apps := make([]localAppInfo, 0, len(paths))
	seen := map[string]bool{}
	for _, p := range paths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			info := readAppBundle(p)
			if info.bundleId == "" {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if !seen[info.bundleId] {
				seen[info.bundleId] = true
				apps = append(apps, info)
			}
		}(p)
	}
	wg.Wait()

	localAppsCache.Lock()
	localAppsCache.apps = apps
	localAppsCache.at = time.Now()
	localAppsCache.Unlock()
	return apps
}

// readAppBundle 读单个 .app 的 Info.plist（plutil 转 json），失败返回零值。
func readAppBundle(appPath string) localAppInfo {
	var zero localAppInfo
	out, err := exec.Command("/usr/bin/plutil", "-convert", "json", "-o", "-", filepath.Join(appPath, "Contents", "Info.plist")).Output()
	if err != nil {
		return zero
	}
	var pl struct {
		BundleID    string `json:"CFBundleIdentifier"`
		BundleName  string `json:"CFBundleName"`
		DisplayName string `json:"CFBundleDisplayName"`
		Executable  string `json:"CFBundleExecutable"`
	}
	if err := json.Unmarshal(out, &pl); err != nil {
		return zero
	}
	name := pl.DisplayName
	if name == "" {
		name = pl.BundleName
	}
	if name == "" {
		name = pl.Executable
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(appPath), ".app")
	}
	return localAppInfo{bundleId: pl.BundleID, name: name}
}

// matchLocalApps 按关键词过滤本机 App（App 名或 bundleId 子串，大小写不敏感；上限 8 条）。
func matchLocalApps(keyword string) []localAppInfo {
	apps := scanLocalApps()
	k := strings.ToLower(keyword)
	if k == "" {
		return nil
	}
	list := make([]localAppInfo, 0, 8)
	for _, a := range apps {
		if strings.Contains(strings.ToLower(a.name), k) || strings.Contains(strings.ToLower(a.bundleId), k) {
			list = append(list, a)
			if len(list) >= 8 {
				break
			}
		}
	}
	return list
}

// DeviceAppPermInfo 设备 App 权限行（含客户端上报的授权状态）。
type DeviceAppPermInfo struct {
	BundleId string `json:"bundleId"`
	Granted  bool   `json:"granted"` // 已授权（探测成功/事件已送达）
	Denied   bool   `json:"denied"`  // 已拒绝（TCC 缓存拒绝 -1743，需重置或系统设置开回）
}

// AdminGetDeviceAppPerms 管理后台：某设备勾选中的授权 App 列表（含授权状态徽标数据）。
func (s *ClientService) AdminGetDeviceAppPerms(ctx context.Context, deviceId string) ([]DeviceAppPermInfo, error) {
	rows, err := s.appPermRepo.ListByDevice(ctx, deviceId)
	if err != nil {
		return nil, err
	}
	out := make([]DeviceAppPermInfo, 0, len(rows))
	for _, r := range rows {
		out = append(out, DeviceAppPermInfo{BundleId: r.BundleId, Granted: r.Granted, Denied: r.Denied})
	}
	return out, nil
}

// AdminResetDeviceAppPerm 管理后台：重置某设备的 App 自动化授权。
// 下发 appperm_reset 指令：客户端 tccutil 清本 App 全部 AppleEvents 授权对（无按目标
// 粒度）+ 清状态 + 立即唤醒引擎——清单内所有运行中 App 的授权框重新弹一遍。
func (s *ClientService) AdminResetDeviceAppPerm(ctx context.Context, deviceId, createdBy string) (string, error) {
	if _, err := s.deviceRepo.GetById(ctx, deviceId); err != nil {
		return "", fmt.Errorf("设备不存在: %w", err)
	}
	return s.AdminEnqueueCommand(ctx, deviceId, "appperm_reset", "client", "", json.RawMessage("{}"), createdBy, "")
}

// AdminSetDeviceAppPerms 管理后台：整组覆盖某设备的 App 权限清单。
// 校验 bundleId 均在目录内；写 aa_client_config_change（source=app_perm）留变更审计。
func (s *ClientService) AdminSetDeviceAppPerms(ctx context.Context, deviceId string, bundleIds []string) error {
	dev, err := s.deviceRepo.GetById(ctx, deviceId)
	if err != nil {
		return fmt.Errorf("设备不存在: %w", err)
	}
	// App 权限清单仅约束 openclaw 桌面控制链路（node 协议与 aa-desktop MCP 都过客户端
	// app_gate）；hermes 自带 terminal/write_file 等工具不经闸门、清单约束不了它——
	// 未启用 openclaw 的设备拒绝保存（与前端置灰同口径；启用 openclaw 后恢复）。
	hasOpenclaw := false
	for _, a := range selectedAgentsFromExtra(dev.Extra) {
		if a == "openclaw" {
			hasOpenclaw = true
			break
		}
	}
	if !hasOpenclaw {
		return fmt.Errorf("App 权限管理仅对启用 openclaw 的设备生效：hermes 使用自带终端工具操作桌面，不受此清单约束")
	}
	// 去重去空 + 目录校验
	catalog, err := s.appRepo.ListAll(ctx)
	if err != nil {
		return err
	}
	known := map[string]string{}
	for _, a := range catalog {
		known[a.BundleId] = a.AppName
	}
	seen := map[string]bool{}
	clean := make([]string, 0, len(bundleIds))
	for _, b := range bundleIds {
		if b == "" || seen[b] {
			continue
		}
		if _, ok := known[b]; !ok {
			return fmt.Errorf("bundle id 不在 App 目录: %s", b)
		}
		seen[b] = true
		clean = append(clean, b)
	}
	// 旧清单算 diff
	oldRows, _ := s.appPermRepo.ListByDevice(ctx, deviceId)
	oldSet := map[string]bool{}
	for _, r := range oldRows {
		oldSet[r.BundleId] = true
	}
	var added, removed []string
	for _, b := range clean {
		if !oldSet[b] {
			added = append(added, known[b]+"（"+b+"）")
		}
	}
	newSet := map[string]bool{}
	for _, b := range clean {
		newSet[b] = true
	}
	for _, r := range oldRows {
		if !newSet[r.BundleId] {
			name := r.BundleId
			if n, ok := known[r.BundleId]; ok {
				name = n + "（" + r.BundleId + "）"
			}
			removed = append(removed, name)
		}
	}
	if err := s.appPermRepo.ReplaceDevice(ctx, deviceId, clean); err != nil {
		return err
	}
	// 变更记录（source=app_perm；agent=client 不与 openclaw/hermes/obsidian 配置预览取数冲突）
	// content 与 client 端 aa.json 完全同构：app_perms（本次下发清单）+ granted_perms/
	// denied_perms（保存后库里的授权状态快照）——只记清单会让人误以为下发丢了授权状态键。
	auditPerms := make([]AppPerm, 0, len(clean))
	for _, b := range clean {
		auditPerms = append(auditPerms, AppPerm{BundleId: b, AppName: known[b]})
	}
	grantedList := make([]string, 0)
	deniedList := make([]string, 0)
	if savedRows, err := s.appPermRepo.ListByDevice(ctx, deviceId); err == nil {
		for _, r := range savedRows {
			if r.Granted {
				grantedList = append(grantedList, r.BundleId)
			}
			if r.Denied {
				deniedList = append(deniedList, r.BundleId)
			}
		}
	}
	content, _ := json.Marshal(map[string]interface{}{
		"app_perms":     auditPerms,
		"granted_perms": grantedList,
		"denied_perms":  deniedList,
	})
	detail := "App权限配置"
	if len(added) > 0 {
		detail += "：" + "+" + strings.Join(added, "、")
	}
	if len(removed) > 0 {
		detail += "：" + "-" + strings.Join(removed, "、")
	}
	if len(added) == 0 && len(removed) == 0 {
		detail += "（无变化）"
	}
	sum := sha256.Sum256(content)
	_ = s.configChangeRepo.Insert(ctx, &entity.ClientConfigChange{
		DeviceId:     deviceId,
		Agent:        "client",
		ConfigHash:   hex.EncodeToString(sum[:]),
		ConfigSize:   int64(len(content)),
		Content:      string(content),
		Source:       "app_perm",
		SourceDetail: detail,
	})
	return nil
}

// AdminListLogs 管理后台：某设备某客户端最近 N 行日志。
func (s *ClientService) AdminListLogs(ctx context.Context, deviceId, agent string, limit int) ([]*entity.ClientLog, error) {
	return s.logRepo.ListByDevice(ctx, deviceId, agent, limit)
}

// AdminListCommandLogs 某条命令的分步日志（命令历史展开看每一步执行）。
func (s *ClientService) AdminListCommandLogs(ctx context.Context, commandId string, limit int) ([]*entity.ClientLog, error) {
	return s.logRepo.ListByCommand(ctx, commandId, limit)
}

// AdminListConfigChanges 管理后台：某设备配置变更历史。
func (s *ClientService) AdminListConfigChanges(ctx context.Context, deviceId string, limit int) ([]*entity.ClientConfigChange, error) {
	return s.configChangeRepo.ListByDevice(ctx, deviceId, limit)
}

// AdminPreviewOpenclawConfig 取该设备 openclaw 最新一条 config_change 内容（含真实 apiKey 等
// 全量字段），把 models.providers.aiarsenal.models 替换成 aa_llm_model 当前活跃集，2 空格美化后
// 返回。用于后台「手动下发配置」输入框预填——展示「库里存的配置主体 + 代码侧最新可用模型」，
// 而非下发当时 /v1/models 没拉到时落库的兜底 default 模型。库中无配置则返回空串。
func (s *ClientService) AdminPreviewOpenclawConfig(ctx context.Context, deviceId string) (string, error) {
	latest, err := s.configChangeRepo.GetLatest(ctx, deviceId, "openclaw")
	if err != nil {
		log.Printf("[openclaw-config] preview get latest: %v", err)
		return "", nil
	}
	if latest == nil || latest.Content == "" {
		return "", nil
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(latest.Content), &cfg); err != nil {
		log.Printf("[openclaw-config] preview parse stored content: %v", err)
		return "", nil
	}
	// 导航到 models.providers.aiarsenal.models，任一层缺则补建。
	modelsField, ok := cfg["models"].(map[string]interface{})
	if !ok {
		modelsField = map[string]interface{}{"mode": "merge", "providers": map[string]interface{}{}}
		cfg["models"] = modelsField
	}
	providers, ok := modelsField["providers"].(map[string]interface{})
	if !ok {
		providers = map[string]interface{}{}
		modelsField["providers"] = providers
	}
	aiarsenal, ok := providers["aiarsenal"].(map[string]interface{})
	if !ok {
		aiarsenal = map[string]interface{}{}
		providers["aiarsenal"] = aiarsenal
	}
	aiarsenal["models"] = s.buildOpenclawModelsFromDB(ctx)
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// buildOpenclawModelsFromDB 从 aa_llm_model 取活跃部署（deleted_at null + blocked=0 + status=1），
// 映射成 openclaw.json models.providers.aiarsenal.models 数组项，字段与 fetchLLMModels 一致。
// 只读 DB 不调 /v1/models——预览要的是「当前最新可用模型」，不受网关临时不可达影响；空表回退单条 default。
func (s *ClientService) buildOpenclawModelsFromDB(ctx context.Context) []interface{} {
	fallback := []interface{}{
		map[string]interface{}{"id": "default", "name": "AI Arsenal Default"},
	}
	if s.llmModelRepo == nil {
		return fallback
	}
	rows, err := s.llmModelRepo.All(ctx)
	if err != nil {
		log.Printf("[openclaw-config] preview load aa_llm_model: %v", err)
		return fallback
	}
	// 按 id 去重：池入口（裸 group）取组内代表模型；具体 group/model 取精确部署。
	// 保留首次命中顺序，与 /v1/models 形态一致。
	type cell struct {
		m         *entity.LLMModel
		composite bool
	}
	order := []string{}
	cells := map[string]cell{}
	for _, m := range rows {
		if m == nil || m.Status != 1 || m.ModelGroup == "" {
			continue
		}
		if _, ok := cells[m.ModelGroup]; !ok {
			cells[m.ModelGroup] = cell{m, false}
			order = append(order, m.ModelGroup)
		}
		if m.ModelName != "" {
			composite := m.ModelGroup + "/" + m.ModelName
			if _, ok := cells[composite]; !ok {
				cells[composite] = cell{m, true}
				order = append(order, composite)
			}
		}
	}
	models := make([]interface{}, 0, len(order))
	for _, id := range order {
		c := cells[id]
		m := c.m
		name := id // 池入口 name=id；具体 name=model_name
		if c.composite && m.ModelName != "" {
			name = m.ModelName
		}
		item := map[string]interface{}{
			"id":        id,
			"name":      name,
			"reasoning": false,
			"input":     []string{"text"},
			// 对外保持 $/token 语义（消费方兼容）：毫/百万 tokens → $/token = 毫 / 1e10。
			"cost": map[string]interface{}{
				"input":      float64(m.InputPricePerMillion) / 1e10,
				"output":     float64(m.OutputPricePerMillion) / 1e10,
				"cacheRead":  float64(m.CacheReadPricePerMillion) / 1e10,
				"cacheWrite": float64(m.CacheCreationPricePerMillion) / 1e10,
			},
		}
		if m.MaxInputTokens > 0 {
			item["contextWindow"] = m.MaxInputTokens
		}
		if m.MaxOutputTokens > 0 {
			item["maxTokens"] = m.MaxOutputTokens
		}
		models = append(models, item)
	}
	if len(models) == 0 {
		return fallback
	}
	return models
}

// configWriteActions 会写配置文件的后台命令：config_push/install/restart/start 的
// payload.configContent 由 App 先落盘再执行动作；plugin_install 合并 channels 进 openclaw.json；
// appperm_reset 清 aa.json 的 granted/denied 键（agent=client 的配置文件）。
var configWriteActions = map[string]bool{
	"config_push":    true,
	"install":        true,
	"restart":        true,
	"start":          true,
	"plugin_install": true,
	"appperm_reset":  true,
}

// attributeConfigChange 配置变更来源归因（心跳发现 hash 变化、落库前判定）：
//  1. JSON 叶子路径差异仅落在 models.* → llm_model（LLM 可用模型清单变化触发，
//     后台签发新 key 下发或客户端重渲染模型清单都是这个形态）；
//  2. 关联到最近 15 分钟内完成的写配置命令 → admin_<action>（带操作人/命令 id）；
//     2.5 agent=client 且最近 15 分钟内有后台 App 权限保存 → app_perm（下发清单的生效回执）；
//  3. 否则 local_change（无命令关联，本机 openclaw / 客户端自行写入）。
//
// prev 为 nil 表示首个快照（无旧内容可比对），优先命令归因。
func (s *ClientService) attributeConfigChange(ctx context.Context, deviceId, agent string, prev *entity.ClientConfigChange, newContent string) (string, string) {
	oldContent := ""
	if prev != nil {
		oldContent = prev.Content
	}
	onlyModels, addCnt, delCnt := diffOnlyModels(oldContent, newContent)

	// 关联最近完成的写配置命令（ListByDevice 按 created_at 倒序，取首条命中）
	var cmd *entity.ClientCommand
	if s.commandRepo != nil {
		if cmds, err := s.commandRepo.ListByDevice(ctx, deviceId, 30); err == nil {
			cutoff := time.Now().Add(-15 * time.Minute)
			for i := range cmds {
				c := cmds[i]
				if c.Agent != agent || !configWriteActions[c.Action] || c.Status != "done" {
					continue
				}
				if c.FinishedAt == nil || time.Time(*c.FinishedAt).Before(cutoff) {
					continue
				}
				cmd = c
				break
			}
		}
	}

	if onlyModels {
		d := fmt.Sprintf("差异仅为模型清单（models.* 新增/变化 %d · 移除 %d，按叶子路径计）", addCnt, delCnt)
		if cmd != nil {
			return "llm_model", d + fmt.Sprintf("；由后台命令下发（%s · 操作人 %s · 命令 %s）", cmd.Action, cmd.CreatedBy, cmd.CommandId)
		}
		return "llm_model", d
	}
	if cmd != nil {
		return "admin_" + cmd.Action, fmt.Sprintf("后台命令写入（%s · 操作人 %s · 命令 %s）", cmd.Action, cmd.CreatedBy, cmd.CommandId)
	}
	// 后台 App 权限保存（非命令、走心跳下发）：客户端应用新清单后的首个上报归因到它，
	// 否则会显示成「本机自行写入」——实际它是后台下发的生效回执。
	// 注意 source 必须与审计行（app_perm）区分：GetLatestReported 按 source<>'app_perm'
	// 选基线，回执行若也叫 app_perm 会永远进不了基线 → 每次心跳都插一条（曾踩）。
	if s.configChangeRepo != nil && agent == "client" {
		if ap, err := s.configChangeRepo.GetLatestBySource(ctx, deviceId, agent, "app_perm"); err == nil && ap != nil && ap.ChangedAt != nil {
			if !time.Time(*ap.ChangedAt).Before(time.Now().Add(-15 * time.Minute)) {
				return "app_perm_applied", "后台下发 App 权限清单，客户端已应用（" + ap.SourceDetail + "）"
			}
		}
	}
	if prev == nil {
		return "local_change", "首次配置快照（无历史版本可比对）"
	}
	return "local_change", "无后台命令关联（近 15 分钟），本机 openclaw / 客户端自行写入"
}

// diffOnlyModels 比较新旧配置 JSON 的叶子路径差异，返回：差异是否仅落在 models.*、
// models 下新增/移除（含值变化）的叶子数。任一侧不是合法 JSON 时无法判定，返回 false。
func diffOnlyModels(oldContent, newContent string) (bool, int, int) {
	var oldV, newV any
	if err := json.Unmarshal([]byte(oldContent), &oldV); err != nil {
		return false, 0, 0
	}
	if err := json.Unmarshal([]byte(newContent), &newV); err != nil {
		return false, 0, 0
	}
	oldFlat, newFlat := map[string]string{}, map[string]string{}
	flattenJSON(oldV, "", oldFlat)
	flattenJSON(newV, "", newFlat)

	onlyModels, add, del := true, 0, 0
	for k, v := range newFlat {
		if ov, ok := oldFlat[k]; !ok || ov != v {
			if strings.HasPrefix(k, "models.") || k == "models" {
				add++
			} else {
				onlyModels = false
			}
		}
	}
	for k := range oldFlat {
		if _, ok := newFlat[k]; !ok {
			if strings.HasPrefix(k, "models.") || k == "models" {
				del++
			} else {
				onlyModels = false
			}
		}
	}
	return onlyModels && add+del > 0, add, del
}

// flattenJSON 把 JSON 展平成「叶子路径 → 字符串值」（数组按下标展开，空对象/数组视为叶子）。
func flattenJSON(v any, prefix string, out map[string]string) {
	switch t := v.(type) {
	case map[string]any:
		if len(t) == 0 {
			out[prefix] = "{}"
			return
		}
		for k, cv := range t {
			p := k
			if prefix != "" {
				p = prefix + "." + k
			}
			flattenJSON(cv, p, out)
		}
	case []any:
		if len(t) == 0 {
			out[prefix] = "[]"
			return
		}
		for i, cv := range t {
			p := fmt.Sprintf("%s.%d", prefix, i)
			flattenJSON(cv, p, out)
		}
	default:
		out[prefix] = fmt.Sprintf("%v", v)
	}
}
