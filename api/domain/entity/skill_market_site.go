package entity

import "aiarsenal/domain/types"

// SkillMarketSite 对应表 aa_skill_market_site（第三方 skill 市场目录，aa_skill 库）。
// 只是「外部市场链接目录」：引导用户去原站登录搜索、拿到安装引用后回平台走导入安装
// （安全扫描→通知→安装）；平台不从这些站点代搜/代抓（可同步的讯飞源走代码内适配器，
// 见 infrastructure/market）。
// MarketId 为业务 id；CreatedBy / UpdatedBy 引用 aa_user.user_id。
type SkillMarketSite struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	MarketId    string      `gorm:"column:market_id;uniqueIndex" json:"marketId"`
	Name        string      `gorm:"column:name" json:"name"`        // 展示名（如「腾讯技能社区」）
	SiteUrl     string      `gorm:"column:site_url" json:"siteUrl"` // 市场站点链接（点卡片跳转）
	Description string      `gorm:"column:description" json:"description"`
	Icon        string      `gorm:"column:icon" json:"icon"`              // emoji 兜底图标
	LogoUrl     string      `gorm:"column:logo_url" json:"logoUrl"`       // logo 图片 URL（空则用 Icon）
	ThemeColor  string      `gorm:"column:theme_color" json:"themeColor"` // 卡片主题色
	Status      uint        `gorm:"column:status" json:"status"`          // 1=显示 0=隐藏
	Sort        int         `gorm:"column:sort" json:"sort"`              // 排序（小在前）
	CreatedBy   string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt   *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedBy   string      `gorm:"column:updated_by" json:"updatedBy"`
	UpdatedAt   *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (SkillMarketSite) TableName() string { return "aa_skill_market_site" }
