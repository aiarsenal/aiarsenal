<!-- 标题建议: type(scope): 简述,如 fix(api): 修复渠道重试计数 / fix(api): fix channel retry count -->

## 改动说明 / What does this PR do?

<!-- 简述改动内容与动机;关联 issue 用 "Closes #123" / Brief summary; link issues with "Closes #123" -->

## 测试方式 / How was this tested?

<!-- 如何验证:本地命令、手工步骤、新增/修改的测试 / Local commands, manual steps, tests added or updated -->
- [ ] `cd api && go test ./...`
- [ ] `cd ui && pnpm --filter @vben/web-antd build`
- [ ] 客户端改动已跑 `cargo check`(对应 mac / win)

## 自检清单 / Checklist

- [ ] 已在本地跑通相关构建与测试 / Relevant builds and tests pass locally
- [ ] 不依赖数据库的单测已覆盖,需外部依赖的测试加了 `testing.Short()` 跳过声明 / DB-free unit tests added; external-dependency tests declare a `testing.Short()` skip
- [ ] 涉及 `.github/workflows` 改动时,已在 fork/分支上验证过 / Workflow changes verified on a branch
- [ ] 无敏感信息(密钥、内网地址)硬编码 / No secrets or internal endpoints hardcoded
- [ ] 文档(README/docs)同步更新 / Docs updated if behavior changed
