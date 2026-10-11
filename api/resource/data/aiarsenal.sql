-- aiarsenal 基线（squash 固化版）

-- ============================================================================
-- 基线固化说明（2026-09 squash）：正文为 pg_dump 固化版，已合并全部历史迁移（原 migrations/ 已删，git 可查）。原尾部“connect aiarsenal 灌菜单”段已并入 aiarsenal.sql。后续变更请新建迁移文件（从 000001 重新编号）；存量库需先 TRUNCATE schema_migrations。

--
-- PostgreSQL database dump
--

\restrict ixXf5k83YrWMPq6rgBcTO96NwdVldCXO66ThxuXV72oCXAx2k6DRDgAU73I8A5K

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

DROP INDEX IF EXISTS public.idx_aa_user_user_mobile;
DROP INDEX IF EXISTS public.idx_aa_user_user_login;
DROP INDEX IF EXISTS public.idx_aa_user_user_id;
DROP INDEX IF EXISTS public.idx_aa_user_user_empno;
DROP INDEX IF EXISTS public.idx_aa_user_user_email;
DROP INDEX IF EXISTS public.idx_aa_user_online_uni_uuid;
DROP INDEX IF EXISTS public.idx_aa_user_online_uni_token;
DROP INDEX IF EXISTS public.idx_aa_user_name;
DROP INDEX IF EXISTS public.idx_aa_role_role_id;
DROP INDEX IF EXISTS public.idx_aa_review_order_status;
DROP INDEX IF EXISTS public.idx_aa_review_order_id;
DROP INDEX IF EXISTS public.idx_aa_review_order_applicant;
DROP INDEX IF EXISTS public.idx_aa_post_post_id;
DROP INDEX IF EXISTS public.idx_aa_perm_group_group_id;
DROP INDEX IF EXISTS public.idx_aa_oper_log_oper_id;
DROP INDEX IF EXISTS public.idx_aa_notice_user_read;
DROP INDEX IF EXISTS public.idx_aa_notice_user_id;
DROP INDEX IF EXISTS public.idx_aa_notice_notice_id;
DROP INDEX IF EXISTS public.idx_aa_menu_weigh;
DROP INDEX IF EXISTS public.idx_aa_menu_pid;
DROP INDEX IF EXISTS public.idx_aa_menu_name;
DROP INDEX IF EXISTS public.idx_aa_menu_menu_id;
DROP INDEX IF EXISTS public.idx_aa_login_log_info_id;
DROP INDEX IF EXISTS public.idx_aa_emp_set_set_id;
DROP INDEX IF EXISTS public.idx_aa_dept_dept_id;
ALTER TABLE IF EXISTS ONLY public.aa_user_post DROP CONSTRAINT IF EXISTS aa_user_post_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_user DROP CONSTRAINT IF EXISTS aa_user_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_user_online DROP CONSTRAINT IF EXISTS aa_user_online_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_role DROP CONSTRAINT IF EXISTS aa_role_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_review_order DROP CONSTRAINT IF EXISTS aa_review_order_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_post DROP CONSTRAINT IF EXISTS aa_post_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_perm_group DROP CONSTRAINT IF EXISTS aa_perm_group_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_oper_log DROP CONSTRAINT IF EXISTS aa_oper_log_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_notice DROP CONSTRAINT IF EXISTS aa_notice_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_menu DROP CONSTRAINT IF EXISTS aa_menu_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_login_log DROP CONSTRAINT IF EXISTS aa_login_log_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_emp_set DROP CONSTRAINT IF EXISTS aa_emp_set_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_dept DROP CONSTRAINT IF EXISTS aa_dept_pkey;
ALTER TABLE IF EXISTS public.aa_user_post ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_user_online ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_user ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_role ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_review_order ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_post ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_perm_group ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_oper_log ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_notice ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_menu ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_login_log ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_emp_set ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_dept ALTER COLUMN id DROP DEFAULT;
DROP SEQUENCE IF EXISTS public.aa_user_post_id_seq;
DROP TABLE IF EXISTS public.aa_user_post;
DROP SEQUENCE IF EXISTS public.aa_user_online_id_seq;
DROP TABLE IF EXISTS public.aa_user_online;
DROP SEQUENCE IF EXISTS public.aa_user_id_seq;
DROP TABLE IF EXISTS public.aa_user;
DROP TABLE IF EXISTS public.aa_rule;
DROP SEQUENCE IF EXISTS public.aa_role_id_seq;
DROP TABLE IF EXISTS public.aa_role;
DROP SEQUENCE IF EXISTS public.aa_review_order_id_seq;
DROP TABLE IF EXISTS public.aa_review_order;
DROP SEQUENCE IF EXISTS public.aa_post_id_seq;
DROP TABLE IF EXISTS public.aa_post;
DROP SEQUENCE IF EXISTS public.aa_perm_group_id_seq;
DROP TABLE IF EXISTS public.aa_perm_group;
DROP SEQUENCE IF EXISTS public.aa_oper_log_id_seq;
DROP TABLE IF EXISTS public.aa_oper_log;
DROP SEQUENCE IF EXISTS public.aa_notice_id_seq;
DROP TABLE IF EXISTS public.aa_notice;
DROP SEQUENCE IF EXISTS public.aa_menu_id_seq;
DROP TABLE IF EXISTS public.aa_menu;
DROP SEQUENCE IF EXISTS public.aa_login_log_id_seq;
DROP TABLE IF EXISTS public.aa_login_log;
DROP SEQUENCE IF EXISTS public.aa_emp_set_id_seq;
DROP TABLE IF EXISTS public.aa_emp_set;
DROP SEQUENCE IF EXISTS public.aa_dept_id_seq;
DROP TABLE IF EXISTS public.aa_dept;
SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: aa_dept; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_dept (
    id bigint NOT NULL,
    dept_id character varying(32) DEFAULT ''::character varying NOT NULL,
    parent_id character varying(32) DEFAULT '0'::character varying NOT NULL,
    ancestors character varying(500) DEFAULT ''::character varying,
    dept_name character varying(30) DEFAULT ''::character varying,
    order_num integer DEFAULT 0,
    leader_id character varying(32) DEFAULT '0'::character varying NOT NULL,
    status smallint DEFAULT 0,
    created_by character varying(32) DEFAULT '0'::character varying,
    updated_by character varying(32) DEFAULT '0'::character varying,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone,
    llm_models text DEFAULT ''::text NOT NULL,
    llm_tpm_limit bigint,
    llm_rpm_limit bigint,
    llm_max_parallel_requests integer,
    llm_budget_duration character varying(20),
    llm_budget_reset_at timestamp without time zone,
    llm_router_settings text DEFAULT ''::text NOT NULL,
    llm_blocked smallint DEFAULT 0 NOT NULL
);


--
-- Name: aa_dept_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_dept_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_dept_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_dept_id_seq OWNED BY public.aa_dept.id;


--
-- Name: aa_emp_set; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_emp_set (
    id bigint NOT NULL,
    set_id character varying(32) DEFAULT ''::character varying NOT NULL,
    name character varying(64) DEFAULT ''::character varying NOT NULL,
    remark character varying(255) DEFAULT ''::character varying NOT NULL,
    emp_nos jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_emp_set_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_emp_set_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_emp_set_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_emp_set_id_seq OWNED BY public.aa_emp_set.id;


--
-- Name: aa_login_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_login_log (
    id bigint NOT NULL,
    info_id character varying(32) DEFAULT ''::character varying NOT NULL,
    login_name character varying(50) DEFAULT ''::character varying,
    ipaddr character varying(50) DEFAULT ''::character varying,
    login_location character varying(255) DEFAULT ''::character varying,
    browser character varying(50) DEFAULT ''::character varying,
    os character varying(50) DEFAULT ''::character varying,
    status smallint DEFAULT 0,
    msg character varying(255) DEFAULT ''::character varying,
    login_time timestamp without time zone,
    module character varying(30) DEFAULT ''::character varying
);


--
-- Name: aa_login_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_login_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_login_log_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_login_log_id_seq OWNED BY public.aa_login_log.id;


--
-- Name: aa_menu; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_menu (
    id bigint NOT NULL,
    menu_id character varying(32) DEFAULT ''::character varying NOT NULL,
    pid character varying(32) DEFAULT '0'::character varying NOT NULL,
    name character varying(100) DEFAULT ''::character varying NOT NULL,
    title character varying(50) DEFAULT ''::character varying NOT NULL,
    en_name character varying(50) DEFAULT ''::character varying NOT NULL,
    icon character varying(300) DEFAULT ''::character varying NOT NULL,
    condition character varying(255) DEFAULT ''::character varying NOT NULL,
    remark character varying(255) DEFAULT ''::character varying NOT NULL,
    menu_type smallint DEFAULT 0 NOT NULL,
    weigh integer DEFAULT 0 NOT NULL,
    is_hide smallint DEFAULT 0 NOT NULL,
    path character varying(100) DEFAULT ''::character varying NOT NULL,
    component character varying(100) DEFAULT ''::character varying NOT NULL,
    is_link smallint DEFAULT 0 NOT NULL,
    module_type character varying(30) DEFAULT ''::character varying NOT NULL,
    model_id integer DEFAULT 0 NOT NULL,
    is_iframe smallint DEFAULT 0 NOT NULL,
    is_cached smallint DEFAULT 0 NOT NULL,
    redirect character varying(255) DEFAULT ''::character varying NOT NULL,
    is_affix smallint DEFAULT 0 NOT NULL,
    link_url character varying(500) DEFAULT ''::character varying NOT NULL,
    grant_rules jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_menu_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_menu_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_menu_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_menu_id_seq OWNED BY public.aa_menu.id;


--
-- Name: aa_notice; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_notice (
    id bigint NOT NULL,
    notice_id character varying(32) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    title character varying(200) DEFAULT ''::character varying NOT NULL,
    content text,
    link character varying(500) DEFAULT ''::character varying NOT NULL,
    type character varying(30) DEFAULT 'info'::character varying NOT NULL,
    is_read smallint DEFAULT 0 NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone
);


--
-- Name: aa_notice_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_notice_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_notice_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_notice_id_seq OWNED BY public.aa_notice.id;


--
-- Name: aa_oper_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_oper_log (
    id bigint NOT NULL,
    oper_id character varying(32) DEFAULT ''::character varying NOT NULL,
    title character varying(50) DEFAULT ''::character varying,
    business_type integer DEFAULT 0,
    method character varying(100) DEFAULT ''::character varying,
    request_method character varying(10) DEFAULT ''::character varying,
    operator_type integer DEFAULT 0,
    oper_name character varying(50) DEFAULT ''::character varying,
    dept_name character varying(50) DEFAULT ''::character varying,
    oper_url character varying(500) DEFAULT ''::character varying,
    oper_ip character varying(50) DEFAULT ''::character varying,
    oper_location character varying(255) DEFAULT ''::character varying,
    oper_param text,
    error_msg character varying(2000) DEFAULT ''::character varying,
    oper_time timestamp without time zone
);


--
-- Name: aa_oper_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_oper_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_oper_log_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_oper_log_id_seq OWNED BY public.aa_oper_log.id;


--
-- Name: aa_perm_group; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_perm_group (
    id bigint NOT NULL,
    group_id character varying(32) DEFAULT ''::character varying NOT NULL,
    name character varying(64) DEFAULT ''::character varying NOT NULL,
    remark character varying(255) DEFAULT ''::character varying NOT NULL,
    terms jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_perm_group_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_perm_group_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_perm_group_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_perm_group_id_seq OWNED BY public.aa_perm_group.id;


--
-- Name: aa_post; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_post (
    id bigint NOT NULL,
    post_id character varying(32) DEFAULT ''::character varying NOT NULL,
    post_code character varying(64) DEFAULT ''::character varying,
    post_name character varying(50) DEFAULT ''::character varying,
    post_sort integer DEFAULT 0,
    status smallint DEFAULT 0,
    remark character varying(500) DEFAULT NULL::character varying,
    created_by character varying(32) DEFAULT '0'::character varying,
    updated_by character varying(32) DEFAULT '0'::character varying,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_post_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_post_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_post_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_post_id_seq OWNED BY public.aa_post.id;


--
-- Name: aa_review_order; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_review_order (
    id bigint NOT NULL,
    order_id character varying(32) DEFAULT ''::character varying NOT NULL,
    order_type character varying(32) DEFAULT ''::character varying NOT NULL,
    title character varying(200) DEFAULT ''::character varying NOT NULL,
    applicant_id character varying(32) DEFAULT ''::character varying NOT NULL,
    applicant_name character varying(100) DEFAULT ''::character varying NOT NULL,
    dept_id character varying(32) DEFAULT ''::character varying NOT NULL,
    amount bigint DEFAULT 0 NOT NULL,
    payload text DEFAULT '{}'::text NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    reviewer_id character varying(32) DEFAULT ''::character varying NOT NULL,
    reviewer_name character varying(100) DEFAULT ''::character varying NOT NULL,
    review_note character varying(500) DEFAULT ''::character varying NOT NULL,
    result_ref character varying(64) DEFAULT ''::character varying NOT NULL,
    reviewed_at timestamp without time zone,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    updated_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_review_order_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_review_order_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_review_order_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_review_order_id_seq OWNED BY public.aa_review_order.id;


--
-- Name: aa_role; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_role (
    id bigint NOT NULL,
    role_id character varying(32) DEFAULT ''::character varying NOT NULL,
    status smallint DEFAULT 0,
    list_order integer DEFAULT 0,
    name character varying(30) DEFAULT ''::character varying,
    remark character varying(100) DEFAULT NULL::character varying,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_role_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_role_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_role_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_role_id_seq OWNED BY public.aa_role.id;


--
-- Name: aa_rule; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_rule (
    ptype character varying(10) DEFAULT NULL::character varying,
    v0 character varying(256) DEFAULT NULL::character varying,
    v1 character varying(256) DEFAULT NULL::character varying,
    v2 character varying(256) DEFAULT NULL::character varying,
    v3 character varying(256) DEFAULT NULL::character varying,
    v4 character varying(256) DEFAULT NULL::character varying,
    v5 character varying(256) DEFAULT NULL::character varying
);


--
-- Name: aa_user; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_user (
    id bigint NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    user_name character varying(60) DEFAULT ''::character varying NOT NULL,
    employee_no character varying(20) DEFAULT ''::character varying NOT NULL,
    name character varying(50) DEFAULT ''::character varying NOT NULL,
    mobile character varying(20) DEFAULT ''::character varying NOT NULL,
    user_nickname character varying(50) DEFAULT ''::character varying NOT NULL,
    birthday integer DEFAULT 0 NOT NULL,
    user_password character varying(255) DEFAULT ''::character varying NOT NULL,
    user_salt character(10) NOT NULL,
    user_status smallint DEFAULT 1 NOT NULL,
    user_email character varying(100) DEFAULT ''::character varying NOT NULL,
    sex smallint DEFAULT 0 NOT NULL,
    avatar character varying(255) DEFAULT ''::character varying NOT NULL,
    dept_id character varying(32) DEFAULT '0'::character varying NOT NULL,
    remark character varying(255) DEFAULT ''::character varying NOT NULL,
    is_admin smallint DEFAULT 0 NOT NULL,
    address character varying(255) DEFAULT ''::character varying NOT NULL,
    describe character varying(255) DEFAULT ''::character varying NOT NULL,
    last_login_ip character varying(50) DEFAULT ''::character varying NOT NULL,
    last_login_time timestamp without time zone,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_user_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_user_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_user_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_user_id_seq OWNED BY public.aa_user.id;


--
-- Name: aa_user_online; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_user_online (
    id bigint NOT NULL,
    uuid character(32) DEFAULT ''::bpchar NOT NULL,
    token character varying(255) DEFAULT ''::character varying NOT NULL,
    create_time timestamp without time zone,
    user_name character varying(255) NOT NULL,
    ip character varying(120) DEFAULT ''::character varying NOT NULL,
    explorer character varying(30) DEFAULT ''::character varying NOT NULL,
    os character varying(30) DEFAULT ''::character varying NOT NULL
);


--
-- Name: aa_user_online_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_user_online_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_user_online_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_user_online_id_seq OWNED BY public.aa_user_online.id;


--
-- Name: aa_user_post; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_user_post (
    id bigint NOT NULL,
    user_id character varying(32) NOT NULL,
    post_id character varying(32) NOT NULL
);


--
-- Name: aa_user_post_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_user_post_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_user_post_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_user_post_id_seq OWNED BY public.aa_user_post.id;


--
-- Name: aa_dept id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_dept ALTER COLUMN id SET DEFAULT nextval('public.aa_dept_id_seq'::regclass);


--
-- Name: aa_emp_set id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_emp_set ALTER COLUMN id SET DEFAULT nextval('public.aa_emp_set_id_seq'::regclass);


--
-- Name: aa_login_log id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_login_log ALTER COLUMN id SET DEFAULT nextval('public.aa_login_log_id_seq'::regclass);


--
-- Name: aa_menu id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_menu ALTER COLUMN id SET DEFAULT nextval('public.aa_menu_id_seq'::regclass);


--
-- Name: aa_notice id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_notice ALTER COLUMN id SET DEFAULT nextval('public.aa_notice_id_seq'::regclass);


--
-- Name: aa_oper_log id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_oper_log ALTER COLUMN id SET DEFAULT nextval('public.aa_oper_log_id_seq'::regclass);


--
-- Name: aa_perm_group id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_perm_group ALTER COLUMN id SET DEFAULT nextval('public.aa_perm_group_id_seq'::regclass);


--
-- Name: aa_post id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_post ALTER COLUMN id SET DEFAULT nextval('public.aa_post_id_seq'::regclass);


--
-- Name: aa_review_order id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_review_order ALTER COLUMN id SET DEFAULT nextval('public.aa_review_order_id_seq'::regclass);


--
-- Name: aa_role id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_role ALTER COLUMN id SET DEFAULT nextval('public.aa_role_id_seq'::regclass);


--
-- Name: aa_user id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_user ALTER COLUMN id SET DEFAULT nextval('public.aa_user_id_seq'::regclass);


--
-- Name: aa_user_online id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_user_online ALTER COLUMN id SET DEFAULT nextval('public.aa_user_online_id_seq'::regclass);


--
-- Name: aa_user_post id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_user_post ALTER COLUMN id SET DEFAULT nextval('public.aa_user_post_id_seq'::regclass);


--
-- Data for Name: aa_dept; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_dept (id, dept_id, parent_id, ancestors, dept_name, order_num, leader_id, status, created_by, updated_by, created_at, updated_at, deleted_at, llm_models, llm_tpm_limit, llm_rpm_limit, llm_max_parallel_requests, llm_budget_duration, llm_budget_reset_at, llm_router_settings, llm_blocked) FROM stdin;
100	dqr6r6e6exwggi02v2h0tqkbdd46c3sb	0	0	演示集团公司	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
101	lvyrsk0fjk7br36yh0ti1fug3c8oewsz	dqr6r6e6exwggi02v2h0tqkbdd46c3sb	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb	第一分公司	2	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
102	yvd2rzcq3p6on80czv6q7955j7gc5y3t	lvyrsk0fjk7br36yh0ti1fug3c8oewsz	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz	第一作业区	3	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
103	oyy8hxpdfp1b5blfpq8or6tk15hvhwwk	yvd2rzcq3p6on80czv6q7955j7gc5y3t	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz,yvd2rzcq3p6on80czv6q7955j7gc5y3t	1号工区	4	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
104	g84a2esunq3ggigh6rie4iso5mvqejoc	oyy8hxpdfp1b5blfpq8or6tk15hvhwwk	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz,yvd2rzcq3p6on80czv6q7955j7gc5y3t,oyy8hxpdfp1b5blfpq8or6tk15hvhwwk	甲班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
105	nfnkkl3va2fdvuk9fwmiz21y15xpl8lh	oyy8hxpdfp1b5blfpq8or6tk15hvhwwk	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz,yvd2rzcq3p6on80czv6q7955j7gc5y3t,oyy8hxpdfp1b5blfpq8or6tk15hvhwwk	乙班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
106	v5r1ilh98qyl0kwnsrfr79whbyy1twzf	oyy8hxpdfp1b5blfpq8or6tk15hvhwwk	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz,yvd2rzcq3p6on80czv6q7955j7gc5y3t,oyy8hxpdfp1b5blfpq8or6tk15hvhwwk	丙班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
107	fnrcvzwdge0dwbwjaeyilydchrlmff9g	yvd2rzcq3p6on80czv6q7955j7gc5y3t	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz,yvd2rzcq3p6on80czv6q7955j7gc5y3t	2号工区	4	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
108	n7c0xt6ksdqwia6gjrtebatzlzz4c2tb	fnrcvzwdge0dwbwjaeyilydchrlmff9g	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz,yvd2rzcq3p6on80czv6q7955j7gc5y3t,fnrcvzwdge0dwbwjaeyilydchrlmff9g	早班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
109	qdei4bqdhl2s48229y716b7zmyl0yhov	fnrcvzwdge0dwbwjaeyilydchrlmff9g	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz,yvd2rzcq3p6on80czv6q7955j7gc5y3t,fnrcvzwdge0dwbwjaeyilydchrlmff9g	中班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
110	gzbhisqghr688sew022riq0m9ph1tnul	fnrcvzwdge0dwbwjaeyilydchrlmff9g	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz,yvd2rzcq3p6on80czv6q7955j7gc5y3t,fnrcvzwdge0dwbwjaeyilydchrlmff9g	晚班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
111	lcx6976nybie9legnbf7efda17gzms8p	yvd2rzcq3p6on80czv6q7955j7gc5y3t	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz,yvd2rzcq3p6on80czv6q7955j7gc5y3t	中转站	4	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
112	tuon2vwblsnov4mnykch7sypzbvwgqma	lcx6976nybie9legnbf7efda17gzms8p	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz,yvd2rzcq3p6on80czv6q7955j7gc5y3t,lcx6976nybie9legnbf7efda17gzms8p	装卸班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
113	v82stxipz7d1bdq1iz4ovthbs019tphi	lcx6976nybie9legnbf7efda17gzms8p	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz,yvd2rzcq3p6on80czv6q7955j7gc5y3t,lcx6976nybie9legnbf7efda17gzms8p	检验班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
114	kheqge89r0euopogj346fyg25m8t3rya	lvyrsk0fjk7br36yh0ti1fug3c8oewsz	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz	第二作业区	3	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
115	fop8s0srp4a6or6ceam5nyp6632unzbg	kheqge89r0euopogj346fyg25m8t3rya	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz,kheqge89r0euopogj346fyg25m8t3rya	3号工区	4	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
116	hbxawf9h3d7xkyjmbw4uqeon4i3nfn2w	fop8s0srp4a6or6ceam5nyp6632unzbg	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz,kheqge89r0euopogj346fyg25m8t3rya,fop8s0srp4a6or6ceam5nyp6632unzbg	甲班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
117	ivgstgmxznc1erf0sxvtxjqfikq7wazs	fop8s0srp4a6or6ceam5nyp6632unzbg	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz,kheqge89r0euopogj346fyg25m8t3rya,fop8s0srp4a6or6ceam5nyp6632unzbg	乙班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
118	kye5pyfqoq5doipzenotptllr2g6mxqg	kheqge89r0euopogj346fyg25m8t3rya	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz,kheqge89r0euopogj346fyg25m8t3rya	供水站	4	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
119	zujehiadtnjaqtgooixca2iqp9safihi	kye5pyfqoq5doipzenotptllr2g6mxqg	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz,kheqge89r0euopogj346fyg25m8t3rya,kye5pyfqoq5doipzenotptllr2g6mxqg	供水班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
120	eow0qt4tkj5qw2k56wi30oupnb3yct4m	kye5pyfqoq5doipzenotptllr2g6mxqg	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz,kheqge89r0euopogj346fyg25m8t3rya,kye5pyfqoq5doipzenotptllr2g6mxqg	环保班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
121	eleqfwc2744a9ozwn5ndpayuuw7c4zqy	lvyrsk0fjk7br36yh0ti1fug3c8oewsz	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz	物流作业区	3	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
122	llxllcao3wked14skgm1lb5i7p11a5jt	eleqfwc2744a9ozwn5ndpayuuw7c4zqy	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz,eleqfwc2744a9ozwn5ndpayuuw7c4zqy	运输队	4	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
123	cyrj194puwursqce38vm471isql0qc4j	eleqfwc2744a9ozwn5ndpayuuw7c4zqy	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,lvyrsk0fjk7br36yh0ti1fug3c8oewsz,eleqfwc2744a9ozwn5ndpayuuw7c4zqy	维修队	4	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
124	m2hu15ojpus1layz4k4zhifi9cj8erbt	dqr6r6e6exwggi02v2h0tqkbdd46c3sb	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb	第二分公司	2	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
125	b52vpvw2wompdleclbp5467pkw017oab	m2hu15ojpus1layz4k4zhifi9cj8erbt	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,m2hu15ojpus1layz4k4zhifi9cj8erbt	第三作业区	3	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
126	gb2wz5r8etns12cgiu4pi627n2sqqlfw	b52vpvw2wompdleclbp5467pkw017oab	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,m2hu15ojpus1layz4k4zhifi9cj8erbt,b52vpvw2wompdleclbp5467pkw017oab	4号工区	4	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
127	tm1yja3iahb1f4v3bhc7q7amvcu9crhk	gb2wz5r8etns12cgiu4pi627n2sqqlfw	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,m2hu15ojpus1layz4k4zhifi9cj8erbt,b52vpvw2wompdleclbp5467pkw017oab,gb2wz5r8etns12cgiu4pi627n2sqqlfw	甲班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
128	tekf85gn813frzxhmyy3yaj6j0x4e1fo	gb2wz5r8etns12cgiu4pi627n2sqqlfw	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,m2hu15ojpus1layz4k4zhifi9cj8erbt,b52vpvw2wompdleclbp5467pkw017oab,gb2wz5r8etns12cgiu4pi627n2sqqlfw	乙班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
129	n59qw5rkvekax5p3m1ahoxmf1htfy42k	b52vpvw2wompdleclbp5467pkw017oab	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,m2hu15ojpus1layz4k4zhifi9cj8erbt,b52vpvw2wompdleclbp5467pkw017oab	5号工区	4	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
130	hwjzw8gpww9l4eqrvgfr729lk2o3lvzf	n59qw5rkvekax5p3m1ahoxmf1htfy42k	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,m2hu15ojpus1layz4k4zhifi9cj8erbt,b52vpvw2wompdleclbp5467pkw017oab,n59qw5rkvekax5p3m1ahoxmf1htfy42k	甲班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
131	oouqcfyiwgzfx9m1lcm6fx21ef3jzhkl	m2hu15ojpus1layz4k4zhifi9cj8erbt	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,m2hu15ojpus1layz4k4zhifi9cj8erbt	第四作业区	3	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
132	jxtny36vgipqmo8dorsquvfonx2bitdm	oouqcfyiwgzfx9m1lcm6fx21ef3jzhkl	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,m2hu15ojpus1layz4k4zhifi9cj8erbt,oouqcfyiwgzfx9m1lcm6fx21ef3jzhkl	6号工区	4	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
133	p993upszgt1evrgcpxv7f910ixu2sbgs	jxtny36vgipqmo8dorsquvfonx2bitdm	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,m2hu15ojpus1layz4k4zhifi9cj8erbt,oouqcfyiwgzfx9m1lcm6fx21ef3jzhkl,jxtny36vgipqmo8dorsquvfonx2bitdm	甲班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
134	azohlyb3og2beqrj9kpn96sakc44dadf	jxtny36vgipqmo8dorsquvfonx2bitdm	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,m2hu15ojpus1layz4k4zhifi9cj8erbt,oouqcfyiwgzfx9m1lcm6fx21ef3jzhkl,jxtny36vgipqmo8dorsquvfonx2bitdm	乙班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
135	aeqjdjfs1wyl9p93u8bayg352ztdm7ya	jxtny36vgipqmo8dorsquvfonx2bitdm	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,m2hu15ojpus1layz4k4zhifi9cj8erbt,oouqcfyiwgzfx9m1lcm6fx21ef3jzhkl,jxtny36vgipqmo8dorsquvfonx2bitdm	丙班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
136	zclzz8og8qqe6kebyzszab5g2ufrigzm	dqr6r6e6exwggi02v2h0tqkbdd46c3sb	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb	第三分公司	2	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
137	kdipud5mow01mfleoyegyrf9nzlh0omp	zclzz8og8qqe6kebyzszab5g2ufrigzm	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,zclzz8og8qqe6kebyzszab5g2ufrigzm	运维作业区	3	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
138	c4psurjud21mvvvpmuelaf1rt7gfjnno	kdipud5mow01mfleoyegyrf9nzlh0omp	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,zclzz8og8qqe6kebyzszab5g2ufrigzm,kdipud5mow01mfleoyegyrf9nzlh0omp	B1号站	4	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
139	lhq17wpyf7oh04funia9ki4t2tblbq6r	c4psurjud21mvvvpmuelaf1rt7gfjnno	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,zclzz8og8qqe6kebyzszab5g2ufrigzm,kdipud5mow01mfleoyegyrf9nzlh0omp,c4psurjud21mvvvpmuelaf1rt7gfjnno	甲班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
140	pw2cp1u6l37m0h9vrnlgx8aq4ddnelxi	c4psurjud21mvvvpmuelaf1rt7gfjnno	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,zclzz8og8qqe6kebyzszab5g2ufrigzm,kdipud5mow01mfleoyegyrf9nzlh0omp,c4psurjud21mvvvpmuelaf1rt7gfjnno	乙班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
141	bgnoctfgtiezihbpp2xjmzh55134eykr	zclzz8og8qqe6kebyzszab5g2ufrigzm	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,zclzz8og8qqe6kebyzszab5g2ufrigzm	综合作业区	3	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
142	jqtjxsygdgv7btkrb6sfau7fp8coeqgz	bgnoctfgtiezihbpp2xjmzh55134eykr	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,zclzz8og8qqe6kebyzszab5g2ufrigzm,bgnoctfgtiezihbpp2xjmzh55134eykr	A1号站	4	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
143	megotp94fof0d26f3g6zj3xg2lgsuwut	jqtjxsygdgv7btkrb6sfau7fp8coeqgz	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,zclzz8og8qqe6kebyzszab5g2ufrigzm,bgnoctfgtiezihbpp2xjmzh55134eykr,jqtjxsygdgv7btkrb6sfau7fp8coeqgz	甲班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
144	r7haexv1ewuo2e40uh4tnx5zxxmqnfns	jqtjxsygdgv7btkrb6sfau7fp8coeqgz	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,zclzz8og8qqe6kebyzszab5g2ufrigzm,bgnoctfgtiezihbpp2xjmzh55134eykr,jqtjxsygdgv7btkrb6sfau7fp8coeqgz	乙班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
145	u1ai4dx2jxsimrdtb0q22n1p6ie71z8b	jqtjxsygdgv7btkrb6sfau7fp8coeqgz	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,zclzz8og8qqe6kebyzszab5g2ufrigzm,bgnoctfgtiezihbpp2xjmzh55134eykr,jqtjxsygdgv7btkrb6sfau7fp8coeqgz	丙班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
146	in2z18l0hxz17jljs3h3qckjtq7q1kti	bgnoctfgtiezihbpp2xjmzh55134eykr	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,zclzz8og8qqe6kebyzszab5g2ufrigzm,bgnoctfgtiezihbpp2xjmzh55134eykr	A2号站	4	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
147	r650hfoqzv599kagklh6f836tto2rbbw	in2z18l0hxz17jljs3h3qckjtq7q1kti	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,zclzz8og8qqe6kebyzszab5g2ufrigzm,bgnoctfgtiezihbpp2xjmzh55134eykr,in2z18l0hxz17jljs3h3qckjtq7q1kti	甲班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
148	gj2z499cescrl02ierj1r3hhwzqwlqjo	dqr6r6e6exwggi02v2h0tqkbdd46c3sb	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb	重油开发公司	2	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
149	w5uych9iovhs7ytww2xyni9a3gvn9f9p	gj2z499cescrl02ierj1r3hhwzqwlqjo	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,gj2z499cescrl02ierj1r3hhwzqwlqjo	稠油作业区	3	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
150	r836v7gwal4mjozdm21vu2vhvp5tjoax	w5uych9iovhs7ytww2xyni9a3gvn9f9p	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,gj2z499cescrl02ierj1r3hhwzqwlqjo,w5uych9iovhs7ytww2xyni9a3gvn9f9p	SAGD1号站	4	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
151	kzd9e9wccd2d1fyonp6ucvani848uqjq	r836v7gwal4mjozdm21vu2vhvp5tjoax	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,gj2z499cescrl02ierj1r3hhwzqwlqjo,w5uych9iovhs7ytww2xyni9a3gvn9f9p,r836v7gwal4mjozdm21vu2vhvp5tjoax	甲班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
152	i1qwazesxbz321eroqn5o0y0i6ri5ucq	r836v7gwal4mjozdm21vu2vhvp5tjoax	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,gj2z499cescrl02ierj1r3hhwzqwlqjo,w5uych9iovhs7ytww2xyni9a3gvn9f9p,r836v7gwal4mjozdm21vu2vhvp5tjoax	乙班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
153	huo1n8eyycgqr41cqhjoq3k3f5t4iapq	gj2z499cescrl02ierj1r3hhwzqwlqjo	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,gj2z499cescrl02ierj1r3hhwzqwlqjo	SAGD作业区	3	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
154	mot73avfyez5gquc16nke4g4j6zmwrru	huo1n8eyycgqr41cqhjoq3k3f5t4iapq	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,gj2z499cescrl02ierj1r3hhwzqwlqjo,huo1n8eyycgqr41cqhjoq3k3f5t4iapq	SAGD2号站	4	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
155	xdt0wohr8eddgcpxfenx48iekck2rlyn	mot73avfyez5gquc16nke4g4j6zmwrru	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,gj2z499cescrl02ierj1r3hhwzqwlqjo,huo1n8eyycgqr41cqhjoq3k3f5t4iapq,mot73avfyez5gquc16nke4g4j6zmwrru	甲班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
156	vutmnr0p3uj6cwntn6x0fdypeckea2dp	mot73avfyez5gquc16nke4g4j6zmwrru	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,gj2z499cescrl02ierj1r3hhwzqwlqjo,huo1n8eyycgqr41cqhjoq3k3f5t4iapq,mot73avfyez5gquc16nke4g4j6zmwrru	乙班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
157	y8ckjvpi1fzt332kbs779wdylwwf919d	dqr6r6e6exwggi02v2h0tqkbdd46c3sb	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb	工程技术事业部	2	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
158	yyxgogstqncjk8aw1bka8ww8ql78t4oy	y8ckjvpi1fzt332kbs779wdylwwf919d	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,y8ckjvpi1fzt332kbs779wdylwwf919d	钻井工程大队	3	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
159	aazaaxi3fj8ioshkfzfwd52wqn2bdvaq	yyxgogstqncjk8aw1bka8ww8ql78t4oy	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,y8ckjvpi1fzt332kbs779wdylwwf919d,yyxgogstqncjk8aw1bka8ww8ql78t4oy	钻井一队	4	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
160	rtcw7nc9vixl5y6u38ejdk6sisbngq4j	aazaaxi3fj8ioshkfzfwd52wqn2bdvaq	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,y8ckjvpi1fzt332kbs779wdylwwf919d,yyxgogstqncjk8aw1bka8ww8ql78t4oy,aazaaxi3fj8ioshkfzfwd52wqn2bdvaq	钻井甲班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
161	td7m81oe400y1y047drts97ljd1p9gdo	aazaaxi3fj8ioshkfzfwd52wqn2bdvaq	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,y8ckjvpi1fzt332kbs779wdylwwf919d,yyxgogstqncjk8aw1bka8ww8ql78t4oy,aazaaxi3fj8ioshkfzfwd52wqn2bdvaq	钻井乙班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
162	kzru6bn2yy8rmbss3l16blzo04uzf3qc	yyxgogstqncjk8aw1bka8ww8ql78t4oy	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,y8ckjvpi1fzt332kbs779wdylwwf919d,yyxgogstqncjk8aw1bka8ww8ql78t4oy	钻井二队	4	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
163	ec0d5a12u547oegklqcfbx1ajrmhnhna	kzru6bn2yy8rmbss3l16blzo04uzf3qc	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,y8ckjvpi1fzt332kbs779wdylwwf919d,yyxgogstqncjk8aw1bka8ww8ql78t4oy,kzru6bn2yy8rmbss3l16blzo04uzf3qc	钻井甲班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
164	m12ho3wliz43udlq3dhwmc6nu3wz9arh	y8ckjvpi1fzt332kbs779wdylwwf919d	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,y8ckjvpi1fzt332kbs779wdylwwf919d	井下作业大队	3	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
165	ejt8axdoxjkpq2er6gk12yxfp6qsyvyh	m12ho3wliz43udlq3dhwmc6nu3wz9arh	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,y8ckjvpi1fzt332kbs779wdylwwf919d,m12ho3wliz43udlq3dhwmc6nu3wz9arh	修井队	4	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
166	sj8j1hw1likzmdbk4etbrqk5t6pnylle	ejt8axdoxjkpq2er6gk12yxfp6qsyvyh	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,y8ckjvpi1fzt332kbs779wdylwwf919d,m12ho3wliz43udlq3dhwmc6nu3wz9arh,ejt8axdoxjkpq2er6gk12yxfp6qsyvyh	甲班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
167	vmxjerxj1mbh16efg08wykyqsw2ankvp	ejt8axdoxjkpq2er6gk12yxfp6qsyvyh	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,y8ckjvpi1fzt332kbs779wdylwwf919d,m12ho3wliz43udlq3dhwmc6nu3wz9arh,ejt8axdoxjkpq2er6gk12yxfp6qsyvyh	乙班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
168	cqu8wn24karw76b2d2kqluxoniq77mnh	m12ho3wliz43udlq3dhwmc6nu3wz9arh	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,y8ckjvpi1fzt332kbs779wdylwwf919d,m12ho3wliz43udlq3dhwmc6nu3wz9arh	试油队	4	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
169	g1crjufoco9y6ylxn4tgaq5l13mn0isz	cqu8wn24karw76b2d2kqluxoniq77mnh	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,y8ckjvpi1fzt332kbs779wdylwwf919d,m12ho3wliz43udlq3dhwmc6nu3wz9arh,cqu8wn24karw76b2d2kqluxoniq77mnh	甲班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
170	d3etjyk6js9jy8c0thmloppmoabyyxli	cqu8wn24karw76b2d2kqluxoniq77mnh	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,y8ckjvpi1fzt332kbs779wdylwwf919d,m12ho3wliz43udlq3dhwmc6nu3wz9arh,cqu8wn24karw76b2d2kqluxoniq77mnh	乙班	5	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
171	q706ie5vt4qhfp6kl55fhwa7619x3ljj	y8ckjvpi1fzt332kbs779wdylwwf919d	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,y8ckjvpi1fzt332kbs779wdylwwf919d	地面工程大队	3	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
172	enc3p7e6phadqkpaoukpxn3qxaeojfou	q706ie5vt4qhfp6kl55fhwa7619x3ljj	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,y8ckjvpi1fzt332kbs779wdylwwf919d,q706ie5vt4qhfp6kl55fhwa7619x3ljj	土建队	4	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
173	r2ss4wkobo5w325niausqmror2ny4vpb	q706ie5vt4qhfp6kl55fhwa7619x3ljj	0,dqr6r6e6exwggi02v2h0tqkbdd46c3sb,y8ckjvpi1fzt332kbs779wdylwwf919d,q706ie5vt4qhfp6kl55fhwa7619x3ljj	安装队	4	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-07-04 00:00:00	2026-07-04 00:00:00	\N		\N	\N	\N	\N	\N		0
\.


--
-- Data for Name: aa_emp_set; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_emp_set (id, set_id, name, remark, emp_nos, created_by, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_login_log; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_login_log (id, info_id, login_name, ipaddr, login_location, browser, os, status, msg, login_time, module) FROM stdin;
\.


--
-- Data for Name: aa_menu; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_menu (id, menu_id, pid, name, title, en_name, icon, condition, remark, menu_type, weigh, is_hide, path, component, is_link, module_type, model_id, is_iframe, is_cached, redirect, is_affix, link_url, grant_rules, created_at, updated_at) FROM stdin;
1	jeo9mvaihkkm4hs0mrhlpqg2nxuvvgqr	0	api/v1/system	系统管理	System Management	⚙️			0	54	0	/system	layout/routerView/parent	0		0	0	1	0	0		[]	2022-03-24 15:03:37	2022-04-14 16:29:19
2	t5w3n2zlz7hssio12g960kmw9mb0q6gp	jeo9mvaihkkm4hs0mrhlpqg2nxuvvgqr	api/v1/system/menuList	菜单管理	Menu Management	📋			1	0	0	/system/menu/list	system/menu/index	0		0	0	1		0		[]	2022-03-24 17:24:13	2022-03-29 10:54:49
3	jnqs6sca03fhu76y78j6w1oncy6en7kr	t5w3n2zlz7hssio12g960kmw9mb0q6gp	api/v1/system/menu/add	添加菜单	Add Menu				2	0	0			0		0	0	1		0		[]	2022-03-29 16:48:43	2022-03-29 17:05:19
4	j7endjy4gs1co0opax9vv58h7dfxhddk	t5w3n2zlz7hssio12g960kmw9mb0q6gp	api/v1/system/menu/update	修改菜单	Edit Menu				2	0	0			0		0	0	1		0		[]	2022-03-29 17:04:25	2022-03-29 18:11:36
10	uuj4xhr6fi4mg0t0fhd9a15dboklgk9z	jeo9mvaihkkm4hs0mrhlpqg2nxuvvgqr	api/v1/system/role/list	角色管理	Role Management	👥			1	0	0	/system/role/list	system/role/index	0		0	0	1		0		[]	2022-03-29 18:15:03	2022-03-30 10:25:34
11	c999jlcifgwwk4be7cpak9vohng8suoo	t5w3n2zlz7hssio12g960kmw9mb0q6gp	api/v1/system/menu/delete	删除菜单	Delete Menu				2	0	0			0		0	0	1		0		[]	2022-04-06 14:49:10	2022-04-06 14:49:17
12	q4n01yci6swfnrjt1azphn6z6wwnnbco	uuj4xhr6fi4mg0t0fhd9a15dboklgk9z	api/v1/system/role/add	添加角色	Add Role				2	0	0			0		0	0	1		0		[]	2022-04-06 14:49:46	2022-04-06 14:49:46
13	oko5fv9off4dr4o4u3g9gxlvfh8780au	uuj4xhr6fi4mg0t0fhd9a15dboklgk9z	/api/v1/system/role/edit	修改角色	Edit Role				2	0	0			0		0	0	1		0		[]	2022-04-06 14:50:08	2022-04-06 14:50:08
14	ywtex3edy9d0gfr5iyo9fmmvhgb7yz4r	uuj4xhr6fi4mg0t0fhd9a15dboklgk9z	/api/v1/system/role/delete	删除角色	Delete Role				2	0	0			0		0	0	1		0		[]	2022-04-06 14:50:22	2022-04-06 14:50:22
15	xr83ya4kyxlnb3t1163puyimzs0awq1v	jeo9mvaihkkm4hs0mrhlpqg2nxuvvgqr	api/v1/system/dept/list	部门管理	Department	🏢			1	0	0	/system/dept/list	system/dept/index	0		0	0	1		0		[]	2022-04-06 14:52:23	2022-04-07 22:59:20
19	vyzbuuux5wo6poqapcoet020d0efvtco	xr83ya4kyxlnb3t1163puyimzs0awq1v	api/v1/system/dept/add	添加部门	Add Department				2	0	0			0		0	0	1		0		[]	2022-04-07 22:56:39	2022-04-07 22:56:39
20	hi4iu377mcuv3xoj48uh9bnq5tycn5ar	xr83ya4kyxlnb3t1163puyimzs0awq1v	api/v1/system/dept/edit	修改部门	Edit Department				2	0	0			0		0	0	1		0		[]	2022-04-07 22:57:00	2022-04-07 22:57:00
21	tqpajoh8ufqacz13dijgnsq8fyssedgb	xr83ya4kyxlnb3t1163puyimzs0awq1v	api/v1/system/dept/delete	删除部门	Delete Department				2	0	0			0		0	0	1		0		[]	2022-04-07 22:57:30	2022-04-07 22:57:30
22	mrcmjf6214rehygpt9w92v3uqcbaljea	jeo9mvaihkkm4hs0mrhlpqg2nxuvvgqr	api/v1/system/post/list	岗位管理	Position	💼			1	0	0	/system/post/list	system/post/index	0		0	0	1		0		[]	2022-04-07 22:58:46	2022-04-09 14:26:15
23	djihplrjenn31sqbebda8b6h0wqtey1w	mrcmjf6214rehygpt9w92v3uqcbaljea	api/v1/system/post/add	添加岗位	Add Position				2	0	0			0		0	0	1		0		[]	2022-04-09 14:14:49	2022-04-09 14:14:49
24	oi4mubhpz95iudjrjiisaruh9i2imdtp	mrcmjf6214rehygpt9w92v3uqcbaljea	api/v1/system/post/edit	修改岗位	Edit Position				2	0	0			0		0	0	1		0		[]	2022-04-09 14:15:25	2022-04-09 14:15:25
25	nxcf3qsruwghnrw9yxjvxx07p0ujhz0r	mrcmjf6214rehygpt9w92v3uqcbaljea	api/v1/system/post/delete	删除岗位	Delete Position				2	0	0			0		0	0	1		0		[]	2022-04-09 14:15:47	2022-04-09 14:15:47
26	sgfi0ulrhgjur9jw1z2wnr1nmfv7gsba	jeo9mvaihkkm4hs0mrhlpqg2nxuvvgqr	api/v1/system/user/list	员工管理	Employee Management	👤			1	0	0	/system/user/list	system/user/index	0		0	0	1		0		[]	2022-04-09 14:19:10	2022-04-09 14:19:58
31	dro7ldd3tizp78z3ekepx74b6820h0nw	0	api/v1/system/monitor	系统监控	System Monitor	📊			0	30	0	/system/monitor	layout/routerView/parent	0		0	0	1		0		[]	2022-04-19 10:40:19	2022-04-19 10:44:38
32	dgk3u4jbh0idyt533flqch7ix9oqssem	dro7ldd3tizp78z3ekepx74b6820h0nw	api/v1/system/monitor/server	服务监控	Server Monitor	🖥️			1	0	0	/system/monitor/server	system/monitor/server/index	0		0	0	1		0		[]	2022-04-19 10:43:32	2022-04-19 10:44:47
34	qh93mn1hmam93p21aoxgpzw5g5cr73xk	dro7ldd3tizp78z3ekepx74b6820h0nw	api/v1/system/loginLog/list	登录日志	Login Log	📝			1	0	0	/system/monitor/loginLog	system/monitor/loginLog/index	0		0	0	1		0		[]	2022-04-28 09:59:47	2022-04-28 09:59:47
38	brh3lmp6u0p6awn3qqz6k3sw1po98r3w	dro7ldd3tizp78z3ekepx74b6820h0nw	api/v1/system/operLog/list	操作日志	Operation Log	📜			1	0	0	/system/monitor/operLog	system/monitor/operLog/index	0		0	0	1		0		[]	2022-12-23 16:19:05	2022-12-23 16:21:50
39	fqrr2132fcbq7w03t7sbqg4i47tbnmst	dro7ldd3tizp78z3ekepx74b6820h0nw	api/v1/system/online/list	在线用户	Online Users	🟢			1	0	0	/system/monitor/userOnlie	system/monitor/userOnline/index	0		0	0	1		0		[]	2023-01-11 15:48:06	2023-01-11 17:02:39
537	e1a1a1a1a1a1a1a1a1a1a1a1a1a319	e1a1a1a1a1a1a1a1a1a1a1a1a1a311	api/v1/agent/instance/mcp/list	MCP分组列表	MCP Groups				2	0	0			0		0	0	1		0		[]	2026-08-30 00:00:00	2026-08-30 00:00:00
538	e1a1a1a1a1a1a1a1a1a1a1a1a1a320	e1a1a1a1a1a1a1a1a1a1a1a1a1a311	api/v1/agent/instance/mcp/bind	绑定MCP分组	Bind MCP Group				2	0	0			0		0	0	1		0		[]	2026-08-30 00:00:00	2026-08-30 00:00:00
539	e1a1a1a1a1a1a1a1a1a1a1a1a1a321	e1a1a1a1a1a1a1a1a1a1a1a1a1a311	api/v1/agent/instance/mcp/unbind	解绑MCP分组	Unbind MCP Group				2	0	0			0		0	0	1		0		[]	2026-08-30 00:00:00	2026-08-30 00:00:00
540	clientmenudevmcplist0000000000a	clientmenudevice00000000000000a	api/v1/system/client/admin/device/mcp	MCP分组列表	MCP Groups				2	0	0			0		0	0	1		0		[]	2026-08-30 00:00:00	2026-08-30 00:00:00
541	clientmenudevmcpbind0000000000a	clientmenudevice00000000000000a	api/v1/system/client/admin/device/mcp/bind	绑定MCP分组	Bind MCP Group				2	0	0			0		0	0	1		0		[]	2026-08-30 00:00:00	2026-08-30 00:00:00
542	clientmenudevmcpunbind00000000a	clientmenudevice00000000000000a	api/v1/system/client/admin/device/mcp/unbind	解绑MCP分组	Unbind MCP Group				2	0	0			0		0	0	1		0		[]	2026-08-30 00:00:00	2026-08-30 00:00:00
574	statsmenudir0000000000000000000a	0	api/v1/stats	数据看板	Data Dashboard	📈			0	23	0	/stats	layout/routerView/parent	0		0	0	1	0	0		[]	2026-09-01 00:00:00	2026-09-01 00:00:00
575	statsllmdatamenu000000000000000a	statsmenudir0000000000000000000a	api/v1/stats/llmData	LLM数据	LLM Data				1	0	0	/stats/llm	stats/llm/index	0		0	0	1		0		[]	2026-09-01 00:00:00	2026-09-01 00:00:00
576	statsskilldatamenu0000000000000a	statsmenudir0000000000000000000a	api/v1/stats/skillData	SKILL数据	Skill Data				1	0	0	/stats/skill	stats/skill/index	0		0	0	1		0		[]	2026-09-01 00:00:00	2026-09-01 00:00:00
577	reviewmenudir000000000000000000a	0	api/v1/review	审核系统	Review System	🛡️			0	22	0	/review	layout/routerView/parent	0		0	0	1	0	0		[]	2026-09-01 00:00:00	2026-09-01 00:00:00
606	docmenumyspace00000000000000000a	docmenudir000000000000000000000a	api/v1/doc/myspace	我的空间	My Space	👤			1	0	0	/doc/myspace	system/doc/index	0		0	0	1		0		[]	2026-09-07 00:00:00	2026-09-07 00:00:00
526	clientmenuappperm00000000000000a	clientmenutopdir00000000000000a	api/v1/system/client/admin/app/list	App权限管理	App Permissions	🔑			1	0	0	/client/permission	system/client/appperm/index	0		0	0	1		0		[]	2026-08-18 00:00:00	2026-08-18 00:00:00
527	clientmenuapppermsave0000000000a	clientmenuappperm00000000000000a	api/v1/system/client/admin/appperm/save	保存App权限	Save Permissions				2	0	0			0		0	0	1		0		[]	2026-08-18 00:00:00	2026-08-18 00:00:00
528	clientmenuapppermappsave0000000a	clientmenuappperm00000000000000a	api/v1/system/client/admin/app/save	保存App	Save App				2	0	0			0		0	0	1		0		[]	2026-08-18 00:00:00	2026-08-18 00:00:00
529	clientmenuapppermappdel00000000a	clientmenuappperm00000000000000a	api/v1/system/client/admin/app/delete	删除App	Delete App				2	0	0			0		0	0	1		0		[]	2026-08-18 00:00:00	2026-08-18 00:00:00
543	mcpmenuserveralldata00000000000a	f4fd9493307ce4018c0e42234cdb9d5d	api/v1/mcp/server/all	服务器全量	All Servers				2	0	0			0		0	0	1		0		[]	2026-08-30 00:00:00	2026-08-30 00:00:00
544	mcpmenugroupalldata000000000000a	ca4b51d526379dfa58f131bcbe8a504b	api/v1/mcp/group/all	分组全量	All Groups				2	0	0			0		0	0	1		0		[]	2026-08-30 00:00:00	2026-08-30 00:00:00
580	pgmsetresolve000000000000000000a	pgmenumain000000000000000000000a	api/v1/system/permgroup/set/resolve	解析集合成员	Resolve Members				2	0	0			0		0	0	1		0		[]	2026-09-01 00:00:00	2026-09-01 00:00:00
607	schedmenudir0000000000000000000a	0	api/v1/schedule	日程	Schedule	📅			0	33	0	/schedule	layout/routerView/parent	0		0	0	1	0	0		[]	2026-09-11 00:00:00	2026-09-11 00:00:00
608	schedmenumy00000000000000000000a	schedmenudir0000000000000000000a	api/v1/schedule/event/list	我的日程	My Schedule	🗓️			1	0	0	/schedule/my	schedule/my/index	0		0	0	1		0		[]	2026-09-11 00:00:00	2026-09-11 00:00:00
609	schedmenuroom000000000000000000a	schedmenudir0000000000000000000a	api/v1/schedule/room/overview	会议室	Meeting Rooms	🏢			1	0	0	/schedule/room	schedule/room/index	0		0	0	1		0		[]	2026-09-11 00:00:00	2026-09-11 00:00:00
610	schedmenuroommanage000000000000a	schedmenudir0000000000000000000a	api/v1/schedule/room/manage	会议室管理	Room Management	🛠️			1	0	0	/schedule/room-manage	schedule/room-manage/index	0		0	0	1		0		[]	2026-09-11 00:00:00	2026-09-11 00:00:00
611	schedmenueventadd00000000000000a	schedmenumy00000000000000000000a	api/v1/schedule/event/add	新建日程	Add Event				2	0	0			0		0	0	1		0		[]	2026-09-11 00:00:00	2026-09-11 00:00:00
612	schedmenueventedit0000000000000a	schedmenumy00000000000000000000a	api/v1/schedule/event/edit	修改日程	Edit Event				2	0	0			0		0	0	1		0		[]	2026-09-11 00:00:00	2026-09-11 00:00:00
613	schedmenueventdel00000000000000a	schedmenumy00000000000000000000a	api/v1/schedule/event/delete	取消日程	Cancel Event				2	0	0			0		0	0	1		0		[]	2026-09-11 00:00:00	2026-09-11 00:00:00
614	schedmenueventexc00000000000000a	schedmenumy00000000000000000000a	api/v1/schedule/event/exception	单场调整	Adjust Occurrence				2	0	0			0		0	0	1		0		[]	2026-09-11 00:00:00	2026-09-11 00:00:00
615	schedmenurespond000000000000000a	schedmenumy00000000000000000000a	api/v1/schedule/attendee/respond	参会回执	Respond Invite				2	0	0			0		0	0	1		0		[]	2026-09-11 00:00:00	2026-09-11 00:00:00
616	schedmenuroomadd000000000000000a	schedmenuroommanage000000000000a	api/v1/schedule/room/add	添加会议室	Add Room				2	0	0			0		0	0	1		0		[]	2026-09-11 00:00:00	2026-09-11 00:00:00
617	schedmenuroomedit00000000000000a	schedmenuroommanage000000000000a	api/v1/schedule/room/edit	修改会议室	Edit Room				2	0	0			0		0	0	1		0		[]	2026-09-11 00:00:00	2026-09-11 00:00:00
618	schedmenuroomdel000000000000000a	schedmenuroommanage000000000000a	api/v1/schedule/room/delete	删除会议室	Delete Room				2	0	0			0		0	0	1		0		[]	2026-09-11 00:00:00	2026-09-11 00:00:00
530	clientmenuapppermreset000000000a	clientmenuappperm00000000000000a	api/v1/system/client/admin/appperm/reset	重置App权限	Reset Permissions				2	0	0			0		0	0	1		0		[]	2026-08-20 00:00:00	2026-08-20 00:00:00
545	docmenudir000000000000000000000a	0	api/v1/doc	文档管理	Document Management	📄			0	24	0	/doc	layout/routerView/parent	0		0	0	1	0	0		[]	2026-08-30 00:00:00	2026-08-30 00:00:00
546	docmenulist00000000000000000000a	docmenudir000000000000000000000a	api/v1/doc/document/list	文档管理	Documents	📑			1	0	0	/doc/document/list	system/doc/index	0		0	0	1		0		[]	2026-08-30 00:00:00	2026-08-30 00:00:00
547	docmenuupload000000000000000000a	docmenulist00000000000000000000a	api/v1/doc/document/upload	上传文档	Upload Document				2	0	0			0		0	0	1		0		[]	2026-08-30 00:00:00	2026-08-30 00:00:00
548	docmenuupdate000000000000000000a	docmenulist00000000000000000000a	api/v1/doc/document/update	更新文档	Update Document				2	0	0			0		0	0	1		0		[]	2026-08-30 00:00:00	2026-08-30 00:00:00
549	docmenuedit00000000000000000000a	docmenulist00000000000000000000a	api/v1/doc/document/edit	编辑文档	Edit Document				2	0	0			0		0	0	1		0		[]	2026-08-30 00:00:00	2026-08-30 00:00:00
550	docmenudelete000000000000000000a	docmenulist00000000000000000000a	api/v1/doc/document/delete	删除文档	Delete Document				2	0	0			0		0	0	1		0		[]	2026-08-30 00:00:00	2026-08-30 00:00:00
551	docmenuget000000000000000000000a	docmenulist00000000000000000000a	api/v1/doc/document/get	查看文档	View Document				2	0	0			0		0	0	1		0		[]	2026-08-30 00:00:00	2026-08-30 00:00:00
552	docmenucontent00000000000000000a	docmenulist00000000000000000000a	api/v1/doc/document/content	查看全文	View Content				2	0	0			0		0	0	1		0		[]	2026-08-30 00:00:00	2026-08-30 00:00:00
553	docmenusearch000000000000000000a	docmenulist00000000000000000000a	api/v1/doc/document/search	语义检索	Semantic Search				2	0	0			0		0	0	1		0		[]	2026-08-30 00:00:00	2026-08-30 00:00:00
581	kbmenudir000000000000000000000a	0	api/v1/kb	知识库管理	Knowledge Base Management	📚			0	24	0	/kb	layout/routerView/parent	0		0	0	1	0	0		[]	2026-09-02 00:00:00	2026-09-02 00:00:00
582	kbmenulist00000000000000000000a	kbmenudir000000000000000000000a	api/v1/kb/document/list	知识库管理	Knowledge Base	📖			1	0	0	/kb/document/list	system/kb/index	0		0	0	1		0		[]	2026-09-02 00:00:00	2026-09-02 00:00:00
583	kbmenuupload000000000000000000a	kbmenulist00000000000000000000a	api/v1/kb/document/upload	上传文档	Upload Document				2	0	0			0		0	0	1		0		[]	2026-09-02 00:00:00	2026-09-02 00:00:00
584	kbmenuupdate000000000000000000a	kbmenulist00000000000000000000a	api/v1/kb/document/update	更新文档	Update Document				2	0	0			0		0	0	1		0		[]	2026-09-02 00:00:00	2026-09-02 00:00:00
585	kbmenuedit00000000000000000000a	kbmenulist00000000000000000000a	api/v1/kb/document/edit	编辑文档	Edit Document				2	0	0			0		0	0	1		0		[]	2026-09-02 00:00:00	2026-09-02 00:00:00
586	kbmenudelete000000000000000000a	kbmenulist00000000000000000000a	api/v1/kb/document/delete	删除文档	Delete Document				2	0	0			0		0	0	1		0		[]	2026-09-02 00:00:00	2026-09-02 00:00:00
587	kbmenuget000000000000000000000a	kbmenulist00000000000000000000a	api/v1/kb/document/get	查看文档	View Document				2	0	0			0		0	0	1		0		[]	2026-09-02 00:00:00	2026-09-02 00:00:00
588	kbmenucontent00000000000000000a	kbmenulist00000000000000000000a	api/v1/kb/document/content	查看全文	View Content				2	0	0			0		0	0	1		0		[]	2026-09-02 00:00:00	2026-09-02 00:00:00
589	kbmenuraw000000000000000000000a	kbmenulist00000000000000000000a	api/v1/kb/document/raw	预览原件	Preview Raw File				2	0	0			0		0	0	1		0		[]	2026-09-02 00:00:00	2026-09-02 00:00:00
590	kbmenusearch000000000000000000a	kbmenulist00000000000000000000a	api/v1/kb/document/search	语义检索	Semantic Search				2	0	0			0		0	0	1		0		[]	2026-09-02 00:00:00	2026-09-02 00:00:00
591	kbdirlist000000000000000000000a	kbmenulist00000000000000000000a	api/v1/kb/dir/list	目录列表	Directory List				2	0	0			0		0	0	1		0		[]	2026-09-02 00:00:00	2026-09-02 00:00:00
592	kbdircreate0000000000000000000a	kbmenulist00000000000000000000a	api/v1/kb/dir/create	新建目录	Create Directory				2	0	0			0		0	0	1		0		[]	2026-09-02 00:00:00	2026-09-02 00:00:00
593	kbdirrename0000000000000000000a	kbmenulist00000000000000000000a	api/v1/kb/dir/rename	重命名目录	Rename Directory				2	0	0			0		0	0	1		0		[]	2026-09-02 00:00:00	2026-09-02 00:00:00
594	kbdirmove000000000000000000000a	kbmenulist00000000000000000000a	api/v1/kb/dir/move	移动目录	Move Directory				2	0	0			0		0	0	1		0		[]	2026-09-02 00:00:00	2026-09-02 00:00:00
595	kbdirdelete0000000000000000000a	kbmenulist00000000000000000000a	api/v1/kb/dir/delete	删除目录	Delete Directory				2	0	0			0		0	0	1		0		[]	2026-09-02 00:00:00	2026-09-02 00:00:00
596	kbdocmove000000000000000000000a	kbmenulist00000000000000000000a	api/v1/kb/document/move	移动文档	Move Document				2	0	0			0		0	0	1		0		[]	2026-09-02 00:00:00	2026-09-02 00:00:00
597	kbdocembed00000000000000000000a	kbmenulist00000000000000000000a	api/v1/kb/document/embed	重新嵌入	Re-embed				2	0	0			0		0	0	1		0		[]	2026-09-02 00:00:00	2026-09-02 00:00:00
598	kbdocaccess0000000000000000000a	kbmenulist00000000000000000000a	api/v1/kb/document/access	访问记录	Access Log				2	0	0			0		0	0	1		0		[]	2026-09-02 00:00:00	2026-09-02 00:00:00
599	kbgrantget00000000000000000000a	kbmenulist00000000000000000000a	api/v1/kb/grant/get	查授权配置	Get Grants				2	0	0			0		0	0	1		0		[]	2026-09-02 00:00:00	2026-09-02 00:00:00
600	kbgrantsave0000000000000000000a	kbmenulist00000000000000000000a	api/v1/kb/grant/save	保存授权配置	Save Grants				2	0	0			0		0	0	1		0		[]	2026-09-02 00:00:00	2026-09-02 00:00:00
554	docdirlist000000000000000000000a	docmenulist00000000000000000000a	api/v1/doc/dir/list	目录列表	Directory List				2	0	0			0		0	0	1		0		[]	2026-08-31 00:00:00	2026-08-31 00:00:00
555	docdircreate0000000000000000000a	docmenulist00000000000000000000a	api/v1/doc/dir/create	新建目录	Create Directory				2	0	0			0		0	0	1		0		[]	2026-08-31 00:00:00	2026-08-31 00:00:00
556	docdirrename0000000000000000000a	docmenulist00000000000000000000a	api/v1/doc/dir/rename	重命名目录	Rename Directory				2	0	0			0		0	0	1		0		[]	2026-08-31 00:00:00	2026-08-31 00:00:00
557	docdirmove000000000000000000000a	docmenulist00000000000000000000a	api/v1/doc/dir/move	移动目录	Move Directory				2	0	0			0		0	0	1		0		[]	2026-08-31 00:00:00	2026-08-31 00:00:00
558	docdirdelete0000000000000000000a	docmenulist00000000000000000000a	api/v1/doc/dir/delete	删除目录	Delete Directory				2	0	0			0		0	0	1		0		[]	2026-08-31 00:00:00	2026-08-31 00:00:00
559	docdocmove000000000000000000000a	docmenulist00000000000000000000a	api/v1/doc/document/move	移动文档	Move Document				2	0	0			0		0	0	1		0		[]	2026-08-31 00:00:00	2026-08-31 00:00:00
560	docdocembed00000000000000000000a	docmenulist00000000000000000000a	api/v1/doc/document/embed	重新嵌入	Re-embed				2	0	0			0		0	0	1		0		[]	2026-08-31 00:00:00	2026-08-31 00:00:00
561	docdocaccess0000000000000000000a	docmenulist00000000000000000000a	api/v1/doc/document/access	访问记录	Access Log				2	0	0			0		0	0	1		0		[]	2026-08-31 00:00:00	2026-08-31 00:00:00
601	mybudgetmenu0000000000000000000a	45b37c9e9663411570679e7b5170fbbd	api/v1/llm/budget/mine	我的预算	My Budget	💳			1	0	0	/llm/budget/mine	system/llm/mybudget/index	0		0	0	1		0		[]	2026-09-05 00:00:00	2026-09-05 00:00:00
532	clientmenuapppermappsuggest0000a	clientmenuappperm00000000000000a	api/v1/system/client/admin/app/suggest	联想App	Suggest App				2	0	0			0		0	0	1		0		[]	2026-08-25 00:00:00	2026-08-25 00:00:00
562	docmenuraw000000000000000000000a	docmenulist00000000000000000000a	api/v1/doc/document/raw	预览原件	Preview Raw File				2	0	0			0		0	0	1		0		[]	2026-08-30 00:00:00	2026-08-30 00:00:00
533	clientmenudevskillinst00000000a	clientmenudevice00000000000000a	api/v1/system/client/admin/device/skill/install	安装技能	Install Skill				2	0	0			0		0	0	1		0		[]	2026-08-27 00:00:00	2026-08-27 00:00:00
534	clientmenudevskilluninst0000000a	clientmenudevice00000000000000a	api/v1/system/client/admin/device/skill/uninstall	卸载技能	Uninstall Skill				2	0	0			0		0	0	1		0		[]	2026-08-27 00:00:00	2026-08-27 00:00:00
563	docgrantget00000000000000000000a	docmenulist00000000000000000000a	api/v1/doc/grant/get	查授权配置	Get Grants				2	0	0			0		0	0	1		0		[]	2026-09-01 00:00:00	2026-09-01 00:00:00
564	docgrantsave0000000000000000000a	docmenulist00000000000000000000a	api/v1/doc/grant/save	保存授权配置	Save Grants				2	0	0			0		0	0	1		0		[]	2026-09-01 00:00:00	2026-09-01 00:00:00
67	mk7t9p2xq5r8w1s4n6y0zj2dl8scba01	hu789rpqoi29g6tcgjuut0w9cs7zvccd	api/v1/skill/market/site/all	第三方市场	Skill Market	🛒			1	0	0	/skill/market	system/skill/market/index	0		0	0	1		0		[]	2026-07-29 00:00:00	2026-09-17 05:40:59.868789
603	reviewminemenu00000000000000000a	reviewmenudir000000000000000000a	api/v1/review/mine	我的提交	My Submissions	📤			1	2	0	/review/mine	review/mine/index	0		0	0	1		0		[]	2026-09-06 00:00:00	2026-09-06 00:00:00
604	reviewtodomenu00000000000000000a	reviewmenudir000000000000000000a	api/v1/review/todo	我的审核	My Reviews	✅			1	1	0	/review/todo	review/todo/index	0		0	0	1		0		[]	2026-09-06 00:00:00	2026-09-06 00:00:00
40	hu789rpqoi29g6tcgjuut0w9cs7zvccd	0	api/v1/skill	技能管理	Skill Management	🧩			0	30	0	/skill	layout/routerView/parent	0		0	0	1	0	0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
41	bp7ked9xg7qcr0xrrgds76of4a9mu4ea	hu789rpqoi29g6tcgjuut0w9cs7zvccd	api/v1/skill/skill/list	技能管理	Skill	🧠			1	0	0	/skill/skill/list	system/skill/skill/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
42	e95v8nw4je4akpcymbrhvcxiwrle8gwn	hu789rpqoi29g6tcgjuut0w9cs7zvccd	api/v1/skill/namespace/list	命名空间	Namespace	📦			1	0	0	/skill/namespace/list	system/skill/namespace/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
44	knisg98yampdzasuncvlqizxtn7p1e9w	hu789rpqoi29g6tcgjuut0w9cs7zvccd	api/v1/skill/label/list	标签管理	Label	🏷️			1	0	0	/skill/label/list	system/skill/label/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
45	wcloh1ihir32gyjyh475kileb31wu27q	hu789rpqoi29g6tcgjuut0w9cs7zvccd	api/v1/skill/security/list	安全审计	Security Audit	🔒			1	0	0	/skill/security/list	system/skill/security/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
48	r0fazcz59xrcqn2o9zso7s7lmfn3m9qn	bp7ked9xg7qcr0xrrgds76of4a9mu4ea	api/v1/skill/skill/add	添加技能	Add Skill				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
49	mm8my37lbovupqokiplndn5pcli6q2ru	bp7ked9xg7qcr0xrrgds76of4a9mu4ea	api/v1/skill/skill/edit	修改技能	Edit Skill				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
50	n0ytxdyn6yeqr29fdddddqfmtelkxkfy	bp7ked9xg7qcr0xrrgds76of4a9mu4ea	api/v1/skill/skill/delete	删除技能	Delete Skill				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
51	rq42o9g1tozdqew1yv2xwkkd629hmirc	bp7ked9xg7qcr0xrrgds76of4a9mu4ea	api/v1/skill/skill/publish	发布技能	Publish Skill				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
52	s19vell09rxm8ac5wobq8gp0jbif8s3e	bp7ked9xg7qcr0xrrgds76of4a9mu4ea	api/v1/skill/skill/yank	撤回版本	Yank Version				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
53	ax0los0xmne4irua6rwqljp4e7sqgcnc	e95v8nw4je4akpcymbrhvcxiwrle8gwn	api/v1/skill/namespace/add	添加命名空间	Add Namespace				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
54	svwv5nl78pe01wj1995ydv087aev2sgj	e95v8nw4je4akpcymbrhvcxiwrle8gwn	api/v1/skill/namespace/edit	修改命名空间	Edit Namespace				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
55	qm6p5nkrzth8txnmtmwz9x7mb3mrgkgm	e95v8nw4je4akpcymbrhvcxiwrle8gwn	api/v1/skill/namespace/delete	删除命名空间	Delete Namespace				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
60	hj4xiz4qnky8ie8uo59sbppfklry15du	knisg98yampdzasuncvlqizxtn7p1e9w	api/v1/skill/label/add	添加标签	Add Label				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
61	wbwceu6qsg78htmbqixiqm2kv66ri04v	knisg98yampdzasuncvlqizxtn7p1e9w	api/v1/skill/label/edit	修改标签	Edit Label				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
62	lcj6og6zmrci9uht5vp0xqejcan66gkm	knisg98yampdzasuncvlqizxtn7p1e9w	api/v1/skill/label/delete	删除标签	Delete Label				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
63	rkpal1wh3sl6bta39qqr27526rrjhm6h	wcloh1ihir32gyjyh475kileb31wu27q	api/v1/skill/security/rescan	重新扫描	Rescan				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
70	45b37c9e9663411570679e7b5170fbbd	0	api/v1/llm	LLM管理	LLM Gateway	🤖			0	30	0	/llm	layout/routerView/parent	0		0	0	1	0	0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
71	d0d2845e0c475f24035287b10bef501c	45b37c9e9663411570679e7b5170fbbd	api/v1/llm/credential/list	凭据管理	Credentials	🔑			1	0	0	/llm/credential/list	system/llm/credential/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
72	b77caa93b4383f2eabd6a9d92d6554d7	45b37c9e9663411570679e7b5170fbbd	api/v1/llm/model/list	模型管理	Models	🧠			1	0	0	/llm/model/list	system/llm/model/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
73	7ad5be19e1ff1b2d201d42d20b2b8ec4	45b37c9e9663411570679e7b5170fbbd	api/v1/llm/key/list	密钥管理	Virtual Keys	🗝️			1	0	0	/llm/key/list	system/llm/key/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
74	aebf72188cf62a5ef001cce2f179be97	45b37c9e9663411570679e7b5170fbbd	api/v1/llm/budget/list	预算管理	Budgets	💰			1	0	0	/llm/budget/list	system/llm/budget/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
75	23c7ec868b212bd8e19640efd3cddf0d	45b37c9e9663411570679e7b5170fbbd	api/v1/llm/dept/list	部门配置	LLM Dept	👥			1	0	0	/llm/dept/list	system/llm/dept/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
76	08bd693d553e3f43a444996424d797c0	45b37c9e9663411570679e7b5170fbbd	api/v1/llm/spend/list	用量日志	Spend Logs	📈			1	0	0	/llm/spend/list	system/llm/spend/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
77	3471bff0aed5ac04aa0eef338faf8c53	45b37c9e9663411570679e7b5170fbbd	api/v1/llm/error/list	错误日志	Error Logs	⚠️			1	0	0	/llm/error/list	system/llm/error/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
78	a5764d2120cd8404d7fa4051f83356ff	45b37c9e9663411570679e7b5170fbbd	api/v1/llm/health/list	健康检查	Health	💚			1	0	0	/llm/health/list	system/llm/health/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
79	35179a1f7f7cf422172987c84f4b1d4a	d0d2845e0c475f24035287b10bef501c	api/v1/llm/credential/add	添加凭据	Add Credential				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
80	d76c76816200d71574e2de97750e7222	d0d2845e0c475f24035287b10bef501c	api/v1/llm/credential/edit	修改凭据	Edit Credential				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
81	2779273a4a12bbab40c0efc1e6ca2003	d0d2845e0c475f24035287b10bef501c	api/v1/llm/credential/delete	删除凭据	Delete Credential				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
82	d3db2dfda00515db87376d3cda9165ee	b77caa93b4383f2eabd6a9d92d6554d7	api/v1/llm/model/add	添加模型	Add Model				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
83	4bc080ec9166fd43099de83bb865a36f	b77caa93b4383f2eabd6a9d92d6554d7	api/v1/llm/model/edit	修改模型	Edit Model				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
84	4bb496f3025c7ce1a0ec24538258b007	b77caa93b4383f2eabd6a9d92d6554d7	api/v1/llm/model/delete	删除模型	Delete Model				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
85	a4839d2c0cc885152ad9e58024c9f838	7ad5be19e1ff1b2d201d42d20b2b8ec4	api/v1/llm/key/add	签发密钥	Add Key				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
86	d5c7a36fe498b320c4be8b678e5018b6	7ad5be19e1ff1b2d201d42d20b2b8ec4	api/v1/llm/key/edit	修改密钥	Edit Key				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
87	453b68f9b1cd77741f3cd30bcf34557e	7ad5be19e1ff1b2d201d42d20b2b8ec4	api/v1/llm/key/delete	删除密钥	Delete Key				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
88	bf64c2d1d1ffa8d5bcc33e22213a0d01	7ad5be19e1ff1b2d201d42d20b2b8ec4	api/v1/llm/key/rotate	轮换密钥	Rotate Key				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
302	412d3aff94b3996da16d2be7ddc927a9	45b37c9e9663411570679e7b5170fbbd	api/v1/llm/group/list	分组管理	Groups	🏷️			1	0	0	/llm/group/list	system/llm/group/index	0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
303	e388be201d9091b1a7c747fa863d3905	412d3aff94b3996da16d2be7ddc927a9	api/v1/llm/group/add	添加分组	Add Group				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
304	15b38df935279fd31a8d48e6fd2c1beb	412d3aff94b3996da16d2be7ddc927a9	api/v1/llm/group/edit	修改分组	Edit Group				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
305	7e30d74a0dd78c8fba6da7a93a413dd8	412d3aff94b3996da16d2be7ddc927a9	api/v1/llm/group/delete	删除分组	Delete Group				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
307	e4d4281cba5ee1a570865c1696bf974a	0	api/v1/mcp	MCP聚合	MCP Management	🔌			0	25	0	/mcp	layout/routerView/parent	0		0	0	1	0	0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
308	f4fd9493307ce4018c0e42234cdb9d5d	e4d4281cba5ee1a570865c1696bf974a	api/v1/mcp/server/list	服务器	Servers	🖥️			1	0	0	/mcp/server/list	system/mcp/server/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
309	ca4b51d526379dfa58f131bcbe8a504b	e4d4281cba5ee1a570865c1696bf974a	api/v1/mcp/group/list	分组	Groups	🗂️			1	0	0	/mcp/group/list	system/mcp/group/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
310	da5007870676312ec5f6770b077559cc	e4d4281cba5ee1a570865c1696bf974a	api/v1/mcp/key/list	密钥	Bearer Keys	🗝️			1	0	0	/mcp/key/list	system/mcp/key/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
311	f3dd20808249d088269157d37897ca2e	e4d4281cba5ee1a570865c1696bf974a	api/v1/mcp/activity/list	调用审计	Activities	📊			1	0	0	/mcp/activity/list	system/mcp/activity/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
312	c76af0b8f4813fea41b45eac6f046baf	e4d4281cba5ee1a570865c1696bf974a	api/v1/mcp/prompt/list	提示词	Prompts	💬			1	0	0	/mcp/prompt/list	system/mcp/prompt/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
313	e3b46495390b7dc31da1e80dfc06dccc	e4d4281cba5ee1a570865c1696bf974a	api/v1/mcp/resource/list	资源	Resources	📦			1	0	0	/mcp/resource/list	system/mcp/resource/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
314	c1b8876c5e5b8f5ce0e282b5b818b21c	e4d4281cba5ee1a570865c1696bf974a	api/v1/mcp/config/get	设置	Settings	⚙️			1	0	0	/mcp/config	system/mcp/config/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
315	f4e91e7f7f8cc775d9e00758448354ed	f4fd9493307ce4018c0e42234cdb9d5d	api/v1/mcp/server/add	添加服务器	Add Server				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
316	c6740491fa9a97b71e1496b626d9848b	f4fd9493307ce4018c0e42234cdb9d5d	api/v1/mcp/server/edit	修改服务器	Edit Server				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
317	dc5ab2295abb3ce7b68c58e97f13013d	f4fd9493307ce4018c0e42234cdb9d5d	api/v1/mcp/server/delete	删除服务器	Delete Server				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
321	b1add1fd4cd612c770607b848a37247c	ca4b51d526379dfa58f131bcbe8a504b	api/v1/mcp/group/add	添加分组	Add Group				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
322	dec3380809207336b8aa9541993077dc	ca4b51d526379dfa58f131bcbe8a504b	api/v1/mcp/group/edit	修改分组	Edit Group				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
323	bfe02cff06bf7c07e81849bc42c1bdad	ca4b51d526379dfa58f131bcbe8a504b	api/v1/mcp/group/delete	删除分组	Delete Group				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
121	e1a1a1a1a1a1a1a1a1a1a1a1a1a30c	e1a1a1a1a1a1a1a1a1a1a1a1a1a304	api/v1/agent/user/add	添加用户	Add User				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
325	a2df459b1d4b3dcc0100fa138ccb228f	da5007870676312ec5f6770b077559cc	api/v1/mcp/key/add	签发密钥	Add Key				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
326	e526c7748f1630b2de553da74f57b73b	da5007870676312ec5f6770b077559cc	api/v1/mcp/key/edit	修改密钥	Edit Key				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
327	d5afeac0b75f392429b62ffb874ec14d	da5007870676312ec5f6770b077559cc	api/v1/mcp/key/delete	删除密钥	Delete Key				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
328	fbb858d5f66b60a4a1fc3fc160867c9f	da5007870676312ec5f6770b077559cc	api/v1/mcp/key/rotate	轮换密钥	Rotate Key				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
329	d3fa0b1904a4bc4f89fa38333c241ebd	c76af0b8f4813fea41b45eac6f046baf	api/v1/mcp/prompt/add	添加提示词	Add Prompt				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
330	ac4b25507a71f8b1d732f19b0967381a	c76af0b8f4813fea41b45eac6f046baf	api/v1/mcp/prompt/edit	修改提示词	Edit Prompt				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
331	c2b3be3ea2e62a1bbd936e9c60afa64c	c76af0b8f4813fea41b45eac6f046baf	api/v1/mcp/prompt/delete	删除提示词	Delete Prompt				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
332	e5f6e852cf1af738d4218692672dd38c	e3b46495390b7dc31da1e80dfc06dccc	api/v1/mcp/resource/add	添加资源	Add Resource				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
333	efb22631da5db131ae7e3362ccf762ea	e3b46495390b7dc31da1e80dfc06dccc	api/v1/mcp/resource/edit	修改资源	Edit Resource				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
334	c058f38f60e5fdd744179c987c1446ab	e3b46495390b7dc31da1e80dfc06dccc	api/v1/mcp/resource/delete	删除资源	Delete Resource				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
335	b3d6b62939c0e3560e4e2886750b87dc	c1b8876c5e5b8f5ce0e282b5b818b21c	api/v1/mcp/config/edit	保存设置	Edit Settings				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
92	a1b2c3d4e5f64780901234567890abcd	0	api/v1/agent	Agent管理	Agent Workspace	🤖			0	28	0	/agent	layout/routerView/parent	0		0	0	1	0	0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
93	b1c2d3e4f5a64780901234567890abcd	a1b2c3d4e5f64780901234567890abcd	api/v1/agent/config/list	配置中心	Config Center	🧩			1	0	0	/agent/config/list	system/agent/config/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
95	d1e2f3a4b5c64780901234567890abcd	a1b2c3d4e5f64780901234567890abcd	api/v1/agent/scan/list	安全扫描	Security Scan	🛡️			1	0	0	/agent/scan/list	system/agent/scan/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
96	e1f2a3b4c5d64780901234567890abcd	a1b2c3d4e5f64780901234567890abcd	api/v1/agent/model/list	模型与规则	Models & Rules	⚙️			1	0	0	/agent/model/list	system/agent/model/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
97	f1a2b3c4d5e64780901234567890abcd	a1b2c3d4e5f64780901234567890abcd	api/v1/agent/monitor/list	治理监控	Governance	📊			1	0	0	/agent/monitor/list	system/agent/monitor/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
98	11a2b3c4d5e64780901234567890abcd	b1c2d3e4f5a64780901234567890abcd	api/v1/agent/config/add	添加配置	Add Config				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
99	12a2b3c4d5e64780901234567890abcd	b1c2d3e4f5a64780901234567890abcd	api/v1/agent/config/edit	修改配置	Edit Config				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
100	13a2b3c4d5e64780901234567890abcd	b1c2d3e4f5a64780901234567890abcd	api/v1/agent/config/delete	删除配置	Delete Config				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
104	31a2b3c4d5e64780901234567890abcd	d1e2f3a4b5c64780901234567890abcd	api/v1/agent/scan/add	新建扫描	Add Scan				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
105	32a2b3c4d5e64780901234567890abcd	d1e2f3a4b5c64780901234567890abcd	api/v1/agent/scan/edit	修改扫描	Edit Scan				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
106	33a2b3c4d5e64780901234567890abcd	d1e2f3a4b5c64780901234567890abcd	api/v1/agent/scan/delete	删除扫描	Delete Scan				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
107	41a2b3c4d5e64780901234567890abcd	e1f2a3b4c5d64780901234567890abcd	api/v1/agent/model/add	添加模型	Add Model				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
108	42a2b3c4d5e64780901234567890abcd	e1f2a3b4c5d64780901234567890abcd	api/v1/agent/model/edit	修改模型	Edit Model				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
109	43a2b3c4d5e64780901234567890abcd	e1f2a3b4c5d64780901234567890abcd	api/v1/agent/model/delete	删除模型	Delete Model				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
110	e1a1a1a1a1a1a1a1a1a1a1a1a1a301	a1b2c3d4e5f64780901234567890abcd	api/v1/agent/team/list	Team协作	Teams	👥			1	0	0	/agent/team/list	system/agent/team/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
111	e1a1a1a1a1a1a1a1a1a1a1a1a1a302	a1b2c3d4e5f64780901234567890abcd	api/v1/agent/openclaw/configs	OpenClaw配置	OpenClaw	🔌			1	0	0	/agent/openclaw/list	system/agent/openclaw/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
112	e1a1a1a1a1a1a1a1a1a1a1a1a1a303	a1b2c3d4e5f64780901234567890abcd	api/v1/agent/dashboard/list	概览	Overview	📈			1	0	0	/agent/dashboard/list	system/agent/dashboard/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
113	e1a1a1a1a1a1a1a1a1a1a1a1a1a304	a1b2c3d4e5f64780901234567890abcd	api/v1/agent/user/list	用户管理	Users	👤			1	0	0	/agent/user/list	system/agent/user/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
114	e1a1a1a1a1a1a1a1a1a1a1a1a1a305	a1b2c3d4e5f64780901234567890abcd	api/v1/agent/settings/list	系统设置	System	⚙️			1	0	0	/agent/settings/list	system/agent/settings/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
115	e1a1a1a1a1a1a1a1a1a1a1a1a1a306	e1a1a1a1a1a1a1a1a1a1a1a1a1a301	api/v1/agent/team/add	创建Team	Add Team				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
116	e1a1a1a1a1a1a1a1a1a1a1a1a1a307	e1a1a1a1a1a1a1a1a1a1a1a1a1a301	api/v1/agent/team/edit	修改Team	Edit Team				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
117	e1a1a1a1a1a1a1a1a1a1a1a1a1a308	e1a1a1a1a1a1a1a1a1a1a1a1a1a301	api/v1/agent/team/delete	删除Team	Delete Team				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
118	e1a1a1a1a1a1a1a1a1a1a1a1a1a309	e1a1a1a1a1a1a1a1a1a1a1a1a1a302	api/v1/agent/openclaw/configs	查看配置	View Config				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
119	e1a1a1a1a1a1a1a1a1a1a1a1a1a30a	e1a1a1a1a1a1a1a1a1a1a1a1a1a302	api/v1/agent/openclaw/configs/validate	校验配置	Validate Config				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
120	e1a1a1a1a1a1a1a1a1a1a1a1a1a30b	e1a1a1a1a1a1a1a1a1a1a1a1a1a302	api/v1/agent/openclaw/configs/save	保存配置	Save Config				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
122	e1a1a1a1a1a1a1a1a1a1a1a1a1a30d	e1a1a1a1a1a1a1a1a1a1a1a1a1a304	api/v1/agent/user/edit	修改用户	Edit User				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
123	e1a1a1a1a1a1a1a1a1a1a1a1a1a30e	e1a1a1a1a1a1a1a1a1a1a1a1a1a304	api/v1/agent/user/delete	删除用户	Delete User				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
124	e1a1a1a1a1a1a1a1a1a1a1a1a1a30f	e1a1a1a1a1a1a1a1a1a1a1a1a1a305	api/v1/agent/settings/upsert	保存镜像	Save Image				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
125	e1a1a1a1a1a1a1a1a1a1a1a1a1a310	e1a1a1a1a1a1a1a1a1a1a1a1a1a305	api/v1/agent/settings/delete	删除镜像	Delete Image				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
126	e1a1a1a1a1a1a1a1a1a1a1a1a1a311	a1b2c3d4e5f64780901234567890abcd	api/v1/agent/instance/list	运行实例	Instances	📦			1	0	0	/agent/instance/list	system/agent/instance/index	0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
127	e1a1a1a1a1a1a1a1a1a1a1a1a1a312	e1a1a1a1a1a1a1a1a1a1a1a1a1a311	api/v1/agent/instance/add	创建实例	Add Instance				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
128	e1a1a1a1a1a1a1a1a1a1a1a1a1a313	e1a1a1a1a1a1a1a1a1a1a1a1a1a311	api/v1/agent/instance/edit	修改实例	Edit Instance				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
129	e1a1a1a1a1a1a1a1a1a1a1a1a1a314	e1a1a1a1a1a1a1a1a1a1a1a1a1a311	api/v1/agent/instance/start	启停实例	Start/Stop				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
130	e1a1a1a1a1a1a1a1a1a1a1a1a1a315	e1a1a1a1a1a1a1a1a1a1a1a1a1a311	api/v1/agent/instance/delete	删除实例	Delete Instance				2	0	0			0		0	0	1		0		[]	2026-07-04 00:00:00	2026-07-04 00:00:00
131	e1a1a1a1a1a1a1a1a1a1a1a1a1a316	e1a1a1a1a1a1a1a1a1a1a1a1a1a311	api/v1/agent/instance/command/list	命令队列	Commands	📨			2	0	0			0		0	0	1		0		[]	2026-07-13 00:00:00	2026-07-13 00:00:00
132	e1a1a1a1a1a1a1a1a1a1a1a1a1a317	e1a1a1a1a1a1a1a1a1a1a1a1a1a311	api/v1/agent/instance/command/enqueue	下发命令	Enqueue Command				2	0	0			0		0	0	1		0		[]	2026-07-13 00:00:00	2026-07-13 00:00:00
133	e1a1a1a1a1a1a1a1a1a1a1a1a1a318	e1a1a1a1a1a1a1a1a1a1a1a1a1a311	api/v1/agent/instance/runtime-status	运行状态	Runtime Status				2	0	0			0		0	0	1		0		[]	2026-07-13 00:00:00	2026-07-13 00:00:00
336	clientmenutopdir00000000000000a	0	api/v1/client	桌面客户端	Desktop	💻			0	29	0	/client	layout/routerView/parent	0		0	0	1		0		[]	2026-07-05 00:00:00	2026-07-05 00:00:00
337	clientmenucfg00000000000000000a	clientmenutopdir00000000000000a	api/v1/system/client/admin/agent/list	Agent配置	Desktop Config	🧩			1	0	0	/client/config	system/client/config/index	0		0	0	1		0		[]	2026-07-05 00:00:00	2026-07-05 00:00:00
338	clientmenustatus00000000000000a	clientmenutopdir00000000000000a	api/v1/system/client/admin/status/list	运行状态	Desktop Status	📊			1	0	0	/client/status	system/client/status/index	0		0	0	1		0		[]	2026-07-05 00:00:00	2026-07-05 00:00:00
339	clientmenudevice00000000000000a	clientmenutopdir00000000000000a	api/v1/system/client/admin/device/list	设备管理	Desktop Device	🖥️			1	0	0	/client/device	system/client/device/index	0		0	0	1		0		[]	2026-07-05 00:00:00	2026-07-05 00:00:00
340	clientmenupackage00000000000a	clientmenutopdir00000000000000a	api/v1/system/client/admin/package/list	安装包管理	Desktop Package	📦			1	0	0	/client/package	system/client/package/index	0		0	0	1		0		[]	2026-07-05 00:00:00	2026-07-05 00:00:00
342	clientmenusave0000000000000000a	clientmenucfg00000000000000000a	api/v1/system/client/admin/agent/save	保存Agent	Save Agent				2	0	0			0		0	0	1		0		[]	2026-07-05 00:00:00	2026-07-05 00:00:00
343	clientmenuversave0000000000000a	clientmenucfg00000000000000000a	api/v1/system/client/admin/version/save	保存版本	Save Version				2	0	0			0		0	0	1		0		[]	2026-07-05 00:00:00	2026-07-05 00:00:00
344	clientmenuverdel00000000000000a	clientmenucfg00000000000000000a	api/v1/system/client/admin/version/delete	删除版本	Delete Version				2	0	0			0		0	0	1		0		[]	2026-07-05 00:00:00	2026-07-05 00:00:00
345	clientmenudevrevoke00000000000a	clientmenudevice00000000000000a	api/v1/system/client/admin/device/revoke	撤销登录	Revoke Device				2	0	0			0		0	0	1		0		[]	2026-07-05 00:00:00	2026-07-05 00:00:00
346	clientmenudevblock00000000000a	clientmenudevice00000000000000a	api/v1/system/client/admin/device/block	封禁设备	Block Device				2	0	0			0		0	0	1		0		[]	2026-07-05 00:00:00	2026-07-05 00:00:00
347	clientmenudevdel00000000000000a	clientmenudevice00000000000000a	api/v1/system/client/admin/device/delete	删除设备	Delete Device				2	0	0			0		0	0	1		0		[]	2026-07-05 00:00:00	2026-07-05 00:00:00
348	clientmenudevcmd0000000000000a	clientmenudevice00000000000000a	api/v1/system/client/admin/device/command	下发指令	Send Command				2	0	0			0		0	0	1		0		[]	2026-07-05 00:00:00	2026-07-05 00:00:00
349	clientmenudevcmdhist0000000000a	clientmenudevice00000000000000a	api/v1/system/client/admin/device/commands	指令历史	Command History				2	0	0			0		0	0	1		0		[]	2026-07-05 00:00:00	2026-07-05 00:00:00
352	clientmenupkgfetch0000000000a	clientmenupackage00000000000a	api/v1/system/client/admin/package/fetch	拉取安装包	Fetch Package				2	0	0			0		0	0	1		0		[]	2026-07-05 00:00:00	2026-07-05 00:00:00
353	clientmenupkgdel0000000000000a	clientmenupackage00000000000a	api/v1/system/client/admin/package/delete	删除安装包	Delete Package				2	0	0			0		0	0	1		0		[]	2026-07-05 00:00:00	2026-07-05 00:00:00
362	wqia8s3qkidtd9m4oyerq1ua1ira106j	0	api/v1/project	项目管理	Project Management	📁			0	32	0	/project	layout/routerView/parent	0		0	0	1	0	0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
363	ef7j2ajpsl7ic1837eob7j24kju6rmsc	wqia8s3qkidtd9m4oyerq1ua1ira106j	api/v1/project/list	项目列表	Projects	📋			1	0	0	/project/list	system/project/list/index	0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
364	nf5ulvetty6q43d2g3u112954dg8ghur	wqia8s3qkidtd9m4oyerq1ua1ira106j	api/v1/project/task/list	任务管理	Tasks	📝			1	0	0	/project/task	system/project/task/index	0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
365	ug2ln624tbfor1s59tqqlh445bgv57rh	wqia8s3qkidtd9m4oyerq1ua1ira106j	api/v1/project/board	任务看板	Board	🔲			1	0	0	/project/board	system/project/board/index	0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
366	s8ivn8synch7tjh0r4w6dqbf8wkid0lb	wqia8s3qkidtd9m4oyerq1ua1ira106j	api/v1/project/milestone/list	里程碑管理	Milestones	🏁			1	0	0	/project/milestone	system/project/milestone/index	0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
367	zpp19f26wnrkjoy1191ls6ihwg047czm	wqia8s3qkidtd9m4oyerq1ua1ira106j	api/v1/project/timeline/list	排期视图	Timeline	🗓️			1	0	0	/project/timeline	system/project/timeline/index	0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
368	ag8znpm5eoyq8ch3ne219aq891qi2bxd	wqia8s3qkidtd9m4oyerq1ua1ira106j	api/v1/project/member/list	成员管理	Members	👥			1	0	0	/project/member	system/project/member/index	0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
369	r1ypvqqxtdbzj5rga9te9vkc6kpguh8n	wqia8s3qkidtd9m4oyerq1ua1ira106j	api/v1/project/report/overview	统计报表	Reports	📊			1	0	0	/project/report	system/project/report/index	0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
370	ith4r52j2f1evx60bdx7mfiapkz7ro0z	wqia8s3qkidtd9m4oyerq1ua1ira106j	api/v1/project/activity/list	活动动态	Activity	💬			1	0	0	/project/activity	system/project/activity/index	0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
371	p0mm8y53uvbddxu9d4mbv7vdxds8zn3j	wqia8s3qkidtd9m4oyerq1ua1ira106j	api/v1/project/config/tag/list	项目设置	Settings	⚙️			1	0	0	/project/config	system/project/config/index	0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
372	abzl156x38kfe3ldmgzg77xqfu111kai	ef7j2ajpsl7ic1837eob7j24kju6rmsc	api/v1/project/add	新建项目	Add Project				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
373	jkh7839a82iwlobkd7zneeb9vxemmm4m	ef7j2ajpsl7ic1837eob7j24kju6rmsc	api/v1/project/edit	修改项目	Edit Project				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
374	x9jenr58jy0izxbi9igj2e4vq4j1iiue	ef7j2ajpsl7ic1837eob7j24kju6rmsc	api/v1/project/delete	删除项目	Delete Project				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
375	ssr8g284gyvjxqrkqbd3uiqel4e6hn1l	nf5ulvetty6q43d2g3u112954dg8ghur	api/v1/project/task/add	新建任务	Add Task				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
376	ocl5el43or45szhej29qcl3p2bgfyygh	nf5ulvetty6q43d2g3u112954dg8ghur	api/v1/project/task/edit	修改任务	Edit Task				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
377	ndtlv64qxrdvyvq49zzdzxo12lmaielk	nf5ulvetty6q43d2g3u112954dg8ghur	api/v1/project/task/delete	删除任务	Delete Task				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
378	k09pl2pm2h1herb2e4ilol3ykpmyjruj	ug2ln624tbfor1s59tqqlh445bgv57rh	api/v1/project/task/move	拖拽改状态	Move Task				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
379	kkul86qh6bup47y7z4or739tgmhcmvhp	s8ivn8synch7tjh0r4w6dqbf8wkid0lb	api/v1/project/milestone/add	新建里程碑	Add Milestone				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
380	jcx0kljb4s0hugnmvya0flxpjutrnoje	s8ivn8synch7tjh0r4w6dqbf8wkid0lb	api/v1/project/milestone/edit	修改里程碑	Edit Milestone				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
381	bpkbouf0j5b05no95ihmvb0v1h9kqaie	s8ivn8synch7tjh0r4w6dqbf8wkid0lb	api/v1/project/milestone/delete	删除里程碑	Delete Milestone				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
382	f7s21n5ohvqrxxohjnaf7dylari6p8ph	ag8znpm5eoyq8ch3ne219aq891qi2bxd	api/v1/project/member/add	添加成员	Add Member				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
383	o31bcxp2wtzbl90d56szmvpfr5t4rb9w	ag8znpm5eoyq8ch3ne219aq891qi2bxd	api/v1/project/member/edit	修改成员	Edit Member				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
384	ar0sgu9sy675xwhgjhpo61pm9yo1shzd	ag8znpm5eoyq8ch3ne219aq891qi2bxd	api/v1/project/member/delete	移除成员	Remove Member				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
385	qi565hgt51dz7gk4bh03ppi8krzyp3nh	ef7j2ajpsl7ic1837eob7j24kju6rmsc	api/v1/project/binding/add	绑定资源	Add Binding				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
386	vggck2olb8t3irdz3skcc90nc0ez025o	ef7j2ajpsl7ic1837eob7j24kju6rmsc	api/v1/project/binding/edit	修改绑定	Edit Binding				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
387	dg1dbh1mq8p3m4uz8sn7k1yjuak8x69d	ef7j2ajpsl7ic1837eob7j24kju6rmsc	api/v1/project/binding/delete	解绑资源	Delete Binding				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
388	nwubrkn0pzsetx9tipmwmki29xu8w3bs	p0mm8y53uvbddxu9d4mbv7vdxds8zn3j	api/v1/project/config/tag/add	添加标签	Add Tag				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
389	lan544jk8jb15vtznpcsaji1us6pcqcv	p0mm8y53uvbddxu9d4mbv7vdxds8zn3j	api/v1/project/config/tag/edit	修改标签	Edit Tag				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
390	eqzht2kiegp7mv853ps6iabtfiio1hxu	p0mm8y53uvbddxu9d4mbv7vdxds8zn3j	api/v1/project/config/tag/delete	删除标签	Delete Tag				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
391	yorvn6hf6plcy19s9kbqqj7l8s62nn4t	p0mm8y53uvbddxu9d4mbv7vdxds8zn3j	api/v1/project/config/status/add	添加状态	Add Status				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
392	rllioozbsqprtybcg3qllx7xpbrgim4c	p0mm8y53uvbddxu9d4mbv7vdxds8zn3j	api/v1/project/config/status/edit	修改状态	Edit Status				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
393	usldtmxppalaa59lpr026ohw7ektrmul	p0mm8y53uvbddxu9d4mbv7vdxds8zn3j	api/v1/project/config/status/delete	删除状态	Delete Status				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
394	q5gwyb22qfjhzlevszccwhanvfqtilrw	p0mm8y53uvbddxu9d4mbv7vdxds8zn3j	api/v1/project/config/field/add	添加字段	Add Field				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
395	ad968wt7y61z0mrd33wqfw5hie4dyd8y	p0mm8y53uvbddxu9d4mbv7vdxds8zn3j	api/v1/project/config/field/edit	修改字段	Edit Field				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
396	nj7bkiz33ef7u4otfhjiv03xwqyjb6mv	p0mm8y53uvbddxu9d4mbv7vdxds8zn3j	api/v1/project/config/field/delete	删除字段	Delete Field				2	0	0			0		0	0	1		0		[]	2026-07-25 00:00:00	2026-07-25 00:00:00
535	mcpmenuservertest00000000000000a	f4fd9493307ce4018c0e42234cdb9d5d	api/v1/mcp/server/test	连接测试	Test Connection				2	0	0			0		0	0	1		0		[]	2026-08-29 00:00:00	2026-08-29 00:00:00
536	mcpmenuservercalltool0000000000a	f4fd9493307ce4018c0e42234cdb9d5d	api/v1/mcp/server/callTool	试调用	Call Tool				2	0	0			0		0	0	1		0		[]	2026-08-29 00:00:00	2026-08-29 00:00:00
565	pgmenumain000000000000000000000a	jeo9mvaihkkm4hs0mrhlpqg2nxuvvgqr	api/v1/system/permgroupList	权限分组管理	Permission Groups	🧩			1	0	0	/system/permgroup/list	system/permgroup/index	0		0	0	1		0		[]	2026-09-01 00:00:00	2026-09-01 00:00:00
566	pgmgrouplist0000000000000000000a	pgmenumain000000000000000000000a	api/v1/system/permgroup/group/list	分组列表	Group List				2	0	0			0		0	0	1		0		[]	2026-09-01 00:00:00	2026-09-01 00:00:00
567	pgmgroupadd00000000000000000000a	pgmenumain000000000000000000000a	api/v1/system/permgroup/group/add	新增分组	Add Group				2	0	0			0		0	0	1		0		[]	2026-09-01 00:00:00	2026-09-01 00:00:00
568	pgmgroupedit0000000000000000000a	pgmenumain000000000000000000000a	api/v1/system/permgroup/group/edit	编辑分组	Edit Group				2	0	0			0		0	0	1		0		[]	2026-09-01 00:00:00	2026-09-01 00:00:00
569	pgmgroupdelete00000000000000000a	pgmenumain000000000000000000000a	api/v1/system/permgroup/group/delete	删除分组	Delete Group				2	0	0			0		0	0	1		0		[]	2026-09-01 00:00:00	2026-09-01 00:00:00
570	pgmsetlist000000000000000000000a	pgmenumain000000000000000000000a	api/v1/system/permgroup/set/list	集合列表	Set List				2	0	0			0		0	0	1		0		[]	2026-09-01 00:00:00	2026-09-01 00:00:00
571	pgmsetadd0000000000000000000000a	pgmenumain000000000000000000000a	api/v1/system/permgroup/set/add	新增集合	Add Set				2	0	0			0		0	0	1		0		[]	2026-09-01 00:00:00	2026-09-01 00:00:00
572	pgmsetedit000000000000000000000a	pgmenumain000000000000000000000a	api/v1/system/permgroup/set/edit	编辑集合	Edit Set				2	0	0			0		0	0	1		0		[]	2026-09-01 00:00:00	2026-09-01 00:00:00
573	pgmsetdelete0000000000000000000a	pgmenumain000000000000000000000a	api/v1/system/permgroup/set/delete	删除集合	Delete Set				2	0	0			0		0	0	1		0		[]	2026-09-01 00:00:00	2026-09-01 00:00:00
619	e1a1a1a1a1a1a1a1a1a1a1a1a322	e1a1a1a1a1a1a1a1a1a1a1a1a1a304	api/v1/agent/user/reset-password	重置密码	Reset Password				2	0	0			0		0	0	1		0		[]	2026-10-16 00:00:00	2026-10-16 00:00:00
620	kbdslistmenu0000000000000000000a	kbmenudir000000000000000000000a	api/v1/kb/datasource/list	数据源	Data Sources				1	0	0	/kb/datasource	system/kb/datasource/index	0		0	0	1		0		[]	2026-10-16 00:00:00	2026-10-16 00:00:00
621	kbdsbtget0000000000000000000000a	kbdslistmenu0000000000000000000a	api/v1/kb/datasource/get	查看数据源	View Datasource				2	0	0			0		0	0	1		0		[]	2026-10-16 00:00:00	2026-10-16 00:00:00
622	kbdsbtadd0000000000000000000000a	kbdslistmenu0000000000000000000a	api/v1/kb/datasource/add	新建数据源	Add Datasource				2	0	0			0		0	0	1		0		[]	2026-10-16 00:00:00	2026-10-16 00:00:00
623	kbdsbtedit000000000000000000000a	kbdslistmenu0000000000000000000a	api/v1/kb/datasource/edit	编辑数据源	Edit Datasource				2	0	0			0		0	0	1		0		[]	2026-10-16 00:00:00	2026-10-16 00:00:00
624	kbdsbtdel0000000000000000000000a	kbdslistmenu0000000000000000000a	api/v1/kb/datasource/delete	删除数据源	Delete Datasource				2	0	0			0		0	0	1		0		[]	2026-10-16 00:00:00	2026-10-16 00:00:00
625	kbdsbtcancel0000000000000000000a	kbdslistmenu0000000000000000000a	api/v1/kb/datasource/runs/cancel	取消同步	Cancel Sync				2	0	0			0		0	0	1		0		[]	2026-10-16 00:00:00	2026-10-16 00:00:00
626	kbdsbttrigger000000000000000000a	kbdslistmenu0000000000000000000a	api/v1/kb/datasource/trigger	触发同步	Trigger Sync				2	0	0			0		0	0	1		0		[]	2026-10-16 00:00:00	2026-10-16 00:00:00
627	kbdsbtruns000000000000000000000a	kbdslistmenu0000000000000000000a	api/v1/kb/datasource/runs	同步记录	Sync Runs				2	0	0			0		0	0	1		0		[]	2026-10-16 00:00:00	2026-10-16 00:00:00
628	mcpoauthlistmenu000000000000000a	e4d4281cba5ee1a570865c1696bf974a	api/v1/mcp/oauth/client/list	OAuth应用	OAuth Clients				1	0	0	/mcp/oauth/list	system/mcp/oauth/index	0		0	0	1		0		[]	2026-10-16 00:00:00	2026-10-16 00:00:00
629	mcpoauthbtadd000000000000000000a	mcpoauthlistmenu000000000000000a	api/v1/mcp/oauth/client/add	添加应用	Add Client				2	0	0			0		0	0	1		0		[]	2026-10-16 00:00:00	2026-10-16 00:00:00
630	mcpoauthbtedit00000000000000000a	mcpoauthlistmenu000000000000000a	api/v1/mcp/oauth/client/edit	编辑应用	Edit Client				2	0	0			0		0	0	1		0		[]	2026-10-16 00:00:00	2026-10-16 00:00:00
631	mcpoauthbtdel000000000000000000a	mcpoauthlistmenu000000000000000a	api/v1/mcp/oauth/client/delete	删除应用	Delete Client				2	0	0			0		0	0	1		0		[]	2026-10-16 00:00:00	2026-10-16 00:00:00
632	mcpoauthbtrotate000000000000000a	mcpoauthlistmenu000000000000000a	api/v1/mcp/oauth/client/rotate-secret	轮换密钥	Rotate Secret				2	0	0			0		0	0	1		0		[]	2026-10-16 00:00:00	2026-10-16 00:00:00
633	mcpoauthtoklist0000000000000000a	mcpoauthlistmenu000000000000000a	api/v1/mcp/oauth/token/list	令牌列表	Token List				2	0	0			0		0	0	1		0		[]	2026-10-16 00:00:00	2026-10-16 00:00:00
634	mcpoauthtokrevoke00000000000000a	mcpoauthlistmenu000000000000000a	api/v1/mcp/oauth/token/revoke	吊销令牌	Revoke Token				2	0	0			0		0	0	1		0		[]	2026-10-16 00:00:00	2026-10-16 00:00:00
\.


--
-- Data for Name: aa_notice; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_notice (id, notice_id, user_id, title, content, link, type, is_read, created_by, created_at, updated_at) FROM stdin;
1	klwc8ycspd2626k76vdf45iv4b0af3jc	system	技能安全扫描通过	技能「aiarsenal-user」1.0.0 安全扫描通过，该版本可发布/安装。	/skill/security/list	success	0	scanner	2026-09-17 05:31:13.541072	\N
\.


--
-- Data for Name: aa_oper_log; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_oper_log (id, oper_id, title, business_type, method, request_method, operator_type, oper_name, dept_name, oper_url, oper_ip, oper_location, oper_param, error_msg, oper_time) FROM stdin;
\.


--
-- Data for Name: aa_perm_group; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_perm_group (id, group_id, name, remark, terms, created_by, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_post; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_post (id, post_id, post_code, post_name, post_sort, status, remark, created_by, updated_by, created_at, updated_at) FROM stdin;
1	lrucr0f4qcbprcobf33lvkqsg8ml3jyj	ceo	董事长	1	1		0	0	2021-07-11 11:32:58	\N
2	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le	se	项目经理	2	1		0	0	2021-07-12 11:01:26	\N
3	w1c2cofhzsnm559td9uot44p6mg35zza	hr	人力资源	3	1		0	pg85t687mpyufz2z4ehcfazh7pavvxut	2021-07-12 11:01:30	2022-09-16 16:48:18
4	las48k9l1ii0ir2962a5r4xfxame2btu	user	普通员工	4	0	普通员工	0	pg85t687mpyufz2z4ehcfazh7pavvxut	2021-07-12 11:01:33	2022-04-08 15:32:23
5	l78iqkryimdkqjna4sj41n9ntneivpab	it	IT部	5	1	信息部	pg85t687mpyufz2z4ehcfazh7pavvxut	pg85t687mpyufz2z4ehcfazh7pavvxut	2021-07-12 11:09:42	2022-04-09 12:59:12
6	ec0minuxxdiktu5c3hqu72eayy06ljrz	cto	技术总监	6	1	技术负责人	pg85t687mpyufz2z4ehcfazh7pavvxut	0	2022-04-08 15:32:44	2022-04-08 15:32:44
7	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v	dev	研发工程师	7	1	产品研发	pg85t687mpyufz2z4ehcfazh7pavvxut	0	2022-04-08 15:32:55	2022-04-08 15:32:55
8	p18peuyi9qaddojzcck35ufolgawzrpg	qa	测试工程师	8	1	质量保障	pg85t687mpyufz2z4ehcfazh7pavvxut	0	2022-04-08 15:33:01	2022-04-08 15:33:01
9	vetr6vywrxw1mfbhxco2oaaiz7okedet	ops	运维工程师	9	1	运维保障	pg85t687mpyufz2z4ehcfazh7pavvxut	0	2022-04-08 15:52:53	2022-04-08 15:52:53
\.


--
-- Data for Name: aa_review_order; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_review_order (id, order_id, order_type, title, applicant_id, applicant_name, dept_id, amount, payload, status, reviewer_id, reviewer_name, review_note, result_ref, reviewed_at, created_by, updated_by, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_role; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_role (id, role_id, status, list_order, name, remark, created_at, updated_at) FROM stdin;
1	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	1	0	超级管理员	备注	2022-04-01 11:38:39	2022-04-28 10:00:15
2	kt9c0598mbwnif8igl6ziaz55ehxqp8t	1	0	普通管理员	备注	2022-04-01 11:38:39	2022-04-28 10:01:34
3	h9driy19c07f6tgmxbpn1qp39vnqdv9c	1	0	站点管理员	站点管理人员	2022-04-01 11:38:39	2022-04-01 11:38:39
4	pql3anx0ft4qn31brg6b2ev2rpguptze	1	0	初级管理员	初级管理员	2022-04-01 11:38:39	2022-04-01 11:38:39
5	yrbx49f8kvuilrnbn8zy4hc2k1x3h99u	1	0	高级管理员	高级管理员	2022-04-01 11:38:39	2022-04-01 11:38:39
8	v8cw0kke6bb75tdk4oya61ku9zbi3dhz	1	0	区级管理员		2022-04-01 11:38:39	2022-04-06 09:53:40
\.


--
-- Data for Name: aa_rule; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_rule (ptype, v0, v1, v2, v3, v4, v5) FROM stdin;
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	jeo9mvaihkkm4hs0mrhlpqg2nxuvvgqr	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	t5w3n2zlz7hssio12g960kmw9mb0q6gp	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	jnqs6sca03fhu76y78j6w1oncy6en7kr	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	j7endjy4gs1co0opax9vv58h7dfxhddk	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	c999jlcifgwwk4be7cpak9vohng8suoo	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	uuj4xhr6fi4mg0t0fhd9a15dboklgk9z	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	q4n01yci6swfnrjt1azphn6z6wwnnbco	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	oko5fv9off4dr4o4u3g9gxlvfh8780au	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	ywtex3edy9d0gfr5iyo9fmmvhgb7yz4r	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	xr83ya4kyxlnb3t1163puyimzs0awq1v	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	vyzbuuux5wo6poqapcoet020d0efvtco	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	hi4iu377mcuv3xoj48uh9bnq5tycn5ar	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	tqpajoh8ufqacz13dijgnsq8fyssedgb	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	mrcmjf6214rehygpt9w92v3uqcbaljea	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	djihplrjenn31sqbebda8b6h0wqtey1w	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	oi4mubhpz95iudjrjiisaruh9i2imdtp	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	nxcf3qsruwghnrw9yxjvxx07p0ujhz0r	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	sgfi0ulrhgjur9jw1z2wnr1nmfv7gsba	All	g		
p	kt9c0598mbwnif8igl6ziaz55ehxqp8t	dro7ldd3tizp78z3ekepx74b6820h0nw	All	g		
p	kt9c0598mbwnif8igl6ziaz55ehxqp8t	dgk3u4jbh0idyt533flqch7ix9oqssem	All	g		
p	kt9c0598mbwnif8igl6ziaz55ehxqp8t	qh93mn1hmam93p21aoxgpzw5g5cr73xk	All	g		
g	u_i47dhcwyx2rulbjl5vlab89orpamongb	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_awaoolltywg611faqbrfjs0a6qq7e7mq	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_ttozve8tomu4bw7q64iqch473vic8xjx	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_l7v7hxinitk68gtbwp1ygd1ud39q88bt	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_g1zzvvdxqrpmzk2kw8q4ky3a26e6qfxv	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_wcsj3t9d38o5o5ryzp9pl0s72bbolqje	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_psnpiaisc4utvtowod2cismxnen0sqji	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_yftk7uwwhy2oxmrzkq3tp4dh89bch1jb	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_ey81ndf6zpobmaoxqiquhofjpu02z8ht	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_hhhnxqvutljctsgaoo780vo563bez55a	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_xdrjswzjeydx1v5ge05jltxd0ixg0yya	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_qqzy7xff86q8kfmyopjq5dmgbwf03lkd	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_hwogmz6ku42vzbxo7l4g9seiftxmf55z	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_vxxrlq1k4nmwnbuiud4z8g4zdn6brnom	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_t8ubxdsaiuixyj57rt57vhrggh6yejco	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_huoc4rxc0p8g6f4cswatmogr6daqfxgl	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_yv6887f2ne6v8cwx0shm2ty97nbv4vme	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_cz5dv5ogq1ifcqmsempf00hqx4w9e7fr	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_ddv2o2ggneyz5omf2hsakq5gq5nb37kp	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_iozdijm6umoh07mwmf9shtyb3gkm6b2e	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_xkbw9qo0volhjszdrgr11ieh9qmy05xm	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_ojmwz7cbldpxwnmu4h9ed5gzjm8yglbj	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_ouvzpmuuf8vmoja8g7ua9x1a6m83dkqe	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_f571jz7rptodss6igenpqkbdv9s8nhgq	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_nn5sxc1va1wbi7c3y8nj3si1tznkil1o	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_rjtjyaegy2r498hswx8htls1ncqz2shp	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_o9ryfxtbs7mz7d1pitt1ji1eyofbuvhl	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_v8nyq4ekvn96ffe69fpteqprfbyw7d5o	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_ccp94mq5kpf7del1n8kixhght9hoc74f	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_i5jq4vt4wk4pf3lb4nx2fh4339wbfz0y	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_h30ls46rppecsk1di4n5zxlz7xpcps5i	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_g4dp1tsgez5vvn1a2hq7vpgrireuwr3f	yrbx49f8kvuilrnbn8zy4hc2k1x3h99u				
g	u_xh9e2z238dalqjxse0gdlrteaf49325f	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_p6k1ds5468pmzbz1i6v4814lt5475m3m	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_nxbyzuyiu6ouyygcuzemmac873aljn3j	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_f386t32hk8sghcnvf24g3st3i41phl9o	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_qu9owum1ui0elni61b5i29akyl7jt03p	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_b7qp8p0xj1okbz6vxrmzngd32jsonytp	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_qh1et5ylq2fm1e82d32fl67gvnqo6riq	yrbx49f8kvuilrnbn8zy4hc2k1x3h99u				
g	u_tyrhx8gab3hzay0xmz65aia5j5xzjcuo	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_wlkajsoyh27oscyja111i6bq5e5cd5gx	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_ki2q6fgdt8o56u3vn2ckanvoy34qqq9a	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_lsinkrec95cl8xk41pc1xa0nwp77r7nq	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_u7keg12zsf1fp6wxnd0w78r03lk9p5ne	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_vj9e202tchk0m7ar1pg1w2xvcb52770u	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_bdlps6p632yb96o44b00s23l1eywr0sy	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_fd5893szdczhr9ylo1cwbrsbb7f6s0np	yrbx49f8kvuilrnbn8zy4hc2k1x3h99u				
g	u_fisec94xg1zmsoe73xqi8s7pcioxs2xy	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_qjnllgil3gdfowp1khpin6o2b6egz7vs	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_et5k755mj9ndmzcz5uqj79o0se8a7hjq	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_z1cnqvbthuj6j2ocm19p5wivwknkwwxb	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_fgyo9wgof0kcalr2r8cskc2g0bejiw4r	yrbx49f8kvuilrnbn8zy4hc2k1x3h99u				
g	u_zzye39s02viahffg8futr2nq2m8ahw9v	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_nwesstx94ue6ekr832lgdj7zz1r81yol	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_fczzjf2lns76m9avq50s302n8bfv6kxy	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_ljkgwn4eowlvuh9aafzdc0ykhj06ptpw	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_rbfrd3phrnqazpxz83q61q87t3s6znun	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_qdroqjw2xx7ckth36lsw9lfmexyf1fbt	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_nadp1hsoga0hqcw16pynzk19h7ehcj5r	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_l5iu02zzxae7gw6denbs5m430pnakwco	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_g5zzcwa6lljcra9bh95ot0wmdbo8bb1v	yrbx49f8kvuilrnbn8zy4hc2k1x3h99u				
g	u_idg18sc5jlnnbs02myk619oj5s8kstej	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_ap2xvhdaf6feytr8bbfa0x563uuoztsm	yrbx49f8kvuilrnbn8zy4hc2k1x3h99u				
g	u_aqjsus7aopdpz778evymkbst0mwg6s4l	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_y9i1zy0zq9jtjx57eleglzfvurltdf9v	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_swy9yjech3it00fnpuk9m5t3ww50f0zu	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_kp0b2zujdinw3quweijsa8k1fn5jo29l	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_t6bh2x83k6hg6haogt4loyl3dw283d1d	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_nk12a6f4keadmc12tfw69abqnb0hco5d	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_zqb590opd1gatrzyadk6xax7e7bx500y	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_hyztdi4uhb1w7ndfi47zpp1y5m16soai	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_kb38c9j616dragwwgbyo8p7j4vk1xnub	yrbx49f8kvuilrnbn8zy4hc2k1x3h99u				
g	u_b0f6pm4d64m6zz6agzg7ff1y80kb112t	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_hhqf1lr8ktqflwomy9ywlr3e2g87c1nr	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_vfuzzdizy3ladqg0f9bwb22csxl2b8zg	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_bred3yjuutw1vt7mlwx4259tlez2hyvt	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_o109vwi25ey3ru4726gkeedt6yed15po	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_qpsr1il81pv2rpwsygkudq6ec28li8ee	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_quguougzmiaoxn29p4jm7vfza0nu2mqn	yrbx49f8kvuilrnbn8zy4hc2k1x3h99u				
g	u_vbsw1wfxo4xiylrqp4ytlvb0iomct94u	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_rhdnuq6wo64neftkd68yny35yste2xtx	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_p6m3p05a3n48hsaq294d431wybsq07zr	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_z5o2otfmasx8ylqa1iolk9z530n08opl	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_fxufj8dsgmh7hwzc1zgbgoiptbr9smex	yrbx49f8kvuilrnbn8zy4hc2k1x3h99u				
g	u_fyqkcyecpap2uqzaocs8q8t8x3cvmvfq	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_v32y4lq3ljmqs3qgv8onxtk16gnutgri	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_y88icjiuzvqdnpjuh4lype9xt97a3xxr	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_smg1dhkia9fg7hjuycfk8449fz2dzj2a	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_so85b77vby7plhnvr6pf17c98b9mf0to	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_r8144ojgas0uiprldnk21ckrbw6z9kgy	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_tdunjhhjpzmmiw9eu8a9shulxqbfp8ec	yrbx49f8kvuilrnbn8zy4hc2k1x3h99u				
g	u_mxlhyiou8vg6szccawm0zmpug4gg5xrb	v8cw0kke6bb75tdk4oya61ku9zbi3dhz				
g	u_nq2gkeoricp2ufl5ksvv6pznixkj7ifz	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_hnvmzuzx6jup48fa2ow28va7vmjmipua	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_eieus50pzqqvzj1ge6l72zoina4v91gy	h9driy19c07f6tgmxbpn1qp39vnqdv9c				
g	u_rump0avc7t7htk5y52r61h7n1l90jv9f	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_fy7jo8iycp36dkl4rjko3oy6r8vknu3m	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_e2ctnmnw5esjmd5o5gp7zg2gw3f6mnwm	pql3anx0ft4qn31brg6b2ev2rpguptze				
g	u_ouzpq54s44roqs97m7ii1breztn12gtu	kt9c0598mbwnif8igl6ziaz55ehxqp8t				
g	u_s29fp7shxnshfy13z03uzw0rpnp9rkrm	pql3anx0ft4qn31brg6b2ev2rpguptze				
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docdocembed00000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenuapppermreset000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenuapppermappsuggest0000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	mcpmenuservertest00000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	mcpmenuservercalltool0000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	mcpmenuserveralldata00000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	mcpmenugroupalldata000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docdirlist000000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docdircreate0000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docdirrename0000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docdirmove000000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docdirdelete0000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	hu789rpqoi29g6tcgjuut0w9cs7zvccd	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	bp7ked9xg7qcr0xrrgds76of4a9mu4ea	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e95v8nw4je4akpcymbrhvcxiwrle8gwn	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	d2epa5csbkkk9pml4lgzttal1qupi90x	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	knisg98yampdzasuncvlqizxtn7p1e9w	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	wcloh1ihir32gyjyh475kileb31wu27q	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	r0fazcz59xrcqn2o9zso7s7lmfn3m9qn	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	mm8my37lbovupqokiplndn5pcli6q2ru	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	n0ytxdyn6yeqr29fdddddqfmtelkxkfy	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	rq42o9g1tozdqew1yv2xwkkd629hmirc	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	s19vell09rxm8ac5wobq8gp0jbif8s3e	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docdocmove000000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docdocembed00000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docdocaccess0000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docgrantget00000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docgrantsave0000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	pgmsetresolve000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	mybudgetmenu0000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	reviewminemenu00000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	reviewtodomenu00000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docmenumyspace00000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	ax0los0xmne4irua6rwqljp4e7sqgcnc	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	svwv5nl78pe01wj1995ydv087aev2sgj	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	qm6p5nkrzth8txnmtmwz9x7mb3mrgkgm	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	c1gt5b19zu5ge0gvygwiz2yd7tbgm1xd	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	vqx2hphoe8viwrxmq24h1a4u4jtg74wi	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	x0fvuaad1qd9hlyb5ne7uibbp66nohkk	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	iqxsa5k9f2j0jg3ylaqxuflzg5fuwydz	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	hj4xiz4qnky8ie8uo59sbppfklry15du	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	wbwceu6qsg78htmbqixiqm2kv66ri04v	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	lcj6og6zmrci9uht5vp0xqejcan66gkm	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	rkpal1wh3sl6bta39qqr27526rrjhm6h	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	mk7t9p2xq5r8w1s4n6y0zj2dl8scba01	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	45b37c9e9663411570679e7b5170fbbd	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	d0d2845e0c475f24035287b10bef501c	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	b77caa93b4383f2eabd6a9d92d6554d7	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	7ad5be19e1ff1b2d201d42d20b2b8ec4	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	aebf72188cf62a5ef001cce2f179be97	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	23c7ec868b212bd8e19640efd3cddf0d	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	08bd693d553e3f43a444996424d797c0	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	3471bff0aed5ac04aa0eef338faf8c53	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	a5764d2120cd8404d7fa4051f83356ff	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	35179a1f7f7cf422172987c84f4b1d4a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	d76c76816200d71574e2de97750e7222	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	2779273a4a12bbab40c0efc1e6ca2003	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	d3db2dfda00515db87376d3cda9165ee	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	4bc080ec9166fd43099de83bb865a36f	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	4bb496f3025c7ce1a0ec24538258b007	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	a4839d2c0cc885152ad9e58024c9f838	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	d5c7a36fe498b320c4be8b678e5018b6	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	453b68f9b1cd77741f3cd30bcf34557e	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	bf64c2d1d1ffa8d5bcc33e22213a0d01	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	412d3aff94b3996da16d2be7ddc927a9	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e388be201d9091b1a7c747fa863d3905	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	15b38df935279fd31a8d48e6fd2c1beb	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	7e30d74a0dd78c8fba6da7a93a413dd8	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e4d4281cba5ee1a570865c1696bf974a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	f4fd9493307ce4018c0e42234cdb9d5d	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	ca4b51d526379dfa58f131bcbe8a504b	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	da5007870676312ec5f6770b077559cc	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	f3dd20808249d088269157d37897ca2e	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	c76af0b8f4813fea41b45eac6f046baf	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e3b46495390b7dc31da1e80dfc06dccc	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	c1b8876c5e5b8f5ce0e282b5b818b21c	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	f4e91e7f7f8cc775d9e00758448354ed	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	c6740491fa9a97b71e1496b626d9848b	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	dc5ab2295abb3ce7b68c58e97f13013d	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	b1add1fd4cd612c770607b848a37247c	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	dec3380809207336b8aa9541993077dc	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	bfe02cff06bf7c07e81849bc42c1bdad	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	a2df459b1d4b3dcc0100fa138ccb228f	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e526c7748f1630b2de553da74f57b73b	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	d5afeac0b75f392429b62ffb874ec14d	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	fbb858d5f66b60a4a1fc3fc160867c9f	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	d3fa0b1904a4bc4f89fa38333c241ebd	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	ac4b25507a71f8b1d732f19b0967381a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	c2b3be3ea2e62a1bbd936e9c60afa64c	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e5f6e852cf1af738d4218692672dd38c	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	efb22631da5db131ae7e3362ccf762ea	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	c058f38f60e5fdd744179c987c1446ab	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	b3d6b62939c0e3560e4e2886750b87dc	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	a1b2c3d4e5f64780901234567890abcd	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	b1c2d3e4f5a64780901234567890abcd	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	d1e2f3a4b5c64780901234567890abcd	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1f2a3b4c5d64780901234567890abcd	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	f1a2b3c4d5e64780901234567890abcd	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	11a2b3c4d5e64780901234567890abcd	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	12a2b3c4d5e64780901234567890abcd	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	13a2b3c4d5e64780901234567890abcd	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	31a2b3c4d5e64780901234567890abcd	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	32a2b3c4d5e64780901234567890abcd	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	33a2b3c4d5e64780901234567890abcd	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	41a2b3c4d5e64780901234567890abcd	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	42a2b3c4d5e64780901234567890abcd	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	43a2b3c4d5e64780901234567890abcd	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a301	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a302	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a303	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a304	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a305	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a306	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a307	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a308	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a309	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a30a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a30b	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a30c	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a30d	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a30e	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a30f	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a310	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a311	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a312	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a313	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a314	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a315	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a316	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a317	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a318	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenutopdir00000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenucfg00000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenustatus00000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenudevice00000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenusave0000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenuversave0000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenuverdel00000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenudevrevoke00000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenudevblock00000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenudevdel00000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenupackage00000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenupkgfetch0000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenupkgdel0000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenudevcmd0000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenudevcmdhist0000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	wqia8s3qkidtd9m4oyerq1ua1ira106j	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	ef7j2ajpsl7ic1837eob7j24kju6rmsc	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	nf5ulvetty6q43d2g3u112954dg8ghur	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	ug2ln624tbfor1s59tqqlh445bgv57rh	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	s8ivn8synch7tjh0r4w6dqbf8wkid0lb	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	zpp19f26wnrkjoy1191ls6ihwg047czm	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	ag8znpm5eoyq8ch3ne219aq891qi2bxd	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	r1ypvqqxtdbzj5rga9te9vkc6kpguh8n	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	ith4r52j2f1evx60bdx7mfiapkz7ro0z	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	p0mm8y53uvbddxu9d4mbv7vdxds8zn3j	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	abzl156x38kfe3ldmgzg77xqfu111kai	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	jkh7839a82iwlobkd7zneeb9vxemmm4m	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	x9jenr58jy0izxbi9igj2e4vq4j1iiue	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	ssr8g284gyvjxqrkqbd3uiqel4e6hn1l	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	ocl5el43or45szhej29qcl3p2bgfyygh	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	ndtlv64qxrdvyvq49zzdzxo12lmaielk	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	k09pl2pm2h1herb2e4ilol3ykpmyjruj	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kkul86qh6bup47y7z4or739tgmhcmvhp	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	jcx0kljb4s0hugnmvya0flxpjutrnoje	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	bpkbouf0j5b05no95ihmvb0v1h9kqaie	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	f7s21n5ohvqrxxohjnaf7dylari6p8ph	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	o31bcxp2wtzbl90d56szmvpfr5t4rb9w	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	ar0sgu9sy675xwhgjhpo61pm9yo1shzd	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	qi565hgt51dz7gk4bh03ppi8krzyp3nh	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	vggck2olb8t3irdz3skcc90nc0ez025o	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	dg1dbh1mq8p3m4uz8sn7k1yjuak8x69d	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	nwubrkn0pzsetx9tipmwmki29xu8w3bs	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	lan544jk8jb15vtznpcsaji1us6pcqcv	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	eqzht2kiegp7mv853ps6iabtfiio1hxu	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	yorvn6hf6plcy19s9kbqqj7l8s62nn4t	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	rllioozbsqprtybcg3qllx7xpbrgim4c	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	usldtmxppalaa59lpr026ohw7ektrmul	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	q5gwyb22qfjhzlevszccwhanvfqtilrw	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	ad968wt7y61z0mrd33wqfw5hie4dyd8y	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	nj7bkiz33ef7u4otfhjiv03xwqyjb6mv	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenuappperm00000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenuapppermsave0000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenuapppermappsave0000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenuapppermappdel00000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenudevskillinst00000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenudevskilluninst0000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a319	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a320	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a1a321	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenudevmcplist0000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenudevmcpbind0000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	clientmenudevmcpunbind00000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docmenudir000000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docmenulist00000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docmenuupload000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docmenuupdate000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docmenuedit00000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docmenudelete000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docmenuget000000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docmenucontent00000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docmenusearch000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	docmenuraw000000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	pgmenumain000000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	pgmgrouplist0000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	pgmgroupadd00000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	pgmgroupedit0000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	pgmgroupdelete00000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	pgmsetlist000000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	pgmsetadd0000000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	pgmsetedit000000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	pgmsetdelete0000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbmenudir000000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbmenulist00000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbmenuupload000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbmenuupdate000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbmenuedit00000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbmenudelete000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbmenuget000000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbmenucontent00000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbmenuraw000000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbmenusearch000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbdirlist000000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbdircreate0000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbdirrename0000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbdirmove000000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbdirdelete0000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbdocmove000000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbdocembed00000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbdocaccess0000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbgrantget00000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbgrantsave0000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	8851690a4edbed97a940c77af995cd1f	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	be59ac10edcefe47862fe4a317b10bc4	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	06a9f5d9fdf67e7053c3e10bc85a2a11	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	daef5f2b0b8bcef606a61689f5d0e59d	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	schedmenudir0000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	schedmenumy00000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	schedmenuroom000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	schedmenuroommanage000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	schedmenueventadd00000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	schedmenueventedit0000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	schedmenueventdel00000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	schedmenueventexc00000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	schedmenurespond000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	schedmenuroomadd000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	schedmenuroomedit00000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	schedmenuroomdel000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	e1a1a1a1a1a1a1a1a1a1a1a1a322	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbdslistmenu0000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbdsbtget0000000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbdsbtadd0000000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbdsbtedit000000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbdsbtdel0000000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbdsbtcancel0000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbdsbttrigger000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	kbdsbtruns000000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	mcpoauthlistmenu000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	mcpoauthbtadd000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	mcpoauthbtedit00000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	mcpoauthbtdel000000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	mcpoauthbtrotate000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	mcpoauthtoklist0000000000000000a	All	g		
p	ghj0ywa1u5dtd52le2xhzui2qtgnj8ec	mcpoauthtokrevoke00000000000000a	All	g		
\.


--
-- Data for Name: aa_user; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_user (id, user_id, user_name, employee_no, name, mobile, user_nickname, birthday, user_password, user_salt, user_status, user_email, sex, avatar, dept_id, remark, is_admin, address, describe, last_login_ip, last_login_time, created_at, updated_at, deleted_at) FROM stdin;
2	i47dhcwyx2rulbjl5vlab89orpamongb	user001	EMP00002	测试用户001	13910000001	测试用户001	0	9b1d4744de72012b9390a000e6a8070a	z6kEm52pMp	1	user001@aiarsenal.cc	1		lvyrsk0fjk7br36yh0ti1fug3c8oewsz		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
3	awaoolltywg611faqbrfjs0a6qq7e7mq	user002	EMP00003	测试用户002	13910000002	测试用户002	0	bd4409a545ef2bbb7379ca8bbc5df1c1	YUJkVpBgoq	1	user002@aiarsenal.cc	2		yvd2rzcq3p6on80czv6q7955j7gc5y3t		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
4	ttozve8tomu4bw7q64iqch473vic8xjx	user003	EMP00004	测试用户003	13910000003	测试用户003	0	6efd2878e397e232d1a8f1613f6c5224	6vzTnE2ajm	1	user003@aiarsenal.cc	0		oyy8hxpdfp1b5blfpq8or6tk15hvhwwk		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
5	l7v7hxinitk68gtbwp1ygd1ud39q88bt	user004	EMP00005	测试用户004	13910000004	测试用户004	0	b864c0029dc2f3f6c68cfe3a5997939e	Q0yMQMGbWO	1	user004@aiarsenal.cc	1		g84a2esunq3ggigh6rie4iso5mvqejoc		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
6	g1zzvvdxqrpmzk2kw8q4ky3a26e6qfxv	user005	EMP00006	测试用户005	13910000005	测试用户005	0	51388aa574753acecd285f7ba054d681	WqwfW8MGX6	1	user005@aiarsenal.cc	2		nfnkkl3va2fdvuk9fwmiz21y15xpl8lh		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
7	wcsj3t9d38o5o5ryzp9pl0s72bbolqje	user006	EMP00007	测试用户006	13910000006	测试用户006	0	3409386721420e4030fa2a8bec68853d	hUSQIl8jD6	1	user006@aiarsenal.cc	0		v5r1ilh98qyl0kwnsrfr79whbyy1twzf		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
8	psnpiaisc4utvtowod2cismxnen0sqji	user007	EMP00008	测试用户007	13910000007	测试用户007	0	f362f0430b3f9fc0c64cd9db15534c65	zk968uweGR	1	user007@aiarsenal.cc	1		fnrcvzwdge0dwbwjaeyilydchrlmff9g		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
9	yftk7uwwhy2oxmrzkq3tp4dh89bch1jb	user008	EMP00009	测试用户008	13910000008	测试用户008	0	24efc3f750075c93a45ab3a906ae6285	8APZ6BJdEB	1	user008@aiarsenal.cc	2		n7c0xt6ksdqwia6gjrtebatzlzz4c2tb		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
10	ey81ndf6zpobmaoxqiquhofjpu02z8ht	user009	EMP00010	测试用户009	13910000009	测试用户009	0	d71375bcdc58fd05979bb3d5f0047bed	V4bse9bK3H	1	user009@aiarsenal.cc	0		qdei4bqdhl2s48229y716b7zmyl0yhov		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
11	hhhnxqvutljctsgaoo780vo563bez55a	user010	EMP00011	测试用户010	13910000010	测试用户010	0	450ccee18571704fe1faaec5715d7b27	pzQapNDT2q	1	user010@aiarsenal.cc	1		gzbhisqghr688sew022riq0m9ph1tnul		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
12	xdrjswzjeydx1v5ge05jltxd0ixg0yya	user011	EMP00012	测试用户011	13910000011	测试用户011	0	b81670ff700f90ed127f42654542fc8c	FZ1JVZu5cg	1	user011@aiarsenal.cc	2		lcx6976nybie9legnbf7efda17gzms8p		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
13	qqzy7xff86q8kfmyopjq5dmgbwf03lkd	user012	EMP00013	测试用户012	13910000012	测试用户012	0	f04b8e08f31eebab0b1223f0dbd69f66	91v3TPoiEk	1	user012@aiarsenal.cc	0		tuon2vwblsnov4mnykch7sypzbvwgqma		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
14	hwogmz6ku42vzbxo7l4g9seiftxmf55z	user013	EMP00014	测试用户013	13910000013	测试用户013	0	e923a73b9b64dfb6827e6bb0d7ed5432	Cy2cM9MWjF	1	user013@aiarsenal.cc	1		v82stxipz7d1bdq1iz4ovthbs019tphi		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
15	vxxrlq1k4nmwnbuiud4z8g4zdn6brnom	user014	EMP00015	测试用户014	13910000014	测试用户014	0	c51a534b0b28d5f77bad060190e00266	RzZKHhVFJJ	1	user014@aiarsenal.cc	2		kheqge89r0euopogj346fyg25m8t3rya		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
16	t8ubxdsaiuixyj57rt57vhrggh6yejco	user015	EMP00016	测试用户015	13910000015	测试用户015	0	a45100773255a1ce138b596af9b99c1d	w5fkk08OMa	1	user015@aiarsenal.cc	0		fop8s0srp4a6or6ceam5nyp6632unzbg		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
17	huoc4rxc0p8g6f4cswatmogr6daqfxgl	user016	EMP00017	测试用户016	13910000016	测试用户016	0	4ee28b5bef5e2717725e44f5581ee410	71R6YZgK3C	1	user016@aiarsenal.cc	1		hbxawf9h3d7xkyjmbw4uqeon4i3nfn2w		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
18	yv6887f2ne6v8cwx0shm2ty97nbv4vme	user017	EMP00018	测试用户017	13910000017	测试用户017	0	86b0a8ec0e73d449c2187f2ba0196521	k6HYFmCrWk	1	user017@aiarsenal.cc	2		ivgstgmxznc1erf0sxvtxjqfikq7wazs		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
19	cz5dv5ogq1ifcqmsempf00hqx4w9e7fr	user018	EMP00019	测试用户018	13910000018	测试用户018	0	bc3e5cce235d694b05496516fd9f8de6	uGY3kBxc4J	1	user018@aiarsenal.cc	0		kye5pyfqoq5doipzenotptllr2g6mxqg		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
20	ddv2o2ggneyz5omf2hsakq5gq5nb37kp	user019	EMP00020	测试用户019	13910000019	测试用户019	0	919e697e494d8d5b822b601def499cf9	6idosZzNt9	1	user019@aiarsenal.cc	1		zujehiadtnjaqtgooixca2iqp9safihi		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
21	iozdijm6umoh07mwmf9shtyb3gkm6b2e	user020	EMP00021	测试用户020	13910000020	测试用户020	0	2098ae4f2c91d3b2fa00e8486b5ed11a	eLIqrmVWTp	1	user020@aiarsenal.cc	2		eow0qt4tkj5qw2k56wi30oupnb3yct4m		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
22	xkbw9qo0volhjszdrgr11ieh9qmy05xm	user021	EMP00022	测试用户021	13910000021	测试用户021	0	9e42b5ea076cd595d6309c6ee2808feb	21eIfppwZb	1	user021@aiarsenal.cc	0		eleqfwc2744a9ozwn5ndpayuuw7c4zqy		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
23	ojmwz7cbldpxwnmu4h9ed5gzjm8yglbj	user022	EMP00023	测试用户022	13910000022	测试用户022	0	ad53fcc9c31ec0780f2383d085134b5f	3ePZt6vwhc	1	user022@aiarsenal.cc	1		llxllcao3wked14skgm1lb5i7p11a5jt		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
24	ouvzpmuuf8vmoja8g7ua9x1a6m83dkqe	user023	EMP00024	测试用户023	13910000023	测试用户023	0	8857e0a253c40fe58d065781ff487e6e	3MujzyroRE	1	user023@aiarsenal.cc	2		cyrj194puwursqce38vm471isql0qc4j		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
25	f571jz7rptodss6igenpqkbdv9s8nhgq	user024	EMP00025	测试用户024	13910000024	测试用户024	0	1fe06ecc2f9a2e81105bf0a8e927e82c	yfsd5styKR	1	user024@aiarsenal.cc	0		m2hu15ojpus1layz4k4zhifi9cj8erbt		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
26	nn5sxc1va1wbi7c3y8nj3si1tznkil1o	user025	EMP00026	测试用户025	13910000025	测试用户025	0	4f594bbf4c291c77601829942cac4a96	GYgdqZi99n	1	user025@aiarsenal.cc	1		b52vpvw2wompdleclbp5467pkw017oab		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
27	rjtjyaegy2r498hswx8htls1ncqz2shp	user026	EMP00027	测试用户026	13910000026	测试用户026	0	8edf37a915d49f6cde3ed279989e80c0	kEx6LJtYIM	1	user026@aiarsenal.cc	2		gb2wz5r8etns12cgiu4pi627n2sqqlfw		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
28	o9ryfxtbs7mz7d1pitt1ji1eyofbuvhl	user027	EMP00028	测试用户027	13910000027	测试用户027	0	98db4990e1b00bbd5ffa19101c6290b5	jDcuTq7ozs	1	user027@aiarsenal.cc	0		tm1yja3iahb1f4v3bhc7q7amvcu9crhk		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
29	v8nyq4ekvn96ffe69fpteqprfbyw7d5o	user028	EMP00029	测试用户028	13910000028	测试用户028	0	7f11e54100bd766045f3c653e3718c16	Uo3AqHFJo1	1	user028@aiarsenal.cc	1		tekf85gn813frzxhmyy3yaj6j0x4e1fo		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
30	ccp94mq5kpf7del1n8kixhght9hoc74f	user029	EMP00030	测试用户029	13910000029	测试用户029	0	b297df32fffc0743f67490451424fcac	dcjmCpOe62	1	user029@aiarsenal.cc	2		n59qw5rkvekax5p3m1ahoxmf1htfy42k		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
31	i5jq4vt4wk4pf3lb4nx2fh4339wbfz0y	user030	EMP00031	测试用户030	13910000030	测试用户030	0	5c3bb8779c2d5358406ca38642bcc7c3	bYpZ909avt	1	user030@aiarsenal.cc	0		hwjzw8gpww9l4eqrvgfr729lk2o3lvzf		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
32	h30ls46rppecsk1di4n5zxlz7xpcps5i	user031	EMP00032	测试用户031	13910000031	测试用户031	0	6a69b42370247373bd6f40df753377b1	mlk3c0S92B	1	user031@aiarsenal.cc	1		oouqcfyiwgzfx9m1lcm6fx21ef3jzhkl		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
33	g4dp1tsgez5vvn1a2hq7vpgrireuwr3f	user032	EMP00033	测试用户032	13910000032	测试用户032	0	6b543f2e7299cf429d4975189b814d12	43bR34SUpA	1	user032@aiarsenal.cc	2		jxtny36vgipqmo8dorsquvfonx2bitdm		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
34	xh9e2z238dalqjxse0gdlrteaf49325f	user033	EMP00034	测试用户033	13910000033	测试用户033	0	47cd2c50b52f30dfd1fef36e362fad2b	4hBrlzcvWs	1	user033@aiarsenal.cc	0		p993upszgt1evrgcpxv7f910ixu2sbgs		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
35	p6k1ds5468pmzbz1i6v4814lt5475m3m	user034	EMP00035	测试用户034	13910000034	测试用户034	0	446e6b8766100fd708743ab7a2387d3b	uJzT4RVSWy	1	user034@aiarsenal.cc	1		azohlyb3og2beqrj9kpn96sakc44dadf		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
36	nxbyzuyiu6ouyygcuzemmac873aljn3j	user035	EMP00036	测试用户035	13910000035	测试用户035	0	b56f99cb03f12921f3a4b004f5419c5e	ctbzcGNJUA	1	user035@aiarsenal.cc	2		aeqjdjfs1wyl9p93u8bayg352ztdm7ya		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
37	f386t32hk8sghcnvf24g3st3i41phl9o	user036	EMP00037	测试用户036	13910000036	测试用户036	0	3cf27f827a5d762cdc03d1881901de9e	J7d6lCF6KZ	1	user036@aiarsenal.cc	0		zclzz8og8qqe6kebyzszab5g2ufrigzm		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
38	qu9owum1ui0elni61b5i29akyl7jt03p	user037	EMP00038	测试用户037	13910000037	测试用户037	0	8171dc855b96ce240e17c25a5bdd8b6f	tDugPpyeQb	1	user037@aiarsenal.cc	1		kdipud5mow01mfleoyegyrf9nzlh0omp		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
39	b7qp8p0xj1okbz6vxrmzngd32jsonytp	user038	EMP00039	测试用户038	13910000038	测试用户038	0	735b122cdb50fcee8aa8ce28b6c30d57	mOUAetSJ7M	1	user038@aiarsenal.cc	2		c4psurjud21mvvvpmuelaf1rt7gfjnno		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
40	qh1et5ylq2fm1e82d32fl67gvnqo6riq	user039	EMP00040	测试用户039	13910000039	测试用户039	0	03fb28474fbdd20be2e848cbdf80b9ae	WhdSEsjchL	1	user039@aiarsenal.cc	0		lhq17wpyf7oh04funia9ki4t2tblbq6r		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
41	tyrhx8gab3hzay0xmz65aia5j5xzjcuo	user040	EMP00041	测试用户040	13910000040	测试用户040	0	b1fddeea54324296d7834891628d262f	r9A9OJvyF6	1	user040@aiarsenal.cc	1		pw2cp1u6l37m0h9vrnlgx8aq4ddnelxi		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
42	wlkajsoyh27oscyja111i6bq5e5cd5gx	user041	EMP00042	测试用户041	13910000041	测试用户041	0	d36df18e1a21e84cca845363ec2ee64d	nQddHT2S4b	1	user041@aiarsenal.cc	2		bgnoctfgtiezihbpp2xjmzh55134eykr		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
43	ki2q6fgdt8o56u3vn2ckanvoy34qqq9a	user042	EMP00043	测试用户042	13910000042	测试用户042	0	fb30b5fdcd65bf32de07f1ad95054ffd	6BPK5fJY9M	1	user042@aiarsenal.cc	0		jqtjxsygdgv7btkrb6sfau7fp8coeqgz		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
44	lsinkrec95cl8xk41pc1xa0nwp77r7nq	user043	EMP00044	测试用户043	13910000043	测试用户043	0	9e10eabc7b5aff0c45e7b30de330576e	RDTxwgMLxm	1	user043@aiarsenal.cc	1		megotp94fof0d26f3g6zj3xg2lgsuwut		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
45	u7keg12zsf1fp6wxnd0w78r03lk9p5ne	user044	EMP00045	测试用户044	13910000044	测试用户044	0	09711560bbeb35e532741eeed537abdf	8MWEPCAUUf	1	user044@aiarsenal.cc	2		r7haexv1ewuo2e40uh4tnx5zxxmqnfns		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
46	vj9e202tchk0m7ar1pg1w2xvcb52770u	user045	EMP00046	测试用户045	13910000045	测试用户045	0	c5e5a2775588818fdd149fb4b5142c39	LRzOp1kb4P	1	user045@aiarsenal.cc	0		u1ai4dx2jxsimrdtb0q22n1p6ie71z8b		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
47	bdlps6p632yb96o44b00s23l1eywr0sy	user046	EMP00047	测试用户046	13910000046	测试用户046	0	c3aac25c7b6b79dc652e82ed354b231a	DNN63VvJWw	1	user046@aiarsenal.cc	1		in2z18l0hxz17jljs3h3qckjtq7q1kti		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
48	fd5893szdczhr9ylo1cwbrsbb7f6s0np	user047	EMP00048	测试用户047	13910000047	测试用户047	0	afa95ff8cb26301dddb4fe71be44aacc	nFE9TfPX5r	1	user047@aiarsenal.cc	2		r650hfoqzv599kagklh6f836tto2rbbw		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
49	fisec94xg1zmsoe73xqi8s7pcioxs2xy	user048	EMP00049	测试用户048	13910000048	测试用户048	0	ba87406a4f5ea9cdbd562e23be382f2c	bMmU7Pji8y	1	user048@aiarsenal.cc	0		gj2z499cescrl02ierj1r3hhwzqwlqjo		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
50	qjnllgil3gdfowp1khpin6o2b6egz7vs	user049	EMP00050	测试用户049	13910000049	测试用户049	0	c143f767cd9c496784bc010dd9f11586	zLazGKrlF2	1	user049@aiarsenal.cc	1		w5uych9iovhs7ytww2xyni9a3gvn9f9p		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
51	et5k755mj9ndmzcz5uqj79o0se8a7hjq	user050	EMP00051	测试用户050	13910000050	测试用户050	0	4e7aa6392ef8984bd8643fe984edbde0	EvhUEjQItI	1	user050@aiarsenal.cc	2		r836v7gwal4mjozdm21vu2vhvp5tjoax		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
52	z1cnqvbthuj6j2ocm19p5wivwknkwwxb	user051	EMP00052	测试用户051	13910000051	测试用户051	0	028b9fcdd85a8126fc98e71a09da0878	W7jN03xcLr	1	user051@aiarsenal.cc	0		kzd9e9wccd2d1fyonp6ucvani848uqjq		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
53	fgyo9wgof0kcalr2r8cskc2g0bejiw4r	user052	EMP00053	测试用户052	13910000052	测试用户052	0	f7c134982a4e059681f09a2707627269	Sps29ftxlx	1	user052@aiarsenal.cc	1		i1qwazesxbz321eroqn5o0y0i6ri5ucq		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
54	zzye39s02viahffg8futr2nq2m8ahw9v	user053	EMP00054	测试用户053	13910000053	测试用户053	0	8830110501941e314a91c4b2e1554e3c	fDbDrrwHvv	1	user053@aiarsenal.cc	2		huo1n8eyycgqr41cqhjoq3k3f5t4iapq		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
55	nwesstx94ue6ekr832lgdj7zz1r81yol	user054	EMP00055	测试用户054	13910000054	测试用户054	0	b2f900c959eb98ea4ce8eb01f513bf06	fXgfnYSYwI	1	user054@aiarsenal.cc	0		mot73avfyez5gquc16nke4g4j6zmwrru		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
56	fczzjf2lns76m9avq50s302n8bfv6kxy	user055	EMP00056	测试用户055	13910000055	测试用户055	0	27a631e2edf5021e95e40bff1a8ebf44	H3c5MF7EhA	1	user055@aiarsenal.cc	1		xdt0wohr8eddgcpxfenx48iekck2rlyn		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
57	ljkgwn4eowlvuh9aafzdc0ykhj06ptpw	user056	EMP00057	测试用户056	13910000056	测试用户056	0	7123235a544aaf3d164fcd873db5d635	PlpsEBmy6T	1	user056@aiarsenal.cc	2		vutmnr0p3uj6cwntn6x0fdypeckea2dp		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
58	rbfrd3phrnqazpxz83q61q87t3s6znun	user057	EMP00058	测试用户057	13910000057	测试用户057	0	62e65dd188eca08ec64c12cf143819ff	MhbquN6iKj	1	user057@aiarsenal.cc	0		y8ckjvpi1fzt332kbs779wdylwwf919d		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
59	qdroqjw2xx7ckth36lsw9lfmexyf1fbt	user058	EMP00059	测试用户058	13910000058	测试用户058	0	7b93e4ac5c2c75723411f15fe6f774e5	tqbTiDe5bJ	1	user058@aiarsenal.cc	1		yyxgogstqncjk8aw1bka8ww8ql78t4oy		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
60	nadp1hsoga0hqcw16pynzk19h7ehcj5r	user059	EMP00060	测试用户059	13910000059	测试用户059	0	354e5f2bdd79eb23e5cfb6306cb14ef0	HLPvs64Bzv	1	user059@aiarsenal.cc	2		aazaaxi3fj8ioshkfzfwd52wqn2bdvaq		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
61	l5iu02zzxae7gw6denbs5m430pnakwco	user060	EMP00061	测试用户060	13910000060	测试用户060	0	398fbe0a72985f0c007b6772685ab1d5	S8QThD4Q0M	1	user060@aiarsenal.cc	0		rtcw7nc9vixl5y6u38ejdk6sisbngq4j		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
62	g5zzcwa6lljcra9bh95ot0wmdbo8bb1v	user061	EMP00062	测试用户061	13910000061	测试用户061	0	f31732efd1e1c61a910c4bf7187fee8f	GMUobwhnon	1	user061@aiarsenal.cc	1		td7m81oe400y1y047drts97ljd1p9gdo		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
63	idg18sc5jlnnbs02myk619oj5s8kstej	user062	EMP00063	测试用户062	13910000062	测试用户062	0	7505a80e7e31187d15352292cfb41c4a	dAtVvXHNdT	1	user062@aiarsenal.cc	2		kzru6bn2yy8rmbss3l16blzo04uzf3qc		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
64	ap2xvhdaf6feytr8bbfa0x563uuoztsm	user063	EMP00064	测试用户063	13910000063	测试用户063	0	46b823942d4504b553c0be06698dd29f	1TQtfZirpo	1	user063@aiarsenal.cc	0		ec0d5a12u547oegklqcfbx1ajrmhnhna		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
65	aqjsus7aopdpz778evymkbst0mwg6s4l	user064	EMP00065	测试用户064	13910000064	测试用户064	0	975704d74fbee62de7632fd6d4cae74f	bFevfiu9Xd	1	user064@aiarsenal.cc	1		m12ho3wliz43udlq3dhwmc6nu3wz9arh		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
66	y9i1zy0zq9jtjx57eleglzfvurltdf9v	user065	EMP00066	测试用户065	13910000065	测试用户065	0	5012dbe4cd01326a596deeec88f76161	6CPcFsAfcF	1	user065@aiarsenal.cc	2		ejt8axdoxjkpq2er6gk12yxfp6qsyvyh		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
67	swy9yjech3it00fnpuk9m5t3ww50f0zu	user066	EMP00067	测试用户066	13910000066	测试用户066	0	4514a0e031fb93a87dda878349f0350f	rdhUvkmsFp	1	user066@aiarsenal.cc	0		sj8j1hw1likzmdbk4etbrqk5t6pnylle		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
68	kp0b2zujdinw3quweijsa8k1fn5jo29l	user067	EMP00068	测试用户067	13910000067	测试用户067	0	b753b9002826e54f2b661b6a31eb9259	lDUnLSLANU	1	user067@aiarsenal.cc	1		vmxjerxj1mbh16efg08wykyqsw2ankvp		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
69	t6bh2x83k6hg6haogt4loyl3dw283d1d	user068	EMP00069	测试用户068	13910000068	测试用户068	0	eaf547ba20c63c712099709b45056269	TlwKCsjsUs	1	user068@aiarsenal.cc	2		cqu8wn24karw76b2d2kqluxoniq77mnh		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
70	nk12a6f4keadmc12tfw69abqnb0hco5d	user069	EMP00070	测试用户069	13910000069	测试用户069	0	516e3c6bfe15431d96c8333f5ab8fd5e	zPnVEXa8ep	1	user069@aiarsenal.cc	0		g1crjufoco9y6ylxn4tgaq5l13mn0isz		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
71	zqb590opd1gatrzyadk6xax7e7bx500y	user070	EMP00071	测试用户070	13910000070	测试用户070	0	af03d126a57a1d595afc45e06bdfc2ae	pUxlDB9ZqN	1	user070@aiarsenal.cc	1		d3etjyk6js9jy8c0thmloppmoabyyxli		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
72	hyztdi4uhb1w7ndfi47zpp1y5m16soai	user071	EMP00072	测试用户071	13910000071	测试用户071	0	fabc9cda703636f458c6b153fc1dbf02	6tXSukPyGq	1	user071@aiarsenal.cc	2		q706ie5vt4qhfp6kl55fhwa7619x3ljj		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
73	kb38c9j616dragwwgbyo8p7j4vk1xnub	user072	EMP00073	测试用户072	13910000072	测试用户072	0	72b36decc3effd8eb079d5e84a9eb4ee	opd2xQbMkq	1	user072@aiarsenal.cc	0		enc3p7e6phadqkpaoukpxn3qxaeojfou		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
74	b0f6pm4d64m6zz6agzg7ff1y80kb112t	user073	EMP00074	测试用户073	13910000073	测试用户073	0	2a741e0365bbe5aa749a7feeb3b13527	ZPKrQbEOl6	1	user073@aiarsenal.cc	1		r2ss4wkobo5w325niausqmror2ny4vpb		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
75	hhqf1lr8ktqflwomy9ywlr3e2g87c1nr	user074	EMP00075	测试用户074	13910000074	测试用户074	0	6492a8e232d85cd0f787d34bc1c955d5	cYfmbYAVZo	1	user074@aiarsenal.cc	2		lvyrsk0fjk7br36yh0ti1fug3c8oewsz		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
76	vfuzzdizy3ladqg0f9bwb22csxl2b8zg	user075	EMP00076	测试用户075	13910000075	测试用户075	0	f070ddf18b4048b085f04318a77548a1	RlKELGOPQI	1	user075@aiarsenal.cc	0		yvd2rzcq3p6on80czv6q7955j7gc5y3t		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
77	bred3yjuutw1vt7mlwx4259tlez2hyvt	user076	EMP00077	测试用户076	13910000076	测试用户076	0	330646a15b94e5993c8ce06d66e12849	wS6wpaOU64	1	user076@aiarsenal.cc	1		oyy8hxpdfp1b5blfpq8or6tk15hvhwwk		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
78	o109vwi25ey3ru4726gkeedt6yed15po	user077	EMP00078	测试用户077	13910000077	测试用户077	0	ab780dd410f13771eed8396005858f9e	ZnpeIRpYBt	1	user077@aiarsenal.cc	2		g84a2esunq3ggigh6rie4iso5mvqejoc		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
79	qpsr1il81pv2rpwsygkudq6ec28li8ee	user078	EMP00079	测试用户078	13910000078	测试用户078	0	2e6fe02f18d3f0f8f23b26890aa692fc	BLnUlau3X7	1	user078@aiarsenal.cc	0		nfnkkl3va2fdvuk9fwmiz21y15xpl8lh		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
80	quguougzmiaoxn29p4jm7vfza0nu2mqn	user079	EMP00080	测试用户079	13910000079	测试用户079	0	83b5fb67bb7ce0368c7a1576548c4d36	EOHBUzNT2A	1	user079@aiarsenal.cc	1		v5r1ilh98qyl0kwnsrfr79whbyy1twzf		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
81	vbsw1wfxo4xiylrqp4ytlvb0iomct94u	user080	EMP00081	测试用户080	13910000080	测试用户080	0	508f1d50cc1419035f8926bcba656f2a	2rMGWbq5Ei	1	user080@aiarsenal.cc	2		fnrcvzwdge0dwbwjaeyilydchrlmff9g		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
82	rhdnuq6wo64neftkd68yny35yste2xtx	user081	EMP00082	测试用户081	13910000081	测试用户081	0	a94220c56a81ceaf26e56654ed85a542	kjbvUSw4Or	1	user081@aiarsenal.cc	0		n7c0xt6ksdqwia6gjrtebatzlzz4c2tb		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
83	p6m3p05a3n48hsaq294d431wybsq07zr	user082	EMP00083	测试用户082	13910000082	测试用户082	0	e4d4f34bc70addd56755be1d9138e00a	4JD0e4I1WE	1	user082@aiarsenal.cc	1		qdei4bqdhl2s48229y716b7zmyl0yhov		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
84	z5o2otfmasx8ylqa1iolk9z530n08opl	user083	EMP00084	测试用户083	13910000083	测试用户083	0	65364eb24d41da4067cbe65b5c94a35a	MmyPbmjIwy	1	user083@aiarsenal.cc	2		gzbhisqghr688sew022riq0m9ph1tnul		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
85	fxufj8dsgmh7hwzc1zgbgoiptbr9smex	user084	EMP00085	测试用户084	13910000084	测试用户084	0	c5968ab0d40fc233d505989a1dc5c8c6	OMqA49es8s	1	user084@aiarsenal.cc	0		lcx6976nybie9legnbf7efda17gzms8p		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
86	fyqkcyecpap2uqzaocs8q8t8x3cvmvfq	user085	EMP00086	测试用户085	13910000085	测试用户085	0	d40b78b1767a17158e79da8b71b4aaaf	tQfTelwEjT	1	user085@aiarsenal.cc	1		tuon2vwblsnov4mnykch7sypzbvwgqma		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
87	v32y4lq3ljmqs3qgv8onxtk16gnutgri	user086	EMP00087	测试用户086	13910000086	测试用户086	0	dc8c10bebb1b995e351d30aafd954a1a	1sjUlpjIRn	1	user086@aiarsenal.cc	2		v82stxipz7d1bdq1iz4ovthbs019tphi		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
88	y88icjiuzvqdnpjuh4lype9xt97a3xxr	user087	EMP00088	测试用户087	13910000087	测试用户087	0	68f8b636e1e6a4f84d245b8e32ada754	spc6M7e1BL	1	user087@aiarsenal.cc	0		kheqge89r0euopogj346fyg25m8t3rya		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
89	smg1dhkia9fg7hjuycfk8449fz2dzj2a	user088	EMP00089	测试用户088	13910000088	测试用户088	0	e0f498d13ba31badfd7877e470c182b0	jpIVsdIafs	1	user088@aiarsenal.cc	1		fop8s0srp4a6or6ceam5nyp6632unzbg		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
90	so85b77vby7plhnvr6pf17c98b9mf0to	user089	EMP00090	测试用户089	13910000089	测试用户089	0	f396e7265a829c2fa9dd08608a0db8ea	vnYXWthqUr	1	user089@aiarsenal.cc	2		hbxawf9h3d7xkyjmbw4uqeon4i3nfn2w		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
91	r8144ojgas0uiprldnk21ckrbw6z9kgy	user090	EMP00091	测试用户090	13910000090	测试用户090	0	70d34d6320227564024804f11bec06d7	OMeMSCWoeG	1	user090@aiarsenal.cc	0		ivgstgmxznc1erf0sxvtxjqfikq7wazs		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
92	tdunjhhjpzmmiw9eu8a9shulxqbfp8ec	user091	EMP00092	测试用户091	13910000091	测试用户091	0	6830c9b0380e9098e9ce7adb9cbc186a	oXT0B7utQO	1	user091@aiarsenal.cc	1		kye5pyfqoq5doipzenotptllr2g6mxqg		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
93	mxlhyiou8vg6szccawm0zmpug4gg5xrb	user092	EMP00093	测试用户092	13910000092	测试用户092	0	925ff0ce2840c36b73a5033774e2310f	tFAGjPxjXv	1	user092@aiarsenal.cc	2		zujehiadtnjaqtgooixca2iqp9safihi		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
94	nq2gkeoricp2ufl5ksvv6pznixkj7ifz	user093	EMP00094	测试用户093	13910000093	测试用户093	0	715e8f50bfd2e2dc0c03d5d1425e70ca	wF1IZZLPJZ	1	user093@aiarsenal.cc	0		eow0qt4tkj5qw2k56wi30oupnb3yct4m		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
95	hnvmzuzx6jup48fa2ow28va7vmjmipua	user094	EMP00095	测试用户094	13910000094	测试用户094	0	4bfe5312a19506673d7a227754526812	HL2OZY2cJ7	1	user094@aiarsenal.cc	1		eleqfwc2744a9ozwn5ndpayuuw7c4zqy		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
96	eieus50pzqqvzj1ge6l72zoina4v91gy	user095	EMP00096	测试用户095	13910000095	测试用户095	0	de71b5f57151c9d8009deba8f407f66d	Q7la6HawOs	1	user095@aiarsenal.cc	2		llxllcao3wked14skgm1lb5i7p11a5jt		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
97	rump0avc7t7htk5y52r61h7n1l90jv9f	user096	EMP00097	测试用户096	13910000096	测试用户096	0	2543458f52c0d7308239616a90bc8be2	flNVnMDWyi	1	user096@aiarsenal.cc	0		cyrj194puwursqce38vm471isql0qc4j		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
98	fy7jo8iycp36dkl4rjko3oy6r8vknu3m	user097	EMP00098	测试用户097	13910000097	测试用户097	0	c3e3ffe9a4c6287b29d66248e1df918f	9tZra6cQAy	1	user097@aiarsenal.cc	1		m2hu15ojpus1layz4k4zhifi9cj8erbt		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
99	e2ctnmnw5esjmd5o5gp7zg2gw3f6mnwm	user098	EMP00099	测试用户098	13910000098	测试用户098	0	6936dfd3987fa4c5765d30abc02598b0	nFPMUFuf31	1	user098@aiarsenal.cc	2		b52vpvw2wompdleclbp5467pkw017oab		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
100	ouzpq54s44roqs97m7ii1breztn12gtu	user099	EMP00100	测试用户099	13910000099	测试用户099	0	2ff176b0448c43bc7ff68bd5157e50aa	gpevb7hLpi	1	user099@aiarsenal.cc	0		gb2wz5r8etns12cgiu4pi627n2sqqlfw		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
101	s29fp7shxnshfy13z03uzw0rpnp9rkrm	user100	EMP00101	测试用户100	13910000100	测试用户100	0	b3f802869693dcf8f0e13b87cf79f8fd	zEkWDzoNwj	1	user100@aiarsenal.cc	1		tm1yja3iahb1f4v3bhc7q7amvcu9crhk		0				2026-07-04 00:00:00	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
1	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	hi	EMP00001	hi	13800000000	hi	0	222c58572fb4c8c9b8e22ade587ebe85	k7Qm2xP9aL	1	hi@aiarsenal.cc	1		dqr6r6e6exwggi02v2h0tqkbdd46c3sb		1				\N	2026-07-04 00:00:00	2026-07-04 00:00:00	\N
\.


--
-- Data for Name: aa_user_online; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_user_online (id, uuid, token, create_time, user_name, ip, explorer, os) FROM stdin;
\.


--
-- Data for Name: aa_user_post; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_user_post (id, user_id, post_id) FROM stdin;
1	i47dhcwyx2rulbjl5vlab89orpamongb	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
2	awaoolltywg611faqbrfjs0a6qq7e7mq	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
3	ttozve8tomu4bw7q64iqch473vic8xjx	las48k9l1ii0ir2962a5r4xfxame2btu
4	l7v7hxinitk68gtbwp1ygd1ud39q88bt	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
5	g1zzvvdxqrpmzk2kw8q4ky3a26e6qfxv	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
6	wcsj3t9d38o5o5ryzp9pl0s72bbolqje	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
7	psnpiaisc4utvtowod2cismxnen0sqji	p18peuyi9qaddojzcck35ufolgawzrpg
8	yftk7uwwhy2oxmrzkq3tp4dh89bch1jb	vetr6vywrxw1mfbhxco2oaaiz7okedet
9	ey81ndf6zpobmaoxqiquhofjpu02z8ht	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
10	hhhnxqvutljctsgaoo780vo563bez55a	vetr6vywrxw1mfbhxco2oaaiz7okedet
11	xdrjswzjeydx1v5ge05jltxd0ixg0yya	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
12	qqzy7xff86q8kfmyopjq5dmgbwf03lkd	w1c2cofhzsnm559td9uot44p6mg35zza
13	hwogmz6ku42vzbxo7l4g9seiftxmf55z	las48k9l1ii0ir2962a5r4xfxame2btu
14	vxxrlq1k4nmwnbuiud4z8g4zdn6brnom	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
15	t8ubxdsaiuixyj57rt57vhrggh6yejco	p18peuyi9qaddojzcck35ufolgawzrpg
16	huoc4rxc0p8g6f4cswatmogr6daqfxgl	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
17	yv6887f2ne6v8cwx0shm2ty97nbv4vme	vetr6vywrxw1mfbhxco2oaaiz7okedet
18	cz5dv5ogq1ifcqmsempf00hqx4w9e7fr	w1c2cofhzsnm559td9uot44p6mg35zza
19	ddv2o2ggneyz5omf2hsakq5gq5nb37kp	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
20	iozdijm6umoh07mwmf9shtyb3gkm6b2e	las48k9l1ii0ir2962a5r4xfxame2btu
21	xkbw9qo0volhjszdrgr11ieh9qmy05xm	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
22	ojmwz7cbldpxwnmu4h9ed5gzjm8yglbj	vetr6vywrxw1mfbhxco2oaaiz7okedet
23	ouvzpmuuf8vmoja8g7ua9x1a6m83dkqe	vetr6vywrxw1mfbhxco2oaaiz7okedet
24	f571jz7rptodss6igenpqkbdv9s8nhgq	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
25	nn5sxc1va1wbi7c3y8nj3si1tznkil1o	vetr6vywrxw1mfbhxco2oaaiz7okedet
26	rjtjyaegy2r498hswx8htls1ncqz2shp	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
27	o9ryfxtbs7mz7d1pitt1ji1eyofbuvhl	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
28	v8nyq4ekvn96ffe69fpteqprfbyw7d5o	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
29	ccp94mq5kpf7del1n8kixhght9hoc74f	p18peuyi9qaddojzcck35ufolgawzrpg
30	i5jq4vt4wk4pf3lb4nx2fh4339wbfz0y	vetr6vywrxw1mfbhxco2oaaiz7okedet
31	h30ls46rppecsk1di4n5zxlz7xpcps5i	vetr6vywrxw1mfbhxco2oaaiz7okedet
32	g4dp1tsgez5vvn1a2hq7vpgrireuwr3f	ec0minuxxdiktu5c3hqu72eayy06ljrz
33	xh9e2z238dalqjxse0gdlrteaf49325f	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
34	p6k1ds5468pmzbz1i6v4814lt5475m3m	w1c2cofhzsnm559td9uot44p6mg35zza
35	nxbyzuyiu6ouyygcuzemmac873aljn3j	las48k9l1ii0ir2962a5r4xfxame2btu
36	f386t32hk8sghcnvf24g3st3i41phl9o	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
37	qu9owum1ui0elni61b5i29akyl7jt03p	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
38	b7qp8p0xj1okbz6vxrmzngd32jsonytp	p18peuyi9qaddojzcck35ufolgawzrpg
39	qh1et5ylq2fm1e82d32fl67gvnqo6riq	ec0minuxxdiktu5c3hqu72eayy06ljrz
40	tyrhx8gab3hzay0xmz65aia5j5xzjcuo	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
41	wlkajsoyh27oscyja111i6bq5e5cd5gx	vetr6vywrxw1mfbhxco2oaaiz7okedet
42	ki2q6fgdt8o56u3vn2ckanvoy34qqq9a	vetr6vywrxw1mfbhxco2oaaiz7okedet
43	lsinkrec95cl8xk41pc1xa0nwp77r7nq	vetr6vywrxw1mfbhxco2oaaiz7okedet
44	u7keg12zsf1fp6wxnd0w78r03lk9p5ne	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
45	vj9e202tchk0m7ar1pg1w2xvcb52770u	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
46	bdlps6p632yb96o44b00s23l1eywr0sy	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
47	fd5893szdczhr9ylo1cwbrsbb7f6s0np	ec0minuxxdiktu5c3hqu72eayy06ljrz
48	fisec94xg1zmsoe73xqi8s7pcioxs2xy	w1c2cofhzsnm559td9uot44p6mg35zza
49	qjnllgil3gdfowp1khpin6o2b6egz7vs	las48k9l1ii0ir2962a5r4xfxame2btu
50	et5k755mj9ndmzcz5uqj79o0se8a7hjq	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
51	z1cnqvbthuj6j2ocm19p5wivwknkwwxb	vetr6vywrxw1mfbhxco2oaaiz7okedet
52	fgyo9wgof0kcalr2r8cskc2g0bejiw4r	ec0minuxxdiktu5c3hqu72eayy06ljrz
53	zzye39s02viahffg8futr2nq2m8ahw9v	p18peuyi9qaddojzcck35ufolgawzrpg
54	nwesstx94ue6ekr832lgdj7zz1r81yol	vetr6vywrxw1mfbhxco2oaaiz7okedet
55	fczzjf2lns76m9avq50s302n8bfv6kxy	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
56	ljkgwn4eowlvuh9aafzdc0ykhj06ptpw	w1c2cofhzsnm559td9uot44p6mg35zza
57	rbfrd3phrnqazpxz83q61q87t3s6znun	las48k9l1ii0ir2962a5r4xfxame2btu
58	qdroqjw2xx7ckth36lsw9lfmexyf1fbt	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
59	nadp1hsoga0hqcw16pynzk19h7ehcj5r	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
60	l5iu02zzxae7gw6denbs5m430pnakwco	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
61	g5zzcwa6lljcra9bh95ot0wmdbo8bb1v	ec0minuxxdiktu5c3hqu72eayy06ljrz
62	idg18sc5jlnnbs02myk619oj5s8kstej	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
63	ap2xvhdaf6feytr8bbfa0x563uuoztsm	ec0minuxxdiktu5c3hqu72eayy06ljrz
64	aqjsus7aopdpz778evymkbst0mwg6s4l	vetr6vywrxw1mfbhxco2oaaiz7okedet
65	y9i1zy0zq9jtjx57eleglzfvurltdf9v	p18peuyi9qaddojzcck35ufolgawzrpg
66	swy9yjech3it00fnpuk9m5t3ww50f0zu	vetr6vywrxw1mfbhxco2oaaiz7okedet
67	kp0b2zujdinw3quweijsa8k1fn5jo29l	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
68	t6bh2x83k6hg6haogt4loyl3dw283d1d	vetr6vywrxw1mfbhxco2oaaiz7okedet
69	nk12a6f4keadmc12tfw69abqnb0hco5d	vetr6vywrxw1mfbhxco2oaaiz7okedet
70	zqb590opd1gatrzyadk6xax7e7bx500y	w1c2cofhzsnm559td9uot44p6mg35zza
71	hyztdi4uhb1w7ndfi47zpp1y5m16soai	las48k9l1ii0ir2962a5r4xfxame2btu
72	kb38c9j616dragwwgbyo8p7j4vk1xnub	ec0minuxxdiktu5c3hqu72eayy06ljrz
73	b0f6pm4d64m6zz6agzg7ff1y80kb112t	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
74	hhqf1lr8ktqflwomy9ywlr3e2g87c1nr	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
75	vfuzzdizy3ladqg0f9bwb22csxl2b8zg	vetr6vywrxw1mfbhxco2oaaiz7okedet
76	bred3yjuutw1vt7mlwx4259tlez2hyvt	vetr6vywrxw1mfbhxco2oaaiz7okedet
77	o109vwi25ey3ru4726gkeedt6yed15po	p18peuyi9qaddojzcck35ufolgawzrpg
78	qpsr1il81pv2rpwsygkudq6ec28li8ee	vetr6vywrxw1mfbhxco2oaaiz7okedet
79	quguougzmiaoxn29p4jm7vfza0nu2mqn	ec0minuxxdiktu5c3hqu72eayy06ljrz
80	vbsw1wfxo4xiylrqp4ytlvb0iomct94u	vetr6vywrxw1mfbhxco2oaaiz7okedet
81	rhdnuq6wo64neftkd68yny35yste2xtx	w1c2cofhzsnm559td9uot44p6mg35zza
82	p6m3p05a3n48hsaq294d431wybsq07zr	las48k9l1ii0ir2962a5r4xfxame2btu
83	z5o2otfmasx8ylqa1iolk9z530n08opl	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
84	fxufj8dsgmh7hwzc1zgbgoiptbr9smex	ec0minuxxdiktu5c3hqu72eayy06ljrz
85	fyqkcyecpap2uqzaocs8q8t8x3cvmvfq	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
86	v32y4lq3ljmqs3qgv8onxtk16gnutgri	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
87	y88icjiuzvqdnpjuh4lype9xt97a3xxr	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
88	smg1dhkia9fg7hjuycfk8449fz2dzj2a	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
89	so85b77vby7plhnvr6pf17c98b9mf0to	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
90	r8144ojgas0uiprldnk21ckrbw6z9kgy	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
91	tdunjhhjpzmmiw9eu8a9shulxqbfp8ec	ec0minuxxdiktu5c3hqu72eayy06ljrz
92	mxlhyiou8vg6szccawm0zmpug4gg5xrb	vetr6vywrxw1mfbhxco2oaaiz7okedet
93	nq2gkeoricp2ufl5ksvv6pznixkj7ifz	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
94	hnvmzuzx6jup48fa2ow28va7vmjmipua	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
95	eieus50pzqqvzj1ge6l72zoina4v91gy	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
96	rump0avc7t7htk5y52r61h7n1l90jv9f	jm8ebgikyyvpqdvxyq1koy3ka1hrkf3v
97	fy7jo8iycp36dkl4rjko3oy6r8vknu3m	p18peuyi9qaddojzcck35ufolgawzrpg
98	e2ctnmnw5esjmd5o5gp7zg2gw3f6mnwm	vetr6vywrxw1mfbhxco2oaaiz7okedet
99	ouzpq54s44roqs97m7ii1breztn12gtu	z9zwsqp1y5gklcyqwqywh2wbfg2ih0le
100	s29fp7shxnshfy13z03uzw0rpnp9rkrm	w1c2cofhzsnm559td9uot44p6mg35zza
\.


--
-- Name: aa_dept_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_dept_id_seq', 173, true);


--
-- Name: aa_emp_set_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_emp_set_id_seq', 1, false);


--
-- Name: aa_login_log_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_login_log_id_seq', 22, true);


--
-- Name: aa_menu_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_menu_id_seq', 634, true);


--
-- Name: aa_notice_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_notice_id_seq', 1, true);


--
-- Name: aa_oper_log_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_oper_log_id_seq', 56, true);


--
-- Name: aa_perm_group_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_perm_group_id_seq', 1, false);


--
-- Name: aa_post_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_post_id_seq', 9, true);


--
-- Name: aa_review_order_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_review_order_id_seq', 1, false);


--
-- Name: aa_role_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_role_id_seq', 8, true);


--
-- Name: aa_user_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_user_id_seq', 101, true);


--
-- Name: aa_user_online_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_user_online_id_seq', 25, true);


--
-- Name: aa_user_post_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_user_post_id_seq', 100, true);


--
-- Name: aa_dept aa_dept_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_dept
    ADD CONSTRAINT aa_dept_pkey PRIMARY KEY (id);


--
-- Name: aa_emp_set aa_emp_set_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_emp_set
    ADD CONSTRAINT aa_emp_set_pkey PRIMARY KEY (id);


--
-- Name: aa_login_log aa_login_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_login_log
    ADD CONSTRAINT aa_login_log_pkey PRIMARY KEY (id);


--
-- Name: aa_menu aa_menu_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_menu
    ADD CONSTRAINT aa_menu_pkey PRIMARY KEY (id);


--
-- Name: aa_notice aa_notice_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_notice
    ADD CONSTRAINT aa_notice_pkey PRIMARY KEY (id);


--
-- Name: aa_oper_log aa_oper_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_oper_log
    ADD CONSTRAINT aa_oper_log_pkey PRIMARY KEY (id);


--
-- Name: aa_perm_group aa_perm_group_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_perm_group
    ADD CONSTRAINT aa_perm_group_pkey PRIMARY KEY (id);


--
-- Name: aa_post aa_post_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_post
    ADD CONSTRAINT aa_post_pkey PRIMARY KEY (id);


--
-- Name: aa_review_order aa_review_order_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_review_order
    ADD CONSTRAINT aa_review_order_pkey PRIMARY KEY (id);


--
-- Name: aa_role aa_role_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_role
    ADD CONSTRAINT aa_role_pkey PRIMARY KEY (id);


--
-- Name: aa_user_online aa_user_online_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_user_online
    ADD CONSTRAINT aa_user_online_pkey PRIMARY KEY (id);


--
-- Name: aa_user aa_user_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_user
    ADD CONSTRAINT aa_user_pkey PRIMARY KEY (id);


--
-- Name: aa_user_post aa_user_post_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_user_post
    ADD CONSTRAINT aa_user_post_pkey PRIMARY KEY (id);


--
-- Name: idx_aa_dept_dept_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_dept_dept_id ON public.aa_dept USING btree (dept_id);


--
-- Name: idx_aa_emp_set_set_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_emp_set_set_id ON public.aa_emp_set USING btree (set_id);


--
-- Name: idx_aa_login_log_info_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_login_log_info_id ON public.aa_login_log USING btree (info_id);


--
-- Name: idx_aa_menu_menu_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_menu_menu_id ON public.aa_menu USING btree (menu_id);


--
-- Name: idx_aa_menu_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_menu_name ON public.aa_menu USING btree (name);


--
-- Name: idx_aa_menu_pid; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_menu_pid ON public.aa_menu USING btree (pid);


--
-- Name: idx_aa_menu_weigh; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_menu_weigh ON public.aa_menu USING btree (weigh);


--
-- Name: idx_aa_notice_notice_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_notice_notice_id ON public.aa_notice USING btree (notice_id);


--
-- Name: idx_aa_notice_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_notice_user_id ON public.aa_notice USING btree (user_id);


--
-- Name: idx_aa_notice_user_read; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_notice_user_read ON public.aa_notice USING btree (user_id, is_read);


--
-- Name: idx_aa_oper_log_oper_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_oper_log_oper_id ON public.aa_oper_log USING btree (oper_id);


--
-- Name: idx_aa_perm_group_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_perm_group_group_id ON public.aa_perm_group USING btree (group_id);


--
-- Name: idx_aa_post_post_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_post_post_id ON public.aa_post USING btree (post_id);


--
-- Name: idx_aa_review_order_applicant; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_review_order_applicant ON public.aa_review_order USING btree (applicant_id, created_at DESC);


--
-- Name: idx_aa_review_order_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_review_order_id ON public.aa_review_order USING btree (order_id);


--
-- Name: idx_aa_review_order_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_review_order_status ON public.aa_review_order USING btree (status, dept_id);


--
-- Name: idx_aa_role_role_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_role_role_id ON public.aa_role USING btree (role_id);


--
-- Name: idx_aa_user_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_user_name ON public.aa_user USING btree (name) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_user_online_uni_token; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_user_online_uni_token ON public.aa_user_online USING btree (token);


--
-- Name: idx_aa_user_online_uni_uuid; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_user_online_uni_uuid ON public.aa_user_online USING btree (uuid);


--
-- Name: idx_aa_user_user_email; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_user_user_email ON public.aa_user USING btree (user_email) WHERE ((deleted_at IS NULL) AND ((user_email)::text <> ''::text));


--
-- Name: idx_aa_user_user_empno; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_user_user_empno ON public.aa_user USING btree (employee_no) WHERE ((deleted_at IS NULL) AND ((employee_no)::text <> ''::text));


--
-- Name: idx_aa_user_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_user_user_id ON public.aa_user USING btree (user_id);


--
-- Name: idx_aa_user_user_login; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_user_user_login ON public.aa_user USING btree (user_name) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_user_user_mobile; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_user_user_mobile ON public.aa_user USING btree (mobile) WHERE ((deleted_at IS NULL) AND ((mobile)::text <> ''::text));


--
-- PostgreSQL database dump complete
--

\unrestrict ixXf5k83YrWMPq6rgBcTO96NwdVldCXO66ThxuXV72oCXAx2k6DRDgAU73I8A5K

