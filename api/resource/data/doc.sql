-- ============================================================================
-- doc.sql — 文档管理模块数据库与表结构
-- ============================================================================
-- 依赖：PostgreSQL 16 + pgvector（与 aiarsenal / aa_skill 同实例异库）
-- 关联：created_by 软引用 aiarsenal 库的 aa_user.user_id（跨库无 FK）
-- 约定：每表 id bigserial PK（内部）+ <实体>_id varchar(32) UNIQUE（对外业务 id）
-- 设计：文档（元数据+抽取全文）与分块（chunk + 语义向量）分表——检索按块向量
--       打分、聚合回文档，命中块内容即摘要；更新文档 = 重抽全文 + 重嵌全部分块
-- 数据：库内只存运行时数据（文档内容/向量均为本机导入，绝不进 git/种子）
-- 重建：psql -f resource/data/doc.sql（每表 DROP IF EXISTS，可重复跑）
-- ============================================================================

-- ----------------------------------------------------------------------------
-- Part A — 独立数据库 aa_doc
-- ----------------------------------------------------------------------------

SELECT 'CREATE DATABASE aa_doc'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'aa_doc')\gexec

\connect aa_doc

-- 以上为历史前导（gexec 建库 + connect 目标库）；以下正文为 pg_dump 固化版（2026-09 squash，已合并全部历史迁移）
-- PostgreSQL database dump
--

\restrict ID6alf558UQN1zSvtAQMGH9ZmkzSeYKKPO1jttadGrIpDeDsAx3Lz9Q3htff7UX

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

DROP INDEX IF EXISTS public.uq_aa_doc_dir_name;
DROP INDEX IF EXISTS public.uq_aa_doc_dir_kind_ref;
DROP INDEX IF EXISTS public.idx_aa_doc_grant_target;
DROP INDEX IF EXISTS public.idx_aa_doc_document_type;
DROP INDEX IF EXISTS public.idx_aa_doc_document_hash;
DROP INDEX IF EXISTS public.idx_aa_doc_document_dir;
DROP INDEX IF EXISTS public.idx_aa_doc_document_created;
DROP INDEX IF EXISTS public.idx_aa_doc_dir_parent;
DROP INDEX IF EXISTS public.idx_aa_doc_chunk_doc;
DROP INDEX IF EXISTS public.idx_aa_doc_access_doc;
DROP INDEX IF EXISTS public.idx_aa_doc_access_created;
ALTER TABLE IF EXISTS ONLY public.aa_doc_grant DROP CONSTRAINT IF EXISTS aa_doc_grant_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_doc_document DROP CONSTRAINT IF EXISTS aa_doc_document_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_doc_document DROP CONSTRAINT IF EXISTS aa_doc_document_doc_id_key;
ALTER TABLE IF EXISTS ONLY public.aa_doc_dir DROP CONSTRAINT IF EXISTS aa_doc_dir_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_doc_dir DROP CONSTRAINT IF EXISTS aa_doc_dir_dir_id_key;
ALTER TABLE IF EXISTS ONLY public.aa_doc_chunk DROP CONSTRAINT IF EXISTS aa_doc_chunk_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_doc_access_log DROP CONSTRAINT IF EXISTS aa_doc_access_log_pkey;
ALTER TABLE IF EXISTS public.aa_doc_grant ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_doc_document ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_doc_dir ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_doc_chunk ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_doc_access_log ALTER COLUMN id DROP DEFAULT;
DROP SEQUENCE IF EXISTS public.aa_doc_grant_id_seq;
DROP TABLE IF EXISTS public.aa_doc_grant;
DROP SEQUENCE IF EXISTS public.aa_doc_document_id_seq;
DROP TABLE IF EXISTS public.aa_doc_document;
DROP SEQUENCE IF EXISTS public.aa_doc_dir_id_seq;
DROP TABLE IF EXISTS public.aa_doc_dir;
DROP SEQUENCE IF EXISTS public.aa_doc_chunk_id_seq;
DROP TABLE IF EXISTS public.aa_doc_chunk;
DROP SEQUENCE IF EXISTS public.aa_doc_access_log_id_seq;
DROP TABLE IF EXISTS public.aa_doc_access_log;
DROP EXTENSION IF EXISTS vector;
--
-- Name: vector; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: aa_doc_access_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_doc_access_log (
    id bigint NOT NULL,
    doc_id character varying(32) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    action character varying(8) DEFAULT 'read'::character varying NOT NULL,
    keyword character varying(256) DEFAULT ''::character varying NOT NULL,
    hits integer DEFAULT 0 NOT NULL,
    source character varying(8) DEFAULT 'ui'::character varying NOT NULL,
    created_at timestamp without time zone DEFAULT now()
);


--
-- Name: aa_doc_access_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_doc_access_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_doc_access_log_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_doc_access_log_id_seq OWNED BY public.aa_doc_access_log.id;


--
-- Name: aa_doc_chunk; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_doc_chunk (
    id bigint NOT NULL,
    doc_id character varying(32) DEFAULT ''::character varying NOT NULL,
    seq integer DEFAULT 0 NOT NULL,
    content text DEFAULT ''::text NOT NULL,
    semantic_vector public.vector(1536),
    created_at timestamp without time zone DEFAULT now()
);


--
-- Name: aa_doc_chunk_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_doc_chunk_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_doc_chunk_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_doc_chunk_id_seq OWNED BY public.aa_doc_chunk.id;


--
-- Name: aa_doc_dir; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_doc_dir (
    id bigint NOT NULL,
    dir_id character varying(32) DEFAULT ''::character varying NOT NULL,
    name character varying(128) DEFAULT ''::character varying NOT NULL,
    parent_id character varying(32) DEFAULT ''::character varying NOT NULL,
    visibility smallint DEFAULT 2 NOT NULL,
    kind character varying(16) DEFAULT ''::character varying NOT NULL,
    ref_id character varying(32) DEFAULT ''::character varying NOT NULL,
    created_by character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: aa_doc_dir_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_doc_dir_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_doc_dir_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_doc_dir_id_seq OWNED BY public.aa_doc_dir.id;


--
-- Name: aa_doc_document; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_doc_document (
    id bigint NOT NULL,
    doc_id character varying(32) DEFAULT ''::character varying NOT NULL,
    title character varying(512) DEFAULT ''::character varying NOT NULL,
    summary text,
    file_name character varying(512) DEFAULT ''::character varying NOT NULL,
    file_type character varying(16) DEFAULT ''::character varying NOT NULL,
    file_size bigint DEFAULT 0 NOT NULL,
    file_hash character varying(64) DEFAULT ''::character varying NOT NULL,
    content_text text DEFAULT ''::text NOT NULL,
    char_count integer DEFAULT 0 NOT NULL,
    chunk_count integer DEFAULT 0 NOT NULL,
    embed_status smallint DEFAULT 0 NOT NULL,
    embed_error text,
    source character varying(16) DEFAULT 'ui'::character varying NOT NULL,
    storage_path character varying(512) DEFAULT ''::character varying NOT NULL,
    dir_id character varying(32) DEFAULT ''::character varying NOT NULL,
    visibility smallint DEFAULT 2 NOT NULL,
    created_by character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: aa_doc_document_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_doc_document_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_doc_document_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_doc_document_id_seq OWNED BY public.aa_doc_document.id;


--
-- Name: aa_doc_grant; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_doc_grant (
    id bigint NOT NULL,
    target_type character varying(8) DEFAULT ''::character varying NOT NULL,
    target_id character varying(32) DEFAULT ''::character varying NOT NULL,
    action character varying(8) DEFAULT 'read'::character varying NOT NULL,
    user_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    dept_id character varying(32) DEFAULT ''::character varying NOT NULL,
    role_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    post_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    exclude boolean DEFAULT false NOT NULL,
    group_id character varying(32) DEFAULT ''::character varying NOT NULL,
    set_id character varying(32) DEFAULT ''::character varying NOT NULL,
    created_by character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone DEFAULT now()
);


--
-- Name: aa_doc_grant_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_doc_grant_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_doc_grant_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_doc_grant_id_seq OWNED BY public.aa_doc_grant.id;


--
-- Name: aa_doc_access_log id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_doc_access_log ALTER COLUMN id SET DEFAULT nextval('public.aa_doc_access_log_id_seq'::regclass);


--
-- Name: aa_doc_chunk id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_doc_chunk ALTER COLUMN id SET DEFAULT nextval('public.aa_doc_chunk_id_seq'::regclass);


--
-- Name: aa_doc_dir id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_doc_dir ALTER COLUMN id SET DEFAULT nextval('public.aa_doc_dir_id_seq'::regclass);


--
-- Name: aa_doc_document id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_doc_document ALTER COLUMN id SET DEFAULT nextval('public.aa_doc_document_id_seq'::regclass);


--
-- Name: aa_doc_grant id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_doc_grant ALTER COLUMN id SET DEFAULT nextval('public.aa_doc_grant_id_seq'::regclass);


--
-- Data for Name: aa_doc_access_log; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_doc_access_log (id, doc_id, user_id, action, keyword, hits, source, created_at) FROM stdin;
\.


--
-- Data for Name: aa_doc_chunk; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_doc_chunk (id, doc_id, seq, content, semantic_vector, created_at) FROM stdin;
\.


--
-- Data for Name: aa_doc_dir; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_doc_dir (id, dir_id, name, parent_id, visibility, kind, ref_id, created_by, created_at, updated_at) FROM stdin;
1	docspaceprojectroot000000000a	项目空间		2	project_space			2026-09-16 22:49:37.919892	2026-09-16 22:49:37.919892
2	docspacepersonalroot000000000a	个人空间		2	personal_space			2026-09-16 22:49:37.919892	2026-09-16 22:49:37.919892
3	w688rwr3pv1qrvf14ferr4rwvrrwbwcw	hi	docspacepersonalroot000000000a	0	personal	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	wxd3a2drv1ikdkqxub4hp49e7vrvcd3t	2026-09-17 05:31:13.553479	2026-09-17 05:31:13.553479
4	zufiuh1wlotp55k5kez6m4weh2i7x6lr	用户空间004	docspacepersonalroot000000000a	0	personal	i47dhcwyx2rulbjl5vlab89orpamongb	i47dhcwyx2rulbjl5vlab89orpamongb	2026-09-17 05:31:13.554223	2026-09-17 05:31:13.554223
5	g6jkla72z3dkgfhd5f7wy97x8bt37y7i	用户空间005	docspacepersonalroot000000000a	0	personal	awaoolltywg611faqbrfjs0a6qq7e7mq	awaoolltywg611faqbrfjs0a6qq7e7mq	2026-09-17 05:31:13.554574	2026-09-17 05:31:13.554574
6	a5y6q0wm9jh9sflsy6dwkg3yz12qg45n	用户空间006	docspacepersonalroot000000000a	0	personal	ttozve8tomu4bw7q64iqch473vic8xjx	ttozve8tomu4bw7q64iqch473vic8xjx	2026-09-17 05:31:13.554946	2026-09-17 05:31:13.554946
7	jr8w1905r82j6vapcdcb58wkpoktvl1e	用户空间007	docspacepersonalroot000000000a	0	personal	l7v7hxinitk68gtbwp1ygd1ud39q88bt	l7v7hxinitk68gtbwp1ygd1ud39q88bt	2026-09-17 05:31:13.55531	2026-09-17 05:31:13.55531
8	c0fszo72v1in0szog4lccxjej5ten3ss	用户空间008	docspacepersonalroot000000000a	0	personal	g1zzvvdxqrpmzk2kw8q4ky3a26e6qfxv	g1zzvvdxqrpmzk2kw8q4ky3a26e6qfxv	2026-09-17 05:31:13.555708	2026-09-17 05:31:13.555708
9	w4p082lzsy3tdd0ftd4zf60h2zwko34h	用户空间009	docspacepersonalroot000000000a	0	personal	wcsj3t9d38o5o5ryzp9pl0s72bbolqje	wcsj3t9d38o5o5ryzp9pl0s72bbolqje	2026-09-17 05:31:13.556028	2026-09-17 05:31:13.556028
10	bbrd7q8r6psneh94936quvr4lwzhwecu	用户空间010	docspacepersonalroot000000000a	0	personal	psnpiaisc4utvtowod2cismxnen0sqji	psnpiaisc4utvtowod2cismxnen0sqji	2026-09-17 05:31:13.556396	2026-09-17 05:31:13.556396
11	h8l0uibr2l8s6z11owefyjnmxmp6qxbm	用户空间011	docspacepersonalroot000000000a	0	personal	yftk7uwwhy2oxmrzkq3tp4dh89bch1jb	yftk7uwwhy2oxmrzkq3tp4dh89bch1jb	2026-09-17 05:31:13.556716	2026-09-17 05:31:13.556716
12	eknu1t72qyi4f8woo3wgpe5ifyjrgdmz	用户空间012	docspacepersonalroot000000000a	0	personal	ey81ndf6zpobmaoxqiquhofjpu02z8ht	ey81ndf6zpobmaoxqiquhofjpu02z8ht	2026-09-17 05:31:13.557018	2026-09-17 05:31:13.557018
13	vili6nc6juenew6xy6okxg4w9x3t5osq	用户空间013	docspacepersonalroot000000000a	0	personal	hhhnxqvutljctsgaoo780vo563bez55a	hhhnxqvutljctsgaoo780vo563bez55a	2026-09-17 05:31:13.557413	2026-09-17 05:31:13.557413
14	fmsazill5nu6mjoidxu0apoe8uqiovam	用户空间014	docspacepersonalroot000000000a	0	personal	xdrjswzjeydx1v5ge05jltxd0ixg0yya	xdrjswzjeydx1v5ge05jltxd0ixg0yya	2026-09-17 05:31:13.557709	2026-09-17 05:31:13.557709
15	bxlhy6c3hjzp0zorkoqytkdxm37hya2s	用户空间015	docspacepersonalroot000000000a	0	personal	qqzy7xff86q8kfmyopjq5dmgbwf03lkd	qqzy7xff86q8kfmyopjq5dmgbwf03lkd	2026-09-17 05:31:13.558049	2026-09-17 05:31:13.558049
16	l5tw1ziqcmklgwpfqrkhdqgkhap3kq0i	用户空间016	docspacepersonalroot000000000a	0	personal	hwogmz6ku42vzbxo7l4g9seiftxmf55z	hwogmz6ku42vzbxo7l4g9seiftxmf55z	2026-09-17 05:31:13.558411	2026-09-17 05:31:13.558411
17	jna7rxab1wxynal3eetuk8ap2oe10vfx	用户空间017	docspacepersonalroot000000000a	0	personal	vxxrlq1k4nmwnbuiud4z8g4zdn6brnom	vxxrlq1k4nmwnbuiud4z8g4zdn6brnom	2026-09-17 05:31:13.558735	2026-09-17 05:31:13.558735
18	peim503otp0d0gpd0tirk1wlk4y5sd3i	用户空间018	docspacepersonalroot000000000a	0	personal	t8ubxdsaiuixyj57rt57vhrggh6yejco	t8ubxdsaiuixyj57rt57vhrggh6yejco	2026-09-17 05:31:13.559184	2026-09-17 05:31:13.559184
19	e2cd6l4koy67czv5rug10sbppyq9gejf	用户空间019	docspacepersonalroot000000000a	0	personal	huoc4rxc0p8g6f4cswatmogr6daqfxgl	huoc4rxc0p8g6f4cswatmogr6daqfxgl	2026-09-17 05:31:13.55954	2026-09-17 05:31:13.55954
20	tjo478m67h0zklf7e6vdnubu9mzg417d	用户空间020	docspacepersonalroot000000000a	0	personal	yv6887f2ne6v8cwx0shm2ty97nbv4vme	yv6887f2ne6v8cwx0shm2ty97nbv4vme	2026-09-17 05:31:13.559891	2026-09-17 05:31:13.559891
21	aaiozpqzwx1nifmewe2s19pc0vxrblvv	用户空间021	docspacepersonalroot000000000a	0	personal	cz5dv5ogq1ifcqmsempf00hqx4w9e7fr	cz5dv5ogq1ifcqmsempf00hqx4w9e7fr	2026-09-17 05:31:13.562035	2026-09-17 05:31:13.562035
22	jhuifbshfnzj7sh7uanoazk1l6cc9juk	用户空间022	docspacepersonalroot000000000a	0	personal	ddv2o2ggneyz5omf2hsakq5gq5nb37kp	ddv2o2ggneyz5omf2hsakq5gq5nb37kp	2026-09-17 05:31:13.562405	2026-09-17 05:31:13.562405
23	tyzgg93517meat1c5iod9eu3mbx9lqjt	用户空间023	docspacepersonalroot000000000a	0	personal	iozdijm6umoh07mwmf9shtyb3gkm6b2e	iozdijm6umoh07mwmf9shtyb3gkm6b2e	2026-09-17 05:31:13.562712	2026-09-17 05:31:13.562712
24	uxz4mh74rv3axyrgzp4f688ztm8x83dp	用户空间024	docspacepersonalroot000000000a	0	personal	xkbw9qo0volhjszdrgr11ieh9qmy05xm	xkbw9qo0volhjszdrgr11ieh9qmy05xm	2026-09-17 05:31:13.563016	2026-09-17 05:31:13.563016
25	po6cdovp6wxasvb2qo3szqp7pavn58jz	用户空间025	docspacepersonalroot000000000a	0	personal	ojmwz7cbldpxwnmu4h9ed5gzjm8yglbj	ojmwz7cbldpxwnmu4h9ed5gzjm8yglbj	2026-09-17 05:31:13.56331	2026-09-17 05:31:13.56331
26	n1o9xe0ha6zxghujmlgr5jr9j70qyy7p	用户空间026	docspacepersonalroot000000000a	0	personal	ouvzpmuuf8vmoja8g7ua9x1a6m83dkqe	ouvzpmuuf8vmoja8g7ua9x1a6m83dkqe	2026-09-17 05:31:13.56363	2026-09-17 05:31:13.56363
27	ut31i71u9y8sznsxj4anyjdfzghsohrr	用户空间027	docspacepersonalroot000000000a	0	personal	f571jz7rptodss6igenpqkbdv9s8nhgq	f571jz7rptodss6igenpqkbdv9s8nhgq	2026-09-17 05:31:13.563977	2026-09-17 05:31:13.563977
28	b61nnamphdsb2g8v3ucdplia7pua21xv	用户空间028	docspacepersonalroot000000000a	0	personal	nn5sxc1va1wbi7c3y8nj3si1tznkil1o	nn5sxc1va1wbi7c3y8nj3si1tznkil1o	2026-09-17 05:31:13.564343	2026-09-17 05:31:13.564343
29	okn9ne7rmdmu8fv32v6dj28dg05ywn5s	用户空间029	docspacepersonalroot000000000a	0	personal	rjtjyaegy2r498hswx8htls1ncqz2shp	rjtjyaegy2r498hswx8htls1ncqz2shp	2026-09-17 05:31:13.564649	2026-09-17 05:31:13.564649
30	kzw48vv82c6vcjaur8n7561u09rqd9bk	用户空间030	docspacepersonalroot000000000a	0	personal	o9ryfxtbs7mz7d1pitt1ji1eyofbuvhl	o9ryfxtbs7mz7d1pitt1ji1eyofbuvhl	2026-09-17 05:31:13.565013	2026-09-17 05:31:13.565013
31	cayxecqklj90gixcswx5x3djdzvrzqil	用户空间031	docspacepersonalroot000000000a	0	personal	v8nyq4ekvn96ffe69fpteqprfbyw7d5o	v8nyq4ekvn96ffe69fpteqprfbyw7d5o	2026-09-17 05:31:13.565325	2026-09-17 05:31:13.565325
32	vesm1x6c8i7pzyf4gonyoye34uk2i7eg	用户空间032	docspacepersonalroot000000000a	0	personal	ccp94mq5kpf7del1n8kixhght9hoc74f	ccp94mq5kpf7del1n8kixhght9hoc74f	2026-09-17 05:31:13.565623	2026-09-17 05:31:13.565623
33	os45kqii44nvm91ewd1a1a9mu6ujyodp	用户空间033	docspacepersonalroot000000000a	0	personal	i5jq4vt4wk4pf3lb4nx2fh4339wbfz0y	i5jq4vt4wk4pf3lb4nx2fh4339wbfz0y	2026-09-17 05:31:13.565956	2026-09-17 05:31:13.565956
34	b3q8i5tfmi2x4kkbm5fk0vngop1ewajt	用户空间034	docspacepersonalroot000000000a	0	personal	h30ls46rppecsk1di4n5zxlz7xpcps5i	h30ls46rppecsk1di4n5zxlz7xpcps5i	2026-09-17 05:31:13.566275	2026-09-17 05:31:13.566275
35	eatb9z6addwbch021q63cz9xt0og6j0d	用户空间035	docspacepersonalroot000000000a	0	personal	g4dp1tsgez5vvn1a2hq7vpgrireuwr3f	g4dp1tsgez5vvn1a2hq7vpgrireuwr3f	2026-09-17 05:31:13.56662	2026-09-17 05:31:13.56662
36	z5nii3ln50huspfqih3zevo1x7yvizpt	用户空间036	docspacepersonalroot000000000a	0	personal	xh9e2z238dalqjxse0gdlrteaf49325f	xh9e2z238dalqjxse0gdlrteaf49325f	2026-09-17 05:31:13.566916	2026-09-17 05:31:13.566916
37	ktbm073la7o1jt4vk686uu3yl4cqpi4z	用户空间037	docspacepersonalroot000000000a	0	personal	p6k1ds5468pmzbz1i6v4814lt5475m3m	p6k1ds5468pmzbz1i6v4814lt5475m3m	2026-09-17 05:31:13.567224	2026-09-17 05:31:13.567224
38	fhnl3j9iyfb2ltyry254v8z3k2gctkvd	用户空间038	docspacepersonalroot000000000a	0	personal	nxbyzuyiu6ouyygcuzemmac873aljn3j	nxbyzuyiu6ouyygcuzemmac873aljn3j	2026-09-17 05:31:13.567551	2026-09-17 05:31:13.567551
39	htcr4p2lb185v7ehl66v151mi9n6ym0a	用户空间039	docspacepersonalroot000000000a	0	personal	f386t32hk8sghcnvf24g3st3i41phl9o	f386t32hk8sghcnvf24g3st3i41phl9o	2026-09-17 05:31:13.567936	2026-09-17 05:31:13.567936
40	vxofagsi9de7r6g7rk5g6ila1qpzfiks	用户空间040	docspacepersonalroot000000000a	0	personal	qu9owum1ui0elni61b5i29akyl7jt03p	qu9owum1ui0elni61b5i29akyl7jt03p	2026-09-17 05:31:13.568275	2026-09-17 05:31:13.568275
41	sbfxvszpdqpgic7xm1xwallmhp4dmdqf	用户空间041	docspacepersonalroot000000000a	0	personal	b7qp8p0xj1okbz6vxrmzngd32jsonytp	b7qp8p0xj1okbz6vxrmzngd32jsonytp	2026-09-17 05:31:13.568577	2026-09-17 05:31:13.568577
42	miot6psm8ijn2bx7v0tsnknw4je45kuo	用户空间042	docspacepersonalroot000000000a	0	personal	qh1et5ylq2fm1e82d32fl67gvnqo6riq	qh1et5ylq2fm1e82d32fl67gvnqo6riq	2026-09-17 05:31:13.569118	2026-09-17 05:31:13.569118
43	uhps7ohrqh8zkvbmcyqii9lna86uqwna	用户空间043	docspacepersonalroot000000000a	0	personal	tyrhx8gab3hzay0xmz65aia5j5xzjcuo	tyrhx8gab3hzay0xmz65aia5j5xzjcuo	2026-09-17 05:31:13.569512	2026-09-17 05:31:13.569512
44	r0fpizfh5d84g8rf5niqjkt3jhgrbs6e	用户空间044	docspacepersonalroot000000000a	0	personal	wlkajsoyh27oscyja111i6bq5e5cd5gx	wlkajsoyh27oscyja111i6bq5e5cd5gx	2026-09-17 05:31:13.56981	2026-09-17 05:31:13.56981
45	ctr32hdpdq4he215l5v6h20sts69ctda	用户空间045	docspacepersonalroot000000000a	0	personal	ki2q6fgdt8o56u3vn2ckanvoy34qqq9a	ki2q6fgdt8o56u3vn2ckanvoy34qqq9a	2026-09-17 05:31:13.570093	2026-09-17 05:31:13.570093
46	thbr8tah425br0g110fmolpeehd6tvcg	用户空间046	docspacepersonalroot000000000a	0	personal	lsinkrec95cl8xk41pc1xa0nwp77r7nq	lsinkrec95cl8xk41pc1xa0nwp77r7nq	2026-09-17 05:31:13.570423	2026-09-17 05:31:13.570423
47	ieibtufs8cmlas16sbp2qmcekvajkv0g	用户空间047	docspacepersonalroot000000000a	0	personal	u7keg12zsf1fp6wxnd0w78r03lk9p5ne	u7keg12zsf1fp6wxnd0w78r03lk9p5ne	2026-09-17 05:31:13.570753	2026-09-17 05:31:13.570753
48	tnrdj8jaquto4ahpykrdcw4l0sxsm7zk	用户空间048	docspacepersonalroot000000000a	0	personal	vj9e202tchk0m7ar1pg1w2xvcb52770u	vj9e202tchk0m7ar1pg1w2xvcb52770u	2026-09-17 05:31:13.571046	2026-09-17 05:31:13.571046
49	d8x4k17q2mqb6e0357hamy8vjrfqx54h	用户空间049	docspacepersonalroot000000000a	0	personal	bdlps6p632yb96o44b00s23l1eywr0sy	bdlps6p632yb96o44b00s23l1eywr0sy	2026-09-17 05:31:13.571338	2026-09-17 05:31:13.571338
50	ms07suzmxrxot03ywfekezlp884zhtsm	用户空间050	docspacepersonalroot000000000a	0	personal	fd5893szdczhr9ylo1cwbrsbb7f6s0np	fd5893szdczhr9ylo1cwbrsbb7f6s0np	2026-09-17 05:31:13.571637	2026-09-17 05:31:13.571637
51	yoxze8ejlou3qztcq005qgkmrba5f0gt	用户空间051	docspacepersonalroot000000000a	0	personal	fisec94xg1zmsoe73xqi8s7pcioxs2xy	fisec94xg1zmsoe73xqi8s7pcioxs2xy	2026-09-17 05:31:13.571913	2026-09-17 05:31:13.571913
52	wbal6jkpp099m71wrglbx3f2peygaxcy	用户空间052	docspacepersonalroot000000000a	0	personal	qjnllgil3gdfowp1khpin6o2b6egz7vs	qjnllgil3gdfowp1khpin6o2b6egz7vs	2026-09-17 05:31:13.572197	2026-09-17 05:31:13.572197
53	g0k5jlf3jdsucbud3bcahfedxnhj261c	用户空间053	docspacepersonalroot000000000a	0	personal	et5k755mj9ndmzcz5uqj79o0se8a7hjq	et5k755mj9ndmzcz5uqj79o0se8a7hjq	2026-09-17 05:31:13.572515	2026-09-17 05:31:13.572515
54	povr2j52bpp0fodbn2m70wd7bswrkvzv	用户空间054	docspacepersonalroot000000000a	0	personal	z1cnqvbthuj6j2ocm19p5wivwknkwwxb	z1cnqvbthuj6j2ocm19p5wivwknkwwxb	2026-09-17 05:31:13.57283	2026-09-17 05:31:13.57283
55	tasib2q73mtcolky7q2mvvomlkwkcvrw	用户空间055	docspacepersonalroot000000000a	0	personal	fgyo9wgof0kcalr2r8cskc2g0bejiw4r	fgyo9wgof0kcalr2r8cskc2g0bejiw4r	2026-09-17 05:31:13.57316	2026-09-17 05:31:13.57316
56	k33f1crbg3aaed535zjqt33r49totrsv	用户空间056	docspacepersonalroot000000000a	0	personal	zzye39s02viahffg8futr2nq2m8ahw9v	zzye39s02viahffg8futr2nq2m8ahw9v	2026-09-17 05:31:13.57346	2026-09-17 05:31:13.57346
57	ywki86iv45srbrnhg22w08gtwo81w9iv	用户空间057	docspacepersonalroot000000000a	0	personal	nwesstx94ue6ekr832lgdj7zz1r81yol	nwesstx94ue6ekr832lgdj7zz1r81yol	2026-09-17 05:31:13.573791	2026-09-17 05:31:13.573791
58	wsambm1rsmnj6r5kyqw4r3jhi79kzm0h	用户空间058	docspacepersonalroot000000000a	0	personal	fczzjf2lns76m9avq50s302n8bfv6kxy	fczzjf2lns76m9avq50s302n8bfv6kxy	2026-09-17 05:31:13.574117	2026-09-17 05:31:13.574117
59	w8jb3g69cs5rq2tz3t7re2lnfu060eqk	用户空间059	docspacepersonalroot000000000a	0	personal	ljkgwn4eowlvuh9aafzdc0ykhj06ptpw	ljkgwn4eowlvuh9aafzdc0ykhj06ptpw	2026-09-17 05:31:13.574454	2026-09-17 05:31:13.574454
60	lj54tvm0pm2aovd3u4mbacub10wugs7m	用户空间060	docspacepersonalroot000000000a	0	personal	rbfrd3phrnqazpxz83q61q87t3s6znun	rbfrd3phrnqazpxz83q61q87t3s6znun	2026-09-17 05:31:13.574744	2026-09-17 05:31:13.574744
61	ifwu8tv5yzsidg38y354kl0rlowlju7u	用户空间061	docspacepersonalroot000000000a	0	personal	qdroqjw2xx7ckth36lsw9lfmexyf1fbt	qdroqjw2xx7ckth36lsw9lfmexyf1fbt	2026-09-17 05:31:13.575053	2026-09-17 05:31:13.575053
62	d264z72m0sbg9h1vrexzpawp3d0n6hjv	用户空间062	docspacepersonalroot000000000a	0	personal	nadp1hsoga0hqcw16pynzk19h7ehcj5r	nadp1hsoga0hqcw16pynzk19h7ehcj5r	2026-09-17 05:31:13.575411	2026-09-17 05:31:13.575411
63	hto00em2ygcw3co42dpxavixb21wbadk	用户空间063	docspacepersonalroot000000000a	0	personal	l5iu02zzxae7gw6denbs5m430pnakwco	l5iu02zzxae7gw6denbs5m430pnakwco	2026-09-17 05:31:13.575724	2026-09-17 05:31:13.575724
64	sfkrfvhfr51b7u5pttfdda46tguk9lqc	用户空间064	docspacepersonalroot000000000a	0	personal	g5zzcwa6lljcra9bh95ot0wmdbo8bb1v	g5zzcwa6lljcra9bh95ot0wmdbo8bb1v	2026-09-17 05:31:13.576112	2026-09-17 05:31:13.576112
65	n4g2oox4z75frpcl0fz8b2v2muaebvdh	用户空间065	docspacepersonalroot000000000a	0	personal	idg18sc5jlnnbs02myk619oj5s8kstej	idg18sc5jlnnbs02myk619oj5s8kstej	2026-09-17 05:31:13.576432	2026-09-17 05:31:13.576432
66	elu2azi2qybnkw3gccpayr7wyiv0bcsr	用户空间066	docspacepersonalroot000000000a	0	personal	ap2xvhdaf6feytr8bbfa0x563uuoztsm	ap2xvhdaf6feytr8bbfa0x563uuoztsm	2026-09-17 05:31:13.576867	2026-09-17 05:31:13.576867
67	ubphudgh1tjltggzjxfnh3byfpy2abum	用户空间067	docspacepersonalroot000000000a	0	personal	aqjsus7aopdpz778evymkbst0mwg6s4l	aqjsus7aopdpz778evymkbst0mwg6s4l	2026-09-17 05:31:13.577164	2026-09-17 05:31:13.577164
68	wn8zvaayqa64e7a76jyaxs3dkahpdjtv	用户空间068	docspacepersonalroot000000000a	0	personal	y9i1zy0zq9jtjx57eleglzfvurltdf9v	y9i1zy0zq9jtjx57eleglzfvurltdf9v	2026-09-17 05:31:13.577469	2026-09-17 05:31:13.577469
69	jg0qa46cp0lnqlna9306kcl5fnhxvwxb	用户空间069	docspacepersonalroot000000000a	0	personal	swy9yjech3it00fnpuk9m5t3ww50f0zu	swy9yjech3it00fnpuk9m5t3ww50f0zu	2026-09-17 05:31:13.577791	2026-09-17 05:31:13.577791
70	prfscx8e2n6s46ek97kx0dv52wzf8yer	用户空间070	docspacepersonalroot000000000a	0	personal	kp0b2zujdinw3quweijsa8k1fn5jo29l	kp0b2zujdinw3quweijsa8k1fn5jo29l	2026-09-17 05:31:13.578145	2026-09-17 05:31:13.578145
71	baovzmetksfjcdifphi5phlggg1nmu7v	用户空间071	docspacepersonalroot000000000a	0	personal	t6bh2x83k6hg6haogt4loyl3dw283d1d	t6bh2x83k6hg6haogt4loyl3dw283d1d	2026-09-17 05:31:13.578483	2026-09-17 05:31:13.578483
72	gt3bnp6bew0idi28n068pnbwb6bwrdul	用户空间072	docspacepersonalroot000000000a	0	personal	nk12a6f4keadmc12tfw69abqnb0hco5d	nk12a6f4keadmc12tfw69abqnb0hco5d	2026-09-17 05:31:13.578788	2026-09-17 05:31:13.578788
73	o5vkqbfqnmrnklxph6l0nhbpa5v10zij	用户空间073	docspacepersonalroot000000000a	0	personal	zqb590opd1gatrzyadk6xax7e7bx500y	zqb590opd1gatrzyadk6xax7e7bx500y	2026-09-17 05:31:13.57911	2026-09-17 05:31:13.57911
74	gy15bbo3zuwyhcqqzw4315e0rwe3ifra	用户空间074	docspacepersonalroot000000000a	0	personal	hyztdi4uhb1w7ndfi47zpp1y5m16soai	hyztdi4uhb1w7ndfi47zpp1y5m16soai	2026-09-17 05:31:13.579442	2026-09-17 05:31:13.579442
75	e7zlq7wa67aywfacxxxcccfqbcci9g5o	用户空间075	docspacepersonalroot000000000a	0	personal	kb38c9j616dragwwgbyo8p7j4vk1xnub	kb38c9j616dragwwgbyo8p7j4vk1xnub	2026-09-17 05:31:13.579729	2026-09-17 05:31:13.579729
76	em96srj2vbwwtf83dwmkxyjjuz7spm0h	用户空间076	docspacepersonalroot000000000a	0	personal	b0f6pm4d64m6zz6agzg7ff1y80kb112t	b0f6pm4d64m6zz6agzg7ff1y80kb112t	2026-09-17 05:31:13.580023	2026-09-17 05:31:13.580023
77	akunjk0c97l89e9v0gbxdf588y67vafs	用户空间077	docspacepersonalroot000000000a	0	personal	hhqf1lr8ktqflwomy9ywlr3e2g87c1nr	hhqf1lr8ktqflwomy9ywlr3e2g87c1nr	2026-09-17 05:31:13.580341	2026-09-17 05:31:13.580341
78	xfskccp13m385g1mgm0aeal18k2qf4vm	用户空间078	docspacepersonalroot000000000a	0	personal	vfuzzdizy3ladqg0f9bwb22csxl2b8zg	vfuzzdizy3ladqg0f9bwb22csxl2b8zg	2026-09-17 05:31:13.580633	2026-09-17 05:31:13.580633
79	bs41tdu5tj8a6yeunxba5opk8gjj0f2o	用户空间079	docspacepersonalroot000000000a	0	personal	bred3yjuutw1vt7mlwx4259tlez2hyvt	bred3yjuutw1vt7mlwx4259tlez2hyvt	2026-09-17 05:31:13.580908	2026-09-17 05:31:13.580908
80	mnr9phvg5xf153m7yvdbrl9sph0rp5wc	用户空间080	docspacepersonalroot000000000a	0	personal	o109vwi25ey3ru4726gkeedt6yed15po	o109vwi25ey3ru4726gkeedt6yed15po	2026-09-17 05:31:13.581236	2026-09-17 05:31:13.581236
81	t0u4h6ef1lmti79598rbrqgfv796mfgy	用户空间081	docspacepersonalroot000000000a	0	personal	qpsr1il81pv2rpwsygkudq6ec28li8ee	qpsr1il81pv2rpwsygkudq6ec28li8ee	2026-09-17 05:31:13.581575	2026-09-17 05:31:13.581575
82	nhlhk1yy91j8p2e35cwbs9uil2isxe3r	用户空间082	docspacepersonalroot000000000a	0	personal	quguougzmiaoxn29p4jm7vfza0nu2mqn	quguougzmiaoxn29p4jm7vfza0nu2mqn	2026-09-17 05:31:13.581873	2026-09-17 05:31:13.581873
83	mh4p5i5qxwrqxihvy9t7n10kt33atewy	用户空间083	docspacepersonalroot000000000a	0	personal	vbsw1wfxo4xiylrqp4ytlvb0iomct94u	vbsw1wfxo4xiylrqp4ytlvb0iomct94u	2026-09-17 05:31:13.582182	2026-09-17 05:31:13.582182
84	ho6m0ga7fc9wj1e5qgxm1dtcjp3oenkw	用户空间084	docspacepersonalroot000000000a	0	personal	rhdnuq6wo64neftkd68yny35yste2xtx	rhdnuq6wo64neftkd68yny35yste2xtx	2026-09-17 05:31:13.582487	2026-09-17 05:31:13.582487
85	iafzlnc09566gfvnky5q5t5qlhnmmxtw	用户空间085	docspacepersonalroot000000000a	0	personal	p6m3p05a3n48hsaq294d431wybsq07zr	p6m3p05a3n48hsaq294d431wybsq07zr	2026-09-17 05:31:13.5828	2026-09-17 05:31:13.5828
86	el7mw4wgsmjrhq9kz76mejkp84kegnlg	用户空间086	docspacepersonalroot000000000a	0	personal	z5o2otfmasx8ylqa1iolk9z530n08opl	z5o2otfmasx8ylqa1iolk9z530n08opl	2026-09-17 05:31:13.583114	2026-09-17 05:31:13.583114
87	ustogjuakmw98aeqfmgwyckykgj2zlcb	用户空间087	docspacepersonalroot000000000a	0	personal	fxufj8dsgmh7hwzc1zgbgoiptbr9smex	fxufj8dsgmh7hwzc1zgbgoiptbr9smex	2026-09-17 05:31:13.583417	2026-09-17 05:31:13.583417
88	usqjmvm46iehgzql6pua2iy4evejgw2z	用户空间088	docspacepersonalroot000000000a	0	personal	fyqkcyecpap2uqzaocs8q8t8x3cvmvfq	fyqkcyecpap2uqzaocs8q8t8x3cvmvfq	2026-09-17 05:31:13.583863	2026-09-17 05:31:13.583863
89	k3nhadw80z2dpvsz4atc59zcgykapm4n	用户空间089	docspacepersonalroot000000000a	0	personal	v32y4lq3ljmqs3qgv8onxtk16gnutgri	v32y4lq3ljmqs3qgv8onxtk16gnutgri	2026-09-17 05:31:13.584167	2026-09-17 05:31:13.584167
90	dvmdxuxfzcmeif03ka5dtaygiwe4gxdm	用户空间090	docspacepersonalroot000000000a	0	personal	y88icjiuzvqdnpjuh4lype9xt97a3xxr	y88icjiuzvqdnpjuh4lype9xt97a3xxr	2026-09-17 05:31:13.584525	2026-09-17 05:31:13.584525
91	ibwltu381z8p9jbbief1gwi2jn34d77g	用户空间091	docspacepersonalroot000000000a	0	personal	smg1dhkia9fg7hjuycfk8449fz2dzj2a	smg1dhkia9fg7hjuycfk8449fz2dzj2a	2026-09-17 05:31:13.584832	2026-09-17 05:31:13.584832
92	kwpg1bpxsqwem71y378hg6ymyfyelubl	用户空间092	docspacepersonalroot000000000a	0	personal	so85b77vby7plhnvr6pf17c98b9mf0to	so85b77vby7plhnvr6pf17c98b9mf0to	2026-09-17 05:31:13.585122	2026-09-17 05:31:13.585122
93	jds36w9p54v04yk4958hqckfaoqp2ksr	用户空间093	docspacepersonalroot000000000a	0	personal	r8144ojgas0uiprldnk21ckrbw6z9kgy	r8144ojgas0uiprldnk21ckrbw6z9kgy	2026-09-17 05:31:13.585427	2026-09-17 05:31:13.585427
94	uubvb320bgtgdokf77cz03rirjx5iuls	用户空间094	docspacepersonalroot000000000a	0	personal	tdunjhhjpzmmiw9eu8a9shulxqbfp8ec	tdunjhhjpzmmiw9eu8a9shulxqbfp8ec	2026-09-17 05:31:13.585768	2026-09-17 05:31:13.585768
95	rh5l9m6s3ge4awqalbyxn0cem0hnv26i	用户空间095	docspacepersonalroot000000000a	0	personal	mxlhyiou8vg6szccawm0zmpug4gg5xrb	mxlhyiou8vg6szccawm0zmpug4gg5xrb	2026-09-17 05:31:13.58607	2026-09-17 05:31:13.58607
96	duexerntb0vptwav22mcw5trmpq9m7lz	用户空间096	docspacepersonalroot000000000a	0	personal	nq2gkeoricp2ufl5ksvv6pznixkj7ifz	nq2gkeoricp2ufl5ksvv6pznixkj7ifz	2026-09-17 05:31:13.586376	2026-09-17 05:31:13.586376
97	mnaep3m8bi1xlib8ztg030shv7ld1w4i	用户空间097	docspacepersonalroot000000000a	0	personal	hnvmzuzx6jup48fa2ow28va7vmjmipua	hnvmzuzx6jup48fa2ow28va7vmjmipua	2026-09-17 05:31:13.586676	2026-09-17 05:31:13.586676
98	r9h32ryx9lbysgqzw5xv0sjketxgd7se	用户空间098	docspacepersonalroot000000000a	0	personal	eieus50pzqqvzj1ge6l72zoina4v91gy	eieus50pzqqvzj1ge6l72zoina4v91gy	2026-09-17 05:31:13.586954	2026-09-17 05:31:13.586954
99	dzlsj3znit980wlj5uy0hhb96ait5foy	用户空间099	docspacepersonalroot000000000a	0	personal	rump0avc7t7htk5y52r61h7n1l90jv9f	rump0avc7t7htk5y52r61h7n1l90jv9f	2026-09-17 05:31:13.587227	2026-09-17 05:31:13.587227
100	vdcx2765g5xnzlvc1t9g450hlgv5kicu	用户空间100	docspacepersonalroot000000000a	0	personal	fy7jo8iycp36dkl4rjko3oy6r8vknu3m	fy7jo8iycp36dkl4rjko3oy6r8vknu3m	2026-09-17 05:31:13.587484	2026-09-17 05:31:13.587484
101	izyrgn9cilpydg0ydp3c2a7hm0hdw8nv	用户空间101	docspacepersonalroot000000000a	0	personal	e2ctnmnw5esjmd5o5gp7zg2gw3f6mnwm	e2ctnmnw5esjmd5o5gp7zg2gw3f6mnwm	2026-09-17 05:31:13.587768	2026-09-17 05:31:13.587768
102	gypcd51u4lvcgt7esz0827ih2mjbr8vq	用户空间102	docspacepersonalroot000000000a	0	personal	ouzpq54s44roqs97m7ii1breztn12gtu	ouzpq54s44roqs97m7ii1breztn12gtu	2026-09-17 05:31:13.588144	2026-09-17 05:31:13.588144
103	wevg233847ewfztwgomfu51lo74qdbrg	用户空间103	docspacepersonalroot000000000a	0	personal	s29fp7shxnshfy13z03uzw0rpnp9rkrm	s29fp7shxnshfy13z03uzw0rpnp9rkrm	2026-09-17 05:31:13.588448	2026-09-17 05:31:13.588448
\.


--
-- Data for Name: aa_doc_document; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_doc_document (id, doc_id, title, summary, file_name, file_type, file_size, file_hash, content_text, char_count, chunk_count, embed_status, embed_error, source, storage_path, dir_id, visibility, created_by, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_doc_grant; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_doc_grant (id, target_type, target_id, action, user_ids, dept_id, role_ids, post_ids, exclude, group_id, set_id, created_by, created_at) FROM stdin;
\.


--
-- Name: aa_doc_access_log_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_doc_access_log_id_seq', 1, false);


--
-- Name: aa_doc_chunk_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_doc_chunk_id_seq', 1, false);


--
-- Name: aa_doc_dir_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_doc_dir_id_seq', 103, true);


--
-- Name: aa_doc_document_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_doc_document_id_seq', 1, false);


--
-- Name: aa_doc_grant_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_doc_grant_id_seq', 1, false);


--
-- Name: aa_doc_access_log aa_doc_access_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_doc_access_log
    ADD CONSTRAINT aa_doc_access_log_pkey PRIMARY KEY (id);


--
-- Name: aa_doc_chunk aa_doc_chunk_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_doc_chunk
    ADD CONSTRAINT aa_doc_chunk_pkey PRIMARY KEY (id);


--
-- Name: aa_doc_dir aa_doc_dir_dir_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_doc_dir
    ADD CONSTRAINT aa_doc_dir_dir_id_key UNIQUE (dir_id);


--
-- Name: aa_doc_dir aa_doc_dir_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_doc_dir
    ADD CONSTRAINT aa_doc_dir_pkey PRIMARY KEY (id);


--
-- Name: aa_doc_document aa_doc_document_doc_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_doc_document
    ADD CONSTRAINT aa_doc_document_doc_id_key UNIQUE (doc_id);


--
-- Name: aa_doc_document aa_doc_document_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_doc_document
    ADD CONSTRAINT aa_doc_document_pkey PRIMARY KEY (id);


--
-- Name: aa_doc_grant aa_doc_grant_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_doc_grant
    ADD CONSTRAINT aa_doc_grant_pkey PRIMARY KEY (id);


--
-- Name: idx_aa_doc_access_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_doc_access_created ON public.aa_doc_access_log USING btree (created_at DESC);


--
-- Name: idx_aa_doc_access_doc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_doc_access_doc ON public.aa_doc_access_log USING btree (doc_id, created_at DESC);


--
-- Name: idx_aa_doc_chunk_doc; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_doc_chunk_doc ON public.aa_doc_chunk USING btree (doc_id, seq);


--
-- Name: idx_aa_doc_dir_parent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_doc_dir_parent ON public.aa_doc_dir USING btree (parent_id);


--
-- Name: idx_aa_doc_document_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_doc_document_created ON public.aa_doc_document USING btree (created_at DESC);


--
-- Name: idx_aa_doc_document_dir; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_doc_document_dir ON public.aa_doc_document USING btree (dir_id);


--
-- Name: idx_aa_doc_document_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_doc_document_hash ON public.aa_doc_document USING btree (file_hash);


--
-- Name: idx_aa_doc_document_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_doc_document_type ON public.aa_doc_document USING btree (file_type);


--
-- Name: idx_aa_doc_grant_target; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_doc_grant_target ON public.aa_doc_grant USING btree (target_type, target_id, action);


--
-- Name: uq_aa_doc_dir_kind_ref; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_aa_doc_dir_kind_ref ON public.aa_doc_dir USING btree (kind, ref_id) WHERE ((ref_id)::text <> ''::text);


--
-- Name: uq_aa_doc_dir_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_aa_doc_dir_name ON public.aa_doc_dir USING btree (parent_id, name);


--
-- PostgreSQL database dump complete
--

\unrestrict ID6alf558UQN1zSvtAQMGH9ZmkzSeYKKPO1jttadGrIpDeDsAx3Lz9Q3htff7UX

