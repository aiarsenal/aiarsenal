DROP TABLE IF EXISTS public.aa_schedule_sync_link;
DROP TABLE IF EXISTS public.aa_schedule_sync_account;
DROP INDEX IF EXISTS public.idx_aa_schedule_event_source_uid;
ALTER TABLE public.aa_schedule_event
    DROP COLUMN IF EXISTS external_uid,
    DROP COLUMN IF EXISTS source_account_id;
