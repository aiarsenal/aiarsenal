---
name: aiarsenal-user
title: 平台员工信息查询
description: 平台员工信息查询：user_lookup 按姓名/工号找人拿 user_id，dept_list 部门树，dept_user_list 花名册；发通知、找负责人、查「谁在哪个部门」都先来这里。
version: 1.0.1
keywords: [员工, 通讯录, 部门, 花名册, user, dept]
triggers: [用户问某人在哪个部门/手机号, 需要找某人的 user_id, 查某部门的成员名单]
---

# 员工信息查询工作流

经网关分发后工具名带前缀（如 `platform-user__user_lookup`），本技能以短名指代；
端点 platform-user 只读（员工档案由管理端维护，agent 只查不改）。

## 找人

1. `user_lookup` 一把梭：姓名 / 工号 / 手机号 / 邮箱 / 账号名任一模糊匹配，
   可用 dept_id 限定部门。
2. 重名常见——结果里看 employee_no、部门、性别核对，别拿第一条就当答案。
3. `user_get` 按 user_id 取完整档案（部门、上级部门、联系方式、备注、最近登录）。

## 查部门

1. `dept_list` 全量部门树（含负责人、在编人数），可按名称过滤；返回的 dept_id
   是后续查询的钥匙。
2. `dept_user_list` 花名册：include_children=true 连同全部下级部门（如「某分厂
   全部人员」），分页取。

## 与其它域配合

- 发站内通知（platform-notice-write notice_send）前用 `user_lookup` 解析收件人
  user_id，**绝不凭姓名猜 user_id**。
- 项目任务派给人、审批找负责人，同样先 user_lookup 核实。

## 护栏

- 查询结果含手机号/邮箱等个人信息——只在用户明确要查时使用，答复直接相关内容，
  不要主动大范围拉取花名册。
- user_status=停用 的账号不再是有效收件人/负责人，答复时注明。
