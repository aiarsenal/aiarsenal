package auth

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"aiarsenal/config"
)

// password_policy.go — 口令强度策略（等保 GB/T 22239-2019 身份鉴别 a 项：鉴别信息
// 复杂度）。三档由部署在 config.yaml 定（password.policy.level），后端强制，报错
// 聚合并逐条给出所缺项：
//
//	loose    ≥8 位
//	standard ≥8 位 + 大写/小写/数字/特殊字符至少三类 + 不含账号名 + 不在弱口令黑名单（等保基线，默认）
//	strict   ≥12 位 + 四类全含 + 不含账号名 + 不在弱口令黑名单
//
// 手写校验器的理由见 PR 正文：等保条目与档位一一映射、报错文案可控；熵类库
// （go-password-validator/zxcvbn-go 均停更/归档）映射不出「差哪一类字符」这种条文。

//go:embed password_blacklist.txt
var passwordBlacklistRaw string

// passwordBlacklist 弱口令黑名单（内嵌 SecLists Pwdb_top-1000，来源与许可见文件头）。
// 启动载入一次；比较统一小写精确匹配——子串匹配会误伤含常见词的正常口令。
var passwordBlacklist = buildBlacklist()

func buildBlacklist() map[string]struct{} {
	m := make(map[string]struct{})
	for _, line := range strings.Split(passwordBlacklistRaw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m[strings.ToLower(line)] = struct{}{}
	}
	return m
}

// passwordMaxLength 口令长度上限：超长口令会让 PBKDF2 的单次校验成本变成可被请求
// 方操纵的 CPU 放大面；128 字符对人类口令绰绰有余。
const passwordMaxLength = 128

// ValidatePasswordPolicy 按当前档位校验口令，聚合返回所有不满足项。username 为该
// 口令所属账号的账号名（管理端代重置时传目标用户的，自助改密传本人的）。
func ValidatePasswordPolicy(password, username string) error {
	if len(password) > passwordMaxLength {
		return fmt.Errorf("密码长度不能超过 %d 个字符", passwordMaxLength)
	}
	level := config.Get().Password.Policy.Level
	minLen, minClasses := 8, 3
	if level == "strict" {
		minLen, minClasses = 12, 4
	}

	var problems []string
	runes := []rune(password)
	if len(runes) < minLen {
		problems = append(problems, fmt.Sprintf("长度至少 %d 位", minLen))
	}

	hasUpper, hasLower, hasDigit, hasSpecial := false, false, false, false
	for _, r := range runes {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		default:
			hasSpecial = true // 非字母数字（符号、空格等）按特殊字符计
		}
	}
	classes := 0
	for _, ok := range []bool{hasUpper, hasLower, hasDigit, hasSpecial} {
		if ok {
			classes++
		}
	}
	// loose 只看长度（档位语义：给内网低敏场景一个不卡复杂度的退路）
	if level == "loose" {
		if len(problems) == 0 {
			return nil
		}
		return errors.New("密码强度不足：" + strings.Join(problems, "；"))
	}
	if classes < minClasses {
		problems = append(problems,
			fmt.Sprintf("需包含大写字母/小写字母/数字/特殊字符中至少 %d 类（当前 %d 类）", minClasses, classes))
	}
	if u := strings.ToLower(strings.TrimSpace(username)); u != "" &&
		strings.Contains(strings.ToLower(password), u) {
		problems = append(problems, "不能包含账号名")
	}
	if _, hit := passwordBlacklist[strings.ToLower(password)]; hit {
		problems = append(problems, "过于常见，已被列入弱口令黑名单")
	}

	if len(problems) == 0 {
		return nil
	}
	return errors.New("密码强度不足：" + strings.Join(problems, "；"))
}
