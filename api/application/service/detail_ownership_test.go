package service

import "testing"

// F-017 归属谓词表测：删任一分支必有格红。
func TestCanReadNamespace(t *testing.T) {
	cases := []struct {
		name      string
		createdBy string
		caller    string
		admin     bool
		member    bool
		want      bool
	}{
		{"创建者可读", "u1", "u1", false, false, true},
		{"成员可读", "u1", "u2", false, true, true},
		{"超管可读", "u1", "u9", true, false, true},
		{"陌生不可读", "u1", "u2", false, false, false},
		{"空 caller 不可读（系统路径之外的缺省）", "u1", "", false, false, false},
	}
	for _, c := range cases {
		if got := canReadNamespace(c.createdBy, c.caller, c.admin, c.member); got != c.want {
			t.Errorf("%s: got=%v want=%v", c.name, got, c.want)
		}
	}
}

func TestCanReadSkill(t *testing.T) {
	cases := []struct {
		name   string
		owner  string
		vis    uint
		caller string
		admin  bool
		member bool
		want   bool
	}{
		{"PUBLIC 登录即可读（市场/安装流）", "u1", 0, "u2", false, false, true},
		{"PRIVATE 属主可读", "u1", 1, "u1", false, false, true},
		{"PRIVATE 成员可读", "u1", 1, "u2", false, true, true},
		{"PRIVATE 超管可读", "u1", 1, "u9", true, false, true},
		{"PRIVATE 陌生不可读", "u1", 1, "u2", false, false, false},
		{"PRIVATE 空 caller 不可读", "u1", 1, "", false, false, false},
	}
	for _, c := range cases {
		if got := canReadSkill(c.owner, c.vis, c.caller, c.admin, c.member); got != c.want {
			t.Errorf("%s: got=%v want=%v", c.name, got, c.want)
		}
	}
}

func TestCanReadMCPServer(t *testing.T) {
	cases := []struct {
		name      string
		createdBy string
		caller    string
		admin     bool
		want      bool
	}{
		{"创建者可读", "u1", "u1", false, true},
		{"超管可读", "u1", "u9", true, true},
		{"陌生不可读（配置含地址/请求头/环境变量，成员也不给）", "u1", "u2", false, false},
		{"空 createdBy 陌生不可读", "", "u2", false, false},
	}
	for _, c := range cases {
		if got := canReadMCPServer(c.createdBy, c.caller, c.admin); got != c.want {
			t.Errorf("%s: got=%v want=%v", c.name, got, c.want)
		}
	}
}
