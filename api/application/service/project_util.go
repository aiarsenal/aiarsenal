package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
)

// 项目管理模块服务层公用助手。

var timeLayouts = []string{
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

// parseTime 把前端日期字符串解析为 *types.Time；空串返回 nil。
func parseTime(s string) *types.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, lay := range timeLayouts {
		if t, err := time.Parse(lay, s); err == nil {
			v := types.Time(t)
			return &v
		}
	}
	return nil
}

// tagsJSON 把字符串数组序列化为 jsonb 文本；空数组返回 "[]"。
func tagsJSON(arr []string) string {
	if len(arr) == 0 {
		return "[]"
	}
	b, err := json.Marshal(arr)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// userNameMap 跨库批量取姓名（aa_project → aiarsenal.aa_user）；失败返回空表，展示回落 userId。
func userNameMap(ctx context.Context, userRepo repository.IUserRepository, ids []string) map[string]string {
	m := make(map[string]string, len(ids))
	if userRepo == nil || len(ids) == 0 {
		return m
	}
	users, err := userRepo.GetUsersByIds(ctx, ids)
	if err != nil {
		return m
	}
	for _, u := range users {
		if u.Name != "" {
			m[u.Id] = u.Name
		} else {
			m[u.Id] = u.UserName
		}
	}
	return m
}

// enrichTaskAssignees 回填任务负责人姓名（task/board 列表共用）。
func enrichTaskAssignees(ctx context.Context, userRepo repository.IUserRepository, tasks []*entity.ProjectTask) {
	ids := make([]string, 0, len(tasks))
	seen := map[string]bool{}
	for _, t := range tasks {
		if t.AssigneeId != "" && !seen[t.AssigneeId] {
			seen[t.AssigneeId] = true
			ids = append(ids, t.AssigneeId)
		}
	}
	m := userNameMap(ctx, userRepo, ids)
	for _, t := range tasks {
		if n, ok := m[t.AssigneeId]; ok {
			t.AssigneeName = n
		}
	}
}
