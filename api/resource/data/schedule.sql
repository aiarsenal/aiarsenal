-- ============================================================================
-- schedule.sql — 日程模块数据库与表结构（我的日程/会议室预订/周期会议）
-- ============================================================================
-- 依赖：PostgreSQL 16（与 aiarsenal / aa_project 同实例异库）
-- 关联：organizer_id/user_id/created_by 软引用 aiarsenal 库的 aa_user.user_id
--       （跨库无实际 FK，遵循 aiarsenal 既有模式：以 varchar(32) biz_id 软引用）
--       room_id 软引用本库 aa_meeting_room.room_id（删除房间不级联，悬空回退显示 room_id）
-- 约定：每表 id bigserial PK（内部）+ <实体>_id varchar(32) UNIQUE（对外业务 id）
--       时间 timestamp（本地时刻 "2006-01-02 15:04:05"）；布尔/状态 smallint；jsonb 用 text
-- 周期模型：series 母事件（rrule jsonb）+ aa_schedule_exception 单场例外，不物化场次；
--       展开是纯函数（api/application/service/schedule_service.go expandEvent）
-- 存量环境启用：先跑本文件（建库+建表+菜单种子），再把 aa_schedule 全表 OWNER 收口给
--       aiarsenal（ALTER TABLE 须所有者，否则迁移报 "must be owner of table"，见 scripts/local-test.sh）
-- 重建：psql -f resource/data/schedule.sql（每表 DROP IF EXISTS，可重复跑）
-- ============================================================================

-- ----------------------------------------------------------------------------
-- Part A — 独立数据库 aa_schedule
-- ----------------------------------------------------------------------------

SELECT 'CREATE DATABASE aa_schedule'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'aa_schedule')\gexec

\connect aa_schedule

-- ============================================================================
-- 基线固化说明（2026-09 squash）
-- 正文已替换为 pg_dump 固化版：合并了历史上全部增量迁移（原 migrations/ 已删，git 可查）。
-- 原文件尾部「
--
-- PostgreSQL database dump
--

\restrict dFSBFEInag421gLR7S7uZU67eeEGPEISDjSIePvBZBabwvAanxUhAza6pbdnPvx

-- Dumped from database version 15.19 (Homebrew)
-- Dumped by pg_dump version 15.19 (Homebrew)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

DROP INDEX IF EXISTS public.idx_aa_schedule_exception_id;
DROP INDEX IF EXISTS public.idx_aa_schedule_exception_evt_date;
DROP INDEX IF EXISTS public.idx_aa_schedule_event_room;
DROP INDEX IF EXISTS public.idx_aa_schedule_event_org;
DROP INDEX IF EXISTS public.idx_aa_schedule_event_id;
DROP INDEX IF EXISTS public.idx_aa_schedule_attendee_user;
DROP INDEX IF EXISTS public.idx_aa_schedule_attendee_id;
DROP INDEX IF EXISTS public.idx_aa_schedule_attendee_evt_user;
DROP INDEX IF EXISTS public.idx_aa_meeting_room_id;
ALTER TABLE IF EXISTS ONLY public.aa_schedule_exception DROP CONSTRAINT IF EXISTS aa_schedule_exception_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_schedule_event DROP CONSTRAINT IF EXISTS aa_schedule_event_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_schedule_attendee DROP CONSTRAINT IF EXISTS aa_schedule_attendee_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_meeting_room DROP CONSTRAINT IF EXISTS aa_meeting_room_pkey;
ALTER TABLE IF EXISTS public.aa_schedule_exception ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_schedule_event ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_schedule_attendee ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_meeting_room ALTER COLUMN id DROP DEFAULT;
DROP SEQUENCE IF EXISTS public.aa_schedule_exception_id_seq;
DROP TABLE IF EXISTS public.aa_schedule_exception;
DROP SEQUENCE IF EXISTS public.aa_schedule_event_id_seq;
DROP TABLE IF EXISTS public.aa_schedule_event;
DROP SEQUENCE IF EXISTS public.aa_schedule_attendee_id_seq;
DROP TABLE IF EXISTS public.aa_schedule_attendee;
DROP SEQUENCE IF EXISTS public.aa_meeting_room_id_seq;
DROP TABLE IF EXISTS public.aa_meeting_room;
SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: aa_meeting_room; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_meeting_room (
    id bigint NOT NULL,
    room_id character varying(32) DEFAULT ''::character varying NOT NULL,
    name character varying(128) DEFAULT ''::character varying NOT NULL,
    location character varying(255) DEFAULT ''::character varying NOT NULL,
    capacity integer DEFAULT 0 NOT NULL,
    equipment text DEFAULT '[]'::text NOT NULL,
    enabled smallint DEFAULT 1 NOT NULL,
    remark character varying(512) DEFAULT ''::character varying NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_meeting_room_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_meeting_room_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_meeting_room_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_meeting_room_id_seq OWNED BY public.aa_meeting_room.id;


--
-- Name: aa_schedule_attendee; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_schedule_attendee (
    id bigint NOT NULL,
    attendee_id character varying(32) DEFAULT ''::character varying NOT NULL,
    event_id character varying(32) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    status smallint DEFAULT 0 NOT NULL,
    responded_at timestamp without time zone,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_schedule_attendee_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_schedule_attendee_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_schedule_attendee_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_schedule_attendee_id_seq OWNED BY public.aa_schedule_attendee.id;


--
-- Name: aa_schedule_event; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_schedule_event (
    id bigint NOT NULL,
    event_id character varying(32) DEFAULT ''::character varying NOT NULL,
    title character varying(255) DEFAULT ''::character varying NOT NULL,
    event_type smallint DEFAULT 0 NOT NULL,
    start_at timestamp without time zone,
    end_at timestamp without time zone,
    is_recurring smallint DEFAULT 0 NOT NULL,
    rrule text DEFAULT '{}'::text NOT NULL,
    tz character varying(64) DEFAULT 'Asia/Shanghai'::character varying NOT NULL,
    room_id character varying(32) DEFAULT ''::character varying NOT NULL,
    online_provider character varying(64) DEFAULT ''::character varying NOT NULL,
    online_url character varying(512) DEFAULT ''::character varying NOT NULL,
    location character varying(255) DEFAULT ''::character varying NOT NULL,
    organizer_id character varying(32) DEFAULT ''::character varying NOT NULL,
    remark text,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_schedule_event_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_schedule_event_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_schedule_event_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_schedule_event_id_seq OWNED BY public.aa_schedule_event.id;


--
-- Name: aa_schedule_exception; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_schedule_exception (
    id bigint NOT NULL,
    exception_id character varying(32) DEFAULT ''::character varying NOT NULL,
    event_id character varying(32) DEFAULT ''::character varying NOT NULL,
    occurrence_date character varying(10) DEFAULT ''::character varying NOT NULL,
    status smallint DEFAULT 0 NOT NULL,
    new_start_at timestamp without time zone,
    new_end_at timestamp without time zone,
    new_room_id character varying(32) DEFAULT ''::character varying NOT NULL,
    new_online_url character varying(512) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_schedule_exception_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_schedule_exception_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_schedule_exception_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_schedule_exception_id_seq OWNED BY public.aa_schedule_exception.id;


--
-- Name: aa_meeting_room id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_meeting_room ALTER COLUMN id SET DEFAULT nextval('public.aa_meeting_room_id_seq'::regclass);


--
-- Name: aa_schedule_attendee id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_schedule_attendee ALTER COLUMN id SET DEFAULT nextval('public.aa_schedule_attendee_id_seq'::regclass);


--
-- Name: aa_schedule_event id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_schedule_event ALTER COLUMN id SET DEFAULT nextval('public.aa_schedule_event_id_seq'::regclass);


--
-- Name: aa_schedule_exception id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_schedule_exception ALTER COLUMN id SET DEFAULT nextval('public.aa_schedule_exception_id_seq'::regclass);


--
-- Data for Name: aa_meeting_room; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_meeting_room (id, room_id, name, location, capacity, equipment, enabled, remark, created_by, created_at, updated_at, deleted_at) FROM stdin;
1	gb7td9bmhkwz84n29kxdlr1ydw6d6gqe	小会议室	默认楼层	6	["电视","白板"]	1	种子预置，可在会议室管理中修改		2026-09-17 05:31:13.543815	2026-09-17 05:31:13.543815	\N
2	u53vkt0mx09kxgveb8ludal0zhegp6wq	中会议室	默认楼层	12	["电视","投影","视频会议"]	1	种子预置，可在会议室管理中修改		2026-09-17 05:31:13.544656	2026-09-17 05:31:13.544656	\N
3	rzch6n9roy2mg5z9y7xecg0026wz6bgm	大会议室	默认楼层	30	["投影","视频会议","音响"]	1	种子预置，可在会议室管理中修改		2026-09-17 05:31:13.544914	2026-09-17 05:31:13.544914	\N
\.


--
-- Data for Name: aa_schedule_attendee; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_schedule_attendee (id, attendee_id, event_id, user_id, status, responded_at, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_schedule_event; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_schedule_event (id, event_id, title, event_type, start_at, end_at, is_recurring, rrule, tz, room_id, online_provider, online_url, location, organizer_id, remark, created_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_schedule_exception; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_schedule_exception (id, exception_id, event_id, occurrence_date, status, new_start_at, new_end_at, new_room_id, new_online_url, created_at, updated_at) FROM stdin;
\.


--
-- Name: aa_meeting_room_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_meeting_room_id_seq', 3, true);


--
-- Name: aa_schedule_attendee_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_schedule_attendee_id_seq', 1, false);


--
-- Name: aa_schedule_event_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_schedule_event_id_seq', 1, false);


--
-- Name: aa_schedule_exception_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_schedule_exception_id_seq', 1, false);


--
-- Name: aa_meeting_room aa_meeting_room_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_meeting_room
    ADD CONSTRAINT aa_meeting_room_pkey PRIMARY KEY (id);


--
-- Name: aa_schedule_attendee aa_schedule_attendee_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_schedule_attendee
    ADD CONSTRAINT aa_schedule_attendee_pkey PRIMARY KEY (id);


--
-- Name: aa_schedule_event aa_schedule_event_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_schedule_event
    ADD CONSTRAINT aa_schedule_event_pkey PRIMARY KEY (id);


--
-- Name: aa_schedule_exception aa_schedule_exception_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_schedule_exception
    ADD CONSTRAINT aa_schedule_exception_pkey PRIMARY KEY (id);


--
-- Name: idx_aa_meeting_room_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_meeting_room_id ON public.aa_meeting_room USING btree (room_id);


--
-- Name: idx_aa_schedule_attendee_evt_user; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_schedule_attendee_evt_user ON public.aa_schedule_attendee USING btree (event_id, user_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_schedule_attendee_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_schedule_attendee_id ON public.aa_schedule_attendee USING btree (attendee_id);


--
-- Name: idx_aa_schedule_attendee_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_schedule_attendee_user ON public.aa_schedule_attendee USING btree (user_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_schedule_event_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_schedule_event_id ON public.aa_schedule_event USING btree (event_id);


--
-- Name: idx_aa_schedule_event_org; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_schedule_event_org ON public.aa_schedule_event USING btree (organizer_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_schedule_event_room; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_schedule_event_room ON public.aa_schedule_event USING btree (room_id) WHERE ((deleted_at IS NULL) AND ((room_id)::text <> ''::text));


--
-- Name: idx_aa_schedule_exception_evt_date; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_schedule_exception_evt_date ON public.aa_schedule_exception USING btree (event_id, occurrence_date);


--
-- Name: idx_aa_schedule_exception_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_schedule_exception_id ON public.aa_schedule_exception USING btree (exception_id);


--
-- PostgreSQL database dump complete
--

\unrestrict dFSBFEInag421gLR7S7uZU67eeEGPEISDjSIePvBZBabwvAanxUhAza6pbdnPvx

-- ============================================================================
-- CalDAV 双向同步（2026-10）：事件来源映射、账号凭据与冲突保护。
-- 增量迁移见 migrations/aa_schedule/000001_calendar_sync.up.sql。
-- ============================================================================

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
