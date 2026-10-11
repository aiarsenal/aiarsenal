// Package grantmeta 权限分组/员工集合的内存快照与身份解析。
//
// 引用行（分组/集合）的鉴权依赖请求时实时解析的 Identity.Groups/Sets；
// 本包维护两字典的内存快照（RWMutex + copy-on-write），写路径（permgroup 服务）
// 保存后调 Reload 立即生效。api 与 mcp-ops 是两个进程，ops 侧快照无人推送刷新，
// 故 Resolve 按 TTL 兜底重载（也覆盖手改库的场景）；跨进程可见性最多滞后一个 TTL。
package grantmeta

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"aiarsenal/domain/model"
	"aiarsenal/infrastructure/db"
)

// TTL 快照最大驻留时长：超过即在下次 Resolve 时重载。
// 测试可置 0 强制每次重载，或负值禁用 TTL（仅写路径 Reload 生效）。
var TTL = 5 * time.Second

type snapshot struct {
	groups   map[string][]model.GrantRow // group_id → 词条行
	sets     map[string][]string         // set_id → 工号数组
	loadedAt time.Time
}

var (
	mu   sync.RWMutex
	snap *snapshot
)

type groupRow struct {
	GroupID string          `gorm:"column:group_id"`
	Terms   json.RawMessage `gorm:"column:terms"`
}

type setRow struct {
	SetID  string          `gorm:"column:set_id"`
	EmpNos json.RawMessage `gorm:"column:emp_nos"`
}

// Reload 重新加载两字典快照（写路径保存后调用，本进程立即生效）。
// 依赖 db.DB() 惰性初始化（ops 进程不预 init 默认库，与 identityFromCtx 同模式）。
func Reload() error {
	g := db.DB()
	var gRows []groupRow
	if err := g.WithContext(context.Background()).
		Table("aa_perm_group").Select("group_id", "terms").Find(&gRows).Error; err != nil {
		return err
	}
	var sRows []setRow
	if err := g.WithContext(context.Background()).
		Table("aa_emp_set").Select("set_id", "emp_nos").Find(&sRows).Error; err != nil {
		return err
	}

	ns := &snapshot{
		groups:   make(map[string][]model.GrantRow, len(gRows)),
		sets:     make(map[string][]string, len(sRows)),
		loadedAt: time.Now(),
	}
	for _, r := range gRows {
		// 入库前经 ParseGrantRows 校验，此处解析失败跳过该分组（fail-closed：不命中）
		rows, err := model.ParseGrantRows(string(r.Terms))
		if err != nil {
			continue
		}
		ns.groups[r.GroupID] = rows
	}
	for _, r := range sRows {
		nos, err := model.ParseEmpNos(string(r.EmpNos))
		if err != nil {
			continue
		}
		ns.sets[r.SetID] = nos
	}

	mu.Lock()
	snap = ns
	mu.Unlock()
	return nil
}

func load() (*snapshot, error) {
	mu.RLock()
	s := snap
	mu.RUnlock()
	if s != nil && (TTL < 0 || time.Since(s.loadedAt) < TTL) {
		return s, nil
	}
	err := Reload()
	// 无论重载成败都重读快照：成功拿新快照；失败退回旧快照；都没有则空快照
	//（引用行不命中，fail-closed）。修首请求空指针：进程启动后首次 load 时 snap
	// 为 nil、重载成功，旧代码返回的却仍是开头读到的 nil，调用方 s.sets 即 panic。
	mu.RLock()
	s = snap
	mu.RUnlock()
	if s == nil {
		s = &snapshot{groups: map[string][]model.GrantRow{}, sets: map[string][]string{}}
	}
	return s, err
}

// ResolveIdentity 就地解析身份命中的分组/集合（填 t.Sets/t.Groups）。
// 先集合后分组（分组词条可引用集合）；解析失败保持零值（fail-closed）。
func ResolveIdentity(t *model.Identity) {
	s, _ := load()
	t.Sets = model.SetHits(s.sets, *t)
	t.Groups = model.GroupHits(s.groups, s.sets, *t)
}

// Snapshots 暴露快照副本（保存侧环校验/引用存在性检查用）。
func Snapshots() (groups map[string][]model.GrantRow, sets map[string][]string, err error) {
	s, err := load()
	if err != nil {
		return nil, nil, err
	}
	gs := make(map[string][]model.GrantRow, len(s.groups))
	for k, v := range s.groups {
		gs[k] = v
	}
	ss := make(map[string][]string, len(s.sets))
	for k, v := range s.sets {
		ss[k] = v
	}
	return gs, ss, nil
}
