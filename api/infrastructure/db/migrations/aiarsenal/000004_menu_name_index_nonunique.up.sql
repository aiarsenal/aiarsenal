-- 菜单重名索引降级：aa_menu 的「目录 + 权限按钮」合法共用同一 API 路径作 name
-- （如 api/v1/agent/openclaw/configs 同时是目录 id=111 与按钮 id=118），
-- 唯一约束与业务模式冲突，干净库按基线 aiarsenal.sql 初始化会在建唯一索引阶段报重复键。
-- casbin 按 menu_id Enforce；URL→menuId 映射按 name 首匹配（interfaces/middleware/casbin.go，
-- 定序 weigh desc,id asc，menu_repo.go GetMenuList），重名行匹配序确定且与存量库行为一致，
-- 降为普通索引不改变该行为。
-- 先 DROP 再 CREATE，幂等兼容两种存量状态：干净库从未建成唯一索引 / 老库已建出唯一索引。
DROP INDEX IF EXISTS public.idx_aa_menu_name;
CREATE INDEX idx_aa_menu_name ON public.aa_menu USING btree (name);
