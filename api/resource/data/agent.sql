-- ============================================================================
-- agent.sql — Agent 模块数据库与表结构
-- ============================================================================
-- 依赖：PostgreSQL 16（与 aiarsenal / aa_skill / aa_llm 同实例异库）
-- 关联：user_id 软引用 aiarsenal 库的 aa_user.user_id（跨库无实际 FK，
--       遵循 aiarsenal 既有模式：以 varchar(32) biz_id 软引用）
-- 约定：每表 id bigserial PK（内部）+ <实体>_id varchar(32) UNIQUE（对外业务 id）
--       FK 列存 biz_id 字符串，根级用 '0'；时间用 timestamp；布尔/状态用 smallint
--       数组/复杂结构用 jsonb（GORM json.RawMessage / datatypes.JSON）
-- 来源：schema 翻译为 PostgreSQL + aa_agent_ 前缀
--       v1 范围：配置中心 / Skill / 安全扫描 / AI Gateway 治理记录 / 审计配额（20 表）
--       K8s 运行时层（instance/runtime/team/控制面等）留 v2
-- 重建：psql -f resource/data/agent.sql（每表 DROP IF EXISTS，可重复跑）
-- ============================================================================

-- ----------------------------------------------------------------------------
-- Part A — 独立数据库 aa_agent
-- ----------------------------------------------------------------------------

-- CREATE DATABASE 不能在事务内执行，且无 IF NOT EXISTS；用 psql \gexec 条件创建
SELECT 'CREATE DATABASE aa_agent'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'aa_agent')\gexec

\connect aa_agent

-- ============================================================================
-- 基线固化说明（2026-09 squash）
-- 正文已替换为 pg_dump 固化版：合并了历史上全部增量迁移（原 migrations/ 已删，git 可查）。
-- 原文件尾部「
--
-- PostgreSQL database dump
--

\restrict oezNLvxU6qzfdqxFDox53FOUkAmcGg0OpT3wzrrJ7dJT4O8HSO8lIyCBhDfCOOL

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

DROP INDEX IF EXISTS public.idx_aa_agent_workspace_file_audit_instance;
DROP INDEX IF EXISTS public.idx_aa_agent_workspace_file_audit_id;
DROP INDEX IF EXISTS public.idx_aa_agent_user_quota_user;
DROP INDEX IF EXISTS public.idx_aa_agent_user_quota_id;
DROP INDEX IF EXISTS public.idx_aa_agent_team_work_item_work;
DROP INDEX IF EXISTS public.idx_aa_agent_team_work_item_root;
DROP INDEX IF EXISTS public.idx_aa_agent_team_work_item_id;
DROP INDEX IF EXISTS public.idx_aa_agent_team_user_name;
DROP INDEX IF EXISTS public.idx_aa_agent_team_task_team;
DROP INDEX IF EXISTS public.idx_aa_agent_team_task_id;
DROP INDEX IF EXISTS public.idx_aa_agent_team_member_team;
DROP INDEX IF EXISTS public.idx_aa_agent_team_member_pair;
DROP INDEX IF EXISTS public.idx_aa_agent_team_member_id;
DROP INDEX IF EXISTS public.idx_aa_agent_team_id;
DROP INDEX IF EXISTS public.idx_aa_agent_team_event_team_seq;
DROP INDEX IF EXISTS public.idx_aa_agent_team_event_id;
DROP INDEX IF EXISTS public.idx_aa_agent_team_event_event;
DROP INDEX IF EXISTS public.idx_aa_agent_system_image_setting_id;
DROP INDEX IF EXISTS public.idx_aa_agent_system_image_setting_agent_version;
DROP INDEX IF EXISTS public.idx_aa_agent_security_scan_report_job;
DROP INDEX IF EXISTS public.idx_aa_agent_security_scan_report_id;
DROP INDEX IF EXISTS public.idx_aa_agent_security_scan_job_status;
DROP INDEX IF EXISTS public.idx_aa_agent_security_scan_job_item_job;
DROP INDEX IF EXISTS public.idx_aa_agent_security_scan_job_item_id;
DROP INDEX IF EXISTS public.idx_aa_agent_security_scan_job_id;
DROP INDEX IF EXISTS public.idx_aa_agent_security_scan_config_id;
DROP INDEX IF EXISTS public.idx_aa_agent_runtime_rollout_status;
DROP INDEX IF EXISTS public.idx_aa_agent_runtime_rollout_id;
DROP INDEX IF EXISTS public.idx_aa_agent_runtime_pod_state;
DROP INDEX IF EXISTS public.idx_aa_agent_runtime_pod_rollout;
DROP INDEX IF EXISTS public.idx_aa_agent_runtime_pod_name;
DROP INDEX IF EXISTS public.idx_aa_agent_runtime_pod_id;
DROP INDEX IF EXISTS public.idx_aa_agent_risk_rule_id;
DROP INDEX IF EXISTS public.idx_aa_agent_risk_rule_enabled;
DROP INDEX IF EXISTS public.idx_aa_agent_risk_rule_code;
DROP INDEX IF EXISTS public.idx_aa_agent_risk_hit_trace;
DROP INDEX IF EXISTS public.idx_aa_agent_risk_hit_rule;
DROP INDEX IF EXISTS public.idx_aa_agent_risk_hit_id;
DROP INDEX IF EXISTS public.idx_aa_agent_persistent_volume_name;
DROP INDEX IF EXISTS public.idx_aa_agent_persistent_volume_instance;
DROP INDEX IF EXISTS public.idx_aa_agent_persistent_volume_id;
DROP INDEX IF EXISTS public.idx_aa_agent_openclaw_injection_snapshot_instance;
DROP INDEX IF EXISTS public.idx_aa_agent_openclaw_injection_snapshot_id;
DROP INDEX IF EXISTS public.idx_aa_agent_model_provider;
DROP INDEX IF EXISTS public.idx_aa_agent_model_invocation_user;
DROP INDEX IF EXISTS public.idx_aa_agent_model_invocation_trace;
DROP INDEX IF EXISTS public.idx_aa_agent_model_invocation_model;
DROP INDEX IF EXISTS public.idx_aa_agent_model_invocation_id;
DROP INDEX IF EXISTS public.idx_aa_agent_model_id;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_user_name;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_user;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_usage_recorded;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_usage_instance;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_usage_id;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_status;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_skill_pair;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_skill_instance;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_skill_id;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_runtime_status_instance;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_runtime_status_id;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_runtime_status_history_instance;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_runtime_status_history_id;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_runtime_binding_instance;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_runtime_binding_id;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_mcp_pair;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_mcp_instance;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_mcp_id;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_id;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_external_access_slug;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_external_access_instance;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_external_access_id;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_desired_state_instance;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_config_revision_instance;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_config_revision_id;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_config_change_instance;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_config_change_id;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_command_instance;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_command_id;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_agent_instance;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_agent_id;
DROP INDEX IF EXISTS public.idx_aa_agent_cost_record_user;
DROP INDEX IF EXISTS public.idx_aa_agent_cost_record_trace;
DROP INDEX IF EXISTS public.idx_aa_agent_cost_record_id;
DROP INDEX IF EXISTS public.idx_aa_agent_config_resource_user;
DROP INDEX IF EXISTS public.idx_aa_agent_config_resource_type;
DROP INDEX IF EXISTS public.idx_aa_agent_config_resource_id;
DROP INDEX IF EXISTS public.idx_aa_agent_config_bundle_user;
DROP INDEX IF EXISTS public.idx_aa_agent_config_bundle_skill_id;
DROP INDEX IF EXISTS public.idx_aa_agent_config_bundle_skill_bundle;
DROP INDEX IF EXISTS public.idx_aa_agent_config_bundle_item_id;
DROP INDEX IF EXISTS public.idx_aa_agent_config_bundle_item_bundle;
DROP INDEX IF EXISTS public.idx_aa_agent_config_bundle_id;
DROP INDEX IF EXISTS public.idx_aa_agent_backup_schedule_instance;
DROP INDEX IF EXISTS public.idx_aa_agent_backup_schedule_id;
DROP INDEX IF EXISTS public.idx_aa_agent_backup_instance;
DROP INDEX IF EXISTS public.idx_aa_agent_backup_id;
DROP INDEX IF EXISTS public.idx_aa_agent_audit_log_user;
DROP INDEX IF EXISTS public.idx_aa_agent_audit_log_id;
DROP INDEX IF EXISTS public.idx_aa_agent_audit_log_action;
DROP INDEX IF EXISTS public.idx_aa_agent_audit_event_user;
DROP INDEX IF EXISTS public.idx_aa_agent_audit_event_trace;
DROP INDEX IF EXISTS public.idx_aa_agent_runtime_pod_rollout;
DROP INDEX IF EXISTS public.idx_aa_agent_instance_usage_recorded;
DROP INDEX IF EXISTS public.idx_aa_agent_audit_event_id;
ALTER TABLE IF EXISTS ONLY public.aa_agent_workspace_file_audit DROP CONSTRAINT IF EXISTS aa_agent_workspace_file_audit_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_user DROP CONSTRAINT IF EXISTS aa_agent_user_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_user DROP CONSTRAINT IF EXISTS aa_agent_user_agent_user_id_key;
ALTER TABLE IF EXISTS ONLY public.aa_agent_user DROP CONSTRAINT IF EXISTS aa_agent_user_username_key;
ALTER TABLE IF EXISTS ONLY public.aa_agent_user_quota DROP CONSTRAINT IF EXISTS aa_agent_user_quota_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_team_work_item DROP CONSTRAINT IF EXISTS aa_agent_team_work_item_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_team_task DROP CONSTRAINT IF EXISTS aa_agent_team_task_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_team DROP CONSTRAINT IF EXISTS aa_agent_team_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_team_member DROP CONSTRAINT IF EXISTS aa_agent_team_member_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_team_event DROP CONSTRAINT IF EXISTS aa_agent_team_event_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_system_image_setting DROP CONSTRAINT IF EXISTS aa_agent_system_image_setting_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_security_scan_report DROP CONSTRAINT IF EXISTS aa_agent_security_scan_report_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_security_scan_job DROP CONSTRAINT IF EXISTS aa_agent_security_scan_job_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_security_scan_job_item DROP CONSTRAINT IF EXISTS aa_agent_security_scan_job_item_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_security_scan_config DROP CONSTRAINT IF EXISTS aa_agent_security_scan_config_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_runtime_rollout DROP CONSTRAINT IF EXISTS aa_agent_runtime_rollout_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_runtime_pod DROP CONSTRAINT IF EXISTS aa_agent_runtime_pod_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_risk_rule DROP CONSTRAINT IF EXISTS aa_agent_risk_rule_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_risk_hit DROP CONSTRAINT IF EXISTS aa_agent_risk_hit_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_persistent_volume DROP CONSTRAINT IF EXISTS aa_agent_persistent_volume_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_openclaw_injection_snapshot DROP CONSTRAINT IF EXISTS aa_agent_openclaw_injection_snapshot_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_model DROP CONSTRAINT IF EXISTS aa_agent_model_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_model_invocation DROP CONSTRAINT IF EXISTS aa_agent_model_invocation_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_instance_usage DROP CONSTRAINT IF EXISTS aa_agent_instance_usage_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_instance_skill DROP CONSTRAINT IF EXISTS aa_agent_instance_skill_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_instance_runtime_status DROP CONSTRAINT IF EXISTS aa_agent_instance_runtime_status_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_instance_runtime_status_history DROP CONSTRAINT IF EXISTS aa_agent_instance_runtime_status_history_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_instance_runtime_binding DROP CONSTRAINT IF EXISTS aa_agent_instance_runtime_binding_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_instance DROP CONSTRAINT IF EXISTS aa_agent_instance_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_instance_mcp_group DROP CONSTRAINT IF EXISTS aa_agent_instance_mcp_group_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_instance_external_access DROP CONSTRAINT IF EXISTS aa_agent_instance_external_access_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_instance_desired_state DROP CONSTRAINT IF EXISTS aa_agent_instance_desired_state_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_instance_config_revision DROP CONSTRAINT IF EXISTS aa_agent_instance_config_revision_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_instance_config_change DROP CONSTRAINT IF EXISTS aa_agent_instance_config_change_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_instance_command DROP CONSTRAINT IF EXISTS aa_agent_instance_command_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_instance_agent DROP CONSTRAINT IF EXISTS aa_agent_instance_agent_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_cost_record DROP CONSTRAINT IF EXISTS aa_agent_cost_record_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_config_resource DROP CONSTRAINT IF EXISTS aa_agent_config_resource_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_config_bundle_skill DROP CONSTRAINT IF EXISTS aa_agent_config_bundle_skill_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_config_bundle DROP CONSTRAINT IF EXISTS aa_agent_config_bundle_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_config_bundle_item DROP CONSTRAINT IF EXISTS aa_agent_config_bundle_item_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_backup_schedule DROP CONSTRAINT IF EXISTS aa_agent_backup_schedule_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_backup DROP CONSTRAINT IF EXISTS aa_agent_backup_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_audit_log DROP CONSTRAINT IF EXISTS aa_agent_audit_log_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_agent_audit_event DROP CONSTRAINT IF EXISTS aa_agent_audit_event_pkey;
ALTER TABLE IF EXISTS public.aa_agent_workspace_file_audit ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_user_quota ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_team_work_item ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_team_task ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_team_member ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_team_event ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_team ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_system_image_setting ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_security_scan_report ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_security_scan_job_item ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_security_scan_job ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_security_scan_config ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_runtime_rollout ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_runtime_pod ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_risk_rule ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_risk_hit ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_persistent_volume ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_openclaw_injection_snapshot ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_model_invocation ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_model ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_instance_usage ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_instance_skill ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_instance_runtime_status_history ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_instance_runtime_status ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_instance_runtime_binding ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_instance_mcp_group ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_instance_external_access ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_instance_desired_state ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_instance_config_revision ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_instance_config_change ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_instance_command ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_instance_agent ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_instance ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_cost_record ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_config_resource ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_config_bundle_skill ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_config_bundle_item ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_config_bundle ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_backup_schedule ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_backup ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_audit_log ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_agent_audit_event ALTER COLUMN id DROP DEFAULT;
DROP SEQUENCE IF EXISTS public.aa_agent_workspace_file_audit_id_seq;
DROP TABLE IF EXISTS public.aa_agent_workspace_file_audit;
DROP SEQUENCE IF EXISTS public.aa_agent_user_quota_id_seq;
DROP TABLE IF EXISTS public.aa_agent_user_quota;
DROP SEQUENCE IF EXISTS public.aa_agent_user_id_seq;
DROP TABLE IF EXISTS public.aa_agent_user;
DROP SEQUENCE IF EXISTS public.aa_agent_team_work_item_id_seq;
DROP TABLE IF EXISTS public.aa_agent_team_work_item;
DROP SEQUENCE IF EXISTS public.aa_agent_team_task_id_seq;
DROP TABLE IF EXISTS public.aa_agent_team_task;
DROP SEQUENCE IF EXISTS public.aa_agent_team_member_id_seq;
DROP TABLE IF EXISTS public.aa_agent_team_member;
DROP SEQUENCE IF EXISTS public.aa_agent_team_id_seq;
DROP SEQUENCE IF EXISTS public.aa_agent_team_event_id_seq;
DROP TABLE IF EXISTS public.aa_agent_team_event;
DROP TABLE IF EXISTS public.aa_agent_team;
DROP SEQUENCE IF EXISTS public.aa_agent_system_image_setting_id_seq;
DROP TABLE IF EXISTS public.aa_agent_system_image_setting;
DROP SEQUENCE IF EXISTS public.aa_agent_security_scan_report_id_seq;
DROP TABLE IF EXISTS public.aa_agent_security_scan_report;
DROP SEQUENCE IF EXISTS public.aa_agent_security_scan_job_item_id_seq;
DROP TABLE IF EXISTS public.aa_agent_security_scan_job_item;
DROP SEQUENCE IF EXISTS public.aa_agent_security_scan_job_id_seq;
DROP TABLE IF EXISTS public.aa_agent_security_scan_job;
DROP SEQUENCE IF EXISTS public.aa_agent_security_scan_config_id_seq;
DROP TABLE IF EXISTS public.aa_agent_security_scan_config;
DROP SEQUENCE IF EXISTS public.aa_agent_runtime_rollout_id_seq;
DROP TABLE IF EXISTS public.aa_agent_runtime_rollout;
DROP SEQUENCE IF EXISTS public.aa_agent_runtime_pod_id_seq;
DROP TABLE IF EXISTS public.aa_agent_runtime_pod;
DROP SEQUENCE IF EXISTS public.aa_agent_risk_rule_id_seq;
DROP TABLE IF EXISTS public.aa_agent_risk_rule;
DROP SEQUENCE IF EXISTS public.aa_agent_risk_hit_id_seq;
DROP TABLE IF EXISTS public.aa_agent_risk_hit;
DROP SEQUENCE IF EXISTS public.aa_agent_persistent_volume_id_seq;
DROP TABLE IF EXISTS public.aa_agent_persistent_volume;
DROP SEQUENCE IF EXISTS public.aa_agent_openclaw_injection_snapshot_id_seq;
DROP TABLE IF EXISTS public.aa_agent_openclaw_injection_snapshot;
DROP SEQUENCE IF EXISTS public.aa_agent_model_invocation_id_seq;
DROP TABLE IF EXISTS public.aa_agent_model_invocation;
DROP SEQUENCE IF EXISTS public.aa_agent_model_id_seq;
DROP TABLE IF EXISTS public.aa_agent_model;
DROP SEQUENCE IF EXISTS public.aa_agent_instance_usage_id_seq;
DROP TABLE IF EXISTS public.aa_agent_instance_usage;
DROP SEQUENCE IF EXISTS public.aa_agent_instance_skill_id_seq;
DROP TABLE IF EXISTS public.aa_agent_instance_skill;
DROP SEQUENCE IF EXISTS public.aa_agent_instance_runtime_status_id_seq;
DROP SEQUENCE IF EXISTS public.aa_agent_instance_runtime_status_history_id_seq;
DROP TABLE IF EXISTS public.aa_agent_instance_runtime_status_history;
DROP TABLE IF EXISTS public.aa_agent_instance_runtime_status;
DROP SEQUENCE IF EXISTS public.aa_agent_instance_runtime_binding_id_seq;
DROP TABLE IF EXISTS public.aa_agent_instance_runtime_binding;
DROP SEQUENCE IF EXISTS public.aa_agent_instance_mcp_group_id_seq;
DROP TABLE IF EXISTS public.aa_agent_instance_mcp_group;
DROP SEQUENCE IF EXISTS public.aa_agent_instance_id_seq;
DROP SEQUENCE IF EXISTS public.aa_agent_instance_external_access_id_seq;
DROP TABLE IF EXISTS public.aa_agent_instance_external_access;
DROP SEQUENCE IF EXISTS public.aa_agent_instance_desired_state_id_seq;
DROP TABLE IF EXISTS public.aa_agent_instance_desired_state;
DROP SEQUENCE IF EXISTS public.aa_agent_instance_config_revision_id_seq;
DROP TABLE IF EXISTS public.aa_agent_instance_config_revision;
DROP SEQUENCE IF EXISTS public.aa_agent_instance_config_change_id_seq;
DROP TABLE IF EXISTS public.aa_agent_instance_config_change;
DROP SEQUENCE IF EXISTS public.aa_agent_instance_command_id_seq;
DROP TABLE IF EXISTS public.aa_agent_instance_command;
DROP SEQUENCE IF EXISTS public.aa_agent_instance_agent_id_seq;
DROP TABLE IF EXISTS public.aa_agent_instance_agent;
DROP TABLE IF EXISTS public.aa_agent_instance;
DROP SEQUENCE IF EXISTS public.aa_agent_cost_record_id_seq;
DROP TABLE IF EXISTS public.aa_agent_cost_record;
DROP SEQUENCE IF EXISTS public.aa_agent_config_resource_id_seq;
DROP TABLE IF EXISTS public.aa_agent_config_resource;
DROP SEQUENCE IF EXISTS public.aa_agent_config_bundle_skill_id_seq;
DROP TABLE IF EXISTS public.aa_agent_config_bundle_skill;
DROP SEQUENCE IF EXISTS public.aa_agent_config_bundle_item_id_seq;
DROP TABLE IF EXISTS public.aa_agent_config_bundle_item;
DROP SEQUENCE IF EXISTS public.aa_agent_config_bundle_id_seq;
DROP TABLE IF EXISTS public.aa_agent_config_bundle;
DROP SEQUENCE IF EXISTS public.aa_agent_backup_schedule_id_seq;
DROP TABLE IF EXISTS public.aa_agent_backup_schedule;
DROP SEQUENCE IF EXISTS public.aa_agent_backup_id_seq;
DROP TABLE IF EXISTS public.aa_agent_backup;
DROP SEQUENCE IF EXISTS public.aa_agent_audit_log_id_seq;
DROP TABLE IF EXISTS public.aa_agent_audit_log;
DROP SEQUENCE IF EXISTS public.aa_agent_audit_event_id_seq;
DROP TABLE IF EXISTS public.aa_agent_audit_event;
SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: aa_agent_audit_event; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_audit_event (
    id bigint NOT NULL,
    audit_event_id character varying(32) DEFAULT ''::character varying NOT NULL,
    trace_id character varying(64) DEFAULT ''::character varying NOT NULL,
    session_id character varying(64) DEFAULT ''::character varying NOT NULL,
    request_id character varying(64) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_mode character varying(30) DEFAULT ''::character varying NOT NULL,
    runtime_type character varying(30) DEFAULT ''::character varying NOT NULL,
    gateway_id character varying(64) DEFAULT ''::character varying NOT NULL,
    runtime_pod_id character varying(32) DEFAULT ''::character varying NOT NULL,
    invocation_id character varying(32) DEFAULT ''::character varying NOT NULL,
    event_type character varying(50) DEFAULT ''::character varying NOT NULL,
    traffic_class character varying(30) DEFAULT ''::character varying NOT NULL,
    severity smallint DEFAULT 1 NOT NULL,
    message character varying(500) DEFAULT ''::character varying NOT NULL,
    details jsonb,
    created_at timestamp without time zone
);


--
-- Name: aa_agent_audit_event_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_audit_event_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_audit_event_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_audit_event_id_seq OWNED BY public.aa_agent_audit_event.id;


--
-- Name: aa_agent_audit_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_audit_log (
    id bigint NOT NULL,
    audit_log_id character varying(32) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    action character varying(50) DEFAULT ''::character varying NOT NULL,
    resource_type character varying(50) DEFAULT ''::character varying NOT NULL,
    resource_id character varying(32) DEFAULT ''::character varying NOT NULL,
    details jsonb,
    ip_address character varying(64) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone
);


--
-- Name: aa_agent_audit_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_audit_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_audit_log_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_audit_log_id_seq OWNED BY public.aa_agent_audit_log.id;


--
-- Name: aa_agent_backup; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_backup (
    id bigint NOT NULL,
    backup_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    backup_name character varying(255) DEFAULT ''::character varying NOT NULL,
    backup_size_gb integer DEFAULT 0 NOT NULL,
    backup_path character varying(500) DEFAULT ''::character varying NOT NULL,
    status smallint DEFAULT 0 NOT NULL,
    backup_type smallint DEFAULT 0 NOT NULL,
    completed_at timestamp without time zone,
    expires_at timestamp without time zone,
    created_by character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_backup_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_backup_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_backup_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_backup_id_seq OWNED BY public.aa_agent_backup.id;


--
-- Name: aa_agent_backup_schedule; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_backup_schedule (
    id bigint NOT NULL,
    backup_schedule_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    schedule_name character varying(255) DEFAULT ''::character varying NOT NULL,
    cron_expression character varying(100) DEFAULT ''::character varying NOT NULL,
    retention_days integer DEFAULT 30 NOT NULL,
    is_active smallint DEFAULT 1 NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_backup_schedule_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_backup_schedule_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_backup_schedule_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_backup_schedule_id_seq OWNED BY public.aa_agent_backup_schedule.id;


--
-- Name: aa_agent_config_bundle; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_config_bundle (
    id bigint NOT NULL,
    config_bundle_id character varying(32) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    name character varying(100) DEFAULT ''::character varying NOT NULL,
    description character varying(500) DEFAULT ''::character varying NOT NULL,
    enabled smallint DEFAULT 1 NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    created_by character varying(32) DEFAULT ''::character varying NOT NULL,
    updated_by character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_agent_config_bundle_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_config_bundle_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_config_bundle_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_config_bundle_id_seq OWNED BY public.aa_agent_config_bundle.id;


--
-- Name: aa_agent_config_bundle_item; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_config_bundle_item (
    id bigint NOT NULL,
    bundle_item_id character varying(32) DEFAULT ''::character varying NOT NULL,
    bundle_id character varying(32) DEFAULT ''::character varying NOT NULL,
    resource_id character varying(32) DEFAULT ''::character varying NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    required smallint DEFAULT 0 NOT NULL,
    created_by character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_agent_config_bundle_item_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_config_bundle_item_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_config_bundle_item_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_config_bundle_item_id_seq OWNED BY public.aa_agent_config_bundle_item.id;


--
-- Name: aa_agent_config_bundle_skill; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_config_bundle_skill (
    id bigint NOT NULL,
    bundle_skill_id character varying(32) DEFAULT ''::character varying NOT NULL,
    bundle_id character varying(32) DEFAULT ''::character varying NOT NULL,
    skill_id character varying(32) DEFAULT ''::character varying NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    required smallint DEFAULT 0 NOT NULL,
    created_by character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_agent_config_bundle_skill_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_config_bundle_skill_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_config_bundle_skill_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_config_bundle_skill_id_seq OWNED BY public.aa_agent_config_bundle_skill.id;


--
-- Name: aa_agent_config_resource; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_config_resource (
    id bigint NOT NULL,
    config_resource_id character varying(32) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    resource_type character varying(50) DEFAULT ''::character varying NOT NULL,
    resource_key character varying(128) DEFAULT ''::character varying NOT NULL,
    name character varying(100) DEFAULT ''::character varying NOT NULL,
    description character varying(500) DEFAULT ''::character varying NOT NULL,
    enabled smallint DEFAULT 1 NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    tags_json jsonb DEFAULT '[]'::jsonb NOT NULL,
    content_json jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_by character varying(32) DEFAULT ''::character varying NOT NULL,
    updated_by character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_agent_config_resource_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_config_resource_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_config_resource_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_config_resource_id_seq OWNED BY public.aa_agent_config_resource.id;


--
-- Name: aa_agent_cost_record; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_cost_record (
    id bigint NOT NULL,
    cost_record_id character varying(32) DEFAULT ''::character varying NOT NULL,
    trace_id character varying(64) DEFAULT ''::character varying NOT NULL,
    session_id character varying(64) DEFAULT ''::character varying NOT NULL,
    request_id character varying(64) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_mode character varying(30) DEFAULT ''::character varying NOT NULL,
    runtime_type character varying(30) DEFAULT ''::character varying NOT NULL,
    gateway_id character varying(64) DEFAULT ''::character varying NOT NULL,
    runtime_pod_id character varying(32) DEFAULT ''::character varying NOT NULL,
    invocation_id character varying(32) DEFAULT ''::character varying NOT NULL,
    model_id character varying(32) DEFAULT ''::character varying NOT NULL,
    provider_type character varying(50) DEFAULT ''::character varying NOT NULL,
    model_name character varying(100) DEFAULT ''::character varying NOT NULL,
    currency character varying(20) DEFAULT 'USD'::character varying NOT NULL,
    prompt_tokens integer DEFAULT 0 NOT NULL,
    completion_tokens integer DEFAULT 0 NOT NULL,
    total_tokens integer DEFAULT 0 NOT NULL,
    input_unit_price double precision DEFAULT 0 NOT NULL,
    output_unit_price double precision DEFAULT 0 NOT NULL,
    estimated_cost double precision DEFAULT 0 NOT NULL,
    actual_cost double precision DEFAULT 0 NOT NULL,
    internal_cost double precision DEFAULT 0 NOT NULL,
    recorded_at timestamp without time zone
);


--
-- Name: aa_agent_cost_record_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_cost_record_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_cost_record_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_cost_record_id_seq OWNED BY public.aa_agent_cost_record.id;


--
-- Name: aa_agent_instance; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_instance (
    id bigint NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    name character varying(255) DEFAULT ''::character varying NOT NULL,
    description character varying(1000) DEFAULT ''::character varying NOT NULL,
    agent character varying(30) DEFAULT ''::character varying NOT NULL,
    version character varying(16) DEFAULT ''::character varying NOT NULL,
    status smallint DEFAULT 0 NOT NULL,
    cpu_cores integer DEFAULT 1 NOT NULL,
    memory_gb integer DEFAULT 1 NOT NULL,
    disk_gb integer DEFAULT 10 NOT NULL,
    gpu_enabled smallint DEFAULT 0 NOT NULL,
    gpu_type character varying(100) DEFAULT ''::character varying NOT NULL,
    gpu_count integer DEFAULT 0 NOT NULL,
    os_type character varying(50) DEFAULT ''::character varying NOT NULL,
    os_version character varying(50) DEFAULT ''::character varying NOT NULL,
    image_registry character varying(255) DEFAULT ''::character varying NOT NULL,
    image_tag character varying(128) DEFAULT ''::character varying NOT NULL,
    environment_overrides_json jsonb,
    storage_class character varying(50) DEFAULT 'standard'::character varying NOT NULL,
    mount_path character varying(255) DEFAULT '/data'::character varying NOT NULL,
    container_id character varying(128) DEFAULT ''::character varying NOT NULL,
    container_name character varying(255) DEFAULT ''::character varying NOT NULL,
    host_port integer DEFAULT 0 NOT NULL,
    access_url character varying(500) DEFAULT ''::character varying NOT NULL,
    access_token character varying(255) DEFAULT ''::character varying NOT NULL,
    instance_token character varying(512) DEFAULT ''::character varying NOT NULL,
    llm_key_hash character varying(128) DEFAULT ''::character varying NOT NULL,
    mcp_key_plain character varying(128) DEFAULT ''::character varying NOT NULL,
    started_at timestamp without time zone,
    stopped_at timestamp without time zone,
    created_by character varying(32) DEFAULT ''::character varying NOT NULL,
    updated_by character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_agent_instance_agent; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_instance_agent (
    id bigint NOT NULL,
    instance_agent_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    agent_id character varying(255) DEFAULT ''::character varying NOT NULL,
    agent_version character varying(50) DEFAULT ''::character varying NOT NULL,
    protocol_version character varying(50) DEFAULT ''::character varying NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    capabilities_json jsonb,
    host_info_json jsonb,
    session_token character varying(255) DEFAULT ''::character varying NOT NULL,
    session_expires_at timestamp without time zone,
    last_heartbeat_at timestamp without time zone,
    last_reported_at timestamp without time zone,
    last_seen_ip character varying(45) DEFAULT ''::character varying NOT NULL,
    registered_at timestamp without time zone,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_instance_agent_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_instance_agent_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_instance_agent_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_instance_agent_id_seq OWNED BY public.aa_agent_instance_agent.id;


--
-- Name: aa_agent_instance_command; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_instance_command (
    id bigint NOT NULL,
    instance_command_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    agent_id character varying(255) DEFAULT ''::character varying NOT NULL,
    command_type character varying(50) DEFAULT ''::character varying NOT NULL,
    payload_json jsonb,
    status smallint DEFAULT 0 NOT NULL,
    idempotency_key character varying(255) DEFAULT ''::character varying NOT NULL,
    issued_by character varying(32) DEFAULT ''::character varying NOT NULL,
    timeout_seconds integer DEFAULT 300 NOT NULL,
    result_json jsonb,
    progress_json jsonb,
    error_message character varying(1000) DEFAULT ''::character varying NOT NULL,
    dispatched_at timestamp without time zone,
    started_at timestamp without time zone,
    last_progress_at timestamp without time zone,
    finished_at timestamp without time zone,
    issued_at timestamp without time zone,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_instance_command_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_instance_command_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_instance_command_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_instance_command_id_seq OWNED BY public.aa_agent_instance_command.id;


--
-- Name: aa_agent_instance_config_change; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_instance_config_change (
    id bigint NOT NULL,
    change_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    agent_id character varying(64) DEFAULT ''::character varying NOT NULL,
    source character varying(20) DEFAULT 'runtime'::character varying NOT NULL,
    dispatcher character varying(32) DEFAULT ''::character varying NOT NULL,
    config_hash character varying(64) DEFAULT ''::character varying NOT NULL,
    config_size bigint DEFAULT 0 NOT NULL,
    config_mtime timestamp without time zone,
    content text,
    changed_at timestamp without time zone
);


--
-- Name: aa_agent_instance_config_change_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_instance_config_change_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_instance_config_change_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_instance_config_change_id_seq OWNED BY public.aa_agent_instance_config_change.id;


--
-- Name: aa_agent_instance_config_revision; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_instance_config_revision (
    id bigint NOT NULL,
    config_revision_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    source_snapshot_id character varying(32) DEFAULT '0'::character varying NOT NULL,
    source_bundle_id character varying(32) DEFAULT '0'::character varying NOT NULL,
    revision_no integer DEFAULT 1 NOT NULL,
    content_json jsonb,
    checksum character varying(255) DEFAULT ''::character varying NOT NULL,
    status smallint DEFAULT 1 NOT NULL,
    published_by character varying(32) DEFAULT ''::character varying NOT NULL,
    published_at timestamp without time zone,
    activated_at timestamp without time zone,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_instance_config_revision_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_instance_config_revision_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_instance_config_revision_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_instance_config_revision_id_seq OWNED BY public.aa_agent_instance_config_revision.id;


--
-- Name: aa_agent_instance_desired_state; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_instance_desired_state (
    id bigint NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    desired_power_state character varying(30) DEFAULT 'running'::character varying NOT NULL,
    desired_config_revision_id character varying(32) DEFAULT '0'::character varying NOT NULL,
    desired_runtime_action character varying(50) DEFAULT ''::character varying NOT NULL,
    updated_by character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_instance_desired_state_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_instance_desired_state_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_instance_desired_state_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_instance_desired_state_id_seq OWNED BY public.aa_agent_instance_desired_state.id;


--
-- Name: aa_agent_instance_external_access; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_instance_external_access (
    id bigint NOT NULL,
    external_access_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    enabled smallint DEFAULT 0 NOT NULL,
    auth_mode character varying(32) DEFAULT 'public_link'::character varying NOT NULL,
    public_slug character varying(64) DEFAULT ''::character varying NOT NULL,
    public_token_hash character varying(128) DEFAULT ''::character varying NOT NULL,
    short_code_hash character varying(128) DEFAULT ''::character varying NOT NULL,
    api_key_hash character varying(128) DEFAULT ''::character varying NOT NULL,
    password_value character varying(128) DEFAULT ''::character varying NOT NULL,
    api_key_prefix character varying(32) DEFAULT ''::character varying NOT NULL,
    expires_at timestamp without time zone,
    created_by character varying(32) DEFAULT ''::character varying NOT NULL,
    last_used_at timestamp without time zone,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_instance_external_access_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_instance_external_access_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_instance_external_access_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_instance_external_access_id_seq OWNED BY public.aa_agent_instance_external_access.id;


--
-- Name: aa_agent_instance_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_instance_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_instance_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_instance_id_seq OWNED BY public.aa_agent_instance.id;


--
-- Name: aa_agent_instance_mcp_group; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_instance_mcp_group (
    id bigint NOT NULL,
    instance_mcp_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    group_id character varying(32) DEFAULT ''::character varying NOT NULL,
    group_name character varying(200) DEFAULT ''::character varying NOT NULL,
    created_by character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_instance_mcp_group_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_instance_mcp_group_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_instance_mcp_group_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_instance_mcp_group_id_seq OWNED BY public.aa_agent_instance_mcp_group.id;


--
-- Name: aa_agent_instance_runtime_binding; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_instance_runtime_binding (
    id bigint NOT NULL,
    runtime_binding_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    runtime_pod_id character varying(32) DEFAULT '0'::character varying NOT NULL,
    runtime_type character varying(32) DEFAULT ''::character varying NOT NULL,
    gateway_id character varying(128) DEFAULT ''::character varying NOT NULL,
    gateway_port integer DEFAULT 0 NOT NULL,
    gateway_pid character varying(64) DEFAULT ''::character varying NOT NULL,
    workspace_path character varying(1024) DEFAULT ''::character varying NOT NULL,
    state smallint DEFAULT 0 NOT NULL,
    generation integer DEFAULT 1 NOT NULL,
    last_health_at timestamp without time zone,
    error_message character varying(1000) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_instance_runtime_binding_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_instance_runtime_binding_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_instance_runtime_binding_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_instance_runtime_binding_id_seq OWNED BY public.aa_agent_instance_runtime_binding.id;


--
-- Name: aa_agent_instance_runtime_status; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_instance_runtime_status (
    id bigint NOT NULL,
    instance_runtime_status_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    infra_status smallint DEFAULT 0 NOT NULL,
    agent_status smallint DEFAULT 0 NOT NULL,
    agent_proc_status smallint DEFAULT 0 NOT NULL,
    agent_proc_pid character varying(64) DEFAULT ''::character varying NOT NULL,
    agent_proc_version character varying(100) DEFAULT ''::character varying NOT NULL,
    current_config_revision_id character varying(32) DEFAULT '0'::character varying NOT NULL,
    desired_config_revision_id character varying(32) DEFAULT '0'::character varying NOT NULL,
    summary_json jsonb,
    system_info_json jsonb,
    health_json jsonb,
    last_reported_at timestamp without time zone,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_instance_runtime_status_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_instance_runtime_status_history (
    id bigint NOT NULL,
    history_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    infra_status smallint DEFAULT 0 NOT NULL,
    agent_status smallint DEFAULT 0 NOT NULL,
    agent_proc_status smallint DEFAULT 0 NOT NULL,
    agent_proc_pid character varying(64) DEFAULT ''::character varying NOT NULL,
    agent_proc_version character varying(100) DEFAULT ''::character varying NOT NULL,
    config_hash character varying(64) DEFAULT ''::character varying NOT NULL,
    running_command_sig character varying(64) DEFAULT ''::character varying NOT NULL,
    summary_json jsonb,
    reported_at timestamp without time zone,
    created_at timestamp without time zone
);


--
-- Name: aa_agent_instance_runtime_status_history_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_instance_runtime_status_history_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_instance_runtime_status_history_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_instance_runtime_status_history_id_seq OWNED BY public.aa_agent_instance_runtime_status_history.id;


--
-- Name: aa_agent_instance_runtime_status_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_instance_runtime_status_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_instance_runtime_status_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_instance_runtime_status_id_seq OWNED BY public.aa_agent_instance_runtime_status.id;


--
-- Name: aa_agent_instance_skill; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_instance_skill (
    id bigint NOT NULL,
    instance_skill_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    skill_id character varying(32) DEFAULT ''::character varying NOT NULL,
    version_id character varying(64) DEFAULT ''::character varying NOT NULL,
    version character varying(128) DEFAULT ''::character varying NOT NULL,
    enabled smallint DEFAULT 1 NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_instance_skill_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_instance_skill_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_instance_skill_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_instance_skill_id_seq OWNED BY public.aa_agent_instance_skill.id;


--
-- Name: aa_agent_instance_usage; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_instance_usage (
    id bigint NOT NULL,
    instance_usage_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    cpu_usage_percent numeric(5,2) DEFAULT 0 NOT NULL,
    memory_usage_gb numeric(10,2) DEFAULT 0 NOT NULL,
    disk_usage_gb numeric(10,2) DEFAULT 0 NOT NULL,
    gpu_usage_percent numeric(5,2) DEFAULT 0 NOT NULL,
    uptime_seconds integer DEFAULT 0 NOT NULL,
    model character varying(128) DEFAULT ''::character varying NOT NULL,
    prompt_tokens bigint DEFAULT 0 NOT NULL,
    completion_tokens bigint DEFAULT 0 NOT NULL,
    cost numeric(12,6) DEFAULT 0 NOT NULL,
    logs_json jsonb DEFAULT '{}'::jsonb,
    recorded_at timestamp without time zone
);


--
-- Name: aa_agent_instance_usage_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_instance_usage_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_instance_usage_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_instance_usage_id_seq OWNED BY public.aa_agent_instance_usage.id;


--
-- Name: aa_agent_model; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_model (
    id bigint NOT NULL,
    agent_model_id character varying(32) DEFAULT ''::character varying NOT NULL,
    display_name character varying(100) DEFAULT ''::character varying NOT NULL,
    description character varying(500) DEFAULT ''::character varying NOT NULL,
    provider_type character varying(50) DEFAULT ''::character varying NOT NULL,
    protocol_type character varying(50) DEFAULT ''::character varying NOT NULL,
    base_url character varying(255) DEFAULT ''::character varying NOT NULL,
    provider_model_name character varying(100) DEFAULT ''::character varying NOT NULL,
    api_key character varying(255) DEFAULT ''::character varying NOT NULL,
    api_key_secret_ref character varying(255) DEFAULT ''::character varying NOT NULL,
    is_secure smallint DEFAULT 0 NOT NULL,
    is_active smallint DEFAULT 1 NOT NULL,
    input_price double precision DEFAULT 0 NOT NULL,
    output_price double precision DEFAULT 0 NOT NULL,
    currency character varying(20) DEFAULT 'USD'::character varying NOT NULL,
    created_by character varying(32) DEFAULT ''::character varying NOT NULL,
    updated_by character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_agent_model_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_model_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_model_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_model_id_seq OWNED BY public.aa_agent_model.id;


--
-- Name: aa_agent_model_invocation; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_model_invocation (
    id bigint NOT NULL,
    invocation_id character varying(32) DEFAULT ''::character varying NOT NULL,
    trace_id character varying(64) DEFAULT ''::character varying NOT NULL,
    session_id character varying(64) DEFAULT ''::character varying NOT NULL,
    request_id character varying(64) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_mode character varying(30) DEFAULT ''::character varying NOT NULL,
    runtime_type character varying(30) DEFAULT ''::character varying NOT NULL,
    gateway_id character varying(64) DEFAULT ''::character varying NOT NULL,
    runtime_pod_id character varying(32) DEFAULT ''::character varying NOT NULL,
    model_id character varying(32) DEFAULT ''::character varying NOT NULL,
    provider_type character varying(50) DEFAULT ''::character varying NOT NULL,
    requested_model character varying(100) DEFAULT ''::character varying NOT NULL,
    actual_provider_model character varying(100) DEFAULT ''::character varying NOT NULL,
    traffic_class character varying(30) DEFAULT ''::character varying NOT NULL,
    request_payload jsonb,
    response_payload jsonb,
    prompt_tokens integer DEFAULT 0 NOT NULL,
    completion_tokens integer DEFAULT 0 NOT NULL,
    total_tokens integer DEFAULT 0 NOT NULL,
    cached_tokens integer DEFAULT 0 NOT NULL,
    reasoning_tokens integer DEFAULT 0 NOT NULL,
    latency_ms integer DEFAULT 0 NOT NULL,
    is_streaming smallint DEFAULT 0 NOT NULL,
    status character varying(20) DEFAULT ''::character varying NOT NULL,
    error_message character varying(500) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    completed_at timestamp without time zone
);


--
-- Name: aa_agent_model_invocation_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_model_invocation_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_model_invocation_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_model_invocation_id_seq OWNED BY public.aa_agent_model_invocation.id;


--
-- Name: aa_agent_openclaw_injection_snapshot; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_openclaw_injection_snapshot (
    id bigint NOT NULL,
    injection_snapshot_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    config_revision_id character varying(32) DEFAULT '0'::character varying NOT NULL,
    bundle_id character varying(32) DEFAULT '0'::character varying NOT NULL,
    compiled_json jsonb,
    checksum character varying(255) DEFAULT ''::character varying NOT NULL,
    injected_at timestamp without time zone,
    created_at timestamp without time zone
);


--
-- Name: aa_agent_openclaw_injection_snapshot_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_openclaw_injection_snapshot_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_openclaw_injection_snapshot_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_openclaw_injection_snapshot_id_seq OWNED BY public.aa_agent_openclaw_injection_snapshot.id;


--
-- Name: aa_agent_persistent_volume; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_persistent_volume (
    id bigint NOT NULL,
    persistent_volume_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    volume_name character varying(255) DEFAULT ''::character varying NOT NULL,
    storage_size_gb integer DEFAULT 10 NOT NULL,
    storage_class character varying(50) DEFAULT 'standard'::character varying NOT NULL,
    mount_path character varying(255) DEFAULT '/data'::character varying NOT NULL,
    status smallint DEFAULT 0 NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_persistent_volume_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_persistent_volume_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_persistent_volume_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_persistent_volume_id_seq OWNED BY public.aa_agent_persistent_volume.id;


--
-- Name: aa_agent_risk_hit; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_risk_hit (
    id bigint NOT NULL,
    risk_hit_id character varying(32) DEFAULT ''::character varying NOT NULL,
    trace_id character varying(64) DEFAULT ''::character varying NOT NULL,
    session_id character varying(64) DEFAULT ''::character varying NOT NULL,
    request_id character varying(64) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_mode character varying(30) DEFAULT ''::character varying NOT NULL,
    runtime_type character varying(30) DEFAULT ''::character varying NOT NULL,
    gateway_id character varying(64) DEFAULT ''::character varying NOT NULL,
    runtime_pod_id character varying(32) DEFAULT ''::character varying NOT NULL,
    invocation_id character varying(32) DEFAULT ''::character varying NOT NULL,
    rule_id character varying(64) DEFAULT ''::character varying NOT NULL,
    rule_name character varying(100) DEFAULT ''::character varying NOT NULL,
    severity smallint DEFAULT 1 NOT NULL,
    action character varying(30) DEFAULT ''::character varying NOT NULL,
    match_summary character varying(500) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone
);


--
-- Name: aa_agent_risk_hit_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_risk_hit_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_risk_hit_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_risk_hit_id_seq OWNED BY public.aa_agent_risk_hit.id;


--
-- Name: aa_agent_risk_rule; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_risk_rule (
    id bigint NOT NULL,
    risk_rule_id character varying(32) DEFAULT ''::character varying NOT NULL,
    rule_id character varying(64) DEFAULT ''::character varying NOT NULL,
    display_name character varying(100) DEFAULT ''::character varying NOT NULL,
    description character varying(500) DEFAULT ''::character varying NOT NULL,
    pattern character varying(500) DEFAULT ''::character varying NOT NULL,
    severity smallint DEFAULT 1 NOT NULL,
    action character varying(30) DEFAULT ''::character varying NOT NULL,
    is_enabled smallint DEFAULT 1 NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    created_by character varying(32) DEFAULT ''::character varying NOT NULL,
    updated_by character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_agent_risk_rule_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_risk_rule_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_risk_rule_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_risk_rule_id_seq OWNED BY public.aa_agent_risk_rule.id;


--
-- Name: aa_agent_runtime_pod; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_runtime_pod (
    id bigint NOT NULL,
    runtime_pod_id character varying(32) DEFAULT ''::character varying NOT NULL,
    rollout_id character varying(32) DEFAULT ''::character varying NOT NULL,
    runtime_type character varying(32) DEFAULT ''::character varying NOT NULL,
    namespace character varying(128) DEFAULT 'default'::character varying NOT NULL,
    pod_name character varying(255) DEFAULT ''::character varying NOT NULL,
    pod_uid character varying(128) DEFAULT ''::character varying NOT NULL,
    pod_ip character varying(64) DEFAULT ''::character varying NOT NULL,
    node_name character varying(255) DEFAULT ''::character varying NOT NULL,
    deployment_name character varying(255) DEFAULT ''::character varying NOT NULL,
    image_ref character varying(512) DEFAULT ''::character varying NOT NULL,
    agent_endpoint character varying(255) DEFAULT ''::character varying NOT NULL,
    state smallint DEFAULT 0 NOT NULL,
    capacity integer DEFAULT 100 NOT NULL,
    used_slots integer DEFAULT 0 NOT NULL,
    draining smallint DEFAULT 0 NOT NULL,
    cpu_millis_used bigint DEFAULT 0 NOT NULL,
    memory_bytes_used bigint DEFAULT 0 NOT NULL,
    disk_bytes_used bigint DEFAULT 0 NOT NULL,
    network_rx_bytes bigint DEFAULT 0 NOT NULL,
    network_tx_bytes bigint DEFAULT 0 NOT NULL,
    metrics_json jsonb,
    last_seen_at timestamp without time zone,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_runtime_pod_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_runtime_pod_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_runtime_pod_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_runtime_pod_id_seq OWNED BY public.aa_agent_runtime_pod.id;


--
-- Name: aa_agent_runtime_rollout; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_runtime_rollout (
    id bigint NOT NULL,
    runtime_rollout_id character varying(32) DEFAULT ''::character varying NOT NULL,
    runtime_type character varying(32) DEFAULT ''::character varying NOT NULL,
    target_image_ref character varying(512) DEFAULT ''::character varying NOT NULL,
    status smallint DEFAULT 0 NOT NULL,
    batch_size integer DEFAULT 1 NOT NULL,
    max_unavailable integer DEFAULT 1 NOT NULL,
    started_by character varying(32) DEFAULT ''::character varying NOT NULL,
    error_message character varying(1000) DEFAULT ''::character varying NOT NULL,
    started_at timestamp without time zone,
    finished_at timestamp without time zone,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_runtime_rollout_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_runtime_rollout_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_runtime_rollout_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_runtime_rollout_id_seq OWNED BY public.aa_agent_runtime_rollout.id;


--
-- Name: aa_agent_security_scan_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_security_scan_config (
    id bigint NOT NULL,
    scan_config_id character varying(32) DEFAULT ''::character varying NOT NULL,
    default_mode character varying(30) DEFAULT ''::character varying NOT NULL,
    quick_analyzers_json jsonb DEFAULT '[]'::jsonb NOT NULL,
    deep_analyzers_json jsonb DEFAULT '[]'::jsonb NOT NULL,
    quick_timeout_seconds integer DEFAULT 60 NOT NULL,
    deep_timeout_seconds integer DEFAULT 300 NOT NULL,
    allow_fallback smallint DEFAULT 0 NOT NULL,
    updated_by character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_agent_security_scan_config_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_security_scan_config_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_security_scan_config_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_security_scan_config_id_seq OWNED BY public.aa_agent_security_scan_config.id;


--
-- Name: aa_agent_security_scan_job; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_security_scan_job (
    id bigint NOT NULL,
    scan_job_id character varying(32) DEFAULT ''::character varying NOT NULL,
    asset_type character varying(50) DEFAULT ''::character varying NOT NULL,
    scan_mode character varying(30) DEFAULT ''::character varying NOT NULL,
    status smallint DEFAULT 0 NOT NULL,
    requested_by character varying(32) DEFAULT ''::character varying NOT NULL,
    scope_json jsonb,
    total_items integer DEFAULT 0 NOT NULL,
    completed_items integer DEFAULT 0 NOT NULL,
    failed_items integer DEFAULT 0 NOT NULL,
    current_item_name character varying(255) DEFAULT ''::character varying NOT NULL,
    started_at timestamp without time zone,
    finished_at timestamp without time zone,
    created_by character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_agent_security_scan_job_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_security_scan_job_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_security_scan_job_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_security_scan_job_id_seq OWNED BY public.aa_agent_security_scan_job.id;


--
-- Name: aa_agent_security_scan_job_item; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_security_scan_job_item (
    id bigint NOT NULL,
    job_item_id character varying(32) DEFAULT ''::character varying NOT NULL,
    job_id character varying(32) DEFAULT ''::character varying NOT NULL,
    asset_type character varying(50) DEFAULT ''::character varying NOT NULL,
    asset_id character varying(32) DEFAULT ''::character varying NOT NULL,
    asset_name character varying(255) DEFAULT ''::character varying NOT NULL,
    status smallint DEFAULT 0 NOT NULL,
    progress_pct integer DEFAULT 0 NOT NULL,
    risk_level smallint DEFAULT 0 NOT NULL,
    summary character varying(500) DEFAULT ''::character varying NOT NULL,
    scan_result_id character varying(32) DEFAULT '0'::character varying NOT NULL,
    cached_result smallint DEFAULT 0 NOT NULL,
    error_message character varying(500) DEFAULT ''::character varying NOT NULL,
    started_at timestamp without time zone,
    finished_at timestamp without time zone,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_security_scan_job_item_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_security_scan_job_item_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_security_scan_job_item_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_security_scan_job_item_id_seq OWNED BY public.aa_agent_security_scan_job_item.id;


--
-- Name: aa_agent_security_scan_report; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_security_scan_report (
    id bigint NOT NULL,
    scan_report_id character varying(32) DEFAULT ''::character varying NOT NULL,
    job_id character varying(32) DEFAULT ''::character varying NOT NULL,
    summary_json jsonb,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_security_scan_report_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_security_scan_report_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_security_scan_report_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_security_scan_report_id_seq OWNED BY public.aa_agent_security_scan_report.id;


--
-- Name: aa_agent_system_image_setting; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_system_image_setting (
    id bigint NOT NULL,
    system_image_id character varying(32) DEFAULT ''::character varying NOT NULL,
    agent character varying(30) DEFAULT ''::character varying NOT NULL,
    version character varying(16) DEFAULT ''::character varying NOT NULL,
    display_name character varying(255) DEFAULT ''::character varying NOT NULL,
    image character varying(500) DEFAULT ''::character varying NOT NULL,
    is_enabled smallint DEFAULT 1 NOT NULL,
    updated_by character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_system_image_setting_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_system_image_setting_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_system_image_setting_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_system_image_setting_id_seq OWNED BY public.aa_agent_system_image_setting.id;


--
-- Name: aa_agent_team; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_team (
    id bigint NOT NULL,
    team_id character varying(32) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    name character varying(255) DEFAULT ''::character varying NOT NULL,
    description character varying(1000) DEFAULT ''::character varying NOT NULL,
    status smallint DEFAULT 0 NOT NULL,
    communication_mode character varying(50) DEFAULT 'leader_only'::character varying NOT NULL,
    redis_url character varying(500) DEFAULT ''::character varying NOT NULL,
    team_token character varying(255) DEFAULT ''::character varying NOT NULL,
    redis_events_last_id character varying(128) DEFAULT '0-0'::character varying NOT NULL,
    shared_volume_name character varying(255) DEFAULT ''::character varying NOT NULL,
    shared_mount_path character varying(255) DEFAULT '/team'::character varying NOT NULL,
    storage_class character varying(100) DEFAULT 'standard'::character varying NOT NULL,
    created_by character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_agent_team_event; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_team_event (
    id bigint NOT NULL,
    team_event_id character varying(32) DEFAULT ''::character varying NOT NULL,
    event_id character varying(128) DEFAULT ''::character varying NOT NULL,
    completion_id character varying(255) DEFAULT ''::character varying NOT NULL,
    sequence_no bigint DEFAULT 0 NOT NULL,
    team_id character varying(32) DEFAULT ''::character varying NOT NULL,
    member_id character varying(32) DEFAULT '0'::character varying NOT NULL,
    task_id character varying(32) DEFAULT '0'::character varying NOT NULL,
    message_id character varying(255) DEFAULT ''::character varying NOT NULL,
    event_type character varying(100) DEFAULT ''::character varying NOT NULL,
    payload_json jsonb,
    redis_stream_id character varying(128) DEFAULT ''::character varying NOT NULL,
    occurred_at timestamp without time zone,
    created_at timestamp without time zone
);


--
-- Name: aa_agent_team_event_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_team_event_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_team_event_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_team_event_id_seq OWNED BY public.aa_agent_team_event.id;


--
-- Name: aa_agent_team_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_team_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_team_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_team_id_seq OWNED BY public.aa_agent_team.id;


--
-- Name: aa_agent_team_member; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_team_member (
    id bigint NOT NULL,
    team_member_id character varying(32) DEFAULT ''::character varying NOT NULL,
    team_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_id character varying(32) DEFAULT '0'::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    member_key character varying(100) DEFAULT ''::character varying NOT NULL,
    display_name character varying(255) DEFAULT ''::character varying NOT NULL,
    role character varying(100) DEFAULT 'member'::character varying NOT NULL,
    description character varying(500) DEFAULT ''::character varying NOT NULL,
    status smallint DEFAULT 0 NOT NULL,
    current_task_id character varying(32) DEFAULT '0'::character varying NOT NULL,
    progress integer DEFAULT 0 NOT NULL,
    last_seen_at timestamp without time zone,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_team_member_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_team_member_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_team_member_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_team_member_id_seq OWNED BY public.aa_agent_team_member.id;


--
-- Name: aa_agent_team_task; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_team_task (
    id bigint NOT NULL,
    team_task_id character varying(32) DEFAULT ''::character varying NOT NULL,
    team_id character varying(32) DEFAULT ''::character varying NOT NULL,
    target_member_id character varying(32) DEFAULT ''::character varying NOT NULL,
    created_by character varying(32) DEFAULT ''::character varying NOT NULL,
    message_id character varying(255) DEFAULT ''::character varying NOT NULL,
    status smallint DEFAULT 0 NOT NULL,
    payload_json jsonb,
    result_json jsonb,
    error_message character varying(1000) DEFAULT ''::character varying NOT NULL,
    redis_stream_id character varying(128) DEFAULT ''::character varying NOT NULL,
    dispatched_at timestamp without time zone,
    started_at timestamp without time zone,
    finished_at timestamp without time zone,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_team_task_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_team_task_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_team_task_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_team_task_id_seq OWNED BY public.aa_agent_team_task.id;


--
-- Name: aa_agent_team_work_item; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_team_work_item (
    id bigint NOT NULL,
    work_item_id character varying(32) DEFAULT ''::character varying NOT NULL,
    team_id character varying(32) DEFAULT ''::character varying NOT NULL,
    root_task_id character varying(32) DEFAULT ''::character varying NOT NULL,
    work_id character varying(255) DEFAULT ''::character varying NOT NULL,
    owner_member_id character varying(32) DEFAULT '0'::character varying NOT NULL,
    title character varying(500) DEFAULT ''::character varying NOT NULL,
    status smallint DEFAULT 0 NOT NULL,
    depends_on_json jsonb,
    result_json jsonb,
    artifact_refs_json jsonb,
    started_at timestamp without time zone,
    finished_at timestamp without time zone,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_team_work_item_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_team_work_item_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_team_work_item_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_team_work_item_id_seq OWNED BY public.aa_agent_team_work_item.id;


--
-- Name: aa_agent_user; Type: TABLE; Schema: public; Owner: -
--

-- Agent 平台用户（v2 占位菜单）：与主库 aa_user 解耦软引用（username 唯一），
-- 自有角色 admin/operator/member、口令盐+哈希（md5(md5(pw)+md5(salt))，明文不落库）、
-- 备注与实例授权（instance_ids jsonb，元素为 aa_agent_instance.instance_id）。

CREATE TABLE public.aa_agent_user (
    id bigint NOT NULL,
    agent_user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    username character varying(64) DEFAULT ''::character varying NOT NULL,
    display_name character varying(128) DEFAULT ''::character varying NOT NULL,
    role character varying(16) DEFAULT 'member'::character varying NOT NULL,
    instance_ids jsonb DEFAULT '[]'::jsonb,
    is_enabled smallint DEFAULT 1 NOT NULL,
    password_salt character varying(64) DEFAULT ''::character varying NOT NULL,
    password_hash character varying(255) DEFAULT ''::character varying NOT NULL,
    remark character varying(500) DEFAULT ''::character varying NOT NULL,
    created_by character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_user_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_user_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_user_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_user_id_seq OWNED BY public.aa_agent_user.id;


--
-- Name: aa_agent_user_quota; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_user_quota (
    id bigint NOT NULL,
    user_quota_id character varying(32) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    max_instances integer DEFAULT 10 NOT NULL,
    max_cpu_cores double precision DEFAULT 40 NOT NULL,
    max_memory_gb integer DEFAULT 100 NOT NULL,
    max_storage_gb integer DEFAULT 500 NOT NULL,
    max_gpu_count integer DEFAULT 2 NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_agent_user_quota_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_user_quota_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_user_quota_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_user_quota_id_seq OWNED BY public.aa_agent_user_quota.id;


--
-- Name: aa_agent_workspace_file_audit; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_agent_workspace_file_audit (
    id bigint NOT NULL,
    workspace_audit_id character varying(32) DEFAULT ''::character varying NOT NULL,
    instance_id character varying(32) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    action character varying(32) DEFAULT ''::character varying NOT NULL,
    relative_path character varying(1024) DEFAULT ''::character varying NOT NULL,
    bytes bigint DEFAULT 0 NOT NULL,
    created_at timestamp without time zone
);


--
-- Name: aa_agent_workspace_file_audit_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_agent_workspace_file_audit_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_agent_workspace_file_audit_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_agent_workspace_file_audit_id_seq OWNED BY public.aa_agent_workspace_file_audit.id;


--
-- Name: aa_agent_audit_event id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_audit_event ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_audit_event_id_seq'::regclass);


--
-- Name: aa_agent_audit_log id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_audit_log ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_audit_log_id_seq'::regclass);


--
-- Name: aa_agent_backup id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_backup ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_backup_id_seq'::regclass);


--
-- Name: aa_agent_backup_schedule id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_backup_schedule ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_backup_schedule_id_seq'::regclass);


--
-- Name: aa_agent_config_bundle id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_config_bundle ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_config_bundle_id_seq'::regclass);


--
-- Name: aa_agent_config_bundle_item id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_config_bundle_item ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_config_bundle_item_id_seq'::regclass);


--
-- Name: aa_agent_config_bundle_skill id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_config_bundle_skill ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_config_bundle_skill_id_seq'::regclass);


--
-- Name: aa_agent_config_resource id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_config_resource ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_config_resource_id_seq'::regclass);


--
-- Name: aa_agent_cost_record id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_cost_record ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_cost_record_id_seq'::regclass);


--
-- Name: aa_agent_instance id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_instance_id_seq'::regclass);


--
-- Name: aa_agent_instance_agent id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_agent ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_instance_agent_id_seq'::regclass);


--
-- Name: aa_agent_instance_command id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_command ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_instance_command_id_seq'::regclass);


--
-- Name: aa_agent_instance_config_change id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_config_change ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_instance_config_change_id_seq'::regclass);


--
-- Name: aa_agent_instance_config_revision id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_config_revision ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_instance_config_revision_id_seq'::regclass);


--
-- Name: aa_agent_instance_desired_state id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_desired_state ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_instance_desired_state_id_seq'::regclass);


--
-- Name: aa_agent_instance_external_access id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_external_access ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_instance_external_access_id_seq'::regclass);


--
-- Name: aa_agent_instance_mcp_group id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_mcp_group ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_instance_mcp_group_id_seq'::regclass);


--
-- Name: aa_agent_instance_runtime_binding id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_runtime_binding ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_instance_runtime_binding_id_seq'::regclass);


--
-- Name: aa_agent_instance_runtime_status id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_runtime_status ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_instance_runtime_status_id_seq'::regclass);


--
-- Name: aa_agent_instance_runtime_status_history id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_runtime_status_history ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_instance_runtime_status_history_id_seq'::regclass);


--
-- Name: aa_agent_instance_skill id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_skill ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_instance_skill_id_seq'::regclass);


--
-- Name: aa_agent_instance_usage id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_usage ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_instance_usage_id_seq'::regclass);


--
-- Name: aa_agent_model id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_model ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_model_id_seq'::regclass);


--
-- Name: aa_agent_model_invocation id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_model_invocation ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_model_invocation_id_seq'::regclass);


--
-- Name: aa_agent_openclaw_injection_snapshot id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_openclaw_injection_snapshot ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_openclaw_injection_snapshot_id_seq'::regclass);


--
-- Name: aa_agent_persistent_volume id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_persistent_volume ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_persistent_volume_id_seq'::regclass);


--
-- Name: aa_agent_risk_hit id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_risk_hit ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_risk_hit_id_seq'::regclass);


--
-- Name: aa_agent_risk_rule id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_risk_rule ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_risk_rule_id_seq'::regclass);


--
-- Name: aa_agent_runtime_pod id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_runtime_pod ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_runtime_pod_id_seq'::regclass);


--
-- Name: aa_agent_runtime_rollout id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_runtime_rollout ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_runtime_rollout_id_seq'::regclass);


--
-- Name: aa_agent_security_scan_config id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_security_scan_config ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_security_scan_config_id_seq'::regclass);


--
-- Name: aa_agent_security_scan_job id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_security_scan_job ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_security_scan_job_id_seq'::regclass);


--
-- Name: aa_agent_security_scan_job_item id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_security_scan_job_item ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_security_scan_job_item_id_seq'::regclass);


--
-- Name: aa_agent_security_scan_report id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_security_scan_report ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_security_scan_report_id_seq'::regclass);


--
-- Name: aa_agent_system_image_setting id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_system_image_setting ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_system_image_setting_id_seq'::regclass);


--
-- Name: aa_agent_team id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_team ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_team_id_seq'::regclass);


--
-- Name: aa_agent_team_event id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_team_event ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_team_event_id_seq'::regclass);


--
-- Name: aa_agent_team_member id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_team_member ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_team_member_id_seq'::regclass);


--
-- Name: aa_agent_team_task id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_team_task ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_team_task_id_seq'::regclass);


--
-- Name: aa_agent_team_work_item id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_team_work_item ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_team_work_item_id_seq'::regclass);


--
-- Name: aa_agent_user_quota id; Type: DEFAULT; Schema: public; Owner: -
--

--
-- Name: aa_agent_user id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_user ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_user_id_seq'::regclass);


--
-- Name: aa_agent_user_quota id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_user_quota ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_user_quota_id_seq'::regclass);


--
-- Name: aa_agent_workspace_file_audit id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_workspace_file_audit ALTER COLUMN id SET DEFAULT nextval('public.aa_agent_workspace_file_audit_id_seq'::regclass);


--
-- Data for Name: aa_agent_audit_event; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_audit_event (id, audit_event_id, trace_id, session_id, request_id, user_id, instance_id, instance_mode, runtime_type, gateway_id, runtime_pod_id, invocation_id, event_type, traffic_class, severity, message, details, created_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_audit_log; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_audit_log (id, audit_log_id, user_id, action, resource_type, resource_id, details, ip_address, created_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_backup; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_backup (id, backup_id, instance_id, backup_name, backup_size_gb, backup_path, status, backup_type, completed_at, expires_at, created_by, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_backup_schedule; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_backup_schedule (id, backup_schedule_id, instance_id, schedule_name, cron_expression, retention_days, is_active, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_config_bundle; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_config_bundle (id, config_bundle_id, user_id, name, description, enabled, version, created_by, updated_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_config_bundle_item; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_config_bundle_item (id, bundle_item_id, bundle_id, resource_id, sort_order, required, created_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_config_bundle_skill; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_config_bundle_skill (id, bundle_skill_id, bundle_id, skill_id, sort_order, required, created_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_config_resource; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_config_resource (id, config_resource_id, user_id, resource_type, resource_key, name, description, enabled, version, tags_json, content_json, created_by, updated_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_cost_record; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_cost_record (id, cost_record_id, trace_id, session_id, request_id, user_id, instance_id, instance_mode, runtime_type, gateway_id, runtime_pod_id, invocation_id, model_id, provider_type, model_name, currency, prompt_tokens, completion_tokens, total_tokens, input_unit_price, output_unit_price, estimated_cost, actual_cost, internal_cost, recorded_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_instance; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_instance (id, instance_id, user_id, name, description, agent, version, status, cpu_cores, memory_gb, disk_gb, gpu_enabled, gpu_type, gpu_count, os_type, os_version, image_registry, image_tag, environment_overrides_json, storage_class, mount_path, container_id, container_name, host_port, access_url, access_token, instance_token, llm_key_hash, mcp_key_plain, started_at, stopped_at, created_by, updated_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_instance_agent; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_instance_agent (id, instance_agent_id, instance_id, agent_id, agent_version, protocol_version, status, capabilities_json, host_info_json, session_token, session_expires_at, last_heartbeat_at, last_reported_at, last_seen_ip, registered_at, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_instance_command; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_instance_command (id, instance_command_id, instance_id, agent_id, command_type, payload_json, status, idempotency_key, issued_by, timeout_seconds, result_json, progress_json, error_message, dispatched_at, started_at, last_progress_at, finished_at, issued_at, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_instance_config_change; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_instance_config_change (id, change_id, instance_id, agent_id, source, dispatcher, config_hash, config_size, config_mtime, content, changed_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_instance_config_revision; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_instance_config_revision (id, config_revision_id, instance_id, source_snapshot_id, source_bundle_id, revision_no, content_json, checksum, status, published_by, published_at, activated_at, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_instance_desired_state; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_instance_desired_state (id, instance_id, desired_power_state, desired_config_revision_id, desired_runtime_action, updated_by, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_instance_external_access; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_instance_external_access (id, external_access_id, instance_id, enabled, auth_mode, public_slug, public_token_hash, short_code_hash, api_key_hash, password_value, api_key_prefix, expires_at, created_by, last_used_at, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_instance_mcp_group; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_instance_mcp_group (id, instance_mcp_id, instance_id, group_id, group_name, created_by, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_instance_runtime_binding; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_instance_runtime_binding (id, runtime_binding_id, instance_id, runtime_pod_id, runtime_type, gateway_id, gateway_port, gateway_pid, workspace_path, state, generation, last_health_at, error_message, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_instance_runtime_status; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_instance_runtime_status (id, instance_runtime_status_id, instance_id, infra_status, agent_status, agent_proc_status, agent_proc_pid, agent_proc_version, current_config_revision_id, desired_config_revision_id, summary_json, system_info_json, health_json, last_reported_at, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_instance_runtime_status_history; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_instance_runtime_status_history (id, history_id, instance_id, infra_status, agent_status, agent_proc_status, agent_proc_pid, agent_proc_version, config_hash, running_command_sig, summary_json, reported_at, created_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_instance_skill; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_instance_skill (id, instance_skill_id, instance_id, skill_id, version_id, version, enabled, sort_order, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_instance_usage; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_instance_usage (id, instance_usage_id, instance_id, cpu_usage_percent, memory_usage_gb, disk_usage_gb, gpu_usage_percent, uptime_seconds, model, prompt_tokens, completion_tokens, cost, logs_json, recorded_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_model; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_model (id, agent_model_id, display_name, description, provider_type, protocol_type, base_url, provider_model_name, api_key, api_key_secret_ref, is_secure, is_active, input_price, output_price, currency, created_by, updated_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_model_invocation; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_model_invocation (id, invocation_id, trace_id, session_id, request_id, user_id, instance_id, instance_mode, runtime_type, gateway_id, runtime_pod_id, model_id, provider_type, requested_model, actual_provider_model, traffic_class, request_payload, response_payload, prompt_tokens, completion_tokens, total_tokens, cached_tokens, reasoning_tokens, latency_ms, is_streaming, status, error_message, created_at, completed_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_openclaw_injection_snapshot; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_openclaw_injection_snapshot (id, injection_snapshot_id, instance_id, config_revision_id, bundle_id, compiled_json, checksum, injected_at, created_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_persistent_volume; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_persistent_volume (id, persistent_volume_id, instance_id, volume_name, storage_size_gb, storage_class, mount_path, status, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_risk_hit; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_risk_hit (id, risk_hit_id, trace_id, session_id, request_id, user_id, instance_id, instance_mode, runtime_type, gateway_id, runtime_pod_id, invocation_id, rule_id, rule_name, severity, action, match_summary, created_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_risk_rule; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_risk_rule (id, risk_rule_id, rule_id, display_name, description, pattern, severity, action, is_enabled, sort_order, created_by, updated_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_runtime_pod; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_runtime_pod (id, runtime_pod_id, rollout_id, runtime_type, namespace, pod_name, pod_uid, pod_ip, node_name, deployment_name, image_ref, agent_endpoint, state, capacity, used_slots, draining, cpu_millis_used, memory_bytes_used, disk_bytes_used, network_rx_bytes, network_tx_bytes, metrics_json, last_seen_at, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_runtime_rollout; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_runtime_rollout (id, runtime_rollout_id, runtime_type, target_image_ref, status, batch_size, max_unavailable, started_by, error_message, started_at, finished_at, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_security_scan_config; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_security_scan_config (id, scan_config_id, default_mode, quick_analyzers_json, deep_analyzers_json, quick_timeout_seconds, deep_timeout_seconds, allow_fallback, updated_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_security_scan_job; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_security_scan_job (id, scan_job_id, asset_type, scan_mode, status, requested_by, scope_json, total_items, completed_items, failed_items, current_item_name, started_at, finished_at, created_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_security_scan_job_item; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_security_scan_job_item (id, job_item_id, job_id, asset_type, asset_id, asset_name, status, progress_pct, risk_level, summary, scan_result_id, cached_result, error_message, started_at, finished_at, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_security_scan_report; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_security_scan_report (id, scan_report_id, job_id, summary_json, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_system_image_setting; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_system_image_setting (id, system_image_id, agent, version, display_name, image, is_enabled, updated_by, created_at, updated_at) FROM stdin;
1	agentsysimgubuntuhermes00000000a	hermes	0.21.0	Hermes 桌面 (Ubuntu/KDE + Selkies 远程桌面)	aiarsenal-hermes-desktop:0.21.0	1		2026-07-22 00:00:00	2026-07-22 00:00:00
\.


--
-- Data for Name: aa_agent_team; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_team (id, team_id, user_id, name, description, status, communication_mode, redis_url, team_token, redis_events_last_id, shared_volume_name, shared_mount_path, storage_class, created_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_team_event; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_team_event (id, team_event_id, event_id, completion_id, sequence_no, team_id, member_id, task_id, message_id, event_type, payload_json, redis_stream_id, occurred_at, created_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_team_member; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_team_member (id, team_member_id, team_id, instance_id, user_id, member_key, display_name, role, description, status, current_task_id, progress, last_seen_at, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_team_task; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_team_task (id, team_task_id, team_id, target_member_id, created_by, message_id, status, payload_json, result_json, error_message, redis_stream_id, dispatched_at, started_at, finished_at, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_team_work_item; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_team_work_item (id, work_item_id, team_id, root_task_id, work_id, owner_member_id, title, status, depends_on_json, result_json, artifact_refs_json, started_at, finished_at, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_user_quota; Type: TABLE DATA; Schema: public; Owner: -
--

--
-- Data for Name: aa_agent_user; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_user (id, agent_user_id, username, display_name, role, instance_ids, is_enabled, password_salt, password_hash, remark, created_by, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_user_quota; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_user_quota (id, user_quota_id, user_id, max_instances, max_cpu_cores, max_memory_gb, max_storage_gb, max_gpu_count, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_agent_workspace_file_audit; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_agent_workspace_file_audit (id, workspace_audit_id, instance_id, user_id, action, relative_path, bytes, created_at) FROM stdin;
\.


--
-- Name: aa_agent_audit_event_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_audit_event_id_seq', 1, false);


--
-- Name: aa_agent_audit_log_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_audit_log_id_seq', 1, false);


--
-- Name: aa_agent_backup_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_backup_id_seq', 1, false);


--
-- Name: aa_agent_backup_schedule_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_backup_schedule_id_seq', 1, false);


--
-- Name: aa_agent_config_bundle_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_config_bundle_id_seq', 1, false);


--
-- Name: aa_agent_config_bundle_item_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_config_bundle_item_id_seq', 1, false);


--
-- Name: aa_agent_config_bundle_skill_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_config_bundle_skill_id_seq', 1, false);


--
-- Name: aa_agent_config_resource_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_config_resource_id_seq', 1, false);


--
-- Name: aa_agent_cost_record_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_cost_record_id_seq', 1, false);


--
-- Name: aa_agent_instance_agent_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_instance_agent_id_seq', 1, false);


--
-- Name: aa_agent_instance_command_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_instance_command_id_seq', 1, false);


--
-- Name: aa_agent_instance_config_change_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_instance_config_change_id_seq', 1, false);


--
-- Name: aa_agent_instance_config_revision_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_instance_config_revision_id_seq', 1, false);


--
-- Name: aa_agent_instance_desired_state_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_instance_desired_state_id_seq', 1, false);


--
-- Name: aa_agent_instance_external_access_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_instance_external_access_id_seq', 1, false);


--
-- Name: aa_agent_instance_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_instance_id_seq', 1, false);


--
-- Name: aa_agent_instance_mcp_group_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_instance_mcp_group_id_seq', 1, false);


--
-- Name: aa_agent_instance_runtime_binding_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_instance_runtime_binding_id_seq', 1, false);


--
-- Name: aa_agent_instance_runtime_status_history_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_instance_runtime_status_history_id_seq', 1, false);


--
-- Name: aa_agent_instance_runtime_status_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_instance_runtime_status_id_seq', 1, false);


--
-- Name: aa_agent_instance_skill_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_instance_skill_id_seq', 1, false);


--
-- Name: aa_agent_instance_usage_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_instance_usage_id_seq', 1, false);


--
-- Name: aa_agent_model_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_model_id_seq', 1, false);


--
-- Name: aa_agent_model_invocation_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_model_invocation_id_seq', 1, false);


--
-- Name: aa_agent_openclaw_injection_snapshot_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_openclaw_injection_snapshot_id_seq', 1, false);


--
-- Name: aa_agent_persistent_volume_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_persistent_volume_id_seq', 1, false);


--
-- Name: aa_agent_risk_hit_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_risk_hit_id_seq', 1, false);


--
-- Name: aa_agent_risk_rule_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_risk_rule_id_seq', 1, false);


--
-- Name: aa_agent_runtime_pod_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_runtime_pod_id_seq', 1, false);


--
-- Name: aa_agent_runtime_rollout_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_runtime_rollout_id_seq', 1, false);


--
-- Name: aa_agent_security_scan_config_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_security_scan_config_id_seq', 1, false);


--
-- Name: aa_agent_security_scan_job_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_security_scan_job_id_seq', 1, false);


--
-- Name: aa_agent_security_scan_job_item_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_security_scan_job_item_id_seq', 1, false);


--
-- Name: aa_agent_security_scan_report_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_security_scan_report_id_seq', 1, false);


--
-- Name: aa_agent_system_image_setting_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_system_image_setting_id_seq', 1, true);


--
-- Name: aa_agent_team_event_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_team_event_id_seq', 1, false);


--
-- Name: aa_agent_team_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_team_id_seq', 1, false);


--
-- Name: aa_agent_team_member_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_team_member_id_seq', 1, false);


--
-- Name: aa_agent_team_task_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_team_task_id_seq', 1, false);


--
-- Name: aa_agent_team_work_item_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_team_work_item_id_seq', 1, false);


--
-- Name: aa_agent_user_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_user_id_seq', 1, false);


--
-- Name: aa_agent_user_quota_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_user_quota_id_seq', 1, false);


--
-- Name: aa_agent_workspace_file_audit_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_agent_workspace_file_audit_id_seq', 1, false);


--
-- Name: aa_agent_audit_event aa_agent_audit_event_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_audit_event
    ADD CONSTRAINT aa_agent_audit_event_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_audit_log aa_agent_audit_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_audit_log
    ADD CONSTRAINT aa_agent_audit_log_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_backup aa_agent_backup_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_backup
    ADD CONSTRAINT aa_agent_backup_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_backup_schedule aa_agent_backup_schedule_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_backup_schedule
    ADD CONSTRAINT aa_agent_backup_schedule_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_config_bundle_item aa_agent_config_bundle_item_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_config_bundle_item
    ADD CONSTRAINT aa_agent_config_bundle_item_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_config_bundle aa_agent_config_bundle_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_config_bundle
    ADD CONSTRAINT aa_agent_config_bundle_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_config_bundle_skill aa_agent_config_bundle_skill_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_config_bundle_skill
    ADD CONSTRAINT aa_agent_config_bundle_skill_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_config_resource aa_agent_config_resource_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_config_resource
    ADD CONSTRAINT aa_agent_config_resource_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_cost_record aa_agent_cost_record_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_cost_record
    ADD CONSTRAINT aa_agent_cost_record_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_instance_agent aa_agent_instance_agent_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_agent
    ADD CONSTRAINT aa_agent_instance_agent_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_instance_command aa_agent_instance_command_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_command
    ADD CONSTRAINT aa_agent_instance_command_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_instance_config_change aa_agent_instance_config_change_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_config_change
    ADD CONSTRAINT aa_agent_instance_config_change_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_instance_config_revision aa_agent_instance_config_revision_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_config_revision
    ADD CONSTRAINT aa_agent_instance_config_revision_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_instance_desired_state aa_agent_instance_desired_state_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_desired_state
    ADD CONSTRAINT aa_agent_instance_desired_state_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_instance_external_access aa_agent_instance_external_access_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_external_access
    ADD CONSTRAINT aa_agent_instance_external_access_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_instance_mcp_group aa_agent_instance_mcp_group_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_mcp_group
    ADD CONSTRAINT aa_agent_instance_mcp_group_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_instance aa_agent_instance_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance
    ADD CONSTRAINT aa_agent_instance_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_instance_runtime_binding aa_agent_instance_runtime_binding_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_runtime_binding
    ADD CONSTRAINT aa_agent_instance_runtime_binding_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_instance_runtime_status_history aa_agent_instance_runtime_status_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_runtime_status_history
    ADD CONSTRAINT aa_agent_instance_runtime_status_history_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_instance_runtime_status aa_agent_instance_runtime_status_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_runtime_status
    ADD CONSTRAINT aa_agent_instance_runtime_status_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_instance_skill aa_agent_instance_skill_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_skill
    ADD CONSTRAINT aa_agent_instance_skill_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_instance_usage aa_agent_instance_usage_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_instance_usage
    ADD CONSTRAINT aa_agent_instance_usage_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_model_invocation aa_agent_model_invocation_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_model_invocation
    ADD CONSTRAINT aa_agent_model_invocation_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_model aa_agent_model_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_model
    ADD CONSTRAINT aa_agent_model_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_openclaw_injection_snapshot aa_agent_openclaw_injection_snapshot_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_openclaw_injection_snapshot
    ADD CONSTRAINT aa_agent_openclaw_injection_snapshot_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_persistent_volume aa_agent_persistent_volume_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_persistent_volume
    ADD CONSTRAINT aa_agent_persistent_volume_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_risk_hit aa_agent_risk_hit_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_risk_hit
    ADD CONSTRAINT aa_agent_risk_hit_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_risk_rule aa_agent_risk_rule_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_risk_rule
    ADD CONSTRAINT aa_agent_risk_rule_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_runtime_pod aa_agent_runtime_pod_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_runtime_pod
    ADD CONSTRAINT aa_agent_runtime_pod_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_runtime_rollout aa_agent_runtime_rollout_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_runtime_rollout
    ADD CONSTRAINT aa_agent_runtime_rollout_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_security_scan_config aa_agent_security_scan_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_security_scan_config
    ADD CONSTRAINT aa_agent_security_scan_config_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_security_scan_job_item aa_agent_security_scan_job_item_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_security_scan_job_item
    ADD CONSTRAINT aa_agent_security_scan_job_item_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_security_scan_job aa_agent_security_scan_job_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_security_scan_job
    ADD CONSTRAINT aa_agent_security_scan_job_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_security_scan_report aa_agent_security_scan_report_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_security_scan_report
    ADD CONSTRAINT aa_agent_security_scan_report_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_system_image_setting aa_agent_system_image_setting_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_system_image_setting
    ADD CONSTRAINT aa_agent_system_image_setting_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_team_event aa_agent_team_event_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_team_event
    ADD CONSTRAINT aa_agent_team_event_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_team_member aa_agent_team_member_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_team_member
    ADD CONSTRAINT aa_agent_team_member_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_team aa_agent_team_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_team
    ADD CONSTRAINT aa_agent_team_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_team_task aa_agent_team_task_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_team_task
    ADD CONSTRAINT aa_agent_team_task_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_team_work_item aa_agent_team_work_item_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_team_work_item
    ADD CONSTRAINT aa_agent_team_work_item_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_user_quota aa_agent_user_quota_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

--
-- Name: aa_agent_user_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_user
    ADD CONSTRAINT aa_agent_user_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_user_agent_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_user
    ADD CONSTRAINT aa_agent_user_agent_user_id_key UNIQUE (agent_user_id);


--
-- Name: aa_agent_user_username_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_user
    ADD CONSTRAINT aa_agent_user_username_key UNIQUE (username);


--
-- Name: aa_agent_user_quota aa_agent_user_quota_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_user_quota
    ADD CONSTRAINT aa_agent_user_quota_pkey PRIMARY KEY (id);


--
-- Name: aa_agent_workspace_file_audit aa_agent_workspace_file_audit_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_agent_workspace_file_audit
    ADD CONSTRAINT aa_agent_workspace_file_audit_pkey PRIMARY KEY (id);


--
-- Name: idx_aa_agent_audit_event_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_audit_event_id ON public.aa_agent_audit_event USING btree (audit_event_id);


--
-- Name: idx_aa_agent_audit_event_trace; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_audit_event_trace ON public.aa_agent_audit_event USING btree (trace_id);


--
-- Name: idx_aa_agent_audit_event_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_audit_event_user ON public.aa_agent_audit_event USING btree (user_id, created_at);


--
-- Name: idx_aa_agent_audit_log_action; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_audit_log_action ON public.aa_agent_audit_log USING btree (action, created_at);


--
-- Name: idx_aa_agent_audit_log_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_audit_log_id ON public.aa_agent_audit_log USING btree (audit_log_id);


--
-- Name: idx_aa_agent_audit_log_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_audit_log_user ON public.aa_agent_audit_log USING btree (user_id, created_at);


--
-- Name: idx_aa_agent_backup_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_backup_id ON public.aa_agent_backup USING btree (backup_id);


--
-- Name: idx_aa_agent_backup_instance; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_backup_instance ON public.aa_agent_backup USING btree (instance_id);


--
-- Name: idx_aa_agent_backup_schedule_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_backup_schedule_id ON public.aa_agent_backup_schedule USING btree (backup_schedule_id);


--
-- Name: idx_aa_agent_backup_schedule_instance; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_backup_schedule_instance ON public.aa_agent_backup_schedule USING btree (instance_id);


--
-- Name: idx_aa_agent_config_bundle_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_config_bundle_id ON public.aa_agent_config_bundle USING btree (config_bundle_id);


--
-- Name: idx_aa_agent_config_bundle_item_bundle; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_config_bundle_item_bundle ON public.aa_agent_config_bundle_item USING btree (bundle_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_agent_config_bundle_item_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_config_bundle_item_id ON public.aa_agent_config_bundle_item USING btree (bundle_item_id);


--
-- Name: idx_aa_agent_config_bundle_skill_bundle; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_config_bundle_skill_bundle ON public.aa_agent_config_bundle_skill USING btree (bundle_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_agent_config_bundle_skill_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_config_bundle_skill_id ON public.aa_agent_config_bundle_skill USING btree (bundle_skill_id);


--
-- Name: idx_aa_agent_config_bundle_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_config_bundle_user ON public.aa_agent_config_bundle USING btree (user_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_agent_config_resource_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_config_resource_id ON public.aa_agent_config_resource USING btree (config_resource_id);


--
-- Name: idx_aa_agent_config_resource_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_config_resource_type ON public.aa_agent_config_resource USING btree (resource_type) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_agent_config_resource_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_config_resource_user ON public.aa_agent_config_resource USING btree (user_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_agent_cost_record_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_cost_record_id ON public.aa_agent_cost_record USING btree (cost_record_id);


--
-- Name: idx_aa_agent_cost_record_trace; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_cost_record_trace ON public.aa_agent_cost_record USING btree (trace_id);


--
-- Name: idx_aa_agent_cost_record_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_cost_record_user ON public.aa_agent_cost_record USING btree (user_id, recorded_at);


--
-- Name: idx_aa_agent_instance_agent_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_instance_agent_id ON public.aa_agent_instance_agent USING btree (instance_agent_id);


--
-- Name: idx_aa_agent_instance_agent_instance; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_instance_agent_instance ON public.aa_agent_instance_agent USING btree (instance_id);


--
-- Name: idx_aa_agent_instance_command_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_instance_command_id ON public.aa_agent_instance_command USING btree (instance_command_id);


--
-- Name: idx_aa_agent_instance_command_instance; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_instance_command_instance ON public.aa_agent_instance_command USING btree (instance_id, status);


--
-- Name: idx_aa_agent_instance_config_change_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_instance_config_change_id ON public.aa_agent_instance_config_change USING btree (change_id);


--
-- Name: idx_aa_agent_instance_config_change_instance; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_instance_config_change_instance ON public.aa_agent_instance_config_change USING btree (instance_id, id);


--
-- Name: idx_aa_agent_instance_config_revision_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_instance_config_revision_id ON public.aa_agent_instance_config_revision USING btree (config_revision_id);


--
-- Name: idx_aa_agent_instance_config_revision_instance; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_instance_config_revision_instance ON public.aa_agent_instance_config_revision USING btree (instance_id, revision_no);


--
-- Name: idx_aa_agent_instance_desired_state_instance; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_instance_desired_state_instance ON public.aa_agent_instance_desired_state USING btree (instance_id);


--
-- Name: idx_aa_agent_instance_external_access_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_instance_external_access_id ON public.aa_agent_instance_external_access USING btree (external_access_id);


--
-- Name: idx_aa_agent_instance_external_access_instance; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_instance_external_access_instance ON public.aa_agent_instance_external_access USING btree (instance_id);


--
-- Name: idx_aa_agent_instance_external_access_slug; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_instance_external_access_slug ON public.aa_agent_instance_external_access USING btree (public_slug) WHERE (enabled = 1);


--
-- Name: idx_aa_agent_instance_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_instance_id ON public.aa_agent_instance USING btree (instance_id);


--
-- Name: idx_aa_agent_instance_mcp_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_instance_mcp_id ON public.aa_agent_instance_mcp_group USING btree (instance_mcp_id);


--
-- Name: idx_aa_agent_instance_mcp_instance; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_instance_mcp_instance ON public.aa_agent_instance_mcp_group USING btree (instance_id);


--
-- Name: idx_aa_agent_instance_mcp_pair; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_instance_mcp_pair ON public.aa_agent_instance_mcp_group USING btree (instance_id, group_id);


--
-- Name: idx_aa_agent_instance_runtime_binding_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_instance_runtime_binding_id ON public.aa_agent_instance_runtime_binding USING btree (runtime_binding_id);


--
-- Name: idx_aa_agent_instance_runtime_binding_instance; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_instance_runtime_binding_instance ON public.aa_agent_instance_runtime_binding USING btree (instance_id);


--
-- Name: idx_aa_agent_instance_runtime_status_history_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_instance_runtime_status_history_id ON public.aa_agent_instance_runtime_status_history USING btree (history_id);


--
-- Name: idx_aa_agent_instance_runtime_status_history_instance; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_instance_runtime_status_history_instance ON public.aa_agent_instance_runtime_status_history USING btree (instance_id, id);


--
-- Name: idx_aa_agent_instance_runtime_status_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_instance_runtime_status_id ON public.aa_agent_instance_runtime_status USING btree (instance_runtime_status_id);


--
-- Name: idx_aa_agent_instance_runtime_status_instance; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_instance_runtime_status_instance ON public.aa_agent_instance_runtime_status USING btree (instance_id);


--
-- Name: idx_aa_agent_instance_skill_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_instance_skill_id ON public.aa_agent_instance_skill USING btree (instance_skill_id);


--
-- Name: idx_aa_agent_instance_skill_instance; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_instance_skill_instance ON public.aa_agent_instance_skill USING btree (instance_id);


--
-- Name: idx_aa_agent_instance_skill_pair; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_instance_skill_pair ON public.aa_agent_instance_skill USING btree (instance_id, skill_id);


--
-- Name: idx_aa_agent_instance_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_instance_status ON public.aa_agent_instance USING btree (status) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_agent_instance_usage_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_instance_usage_id ON public.aa_agent_instance_usage USING btree (instance_usage_id);


--
-- Name: idx_aa_agent_instance_usage_instance; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_instance_usage_instance ON public.aa_agent_instance_usage USING btree (instance_id, recorded_at);


--
-- Name: idx_aa_agent_instance_usage_recorded; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_instance_usage_recorded ON public.aa_agent_instance_usage USING btree (recorded_at);


--
-- Name: idx_aa_agent_instance_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_instance_user ON public.aa_agent_instance USING btree (user_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_agent_instance_user_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_instance_user_name ON public.aa_agent_instance USING btree (user_id, name) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_agent_model_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_model_id ON public.aa_agent_model USING btree (agent_model_id);


--
-- Name: idx_aa_agent_model_invocation_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_model_invocation_id ON public.aa_agent_model_invocation USING btree (invocation_id);


--
-- Name: idx_aa_agent_model_invocation_model; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_model_invocation_model ON public.aa_agent_model_invocation USING btree (model_id, created_at);


--
-- Name: idx_aa_agent_model_invocation_trace; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_model_invocation_trace ON public.aa_agent_model_invocation USING btree (trace_id);


--
-- Name: idx_aa_agent_model_invocation_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_model_invocation_user ON public.aa_agent_model_invocation USING btree (user_id, created_at);


--
-- Name: idx_aa_agent_model_provider; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_model_provider ON public.aa_agent_model USING btree (provider_type) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_agent_openclaw_injection_snapshot_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_openclaw_injection_snapshot_id ON public.aa_agent_openclaw_injection_snapshot USING btree (injection_snapshot_id);


--
-- Name: idx_aa_agent_openclaw_injection_snapshot_instance; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_openclaw_injection_snapshot_instance ON public.aa_agent_openclaw_injection_snapshot USING btree (instance_id);


--
-- Name: idx_aa_agent_persistent_volume_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_persistent_volume_id ON public.aa_agent_persistent_volume USING btree (persistent_volume_id);


--
-- Name: idx_aa_agent_persistent_volume_instance; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_persistent_volume_instance ON public.aa_agent_persistent_volume USING btree (instance_id);


--
-- Name: idx_aa_agent_persistent_volume_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_persistent_volume_name ON public.aa_agent_persistent_volume USING btree (volume_name);


--
-- Name: idx_aa_agent_risk_hit_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_risk_hit_id ON public.aa_agent_risk_hit USING btree (risk_hit_id);


--
-- Name: idx_aa_agent_risk_hit_rule; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_risk_hit_rule ON public.aa_agent_risk_hit USING btree (rule_id, created_at);


--
-- Name: idx_aa_agent_risk_hit_trace; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_risk_hit_trace ON public.aa_agent_risk_hit USING btree (trace_id);


--
-- Name: idx_aa_agent_risk_rule_code; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_risk_rule_code ON public.aa_agent_risk_rule USING btree (rule_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_agent_risk_rule_enabled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_risk_rule_enabled ON public.aa_agent_risk_rule USING btree (is_enabled) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_agent_risk_rule_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_risk_rule_id ON public.aa_agent_risk_rule USING btree (risk_rule_id);


--
-- Name: idx_aa_agent_runtime_pod_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_runtime_pod_id ON public.aa_agent_runtime_pod USING btree (runtime_pod_id);


--
-- Name: idx_aa_agent_runtime_pod_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_runtime_pod_name ON public.aa_agent_runtime_pod USING btree (pod_name);


--
-- Name: idx_aa_agent_runtime_pod_rollout; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_runtime_pod_rollout ON public.aa_agent_runtime_pod USING btree (rollout_id, id);


--
-- Name: idx_aa_agent_runtime_pod_state; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_runtime_pod_state ON public.aa_agent_runtime_pod USING btree (state);


--
-- Name: idx_aa_agent_runtime_rollout_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_runtime_rollout_id ON public.aa_agent_runtime_rollout USING btree (runtime_rollout_id);


--
-- Name: idx_aa_agent_runtime_rollout_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_runtime_rollout_status ON public.aa_agent_runtime_rollout USING btree (status);


--
-- Name: idx_aa_agent_security_scan_config_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_security_scan_config_id ON public.aa_agent_security_scan_config USING btree (scan_config_id);


--
-- Name: idx_aa_agent_security_scan_job_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_security_scan_job_id ON public.aa_agent_security_scan_job USING btree (scan_job_id);


--
-- Name: idx_aa_agent_security_scan_job_item_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_security_scan_job_item_id ON public.aa_agent_security_scan_job_item USING btree (job_item_id);


--
-- Name: idx_aa_agent_security_scan_job_item_job; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_security_scan_job_item_job ON public.aa_agent_security_scan_job_item USING btree (job_id);


--
-- Name: idx_aa_agent_security_scan_job_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_security_scan_job_status ON public.aa_agent_security_scan_job USING btree (status) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_agent_security_scan_report_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_security_scan_report_id ON public.aa_agent_security_scan_report USING btree (scan_report_id);


--
-- Name: idx_aa_agent_security_scan_report_job; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_security_scan_report_job ON public.aa_agent_security_scan_report USING btree (job_id);


--
-- Name: idx_aa_agent_system_image_setting_agent_version; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_system_image_setting_agent_version ON public.aa_agent_system_image_setting USING btree (agent, version);


--
-- Name: idx_aa_agent_system_image_setting_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_system_image_setting_id ON public.aa_agent_system_image_setting USING btree (system_image_id);


--
-- Name: idx_aa_agent_team_event_event; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_team_event_event ON public.aa_agent_team_event USING btree (team_id, event_id);


--
-- Name: idx_aa_agent_team_event_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_team_event_id ON public.aa_agent_team_event USING btree (team_event_id);


--
-- Name: idx_aa_agent_team_event_team_seq; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_team_event_team_seq ON public.aa_agent_team_event USING btree (team_id, sequence_no);


--
-- Name: idx_aa_agent_team_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_team_id ON public.aa_agent_team USING btree (team_id);


--
-- Name: idx_aa_agent_team_member_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_team_member_id ON public.aa_agent_team_member USING btree (team_member_id);


--
-- Name: idx_aa_agent_team_member_pair; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_team_member_pair ON public.aa_agent_team_member USING btree (team_id, member_key);


--
-- Name: idx_aa_agent_team_member_team; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_team_member_team ON public.aa_agent_team_member USING btree (team_id);


--
-- Name: idx_aa_agent_team_task_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_team_task_id ON public.aa_agent_team_task USING btree (team_task_id);


--
-- Name: idx_aa_agent_team_task_team; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_team_task_team ON public.aa_agent_team_task USING btree (team_id, status);


--
-- Name: idx_aa_agent_team_user_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_team_user_name ON public.aa_agent_team USING btree (user_id, name) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_agent_team_work_item_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_team_work_item_id ON public.aa_agent_team_work_item USING btree (work_item_id);


--
-- Name: idx_aa_agent_team_work_item_root; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_team_work_item_root ON public.aa_agent_team_work_item USING btree (root_task_id, status);


--
-- Name: idx_aa_agent_team_work_item_work; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_team_work_item_work ON public.aa_agent_team_work_item USING btree (team_id, root_task_id, work_id);


--
-- Name: idx_aa_agent_user_quota_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_user_quota_id ON public.aa_agent_user_quota USING btree (user_quota_id);


--
-- Name: idx_aa_agent_user_quota_user; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_user_quota_user ON public.aa_agent_user_quota USING btree (user_id);


--
-- Name: idx_aa_agent_workspace_file_audit_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_agent_workspace_file_audit_id ON public.aa_agent_workspace_file_audit USING btree (workspace_audit_id);


--
-- Name: idx_aa_agent_workspace_file_audit_instance; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_agent_workspace_file_audit_instance ON public.aa_agent_workspace_file_audit USING btree (instance_id, created_at);


--
-- PostgreSQL database dump complete
--

\unrestrict oezNLvxU6qzfdqxFDox53FOUkAmcGg0OpT3wzrrJ7dJT4O8HSO8lIyCBhDfCOOL
