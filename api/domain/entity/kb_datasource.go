package entity

import "aiarsenal/domain/types"

// KbDatasource 对应表 aa_kb_datasource（知识库数据源连接器定义）。
// 一行 = 一条「外部内容 → 本库 aa_kb 文档」的同步任务：挂在某 KB 目录（DirId）下，
// 由对应类型的连接器（infrastructure/kbprovider）周期或手动拉取内容入库。
// Config 为该类型私有配置（jsonb 以字符串形态存取，同 KbGrant 的 jsonb 列口径）；
// Schedule 为 5 字段 cron 表达式（” = 仅手动触发）；last_run_at/last_status 回写最近一次执行。
type KbDatasource struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	DsId        string      `gorm:"column:ds_id;uniqueIndex" json:"dsId"`
	DirId       string      `gorm:"column:dir_id" json:"dirId"`   // 归属 KB 目录（''=根；同步产物文档落这里）
	DsType      string      `gorm:"column:ds_type" json:"dsType"` // web_crawl/database/api_pull/object_storage/external_kb
	Name        string      `gorm:"column:name" json:"name"`
	Description string      `gorm:"column:description" json:"description"` // 用途说明（列表展示）
	Config      string      `gorm:"column:config" json:"config"`           // jsonb 文本（各类型私有配置）
	Schedule    string      `gorm:"column:schedule" json:"schedule"`       // cron 表达式，''=手动
	IsEnabled   bool        `gorm:"column:is_enabled" json:"isEnabled"`
	LastRunAt   *types.Time `gorm:"column:last_run_at" json:"lastRunAt"`
	LastStatus  string      `gorm:"column:last_status" json:"lastStatus"` // running/success/failed/''（未跑过）
	CreatedBy   string      `gorm:"column:created_by" json:"-"`
	CreatedAt   *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (KbDatasource) TableName() string { return "aa_kb_datasource" }

// KbDatasourceRun 对应表 aa_kb_datasource_run（数据源同步执行记录）。
// 每次执行一行：running 起、success/failed 终；stats 记计数快照（pages/rows/objects/
// ingested/updated/unchanged…）与进度备注；error 记失败原因。
type KbDatasourceRun struct {
	Id           uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	RunId        string      `gorm:"column:run_id;uniqueIndex" json:"runId"`
	DatasourceId string      `gorm:"column:datasource_id" json:"datasourceId"` // 软引用 aa_kb_datasource.ds_id
	Status       string      `gorm:"column:status" json:"status"`              // running/success/failed
	TriggerType  string      `gorm:"column:trigger_type" json:"triggerType"`   // manual/cron（触发来源）
	Stats        string      `gorm:"column:stats" json:"stats"`                // jsonb 文本（计数快照）
	Error        string      `gorm:"column:error" json:"error"`                // 失败原因（成功为 ''）
	StartedAt    *types.Time `gorm:"column:started_at" json:"startedAt"`
	FinishedAt   *types.Time `gorm:"column:finished_at" json:"finishedAt"` // 未结束为 null
}

func (KbDatasourceRun) TableName() string { return "aa_kb_datasource_run" }
