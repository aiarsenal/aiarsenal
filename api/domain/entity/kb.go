package entity

import "aiarsenal/domain/types"

// KbDocument 对应表 aa_kb_document（知识库管理主表）。
// DocId 为业务 id；CreatedBy 引用 aa_user.user_id（跨库软引用）；
// ContentText 为抽取清洗后的全文，向量在 aa_kb_chunk 分块表。
type KbDocument struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	DocId       string      `gorm:"column:doc_id;uniqueIndex" json:"docId"`
	Title       string      `gorm:"column:title" json:"title"`
	Summary     string      `gorm:"column:summary" json:"summary"`
	FileName    string      `gorm:"column:file_name" json:"fileName"`
	FileType    string      `gorm:"column:file_type" json:"fileType"`
	FileSize    int64       `gorm:"column:file_size" json:"fileSize"`
	FileHash    string      `gorm:"column:file_hash" json:"-"`
	ContentText string      `gorm:"column:content_text" json:"-"`
	CharCount   int         `gorm:"column:char_count" json:"charCount"`
	ChunkCount  int         `gorm:"column:chunk_count" json:"chunkCount"`
	EmbedStatus uint        `gorm:"column:embed_status" json:"embedStatus"` // 0待嵌入 1已嵌入 2失败
	EmbedError  string      `gorm:"column:embed_error" json:"embedError"`   // 嵌入失败原因 / 空抽取说明
	Source      string      `gorm:"column:source" json:"source"`            // ui/mcp
	StoragePath string      `gorm:"column:storage_path" json:"-"`
	DirId       string      `gorm:"column:dir_id" json:"dirId"`          // 所属目录（''=根）
	Visibility  uint        `gorm:"column:visibility" json:"visibility"` // 0私有 1按条件 2全员
	CreatedBy   string      `gorm:"column:created_by" json:"createdBy"`
	CreatorName string      `gorm:"-" json:"creatorName"` // 回填：上传人姓名（aa_user.name）
	CanWrite    bool        `gorm:"-" json:"canWrite"`    // 回填：当前身份可写（编辑/更新/删除/移动/重嵌）
	CreatedAt   *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (KbDocument) TableName() string { return "aa_kb_document" }

// KbChunk 对应表 aa_kb_chunk（文档分块 + 语义向量）。
type KbChunk struct {
	Id             uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	DocId          string      `gorm:"column:doc_id" json:"docId"`
	Seq            int         `gorm:"column:seq" json:"seq"`
	Content        string      `gorm:"column:content" json:"content"`
	SemanticVector string      `gorm:"column:semantic_vector;->" json:"-"` // pgvector 文本形，只写不读
	CreatedAt      *types.Time `gorm:"column:created_at" json:"-"`
}

func (KbChunk) TableName() string { return "aa_kb_chunk" }

// KbDir 对应表 aa_kb_dir（目录树）。ParentId ”=根；同级同名由唯一索引 (parent_id, name) 兜底。
// Kind/RefId 托管目录（项目空间/个人空间）：kind=project→ref_id=project_id、
// kind=personal→ref_id=user_id，部分唯一索引 (kind, ref_id) WHERE ref_id<>'' 兜底至多一条；
// normal（存量行 '' 同义）为普通目录，ref_id 恒空。
type KbDir struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	DirId       string      `gorm:"column:dir_id;uniqueIndex" json:"dirId"`
	Name        string      `gorm:"column:name" json:"name"`
	ParentId    string      `gorm:"column:parent_id" json:"parentId"`
	Visibility  uint        `gorm:"column:visibility" json:"visibility"` // 0私有 1按条件 2全员
	Kind        string      `gorm:"column:kind" json:"kind"`             // normal(存量''同义) project_space/personal_space=固定根 project/personal=系统托管
	RefId       string      `gorm:"column:ref_id" json:"refId"`          // kind=project→project_id；kind=personal→user_id；normal 空
	CreatedBy   string      `gorm:"column:created_by" json:"-"`
	CanWrite    bool        `gorm:"-" json:"canWrite"`    // 回填：当前身份可写（改名/删除/配授权）
	CreatorName string      `gorm:"-" json:"creatorName"` // 回填：创建人姓名（aa_user.name）
	CreatedAt   *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (KbDir) TableName() string { return "aa_kb_dir" }

// KbGrant 对应表 aa_kb_grant（文档/目录组合授权条件行，与 aa_menu.grant_rules 同构）。
// 条件维度语义同 model.GrantRow（六维任意组合、行内 AND，多行 OR，exclude 取反）；
// jsonb 列以字符串形态存取（JSON 数组文本）。owner 与超管判定层天然全权，不落行。
type KbGrant struct {
	Id         uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	TargetType string      `gorm:"column:target_type" json:"targetType"` // doc | dir
	TargetId   string      `gorm:"column:target_id" json:"targetId"`
	Action     string      `gorm:"column:action" json:"action"`    // read | write（写授予蕴含读）
	UserIds    string      `gorm:"column:user_ids" json:"userIds"` // jsonb 数组文本
	DeptId     string      `gorm:"column:dept_id" json:"deptId"`
	RoleIds    string      `gorm:"column:role_ids" json:"roleIds"` // jsonb 数组文本
	PostIds    string      `gorm:"column:post_ids" json:"postIds"` // jsonb 数组文本
	Exclude    bool        `gorm:"column:exclude" json:"exclude"`
	GroupId    string      `gorm:"column:group_id" json:"groupId"` // 权限分组引用（行内 AND 组合维度）
	SetId      string      `gorm:"column:set_id" json:"setId"`     // 员工集合引用
	CreatedBy  string      `gorm:"column:created_by" json:"-"`
	CreatedAt  *types.Time `gorm:"column:created_at" json:"createdAt"`
}

func (KbGrant) TableName() string { return "aa_kb_grant" }

// KbAccessLog 对应表 aa_kb_access_log（访问记录：谁检索/阅读过）。
// 检索：DocId=”、Keyword=检索词、Hits=命中数；阅读：DocId=目标、Keyword=”。
// UserId ”=MCP（agent）侧访问；文档内容绝不入此表。
type KbAccessLog struct {
	Id        uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	DocId     string      `gorm:"column:doc_id" json:"docId"`
	UserId    string      `gorm:"column:user_id" json:"-"`
	UserName  string      `gorm:"-" json:"userName"`           // 回填（aa_user.name；''=MCP）
	DocTitle  string      `gorm:"-" json:"docTitle"`           // 回填（join aa_kb_document）
	Action    string      `gorm:"column:action" json:"action"` // search | read
	Keyword   string      `gorm:"column:keyword" json:"keyword"`
	Hits      int         `gorm:"column:hits" json:"hits"`
	Source    string      `gorm:"column:source" json:"source"` // ui | mcp
	CreatedAt *types.Time `gorm:"column:created_at" json:"createdAt"`
}

func (KbAccessLog) TableName() string { return "aa_kb_access_log" }
