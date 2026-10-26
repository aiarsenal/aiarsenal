-- 双因子（TOTP）：密钥 AES-256-GCM 密文 hex 落库，enabled=1 表示已激活。
-- varchar(128) 余量算式：base32(20B 密钥)=32 字符 → GCM(12+32+16)=60B → hex 120。
ALTER TABLE aa_user ADD COLUMN IF NOT EXISTS totp_secret character varying(128) DEFAULT ''::character varying NOT NULL;
ALTER TABLE aa_user ADD COLUMN IF NOT EXISTS totp_enabled smallint DEFAULT 0 NOT NULL;
