// Package identity 网关侧用户权限身份解析（预算适用范围 grant_rules 判定的主体来源）。
//
// api 侧身份经登录 claims + casbin enforcer + grantmeta 快照解析（见
// api/interfaces/middleware/context.go GetIdentity）；网关拿不到 claims，改由本包
// 从 aiarsenal 库（db.IAMDB 只读连接）实时解析同一六维：
//   - 用户/部门/工号：aa_user（对齐 api userRepo.GetDeptAndPosts）
//   - 岗位：aa_user_post
//   - 角色：aa_rule ptype='g' 边的传递闭包（对齐 enforcer GetImplicitRolesForUser，前缀 u_）
//   - 分组/集合：aa_perm_group / aa_emp_set 快照（对齐 api grantmeta，TTL 兜底重载）
//
// 结果按用户 redis 缓存 5s（与 vkey/vbudget 缓存同节奏）；解析失败返回错误，
// 调用方（auth.userBudgetsOk）fail-open 放行并记日志。
package identity

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"aiarsenal/llm/internal/db"
	"aiarsenal/llm/internal/grantx"
)

// casbin 用户主体前缀（api consts.CasBinUserPrefix，两处须同步）。
const casbinUserPrefix = "u_"

// TTL 快照最大驻留时长（对齐 api grantmeta.TTL）。
var TTL = 5 * time.Second

// userCacheTTL 单用户身份 redis 缓存时长。
const userCacheTTL = 5 * time.Second

type snapshot struct {
	groups   map[string][]grantx.GrantRow // group_id → 词条行
	sets     map[string][]string          // set_id → 工号数组
	gEdges   map[string][]string          // casbin g 边：v0 → v1 列表（用户/角色 → 角色）
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
type gRuleRow struct {
	V0 string `gorm:"column:v0"`
	V1 string `gorm:"column:v1"`
}
type userRow struct {
	DeptId     string `gorm:"column:dept_id"`
	EmployeeNo string `gorm:"column:employee_no"`
}
type userPostRow struct {
	PostId string `gorm:"column:post_id"`
}

func load() (*snapshot, error) {
	mu.RLock()
	s := snap
	mu.RUnlock()
	if s != nil && (TTL < 0 || time.Since(s.loadedAt) < TTL) {
		return s, nil
	}
	err := reload()
	// 无论重载成败都重读快照：成功拿新；失败退旧；都没有则空快照（引用行不命中，fail-closed）
	mu.RLock()
	s = snap
	mu.RUnlock()
	if s == nil {
		s = &snapshot{groups: map[string][]grantx.GrantRow{}, sets: map[string][]string{}, gEdges: map[string][]string{}}
	}
	return s, err
}

func reload() error {
	g := db.IAMDB()
	if g == nil {
		return ErrNotReady
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var gRows []groupRow
	if err := g.WithContext(ctx).Table("aa_perm_group").Select("group_id", "terms").Find(&gRows).Error; err != nil {
		return err
	}
	var sRows []setRow
	if err := g.WithContext(ctx).Table("aa_emp_set").Select("set_id", "emp_nos").Find(&sRows).Error; err != nil {
		return err
	}
	var gEdges []gRuleRow
	if err := g.WithContext(ctx).Table("aa_rule").Select("v0", "v1").Where("ptype = 'g'").Find(&gEdges).Error; err != nil {
		return err
	}

	ns := &snapshot{
		groups:   make(map[string][]grantx.GrantRow, len(gRows)),
		sets:     make(map[string][]string, len(sRows)),
		gEdges:   make(map[string][]string, len(gEdges)),
		loadedAt: time.Now(),
	}
	for _, r := range gRows {
		// 入库前经 api ParseGrantRows 校验，此处解析失败跳过该分组（fail-closed：不命中）
		rows, err := grantx.ParseGrantRows(string(r.Terms))
		if err != nil {
			continue
		}
		ns.groups[r.GroupID] = rows
	}
	for _, r := range sRows {
		ns.sets[r.SetID] = grantx.ParseEmpNos(string(r.EmpNos))
	}
	for _, r := range gEdges {
		ns.gEdges[r.V0] = append(ns.gEdges[r.V0], r.V1)
	}

	mu.Lock()
	snap = ns
	mu.Unlock()
	return nil
}

// roleClosure 从用户 casbin 主体出发沿 g 边收集全部（含嵌套）角色业务 id。
func (s *snapshot) roleClosure(userId string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, 4)
	queue := []string{casbinUserPrefix + userId}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range s.gEdges[cur] {
			if seen[next] {
				continue
			}
			seen[next] = true
			out = append(out, next)
			queue = append(queue, next)
		}
	}
	return out
}

// ErrNotReady 表示 IAM 连接未初始化（配置缺失/启动失败），调用方 fail-open。
var ErrNotReady = errors.New("identity: iam db not ready")

// Resolve 解析用户六维身份（含命中分组/集合）。用户不存在返回错误。
func Resolve(ctx context.Context, userId string) (grantx.Identity, error) {
	if userId == "" {
		return grantx.Identity{}, ErrNotReady
	}
	if rdb := db.Redis(); rdb != nil {
		key := "identity:uid:" + userId
		if v, err := rdb.Get(ctx, key).Result(); err == nil {
			var t grantx.Identity
			if json.Unmarshal([]byte(v), &t) == nil {
				return t, nil
			}
		}
	}

	g := db.IAMDB()
	if g == nil {
		return grantx.Identity{}, ErrNotReady
	}
	qctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var u userRow
	if err := g.WithContext(qctx).Table("aa_user").Select("dept_id", "employee_no").
		Where("user_id = ?", userId).First(&u).Error; err != nil {
		return grantx.Identity{}, err
	}
	var posts []userPostRow
	if err := g.WithContext(qctx).Table("aa_user_post").Select("post_id").
		Where("user_id = ?", userId).Find(&posts).Error; err != nil {
		return grantx.Identity{}, err
	}

	t := grantx.Identity{
		UserId:     userId,
		DeptId:     u.DeptId,
		EmployeeNo: u.EmployeeNo,
		PostIds:    make([]string, 0, len(posts)),
	}
	for _, p := range posts {
		t.PostIds = append(t.PostIds, p.PostId)
	}

	s, serr := load()
	t.RoleIds = s.roleClosure(userId)
	t.Groups = grantx.GroupHits(s.groups, s.sets, t)
	t.Sets = grantx.SetHits(s.sets, t)
	if serr != nil {
		slog.Error("identity 快照重载失败（沿用旧/空快照）", "user", userId, "err", serr)
	}

	if rdb := db.Redis(); rdb != nil {
		if b, err := json.Marshal(t); err == nil {
			_ = rdb.Set(ctx, "identity:uid:"+userId, b, userCacheTTL).Err()
		}
	}
	return t, nil
}
