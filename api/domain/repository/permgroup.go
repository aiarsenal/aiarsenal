package repository

import (
	"context"

	"aiarsenal/domain/entity"
)

// IPermGroupRepository 权限分组（aa_perm_group）
type IPermGroupRepository interface {
	List(ctx context.Context, name string) ([]*entity.PermGroup, error)
	GetById(ctx context.Context, groupId string) (*entity.PermGroup, error)
	// NameExists 名称精确重名检查（excludeId 非空时排除自身，编辑场景）
	NameExists(ctx context.Context, name, excludeId string) (bool, error)
	// TermsLike 引用扫描：terms 文本包含 id 的其他分组（删除保护用；含自身，调用方排除）
	TermsLike(ctx context.Context, id string) ([]*entity.PermGroup, error)
	Add(ctx context.Context, g *entity.PermGroup) (string, error)
	Update(ctx context.Context, g *entity.PermGroup) error
	Delete(ctx context.Context, groupIds []string) error
}

// IEmpSetRepository 员工集合（aa_emp_set）
type IEmpSetRepository interface {
	List(ctx context.Context, name string) ([]*entity.EmpSet, error)
	GetById(ctx context.Context, setId string) (*entity.EmpSet, error)
	// NameExists 名称精确重名检查（excludeId 非空时排除自身，编辑场景）
	NameExists(ctx context.Context, name, excludeId string) (bool, error)
	// TermsLike 引用扫描：权限分组 terms 文本包含集合 id 的分组（删除保护用）
	TermsLike(ctx context.Context, id string) ([]*entity.PermGroup, error)
	Add(ctx context.Context, s *entity.EmpSet) (string, error)
	Update(ctx context.Context, s *entity.EmpSet) error
	Delete(ctx context.Context, setIds []string) error
}
