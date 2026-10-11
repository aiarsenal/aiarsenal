package market

import "context"

// MarketAdapter 第三方 skill 市场适配器接口。
// 每个第三方平台实现一份（如讯飞技能市场），把各自的原生 API 归一成统一契约。
// 新增市场 = 新增一个实现 + 在 main.go 装配时注入 Registry，核心同步链路无需改动。
type MarketAdapter interface {
	// Info 返回市场源元信息。
	Info() MarketSource

	// Search 搜索 skill。page 为 0-based 页码，limit 为每页条数。
	Search(ctx context.Context, q string, page, limit int) (items []MarketSkill, total int, err error)

	// DetailURL 返回某 skill 在原站的链接（兜底用，「原站打开」按钮）。
	DetailURL(slug string) string

	// Download 下载某 skill 指定版本的压缩包字节流（讯飞实测为 zip）。
	// version 为空时取 latest。返回包字节、建议文件名（可空）、错误。
	Download(ctx context.Context, slug, version string) (raw []byte, filename string, err error)
}
