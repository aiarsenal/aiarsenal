-- jti 时代的登录令牌（含 deptId/postIds/jti claims）约 440 字符，超出 varchar(255)，
-- 在线用户记录（aa_user_online，异步 best-effort 写入）静默失败。拓宽到 1024：
-- 440 + postIds 增长余量；令牌长度随 claims 演进，1024 后仍有数倍余量。
ALTER TABLE aa_user_online ALTER COLUMN token TYPE character varying(1024);
