-- F-013：安全审计与内容强绑定——扫描终态写入文件集聚合哈希，门禁比对当前文件集。
-- 基线（resource/data/skills.sql）已含该列；本迁移给存量库追平。
ALTER TABLE IF EXISTS public.aa_skill_security_audit
    ADD COLUMN IF NOT EXISTS content_hash character varying(64);
