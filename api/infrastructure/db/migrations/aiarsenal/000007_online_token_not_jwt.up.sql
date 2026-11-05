-- F-025 闭环加固（审计建议的 schema 级保证）：aa_user_online.token 的合法取值自
-- 000006 起只有三种——32 位 jti 会话标识、'scrub-<uuid>' 失焦标记、空串。明文 JWT
-- 以 "eyJ" 开头（base64("{\"")），加 CHECK 约束让数据库本身拒绝任何未来写入点的
-- 明文回落——应用层接线守卫之外的第二道闸。约束在 000006 清洗之后执行，存量行
-- 已全部合法；全新安装空表直接通过。
ALTER TABLE aa_user_online
  ADD CONSTRAINT chk_aa_user_online_token_not_jwt CHECK (token NOT LIKE 'eyJ%');
