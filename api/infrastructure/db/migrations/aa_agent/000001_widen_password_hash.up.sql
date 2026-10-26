-- 口令加固（硬切换）：aa_agent_user.password_hash 从 varchar(64) 加宽到 varchar(255)，
-- 容纳自描述口令串 $pbkdf2-<算法>$iter$salt$hash（见 infrastructure/auth/password.go）。
-- 与基线 agent.sql 的 CREATE TABLE 同步修改（基线是建表真相源，迁移只给存量库追平）。
ALTER TABLE aa_agent_user ALTER COLUMN password_hash TYPE varchar(255);
