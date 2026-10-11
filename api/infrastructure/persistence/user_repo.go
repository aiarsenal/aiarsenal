package persistence

import (
	"context"
	"crypto/rand"
	"log"
	"log/slog"
	"time"

	"github.com/mssola/user_agent"
	"gorm.io/gorm"

	"aiarsenal/consts"
	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/infrastructure/auth"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

type UserRepository struct{ db *gorm.DB }

func NewUserRepository() *UserRepository { return &UserRepository{db: db.DB()} }

// ===== 登录相关 =====

func (r *UserRepository) GetUserByUsername(ctx context.Context, userName string) (*model.LoginUserRes, error) {
	var u entity.User
	err := r.db.WithContext(ctx).
		Select("user_id, user_name, employee_no, name, user_nickname, user_password, user_salt, user_status, is_admin, avatar, dept_id").
		Where("user_name = ?", userName).First(&u).Error
	if err != nil {
		return nil, err
	}
	return entityToLogin(&u), nil
}

func (r *UserRepository) GetUserById(ctx context.Context, userId string) (*model.LoginUserRes, error) {
	var u entity.User
	err := r.db.WithContext(ctx).
		Select("user_id, user_name, employee_no, name, user_nickname, user_password, user_salt, user_status, is_admin, avatar, dept_id").
		Where("user_id = ?", userId).First(&u).Error
	if err != nil {
		return nil, err
	}
	return entityToLogin(&u), nil
}

func (r *UserRepository) UpdateLoginInfo(ctx context.Context, userId string, ip string) error {
	return r.db.WithContext(ctx).Model(&entity.User{}).Unscoped().
		Where("user_id = ?", userId).Updates(map[string]interface{}{
		"last_login_ip":   ip,
		"last_login_time": time.Now(),
	}).Error
}

func (r *UserRepository) InsertLoginLog(ctx context.Context, params *model.LoginLogParams) {
	ua := user_agent.New(params.UserAgent)
	browser, _ := ua.Browser()
	// os 列 varchar(50)：此前误存完整 UserAgent，浏览器 UA 超长致 INSERT 整行失败
	//（SQLSTATE 22001，浏览器登录记录全丢）。与 InsertUserOnline 一致改存解析后的 OS。
	_ = r.db.WithContext(ctx).Table("aa_login_log").Create(map[string]interface{}{
		"info_id": idgen.New(), "login_name": params.Username, "ipaddr": params.Ip, "browser": browser,
		"os": ua.OS(), "status": params.Status, "msg": params.Msg,
		"login_time": time.Now(), "module": params.Module,
	}).Error
}

func (r *UserRepository) InsertUserOnline(ctx context.Context, params *model.UserOnlineParams) {
	ua := user_agent.New(params.UserAgent)
	browser, _ := ua.Browser()
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("insert user online panic: %v", rec)
		}
	}()
	_ = r.db.WithContext(ctx).Table("aa_user_online").Create(map[string]interface{}{
		"uuid": params.Uuid, "token": params.Token, "create_time": time.Now(),
		"user_name": params.Username, "ip": params.Ip, "explorer": browser, "os": ua.OS(),
	}).Error
}

// ===== 用户管理 =====

func (r *UserRepository) GetEntityById(ctx context.Context, userId string) (*entity.User, error) {
	var u entity.User
	err := r.db.WithContext(ctx).
		Select("*", "user_password", "user_salt").
		Where("user_id = ?", userId).First(&u).Error
	if err != nil {
		return nil, err
	}
	u.UserPassword = ""
	u.UserSalt = ""
	return &u, nil
}

func (r *UserRepository) List(ctx context.Context, deptId, mobile, status, keyWords string, pageNum, pageSize int) (list []*entity.User, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.User{})
	if keyWords != "" {
		kw := "%" + keyWords + "%"
		// 姓名 / 工号 / 手机号 / 邮箱 四字段 OR（与其他人员搜索保持一致）
		q = q.Where("name like ? or employee_no like ? or mobile like ? or user_email like ?", kw, kw, kw, kw)
	}
	if deptId != "" {
		// 含子部门：dept_id 命中，或在祖先链中包含该 dept_id
		q = q.Where("dept_id = ? OR dept_id in (select dept_id from aa_dept where ancestors like ?)", deptId, "%,"+deptId+",%")
	}
	if status != "" {
		q = q.Where("user_status = ?", status)
	}
	if mobile != "" {
		q = q.Where("mobile like ?", "%"+mobile+"%")
	}
	if err = q.Count(&total).Error; err != nil {
		slog.Error("user 列表计数失败", "keyWords", keyWords, "deptId", deptId, "mobile", mobile, "status", status, "err", err)
		return
	}
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	err = q.Select("user_id, user_name, employee_no, name, mobile, user_nickname, birthday, user_status, user_email, sex, avatar, dept_id, remark, is_admin, address, describe, last_login_ip, last_login_time, created_at, updated_at").
		Order("id asc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error
	// 过滤诊断：status 为前端传入字符串（""/"0"/"1"），日志可见筛选条件是否生效、结果数。
	slog.Debug("user 列表查询", "keyWords", keyWords, "deptId", deptId, "mobile", mobile, "status", status, "pageNum", pageNum, "pageSize", pageSize, "total", total, "returned", len(list), "err", err)
	return
}

// EnrichUsers 给用户列表补充部门、角色信息（对应旧 GetUsersRoleDept，Post 留空与旧一致）
func (r *UserRepository) EnrichUsers(ctx context.Context, users []*entity.User) ([]*model.UserRoleDeptRes, error) {
	var allRoles []*entity.Role
	if err := r.db.WithContext(ctx).Find(&allRoles).Error; err != nil {
		return nil, err
	}
	var allDepts []*entity.Dept
	if err := r.db.WithContext(ctx).Find(&allDepts).Error; err != nil {
		return nil, err
	}
	e, err := auth.Enforcer()
	if err != nil {
		return nil, err
	}
	out := make([]*model.UserRoleDeptRes, 0, len(users))
	for _, u := range users {
		item := &model.UserRoleDeptRes{User: u}
		for _, d := range allDepts {
			if u.DeptId == d.DeptId {
				item.Dept = &model.UserRoleInfoRes{RoleId: d.DeptId, Name: d.DeptName}
			}
		}
		gp, _ := e.GetFilteredGroupingPolicy(0, consts.CasBinUserPrefix+u.UserId)
		for _, v := range gp {
			if len(v) >= 2 {
				rid := v[1]
				for _, role := range allRoles {
					if role.RoleId == rid {
						item.RoleInfo = append(item.RoleInfo, &model.UserRoleInfoRes{RoleId: role.RoleId, Name: role.Name})
					}
				}
			}
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *UserRepository) Add(ctx context.Context, u *entity.User, roleIds, postIds []string) (string, error) {
	u.UserId = idgen.New()
	if err := r.db.WithContext(ctx).Create(u).Error; err != nil {
		return "", err
	}
	if err := r.replaceUserRoles(ctx, u.UserId, roleIds); err != nil {
		return u.UserId, err
	}
	if err := r.replaceUserPosts(ctx, u.UserId, postIds); err != nil {
		return u.UserId, err
	}
	return u.UserId, nil
}

func (r *UserRepository) Edit(ctx context.Context, u *entity.User, roleIds, postIds []string) error {
	if err := r.db.WithContext(ctx).Model(&entity.User{}).Where("user_id = ?", u.UserId).Updates(u).Error; err != nil {
		return err
	}
	if err := r.replaceUserRoles(ctx, u.UserId, roleIds); err != nil {
		return err
	}
	if err := r.replaceUserPosts(ctx, u.UserId, postIds); err != nil {
		return err
	}
	return nil
}

func (r *UserRepository) Delete(ctx context.Context, userIds []string) error {
	if len(userIds) == 0 {
		return nil
	}
	e, err := auth.Enforcer()
	if err == nil {
		for _, id := range userIds {
			_, _ = e.RemoveFilteredGroupingPolicy(0, consts.CasBinUserPrefix+id)
		}
	}
	if err := r.db.WithContext(ctx).Where("user_id in ?", userIds).Delete(&entity.UserPost{}).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Where("user_id in ?", userIds).Delete(&entity.User{}).Error
}

func (r *UserRepository) ResetPwd(ctx context.Context, userId string, pwd, salt string) error {
	return r.db.WithContext(ctx).Model(&entity.User{}).Where("user_id = ?", userId).
		Updates(map[string]interface{}{"user_password": pwd, "user_salt": salt}).Error
}

func (r *UserRepository) ChangeStatus(ctx context.Context, userId string, status uint) error {
	return r.db.WithContext(ctx).Model(&entity.User{}).Where("user_id = ?", userId).
		Update("user_status", status).Error
}

func (r *UserRepository) GetEditUser(ctx context.Context, userId string) (user *entity.User, roleIds []string, postIds []string, err error) {
	user, err = r.GetEntityById(ctx, userId)
	if err != nil {
		return
	}
	roleIds = r.userRoleIds(userId)
	postIds = r.userPostIds(userId)
	return
}

func (r *UserRepository) GetUsersByIds(ctx context.Context, userIds []string) ([]*model.UserSimpleRes, error) {
	var users []*entity.User
	if err := r.db.WithContext(ctx).Select("user_id, avatar, sex, user_name, user_nickname, name, employee_no").
		Where("user_id in ?", userIds).Find(&users).Error; err != nil {
		return nil, err
	}
	out := make([]*model.UserSimpleRes, 0, len(users))
	for _, u := range users {
		out = append(out, &model.UserSimpleRes{
			Id: u.UserId, Avatar: u.Avatar, Sex: u.Sex, UserName: u.UserName, UserNickname: u.UserNickname,
			Name: u.Name, EmployeeNo: u.EmployeeNo,
		})
	}
	return out, nil
}

func (r *UserRepository) ReplaceUserRoles(ctx context.Context, userId string, roleIds []string) error {
	return r.replaceUserRoles(ctx, userId, roleIds)
}

// GetUsersByEmpNos 按工号批量取姓名+部门完整层级路径（员工集合成员明细）。
func (r *UserRepository) GetUsersByEmpNos(ctx context.Context, empNos []string) ([]*model.MemberInfoRes, error) {
	if len(empNos) == 0 {
		return []*model.MemberInfoRes{}, nil
	}
	var rows []*model.MemberInfoRes
	if err := r.db.WithContext(ctx).Model(&entity.User{}).
		Where("employee_no in ?", empNos).
		Select("employee_no, name, dept_id").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	paths := r.deptPaths(ctx)
	for _, m := range rows {
		m.DeptName = paths[m.DeptId]
	}
	return rows, nil
}

// --- 内部 ---

func (r *UserRepository) replaceUserRoles(ctx context.Context, userId string, roleIds []string) error {
	e, err := auth.Enforcer()
	if err != nil {
		return err
	}
	key := consts.CasBinUserPrefix + userId
	_, _ = e.RemoveFilteredGroupingPolicy(0, key)
	for _, rid := range roleIds {
		if _, err := e.AddGroupingPolicy(key, rid); err != nil {
			return err
		}
	}
	return nil
}

func (r *UserRepository) replaceUserPosts(ctx context.Context, userId string, postIds []string) error {
	if err := r.db.WithContext(ctx).Where("user_id = ?", userId).Delete(&entity.UserPost{}).Error; err != nil {
		return err
	}
	if len(postIds) == 0 {
		return nil
	}
	rows := make([]*entity.UserPost, 0, len(postIds))
	for _, pid := range postIds {
		rows = append(rows, &entity.UserPost{UserId: userId, PostId: pid})
	}
	return r.db.WithContext(ctx).Create(&rows).Error
}

func (r *UserRepository) userRoleIds(userId string) []string {
	e, err := auth.Enforcer()
	if err != nil {
		return nil
	}
	gp, _ := e.GetFilteredGroupingPolicy(0, consts.CasBinUserPrefix+userId)
	out := []string{}
	for _, v := range gp {
		if len(v) >= 2 {
			out = append(out, v[1])
		}
	}
	return out
}

func (r *UserRepository) userPostIds(userId string) []string {
	var rows []*entity.UserPost
	r.db.WithContext(context.Background()).Where("user_id = ?", userId).Find(&rows)
	out := make([]string, 0, len(rows))
	for _, p := range rows {
		out = append(out, p.PostId)
	}
	return out
}

// GetDeptAndPosts 权限身份的部门/岗位/工号三维（登录签发 token 时取）。
func (r *UserRepository) GetDeptAndPosts(ctx context.Context, userId string) (string, []string, string, error) {
	var u entity.User
	if err := r.db.WithContext(ctx).Select("dept_id", "employee_no").Where("user_id = ?", userId).First(&u).Error; err != nil {
		return "", nil, "", err
	}
	return u.DeptId, r.userPostIds(userId), u.EmployeeNo, nil
}

// ListAdminIds 全部超管 user_id（部门无负责人时临时预算审批的兜底通知对象）。
func (r *UserRepository) ListAdminIds(ctx context.Context) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&entity.User{}).
		Where("is_admin = 1 and deleted_at is null").
		Order("id asc").Pluck("user_id", &ids).Error
	return ids, err
}

// SearchOptions 人员选择器搜索：name / employee_no / mobile / user_email 四字段 OR，
// 仅返回在用账号（user_status=1 且未软删），投影为 UserOptionRes（部门为完整层级路径）。
func (r *UserRepository) SearchOptions(ctx context.Context, keyWords string, pageNum, pageSize int) (list []*model.UserOptionRes, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.User{}).
		Where("user_status = 1 and deleted_at is null")
	if keyWords != "" {
		kw := "%" + keyWords + "%"
		// 整组 OR 必须括起来：否则 gorm 不加括号，按 SQL 优先级(AND>OR)会变成
		// (status=1 AND deleted is null AND name like) OR employee_no like OR mobile like OR user_email like，
		// 后三字段绕过启用/未删过滤，结果与 Count 语义都错乱。
		q = q.Where("(name like ? or employee_no like ? or mobile like ? or user_email like ?)", kw, kw, kw, kw)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	err = q.Select("user_id as id, name, employee_no, mobile, user_email, dept_id").
		Order("id asc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error
	if err != nil {
		return
	}
	r.fillDeptPath(ctx, list)
	return
}

// GetUserOptionsByIds 按用户 id 批量取 UserOptionRes（授权条件行回显已选用户姓名/工号/部门；
// 不做在用过滤——已配置进条件的账号停用后仍需可读回显）。
func (r *UserRepository) GetUserOptionsByIds(ctx context.Context, userIds []string) ([]*model.UserOptionRes, error) {
	if len(userIds) == 0 {
		return []*model.UserOptionRes{}, nil
	}
	var list []*model.UserOptionRes
	if err := r.db.WithContext(ctx).Model(&entity.User{}).
		Where("user_id in ?", userIds).
		Select("user_id as id, name, employee_no, mobile, user_email, dept_id").
		Find(&list).Error; err != nil {
		return nil, err
	}
	r.fillDeptPath(ctx, list)
	return list, nil
}

// fillDeptPath 回填部门完整层级路径「根 / … / 末级」（SearchOptions/GetUserOptionsByIds 共用）。
// 部门是稳定小字典：整表一次取，Go 侧沿 parent_id 上溯拼接（memo 防重复递归，环止于当前级）；
// 查失败不挡主流程（留空串）。
func (r *UserRepository) fillDeptPath(ctx context.Context, list []*model.UserOptionRes) {
	paths := r.deptPaths(ctx)
	for _, u := range list {
		u.DeptName = paths[u.DeptId]
	}
}

func (r *UserRepository) deptPaths(ctx context.Context) map[string]string {
	type deptRow struct {
		DeptId   string
		ParentId string
		DeptName string
	}
	var rows []deptRow
	if err := r.db.WithContext(ctx).Table("aa_dept").
		Select("dept_id, parent_id, dept_name").Find(&rows).Error; err != nil {
		return map[string]string{}
	}
	byId := make(map[string]deptRow, len(rows))
	for _, d := range rows {
		byId[d.DeptId] = d
	}
	memo := make(map[string]string, len(rows))
	var path func(id string, visiting map[string]bool) string
	path = func(id string, visiting map[string]bool) string {
		if v, ok := memo[id]; ok {
			return v
		}
		if visiting[id] { // 脏数据成环：止于当前级，不再上溯
			return byId[id].DeptName
		}
		visiting[id] = true
		d := byId[id]
		s := d.DeptName
		if p, ok := byId[d.ParentId]; ok && p.DeptId != d.DeptId {
			s = path(d.ParentId, visiting) + " / " + s
		}
		visiting[id] = false
		memo[id] = s
		return s
	}
	out := make(map[string]string, len(rows))
	for _, d := range rows {
		out[d.DeptId] = path(d.DeptId, map[string]bool{})
	}
	return out
}

// ===== 辅助 =====

func entityToLogin(u *entity.User) *model.LoginUserRes {
	return &model.LoginUserRes{
		Id:           u.UserId,
		UserName:     u.UserName,
		EmployeeNo:   u.EmployeeNo,
		Name:         u.Name,
		UserNickname: u.UserNickname,
		UserPassword: u.UserPassword,
		UserSalt:     u.UserSalt,
		UserStatus:   u.UserStatus,
		IsAdmin:      u.IsAdmin,
		Avatar:       u.Avatar,
		DeptId:       u.DeptId,
	}
}

// randomSalt 生成 10 位随机盐
func randomSalt(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}

// GenerateSalt 对外暴露（供 application 层生成新盐）
func GenerateSalt() string { return randomSalt(10) }
