-- F-025：aa_user_online.token 曾存明文 JWT——具备库读权限即可重放劫持任意在线会话
-- （超管尤甚，B 轮审计实测）。应用侧自本版本起只写 jti 会话标识（auth.SessionIDOf）。
-- 存量明文令牌就地失焦：以 'scrub-'||uuid 覆写（uuid 有唯一索引，覆写值必然唯一，
-- 不触 token 列的唯一索引）。明文不可恢复属预期——它本就不该被存储。
UPDATE aa_user_online SET token = 'scrub-' || uuid WHERE token LIKE 'eyJ%';
