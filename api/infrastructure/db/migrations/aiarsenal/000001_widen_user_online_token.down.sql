-- 回退：缩回 255。仅当回退后无新登录（存量 440 字符令牌行会因列宽写入失败）。
ALTER TABLE aa_user_online ALTER COLUMN token TYPE character varying(255);
