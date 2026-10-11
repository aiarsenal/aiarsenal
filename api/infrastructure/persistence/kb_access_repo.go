package persistence

import (
	"context"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/db"
)

type KbAccessLogRepository struct{ db *gorm.DB }

func NewKbAccessLogRepository() *KbAccessLogRepository {
	return &KbAccessLogRepository{db: db.KbDB()}
}

func (r *KbAccessLogRepository) Add(ctx context.Context, l *entity.KbAccessLog) error {
	// 裸 insert（时间戳走 now()）：记录是高频小写入，也不必回读
	return r.db.WithContext(ctx).Exec(`
insert into aa_kb_access_log (doc_id, user_id, action, keyword, hits, source, created_at)
values (?, ?, ?, ?, ?, ?, now())`,
		l.DocId, l.UserId, l.Action, l.Keyword, l.Hits, l.Source).Error
}

// List 分页（action/docId 可选过滤，时间倒序）。doc_title 直接 join 回填（同库同事务视图）。
func (r *KbAccessLogRepository) List(ctx context.Context, action, docId string, pageNum, pageSize int) ([]*entity.KbAccessLog, int64, error) {
	where, args := "1=1", []interface{}{}
	if action != "" {
		where += " and l.action = ?"
		args = append(args, action)
	}
	if docId != "" {
		where += " and l.doc_id = ?"
		args = append(args, docId)
	}
	var total int64
	if err := r.db.WithContext(ctx).
		Raw("select count(*) from aa_kb_access_log l where "+where, args...).
		Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	var rows []struct {
		Id        int64
		DocId     string
		UserId    string
		Action    string
		Keyword   string
		Hits      int
		Source    string
		CreatedAt types.Time
		DocTitle  *string
	}
	if err := r.db.WithContext(ctx).
		Raw(`select l.id, l.doc_id, l.user_id, l.action, l.keyword, l.hits, l.source, l.created_at,
		           coalesce(d.title, '') as doc_title
		    from aa_kb_access_log l
		    left join aa_kb_document d on d.doc_id = l.doc_id
		    where `+where+`
		    order by l.id desc
		    limit ? offset ?`, append(args, pageSize, (pageNum-1)*pageSize)...).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	list := make([]*entity.KbAccessLog, 0, len(rows))
	for _, row := range rows {
		l := &entity.KbAccessLog{
			Id: uint(row.Id), DocId: row.DocId, UserId: row.UserId,
			Action: row.Action, Keyword: row.Keyword, Hits: row.Hits,
			Source: row.Source, CreatedAt: &row.CreatedAt,
		}
		if row.DocTitle != nil {
			l.DocTitle = *row.DocTitle
		}
		list = append(list, l)
	}
	return list, total, nil
}
