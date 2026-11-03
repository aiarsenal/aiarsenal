package alert

import (
	"fmt"
	"net/url"
	"strings"

	larksvc "github.com/nicholas-fedor/shoutrrr/pkg/services/chat/lark"
	wecomsvc "github.com/nicholas-fedor/shoutrrr/pkg/services/chat/wecom"
	smtpsvc "github.com/nicholas-fedor/shoutrrr/pkg/services/email/smtp"
	genericsvc "github.com/nicholas-fedor/shoutrrr/pkg/services/specialized/generic"
	stypes "github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// shoutrrr.go — shoutrrr 通道接线（nicholas-fedor/shoutrrr v0.22.1，MIT；供应链审计结论
// 「有条件引入」，条件逐条落在本文件与 go.mod：① pin v0.22.1+go.sum；② 绕过 pkg/router
// ——只 import 用到的 4 个 service 包（lark/wecom/smtp/generic），不触发 servicemap 全量
// 静态注册；③ config 模板不开启 SkipTLSVerify 类选项；④ bus factor=1 与「fork 无第三方
// 安全评估」已在 PR 正文披露）。

// shoutrrrSender shoutrrr 各 service 的公共形状（= stypes.Service 的子集）。
type shoutrrrSender interface {
	Initialize(serviceURL *url.URL, logger stypes.StdLogger) error
	Send(message string, params *stypes.Params) error
}

// senderByScheme 配置 URL scheme → service 构造。白名单外 scheme 明确报错——
// 不引入 pkg/router 的 servicemap 全表，新通道要在这里显式登记（显式调用点原则）。
func senderByScheme(scheme string) (shoutrrrSender, error) {
	switch scheme {
	case "lark":
		return &larksvc.Service{}, nil
	case "wecom":
		return &wecomsvc.Service{}, nil
	case "smtp":
		return &smtpsvc.Service{}, nil
	case "generic":
		return &genericsvc.Service{}, nil
	default:
		return nil, fmt.Errorf("shoutrrr: 不支持的通道 scheme %q（白名单 lark/wecom/smtp/generic）", scheme)
	}
}

// SchemeSupported 通道 scheme 是否在白名单内（config.Load 预校验用，单一真相源）。
func SchemeSupported(scheme string) bool {
	_, err := senderByScheme(strings.ToLower(scheme))
	return err == nil
}

// sendShoutrrr 解析单条通道 URL 并同步投递。构造与 Initialize 每次重建——
// 告警频率低（节流窗兜底），不值得为连接复用引入每通道长生命周期状态；
// 重试由 Dispatcher.deliver 统一负责，这里只做一次诚实投递。
func sendShoutrrr(rawURL, message string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("shoutrrr: parse url: %w", err)
	}
	sender, err := senderByScheme(strings.ToLower(u.Scheme))
	if err != nil {
		return err
	}
	if err := sender.Initialize(u, discardLogger{}); err != nil {
		return fmt.Errorf("shoutrrr: initialize %s: %w", u.Scheme, err)
	}
	if err := sender.Send(message, nil); err != nil {
		return fmt.Errorf("shoutrrr: send %s: %w", u.Scheme, err)
	}
	return nil
}

// discardLogger stypes.StdLogger 空实现（通道内部噪音不进主日志；错误经 Send 返回值上浮）。
type discardLogger struct{}

func (discardLogger) Print(...any)          {}
func (discardLogger) Printf(string, ...any) {}
func (discardLogger) Println(...any)        {}
