-- 回退：删除双因子两列（所有账号回到仅口令鉴别，属预期）。
ALTER TABLE aa_user DROP COLUMN IF EXISTS totp_enabled;
ALTER TABLE aa_user DROP COLUMN IF EXISTS totp_secret;
