package service

import (
	"context"
	"errors"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
)

type DeptService struct{ repo repository.IDeptRepository }

func NewDeptService(r repository.IDeptRepository) *DeptService { return &DeptService{repo: r} }

func (s *DeptService) List(ctx context.Context, name, status string) ([]*model.DeptInfo, error) {
	return s.repo.List(ctx, name, status)
}

// Tree 部门树（供 dept/treeSelect 与用户列表 dept 过滤）
func (s *DeptService) Tree(ctx context.Context, name, status string) ([]*model.DeptTreeNode, error) {
	list, err := s.repo.List(ctx, name, status)
	if err != nil {
		return nil, err
	}
	return buildDeptTree(list, "0"), nil
}

func (s *DeptService) Add(ctx context.Context, d *entity.Dept) error {
	if d.DeptName == "" {
		return errors.New("部门名称不能为空")
	}
	return s.repo.Add(ctx, d)
}
func (s *DeptService) Edit(ctx context.Context, d *entity.Dept) error {
	return s.repo.Edit(ctx, d)
}
func (s *DeptService) Delete(ctx context.Context, id string) error {
	has, _ := s.repo.HasChildren(ctx, id)
	if has {
		return errors.New("存在子部门，不允许删除")
	}
	return s.repo.Delete(ctx, id)
}

// DeptNode 部门树节点（交付层用）
type DeptNode = model.DeptTreeNode

func buildDeptTree(list []*model.DeptInfo, pid string) []*model.DeptTreeNode {
	out := make([]*model.DeptTreeNode, 0)
	for _, d := range list {
		if d.ParentId == pid {
			n := &model.DeptTreeNode{DeptId: d.DeptId, ParentId: d.ParentId, DeptName: d.DeptName}
			n.Children = buildDeptTree(list, d.DeptId)
			out = append(out, n)
		}
	}
	return out
}
