-- 安全事件告警落库（等保三级 8.1.5.4f 识别+报警 / 8.1.4.3c 审计记录保护 / 网安法 21 条留存≥6月）。
-- 本表无删除/清空端点，留存由运维备份保障（≥180 天）；应用层禁止 DROP/TRUNCATE。
CREATE TABLE IF NOT EXISTS aa_alert_record (
  id bigserial PRIMARY KEY,
  event_type varchar(50) NOT NULL,
  severity varchar(20) NOT NULL,
  title varchar(200) NOT NULL,
  body text,
  biz_key varchar(200),
  delivered boolean NOT NULL DEFAULT false,
  retries integer NOT NULL DEFAULT 0,
  last_error text,
  created_at timestamp NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_aa_alert_record_type_time ON public.aa_alert_record USING btree (event_type, created_at);
CREATE INDEX IF NOT EXISTS idx_aa_alert_record_undelivered ON public.aa_alert_record USING btree (delivered) WHERE delivered = false;
