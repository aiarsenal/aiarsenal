package service

import (
	"sort"
	"sync"
	"testing"
	"time"
)

// skillChangeAggregator 短窗聚合：同键（用户+实例+装/卸）窗口内多笔合并成一次 flush；
// 不同键各自独立；窗口从每键首笔起算不滑动。绑定分组自动配套装一批技能时，
// 渠道推送 N 连发轰炸微信（6 连发踩过），聚合后只发一条。
func TestSkillChangeAggregator(t *testing.T) {
	var mu sync.Mutex
	var flushed [][]string // 每次 flush 的技能名单
	agg := newSkillChangeAggregator(30*time.Millisecond, func(userId, instanceId, instanceName string, installed bool, names []string) {
		mu.Lock()
		flushed = append(flushed, names)
		mu.Unlock()
	})

	// 同键 3 笔（同一 fan-out 批次）→ 只 flush 一次、3 个名字齐全。
	agg.add("u1", "inst1", "demo", "kb", true)
	agg.add("u1", "inst1", "demo", "doc", true)
	agg.add("u1", "inst1", "demo", "notice", true)
	// 异键（另一实例）→ 独立 flush。
	agg.add("u1", "inst2", "demo2", "pdf", true)
	// 同实例但动作不同（卸载）→ 独立聚合键。
	agg.add("u1", "inst1", "demo", "old", false)

	time.Sleep(120 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(flushed) != 3 {
		t.Fatalf("flush 次数=%d want 3（inst1 装 / inst2 装 / inst1 卸）: %v", len(flushed), flushed)
	}
	var joined []string
	for _, names := range flushed {
		sort.Strings(names)
		joined = append(joined, names[0]+"|"+string(rune('0'+len(names))))
	}
	sort.Strings(joined)
	// 期望三组（组内排序后取首名+个数，再整体排序）：inst1 安装组 3 个（doc/kb/notice）、
	// inst1 卸载组 1 个、inst2 安装组 1 个。
	want := []string{"doc|3", "old|1", "pdf|1"}
	for i, w := range want {
		if joined[i] != w {
			t.Fatalf("flush 组 %d=%q want %q（全部: %v）", i, joined[i], w, joined)
		}
	}
}

// skillChangeText 文案：单条保持原句式；多条合并点名（含去重去空）。
func TestSkillChangeText(t *testing.T) {
	title, content := skillChangeText([]string{"kb"}, "demo", true)
	if title != "技能已安装" || content != "技能「kb」已安装到实例「demo」。" {
		t.Fatalf("单装文案异常: %q / %q", title, content)
	}
	title, content = skillChangeText([]string{"kb", "doc", "kb", ""}, "demo", true)
	if title != "技能已安装" || content != "2 个技能已安装到实例「demo」：kb、doc。" {
		t.Fatalf("多装聚合文案异常: %q / %q", title, content)
	}
	title, content = skillChangeText([]string{"kb", "doc"}, "demo", false)
	if title != "技能已卸载" || content != "2 个技能已从实例「demo」卸载：kb、doc。" {
		t.Fatalf("多卸聚合文案异常: %q / %q", title, content)
	}
}
