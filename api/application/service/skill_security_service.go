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
	repo      repository.ISkillSecurityRepository
	skillRepo repository.ISkillRepository // F-017b A-01：审计行的技能归属判定
	nsRepo    repository.ISkillNamespaceRepository
	scanner   *ScannerService
}

func NewSkillSecurityService(r repository.ISkillSecurityRepository, sr repository.ISkillRepository, nr repository.ISkillNamespaceRepository, sc *ScannerService) *SkillSecurityService {
	return &SkillSecurityService{repo: r, skillRepo: sr, nsRepo: nr, scanner: sc}
}

// ListAudits 审计列表（F-017b A-01 收口）：VersionId 为空的跨技能枚举仅超管可用；
// 非超管必须指定 versionId 且该技能可读（与详情谓词同语义）——否则「未找到」，
// 堵「分页枚举他人 PRIVATE 技能 versionId → 喂给 version/get 与机器取包」的 ID oracle。
func (s *SkillSecurityService) ListAudits(ctx context.Context, versionId, status string, pageNum, pageSize int, callerId string, callerIsAdmin bool) ([]*entity.SkillSecurityAudit, int64, error) {
	if !callerIsAdmin {
		if versionId == "" {
			return nil, 0, errDetailNotFound
		}
		v, err := s.skillRepo.GetVersion(ctx, versionId)
		if err != nil {
			return nil, 0, errDetailNotFound
		}
		sk, skerr := s.skillRepo.Get(ctx, v.SkillId)
		if skerr != nil {
			return nil, 0, errDetailNotFound
		}
		isMember := false
		if callerId != "" && sk.Visibility == 1 {
			ms, _ := s.nsRepo.ListMembers(ctx, sk.NamespaceId)
			for _, m := range ms {
				if m.UserId == callerId {
					isMember = true
					break
				}
			}
		}
		if !canReadSkill(sk.OwnerId, sk.Visibility, callerId, callerIsAdmin, isMember) {
			return nil, 0, errDetailNotFound
		}
	}
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

// ensureSkillVersionSafe 安装/发布统一门禁：版本最新安全审计必须是「安全」（PASSED）。
// 不安全时把 findings 原因（规则@文件）摘要进报错；扫描失败带 scannerRaw 原因。
// secRepo 未接线（部分装配路径）时放行，保持与既有 nil-guard 行为一致。
func ensureSkillVersionSafe(ctx context.Context, secRepo repository.ISkillSecurityRepository, versionId string) error {
	if secRepo == nil {
		return nil
	}
	a, err := secRepo.LatestAuditBySkillVersion(ctx, versionId)
	if err != nil || a == nil {
		return fmt.Errorf("该版本尚无安全审计记录，请先在安全审计页发起重扫（versionId=%s）", versionId)
	}
	switch a.Status {
	case entity.AuditStatusPassed:
		return nil
	case entity.AuditStatusPending:
		return fmt.Errorf("安全审计未开始（待扫描），暂不可安装/发布（versionId=%s）", versionId)
	case entity.AuditStatusScanning:
		return fmt.Errorf("安全审计扫描中，请稍候再试（versionId=%s）", versionId)
	case entity.AuditStatusError:
		return fmt.Errorf("安全扫描失败：%s（请重扫，versionId=%s）", a.ScannerRaw, versionId)
	default: // AuditStatusFailed：不安全，报出 findings 原因
		return fmt.Errorf("安全审计未通过（不安全）：%s（versionId=%s）", findingsSummary(a.FindingsJson), versionId)
	}
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
