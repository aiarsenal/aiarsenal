package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

type SkillSecurityService struct {
	repo    repository.ISkillSecurityRepository
	scanner *ScannerService
}

func NewSkillSecurityService(r repository.ISkillSecurityRepository, sc *ScannerService) *SkillSecurityService {
	return &SkillSecurityService{repo: r, scanner: sc}
}

func (s *SkillSecurityService) ListAudits(ctx context.Context, versionId, status string, pageNum, pageSize int) ([]*entity.SkillSecurityAudit, int64, error) {
	return s.repo.ListAudits(ctx, versionId, status, pageNum, pageSize)
}

func (s *SkillSecurityService) GetAudit(ctx context.Context, auditId string) (*entity.SkillSecurityAudit, error) {
	return s.repo.GetAudit(ctx, auditId)
}

// Rescan 触发重新扫描：同步创建 PENDING 审计记录（UI 立即可见），扫描本身异步执行。
func (s *SkillSecurityService) Rescan(ctx context.Context, versionId string) (string, error) {
	auditId, err := s.repo.Rescan(ctx, versionId)
	if err != nil {
		return "", err
	}
	if s.scanner != nil {
		s.scanner.ScanVersionAsync(versionId, auditId)
	}
	return auditId, nil
}

// LatestAudit 取版本最新一条审计（同版本多次重扫取最近一次）；
// MCP 外部引用安装流程轮询扫描结论用。
func (s *SkillSecurityService) LatestAudit(ctx context.Context, versionId string) (*entity.SkillSecurityAudit, error) {
	return s.repo.LatestAuditBySkillVersion(ctx, versionId)
}

// FindingsSummary findings_json 摘成「规则@文件」可读原因串
// （ops 工具输出用，与安装/发布门禁报错同源同格式）。
func FindingsSummary(raw json.RawMessage) string {
	return findingsSummary(raw)
}

// ensureSkillVersionSafe 安装/发布统一门禁（F-013 重写）：不再只信「最新一条审计」——
// 要求历史无 FAILED（不安全结论永久阻断）、最新审计为 PASSED、且其内容聚合哈希与当前
// 文件集一致（审计与内容强绑定，竞态期/内容变更后的过期 PASSED 不再放行）。
// 判定逻辑收口在 gateVerdict 纯函数（表测覆盖），本函数只做编排。
// secRepo 未接线（部分装配路径）时放行，保持与既有 nil-guard 行为一致；
// secRepo 已接线而 skillRepo 缺失属异常装配 → 拒（fail-closed，披露于正文）。
func ensureSkillVersionSafe(ctx context.Context, secRepo repository.ISkillSecurityRepository, skillRepo repository.ISkillRepository, versionId string) error {
	if secRepo == nil {
		return nil
	}
	if skillRepo == nil {
		return fmt.Errorf("安全门禁依赖未装配，暂不可安装/发布（versionId=%s）", versionId)
	}
	audits, err := secRepo.ListAllAuditsBySkillVersion(ctx, versionId)
	if err != nil {
		return fmt.Errorf("安全审计读取失败，暂不可安装/发布：%v（versionId=%s）", err, versionId)
	}
	files, err := skillRepo.ListFiles(ctx, versionId)
	if err != nil {
		return fmt.Errorf("版本文件读取失败，暂不可安装/发布：%v（versionId=%s）", err, versionId)
	}
	return gateVerdict(versionId, audits, skillContentHash(files))
}

// gateVerdict 门禁判定收口纯函数（F-013）：audits 为该版本全部审计（id 升序，末位=最新），
// currentHash 为当前文件集聚合哈希（skillContentHash）。规则四条：
//  1. 无审计 → 拒（请先扫描）；
//  2. 历史任一 FAILED → 拒（同一版本文件不可变，FAILED 后的 PASSED 只能来自扫描口径变化，
//     按安全侧处理；恢复途径=上传新版本）；
//  3. 最新非 PASSED → 按状态给既有文案（PENDING/SCANNING/ERROR/FAILED 兜底）；
//  4. 最新 PASSED 但哈希空（升级前存量数据）或与当前文件集不符 → 拒（审计过期/内容漂移）。
//
// 历史中的 ERROR/PENDING/SCANNING 无内容结论，不阻断（否则一次扫描器故障即永久砖死重扫恢复链路）。
func gateVerdict(versionId string, audits []*entity.SkillSecurityAudit, currentHash string) error {
	if len(audits) == 0 {
		return fmt.Errorf("该版本尚无安全审计记录，请先在安全审计页发起重扫（versionId=%s）", versionId)
	}
	for _, a := range audits[:len(audits)-1] {
		if a.Status == entity.AuditStatusFailed {
			return fmt.Errorf("安全审计未通过（不安全）：%s（该版本存在历史不安全结论，须上传新版本，versionId=%s）", findingsSummary(a.FindingsJson), versionId)
		}
	}
	latest := audits[len(audits)-1]
	switch latest.Status {
	case entity.AuditStatusPassed:
		// 哈希绑定检查后放行
	case entity.AuditStatusPending:
		return fmt.Errorf("安全审计未开始（待扫描），暂不可安装/发布（versionId=%s）", versionId)
	case entity.AuditStatusScanning:
		return fmt.Errorf("安全审计扫描中，请稍候再试（versionId=%s）", versionId)
	case entity.AuditStatusError:
		return fmt.Errorf("安全扫描失败：%s（请重扫，versionId=%s）", latest.ScannerRaw, versionId)
	default: // AuditStatusFailed：不安全，报出 findings 原因
		return fmt.Errorf("安全审计未通过（不安全）：%s（versionId=%s）", findingsSummary(latest.FindingsJson), versionId)
	}
	if latest.ContentHash == "" {
		return fmt.Errorf("该版本安全审计过期（无内容绑定），请重扫后再安装/发布（versionId=%s）", versionId)
	}
	if latest.ContentHash != currentHash {
		return fmt.Errorf("该版本内容与安全审计结论不一致，请重扫后再安装/发布（versionId=%s）", versionId)
	}
	return nil
}

// findingsSummary 把 findings_json 摘成「规则@文件」逗号串（最多 5 条，超出的以 …省略）。
// 解析失败/为空时回落「发现危险内容」不阻断报错本身。
func findingsSummary(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "发现危险内容（明细见审计记录 findings）"
	}
	var fs []struct {
		File string `json:"file"`
		Rule string `json:"rule"`
	}
	if err := json.Unmarshal(raw, &fs); err != nil || len(fs) == 0 {
		return "发现危险内容（明细见审计记录 findings）"
	}
	parts := make([]string, 0, len(fs))
	for i, f := range fs {
		if i >= 5 {
			parts = append(parts, "…")
			break
		}
		if f.File != "" {
			parts = append(parts, fmt.Sprintf("%s@%s", f.Rule, f.File))
		} else {
			parts = append(parts, f.Rule)
		}
	}
	return strings.Join(parts, "，")
}
