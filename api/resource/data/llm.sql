-- ============================================================================
-- llm.sql — LLM 网关模块数据库与表结构
-- ============================================================================
-- 依赖：PostgreSQL 16（与 aiarsenal / aa_skill 同实例异库）
-- 关联：user_id 软引用 aiarsenal 库的 aa_user.user_id；org_id 软引用 aa_dept.dept_id
--       （跨库无实际 FK，遵循 aiarsenal 既有模式：以 varchar(32) biz_id 软引用）
-- 约定：每表 id bigserial PK（内部）+ <实体>_id varchar(32) UNIQUE（对外业务 id）
--       FK 列存 biz_id 字符串，根级用 '0'/'；时间用 timestamp；布尔/状态用 smallint
--       数组/复杂结构用 text（GORM datatypes.JSON）
-- 金额：一律 bigint 整数，单位毫 = 万分之一主币单位（$0.0001），展示层换算回主币；
--       模型单价列为「毫/百万 tokens」（Go 侧 types.Milli / model.Milli 自动处理 JSON 边界）
-- 设计：按本期范围裁剪并命名 aa_llm_*；
--       运行时逻辑（路由策略/fallback/cooldown/成本计算/批量 spend 写入）采用通用语义
-- 重建：psql -f resource/data/llm.sql（每表 DROP IF EXISTS，可重复跑）
-- ============================================================================

-- ----------------------------------------------------------------------------
-- Part A — 独立数据库 aa_llm
-- ----------------------------------------------------------------------------

-- CREATE DATABASE 不能在事务内执行，且无 IF NOT EXISTS；用 psql \gexec 条件创建
SELECT 'CREATE DATABASE aa_llm'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'aa_llm')\gexec

\connect aa_llm

-- ============================================================================
-- 基线固化说明（2026-09 squash）
-- 正文已替换为 pg_dump 固化版：合并了历史上全部增量迁移（原 migrations/ 已删，git 可查）。
-- 原文件尾部「
--
-- PostgreSQL database dump
--

\restrict y3o7dULOCnvmboU9Et4h8ySSEfaRRJXBKyoRdkL5TQ5AvckRghEcfgyHT4YMf9P

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

DROP INDEX IF EXISTS public.idx_aa_llm_virtual_key_user;
DROP INDEX IF EXISTS public.idx_aa_llm_virtual_key_team;
DROP INDEX IF EXISTS public.idx_aa_llm_virtual_key_reset;
DROP INDEX IF EXISTS public.idx_aa_llm_virtual_key_hash;
DROP INDEX IF EXISTS public.idx_aa_llm_virtual_key_alias;
DROP INDEX IF EXISTS public.idx_aa_llm_tag_name;
DROP INDEX IF EXISTS public.idx_aa_llm_spend_log_user;
DROP INDEX IF EXISTS public.idx_aa_llm_spend_log_team;
DROP INDEX IF EXISTS public.idx_aa_llm_spend_log_start;
DROP INDEX IF EXISTS public.idx_aa_llm_spend_log_session;
DROP INDEX IF EXISTS public.idx_aa_llm_spend_log_rid;
DROP INDEX IF EXISTS public.idx_aa_llm_spend_log_key;
DROP INDEX IF EXISTS public.idx_aa_llm_spend_log_enduser;
DROP INDEX IF EXISTS public.idx_aa_llm_prompt_version;
DROP INDEX IF EXISTS public.idx_aa_llm_prompt_name;
DROP INDEX IF EXISTS public.idx_aa_llm_prompt_id;
DROP INDEX IF EXISTS public.idx_aa_llm_project_team;
DROP INDEX IF EXISTS public.idx_aa_llm_project_id;
DROP INDEX IF EXISTS public.idx_aa_llm_model_provider;
DROP INDEX IF EXISTS public.idx_aa_llm_model_id;
DROP INDEX IF EXISTS public.idx_aa_llm_model_group_name;
DROP INDEX IF EXISTS public.idx_aa_llm_model_group_id;
DROP INDEX IF EXISTS public.idx_aa_llm_model_group;
DROP INDEX IF EXISTS public.idx_aa_llm_model_deploy;
DROP INDEX IF EXISTS public.idx_aa_llm_health_check_status;
DROP INDEX IF EXISTS public.idx_aa_llm_health_check_model;
DROP INDEX IF EXISTS public.idx_aa_llm_health_check_id;
DROP INDEX IF EXISTS public.idx_aa_llm_error_log_start;
DROP INDEX IF EXISTS public.idx_aa_llm_error_log_rid;
DROP INDEX IF EXISTS public.idx_aa_llm_end_user_id;
DROP INDEX IF EXISTS public.idx_aa_llm_daily_spend_scope;
DROP INDEX IF EXISTS public.idx_aa_llm_daily_spend_model;
DROP INDEX IF EXISTS public.idx_aa_llm_daily_spend_date;
DROP INDEX IF EXISTS public.idx_aa_llm_credential_provider;
DROP INDEX IF EXISTS public.idx_aa_llm_credential_name_provider;
DROP INDEX IF EXISTS public.idx_aa_llm_credential_id;
DROP INDEX IF EXISTS public.idx_aa_llm_budget_request_status;
DROP INDEX IF EXISTS public.idx_aa_llm_budget_request_id;
DROP INDEX IF EXISTS public.idx_aa_llm_budget_request_applicant;
DROP INDEX IF EXISTS public.idx_aa_llm_budget_id;
DROP INDEX IF EXISTS public.idx_aa_llm_audit_log_time;
DROP INDEX IF EXISTS public.idx_aa_llm_audit_log_table;
ALTER TABLE IF EXISTS ONLY public.aa_llm_daily_spend DROP CONSTRAINT IF EXISTS uq_aa_llm_daily_spend;
ALTER TABLE IF EXISTS ONLY public.aa_llm_budget_notice DROP CONSTRAINT IF EXISTS uq_aa_llm_budget_notice;
ALTER TABLE IF EXISTS ONLY public.aa_llm_virtual_key DROP CONSTRAINT IF EXISTS aa_llm_virtual_key_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_llm_tag DROP CONSTRAINT IF EXISTS aa_llm_tag_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_llm_spend_log DROP CONSTRAINT IF EXISTS aa_llm_spend_log_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_llm_prompt DROP CONSTRAINT IF EXISTS aa_llm_prompt_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_llm_project DROP CONSTRAINT IF EXISTS aa_llm_project_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_llm_model DROP CONSTRAINT IF EXISTS aa_llm_model_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_llm_model_group DROP CONSTRAINT IF EXISTS aa_llm_model_group_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_llm_health_check DROP CONSTRAINT IF EXISTS aa_llm_health_check_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_llm_error_log DROP CONSTRAINT IF EXISTS aa_llm_error_log_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_llm_end_user DROP CONSTRAINT IF EXISTS aa_llm_end_user_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_llm_daily_spend DROP CONSTRAINT IF EXISTS aa_llm_daily_spend_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_llm_credential DROP CONSTRAINT IF EXISTS aa_llm_credential_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_llm_budget_request_archive DROP CONSTRAINT IF EXISTS aa_llm_budget_request_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_llm_budget DROP CONSTRAINT IF EXISTS aa_llm_budget_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_llm_budget_notice DROP CONSTRAINT IF EXISTS aa_llm_budget_notice_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_llm_audit_log DROP CONSTRAINT IF EXISTS aa_llm_audit_log_pkey;
ALTER TABLE IF EXISTS public.aa_llm_virtual_key ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_llm_tag ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_llm_spend_log ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_llm_prompt ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_llm_project ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_llm_model_group ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_llm_model ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_llm_health_check ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_llm_error_log ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_llm_end_user ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_llm_daily_spend ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_llm_credential ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_llm_budget_request_archive ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_llm_budget_notice ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_llm_budget ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_llm_audit_log ALTER COLUMN id DROP DEFAULT;
DROP SEQUENCE IF EXISTS public.aa_llm_virtual_key_id_seq;
DROP TABLE IF EXISTS public.aa_llm_virtual_key;
DROP SEQUENCE IF EXISTS public.aa_llm_tag_id_seq;
DROP TABLE IF EXISTS public.aa_llm_tag;
DROP SEQUENCE IF EXISTS public.aa_llm_spend_log_id_seq;
DROP TABLE IF EXISTS public.aa_llm_spend_log;
DROP SEQUENCE IF EXISTS public.aa_llm_prompt_id_seq;
DROP TABLE IF EXISTS public.aa_llm_prompt;
DROP SEQUENCE IF EXISTS public.aa_llm_project_id_seq;
DROP TABLE IF EXISTS public.aa_llm_project;
DROP SEQUENCE IF EXISTS public.aa_llm_model_id_seq;
DROP SEQUENCE IF EXISTS public.aa_llm_model_group_id_seq;
DROP TABLE IF EXISTS public.aa_llm_model_group;
DROP TABLE IF EXISTS public.aa_llm_model;
DROP SEQUENCE IF EXISTS public.aa_llm_health_check_id_seq;
DROP TABLE IF EXISTS public.aa_llm_health_check;
DROP SEQUENCE IF EXISTS public.aa_llm_error_log_id_seq;
DROP TABLE IF EXISTS public.aa_llm_error_log;
DROP SEQUENCE IF EXISTS public.aa_llm_end_user_id_seq;
DROP TABLE IF EXISTS public.aa_llm_end_user;
DROP SEQUENCE IF EXISTS public.aa_llm_daily_spend_id_seq;
DROP TABLE IF EXISTS public.aa_llm_daily_spend;
DROP SEQUENCE IF EXISTS public.aa_llm_credential_id_seq;
DROP TABLE IF EXISTS public.aa_llm_credential;
DROP SEQUENCE IF EXISTS public.aa_llm_budget_request_id_seq;
DROP TABLE IF EXISTS public.aa_llm_budget_request_archive;
DROP SEQUENCE IF EXISTS public.aa_llm_budget_notice_id_seq;
DROP TABLE IF EXISTS public.aa_llm_budget_notice;
DROP SEQUENCE IF EXISTS public.aa_llm_budget_id_seq;
DROP TABLE IF EXISTS public.aa_llm_budget;
DROP SEQUENCE IF EXISTS public.aa_llm_audit_log_id_seq;
DROP TABLE IF EXISTS public.aa_llm_audit_log;
SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: aa_llm_audit_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_llm_audit_log (
    id bigint NOT NULL,
    action character varying(20) DEFAULT ''::character varying NOT NULL,
    table_name character varying(50) DEFAULT ''::character varying NOT NULL,
    object_id character varying(64) DEFAULT ''::character varying NOT NULL,
    before_value text,
    updated_values text,
    changed_by character varying(32) DEFAULT ''::character varying NOT NULL,
    changed_by_api_key character varying(64) DEFAULT ''::character varying NOT NULL,
    updated_at timestamp without time zone
);


--
-- Name: aa_llm_audit_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_llm_audit_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_llm_audit_log_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_llm_audit_log_id_seq OWNED BY public.aa_llm_audit_log.id;


--
-- Name: aa_llm_budget; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_llm_budget (
    id bigint NOT NULL,
    budget_id character varying(32) DEFAULT ''::character varying NOT NULL,
    budget_name character varying(100) DEFAULT ''::character varying NOT NULL,
    max_budget bigint,
    soft_budget bigint,
    spend bigint DEFAULT 0 NOT NULL,
    max_parallel_requests integer,
    tpm_limit bigint,
    rpm_limit bigint,
    model_max_budget text DEFAULT ''::text NOT NULL,
    budget_duration character varying(20),
    budget_reset_at timestamp without time zone,
    allowed_models text DEFAULT ''::text NOT NULL,
    grant_rules jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    updated_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_llm_budget_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_llm_budget_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_llm_budget_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_llm_budget_id_seq OWNED BY public.aa_llm_budget.id;


--
-- Name: aa_llm_budget_notice; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_llm_budget_notice (
    id bigint NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    budget_id character varying(32) DEFAULT ''::character varying NOT NULL,
    window_key character varying(64) DEFAULT ''::character varying NOT NULL,
    level character varying(16) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone DEFAULT now()
);


--
-- Name: aa_llm_budget_notice_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_llm_budget_notice_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_llm_budget_notice_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_llm_budget_notice_id_seq OWNED BY public.aa_llm_budget_notice.id;


--
-- Name: aa_llm_budget_request_archive; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_llm_budget_request_archive (
    id bigint NOT NULL,
    request_id character varying(32) DEFAULT ''::character varying NOT NULL,
    base_budget_id character varying(32) DEFAULT ''::character varying NOT NULL,
    base_budget_name character varying(100) DEFAULT ''::character varying NOT NULL,
    applicant_id character varying(32) DEFAULT ''::character varying NOT NULL,
    applicant_name character varying(100) DEFAULT ''::character varying NOT NULL,
    dept_id character varying(32) DEFAULT ''::character varying NOT NULL,
    amount bigint DEFAULT 0 NOT NULL,
    reason character varying(500) DEFAULT ''::character varying NOT NULL,
    budget_duration character varying(20) DEFAULT '30d'::character varying NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    reviewer_id character varying(32) DEFAULT ''::character varying NOT NULL,
    reviewer_name character varying(100) DEFAULT ''::character varying NOT NULL,
    review_note character varying(500) DEFAULT ''::character varying NOT NULL,
    granted_budget_id character varying(32) DEFAULT ''::character varying NOT NULL,
    reviewed_at timestamp without time zone,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    updated_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_llm_budget_request_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_llm_budget_request_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_llm_budget_request_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_llm_budget_request_id_seq OWNED BY public.aa_llm_budget_request_archive.id;


--
-- Name: aa_llm_credential; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_llm_credential (
    id bigint NOT NULL,
    credential_id character varying(32) DEFAULT ''::character varying NOT NULL,
    credential_name character varying(100) DEFAULT ''::character varying NOT NULL,
    provider_type character varying(50) DEFAULT ''::character varying NOT NULL,
    base_url character varying(255) DEFAULT ''::character varying NOT NULL,
    credential_values text DEFAULT ''::text NOT NULL,
    credential_info text,
    status smallint DEFAULT 1 NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    updated_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_llm_credential_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_llm_credential_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_llm_credential_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_llm_credential_id_seq OWNED BY public.aa_llm_credential.id;


--
-- Name: aa_llm_daily_spend; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_llm_daily_spend (
    id bigint NOT NULL,
    scope_type character varying(20) DEFAULT ''::character varying NOT NULL,
    scope_id character varying(64) DEFAULT ''::character varying NOT NULL,
    date character varying(10) DEFAULT ''::character varying NOT NULL,
    api_key character varying(64) DEFAULT ''::character varying NOT NULL,
    model character varying(100) DEFAULT ''::character varying NOT NULL,
    model_group character varying(100) DEFAULT ''::character varying NOT NULL,
    custom_llm_provider character varying(50) DEFAULT ''::character varying NOT NULL,
    endpoint character varying(50) DEFAULT ''::character varying NOT NULL,
    mcp_namespaced_tool_name character varying(100) DEFAULT ''::character varying NOT NULL,
    prompt_tokens bigint DEFAULT 0 NOT NULL,
    completion_tokens bigint DEFAULT 0 NOT NULL,
    cache_read_input_tokens bigint DEFAULT 0 NOT NULL,
    cache_creation_input_tokens bigint DEFAULT 0 NOT NULL,
    spend bigint DEFAULT 0 NOT NULL,
    api_requests bigint DEFAULT 0 NOT NULL,
    successful_requests bigint DEFAULT 0 NOT NULL,
    failed_requests bigint DEFAULT 0 NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_llm_daily_spend_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_llm_daily_spend_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_llm_daily_spend_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_llm_daily_spend_id_seq OWNED BY public.aa_llm_daily_spend.id;


--
-- Name: aa_llm_end_user; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_llm_end_user (
    id bigint NOT NULL,
    end_user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    alias character varying(100) DEFAULT ''::character varying NOT NULL,
    budget_id character varying(32) DEFAULT ''::character varying NOT NULL,
    allowed_model_region character varying(30) DEFAULT ''::character varying NOT NULL,
    default_model character varying(100) DEFAULT ''::character varying NOT NULL,
    spend bigint DEFAULT 0 NOT NULL,
    blocked smallint DEFAULT 0 NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_llm_end_user_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_llm_end_user_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_llm_end_user_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_llm_end_user_id_seq OWNED BY public.aa_llm_end_user.id;


--
-- Name: aa_llm_error_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_llm_error_log (
    id bigint NOT NULL,
    request_id character varying(64) DEFAULT ''::character varying NOT NULL,
    start_time timestamp without time zone,
    end_time timestamp without time zone,
    api_base character varying(255) DEFAULT ''::character varying NOT NULL,
    model_group character varying(100) DEFAULT ''::character varying NOT NULL,
    model_name character varying(100) DEFAULT ''::character varying NOT NULL,
    model_id character varying(32) DEFAULT ''::character varying NOT NULL,
    request_kwargs text DEFAULT ''::text NOT NULL,
    exception_type character varying(100) DEFAULT ''::character varying NOT NULL,
    exception_string text,
    status_code character varying(10) DEFAULT ''::character varying NOT NULL,
    api_key character varying(64) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    team_id character varying(32) DEFAULT ''::character varying NOT NULL
);


--
-- Name: aa_llm_error_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_llm_error_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_llm_error_log_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_llm_error_log_id_seq OWNED BY public.aa_llm_error_log.id;


--
-- Name: aa_llm_health_check; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_llm_health_check (
    id bigint NOT NULL,
    health_check_id character varying(32) DEFAULT ''::character varying NOT NULL,
    model_name character varying(100) DEFAULT ''::character varying NOT NULL,
    model_id character varying(32) DEFAULT ''::character varying NOT NULL,
    status character varying(20) DEFAULT ''::character varying NOT NULL,
    healthy_count integer DEFAULT 0 NOT NULL,
    unhealthy_count integer DEFAULT 0 NOT NULL,
    error_message character varying(500) DEFAULT ''::character varying NOT NULL,
    response_time_ms double precision,
    details text,
    checked_by character varying(32) DEFAULT ''::character varying NOT NULL,
    checked_at timestamp without time zone,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_llm_health_check_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_llm_health_check_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_llm_health_check_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_llm_health_check_id_seq OWNED BY public.aa_llm_health_check.id;


--
-- Name: aa_llm_model; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_llm_model (
    id bigint NOT NULL,
    model_id character varying(32) DEFAULT ''::character varying NOT NULL,
    model_name character varying(100) DEFAULT ''::character varying NOT NULL,
    model_group character varying(100) DEFAULT ''::character varying NOT NULL,
    provider character varying(50) DEFAULT ''::character varying NOT NULL,
    provider_model character varying(100) DEFAULT ''::character varying NOT NULL,
    credential_id character varying(32) DEFAULT ''::character varying NOT NULL,
    extra_params text DEFAULT ''::text NOT NULL,
    model_info text,
    input_price_per_million bigint DEFAULT 0,
    output_price_per_million bigint DEFAULT 0,
    cache_read_price_per_million bigint DEFAULT 0,
    cache_creation_price_per_million bigint DEFAULT 0,
    max_input_tokens integer DEFAULT 0,
    max_output_tokens integer DEFAULT 0,
    tpm_limit bigint DEFAULT 0,
    rpm_limit bigint DEFAULT 0,
    weight integer DEFAULT 1 NOT NULL,
    blocked smallint DEFAULT 0 NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    updated_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_llm_model_group; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_llm_model_group (
    id bigint NOT NULL,
    group_id character varying(32) DEFAULT ''::character varying NOT NULL,
    group_name character varying(100) DEFAULT ''::character varying NOT NULL,
    description text,
    status smallint DEFAULT 1 NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    updated_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_llm_model_group_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_llm_model_group_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_llm_model_group_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_llm_model_group_id_seq OWNED BY public.aa_llm_model_group.id;


--
-- Name: aa_llm_model_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_llm_model_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_llm_model_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_llm_model_id_seq OWNED BY public.aa_llm_model.id;


--
-- Name: aa_llm_project; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_llm_project (
    id bigint NOT NULL,
    project_id character varying(32) DEFAULT ''::character varying NOT NULL,
    project_alias character varying(100) DEFAULT ''::character varying NOT NULL,
    description character varying(500) DEFAULT ''::character varying NOT NULL,
    team_id character varying(32) DEFAULT ''::character varying NOT NULL,
    budget_id character varying(32) DEFAULT ''::character varying NOT NULL,
    models text DEFAULT ''::text NOT NULL,
    model_rpm_limit text DEFAULT ''::text NOT NULL,
    model_tpm_limit text DEFAULT ''::text NOT NULL,
    spend bigint DEFAULT 0 NOT NULL,
    model_spend text DEFAULT ''::text NOT NULL,
    blocked smallint DEFAULT 0 NOT NULL,
    metadata text DEFAULT ''::text NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    updated_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_llm_project_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_llm_project_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_llm_project_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_llm_project_id_seq OWNED BY public.aa_llm_project.id;


--
-- Name: aa_llm_prompt; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_llm_prompt (
    id bigint NOT NULL,
    prompt_id character varying(32) DEFAULT ''::character varying NOT NULL,
    name character varying(100) DEFAULT ''::character varying NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    environment character varying(30) DEFAULT 'development'::character varying NOT NULL,
    extra_params text DEFAULT ''::text NOT NULL,
    prompt_info text,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    updated_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_llm_prompt_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_llm_prompt_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_llm_prompt_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_llm_prompt_id_seq OWNED BY public.aa_llm_prompt.id;


--
-- Name: aa_llm_spend_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_llm_spend_log (
    id bigint NOT NULL,
    request_id character varying(64) DEFAULT ''::character varying NOT NULL,
    call_type character varying(30) DEFAULT ''::character varying NOT NULL,
    api_key character varying(64) DEFAULT ''::character varying NOT NULL,
    spend bigint DEFAULT 0 NOT NULL,
    total_tokens integer DEFAULT 0 NOT NULL,
    prompt_tokens integer DEFAULT 0 NOT NULL,
    completion_tokens integer DEFAULT 0 NOT NULL,
    cache_read_input_tokens integer DEFAULT 0 NOT NULL,
    cache_creation_input_tokens integer DEFAULT 0 NOT NULL,
    start_time timestamp without time zone,
    end_time timestamp without time zone,
    request_duration_ms integer,
    completion_start_time timestamp without time zone,
    model character varying(100) DEFAULT ''::character varying NOT NULL,
    model_id character varying(32) DEFAULT ''::character varying NOT NULL,
    model_group character varying(100) DEFAULT ''::character varying NOT NULL,
    custom_llm_provider character varying(50) DEFAULT ''::character varying NOT NULL,
    api_base character varying(255) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    team_id character varying(32) DEFAULT ''::character varying NOT NULL,
    org_id character varying(32) DEFAULT ''::character varying NOT NULL,
    project_id character varying(32) DEFAULT ''::character varying NOT NULL,
    end_user character varying(64) DEFAULT ''::character varying NOT NULL,
    session_id character varying(64) DEFAULT ''::character varying NOT NULL,
    requester_ip_address character varying(64) DEFAULT ''::character varying NOT NULL,
    cache_hit character varying(10) DEFAULT ''::character varying NOT NULL,
    cache_key character varying(255) DEFAULT ''::character varying NOT NULL,
    request_tags text DEFAULT ''::text NOT NULL,
    metadata text DEFAULT ''::text NOT NULL,
    messages text,
    response text,
    status character varying(20) DEFAULT ''::character varying NOT NULL
);


--
-- Name: aa_llm_spend_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_llm_spend_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_llm_spend_log_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_llm_spend_log_id_seq OWNED BY public.aa_llm_spend_log.id;


--
-- Name: aa_llm_tag; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_llm_tag (
    id bigint NOT NULL,
    tag_name character varying(64) DEFAULT ''::character varying NOT NULL,
    description character varying(255) DEFAULT ''::character varying NOT NULL,
    models text DEFAULT ''::text NOT NULL,
    model_info text,
    spend bigint DEFAULT 0 NOT NULL,
    budget_id character varying(32) DEFAULT ''::character varying NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    updated_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_llm_tag_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_llm_tag_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_llm_tag_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_llm_tag_id_seq OWNED BY public.aa_llm_tag.id;


--
-- Name: aa_llm_virtual_key; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_llm_virtual_key (
    id bigint NOT NULL,
    token_hash character varying(64) DEFAULT ''::character varying NOT NULL,
    key_alias character varying(100) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    team_id character varying(32) DEFAULT ''::character varying NOT NULL,
    project_id character varying(32) DEFAULT ''::character varying NOT NULL,
    org_id character varying(32) DEFAULT ''::character varying NOT NULL,
    models text DEFAULT ''::text NOT NULL,
    aliases text DEFAULT ''::text NOT NULL,
    config text DEFAULT ''::text NOT NULL,
    router_settings text DEFAULT ''::text NOT NULL,
    permissions text DEFAULT ''::text NOT NULL,
    spend bigint DEFAULT 0 NOT NULL,
    max_budget bigint,
    soft_budget bigint,
    budget_id character varying(32) DEFAULT ''::character varying NOT NULL,
    tpm_limit bigint,
    rpm_limit bigint,
    max_parallel_requests integer,
    budget_duration character varying(20),
    budget_reset_at timestamp without time zone,
    model_spend text DEFAULT ''::text NOT NULL,
    model_max_budget text DEFAULT ''::text NOT NULL,
    allowed_models text DEFAULT ''::text NOT NULL,
    allowed_routes text DEFAULT ''::text NOT NULL,
    expires timestamp without time zone,
    blocked smallint DEFAULT 0 NOT NULL,
    last_active timestamp without time zone,
    auto_rotate smallint DEFAULT 0 NOT NULL,
    rotation_interval character varying(20),
    last_rotation_at timestamp without time zone,
    key_rotation_at timestamp without time zone,
    rotation_count integer DEFAULT 0 NOT NULL,
    deprecated_token_hash character varying(64) DEFAULT ''::character varying NOT NULL,
    deprecate_expires_at timestamp without time zone,
    metadata text DEFAULT ''::text NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    updated_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_llm_virtual_key_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_llm_virtual_key_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_llm_virtual_key_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_llm_virtual_key_id_seq OWNED BY public.aa_llm_virtual_key.id;


--
-- Name: aa_llm_audit_log id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_audit_log ALTER COLUMN id SET DEFAULT nextval('public.aa_llm_audit_log_id_seq'::regclass);


--
-- Name: aa_llm_budget id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_budget ALTER COLUMN id SET DEFAULT nextval('public.aa_llm_budget_id_seq'::regclass);


--
-- Name: aa_llm_budget_notice id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_budget_notice ALTER COLUMN id SET DEFAULT nextval('public.aa_llm_budget_notice_id_seq'::regclass);


--
-- Name: aa_llm_budget_request_archive id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_budget_request_archive ALTER COLUMN id SET DEFAULT nextval('public.aa_llm_budget_request_id_seq'::regclass);


--
-- Name: aa_llm_credential id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_credential ALTER COLUMN id SET DEFAULT nextval('public.aa_llm_credential_id_seq'::regclass);


--
-- Name: aa_llm_daily_spend id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_daily_spend ALTER COLUMN id SET DEFAULT nextval('public.aa_llm_daily_spend_id_seq'::regclass);


--
-- Name: aa_llm_end_user id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_end_user ALTER COLUMN id SET DEFAULT nextval('public.aa_llm_end_user_id_seq'::regclass);


--
-- Name: aa_llm_error_log id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_error_log ALTER COLUMN id SET DEFAULT nextval('public.aa_llm_error_log_id_seq'::regclass);


--
-- Name: aa_llm_health_check id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_health_check ALTER COLUMN id SET DEFAULT nextval('public.aa_llm_health_check_id_seq'::regclass);


--
-- Name: aa_llm_model id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_model ALTER COLUMN id SET DEFAULT nextval('public.aa_llm_model_id_seq'::regclass);


--
-- Name: aa_llm_model_group id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_model_group ALTER COLUMN id SET DEFAULT nextval('public.aa_llm_model_group_id_seq'::regclass);


--
-- Name: aa_llm_project id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_project ALTER COLUMN id SET DEFAULT nextval('public.aa_llm_project_id_seq'::regclass);


--
-- Name: aa_llm_prompt id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_prompt ALTER COLUMN id SET DEFAULT nextval('public.aa_llm_prompt_id_seq'::regclass);


--
-- Name: aa_llm_spend_log id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_spend_log ALTER COLUMN id SET DEFAULT nextval('public.aa_llm_spend_log_id_seq'::regclass);


--
-- Name: aa_llm_tag id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_tag ALTER COLUMN id SET DEFAULT nextval('public.aa_llm_tag_id_seq'::regclass);


--
-- Name: aa_llm_virtual_key id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_virtual_key ALTER COLUMN id SET DEFAULT nextval('public.aa_llm_virtual_key_id_seq'::regclass);


--
-- Data for Name: aa_llm_audit_log; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_llm_audit_log (id, action, table_name, object_id, before_value, updated_values, changed_by, changed_by_api_key, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_llm_budget; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_llm_budget (id, budget_id, budget_name, max_budget, soft_budget, spend, max_parallel_requests, tpm_limit, rpm_limit, model_max_budget, budget_duration, budget_reset_at, allowed_models, grant_rules, created_by, updated_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_llm_budget_notice; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_llm_budget_notice (id, user_id, budget_id, window_key, level, created_at) FROM stdin;
\.


--
-- Data for Name: aa_llm_budget_request_archive; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_llm_budget_request_archive (id, request_id, base_budget_id, base_budget_name, applicant_id, applicant_name, dept_id, amount, reason, budget_duration, status, reviewer_id, reviewer_name, review_note, granted_budget_id, reviewed_at, created_by, updated_by, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_llm_credential; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_llm_credential (id, credential_id, credential_name, provider_type, base_url, credential_values, credential_info, status, created_by, updated_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_llm_daily_spend; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_llm_daily_spend (id, scope_type, scope_id, date, api_key, model, model_group, custom_llm_provider, endpoint, mcp_namespaced_tool_name, prompt_tokens, completion_tokens, cache_read_input_tokens, cache_creation_input_tokens, spend, api_requests, successful_requests, failed_requests, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_llm_end_user; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_llm_end_user (id, end_user_id, alias, budget_id, allowed_model_region, default_model, spend, blocked, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_llm_error_log; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_llm_error_log (id, request_id, start_time, end_time, api_base, model_group, model_name, model_id, request_kwargs, exception_type, exception_string, status_code, api_key, user_id, team_id) FROM stdin;
\.


--
-- Data for Name: aa_llm_health_check; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_llm_health_check (id, health_check_id, model_name, model_id, status, healthy_count, unhealthy_count, error_message, response_time_ms, details, checked_by, checked_at, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_llm_model; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_llm_model (id, model_id, model_name, model_group, provider, provider_model, credential_id, extra_params, model_info, input_price_per_million, output_price_per_million, cache_read_price_per_million, cache_creation_price_per_million, max_input_tokens, max_output_tokens, tpm_limit, rpm_limit, weight, blocked, status, created_by, updated_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_llm_model_group; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_llm_model_group (id, group_id, group_name, description, status, created_by, updated_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_llm_project; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_llm_project (id, project_id, project_alias, description, team_id, budget_id, models, model_rpm_limit, model_tpm_limit, spend, model_spend, blocked, metadata, created_by, updated_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_llm_prompt; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_llm_prompt (id, prompt_id, name, version, environment, extra_params, prompt_info, created_by, updated_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_llm_spend_log; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_llm_spend_log (id, request_id, call_type, api_key, spend, total_tokens, prompt_tokens, completion_tokens, cache_read_input_tokens, cache_creation_input_tokens, start_time, end_time, request_duration_ms, completion_start_time, model, model_id, model_group, custom_llm_provider, api_base, user_id, team_id, org_id, project_id, end_user, session_id, requester_ip_address, cache_hit, cache_key, request_tags, metadata, messages, response, status) FROM stdin;
\.


--
-- Data for Name: aa_llm_tag; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_llm_tag (id, tag_name, description, models, model_info, spend, budget_id, created_by, updated_by, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_llm_virtual_key; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_llm_virtual_key (id, token_hash, key_alias, user_id, team_id, project_id, org_id, models, aliases, config, router_settings, permissions, spend, max_budget, soft_budget, budget_id, tpm_limit, rpm_limit, max_parallel_requests, budget_duration, budget_reset_at, model_spend, model_max_budget, allowed_models, allowed_routes, expires, blocked, last_active, auto_rotate, rotation_interval, last_rotation_at, key_rotation_at, rotation_count, deprecated_token_hash, deprecate_expires_at, metadata, created_by, updated_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Name: aa_llm_audit_log_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_llm_audit_log_id_seq', 1, false);


--
-- Name: aa_llm_budget_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_llm_budget_id_seq', 1, false);


--
-- Name: aa_llm_budget_notice_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_llm_budget_notice_id_seq', 1, false);


--
-- Name: aa_llm_budget_request_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_llm_budget_request_id_seq', 1, false);


--
-- Name: aa_llm_credential_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_llm_credential_id_seq', 1, false);


--
-- Name: aa_llm_daily_spend_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_llm_daily_spend_id_seq', 1, false);


--
-- Name: aa_llm_end_user_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_llm_end_user_id_seq', 1, false);


--
-- Name: aa_llm_error_log_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_llm_error_log_id_seq', 1, false);


--
-- Name: aa_llm_health_check_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_llm_health_check_id_seq', 1, false);


--
-- Name: aa_llm_model_group_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_llm_model_group_id_seq', 1, false);


--
-- Name: aa_llm_model_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_llm_model_id_seq', 1, false);


--
-- Name: aa_llm_project_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_llm_project_id_seq', 1, false);


--
-- Name: aa_llm_prompt_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_llm_prompt_id_seq', 1, false);


--
-- Name: aa_llm_spend_log_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_llm_spend_log_id_seq', 1, false);


--
-- Name: aa_llm_tag_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_llm_tag_id_seq', 1, false);


--
-- Name: aa_llm_virtual_key_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_llm_virtual_key_id_seq', 1, false);


--
-- Name: aa_llm_audit_log aa_llm_audit_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_audit_log
    ADD CONSTRAINT aa_llm_audit_log_pkey PRIMARY KEY (id);


--
-- Name: aa_llm_budget_notice aa_llm_budget_notice_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_budget_notice
    ADD CONSTRAINT aa_llm_budget_notice_pkey PRIMARY KEY (id);


--
-- Name: aa_llm_budget aa_llm_budget_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_budget
    ADD CONSTRAINT aa_llm_budget_pkey PRIMARY KEY (id);


--
-- Name: aa_llm_budget_request_archive aa_llm_budget_request_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_budget_request_archive
    ADD CONSTRAINT aa_llm_budget_request_pkey PRIMARY KEY (id);


--
-- Name: aa_llm_credential aa_llm_credential_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_credential
    ADD CONSTRAINT aa_llm_credential_pkey PRIMARY KEY (id);


--
-- Name: aa_llm_daily_spend aa_llm_daily_spend_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_daily_spend
    ADD CONSTRAINT aa_llm_daily_spend_pkey PRIMARY KEY (id);


--
-- Name: aa_llm_end_user aa_llm_end_user_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_end_user
    ADD CONSTRAINT aa_llm_end_user_pkey PRIMARY KEY (id);


--
-- Name: aa_llm_error_log aa_llm_error_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_error_log
    ADD CONSTRAINT aa_llm_error_log_pkey PRIMARY KEY (id);


--
-- Name: aa_llm_health_check aa_llm_health_check_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_health_check
    ADD CONSTRAINT aa_llm_health_check_pkey PRIMARY KEY (id);


--
-- Name: aa_llm_model_group aa_llm_model_group_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_model_group
    ADD CONSTRAINT aa_llm_model_group_pkey PRIMARY KEY (id);


--
-- Name: aa_llm_model aa_llm_model_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_model
    ADD CONSTRAINT aa_llm_model_pkey PRIMARY KEY (id);


--
-- Name: aa_llm_project aa_llm_project_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_project
    ADD CONSTRAINT aa_llm_project_pkey PRIMARY KEY (id);


--
-- Name: aa_llm_prompt aa_llm_prompt_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_prompt
    ADD CONSTRAINT aa_llm_prompt_pkey PRIMARY KEY (id);


--
-- Name: aa_llm_spend_log aa_llm_spend_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_spend_log
    ADD CONSTRAINT aa_llm_spend_log_pkey PRIMARY KEY (id);


--
-- Name: aa_llm_tag aa_llm_tag_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_tag
    ADD CONSTRAINT aa_llm_tag_pkey PRIMARY KEY (id);


--
-- Name: aa_llm_virtual_key aa_llm_virtual_key_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_virtual_key
    ADD CONSTRAINT aa_llm_virtual_key_pkey PRIMARY KEY (id);


--
-- Name: aa_llm_budget_notice uq_aa_llm_budget_notice; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_budget_notice
    ADD CONSTRAINT uq_aa_llm_budget_notice UNIQUE (user_id, budget_id, window_key, level);


--
-- Name: aa_llm_daily_spend uq_aa_llm_daily_spend; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_llm_daily_spend
    ADD CONSTRAINT uq_aa_llm_daily_spend UNIQUE (scope_type, scope_id, date, api_key, model, custom_llm_provider, mcp_namespaced_tool_name, endpoint);


--
-- Name: idx_aa_llm_audit_log_table; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_audit_log_table ON public.aa_llm_audit_log USING btree (table_name, object_id);


--
-- Name: idx_aa_llm_audit_log_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_audit_log_time ON public.aa_llm_audit_log USING btree (updated_at);


--
-- Name: idx_aa_llm_budget_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_llm_budget_id ON public.aa_llm_budget USING btree (budget_id);


--
-- Name: idx_aa_llm_budget_request_applicant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_budget_request_applicant ON public.aa_llm_budget_request_archive USING btree (applicant_id, created_at DESC);


--
-- Name: idx_aa_llm_budget_request_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_llm_budget_request_id ON public.aa_llm_budget_request_archive USING btree (request_id);


--
-- Name: idx_aa_llm_budget_request_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_budget_request_status ON public.aa_llm_budget_request_archive USING btree (status, dept_id);


--
-- Name: idx_aa_llm_credential_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_llm_credential_id ON public.aa_llm_credential USING btree (credential_id);


--
-- Name: idx_aa_llm_credential_name_provider; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_llm_credential_name_provider ON public.aa_llm_credential USING btree (credential_name, provider_type) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_llm_credential_provider; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_credential_provider ON public.aa_llm_credential USING btree (provider_type);


--
-- Name: idx_aa_llm_daily_spend_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_daily_spend_date ON public.aa_llm_daily_spend USING btree (date);


--
-- Name: idx_aa_llm_daily_spend_model; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_daily_spend_model ON public.aa_llm_daily_spend USING btree (model);


--
-- Name: idx_aa_llm_daily_spend_scope; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_daily_spend_scope ON public.aa_llm_daily_spend USING btree (scope_type, scope_id, date);


--
-- Name: idx_aa_llm_end_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_llm_end_user_id ON public.aa_llm_end_user USING btree (end_user_id);


--
-- Name: idx_aa_llm_error_log_rid; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_error_log_rid ON public.aa_llm_error_log USING btree (request_id);


--
-- Name: idx_aa_llm_error_log_start; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_error_log_start ON public.aa_llm_error_log USING btree (start_time);


--
-- Name: idx_aa_llm_health_check_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_llm_health_check_id ON public.aa_llm_health_check USING btree (health_check_id);


--
-- Name: idx_aa_llm_health_check_model; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_health_check_model ON public.aa_llm_health_check USING btree (model_id, model_name, checked_at DESC);


--
-- Name: idx_aa_llm_health_check_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_health_check_status ON public.aa_llm_health_check USING btree (status);


--
-- Name: idx_aa_llm_model_deploy; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_llm_model_deploy ON public.aa_llm_model USING btree (model_group, provider_model, credential_id) WHERE ((deleted_at IS NULL) AND ((credential_id)::text <> ''::text));


--
-- Name: idx_aa_llm_model_group; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_model_group ON public.aa_llm_model USING btree (model_group) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_llm_model_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_llm_model_group_id ON public.aa_llm_model_group USING btree (group_id);


--
-- Name: idx_aa_llm_model_group_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_llm_model_group_name ON public.aa_llm_model_group USING btree (group_name) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_llm_model_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_llm_model_id ON public.aa_llm_model USING btree (model_id);


--
-- Name: idx_aa_llm_model_provider; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_model_provider ON public.aa_llm_model USING btree (provider) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_llm_project_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_llm_project_id ON public.aa_llm_project USING btree (project_id);


--
-- Name: idx_aa_llm_project_team; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_project_team ON public.aa_llm_project USING btree (team_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_llm_prompt_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_prompt_id ON public.aa_llm_prompt USING btree (prompt_id, environment);


--
-- Name: idx_aa_llm_prompt_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_prompt_name ON public.aa_llm_prompt USING btree (name) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_llm_prompt_version; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_prompt_version ON public.aa_llm_prompt USING btree (prompt_id, version, environment);


--
-- Name: idx_aa_llm_spend_log_enduser; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_spend_log_enduser ON public.aa_llm_spend_log USING btree (end_user);


--
-- Name: idx_aa_llm_spend_log_key; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_spend_log_key ON public.aa_llm_spend_log USING btree (api_key, start_time);


--
-- Name: idx_aa_llm_spend_log_rid; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_llm_spend_log_rid ON public.aa_llm_spend_log USING btree (request_id);


--
-- Name: idx_aa_llm_spend_log_session; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_spend_log_session ON public.aa_llm_spend_log USING btree (session_id);


--
-- Name: idx_aa_llm_spend_log_start; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_spend_log_start ON public.aa_llm_spend_log USING btree (start_time);


--
-- Name: idx_aa_llm_spend_log_team; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_spend_log_team ON public.aa_llm_spend_log USING btree (team_id, start_time);


--
-- Name: idx_aa_llm_spend_log_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_spend_log_user ON public.aa_llm_spend_log USING btree (user_id, start_time);


--
-- Name: idx_aa_llm_tag_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_llm_tag_name ON public.aa_llm_tag USING btree (tag_name);


--
-- Name: idx_aa_llm_virtual_key_alias; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_virtual_key_alias ON public.aa_llm_virtual_key USING btree (key_alias) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_llm_virtual_key_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_llm_virtual_key_hash ON public.aa_llm_virtual_key USING btree (token_hash);


--
-- Name: idx_aa_llm_virtual_key_reset; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_virtual_key_reset ON public.aa_llm_virtual_key USING btree (budget_reset_at, expires) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_llm_virtual_key_team; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_virtual_key_team ON public.aa_llm_virtual_key USING btree (team_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_llm_virtual_key_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_llm_virtual_key_user ON public.aa_llm_virtual_key USING btree (user_id) WHERE (deleted_at IS NULL);


--
-- PostgreSQL database dump complete
--

\unrestrict y3o7dULOCnvmboU9Et4h8ySSEfaRRJXBKyoRdkL5TQ5AvckRghEcfgyHT4YMf9P

