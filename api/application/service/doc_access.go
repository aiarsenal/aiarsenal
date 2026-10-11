package service

// 文档/目录访问判定核心 + 组合授权管理（读写分离）。
//
// 判定语义（与菜单 GrantRow 同一套条件行模型，单源 model.GrantRow）：
//   canRead  = 超管 ∨ owner ∨ visibility=2 ∨ (visibility=1 ∧ ∃read行命中) ∨ ∃write行命中
//              （文档再叠加目录门：所属目录可读才可见，目录可读=自身可读∧父链全可读）
//   canWrite = 超管 ∨ owner ∨ ∃write行命中
// 行命中 = 非空维度全 AND（维度内多值=其一），Exclude 行取反（黑名单）。
//
// 身份参数 t *model.Identity：nil = 系统/维护路径（MCP system、内部调用），跳过判定全放行；
// 非-nil 一律按五元组收紧。List/Search 的 SQL 过滤谓词（doc_repo.go）与本文件 Go 判定语义逐条对齐。

import (
	"context"
	"encoding/json"
	"errors"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
)

// grantRowFromEntity 实体条件行 → 模型条件行（jsonb 文本 → []string；解析失败按空=不限）
func grantRowFromEntity(g *entity.DocGrant) model.GrantRow {
	return model.GrantRow{
		UserIds: jsonToIDs(g.UserIds),
		DeptId:  g.DeptId,
		RoleIds: jsonToIDs(g.RoleIds),
		PostIds: jsonToIDs(g.PostIds),
		Exclude: g.Exclude,
		GroupId: g.GroupId,
		SetId:   g.SetId,
	}
}

func jsonToIDs(s string) []string {
	if s == "" {
		return nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(s), &ids); err != nil {
		return nil
	}
	return ids
}

// docPermView 一次请求的权限视图：全量目录 + 全量条件行载入内存（两表皆小），
// 目录可读集按「自身可读 ∧ 父链全可读」拓扑求值。
type docPermView struct {
	dirs         map[string]*entity.DocDir
	dirReadRows  map[string][]model.GrantRow
	dirWriteRows map[string][]model.GrantRow
	docReadRows  map[string][]model.GrantRow
	docWriteRows map[string][]model.GrantRow
	readableDirs map[string]bool
}

// loadPerm DocService 侧入口；目录服务同走 buildDocPermView（判定核心单源）
func (s *DocService) loadPerm(ctx context.Context, t model.Identity) (*docPermView, error) {
	return buildDocPermView(ctx, s.dirRepo, s.grantRepo, t)
}

// buildDocPermView 装载权限视图（DocService/DocDirService 共用）
func buildDocPermView(ctx context.Context, dirRepo repository.IDocDirRepository, grantRepo repository.IDocGrantRepository, t model.Identity) (*docPermView, error) {
	dirs, err := dirRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	all, err := grantRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	v := &docPermView{
		dirs:         make(map[string]*entity.DocDir, len(dirs)),
		dirReadRows:  map[string][]model.GrantRow{},
		dirWriteRows: map[string][]model.GrantRow{},
		docReadRows:  map[string][]model.GrantRow{},
		docWriteRows: map[string][]model.GrantRow{},
		readableDirs: map[string]bool{},
	}
	for _, d := range dirs {
		v.dirs[d.DirId] = d
	}
	for _, g := range all {
		row := grantRowFromEntity(g)
		switch {
		case g.TargetType == "dir" && g.Action == "read":
			v.dirReadRows[g.TargetId] = append(v.dirReadRows[g.TargetId], row)
		case g.TargetType == "dir":
			v.dirWriteRows[g.TargetId] = append(v.dirWriteRows[g.TargetId], row)
		case g.Action == "read":
			v.docReadRows[g.TargetId] = append(v.docReadRows[g.TargetId], row)
		default:
			v.docWriteRows[g.TargetId] = append(v.docWriteRows[g.TargetId], row)
		}
	}

	// 目录可读集：父链可读 ∧ 自身可读，按父先于子的顺序求值（树无环，Move 有环检测）。
	children := make(map[string][]string, len(dirs))
	var roots []string
	for _, d := range dirs {
		if _, ok := v.dirs[d.ParentId]; ok && d.ParentId != "" {
			children[d.ParentId] = append(children[d.ParentId], d.DirId)
		} else {
			roots = append(roots, d.DirId)
		}
	}
	queue := append([]string{}, roots...)
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		d := v.dirs[id]
		parentOK := d.ParentId == "" || v.readableDirs[d.ParentId]
		v.readableDirs[id] = parentOK && v.selfReadDir(t, d)
		queue = append(queue, children[id]...)
	}
	return v, nil
}

// selfReadDir 目录自身可读（不含父链；父链由 readableDirs 叠加）
func (v *docPermView) selfReadDir(t model.Identity, d *entity.DocDir) bool {
	if t.IsAdmin || (t.UserId != "" && d.CreatedBy == t.UserId) {
		return true
	}
	if d.Visibility == 2 {
		return true
	}
	if d.Visibility == 1 && model.MatchAny(v.dirReadRows[d.DirId], t) {
		return true
	}
	return model.MatchAny(v.dirWriteRows[d.DirId], t)
}

// canWriteDir 目录可写：改名/删除/在其中建子目录/配授权
func (v *docPermView) canWriteDir(t model.Identity, d *entity.DocDir) bool {
	if t.IsAdmin || (t.UserId != "" && d.CreatedBy == t.UserId) {
		return true
	}
	return model.MatchAny(v.dirWriteRows[d.DirId], t)
}

// canReadDoc 文档可读（含目录门：所属目录不可读则整体不可见）
func (v *docPermView) canReadDoc(t model.Identity, d *entity.Doc) bool {
	if d.DirId != "" && !v.readableDirs[d.DirId] {
		return false
	}
	if t.IsAdmin || (t.UserId != "" && d.CreatedBy == t.UserId) {
		return true
	}
	if d.Visibility == 2 {
		return true
	}
	if d.Visibility == 1 && model.MatchAny(v.docReadRows[d.DocId], t) {
		return true
	}
	return model.MatchAny(v.docWriteRows[d.DocId], t)
}

// canWriteDoc 文档可写：编辑/更新原件/删除/移动/重嵌/配授权
func (v *docPermView) canWriteDoc(t model.Identity, d *entity.Doc) bool {
	if t.IsAdmin || (t.UserId != "" && d.CreatedBy == t.UserId) {
		return true
	}
	return model.MatchAny(v.docWriteRows[d.DocId], t)
}

// ===== DocService 上的断言入口（nil 身份 = 系统路径全放行） =====

// assertDocRead 单文档读校验（详情/全文/原件）
func (s *DocService) assertDocRead(ctx context.Context, docId string, t *model.Identity) (*entity.Doc, error) {
	d, err := s.repo.Get(ctx, docId)
	if err != nil {
		return nil, errors.New("文档不存在")
	}
	if t == nil {
		return d, nil
	}
	v, err := s.loadPerm(ctx, *t)
	if err != nil {
		return nil, err
	}
	if !v.canReadDoc(*t, d) {
		return nil, errors.New("没有访问权限")
	}
	return d, nil
}

// assertDocWrite 单文档写校验（编辑/更新/删除/移动/重嵌）
func (s *DocService) assertDocWrite(ctx context.Context, docId string, t *model.Identity) (*entity.Doc, error) {
	d, err := s.repo.Get(ctx, docId)
	if err != nil {
		return nil, errors.New("文档不存在")
	}
	if t == nil {
		return d, nil
	}
	v, err := s.loadPerm(ctx, *t)
	if err != nil {
		return nil, err
	}
	if !v.canWriteDoc(*t, d) {
		return nil, errors.New("没有操作权限")
	}
	return d, nil
}

// readableDirIds 当前身份可读目录集（List/Search 的目录门下推到 SQL）；nil 身份返回 nil=不限
func (s *DocService) readableDirIds(ctx context.Context, t *model.Identity) ([]string, error) {
	if t == nil {
		return nil, nil
	}
	v, err := s.loadPerm(ctx, *t)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(v.readableDirs))
	for id, ok := range v.readableDirs {
		if ok {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// ===== 组合授权存取（配置入口：超管或目标 canWrite） =====

// DocGrantInfo 授权配置回显
type DocGrantInfo struct {
	Visibility uint             `json:"visibility"` // 0私有 1按条件 2全员
	ReadRows   []model.GrantRow `json:"readRows"`
	WriteRows  []model.GrantRow `json:"writeRows"`
}

// GetGrants 查询目标的授权配置（targetType doc|dir）。nil 身份（系统）不限。
func (s *DocService) GetGrants(ctx context.Context, targetType, targetId string, t *model.Identity) (*DocGrantInfo, error) {
	if targetType != "doc" && targetType != "dir" {
		return nil, errors.New("targetType 取值 doc|dir")
	}
	info := &DocGrantInfo{ReadRows: []model.GrantRow{}, WriteRows: []model.GrantRow{}}
	if targetType == "doc" {
		d, err := s.repo.Get(ctx, targetId)
		if err != nil {
			return nil, errors.New("文档不存在")
		}
		info.Visibility = d.Visibility
	} else {
		d, err := s.dirRepo.Get(ctx, targetId)
		if err != nil {
			return nil, errors.New("目录不存在")
		}
		if isSpaceRootKind(d.Kind) {
			return nil, errors.New("固定空间不支持配置授权")
		}
		info.Visibility = d.Visibility
	}
	rows, err := s.grantRepo.ListByTarget(ctx, targetType, targetId)
	if err != nil {
		return nil, err
	}
	for _, g := range rows {
		if g.Action == "read" {
			info.ReadRows = append(info.ReadRows, grantRowFromEntity(g))
		} else {
			info.WriteRows = append(info.WriteRows, grantRowFromEntity(g))
		}
	}
	// 配置回显也要求写权限（与保存同门槛，防止探查他人文档的授权配置）
	if t != nil {
		allowed, err := s.canManageGrants(ctx, targetType, targetId, *t)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, errors.New("没有操作权限")
		}
	}
	return info, nil
}

// canManageGrants 授权配置权：超管或目标 canWrite（owner 天然满足）
func (s *DocService) canManageGrants(ctx context.Context, targetType, targetId string, t model.Identity) (bool, error) {
	v, err := s.loadPerm(ctx, t)
	if err != nil {
		return false, err
	}
	if targetType == "doc" {
		d, err := s.repo.Get(ctx, targetId)
		return err == nil && d != nil && v.canWriteDoc(t, d), nil
	}
	d, err := s.dirRepo.Get(ctx, targetId)
	return err == nil && d != nil && v.canWriteDir(t, d), nil
}

// SaveGrants 保存目标的可见性档位 + 读/写条件行（整体重写）。nil 身份（系统）不限。
func (s *DocService) SaveGrants(ctx context.Context, targetType, targetId string, visibility uint, readJSON, writeJSON string, t *model.Identity) error {
	if targetType != "doc" && targetType != "dir" {
		return errors.New("targetType 取值 doc|dir")
	}
	if visibility > 2 {
		return errors.New("visibility 取值 0|1|2")
	}
	readRows, err := model.ParseGrantRows(readJSON)
	if err != nil {
		return errors.New("读条件：" + err.Error())
	}
	writeRows, err := model.ParseGrantRows(writeJSON)
	if err != nil {
		return errors.New("写条件：" + err.Error())
	}

	actor := ""
	if t != nil {
		actor = t.UserId
		allowed, err := s.canManageGrants(ctx, targetType, targetId, *t)
		if err != nil {
			return err
		}
		if !allowed {
			return errors.New("没有操作权限")
		}
	}

	if targetType == "doc" {
		if _, err := s.repo.Get(ctx, targetId); err != nil {
			return errors.New("文档不存在")
		}
		if err := s.repo.SetVisibility(ctx, targetId, visibility); err != nil {
			return err
		}
	} else {
		got, err := s.dirRepo.Get(ctx, targetId)
		if err != nil {
			return errors.New("目录不存在")
		}
		if isSpaceRootKind(got.Kind) {
			return errors.New("固定空间不支持配置授权")
		}
		if err := s.dirRepo.SetVisibility(ctx, targetId, visibility); err != nil {
			return err
		}
	}
	if err := s.grantRepo.Replace(ctx, targetType, targetId, "read", readRows, actor); err != nil {
		return err
	}
	return s.grantRepo.Replace(ctx, targetType, targetId, "write", writeRows, actor)
}

// UploadVis 上传批次的可见性：对每个新建文档落 visibility + 条件行（controller 解析表单构造）
type UploadVis struct {
	Visibility uint
	ReadRows   []model.GrantRow
	WriteRows  []model.GrantRow
	Actor      string
}

// applyUploadVis 新建文档落授权（尽力而为：失败不影响入库结果，文档仍为默认全员可读）
func (s *DocService) applyUploadVis(ctx context.Context, docId string, vis *UploadVis) {
	if vis == nil {
		return
	}
	if err := s.repo.SetVisibility(ctx, docId, vis.Visibility); err != nil {
		return
	}
	_ = s.grantRepo.Replace(ctx, "doc", docId, "read", vis.ReadRows, vis.Actor)
	_ = s.grantRepo.Replace(ctx, "doc", docId, "write", vis.WriteRows, vis.Actor)
}
