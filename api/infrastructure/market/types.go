package market

// MarketSource 描述一个第三方 skill 市场源。
type MarketSource struct {
	Key        string `json:"key"`        // 唯一标识，如 "xfyun"
	Name       string `json:"name"`       // 展示名
	Icon       string `json:"icon"`       // emoji（菜单/兜底图标）
	Logo       string `json:"logo"`       // logo 图片 URL（页面展示；空则用 Icon）
	ThemeColor string `json:"themeColor"` // 主题色（卡片强调色）
	SiteURL    string `json:"siteUrl"`    // 官网/市场首页（「来源跳官网」用）
	Desc       string `json:"desc"`       // 简短描述
	Syncable   bool   `json:"syncable"`   // 是否支持站内搜索+一键同步
}

// MarketSkill 第三方市场中的单个 skill（适配器把各源原生结构归一成此结构）。
type MarketSkill struct {
	Slug        string `json:"slug"`        // 唯一标识（用于下载/同步）
	DisplayName string `json:"displayName"` // 展示名
	Summary     string `json:"summary"`     // 简介
	Version     string `json:"version"`     // 最新版本
	UpdatedAt   int64  `json:"updatedAt"`   // 更新时间（毫秒时间戳，0 表示未知）
	DetailURL   string `json:"detailUrl"`   // 原站详情链接（「原站」按钮）
}
