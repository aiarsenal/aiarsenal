package service

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	storagestg "github.com/aiarsenal/runtime-go/storage"
	"golang.org/x/text/unicode/norm"
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
				_ = s.secRepo.UpdateAudit(context.Background(), auditId, entity.AuditStatusError, 0, nil, "scan panic: "+fmt.Sprint(r), "")
			}
			s.notifyScanResult(versionId) // 通知挂在 defer：正常完成/出错/panic 各路径都覆盖
		}()
		_ = s.secRepo.UpdateAudit(context.Background(), auditId, entity.AuditStatusScanning, 0, nil, "", "")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := s.ScanVersion(ctx, versionId, auditId); err != nil {
			slog.Warn("skill version 安全扫描失败(不阻断)", "versionId", versionId, "err", err)
			_ = s.secRepo.UpdateAudit(context.Background(), auditId, entity.AuditStatusError, 0, nil, "scan error: "+err.Error(), "")
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
		if hasBannedExtension(fp) {
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
	return s.secRepo.UpdateAudit(ctx, auditId, status, maxSev, raw, "", skillContentHash(files))
}

// normalizeName 文件名归一口径（F-013 全部文件名判定共用）：NFKC 折叠（全角→半角，
// 含全角句点/全角空格）→ 剥尾点尾空格（Windows 抽取语义会把 "tool.exe." / "tool.exe "
// 落成 "tool.exe"）→ 小写。
// 注意载荷边界：尾点情形（tool.exe.）剥与不剥都会被逐段比对命中（exe 已是段），
// TrimRight 真正承载的是「黑名单段后跟空格」（tool.exe ）与全角空格折叠后的剥离。
func normalizeName(p string) string {
	return strings.TrimRight(strings.ToLower(norm.NFKC.String(path.Base(p))), ". ")
}

// hasBannedExtension 扩展名黑名单判定（F-013）：归一化后按 '.' 逐段比对，堵四类绕过——
// 全角形近字符（tool.eｘe，NFKC 折叠）、尾点/尾空格（Windows 抽取语义）、
// 双扩展名（tool.exe.txt 逐段比对）、大小写变体（ToLower）。
// 已知局限：NFKC 之外的形近字符（如西里尔 е）不折叠，属黑名单口径外残余，正文披露。
func hasBannedExtension(name string) bool {
	for _, seg := range strings.Split(normalizeName(name), ".")[1:] {
		if bannedExt["."+seg] {
			return true
		}
	}
	return false
}

// skillContentHash 版本文件集聚合哈希（F-013 门禁内容绑定）：sha256 over 按路径排序、
// 以 '\n' 连接的 "path:filesha256" 行（无尾换行）。与 ListFiles 返回顺序无关（排序钉死）；
// 文件集任一变化哈希必变。空文件集返回空串（扫描期不该发生——创建编排保证先持久化后派发）。
func skillContentHash(files []*entity.SkillFile) string {
	if len(files) == 0 {
		return ""
	}
	lines := make([]string, 0, len(files))
	for _, f := range files {
		lines = append(lines, f.FilePath+":"+f.Sha256)
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

// isMaybeText 内容扫描的文件选择：与 hasBannedExtension 共用 normalizeName 归一口径——
// 否则 "secrets.ｔｘｔ" / "env.txt " 这类变体既不进内容扫描也不在黑名单，私钥规则整层逃逸。
// 取归一化后的尾段比对白名单（无需逐段，白名单本就只含纯文本扩展）。
func isMaybeText(fp string) bool {
	segs := strings.Split(normalizeName(fp), ".")
	if len(segs) < 2 {
		return false
	}
	switch segs[len(segs)-1] {
	case "md", "txt", "json", "yaml", "yml", "js", "ts", "py", "go", "java",
		"sh", "toml", "xml", "html", "css", "sql", "cfg", "ini", "env":
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
