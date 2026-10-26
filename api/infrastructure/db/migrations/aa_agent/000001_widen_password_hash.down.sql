-- 回退：password_hash 恢复 varchar(64)。仅当回退部署未产生过新格式口令串时安全
-- （新串 85~129 字符会因列宽回缩写入失败）。
ALTER TABLE aa_agent_user ALTER COLUMN password_hash TYPE varchar(64);
