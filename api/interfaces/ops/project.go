package ops

// 项目域（aa_project 库）：项目管理模块的读与写。
// 读端点 platform-project（/mcp/project）、写端点 platform-project-write（/mcp/project-write）。
// 工具是 ProjectTask/Project/Board 等 service 的薄壳——与 Web UI 同一套业务逻辑
// （默认状态流、活动流记录都在 service 层，单源）；repo 的 Edit 是全量 map 更新，
// 本层的 *_update 工具一律先读旧实体、只覆盖调用方给的字段，再落写。

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/types"
)

// projActor 写操作落库的 userID：有调用方身份记真实用户，否则记 "mcp"（doc 域同款）
func projActor(ctx context.Context) string {
	if u := actorUserId(identityFromCtx(ctx)); u != "" {
		return u
	}
	return "mcp"
}

// argDate 解析日期参数（"2026-09-10" / "2026-09-10 15:04" / "2026-09-10 15:04:05"）；
// 缺省 nil，格式非法报错
func argDate(m map[string]any, k string) (*types.Time, error) {
	s, _ := m[k].(string)
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		if tt, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			t := types.Time(tt)
			return &t, nil
		}
	}
	return nil, fmt.Errorf("%s 日期格式应为 YYYY-MM-DD 或 YYYY-MM-DD HH:MM，收到：%q", k, s)
}

// argTags 逗号分隔标签 → jsonb 数组字符串；缺省 ""（由调用方决定保留旧值还是不设）
func argTags(m map[string]any, k string) string {
	s, _ := m[k].(string)
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	b, _ := json.Marshal(parts)
	return string(b)
}

func argFloat(m map[string]any, k string, def float64) float64 {
	switch v := m[k].(type) {
	case float64:
		return v
	case string:
		var f float64
		if _, err := fmt.Sscanf(v, "%g", &f); err == nil {
			return f
		}
	}
	return def
}

func projCompact(p *entity.Project) map[string]any {
	return map[string]any{
		"projectId": p.ProjectId, "name": p.Name, "code": p.Code,
		"status": p.Status, "priority": p.Priority, "progress": p.Progress,
		"owner": p.Owner, "archived": p.Archived,
		"startDate": fmtTime(p.StartDate, "2006-01-02"), "dueDate": fmtTime(p.DueDate, "2006-01-02"),
		"updatedAt": fmtTime(p.UpdatedAt, "2006-01-02 15:04"),
	}
}

func taskCompact(t *entity.ProjectTask) map[string]any {
	return map[string]any{
		"taskId": t.TaskId, "projectId": t.ProjectId, "title": t.Title,
		"status": t.Status, "priority": t.Priority,
		"assigneeId": t.AssigneeId, "reporterId": t.ReporterId,
		"milestoneId": t.MilestoneId, "parentTaskId": t.ParentTaskId,
		"dueDate":   fmtTime(t.DueDate, "2006-01-02"),
		"doneAt":    fmtTime(t.DoneAt, "2006-01-02 15:04"),
		"updatedAt": fmtTime(t.UpdatedAt, "2006-01-02 15:04"),
	}
}

// ===== 读 server（platform-project，/mcp/project） =====

func buildProjectServer(d Deps) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "aiarsenal-mcp-project", Version: "1.0"}, nil)

	opsTool(srv, "project_list", "查项目列表（真实数据）：名称模糊/状态过滤，倒序列表",
		[]toolArg{{"keyword", "名称模糊过滤", false}, {"status", "状态过滤：planning|active|paused|closed|archived", false}, {"page", "页码，默认 1", false}, {"limit", "条数 1-50，默认 10", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			list, total, err := d.Project.List(ctx, argStr(a, "keyword", ""), argStr(a, "status", ""),
				clampInt(argInt(a, "page", 1), 1, 10000), clampInt(argInt(a, "limit", 10), 1, 50))
			if err != nil {
				return "", err
			}
			items := make([]map[string]any, 0, len(list))
			for _, p := range list {
				items = append(items, projCompact(p))
			}
			return jsonOutAny(map[string]any{"total": total, "items": items})
		})

	opsTool(srv, "project_get", "查单个项目详情（真实数据）：project_id 取项目全量字段",
		[]toolArg{{"project_id", "项目 id（project_list 查得）", true}},
		func(ctx context.Context, a map[string]any) (string, error) {
			p, err := d.Project.Get(ctx, argStr(a, "project_id", ""))
			if err != nil {
				return fmt.Sprintf("未找到项目：%s", argStr(a, "project_id", "")), nil
			}
			return jsonOutAny(map[string]any{
				"projectId": p.ProjectId, "name": p.Name, "code": p.Code, "description": p.Description,
				"status": p.Status, "priority": p.Priority, "progress": p.Progress,
				"owner": p.Owner, "visibility": p.Visibility, "deptId": p.DeptId,
				"startDate": fmtTime(p.StartDate, "2006-01-02"), "dueDate": fmtTime(p.DueDate, "2006-01-02"),
				"tags": p.Tags, "archived": p.Archived,
				"createdAt": fmtTime(p.CreatedAt, "2006-01-02 15:04"),
			})
		})

	opsTool(srv, "project_overview", "查项目概览（真实数据）：项目信息 + 成员数；项目维度的第一入口",
		[]toolArg{{"project_id", "项目 id", true}},
		func(ctx context.Context, a map[string]any) (string, error) {
			out, err := d.Project.Overview(ctx, argStr(a, "project_id", ""))
			if err != nil {
				return "", err
			}
			return jsonOutAny(out)
		})

	opsTool(srv, "project_task_list", "查项目任务列表（真实数据）：按状态/负责人/优先级过滤；子任务带 parentTaskId",
		[]toolArg{{"project_id", "项目 id", true}, {"status", "状态 key 过滤（看板列 key，如 todo/doing/done）", false}, {"assignee", "负责人 user_id 过滤", false}, {"priority", "优先级：low|medium|high|urgent", false}, {"page", "页码，默认 1", false}, {"limit", "条数 1-50，默认 10", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			list, total, err := d.Task.List(ctx, argStr(a, "project_id", ""), argStr(a, "status", ""),
				argStr(a, "assignee", ""), argStr(a, "priority", ""),
				clampInt(argInt(a, "page", 1), 1, 10000), clampInt(argInt(a, "limit", 10), 1, 50))
			if err != nil {
				return "", err
			}
			items := make([]map[string]any, 0, len(list))
			for _, t := range list {
				items = append(items, taskCompact(t))
			}
			return jsonOutAny(map[string]any{"total": total, "items": items})
		})

	opsTool(srv, "project_task_get", "查任务详情（真实数据）：task_id 取任务全量字段与描述",
		[]toolArg{{"task_id", "任务 id", true}},
		func(ctx context.Context, a map[string]any) (string, error) {
			t, err := d.Task.Get(ctx, argStr(a, "task_id", ""))
			if err != nil {
				return fmt.Sprintf("未找到任务：%s", argStr(a, "task_id", "")), nil
			}
			return jsonOutAny(map[string]any{
				"taskId": t.TaskId, "projectId": t.ProjectId, "parentTaskId": t.ParentTaskId,
				"milestoneId": t.MilestoneId, "title": t.Title, "description": t.Description,
				"status": t.Status, "priority": t.Priority,
				"assigneeId": t.AssigneeId, "reporterId": t.ReporterId,
				"startDate": fmtTime(t.StartDate, "2006-01-02"), "dueDate": fmtTime(t.DueDate, "2006-01-02"),
				"doneAt": fmtTime(t.DoneAt, "2006-01-02 15:04"), "tags": t.Tags,
				"estimateHours": t.EstimateHours, "spentHours": t.SpentHours,
				"createdAt": fmtTime(t.CreatedAt, "2006-01-02 15:04"),
			})
		})

	opsTool(srv, "project_milestone_list", "查项目里程碑列表（真实数据）：open|in_progress|closed",
		[]toolArg{{"project_id", "项目 id", true}, {"status", "状态过滤：open|in_progress|closed", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			list, total, err := d.Milestone.List(ctx, argStr(a, "project_id", ""), argStr(a, "status", ""), 1, 50)
			if err != nil {
				return "", err
			}
			items := make([]map[string]any, 0, len(list))
			for _, m := range list {
				items = append(items, map[string]any{
					"milestoneId": m.MilestoneId, "name": m.Name, "status": m.Status,
					"startDate": fmtTime(m.StartDate, "2006-01-02"), "dueDate": fmtTime(m.DueDate, "2006-01-02"),
				})
			}
			return jsonOutAny(map[string]any{"total": total, "items": items})
		})

	opsTool(srv, "project_milestone_progress", "查里程碑进度（真实数据）：任务总量/完成量/百分比；milestone_id 缺省为整个项目",
		[]toolArg{{"project_id", "项目 id", true}, {"milestone_id", "里程碑 id（缺省=整个项目）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			total, done, rate, err := d.Milestone.Progress(ctx, argStr(a, "project_id", ""), argStr(a, "milestone_id", ""))
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("任务 %d / 完成 %d（%d%%）", total, done, rate), nil
		})

	opsTool(srv, "project_board", "查项目看板（真实数据）：状态列（key/名称/类别）+ 各列任务，推进任务前先看板",
		[]toolArg{{"project_id", "项目 id", true}},
		func(ctx context.Context, a map[string]any) (string, error) {
			statuses, tasks, err := d.Board.Board(ctx, argStr(a, "project_id", ""))
			if err != nil {
				return "", err
			}
			ss := make([]map[string]any, 0, len(statuses))
			for _, st := range statuses {
				ss = append(ss, map[string]any{
					"key": st.Key, "name": st.Name, "category": st.Category, "sortOrder": st.SortOrder,
				})
			}
			ts := make([]map[string]any, 0, len(tasks))
			for _, t := range tasks {
				ts = append(ts, taskCompact(t))
			}
			return jsonOutAny(map[string]any{"statuses": ss, "tasks": ts})
		})

	opsTool(srv, "project_activity_list", "查项目动态（真实数据）：创建/更新/状态变更/评论/指派 活动流，回溯「谁在何时做了什么」",
		[]toolArg{{"project_id", "项目 id", true}, {"task_id", "按任务过滤", false}, {"action", "按动作过滤：created|updated|commented|status_changed|assigned|archived", false}, {"limit", "条数 1-50，默认 20", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			list, total, err := d.Activity.List(ctx, argStr(a, "project_id", ""), argStr(a, "task_id", ""),
				argStr(a, "action", ""), 1, clampInt(argInt(a, "limit", 20), 1, 50))
			if err != nil {
				return "", err
			}
			items := make([]map[string]any, 0, len(list))
			for _, act := range list {
				items = append(items, map[string]any{
					"ts": fmtTime(act.CreatedAt, "2006-01-02 15:04"), "action": act.Action,
					"taskId": act.TaskId, "userId": act.UserId, "target": act.Target,
				})
			}
			return jsonOutAny(map[string]any{"total": total, "items": items})
		})

	opsTool(srv, "project_report_overview", "查项目报表概览（真实数据）：任务/工时/成员维度的统计汇总",
		[]toolArg{{"project_id", "项目 id", true}},
		func(ctx context.Context, a map[string]any) (string, error) {
			out, err := d.Report.Overview(ctx, argStr(a, "project_id", ""))
			if err != nil {
				return "", err
			}
			return jsonOutAny(out)
		})

	opsTool(srv, "project_timeline_list", "查项目时间线（真实数据）：按起止日期排布的任务视图",
		[]toolArg{{"project_id", "项目 id", true}},
		func(ctx context.Context, a map[string]any) (string, error) {
			list, err := d.Timeline.List(ctx, argStr(a, "project_id", ""))
			if err != nil {
				return "", err
			}
			items := make([]map[string]any, 0, len(list))
			for _, t := range list {
				items = append(items, map[string]any{
					"taskId": t.TaskId, "title": t.Title, "status": t.Status,
					"startDate": fmtTime(t.StartDate, "2006-01-02"), "dueDate": fmtTime(t.DueDate, "2006-01-02"),
				})
			}
			return jsonOutAny(map[string]any{"items": items})
		})

	opsTool(srv, "project_member_list", "查项目成员（真实数据）：userId/角色（owner=maintainer=member=observer）",
		[]toolArg{{"project_id", "项目 id", true}, {"role", "按角色过滤", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			list, total, err := d.Member.List(ctx, argStr(a, "project_id", ""), argStr(a, "role", ""), 1, 50)
			if err != nil {
				return "", err
			}
			items := make([]map[string]any, 0, len(list))
			for _, m := range list {
				items = append(items, map[string]any{
					"memberId": m.MemberId, "userId": m.UserId, "role": m.Role,
					"joinedAt": fmtTime(m.JoinedAt, "2006-01-02"),
				})
			}
			return jsonOutAny(map[string]any{"total": total, "items": items})
		})

	return srv
}

// ===== 写 server（platform-project-write，/mcp/project-write） =====

func buildProjectWriteServer(d Deps) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "aiarsenal-mcp-project-write", Version: "1.0"}, nil)

	opsTool(srv, "project_create", "建项目（写真实库）：名称必填；自动建 owner 成员 + 默认状态流（待办/进行中/已完成）",
		[]toolArg{{"name", "项目名称", true}, {"code", "项目代号（可选）", false}, {"description", "项目描述", false}, {"priority", "优先级：low|medium|high|urgent，默认 medium", false}, {"owner", "负责人 user_id（缺省=当前调用方）", false}, {"start_date", "开始日期 YYYY-MM-DD", false}, {"due_date", "截止日期 YYYY-MM-DD", false}, {"tags", "标签，逗号分隔", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			e := &entity.Project{
				Name: argStr(a, "name", ""), Code: argStr(a, "code", ""),
				Description: argStr(a, "description", ""), Priority: argStr(a, "priority", ""),
				Owner: argStr(a, "owner", ""),
				Tags:  argTags(a, "tags"),
			}
			if e.Owner == "" {
				e.Owner = projActor(ctx)
			}
			var err error
			if e.StartDate, err = argDate(a, "start_date"); err != nil {
				return "", err
			}
			if e.DueDate, err = argDate(a, "due_date"); err != nil {
				return "", err
			}
			e.CreatedBy, e.UpdatedBy = projActor(ctx), projActor(ctx)
			id, err := d.Project.Add(ctx, e)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("已创建项目 %s（project_id=%s）", e.Name, id), nil
		})

	opsTool(srv, "project_update", "改项目（写真实库）：只改给定的字段（名称/描述/状态/优先级/进度/日期等），其余保持原值",
		[]toolArg{{"project_id", "项目 id", true}, {"name", "新名称", false}, {"description", "新描述", false}, {"status", "新状态：planning|active|paused|closed|archived", false}, {"priority", "新优先级", false}, {"progress", "进度 0-100", false}, {"owner", "新负责人 user_id", false}, {"start_date", "开始日期 YYYY-MM-DD", false}, {"due_date", "截止日期 YYYY-MM-DD", false}, {"tags", "标签，逗号分隔", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			id := argStr(a, "project_id", "")
			p, err := d.Project.Get(ctx, id)
			if err != nil {
				return fmt.Sprintf("未找到项目：%s", id), nil
			}
			for k, dst := range map[string]*string{
				"name": &p.Name, "description": &p.Description, "status": &p.Status,
				"priority": &p.Priority, "owner": &p.Owner,
			} {
				if v := argStr(a, k, ""); v != "" {
					*dst = v
				}
			}
			if v := argInt(a, "progress", -1); v >= 0 {
				p.Progress = clampInt(v, 0, 100)
			}
			if v := argTags(a, "tags"); v != "" {
				p.Tags = v
			}
			if t, err := argDate(a, "start_date"); err != nil {
				return "", err
			} else if t != nil {
				p.StartDate = t
			}
			if t, err := argDate(a, "due_date"); err != nil {
				return "", err
			} else if t != nil {
				p.DueDate = t
			}
			p.UpdatedBy = projActor(ctx)
			if err := d.Project.Edit(ctx, p); err != nil {
				return "", err
			}
			return fmt.Sprintf("已更新项目 %s", p.Name), nil
		})

	opsTool(srv, "project_delete", "删项目（写真实库）：软删；其下任务/成员一并隐藏，谨慎使用",
		[]toolArg{{"project_id", "项目 id", true}},
		func(ctx context.Context, a map[string]any) (string, error) {
			id := argStr(a, "project_id", "")
			if err := d.Project.Delete(ctx, []string{id}); err != nil {
				return "", err
			}
			return fmt.Sprintf("已删除项目 %s", id), nil
		})

	opsTool(srv, "project_task_create", "建任务（写真实库）：project_id+标题必填；缺省状态取项目默认列，reporter=当前调用方",
		[]toolArg{{"project_id", "项目 id", true}, {"title", "任务标题", true}, {"description", "任务描述", false}, {"status", "状态 key（缺省=项目默认列）", false}, {"priority", "优先级，默认 medium", false}, {"assignee_id", "负责人 user_id", false}, {"milestone_id", "所属里程碑 id", false}, {"parent_task_id", "父任务 id（建子任务时给）", false}, {"due_date", "截止日期 YYYY-MM-DD", false}, {"estimate_hours", "预估工时（小时）", false}, {"tags", "标签，逗号分隔", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			e := &entity.ProjectTask{
				ProjectId: argStr(a, "project_id", ""), Title: argStr(a, "title", ""),
				Description: argStr(a, "description", ""), Status: argStr(a, "status", ""),
				Priority: argStr(a, "priority", ""), AssigneeId: argStr(a, "assignee_id", ""),
				MilestoneId: argStr(a, "milestone_id", ""), ParentTaskId: argStr(a, "parent_task_id", ""),
				Tags:          argTags(a, "tags"),
				EstimateHours: argFloat(a, "estimate_hours", 0),
			}
			var err error
			if e.DueDate, err = argDate(a, "due_date"); err != nil {
				return "", err
			}
			id, err := d.Task.Add(ctx, e, projActor(ctx))
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("已创建任务 %s（task_id=%s）", e.Title, id), nil
		})

	opsTool(srv, "project_task_update", "改任务（写真实库）：只改给定的字段（标题/描述/状态/优先级/负责人/日期等）",
		[]toolArg{{"task_id", "任务 id", true}, {"title", "新标题", false}, {"description", "新描述", false}, {"status", "新状态 key（状态流转建议用 project_task_move）", false}, {"priority", "新优先级", false}, {"assignee_id", "新负责人 user_id", false}, {"milestone_id", "新里程碑 id", false}, {"due_date", "截止日期 YYYY-MM-DD", false}, {"estimate_hours", "预估工时（小时）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			id := argStr(a, "task_id", "")
			t, err := d.Task.Get(ctx, id)
			if err != nil {
				return fmt.Sprintf("未找到任务：%s", id), nil
			}
			for k, dst := range map[string]*string{
				"title": &t.Title, "description": &t.Description, "status": &t.Status,
				"priority": &t.Priority, "assignee_id": &t.AssigneeId, "milestone_id": &t.MilestoneId,
			} {
				if v := argStr(a, k, ""); v != "" {
					*dst = v
				}
			}
			if v := argTags(a, "tags"); v != "" {
				t.Tags = v
			}
			if _, ok := a["estimate_hours"]; ok {
				t.EstimateHours = argFloat(a, "estimate_hours", t.EstimateHours)
			}
			if tt, err := argDate(a, "due_date"); err != nil {
				return "", err
			} else if tt != nil {
				t.DueDate = tt
			}
			if err := d.Task.Edit(ctx, t, projActor(ctx)); err != nil {
				return "", err
			}
			return fmt.Sprintf("已更新任务 %s", t.Title), nil
		})

	opsTool(srv, "project_task_move", "看板移动任务（写真实库）：把任务挪到目标状态列（todo/doing/done 等项目状态 key），状态变更走这个",
		[]toolArg{{"task_id", "任务 id", true}, {"status", "目标状态 key（project_board 查得）", true}, {"sort_order", "列内序号（可选）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			if err := d.Board.Move(ctx, argStr(a, "task_id", ""), argStr(a, "status", ""), argInt(a, "sort_order", 0), projActor(ctx)); err != nil {
				return "", err
			}
			return fmt.Sprintf("已移动到 %s", argStr(a, "status", "")), nil
		})

	opsTool(srv, "project_task_comment_add", "给任务加评论（写真实库）：同步写入任务活动流",
		[]toolArg{{"task_id", "任务 id", true}, {"content", "评论内容", true}},
		func(ctx context.Context, a map[string]any) (string, error) {
			e := &entity.ProjectTaskComment{TaskId: argStr(a, "task_id", ""), Content: argStr(a, "content", "")}
			if _, err := d.Task.AddComment(ctx, e, projActor(ctx)); err != nil {
				return "", err
			}
			return "评论已添加", nil
		})

	opsTool(srv, "project_milestone_create", "建里程碑（写真实库）：project_id+名称必填",
		[]toolArg{{"project_id", "项目 id", true}, {"name", "里程碑名称", true}, {"description", "描述", false}, {"status", "状态：open|in_progress|closed，默认 open", false}, {"start_date", "开始日期 YYYY-MM-DD", false}, {"due_date", "截止日期 YYYY-MM-DD", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			e := &entity.ProjectMilestone{
				ProjectId: argStr(a, "project_id", ""), Name: argStr(a, "name", ""),
				Description: argStr(a, "description", ""), Status: argStr(a, "status", ""),
				CreatedBy: projActor(ctx),
			}
			var err error
			if e.StartDate, err = argDate(a, "start_date"); err != nil {
				return "", err
			}
			if e.DueDate, err = argDate(a, "due_date"); err != nil {
				return "", err
			}
			id, err := d.Milestone.Add(ctx, e)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("已创建里程碑 %s（milestone_id=%s）", e.Name, id), nil
		})

	opsTool(srv, "project_milestone_update", "改里程碑（写真实库）：只改给定的字段（名称/描述/状态/日期）",
		[]toolArg{{"milestone_id", "里程碑 id", true}, {"name", "新名称", false}, {"description", "新描述", false}, {"status", "新状态：open|in_progress|closed", false}, {"start_date", "开始日期 YYYY-MM-DD", false}, {"due_date", "截止日期 YYYY-MM-DD", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			id := argStr(a, "milestone_id", "")
			m, err := d.Milestone.Get(ctx, id)
			if err != nil {
				return fmt.Sprintf("未找到里程碑：%s", id), nil
			}
			for k, dst := range map[string]*string{
				"name": &m.Name, "description": &m.Description, "status": &m.Status,
			} {
				if v := argStr(a, k, ""); v != "" {
					*dst = v
				}
			}
			if t, err := argDate(a, "start_date"); err != nil {
				return "", err
			} else if t != nil {
				m.StartDate = t
			}
			if t, err := argDate(a, "due_date"); err != nil {
				return "", err
			} else if t != nil {
				m.DueDate = t
			}
			if err := d.Milestone.Edit(ctx, m); err != nil {
				return "", err
			}
			return fmt.Sprintf("已更新里程碑 %s", m.Name), nil
		})

	opsTool(srv, "project_member_add", "加项目成员（写真实库）：user_id + 角色（owner/maintainer/member/observer，默认 member）",
		[]toolArg{{"project_id", "项目 id", true}, {"user_id", "成员 user_id", true}, {"role", "角色，默认 member", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			e := &entity.ProjectMember{
				ProjectId: argStr(a, "project_id", ""), UserId: argStr(a, "user_id", ""),
				Role: argStr(a, "role", "member"), CreatedBy: projActor(ctx),
			}
			if _, err := d.Member.Add(ctx, e); err != nil {
				return "", err
			}
			return fmt.Sprintf("已把 %s 加入项目（%s）", e.UserId, e.Role), nil
		})

	return srv
}
