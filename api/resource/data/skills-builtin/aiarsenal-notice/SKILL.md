---
name: aiarsenal-notice
title: 平台站内通知
description: 平台站内通知触达：notice_user_lookup 先解析收件人 user_id，再 notice_send 发通知（UI 右上角可见）；只发必要通知，勿刷屏。
version: 1.0.1
keywords: [通知, 消息, 提醒, notice]
triggers: [用户要求通知/提醒某人, 用户要看自己的通知或未读数, 任务/审批等事件需要主动告知]
---

# 平台通知工作流

经网关分发后工具名带前缀（如 `platform-notice__notice_send`），本技能以短名指代；
只读端点 platform-notice，写入端点 platform-notice-write。

## 发通知给某人（标准动线）

1. **先解析收件人**：`notice_user_lookup` 按姓名/工号查 user_id（更全的找人/部门查询
   见 platform-user 域的 user_lookup / dept_list）——**永远不要猜 user_id**；
   同名多人时把候选列出来让用户确认，别默认选第一个。
2. `notice_send` 发送：
   - `type` 按语义选：info=一般告知 / success=完成 / warning=需注意 / error=出问题；
   - `link` 三种约定：`http…`=外链、`/xxx`=平台内路由（如 `/system/project`）、`#xxx`=前端弹窗；
   - 标题一句话说清事由，正文给上下文（谁、什么事项、要不要行动）。

## 查自己的通知

- `notice_list`（可按 type/是否已读过滤）、`notice_unread_count` 未读数。
- 处理完 `notice_mark_read` / `notice_mark_all_read`。

## 护栏

- **只发必要通知**：用户没要求、也没有明确业务事件时不要主动发；同一事项不要重复发。
- 收件人是谁以 lookup 结果为准；查无此人就告诉用户「没找到这个人」，不要发给相近的别人。
- 通知是触达手段不是留言板——长内容放项目评论（project_task_comment_add）或文档，通知里给摘要+链接。
