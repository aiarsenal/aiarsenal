package entity

// CasbinRule 对应表 aa_rule（casbin 策略：ptype + v0..v5）
type CasbinRule struct {
	Ptype string `gorm:"column:ptype" json:"ptype"`
	V0    string `gorm:"column:v0" json:"v0"`
	V1    string `gorm:"column:v1" json:"v1"`
	V2    string `gorm:"column:v2" json:"v2"`
	V3    string `gorm:"column:v3" json:"v3"`
	V4    string `gorm:"column:v4" json:"v4"`
	V5    string `gorm:"column:v5" json:"v5"`
}

func (CasbinRule) TableName() string { return "aa_rule" }
