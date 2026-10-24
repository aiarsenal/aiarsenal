package service

import (
	"context"

	"aiarsenal/consts"
	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/auth"
	"aiarsenal/infrastructure/persistence"
)

type PersonalService struct {
	userRepo repository.IUserRepository
	deptRepo repository.IDeptRepository
	roleRepo repository.IRoleRepository
}

func NewPersonalService(u repository.IUserRepository, d repository.IDeptRepository, r repository.IRoleRepository) *PersonalService {
	return &PersonalService{userRepo: u, deptRepo: d, roleRepo: r}
}

// Info 登录用户信息：user + roles(名称) + deptName
func (s *PersonalService) Info(ctx context.Context, userId string) (*entity.User, []string, string, error) {
	user, err := s.userRepo.GetEntityById(ctx, userId)
	if err != nil {
		return nil, nil, "", err
	}
	// 角色名
	roles := []string{}
	allRoles, _ := s.roleRepo.All(ctx)
	e, _ := auth.Enforcer()
	if e != nil {
		gp, _ := e.GetFilteredGroupingPolicy(0, consts.CasBinUserPrefix+userId)
		for _, v := range gp {
			if len(v) >= 2 {
				rid := v[1]
				for _, r := range allRoles {
					if r.RoleId == rid {
						roles = append(roles, r.Name)
					}
				}
			}
		}
	}
	// 部门名
	deptName := ""
	if user.DeptId != "" {
		if d, derr := s.deptRepo.Get(ctx, user.DeptId); derr == nil {
			deptName = d.DeptName
		}
	}
	return user, roles, deptName, nil
}

// Edit 修改个人资料，返回 (userInfo, newToken)
func (s *PersonalService) Edit(ctx context.Context, userId string, p *entity.User) (*entity.User, string, error) {
	p.UserId = userId
	// 自助改资料不得触碰 is_admin：setAdmin 恒 nil（仓储不写该列）。
	if err := s.userRepo.Edit(ctx, p, nil, nil, nil); err != nil {
		return nil, "", err
	}
	user, err := s.userRepo.GetEntityById(ctx, userId)
	if err != nil {
		return nil, "", err
	}
	deptId, postIds, employeeNo, _ := s.userRepo.GetDeptAndPosts(ctx, userId)
	token, err := auth.Generate(userId, user.UserName, user.IsAdmin, deptId, postIds, employeeNo)
	if err != nil {
		return user, "", nil
	}
	return user, token, nil
}

// ResetPwd 重置自己的密码
func (s *PersonalService) ResetPwd(ctx context.Context, userId string, password string) error {
	salt := persistence.GenerateSalt()
	return s.userRepo.ResetPwd(ctx, userId, auth.EncryptPassword(password, salt), salt)
}
