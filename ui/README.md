# AI Arsenal UI（管理后台前端）

基于 [vue-vben-admin](https://github.com/vbenjs/vue-vben-admin) v5（Ant Design Vue 版）深度定制的 monorepo。
**上游以 [MIT License](./LICENSE) 发布，定制部分同样以 MIT 随本目录分发**（仓库根协议为 Apache-2.0）。

## 结构

- `apps/web-antd` — AI Arsenal 管理后台（Ant Design Vue）
- `packages/*` / `internal/*` — 沿用 vben monorepo 的共享包与构建链（@vben/* 命名保留，便于跟进上游）

## 开发

```bash
pnpm install
pnpm --filter @vben/web-antd dev
```

构建（测试/生产模式，详见仓库根 [README.md](../README.md)）：

```bash
pnpm --filter @vben/web-antd build:test
pnpm --filter @vben/web-antd build
```

## 致谢

- [vue-vben-admin](https://github.com/vbenjs/vue-vben-admin) 及其贡献者 —— 底座框架
