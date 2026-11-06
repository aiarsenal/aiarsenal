-- 移除明文 JWT 拒绝约束（down 恢复的是"允许存明文"的旧口径，仅在回滚整个 F-025 修复时使用）。
ALTER TABLE aa_user_online DROP CONSTRAINT IF EXISTS chk_aa_user_online_token_not_jwt;
