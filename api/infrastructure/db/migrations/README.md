# migrations 已 squash（2026-09）

历史上全部增量迁移已固化进 `api/resource/data/*.sql` 基线（各库经 pg_dump 重生成），
迁移目录随之清空（历史见 git）。`migrate.go` 对无 .sql 的库静默跳过，本目录保留
本文件仅为满足 go:embed 非空约束。

- 新装机：`db-init` 直接灌基线即为最新结构。
- 后续变更：在 `migrations/<db>/` 新建迁移文件，版本从 `000001` 重新编号。
- 存量环境升级到本提交时：各库先执行一次 `TRUNCATE schema_migrations;`
  （旧版本序列已作废，不重置则新迁移(版本号≤旧值)永远不会被 golang-migrate 执行）。
