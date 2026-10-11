package entity

// UserPost 对应表 aa_user_post（用户-岗位关联）。Id 为内部 bigint 自增主键；
// UserId/PostId 为 32 位业务 id（分别引用 aa_user.user_id / aa_post.post_id）。
type UserPost struct {
	Id     uint   `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	UserId string `gorm:"column:user_id" json:"userId"`
	PostId string `gorm:"column:post_id" json:"postId"`
}

func (UserPost) TableName() string { return "aa_user_post" }
