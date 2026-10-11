package entity

import "aiarsenal/domain/types"

// SkillStar 对应表 aa_skill_star（收藏）。
type SkillStar struct {
	Id        uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	StarId    string      `gorm:"column:star_id;uniqueIndex" json:"starId"`
	SkillId   string      `gorm:"column:skill_id" json:"skillId"`
	UserId    string      `gorm:"column:user_id" json:"userId"`
	CreatedAt *types.Time `gorm:"column:created_at" json:"createdAt"`
}

func (SkillStar) TableName() string { return "aa_skill_star" }

// SkillRating 对应表 aa_skill_rating（评分 1~5）。
type SkillRating struct {
	Id        uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	RatingId  string      `gorm:"column:rating_id;uniqueIndex" json:"ratingId"`
	SkillId   string      `gorm:"column:skill_id" json:"skillId"`
	UserId    string      `gorm:"column:user_id" json:"userId"`
	Score     uint        `gorm:"column:score" json:"score"`
	CreatedAt *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (SkillRating) TableName() string { return "aa_skill_rating" }

// SkillSubscription 对应表 aa_skill_subscription（订阅）。
type SkillSubscription struct {
	Id        uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	SubId     string      `gorm:"column:sub_id;uniqueIndex" json:"subId"`
	SkillId   string      `gorm:"column:skill_id" json:"skillId"`
	UserId    string      `gorm:"column:user_id" json:"userId"`
	CreatedAt *types.Time `gorm:"column:created_at" json:"createdAt"`
}

func (SkillSubscription) TableName() string { return "aa_skill_subscription" }
