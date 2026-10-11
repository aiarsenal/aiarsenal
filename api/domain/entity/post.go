package entity

import "aiarsenal/domain/types"

// Post 对应表 aa_post。Id 为内部 bigint 自增主键；
// PostId 为 32 位业务 id（对外 = postId），CreatedBy/UpdatedBy 引用 aa_user.user_id。
type Post struct {
	Id        uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	PostId    string      `gorm:"column:post_id;uniqueIndex" json:"postId"`
	PostCode  string      `gorm:"column:post_code" json:"postCode"`
	PostName  string      `gorm:"column:post_name" json:"postName"`
	PostSort  int         `gorm:"column:post_sort" json:"postSort"`
	Status    uint        `gorm:"column:status" json:"status"`
	Remark    string      `gorm:"column:remark" json:"remark"`
	CreatedBy string      `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (Post) TableName() string { return "aa_post" }
