package repository

import (
	"context"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/types"
)

// IKbDatasourceRepository 数据源连接器仓储（aa_kb 库 aa_kb_datasource 表）
type IKbDatasourceRepository interface {
	Add(ctx context.Context, d *entity.KbDatasource) error
	// Update 全字段更新（name/dir/type/config/schedule/is_enabled 随实体给）
	Update(ctx context.Context, d *entity.KbDatasource) error
	Get(ctx context.Context, dsId string) (*entity.KbDatasource, error)
	Delete(ctx context.Context, dsId string) error
	// List 分页（keyword 模糊匹配名称；dsType ''=全部类型）
	List(ctx context.Context, keyword, dsType string, pageNum, pageSize int) ([]*entity.KbDatasource, int64, error)
	// ListScheduled 启动注册用：is_enabled 且 schedule 非空
	ListScheduled(ctx context.Context) ([]*entity.KbDatasource, error)
	// ListEnabledByType 检索透传用：某类型的启用数据源（external_kb）
	ListEnabledByType(ctx context.Context, dsType string) ([]*entity.KbDatasource, error)
	// SetRunState 回写最近一次执行结果（run 结束时）
	SetRunState(ctx context.Context, dsId string, runAt *types.Time, status string) error
}

// IKbDatasourceRunRepository 数据源同步执行记录仓储（aa_kb 库 aa_kb_datasource_run 表）
type IKbDatasourceRunRepository interface {
	Add(ctx context.Context, r *entity.KbDatasourceRun) error
	// Finish 收尾：status/stats/error/finished_at 一次写齐
	Finish(ctx context.Context, runId, status, statsJSON, errMsg string, finishedAt *types.Time) error
	Get(ctx context.Context, runId string) (*entity.KbDatasourceRun, error)
	// CountRunning 某数据源在跑的 run 数（多进程兜底防重复执行）
	CountRunning(ctx context.Context, datasourceId string) (int64, error)
	// ListByDatasource 分页倒序执行记录
	ListByDatasource(ctx context.Context, datasourceId string, pageNum, pageSize int) ([]*entity.KbDatasourceRun, int64, error)
	// DeleteByDatasource 删数据源时随清执行记录
	DeleteByDatasource(ctx context.Context, datasourceId string) error
}
