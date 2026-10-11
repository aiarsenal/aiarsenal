package market

import "sync"

// Registry 市场注册表：可搜索+同步的市场适配器（实现 MarketAdapter）。
// 「仅原站入口」的外部市场不再走注册表——收录到 aa_skill_market_site 目录
// （页面可增删改查，种子见 skills.sql），引导用户去原站搜索后回来导入。
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]MarketAdapter
}

func NewRegistry() *Registry {
	return &Registry{adapters: make(map[string]MarketAdapter)}
}

// Register 注册一个可搜索+同步的适配器。
func (r *Registry) Register(a MarketAdapter) {
	if a == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[a.Info().Key] = a
}

// List 返回全部可搜索+同步的市场源（Syncable=true）。
func (r *Registry) List() []MarketSource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]MarketSource, 0, len(r.adapters))
	for _, a := range r.adapters {
		src := a.Info()
		src.Syncable = true
		out = append(out, src)
	}
	return out
}

// Get 取指定 key 的适配器。
func (r *Registry) Get(key string) (MarketAdapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.adapters[key]
	return a, ok
}
