-- llm_seed_data.sql — LLM 配置类表存量数据（凭据/模型/预算/密钥/项目/标签/终端用户/提示词）
-- 用法：先建库建表（psql -f api/resource/data/llm.sql），再载入本文件：
--   psql -d llm -f api/resource/data/seed/llm_seed_data.sql
-- 说明：
--   - 仅含配置类表，运行时日志（spend_log/daily_spend/error_log/audit_log/health_check）不入 seed。
--   - aa_llm_credential 不再内置任何凭据（历史版本曾误带真实密钥，已清除）——
--     供应商 API Key 请部署后在管理后台「LLM 管理 → 凭据」录入（AES-GCM 加密落库）。
--   - 含显式 id（bigserial），导入后如需新行请先 setval 序列。

--
-- PostgreSQL database dump
--

\restrict KMrGkY2gmzGBnRYD8mddHwVExpcunTl9gwhfyx1byAxmIwDbjFtZUBFbUJqraxN

-- Dumped from database version 16.14 (Ubuntu 16.14-0ubuntu0.24.04.1)
-- Dumped by pg_dump version 16.14 (Ubuntu 16.14-0ubuntu0.24.04.1)

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

--
-- Data for Name: aa_llm_budget; Type: TABLE DATA; Schema: public; Owner: aiarsenal
--



--
-- Data for Name: aa_llm_credential; Type: TABLE DATA; Schema: public; Owner: aiarsenal
--



--
-- Data for Name: aa_llm_end_user; Type: TABLE DATA; Schema: public; Owner: aiarsenal
--



--
-- Data for Name: aa_llm_model; Type: TABLE DATA; Schema: public; Owner: aiarsenal
--



--
-- Data for Name: aa_llm_project; Type: TABLE DATA; Schema: public; Owner: aiarsenal
--



--
-- Data for Name: aa_llm_prompt; Type: TABLE DATA; Schema: public; Owner: aiarsenal
--



--
-- Data for Name: aa_llm_tag; Type: TABLE DATA; Schema: public; Owner: aiarsenal
--



--
-- Data for Name: aa_llm_virtual_key; Type: TABLE DATA; Schema: public; Owner: aiarsenal
--



--
-- Name: aa_llm_budget_id_seq; Type: SEQUENCE SET; Schema: public; Owner: aiarsenal
--

SELECT pg_catalog.setval('public.aa_llm_budget_id_seq', 1, false);


--
-- Name: aa_llm_credential_id_seq; Type: SEQUENCE SET; Schema: public; Owner: aiarsenal
--

SELECT pg_catalog.setval('public.aa_llm_credential_id_seq', 1, false);


--
-- Name: aa_llm_end_user_id_seq; Type: SEQUENCE SET; Schema: public; Owner: aiarsenal
--

SELECT pg_catalog.setval('public.aa_llm_end_user_id_seq', 1, false);


--
-- Name: aa_llm_model_id_seq; Type: SEQUENCE SET; Schema: public; Owner: aiarsenal
--

SELECT pg_catalog.setval('public.aa_llm_model_id_seq', 1, false);


--
-- Name: aa_llm_project_id_seq; Type: SEQUENCE SET; Schema: public; Owner: aiarsenal
--

SELECT pg_catalog.setval('public.aa_llm_project_id_seq', 1, false);


--
-- Name: aa_llm_prompt_id_seq; Type: SEQUENCE SET; Schema: public; Owner: aiarsenal
--

SELECT pg_catalog.setval('public.aa_llm_prompt_id_seq', 1, false);


--
-- Name: aa_llm_tag_id_seq; Type: SEQUENCE SET; Schema: public; Owner: aiarsenal
--

SELECT pg_catalog.setval('public.aa_llm_tag_id_seq', 1, false);


--
-- Name: aa_llm_virtual_key_id_seq; Type: SEQUENCE SET; Schema: public; Owner: aiarsenal
--

SELECT pg_catalog.setval('public.aa_llm_virtual_key_id_seq', 1, false);


--
-- PostgreSQL database dump complete
--

\unrestrict KMrGkY2gmzGBnRYD8mddHwVExpcunTl9gwhfyx1byAxmIwDbjFtZUBFbUJqraxN

