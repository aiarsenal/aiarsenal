-- 回退：删除口令更换时间列（rotationDays 判定随之失效，属预期）。
ALTER TABLE aa_user DROP COLUMN IF EXISTS password_changed_at;
