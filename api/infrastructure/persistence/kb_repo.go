package persistence

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

type KbRepository struct{ db *gorm.DB }

func NewKbRepository() *KbRepository { return &KbRepository{db: db.KbDB()} }

// SetVisibility 文档可见性档位（0私有 1按条件 2全员）
func (r *KbRepository) SetVisibility(ctx context.Context, docId string, visibility uint) error {
	return r.db.WithContext(ctx).Model(&entity.KbDocument{}).
		Where("doc_id = ?", docId).
		Updates(map[string]interface{}{"visibility": visibility, "updated_at": gorm.Expr("now()")}).Error
}

// kbReadPredicate 组合授权读谓词（SQL 形态；与 service/doc_access.go 的 canReadDoc 逐条对齐）：
//
//	(created_by=uid ∨ visibility=2 ∨ ∃(read|write)条件行命中)
//
// 行命中 = 各维度「空=不限 ∨ 命中」全 AND（含分组/集合引用门：t.Groups/t.Sets 已由
// grantmeta 实时解析，引用行下推为 g.group_id ∈ t.Groups 的精确判定，SQL 即权威、无 Go 复核），
// 再与 exclude 异或（<>）。
// jsonb 存在性判定用函数形 jsonb_exists/jsonb_exists_any——操作符形 ? 会被 gorm 当占位符吃掉。
// 身份维度为空（无部门/角色/岗位/用户）时对应维度只允许「不限」行命中，与 Go Hit 一致。
func kbReadPredicate(alias string, t model.Identity) (string, []interface{}) {
	args := make([]interface{}, 0, 10)
	userArm := "g.user_ids = '[]'"
	if t.UserId != "" {
		userArm = "(g.user_ids = '[]' or jsonb_exists(g.user_ids, ?))"
		args = append(args, t.UserId)
	}
	deptArm := "g.dept_id = ''"
	if t.DeptId != "" {
		deptArm = "(g.dept_id = '' or g.dept_id = ?)"
		args = append(args, t.DeptId)
	}
	roleArm := "g.role_ids = '[]'"
	if len(t.RoleIds) > 0 {
		ph := make([]string, 0, len(t.RoleIds))
		for _, id := range t.RoleIds {
			ph = append(ph, "?")
			args = append(args, id)
		}
		roleArm = "(g.role_ids = '[]' or jsonb_exists_any(g.role_ids, array[" + strings.Join(ph, ",") + "]))"
	}
	postArm := "g.post_ids = '[]'"
	if len(t.PostIds) > 0 {
		ph := make([]string, 0, len(t.PostIds))
		for _, id := range t.PostIds {
			ph = append(ph, "?")
			args = append(args, id)
		}
		postArm = "(g.post_ids = '[]' or jsonb_exists_any(g.post_ids, array[" + strings.Join(ph, ",") + "]))"
	}
	groupGate := "g.group_id = ''"
	if len(t.Groups) > 0 {
		ph := make([]string, 0, len(t.Groups))
		for _, id := range t.Groups {
			ph = append(ph, "?")
			args = append(args, id)
		}
		groupGate = "(g.group_id = '' or g.group_id in (" + strings.Join(ph, ",") + "))"
	}
	setGate := "g.set_id = ''"
	if len(t.Sets) > 0 {
		ph := make([]string, 0, len(t.Sets))
		for _, id := range t.Sets {
			ph = append(ph, "?")
			args = append(args, id)
		}
		setGate = "(g.set_id = '' or g.set_id in (" + strings.Join(ph, ",") + "))"
	}
	hit := "(" + userArm + " and " + deptArm + " and " + roleArm + " and " + postArm +
		" and " + groupGate + " and " + setGate + ")"
	exists := fmt.Sprintf(
		"exists (select 1 from aa_kb_grant g where g.target_type = 'doc' and g.target_id = %s.doc_id "+
			"and g.action in ('read','write') and %s <> g.exclude)", alias, hit)
	cond := fmt.Sprintf("(%s.created_by = ? or %s.visibility = 2 or %s)",
		alias, alias, exists)
	args = append([]interface{}{t.UserId}, args...) // created_by 参数在最前
	return cond, args
}

// ListVisible 带身份过滤的分页列表（t nil 时等同 List）。
// readableDirIds：目录门（可读目录集；nil = 不设限——根文档恒可见）。
func (r *KbRepository) ListVisible(ctx context.Context, keyword, fileType string, dirIds []string,
	pageNum, pageSize int, t *model.Identity, readableDirIds []string) ([]*entity.KbDocument, int64, error) {
	q := r.db.WithContext(ctx).Model(&entity.KbDocument{})
	if keyword != "" {
		q = q.Where("title like ? or summary like ? or file_name like ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if fileType != "" {
		q = q.Where("file_type = ?", fileType)
	}
	if len(dirIds) > 0 {
		q = q.Where("dir_id in ?", dirIds)
	}
	if t != nil {
		cond, args := kbReadPredicate("aa_kb_document", *t)
		q = q.Where(cond, args...)
		// 目录门：文档须挂在可读目录下（''=根，恒可见）；无可读目录 = 只见根下文档
		if readableDirIds == nil {
			readableDirIds = []string{}
		}
		q = q.Where("dir_id = '' or dir_id in ?", readableDirIds)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	var list []*entity.KbDocument
	if err := q.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *KbRepository) List(ctx context.Context, keyword, fileType string, dirIds []string, pageNum, pageSize int) (list []*entity.KbDocument, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.KbDocument{})
	if keyword != "" {
		q = q.Where("title like ? or summary like ? or file_name like ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if fileType != "" {
		q = q.Where("file_type = ?", fileType)
	}
	if len(dirIds) > 0 {
		q = q.Where("dir_id in ?", dirIds)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	err = q.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return
}

func (r *KbRepository) Get(ctx context.Context, docId string) (*entity.KbDocument, error) {
	var d entity.KbDocument
	if err := r.db.WithContext(ctx).Where("doc_id = ?", docId).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *KbRepository) GetByTitle(ctx context.Context, title string) (*entity.KbDocument, error) {
	if title == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var d entity.KbDocument
	if err := r.db.WithContext(ctx).Where("title = ?", title).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *KbRepository) GetByHash(ctx context.Context, fileHash string) (*entity.KbDocument, error) {
	if fileHash == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var d entity.KbDocument
	if err := r.db.WithContext(ctx).Where("file_hash = ?", fileHash).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

// Add 建文档 + 全部分块（单事务：文档行与向量块要么都进要么都不进）
func (r *KbRepository) Add(ctx context.Context, d *entity.KbDocument, chunks []*entity.KbChunk) error {
	if d.DocId == "" {
		d.DocId = idgen.New()
	}
	// 时间戳在 Go 侧显式给：gorm 对 nil 指针字段会显式插 NULL，绕过列的 DEFAULT NOW()
	now := types.Time(time.Now())
	d.CreatedAt, d.UpdatedAt = &now, &now
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(d).Error; err != nil {
			return err
		}
		return kbInsertChunks(tx, d.DocId, chunks)
	})
}

func (r *KbRepository) Edit(ctx context.Context, d *entity.KbDocument) error {
	return r.db.WithContext(ctx).Model(&entity.KbDocument{}).
		Where("doc_id = ?", d.DocId).
		Updates(map[string]interface{}{
			"title":      d.Title,
			"summary":    d.Summary,
			"updated_at": gorm.Expr("now()"),
		}).Error
}

// ReplaceChunks 更新文档元数据 + 全量换分块（删旧块、插新块，单事务）
func (r *KbRepository) ReplaceChunks(ctx context.Context, docId string, d *entity.KbDocument, chunks []*entity.KbChunk) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&entity.KbDocument{}).Where("doc_id = ?", docId).Updates(map[string]interface{}{
			"title":        d.Title,
			"summary":      d.Summary,
			"file_name":    d.FileName,
			"file_type":    d.FileType,
			"file_size":    d.FileSize,
			"file_hash":    d.FileHash,
			"content_text": d.ContentText,
			"char_count":   d.CharCount,
			"chunk_count":  d.ChunkCount,
			"embed_status": d.EmbedStatus,
			"embed_error":  d.EmbedError,
			"storage_path": d.StoragePath,
			"updated_at":   gorm.Expr("now()"),
		}).Error; err != nil {
			return err
		}
		if err := tx.Where("doc_id = ?", docId).Delete(&entity.KbChunk{}).Error; err != nil {
			return err
		}
		return kbInsertChunks(tx, docId, chunks)
	})
}

func kbInsertChunks(tx *gorm.DB, docId string, chunks []*entity.KbChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	var sb strings.Builder
	args := make([]interface{}, 0, len(chunks)*4)
	sb.WriteString("insert into aa_kb_chunk (doc_id, seq, content, semantic_vector) values ")
	for i, c := range chunks {
		if i > 0 {
			sb.WriteString(",")
		}
		vec := "null"
		if c.SemanticVector != "" {
			vec = fmt.Sprintf("$%d::vector", len(args)+4)
		}
		sb.WriteString(fmt.Sprintf("($%d, $%d, $%d, %s)", len(args)+1, len(args)+2, len(args)+3, vec))
		args = append(args, docId, c.Seq, c.Content)
		if c.SemanticVector != "" {
			args = append(args, c.SemanticVector)
		}
	}
	return tx.Exec(sb.String(), args...).Error
}

func (r *KbRepository) Delete(ctx context.Context, docIds []string) error {
	if len(docIds) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("doc_id in ?", docIds).Delete(&entity.KbDocument{}).Error; err != nil {
			return err
		}
		return tx.Where("doc_id in ?", docIds).Delete(&entity.KbChunk{}).Error
	})
}

// SearchChunks 向量相似度检索（cosine <=>）：块打分 → 带回文档标题聚合。
// t 非-nil 时叠加组合授权读谓词 + 目录门（块内容即业务数据，检索不得越过授权）。
func (r *KbRepository) SearchChunks(ctx context.Context, queryVectorText string, limit int, t *model.Identity, readableDirIds []string) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 10
	}
	whereExtra := ""
	var args []interface{}
	if t != nil {
		cond, cargs := kbReadPredicate("d", *t)
		// 目录门：显式占位符展开（Raw 语句里 in ? 不走查询构建器的切片展开）
		if len(readableDirIds) == 0 {
			whereExtra = "and (" + cond + ") and d.dir_id = ''"
		} else {
			ph := make([]string, 0, len(readableDirIds))
			for _, id := range readableDirIds {
				ph = append(ph, "?")
				args = append(args, id)
			}
			whereExtra = "and (" + cond + ") and (d.dir_id = '' or d.dir_id in (" + strings.Join(ph, ",") + "))"
		}
		args = append(cargs, args...)
	}
	var rows []map[string]interface{}
	err := r.db.WithContext(ctx).Raw(`
		select c.doc_id as doc_id, d.title as title, d.file_name as file_name, d.file_type as file_type,
		       c.seq as seq, c.content as content,
		       1 - (c.semantic_vector <=> ?::vector) as score
		from aa_kb_chunk c
		join aa_kb_document d on d.doc_id = c.doc_id
		where c.semantic_vector is not null
		  and 1 - (c.semantic_vector <=> ?::vector) > 0.01  -- 滤零分块（标题块等无词项重叠的命中）
		  `+whereExtra+`
		order by c.semantic_vector <=> ?::vector
		limit ?`, append([]interface{}{queryVectorText, queryVectorText}, append(args, queryVectorText, limit)...)...).Scan(&rows).Error
	return rows, err
}

// ListChunkContents 拼某文档全部分块文本（按 seq 排序）
func (r *KbRepository) ListChunkContents(ctx context.Context, docId string) (string, error) {
	var chunks []string
	err := r.db.WithContext(ctx).Model(&entity.KbChunk{}).
		Where("doc_id = ?", docId).Order("seq").
		Pluck("content", &chunks).Error
	return strings.Join(chunks, "\n"), err
}

// MoveDocs 批量改所属目录（dirId ”=根）
func (r *KbRepository) MoveDocs(ctx context.Context, docIds []string, dirId string) error {
	if len(docIds) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.KbDocument{}).
		Where("doc_id in ?", docIds).
		Updates(map[string]interface{}{"dir_id": dirId, "updated_at": gorm.Expr("now()")}).Error
}

// ListByDirIds 取目录集合内全量文档（重嵌目录用；不分页）
func (r *KbRepository) ListByDirIds(ctx context.Context, dirIds []string) ([]*entity.KbDocument, error) {
	if len(dirIds) == 0 {
		return nil, nil
	}
	var list []*entity.KbDocument
	err := r.db.WithContext(ctx).
		Where("dir_id in ?", dirIds).Order("id").
		Find(&list).Error
	return list, err
}

// CountByDir 各目录直属文档数（dir_id → 数量）
func (r *KbRepository) CountByDir(ctx context.Context) (map[string]int64, error) {
	var rows []struct {
		DirId string
		N     int64
	}
	err := r.db.WithContext(ctx).Model(&entity.KbDocument{}).
		Select("dir_id, count(*) as n").
		Where("dir_id <> ''").
		Group("dir_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]int64, len(rows))
	for _, r := range rows {
		m[r.DirId] = r.N
	}
	return m, nil
}

// GetByDirAndName 目录内按 file_name 定位文档（数据源同步 upsert 用：
// file_name = 外部键指纹，同一外部条目在目录内至多一行）。
// 注：本方法（与下方 KbDirRepository.GetByKindRef、数据源仓储）落在 kb_repo.go——
// aa_kb 库仓储实现统一归集，分层结构不变。
func (r *KbRepository) GetByDirAndName(ctx context.Context, dirId, fileName string) (*entity.KbDocument, error) {
	var d entity.KbDocument
	if err := r.db.WithContext(ctx).
		Where("dir_id = ? and file_name = ?", dirId, fileName).
		First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

// GetByKindRef 托管目录反查（kind='project'/'personal' + ref_id；部分唯一索引兜底至多一条）
func (r *KbDirRepository) GetByKindRef(ctx context.Context, kind, refId string) (*entity.KbDir, error) {
	var d entity.KbDir
	if err := r.db.WithContext(ctx).Where("kind = ? and ref_id = ?", kind, refId).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

// ===== 数据源连接器（aa_kb_datasource / aa_kb_datasource_run） =====

type KbDatasourceRepository struct{ db *gorm.DB }

func NewKbDatasourceRepository() *KbDatasourceRepository {
	return &KbDatasourceRepository{db: db.KbDB()}
}

func (r *KbDatasourceRepository) Add(ctx context.Context, d *entity.KbDatasource) error {
	if d.DsId == "" {
		d.DsId = idgen.New()
	}
	now := types.Time(time.Now())
	d.CreatedAt, d.UpdatedAt = &now, &now
	return r.db.WithContext(ctx).Create(d).Error
}

// Update 全字段更新（业务字段 + updated_at）
func (r *KbDatasourceRepository) Update(ctx context.Context, d *entity.KbDatasource) error {
	return r.db.WithContext(ctx).Model(&entity.KbDatasource{}).
		Where("ds_id = ?", d.DsId).
		Updates(map[string]interface{}{
			"dir_id":     d.DirId,
			"ds_type":    d.DsType,
			"name":       d.Name,
			"config":     d.Config,
			"schedule":   d.Schedule,
			"is_enabled": d.IsEnabled,
			"updated_at": gorm.Expr("now()"),
		}).Error
}

func (r *KbDatasourceRepository) Get(ctx context.Context, dsId string) (*entity.KbDatasource, error) {
	var d entity.KbDatasource
	if err := r.db.WithContext(ctx).Where("ds_id = ?", dsId).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *KbDatasourceRepository) Delete(ctx context.Context, dsId string) error {
	return r.db.WithContext(ctx).Where("ds_id = ?", dsId).Delete(&entity.KbDatasource{}).Error
}

func (r *KbDatasourceRepository) List(ctx context.Context, keyword, dsType string, pageNum, pageSize int) (list []*entity.KbDatasource, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.KbDatasource{})
	if keyword != "" {
		q = q.Where("name like ?", "%"+keyword+"%")
	}
	if dsType != "" {
		q = q.Where("ds_type = ?", dsType)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	err = q.Order("id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return
}

// ListScheduled 启动注册用：启用且配了 cron 的数据源
func (r *KbDatasourceRepository) ListScheduled(ctx context.Context) ([]*entity.KbDatasource, error) {
	var list []*entity.KbDatasource
	err := r.db.WithContext(ctx).
		Where("is_enabled and schedule <> ''").Order("id").
		Find(&list).Error
	return list, err
}

// ListEnabledByType 检索透传用：某类型的启用数据源（external_kb）
func (r *KbDatasourceRepository) ListEnabledByType(ctx context.Context, dsType string) ([]*entity.KbDatasource, error) {
	var list []*entity.KbDatasource
	err := r.db.WithContext(ctx).
		Where("ds_type = ? and is_enabled", dsType).Order("id").
		Find(&list).Error
	return list, err
}

// SetRunState 回写最近一次执行结果（run 结束时）
func (r *KbDatasourceRepository) SetRunState(ctx context.Context, dsId string, runAt *types.Time, status string) error {
	return r.db.WithContext(ctx).Model(&entity.KbDatasource{}).
		Where("ds_id = ?", dsId).
		Updates(map[string]interface{}{
			"last_run_at":  runAt,
			"last_status":  status,
			"updated_at":   gorm.Expr("now()"),
		}).Error
}

type KbDatasourceRunRepository struct{ db *gorm.DB }

func NewKbDatasourceRunRepository() *KbDatasourceRunRepository {
	return &KbDatasourceRunRepository{db: db.KbDB()}
}

func (r *KbDatasourceRunRepository) Add(ctx context.Context, run *entity.KbDatasourceRun) error {
	if run.RunId == "" {
		run.RunId = idgen.New()
	}
	now := types.Time(time.Now())
	if run.StartedAt == nil {
		run.StartedAt = &now
	}
	return r.db.WithContext(ctx).Create(run).Error
}

// Finish 收尾：status/stats/error/finished_at 一次写齐
func (r *KbDatasourceRunRepository) Finish(ctx context.Context, runId, status, statsJSON, errMsg string, finishedAt *types.Time) error {
	return r.db.WithContext(ctx).Model(&entity.KbDatasourceRun{}).
		Where("run_id = ?", runId).
		Updates(map[string]interface{}{
			"status":      status,
			"stats":       statsJSON,
			"error":       errMsg,
			"finished_at": finishedAt,
		}).Error
}

func (r *KbDatasourceRunRepository) Get(ctx context.Context, runId string) (*entity.KbDatasourceRun, error) {
	var run entity.KbDatasourceRun
	if err := r.db.WithContext(ctx).Where("run_id = ?", runId).First(&run).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

// CountRunning 某数据源在跑的 run 数（多进程兜底防重复执行）
func (r *KbDatasourceRunRepository) CountRunning(ctx context.Context, datasourceId string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&entity.KbDatasourceRun{}).
		Where("datasource_id = ? and status = ?", datasourceId, "running").
		Count(&n).Error
	return n, err
}

// ListByDatasource 分页倒序执行记录
func (r *KbDatasourceRunRepository) ListByDatasource(ctx context.Context, datasourceId string, pageNum, pageSize int) (list []*entity.KbDatasourceRun, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.KbDatasourceRun{}).
		Where("datasource_id = ?", datasourceId)
	if err = q.Count(&total).Error; err != nil {
		return
	}
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	err = q.Order("started_at desc, id desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return
}

// DeleteByDatasource 删数据源时随清执行记录
func (r *KbDatasourceRunRepository) DeleteByDatasource(ctx context.Context, datasourceId string) error {
	return r.db.WithContext(ctx).
		Where("datasource_id = ?", datasourceId).
		Delete(&entity.KbDatasourceRun{}).Error
}
