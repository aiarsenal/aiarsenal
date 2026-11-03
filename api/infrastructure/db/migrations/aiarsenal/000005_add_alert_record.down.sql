-- 回退：删除告警落库表。注意：删表会连带丢弃其中已留存的安全事件记录
-- （等保审计记录保护口径下属破坏性操作），仅作形式完整，生产勿执行。
DROP INDEX IF EXISTS public.idx_aa_alert_record_undelivered;
DROP INDEX IF EXISTS public.idx_aa_alert_record_type_time;
DROP TABLE IF EXISTS public.aa_alert_record;
