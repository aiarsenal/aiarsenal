package entity

import "aiarsenal/domain/types"

// Menu 对应表 aa_menu（菜单/权限规则）。Id 为内部 bigint 自增主键；
// MenuId 为 32 位业务 id（对外 = id），Pid 引用父菜单的 menu_id。
type Menu struct {
	Id         uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	MenuId     string      `gorm:"column:menu_id;uniqueIndex" json:"id"`
	Pid        string      `gorm:"column:pid" json:"pid"`
	Name       string      `gorm:"column:name" json:"name"`
	Title      string      `gorm:"column:title" json:"title"`
	EnName     string      `gorm:"column:en_name" json:"enName"`
	Icon       string      `gorm:"column:icon" json:"icon"`
	Condition  string      `gorm:"column:condition" json:"condition"`
	Remark     string      `gorm:"column:remark" json:"remark"`
	MenuType   uint        `gorm:"column:menu_type" json:"menuType"`
	Weigh      int         `gorm:"column:weigh" json:"weigh"`
	IsHide     uint        `gorm:"column:is_hide" json:"isHide"`
	Path       string      `gorm:"column:path" json:"path"`
	Component  string      `gorm:"column:component" json:"component"`
	IsLink     uint        `gorm:"column:is_link" json:"isLink"`
	ModuleType string      `gorm:"column:module_type" json:"moduleType"`
	ModelId    uint        `gorm:"column:model_id" json:"modelId"`
	IsIframe   uint        `gorm:"column:is_iframe" json:"isIframe"`
	IsCached   uint        `gorm:"column:is_cached" json:"isCached"`
	Redirect   string      `gorm:"column:redirect" json:"redirect"`
	IsAffix    uint        `gorm:"column:is_affix" json:"isAffix"`
	LinkUrl    string      `gorm:"column:link_url" json:"linkUrl"`
	GrantRules string      `gorm:"column:grant_rules" json:"grantRules"` // 组合授权条件行 JSON（[]model.GrantRow）
	CreatedAt  *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt  *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (Menu) TableName() string { return "aa_menu" }
