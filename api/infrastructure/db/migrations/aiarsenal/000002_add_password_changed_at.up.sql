-- 口令定期更换（可选，password.policy.rotationDays>0 启用）：记录最近一次口令设置时间。
-- 存量账号回填当前时间——升级后获得完整更换窗口，不会被立即判过期。
-- 时间显式锚定 Asia/Shanghai：迁移会话（lib/pq）不带 TimeZone 参数，服务器默认时区
-- 非 CST 时 now() 会偏差 N 小时（应用侧 pgx 会话按 Asia/Shanghai 读写，见 config tzParam）。
ALTER TABLE aa_user ADD COLUMN IF NOT EXISTS password_changed_at timestamp without time zone;
UPDATE aa_user SET password_changed_at = (now() AT TIME ZONE 'Asia/Shanghai') WHERE password_changed_at IS NULL;
