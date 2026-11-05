-- 回退：恢复唯一索引。若存量数据存在重名菜单（业务上合法存在），
-- 本语句将以重复键失败——降级是单向收敛，down 仅作形式完整，实际不可逆。
DROP INDEX IF EXISTS public.idx_aa_menu_name;
CREATE UNIQUE INDEX idx_aa_menu_name ON public.aa_menu USING btree (name);
