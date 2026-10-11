---
name: aiarsenal-project
title: 平台项目管理
description: 平台项目管理闭环：project_list/overview 看项目、project_board 看板推进任务（project_task_move）、project_task_create 建任务、project_activity_list 回溯动态。
version: 1.0.1
keywords: [项目, 任务, 看板, 里程碑, project]
triggers: [用户问项目/任务进展, 用户要求建任务/改状态/看板推进, 用户要求回溯谁做了什么]
---

# 平台项目工作流

经网关分发后工具名带前缀（如 `platform-project__project_board`），本技能以短名指代；
只读端点 platform-project，写入端点 platform-project-write。

## 常规动线

1. **找项目**：`project_list` 按名称过滤拿 project_id；首次接触一个项目先 `project_overview` 拿全貌。
2. **看板**：`project_board` 是任务推进的主视图——状态列（key）+ 各列任务一次拿到。
3. **动任务**：
   - 建任务 `project_task_create`（project_id + 标题必填，状态缺省进项目默认列）；
   - 改字段（负责人/优先级/截止日）`project_task_update`；
   - **状态流转一律 `project_task_move`**（它写活动流，直接 update 状态不记录流转）。
4. **回溯**：`project_activity_list` 看「谁在何时对哪个任务做了什么」，状态变更/指派/评论都在。

## 约定

- 状态 key 是项目级的（常见 todo/doing/done，但每个项目可自定义）——**先 project_board 拿实际 key，别猜**。
- 子任务：建任务时给 parent_task_id；列表里带 parentTaskId 的即子任务。
- 里程碑进度 `project_milestone_progress`；成员名单 `project_member_list`（user_id 可配 notice_send 发提醒）。
- 日期一律 `YYYY-MM-DD`。

## 护栏

- 删除项目（`project_delete`）影响面大，仅在用户明确要求时执行。
- 建项目用 `project_create`（自动建默认状态流）；别手工建状态列。
- 拿不到的项目/任务如实说未找到，不要拿相似 id 硬套。
