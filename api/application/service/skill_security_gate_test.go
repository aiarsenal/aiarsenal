package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aiarsenal/domain/entity"
)

// F-013 门禁判定收口纯函数 gateVerdict 表测：删任一检查分支，对应格必红。
func TestGateVerdict(t *testing.T) {
	passed := func(hash string) *entity.SkillSecurityAudit {
		return &entity.SkillSecurityAudit{Status: entity.AuditStatusPassed, ContentHash: hash}
	}
	failed := func() *entity.SkillSecurityAudit {
		return &entity.SkillSecurityAudit{Status: entity.AuditStatusFailed,
			FindingsJson: json.RawMessage(`[{"file":"tool.exe","rule":"banned_extension"}]`)}
	}

	cases := []struct {
		name    string
		audits  []*entity.SkillSecurityAudit
		curHash string
		wantErr string // 空串=放行；否则断言错误文案含该子串
	}{
		{"无审计→拒", nil, "h", "尚无安全审计记录"},
		{"历史FAILED→拒(最新PASSED也救不回)", []*entity.SkillSecurityAudit{failed(), passed("h")}, "h", "历史不安全结论"},
		{"最新FAILED→拒", []*entity.SkillSecurityAudit{failed()}, "h", "安全审计未通过"},
		{"最新PENDING→拒", []*entity.SkillSecurityAudit{{Status: entity.AuditStatusPending}}, "h", "待扫描"},
		{"最新SCANNING→拒", []*entity.SkillSecurityAudit{{Status: entity.AuditStatusScanning}}, "h", "扫描中"},
		{"最新ERROR→拒", []*entity.SkillSecurityAudit{{Status: entity.AuditStatusError, ScannerRaw: "boom"}}, "h", "扫描失败"},
		{"哈希空(存量过期)→拒", []*entity.SkillSecurityAudit{passed("")}, "h", "无内容绑定"},
		{"哈希空且当前哈希空(零文件版本+存量审计)→拒", []*entity.SkillSecurityAudit{passed("")}, "", "无内容绑定"},
		{"哈希不匹配→拒", []*entity.SkillSecurityAudit{passed("old")}, "new", "内容与安全审计结论不一致"},
		{"历史ERROR+最新PASSED匹配→放行(重扫恢复链路)", []*entity.SkillSecurityAudit{
			{Status: entity.AuditStatusError, ScannerRaw: "boom"}, passed("h")}, "h", ""},
		{"全过→放行", []*entity.SkillSecurityAudit{passed("h")}, "h", ""},
	}
	for _, c := range cases {
		err := gateVerdict("v1", c.audits, c.curHash)
		if c.wantErr == "" {
			if err != nil {
				t.Errorf("%s: 期望放行, 实得 %v", c.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), c.wantErr) {
			t.Errorf("%s: 期望错误含 %q, 实得 %v", c.name, c.wantErr, err)
		}
	}
}

// 变异背书：把 FAILED 检查改成只查最新一条（回归旧行为），历史 FAILED 格必红。
func TestGateVerdictHistoryFailedNotIgnored(t *testing.T) {
	old := &entity.SkillSecurityAudit{Status: entity.AuditStatusFailed,
		FindingsJson: json.RawMessage(`[{"file":"tool.exe","rule":"banned_extension"}]`)}
	latest := &entity.SkillSecurityAudit{Status: entity.AuditStatusPassed, ContentHash: "h"}
	err := gateVerdict("v1", []*entity.SkillSecurityAudit{old, latest}, "h")
	if err == nil {
		t.Fatal("历史 FAILED 被最新 PASSED 掩蔽——门禁退化为旧行为，F-013 防线失守")
	}
}

// F-013 编排接线守卫（免库单测测不到编排顺序）：createVersionFromZip 必须在
// persistVersionFiles 成功之后才 dispatchSecurityScan，且 finalizeVersion 不再派发。
// 断言锚完整语句（含调用形态），mutation 式删改会被下列任一断言抓住。
func TestSkillScanDispatchOrderingGuard(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller 失败")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "skill_package_service.go"))
	if err != nil {
		t.Fatalf("读取 skill_package_service.go: %v", err)
	}
	src := string(raw)
	if !strings.Contains(src, "s.dispatchSecurityScan(ctx, versionId)") {
		t.Fatal("createVersionFromZip 丢失 dispatchSecurityScan 接线（扫描派发被移除？）")
	}
	// 派发调用必须出现在 persistVersionFiles 编排点之后（同一函数体内先后序）
	persistPos := strings.Index(src, "if _, err := s.persistVersionFiles(ctx, sk.SkillId, versionId, version, ins); err != nil {")
	dispatchPos := strings.Index(src, "s.dispatchSecurityScan(ctx, versionId)")
	if persistPos < 0 || dispatchPos < 0 || dispatchPos < persistPos {
		t.Fatalf("派发点(%d) 未在文件持久化(%d) 之后——竞态回归（F-013 修复①被拆）", dispatchPos, persistPos)
	}
	// finalizeVersion 内不得再有 ScanVersionAsync（派发必须收口到 dispatchSecurityScan）
	finPos := strings.Index(src, "func (s *SkillPackageService) finalizeVersion")
	nextFunc := strings.Index(src[finPos+1:], "\nfunc (")
	if nextFunc < 0 {
		nextFunc = len(src) - finPos
	}
	finalizeBody := src[finPos : finPos+nextFunc]
	if strings.Contains(finalizeBody, "ScanVersionAsync") {
		t.Fatal("finalizeVersion 内仍有扫描派发——派发早于文件持久化的竞态回归（F-013 修复①被拆）")
	}
	// 安全审计的创建与派发必须在同一收口函数里（Rescan 与 ScanVersionAsync 相邻）
	if !strings.Contains(src, "auditId, err := s.secRepo.Rescan(ctx, versionId)") ||
		!strings.Contains(src, "s.scanner.ScanVersionAsync(versionId, auditId)") {
		t.Fatal("dispatchSecurityScan 的审计创建/派发接线缺失")
	}
}
