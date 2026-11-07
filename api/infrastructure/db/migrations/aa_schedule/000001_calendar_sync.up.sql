ALTER TABLE public.aa_schedule_event
    ADD COLUMN IF NOT EXISTS source_account_id varchar(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS external_uid varchar(255) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_aa_schedule_event_source_uid
    ON public.aa_schedule_event (source_account_id, external_uid)
    WHERE deleted_at IS NULL AND source_account_id <> '';

CREATE TABLE IF NOT EXISTS public.aa_schedule_sync_account (
    id bigserial PRIMARY KEY,
    account_id varchar(32) NOT NULL UNIQUE,
    name varchar(120) NOT NULL,
    provider varchar(16) NOT NULL DEFAULT 'caldav',
    server_url varchar(500) NOT NULL,
    calendar_url varchar(1000) NOT NULL,
    username varchar(255) NOT NULL,
    password_enc text NOT NULL,
    enabled integer NOT NULL DEFAULT 1,
    sync_interval_minutes integer NOT NULL DEFAULT 15,
    conflict_policy varchar(16) NOT NULL DEFAULT 'manual',
    owner_user_id varchar(32) NOT NULL,
    last_sync_at timestamp without time zone,
    last_sync_status varchar(16) NOT NULL DEFAULT 'idle',
    last_sync_error text NOT NULL DEFAULT '',
    created_at timestamp without time zone NOT NULL DEFAULT (now() AT TIME ZONE 'Asia/Shanghai'),
    updated_at timestamp without time zone NOT NULL DEFAULT (now() AT TIME ZONE 'Asia/Shanghai'),
    deleted_at timestamp without time zone,
    CONSTRAINT aa_schedule_sync_account_policy_check
        CHECK (conflict_policy IN ('manual', 'local', 'remote')),
    CONSTRAINT aa_schedule_sync_account_interval_check
        CHECK (sync_interval_minutes >= 5 AND sync_interval_minutes <= 1440)
);

CREATE INDEX IF NOT EXISTS idx_aa_schedule_sync_account_owner
    ON public.aa_schedule_sync_account (owner_user_id, enabled)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS public.aa_schedule_sync_link (
    id bigserial PRIMARY KEY,
    link_id varchar(32) NOT NULL UNIQUE,
    account_id varchar(32) NOT NULL,
    event_id varchar(32) NOT NULL,
    remote_href text NOT NULL,
    remote_etag varchar(255) NOT NULL DEFAULT '',
    remote_uid varchar(255) NOT NULL DEFAULT '',
    local_updated_at timestamp without time zone,
    exceptions_hash varchar(64) NOT NULL DEFAULT '',
    conflict integer NOT NULL DEFAULT 0,
    conflict_payload text NOT NULL DEFAULT '',
    last_synced_at timestamp without time zone,
    created_at timestamp without time zone NOT NULL DEFAULT (now() AT TIME ZONE 'Asia/Shanghai'),
    updated_at timestamp without time zone NOT NULL DEFAULT (now() AT TIME ZONE 'Asia/Shanghai'),
    CONSTRAINT aa_schedule_sync_link_account_href_unique UNIQUE (account_id, remote_href),
    CONSTRAINT aa_schedule_sync_link_account_event_unique UNIQUE (account_id, event_id)
);

CREATE INDEX IF NOT EXISTS idx_aa_schedule_sync_link_event
    ON public.aa_schedule_sync_link (event_id);
