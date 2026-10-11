package persistence

import (
	"context"
	"errors"
	"log/slog"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

// SkillMarketSiteRepository 第三方市场目录（aa_skill_market_site，aa_skill 库）。
type SkillMarketSiteRepository struct{ db *gorm.DB }

func NewSkillMarketSiteRepository() *SkillMarketSiteRepository {
	return &SkillMarketSiteRepository{db: db.SkillDB()}
}

func (r *SkillMarketSiteRepository) List(ctx context.Context, keyword, status string, pageNum, pageSize int) (list []*entity.SkillMarketSite, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.SkillMarketSite{})
	if keyword != "" {
		q = q.Where("name like ? or site_url like ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err = q.Count(&total).Error; err != nil {
		slog.Error("market site 列表计数失败", "keyword", keyword, "status", status, "err", err)
		return
	}
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	err = q.Order("sort asc, id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return
}

func (r *SkillMarketSiteRepository) All(ctx context.Context) (list []*entity.SkillMarketSite, err error) {
	err = r.db.WithContext(ctx).Where("status = ?", 1).Order("sort asc, id asc").Find(&list).Error
	return
}

func (r *SkillMarketSiteRepository) Get(ctx context.Context, marketId string) (*entity.SkillMarketSite, error) {
	var e entity.SkillMarketSite
	if err := r.db.WithContext(ctx).Where("market_id = ?", marketId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *SkillMarketSiteRepository) Add(ctx context.Context, m *entity.SkillMarketSite) (string, error) {
	m.MarketId = idgen.New()
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		slog.Error("market site 新建失败", "name", m.Name, "siteUrl", m.SiteUrl, "err", err)
		return "", err
	}
	return m.MarketId, nil
}

func (r *SkillMarketSiteRepository) Edit(ctx context.Context, m *entity.SkillMarketSite) error {
	// Updates 选列：只更新目录可编辑字段（MarketId/CreatedBy/CreatedAt 不被覆盖）
	return r.db.WithContext(ctx).Model(&entity.SkillMarketSite{}).
		Where("market_id = ?", m.MarketId).
		Updates(map[string]any{
			"name": m.Name, "site_url": m.SiteUrl, "description": m.Description,
			"icon": m.Icon, "logo_url": m.LogoUrl, "theme_color": m.ThemeColor,
			"status": m.Status, "sort": m.Sort, "updated_by": m.UpdatedBy, "updated_at": m.UpdatedAt,
		}).Error
}

func (r *SkillMarketSiteRepository) Delete(ctx context.Context, marketId string) error {
	res := r.db.WithContext(ctx).Where("market_id = ?", marketId).Delete(&entity.SkillMarketSite{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("市场目录项不存在")
	}
	return nil
}
