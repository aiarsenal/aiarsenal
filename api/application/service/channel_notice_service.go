package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"aiarsenal/domain/repository"
)

// 双通道通知：同一个消息、两条投递路径——站内 aa_notice（右上角铃铛，必达路径）
// + channel_notify 命令下发给收件人名下 hermes 实例（数字人经自己的渠道把「给人看的」
// 通知文本发到与人对话的会话里；数字人只是信使，不是消息的消费方）。
// 消息类型各业务自带（临时预算审批结果、技能安全扫描结论、…以后新增的通知类型），
// 实现统一收口在这里，别处不要再各写一份。
// 渠道推送 fail-open：无实例/下发失败只记日志，不影响站内通知与业务主流程。

const (
	channelNotifyAgent = "hermes"
	channelNotifyUser  = "0" // 平台系统身份签发

	// skillChannelAggWindow 技能装卸渠道推送的聚合窗口：绑定 MCP 分组会自动配套装一批技能，
	// 命令逐条完成逐条回调——不聚合就在微信端 N 连发轰炸（6 连发踩过）。窗口取 8s：
	// fan-out 的命令通常几秒内先后 done，窗口过大手动装卸单条也要白等。
	skillChannelAggWindow = 8 * time.Second
)

// ChannelNoticeService 双通道通知的依赖聚合。runtime/notice 在两个 main 里装配次序
// 不一（扫描器先于 runtime 构建），故支持装配尾期经 SetChannelNotice 补注。
type ChannelNoticeService struct {
	noticeSvc    *NoticeService
	runtimeSvc   *AgentRuntimeService
	instanceRepo repository.IAgentInstanceRepository
	skillAgg     *skillChangeAggregator // 技能装卸渠道推送聚合（懒建，见 PushSkillChange）
	skillAggMu   sync.Mutex
}

func NewChannelNoticeService(n *NoticeService, rt *AgentRuntimeService, i repository.IAgentInstanceRepository) *ChannelNoticeService {
	return &ChannelNoticeService{noticeSvc: n, runtimeSvc: rt, instanceRepo: i}
}

// Push 双通道发一条通知（nil receiver 安全：未装配=直接跳过）。
func (s *ChannelNoticeService) Push(ctx context.Context, userId, title, content, link, ntype, createdBy string) {
	if s == nil {
		return
	}
	pushChannelNotice(ctx, s.noticeSvc, s.runtimeSvc, s.instanceRepo, userId, title, content, link, ntype, createdBy)
}

// PushSkillChange 技能装卸专用双通道：站内铃铛逐条即时（铃铛多条无妨），渠道推送经
// skillChangeAggregator 短窗聚合成一条（微信端免 N 连发）。收件人=实例归属人。
func (s *ChannelNoticeService) PushSkillChange(ctx context.Context, userId, instanceId, instanceName, skillName string, installed bool) {
	if s == nil {
		return
	}
	title, content := skillChangeText([]string{skillName}, instanceName, installed)
	if s.noticeSvc != nil {
		if err := s.noticeSvc.Push(ctx, userId, title, content, "/agent/instance/list", "info", "skill-change"); err != nil {
			slog.Warn("站内通知发送失败", "user", userId, "err", err)
		}
	}
	s.skillAggMu.Lock()
	if s.skillAgg == nil {
		s.skillAgg = newSkillChangeAggregator(skillChannelAggWindow, s.flushSkillChangeChannel)
	}
	s.skillAggMu.Unlock()
	s.skillAgg.add(userId, instanceId, instanceName, skillName, installed)
}

// flushSkillChangeChannel 聚合窗口到点回调（不在原请求 ctx 里，用 Background）：
// 把窗口内攒下的技能名单合并成一条 channel_notify。
func (s *ChannelNoticeService) flushSkillChangeChannel(userId, instanceId, instanceName string, installed bool, names []string) {
	title, content := skillChangeText(names, instanceName, installed)
	pushChannelOnly(context.Background(), s.runtimeSvc, s.instanceRepo, userId, title, content, "/agent/instance/list")
	slog.Info("技能装卸渠道通知已聚合下发", "user", userId, "instance", instanceId, "installed", installed, "skills", strings.Join(names, ","))
}

// skillChangeText 技能装卸通知文案：单条沿用「技能「a」…」句式；多条合并点名（渠道聚合后
// 一条说清，站内单条永远走单条句式）。
func skillChangeText(names []string, instanceName string, installed bool) (title, content string) {
	uniq := dedupeSkillNames(names)
	if installed {
		title = "技能已安装"
		if len(uniq) == 1 {
			content = fmt.Sprintf("技能「%s」已安装到实例「%s」。", uniq[0], instanceName)
		} else {
			content = fmt.Sprintf("%d 个技能已安装到实例「%s」：%s。", len(uniq), instanceName, strings.Join(uniq, "、"))
		}
		return title, content
	}
	title = "技能已卸载"
	if len(uniq) == 1 {
		content = fmt.Sprintf("技能「%s」已从实例「%s」卸载。", uniq[0], instanceName)
	} else {
		content = fmt.Sprintf("%d 个技能已从实例「%s」卸载：%s。", len(uniq), instanceName, strings.Join(uniq, "、"))
	}
	return title, content
}

// dedupeSkillNames 技能名去重去空（保持出现顺序）。刻意不复用包内其它 WIP 的同名工具：
// 那些文件未入库，干净 checkout 会编译失败。
func dedupeSkillNames(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// pendingSkillNotice 一个聚合键（用户+实例+装/卸）窗口内攒的技能名单。
type pendingSkillNotice struct {
	instanceName string
	installed    bool
	names        []string
	timer        *time.Timer
}

// skillChangeAggregator 技能装卸渠道推送短窗聚合器：同键首个通知起一个窗口计时器，
// 窗口内后续通知只攒名单；到点回调 flush 一次性发合并文案。进程内状态，重启即丢
// （fail-open：渠道本就允许丢，站内通知不受影响）。
type skillChangeAggregator struct {
	mu     sync.Mutex
	window time.Duration
	pend   map[string]*pendingSkillNotice
	flush  func(userId, instanceId, instanceName string, installed bool, names []string)
}

func newSkillChangeAggregator(window time.Duration, flush func(userId, instanceId, instanceName string, installed bool, names []string)) *skillChangeAggregator {
	return &skillChangeAggregator{window: window, pend: map[string]*pendingSkillNotice{}, flush: flush}
}

func (a *skillChangeAggregator) key(userId, instanceId string, installed bool) string {
	return userId + "\x00" + instanceId + "\x00" + fmt.Sprintf("%t", installed)
}

// add 记一笔技能装卸；同键首个起计时器，到点 flush 携带全部名单。
func (a *skillChangeAggregator) add(userId, instanceId, instanceName, skillName string, installed bool) {
	k := a.key(userId, instanceId, installed)
	a.mu.Lock()
	defer a.mu.Unlock()
	p, ok := a.pend[k]
	if !ok {
		p = &pendingSkillNotice{instanceName: instanceName, installed: installed}
		a.pend[k] = p
		p.timer = time.AfterFunc(a.window, func() {
			a.mu.Lock()
			names := append([]string(nil), p.names...)
			delete(a.pend, k)
			a.mu.Unlock()
			a.flush(userId, instanceId, p.instanceName, installed, names)
		})
	}
	p.names = append(p.names, skillName)
}

// pushChannelNotice 双通道实现：先站内 aa_notice（必达，失败仅记日志），
// 再把同一消息（title/content/link 原文）channel_notify 下发收件人名下第一个
// hermes 实例。payload_json 恒为非空 JSON 对象。
func pushChannelNotice(ctx context.Context, noticeSvc *NoticeService, runtimeSvc *AgentRuntimeService,
	instanceRepo repository.IAgentInstanceRepository, userId, title, content, link, ntype, createdBy string) {
	if userId == "" {
		return
	}
	if noticeSvc != nil {
		if err := noticeSvc.Push(ctx, userId, title, content, link, ntype, createdBy); err != nil {
			slog.Warn("站内通知发送失败", "user", userId, "err", err)
		}
	}
	pushChannelOnly(ctx, runtimeSvc, instanceRepo, userId, title, content, link)
}

// pushChannelOnly 双通道里的渠道半边：channel_notify 下发收件人名下第一个 hermes 实例。
// 供 pushChannelNotice 与聚合到点的 flushSkillChangeChannel 复用（后者已无原请求 ctx）。
func pushChannelOnly(ctx context.Context, runtimeSvc *AgentRuntimeService,
	instanceRepo repository.IAgentInstanceRepository, userId, title, content, link string) {
	if userId == "" || runtimeSvc == nil || instanceRepo == nil {
		return
	}
	insts, err := instanceRepo.ListByUserAgent(ctx, userId, channelNotifyAgent)
	if err != nil || len(insts) == 0 {
		if err != nil {
			slog.Warn("渠道推送定位 hermes 实例失败", "user", userId, "err", err)
		}
		return
	}
	payload, _ := json.Marshal(map[string]string{"title": title, "content": content, "link": link})
	if _, err := runtimeSvc.EnqueueCommand(ctx, insts[0].InstanceId, "channel_notify", channelNotifyAgent, channelNotifyUser, payload, 120); err != nil {
		slog.Warn("channel_notify 下发失败（站内通知不受影响）", "user", userId, "instanceId", insts[0].InstanceId, "err", err)
	}
}
