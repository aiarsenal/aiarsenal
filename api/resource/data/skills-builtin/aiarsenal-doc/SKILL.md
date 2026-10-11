---
name: aiarsenal-doc
title: 平台文档库
description: 平台文档库读写：日常工作产生的文档（日志/纪要/报告）落这里——doc_create/doc_update 写入自动重嵌，doc_search 语义检索、doc_read 读全文；知识的总结去知识库（只经 Web 上传）。
version: 1.1.1
keywords: [文档, 文档库, 检索, 工作日志, doc]
triggers: [用户要写/存日常工作文档（日志/纪要/报告）, 用户要查/读平台上的文档全文]
---

# 平台文档库工作流

经网关分发后工具名带前缀（如 `platform-doc__doc_search`），本技能以短名指代；
只读端点 platform-doc，写入端点 platform-doc-write。

## 文档库 vs 知识库（先分清）

| | 文档库 platform-doc | 知识库 platform-kb |
|---|---|---|
| 定位 | **日常工作产生的文档**：工作日志、会议纪要、报告、长文 | **一系列知识的总结**：制度要点/FAQ/沉淀资料 |
| 谁能改 | agent 可读写（doc_create/doc_update） | 仅平台 Web 上传变更，agent 只读（kb_search/kb_read） |
| 用户说 | 「帮我写个工作日志/纪要/报告」「查那份文档原文」 | 「……是怎么规定的」 |

用户让「写个文档/记录一下今天的工作」→ 一律走文档库 doc_create；
不要写容器本地文件，也不要往知识库里放日常工作产物。

## 读文档

1. `doc_search` 语义检索定位（带文档标题与相似度）。
2. `doc_read` 按 doc_id 取全文（也可按标题精确匹配）。
3. 转述长文给结论 + 关键摘录，注明出自哪篇文档。

## 写文档（需 platform-doc-write）

1. `doc_search` 查重：同内容已存在 → 返回的提示会带已有 doc_id，改走 `doc_update`。
2. `doc_create`：标题自然可检索，正文纯文本；系统自动清洗→分块→向量嵌入，同 hash 内容会跳过。
3. 修订走 `doc_update`（正文变更自动重嵌），保留 doc_id 不变，外部引用不失效。

## 护栏

- 不确定来源的传闻不要写进文档库；写进去的内容用户会当作可靠资料引用。
- 正文替换是**全量替换**（不是追加），更新前先 doc_read 看一眼现有内容。
