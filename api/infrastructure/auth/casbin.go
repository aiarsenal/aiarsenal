package auth

import (
	"context"
	"fmt"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	"gorm.io/gorm"

	"aiarsenal/config"
	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/db"
)

// CasbinRule 对应表 aa_rule（ptype, v0..v5），与旧 entity.CasbinRule 一致
// 实体定义在 domain/entity/casbin_rule.go

var (
	casbinOnce     sync.Once
	casbinEnforcer *casbin.SyncedEnforcer
	casbinErr      error
)

// gormAdapter 用 GORM 读写 aa_rule 表，实现 casbin persist.Adapter
type gormAdapter struct {
	db *gorm.DB
}

func newAdapter() *gormAdapter {
	return &gormAdapter{db: db.DB()}
}

func (a *gormAdapter) LoadPolicy(m model.Model) error {
	var lines []entity.CasbinRule
	if err := a.db.WithContext(context.Background()).Table("aa_rule").Find(&lines).Error; err != nil {
		return err
	}
	for _, line := range lines {
		loadPolicyLine(&line, m)
	}
	return nil
}

func (a *gormAdapter) SavePolicy(m model.Model) error {
	// 全量落库：清空 aa_rule 后按 p/g 断言重写（casbin SavePolicy 语义），事务保证不留半截。
	return a.db.WithContext(context.Background()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM aa_rule").Error; err != nil {
			return err
		}
		for _, section := range []string{"p", "g"} {
			for ptype, ast := range m[section] {
				for _, rule := range ast.Policy {
					if err := tx.Create(savePolicyLine(ptype, rule)).Error; err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
}

func (a *gormAdapter) AddPolicy(sec string, ptype string, rule []string) error {
	line := savePolicyLine(ptype, rule)
	return a.db.Table("aa_rule").Create(line).Error
}

func (a *gormAdapter) RemovePolicy(sec string, ptype string, rule []string) error {
	line := savePolicyLine(ptype, rule)
	tx := a.db.Table("aa_rule").Where("ptype = ?", line.Ptype)
	if line.V0 != "" {
		tx = tx.Where("v0 = ?", line.V0)
	}
	if line.V1 != "" {
		tx = tx.Where("v1 = ?", line.V1)
	}
	if line.V2 != "" {
		tx = tx.Where("v2 = ?", line.V2)
	}
	return tx.Delete(nil).Error
}

func (a *gormAdapter) RemoveFilteredPolicy(sec string, ptype string, fieldIndex int, fieldValues ...string) error {
	tx := a.db.Table("aa_rule").Where("ptype = ?", ptype)
	fields := []string{"v0", "v1", "v2", "v3", "v4", "v5"}
	for i, v := range fieldValues {
		idx := fieldIndex + i
		if idx >= len(fields) {
			break
		}
		if v != "" {
			tx = tx.Where(fields[idx]+" = ?", v)
		}
	}
	return tx.Delete(nil).Error
}

func loadPolicyLine(line *entity.CasbinRule, m model.Model) {
	lineText := line.Ptype
	if line.V0 != "" {
		lineText += ", " + line.V0
	}
	if line.V1 != "" {
		lineText += ", " + line.V1
	}
	if line.V2 != "" {
		lineText += ", " + line.V2
	}
	if line.V3 != "" {
		lineText += ", " + line.V3
	}
	if line.V4 != "" {
		lineText += ", " + line.V4
	}
	if line.V5 != "" {
		lineText += ", " + line.V5
	}
	persist.LoadPolicyLine(lineText, m)
}

func savePolicyLine(ptype string, rule []string) *entity.CasbinRule {
	line := &entity.CasbinRule{}
	line.Ptype = ptype
	if len(rule) > 0 {
		line.V0 = rule[0]
	}
	if len(rule) > 1 {
		line.V1 = rule[1]
	}
	if len(rule) > 2 {
		line.V2 = rule[2]
	}
	if len(rule) > 3 {
		line.V3 = rule[3]
	}
	if len(rule) > 4 {
		line.V4 = rule[4]
	}
	if len(rule) > 5 {
		line.V5 = rule[5]
	}
	return line
}

// Enforcer casbin 单例（与旧实现一致：模型文件 + aa_rule adapter）
func Enforcer() (*casbin.SyncedEnforcer, error) {
	casbinOnce.Do(func() {
		c := config.Get()
		m, err := model.NewModelFromFile(c.Casbin.ModelFile)
		if err != nil {
			casbinErr = fmt.Errorf("casbin model: %w", err)
			return
		}
		e, err := casbin.NewSyncedEnforcer(m, newAdapter())
		if err != nil {
			casbinErr = fmt.Errorf("casbin enforcer: %w", err)
			return
		}
		if err := e.LoadPolicy(); err != nil {
			casbinErr = fmt.Errorf("casbin load policy: %w", err)
			return
		}
		casbinEnforcer = e
	})
	return casbinEnforcer, casbinErr
}
