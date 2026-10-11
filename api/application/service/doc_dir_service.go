package service

import (
	"context"
	"errors"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/interfaces/dto"
)

// DocDirService 目录树：建/改名/移动（环检测）/删除（须空目录），树带直属与子树文档计数。
// 身份参数 t *model.Identity：nil = 系统/维护路径全放行；非-nil 时树只回可读目录、
// 写操作（建/改名/移动/删除）校验目录 canWrite（判定核心同 doc_access.go）。
type DocDirService struct {
	repo      repository.IDocDirRepository
	docRepo   repository.IDocRepository
	grantRepo repository.IDocGrantRepository
	userRepo  repository.IUserRepository
}

func NewDocDirService(r repository.IDocDirRepository, docRepo repository.IDocRepository,
	gr repository.IDocGrantRepository, ur repository.IUserRepository) *DocDirService {
	return &DocDirService{repo: r, docRepo: docRepo, grantRepo: gr, userRepo: ur}
}

// Tree 全量目录树（含每目录直属/子树文档数）。非-nil 身份：只回可读目录（父链全可读），
// 节点带 visibility/canWrite；nil 身份：全量 + canWrite=true。
func (s *DocDirService) Tree(ctx context.Context, t *model.Identity) ([]*dto.DocDirNode, error) {
	dirs, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	counts, err := s.docRepo.CountByDir(ctx)
	if err != nil {
		return nil, err
	}
	s.enrichCreators(ctx, dirs)

	var view *docPermView
	if t != nil {
		if view, err = buildDocPermView(ctx, s.repo, s.grantRepo, *t); err != nil {
			return nil, err
		}
	}

	nodes := make(map[string]*dto.DocDirNode, len(dirs))
	for _, d := range dirs {
		if view != nil && !view.readableDirs[d.DirId] {
			continue // 不可见目录整枝剪掉（父不可读则子也不可读，无需单独处理）
		}
		// 个人空间只展示自己的个人目录（超管也一样——他人目录无浏览价值，
		// 文档本身仍按权限在「全部文档」可见；nil 身份=系统路径全量）
		if t != nil && d.Kind == DocDirKindPersonal && d.CreatedBy != t.UserId {
			continue
		}
		canWrite := true
		if view != nil {
			canWrite = view.canWriteDir(*t, d)
		}
		nodes[d.DirId] = &dto.DocDirNode{
			DirId: d.DirId, Name: d.Name, ParentId: d.ParentId,
			Visibility: d.Visibility, CanWrite: canWrite,
			Kind:        d.Kind,
			RefId:       d.RefId,
			DocCount:    int(counts[d.DirId]),
			CreatorName: d.CreatorName,
			CreatedAt:   d.CreatedAt,
		}
	}
	var roots []*dto.DocDirNode
	for _, n := range nodes {
		if p, ok := nodes[n.ParentId]; ok && n.ParentId != "" {
			p.Children = append(p.Children, n)
		} else {
			roots = append(roots, n)
		}
	}
	var subtree func(n *dto.DocDirNode) int // 子树文档总数（含自身直属）
	subtree = func(n *dto.DocDirNode) int {
		total := n.DocCount
		for _, c := range n.Children {
			total += subtree(c)
		}
		n.TotalCount = total
		return total
	}
	for _, r := range roots {
		_ = subtree(r)
	}
	sortDirNodes(roots)
	return roots, nil
}

// Create 新建目录（父 ”=根；同级同名拒绝）。visibility/readJSON/writeJSON =
// 创建即配授权（语义同 DocService.SaveGrants）。非-nil 身份须对父目录 canWrite。
func (s *DocDirService) Create(ctx context.Context, parentId, name, userId string,
	visibility uint, readJSON, writeJSON string, t *model.Identity) (*entity.DocDir, error) {
	name = sanitizeName(name, 128)
	if name == "" {
		return nil, errors.New("目录名不能为空")
	}
	if visibility > 2 {
		return nil, errors.New("visibility 取值 0|1|2")
	}
	readRows, err := model.ParseGrantRows(readJSON)
	if err != nil {
		return nil, errors.New("读条件：" + err.Error())
	}
	writeRows, err := model.ParseGrantRows(writeJSON)
	if err != nil {
		return nil, errors.New("写条件：" + err.Error())
	}
	if parentId != "" {
		p, err := s.repo.Get(ctx, parentId)
		if err != nil {
			return nil, errors.New("父目录不存在")
		}
		if isSpaceRootKind(p.Kind) {
			return nil, errors.New("空间根目录下的目录由系统按项目/员工自动创建")
		}
		if t != nil {
			v, err := buildDocPermView(ctx, s.repo, s.grantRepo, *t)
			if err != nil {
				return nil, err
			}
			if !v.canWriteDir(*t, p) {
				return nil, errors.New("没有操作权限")
			}
		}
	}
	if _, err := s.repo.GetByName(ctx, parentId, name); err == nil {
		return nil, errors.New("同级已存在同名目录")
	}
	d := &entity.DocDir{Name: name, ParentId: parentId, CreatedBy: userId, Visibility: visibility}
	if err := s.repo.Add(ctx, d); err != nil {
		return nil, err
	}
	if len(readRows) > 0 {
		_ = s.grantRepo.Replace(ctx, "dir", d.DirId, "read", readRows, userId)
	}
	if len(writeRows) > 0 {
		_ = s.grantRepo.Replace(ctx, "dir", d.DirId, "write", writeRows, userId)
	}
	return d, nil
}

// Rename 重命名（同级同名拒绝；非-nil 身份须 canWrite）
func (s *DocDirService) Rename(ctx context.Context, dirId, name string, t *model.Identity) error {
	name = sanitizeName(name, 128)
	if name == "" {
		return errors.New("目录名不能为空")
	}
	d, err := s.repo.Get(ctx, dirId)
	if err != nil {
		return errors.New("目录不存在")
	}
	if d.Kind != "" {
		return errors.New("系统托管目录由同步管理，不支持重命名")
	}
	if err := s.assertDirWrite(ctx, d, t); err != nil {
		return err
	}
	if d.Name == name {
		return nil
	}
	if _, err := s.repo.GetByName(ctx, d.ParentId, name); err == nil {
		return errors.New("同级已存在同名目录")
	}
	return s.repo.Rename(ctx, dirId, name)
}

// Move 移动目录（新父 ”=根；不得移入自身或自己的子树）。非-nil 身份：
// 自身 canWrite + 新父 canWrite（在别人目录下建枝=写新父）。
func (s *DocDirService) Move(ctx context.Context, dirId, parentId string, t *model.Identity) error {
	if dirId == "" {
		return errors.New("dirId 不能为空")
	}
	d, err := s.repo.Get(ctx, dirId)
	if err != nil {
		return errors.New("目录不存在")
	}
	if d.Kind != "" {
		return errors.New("系统托管目录不支持移动")
	}
	if err := s.assertDirWrite(ctx, d, t); err != nil {
		return err
	}
	if parentId == dirId {
		return errors.New("不能移入自身")
	}
	if parentId != "" {
		p, err := s.repo.Get(ctx, parentId)
		if err != nil {
			return errors.New("目标目录不存在")
		}
		if isSpaceRootKind(p.Kind) {
			return errors.New("不能移入空间根目录")
		}
		if err := s.assertDirWrite(ctx, p, t); err != nil {
			return err
		}
		// 沿新父的祖先链向上走，撞到自身 = 移入了自己的子树
		cur := parentId
		for cur != "" {
			if cur == dirId {
				return errors.New("不能移入自己的子目录")
			}
			p, err := s.repo.Get(ctx, cur)
			if err != nil {
				break
			}
			cur = p.ParentId
		}
	}
	return s.repo.Move(ctx, dirId, parentId)
}

// Delete 删除目录（须无子目录且无文档——内容先清空/移走；非-nil 身份须 canWrite）
func (s *DocDirService) Delete(ctx context.Context, dirId string, t *model.Identity) error {
	if dirId == "" {
		return errors.New("dirId 不能为空")
	}
	d, err := s.repo.Get(ctx, dirId)
	if err != nil {
		return errors.New("目录不存在")
	}
	if d.Kind != "" {
		return errors.New("系统托管目录由同步管理，不支持删除")
	}
	if err := s.assertDirWrite(ctx, d, t); err != nil {
		return err
	}
	if n, err := s.repo.CountChildren(ctx, dirId); err == nil && n > 0 {
		return errors.New("目录下还有子目录，先清空再删除")
	}
	if _, total, err := s.docRepo.List(ctx, "", "", []string{dirId}, 1, 1); err == nil && total > 0 {
		return errors.New("目录下还有文档，先删除或移走")
	}
	if err := s.repo.Delete(ctx, dirId); err != nil {
		return err
	}
	_ = s.grantRepo.DeleteByTargets(ctx, "dir", []string{dirId}) // 授权行随之清理（尽力而为）
	return nil
}

// assertDirWrite 目录写校验（nil 身份放行）
func (s *DocDirService) assertDirWrite(ctx context.Context, d *entity.DocDir, t *model.Identity) error {
	if t == nil {
		return nil
	}
	v, err := buildDocPermView(ctx, s.repo, s.grantRepo, *t)
	if err != nil {
		return err
	}
	if !v.canWriteDir(*t, d) {
		return errors.New("没有操作权限")
	}
	return nil
}

// enrichCreators 回填创建人姓名（跨库读 aiarsenal.aa_user）
func (s *DocDirService) enrichCreators(ctx context.Context, dirs []*entity.DocDir) {
	if s.userRepo == nil {
		return
	}
	ids := make([]string, 0, len(dirs))
	seen := map[string]struct{}{}
	for _, d := range dirs {
		if d.CreatedBy == "" {
			continue
		}
		if _, ok := seen[d.CreatedBy]; !ok {
			seen[d.CreatedBy] = struct{}{}
			ids = append(ids, d.CreatedBy)
		}
	}
	if len(ids) == 0 {
		return
	}
	users, err := s.userRepo.GetUsersByIds(ctx, ids)
	if err != nil {
		return
	}
	m := make(map[string]string, len(users))
	for _, u := range users {
		m[u.Id] = u.Name
	}
	for _, d := range dirs {
		if n, ok := m[d.CreatedBy]; ok {
			d.CreatorName = n
		}
	}
}

func sortDirNodes(nodes []*dto.DocDirNode) {
	// 固定根置顶（项目空间→个人空间→普通按名），其余同级按名
	rank := func(n *dto.DocDirNode) int {
		switch n.Kind {
		case DocDirKindProjectRoot:
			return 0
		case DocDirKindPersonalRoot:
			return 1
		default:
			return 2
		}
	}
	less := func(a, b *dto.DocDirNode) bool {
		if ra, rb := rank(a), rank(b); ra != rb {
			return ra < rb
		}
		return a.Name < b.Name
	}
	for i := 1; i < len(nodes); i++ {
		for j := i; j > 0 && less(nodes[j], nodes[j-1]); j-- {
			nodes[j-1], nodes[j] = nodes[j], nodes[j-1]
		}
	}
	for _, n := range nodes {
		sortDirNodes(n.Children)
	}
}

// isSpaceRootKind 是否两个固定空间根（子目录由 DocSpaceService 托管）
func isSpaceRootKind(kind string) bool {
	return kind == DocDirKindProjectRoot || kind == DocDirKindPersonalRoot
}
