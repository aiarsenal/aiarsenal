package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	storagestg "github.com/aiarsenal/runtime-go/storage"
)

// ScannerService 规则式安全扫描（Go 内置实现）。
// 扫描某版本的所有文件，回写 findings 到 aa_skill_security_audit；
// 落终态（通过/不安全/扫描器出错）后给技能归属人发双通道通知
// （站内铃铛 + hermes 数字人转达同一消息，见 channel_notice_service.go）。
type ScannerService struct {
	repo          repository.ISkillRepository
	secRepo       repository.ISkillSecurityRepository
	storage       storagestg.Storage
	channelNotice *ChannelNoticeService
}

func NewScannerService(r repository.ISkillRepository, sr repository.ISkillSecurityRepository, s storagestg.Storage) *ScannerService {
	return &ScannerService{repo: r, secRepo: sr, storage: s}
}

// SetChannelNotice 装配期注入双通道通知（两个 main 里 runtime/notice 装配在 scanner
// 之后，构造期给不了，装配尾期补注）；nil/未注入=不发通知，扫描结论不受影响。
func (s *ScannerService) SetChannelNotice(c *ChannelNoticeService) { s.channelNotice = c }

type finding struct {
	File     string `json:"file"`
	Rule     string `json:"rule"`
	Severity uint   `json:"severity"` // 0~4
	Message  string `json:"message"`
}

const (
	severityInfo     = 0
	severityLow      = 1
	severityMedium   = 2
	severityHigh     = 3
	severityCritical = 4
)

var (
	bannedExt = map[string]bool{
		".exe": true, ".dll": true, ".so": true, ".dylib": true, ".sh": true,
		".bat": true, ".cmd": true, ".ps1": true, ".vbs": true, ".msi": true,
		".jar": true, ".class": true, ".pyc": true, ".bin": true, ".scr": true,
	}
	patterns = []struct {
		rule string
		sev  uint
		re   *regexp.Regexp
	}{
		{"aws_key", severityHigh, regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
		{"github_token", severityHigh, regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{36,}`)},
		{"google_api_key", severityHigh, regexp.MustCompile(`AIza[0-9A-Za-z\-]{35}`)},
		{"slack_token", severityHigh, regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`)},
		{"private_key", severityHigh, regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
		{"eval_call", severityMedium, regexp.MustCompile(`\beval\s*\(`)},
		{"exec_call", severityMedium, regexp.MustCompile(`\bexec\s*\(`)},
		{"shell_exec", severityMedium, regexp.MustCompile(`\b(?:os\.system|subprocess\.(?:call|run|Popen)|child_process\.exec)\s*\(`)},
	}
	maxFileSize = int64(50 * 1024 * 1024) // 50MB
	scanTextMax = int64(1 * 1024 * 1024)  // 内容扫描仅读 1MB
)

// ScanVersionAsync 异步扫描：上传/重扫请求立即返回，扫描在后台 goroutine 执行。
// 状态流转：先置 SCANNING（扫描中），完成落 PASSED/FAILED（安全/不安全），
// 扫描器自身出错/panic 落 ERROR（扫描失败，非内容结论）——与「不安全」区分开。
// 三个必须点：请求 ctx 随 handler 返回即取消→换 Background+超时；
// 裸 goroutine panic 会带崩整个 api 进程→recover 兜底；
// 失败/panic 显式落审计（ERROR + scanner_raw 记原因），避免 SCANNING 永久卡死。
func (s *ScannerService) ScanVersionAsync(versionId, auditId string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("skill 安全扫描 panic(已捕获)", "versionId", versionId, "panic", r)
				_ = s.secRepo.UpdateAudit(context.Background(), auditId, entity.AuditStatusError, 0, nil, "scan panic: "+fmt.Sprint(r))
			}
			s.notifyScanResult(versionId) // 通知挂在 defer：正常完成/出错/panic 各路径都覆盖
		}()
		_ = s.secRepo.UpdateAudit(context.Background(), auditId, entity.AuditStatusScanning, 0, nil, "")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := s.ScanVersion(ctx, versionId, auditId); err != nil {
			slog.Warn("skill version 安全扫描失败(不阻断)", "versionId", versionId, "err", err)
			_ = s.secRepo.UpdateAudit(context.Background(), auditId, entity.AuditStatusError, 0, nil, "scan error: "+err.Error())
		}
	}()
}

// notifyScanResult 扫描落终态后读回最新审计，给技能归属人（上传/导入/重扫发起人，
// OwnerId 是稳定收口）发双通道通知：通过=success、不安全=warning（带 findings 原因）、
// 扫描器出错=error。fail-open：未注入/查不到/仍在扫/通知链路失败都只记日志，
// 不影响扫描结论本身。
func (s *ScannerService) notifyScanResult(versionId string) {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("skill 扫描结论通知 panic(已捕获)", "versionId", versionId, "panic", r)
		}
	}()
	if s.channelNotice == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	a, err := s.secRepo.LatestAuditBySkillVersion(ctx, versionId)
	if err != nil || a == nil {
		return
	}
	var ntype, title, content string
	switch a.Status {
	case entity.AuditStatusPassed:
		ntype, title = "success", "技能安全扫描通过"
	case entity.AuditStatusFailed:
		ntype, title = "warning", "技能安全扫描未通过"
	case entity.AuditStatusError:
		ntype, title = "error", "技能安全扫描失败"
	default: // PENDING/SCANNING：终态写入失败等边缘，不给假结论
		return
	}
	// 版本→技能一次取齐：收件人（归属人）+ 通知文案用的「技能名 版本号」（回落 id）
	v, err := s.repo.GetVersion(ctx, versionId)
	if err != nil || v == nil {
		return
	}
	sk, err := s.repo.Get(ctx, v.SkillId)
	if err != nil || sk == nil {
		return
	}
	name := orDefault(sk.DisplayName, orDefault(sk.Slug, sk.SkillId))
	ver := orDefault(v.Version, versionId)
	switch a.Status {
	case entity.AuditStatusPassed:
		content = fmt.Sprintf("技能「%s」%s 安全扫描通过，该版本可发布/安装。", name, ver)
	case entity.AuditStatusFailed:
		content = fmt.Sprintf("技能「%s」%s 安全扫描未通过（%s），该版本已禁止发布/安装。", name, ver, findingsSummary(a.FindingsJson))
	default:
		content = fmt.Sprintf("技能「%s」%s 扫描器出错（%s），未得出内容结论，可重扫。", name, ver, a.ScannerRaw)
	}
	s.channelNotice.Push(ctx, sk.OwnerId, title, content, "/skill/security/list", ntype, "scanner")
}

// ScanVersion 扫描版本文件并回写审计结论。
func (s *ScannerService) ScanVersion(ctx context.Context, versionId, auditId string) error {
	files, err := s.repo.ListFiles(ctx, versionId)
	if err != nil {
		return err
	}
	var findings []finding
	hasSkillMD := false
	for _, f := range files {
		fp := f.FilePath
		if strings.Contains(fp, "..") || filepath.IsAbs(fp) {
			findings = append(findings, finding{fp, "path_traversal", severityCritical, "文件路径含非法片段（路径穿越风险）"})
		}
		if bannedExt[strings.ToLower(filepath.Ext(fp))] {
			findings = append(findings, finding{fp, "banned_extension", severityHigh, "禁止的可执行文件扩展名"})
		}
		if strings.EqualFold(filepath.Base(fp), "SKILL.md") {
			hasSkillMD = true
		}
		if f.FileSize > maxFileSize {
			findings = append(findings, finding{fp, "oversize", severityLow, "文件超过 50MB"})
		}
		// 内容扫描
		if f.FileSize > 0 && f.FileSize <= scanTextMax && isMaybeText(fp) {
			if rc, err := s.storage.Open(ctx, f.StorageKey); err == nil {
				findings = scanContent(fp, rc, findings)
				rc.Close()
			}
		}
	}
	if !hasSkillMD {
		findings = append(findings, finding{"", "missing_skill_md", severityLow, "包内未找到 SKILL.md 描述文件"})
	}

	maxSev := uint(severityInfo)
	for _, f := range findings {
		if f.Severity > maxSev {
			maxSev = f.Severity
		}
	}
	status := uint(1) // PASSED
	if maxSev >= severityHigh {
		status = 2 // FAILED
	}
	raw, _ := json.Marshal(findings)
	return s.secRepo.UpdateAudit(ctx, auditId, status, maxSev, raw, "")
}

func isMaybeText(path string) bool {
	e := strings.ToLower(filepath.Ext(path))
	switch e {
	case ".md", ".txt", ".json", ".yaml", ".yml", ".js", ".ts", ".py", ".go", ".java",
		".sh", ".toml", ".xml", ".html", ".css", ".sql", ".cfg", ".ini", ".env":
		return true
	}
	return false
}

func scanContent(filePath string, r io.Reader, findings []finding) []finding {
	br := bufio.NewReader(r)
	buf := make([]byte, 0, 4096)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			buf = append(buf, line...)
		}
		if len(buf) > 1<<20 { // 截断 1MB
			break
		}
		if err != nil {
			break
		}
	}
	for _, p := range patterns {
		if p.re.Find(buf) != nil {
			findings = append(findings, finding{filePath, p.rule, p.sev, "命中敏感模式: " + p.rule})
		}
	}
	return findings
}
