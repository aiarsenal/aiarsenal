package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
)

// SkillMarketSiteService 第三方 skill 市场目录（aa_skill_market_site）CRUD。
// 目录只存外部市场链接（名称/地址/描述/图标/配色/排序/启停），平台不代搜不代抓——
// 引导用户去原站登录搜索、拿到安装引用（链接或 @作者/标识）后回平台走导入安装
// （安全扫描→结论通知→安装，复用既有链路）。
type SkillMarketSiteService struct {
	repo repository.ISkillMarketSiteRepository
}

func NewSkillMarketSiteService(r repository.ISkillMarketSiteRepository) *SkillMarketSiteService {
	return &SkillMarketSiteService{repo: r}
}

// MarketSiteListReq 列表查询（controller 绑定）。
type MarketSiteListReq struct {
	Keyword  string `json:"keyword" form:"keyword"` // 名称/地址模糊
	Status   string `json:"status" form:"status"`   // "0"/"1"，空=全部
	PageNum  int    `json:"pageNum" form:"pageNum"`
	PageSize int    `json:"pageSize" form:"pageSize"`
}

// MarketSiteReq 新建/编辑请求（controller 绑定；Edit 时 MarketId 必填）。
type MarketSiteReq struct {
	MarketId    string `json:"marketId"` // 编辑必填
	Name        string `json:"name"`     // 必填
	SiteUrl     string `json:"siteUrl"`  // 必填，http(s)
	Description string `json:"description"`
	Icon        string `json:"icon"`       // emoji 兜底
	LogoUrl     string `json:"logoUrl"`    // logo 图片 URL
	ThemeColor  string `json:"themeColor"` // 卡片主题色（#rrggbb）
	Status      uint   `json:"status"`     // 1=显示 0=隐藏
	Sort        int    `json:"sort"`
}

func (s *SkillMarketSiteService) List(ctx context.Context, req *MarketSiteListReq) ([]*entity.SkillMarketSite, int64, error) {
	return s.repo.List(ctx, strings.TrimSpace(req.Keyword), req.Status, req.PageNum, req.PageSize)
}

// All 启用中的目录（页面卡片/引导展示用，按 sort 排序）。
func (s *SkillMarketSiteService) All(ctx context.Context) ([]*entity.SkillMarketSite, error) {
	return s.repo.All(ctx)
}

func (s *SkillMarketSiteService) Get(ctx context.Context, marketId string) (*entity.SkillMarketSite, error) {
	return s.repo.Get(ctx, marketId)
}

func (s *SkillMarketSiteService) Add(ctx context.Context, req *MarketSiteReq, userId string) (string, error) {
	e, err := marketSiteFromReq(req)
	if err != nil {
		return "", err
	}
	e.CreatedBy = userId
	now := types.Time(time.Now())
	e.CreatedAt = &now
	return s.repo.Add(ctx, e)
}

func (s *SkillMarketSiteService) Edit(ctx context.Context, req *MarketSiteReq, userId string) error {
	if req == nil || req.MarketId == "" {
		return errors.New("marketId 不能为空")
	}
	e, err := marketSiteFromReq(req)
	if err != nil {
		return err
	}
	e.MarketId = req.MarketId
	e.UpdatedBy = userId
	now := types.Time(time.Now())
	e.UpdatedAt = &now
	return s.repo.Edit(ctx, e)
}

func (s *SkillMarketSiteService) Delete(ctx context.Context, marketId string) error {
	if marketId == "" {
		return errors.New("marketId 不能为空")
	}
	return s.repo.Delete(ctx, marketId)
}

// marketSiteFromReq 请求校验 + 落实体（新建/编辑共用；时间/操作人由调用方补）。
func marketSiteFromReq(req *MarketSiteReq) (*entity.SkillMarketSite, error) {
	if req == nil {
		return nil, errors.New("请求不能为空")
	}
	req.Name = strings.TrimSpace(req.Name)
	req.SiteUrl = strings.TrimSpace(req.SiteUrl)
	if req.Name == "" {
		return nil, errors.New("市场名称不能为空")
	}
	if len([]rune(req.Name)) > 50 {
		return nil, errors.New("市场名称过长（≤50 字）")
	}
	if req.SiteUrl == "" {
		return nil, errors.New("市场链接不能为空")
	}
	u, err := url.Parse(req.SiteUrl)
	if err != nil || u.Scheme == "" || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("市场链接非法（应为 http(s) 地址）: %s", req.SiteUrl)
	}
	if len([]rune(req.Description)) > 200 {
		return nil, errors.New("描述过长（≤200 字）")
	}
	if req.Status != 0 {
		req.Status = 1
	}
	return &entity.SkillMarketSite{
		Name: req.Name, SiteUrl: req.SiteUrl, Description: strings.TrimSpace(req.Description),
		Icon: strings.TrimSpace(req.Icon), LogoUrl: strings.TrimSpace(req.LogoUrl),
		ThemeColor: strings.TrimSpace(req.ThemeColor), Status: req.Status, Sort: req.Sort,
	}, nil
}
