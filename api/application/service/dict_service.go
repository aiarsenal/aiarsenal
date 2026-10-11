package service

import (
	"context"

	"aiarsenal/domain/model"
)

// DictService 字典数据（硬编码，不再依赖 aa_dict_type / aa_dict_data 表）
//
// 本系统不再需要"字典管理"功能（CRUD 已移除），但若干页面（用户性别、菜单显隐、
// 登录/操作日志状态等）仍通过 /dict/data/getDictData 拉取下拉选项数据，故将这些
// 选项直接固化为代码常量。
type DictService struct{}

func NewDictService() *DictService { return &DictService{} }

// dictData 内置字典数据：dictType -> 选项列表
// 选项字段与 model.DictDataRes 对应：key=dictValue, value=dictLabel, isDefault。
var dictData = map[string][]*model.DictDataRes{
	"sys_user_sex": {
		{DictValue: "1", DictLabel: "男", IsDefault: 0},
		{DictValue: "2", DictLabel: "女", IsDefault: 0},
		{DictValue: "0", DictLabel: "保密", IsDefault: 1},
	},
	"sys_show_hide": {
		{DictValue: "0", DictLabel: "显示", IsDefault: 1},
		{DictValue: "1", DictLabel: "隐藏", IsDefault: 0},
	},
	"sys_yes_no": {
		{DictValue: "1", DictLabel: "是", IsDefault: 1},
		{DictValue: "0", DictLabel: "否", IsDefault: 0},
	},
	"admin_login_status": {
		{DictValue: "1", DictLabel: "成功", IsDefault: 0},
		{DictValue: "0", DictLabel: "失败", IsDefault: 0},
	},
	"sys_oper_log_type": {
		{DictValue: "GET", DictLabel: "读取", IsDefault: 0},
		{DictValue: "POST", DictLabel: "新增", IsDefault: 0},
		{DictValue: "PUT", DictLabel: "修改", IsDefault: 0},
		{DictValue: "DELETE", DictLabel: "删除", IsDefault: 0},
	},
}

// dictTypeInfo 内置字典类型元信息：dictType -> {name, remark}
var dictTypeInfo = map[string]*model.DictTypeRes{
	"sys_user_sex":       {DictName: "用户性别", Remark: "用于选择用户性别"},
	"sys_show_hide":      {DictName: "菜单状态", Remark: "菜单显隐"},
	"sys_yes_no":         {DictName: "系统内置", Remark: "是/否"},
	"admin_login_status": {DictName: "管理员登录状态", Remark: "登录成功/失败"},
	"sys_oper_log_type":  {DictName: "操作日志类型", Remark: "请求方法"},
}

// GetDictData 按 dictType 返回字典（前端 getDictData：{info, values}）
func (s *DictService) GetDictData(ctx context.Context, dtype string) (*model.DictTypeRes, []*model.DictDataRes, error) {
	info := dictTypeInfo[dtype]
	if info == nil {
		info = &model.DictTypeRes{}
	}
	values, ok := dictData[dtype]
	if !ok {
		values = []*model.DictDataRes{}
	}
	// 返回副本，避免调用方修改内置数据
	out := make([]*model.DictDataRes, len(values))
	for i, v := range values {
		cp := *v
		out[i] = &cp
	}
	return info, out, nil
}
