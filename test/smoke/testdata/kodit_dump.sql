--
-- PostgreSQL database dump
--

-- Dumped from database version 17.4 (Debian 17.4-1.pgdg120+2)
-- Dumped by pg_dump version 17.4 (Debian 17.4-1.pgdg120+2)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: bm25_catalog; Type: SCHEMA; Schema: -; Owner: postgres
--

CREATE SCHEMA bm25_catalog;


ALTER SCHEMA bm25_catalog OWNER TO postgres;

--
-- Name: tokenizer_catalog; Type: SCHEMA; Schema: -; Owner: postgres
--

CREATE SCHEMA tokenizer_catalog;


ALTER SCHEMA tokenizer_catalog OWNER TO postgres;

--
-- Name: pg_tokenizer; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pg_tokenizer WITH SCHEMA tokenizer_catalog;


--
-- Name: EXTENSION pg_tokenizer; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION pg_tokenizer IS 'pg_tokenizer';


--
-- Name: vector; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;


--
-- Name: EXTENSION vector; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION vector IS 'vector data type and ivfflat and hnsw access methods';


--
-- Name: vchord; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS vchord WITH SCHEMA public;


--
-- Name: EXTENSION vchord; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION vchord IS 'vchord: Vector database plugin for Postgres, written in Rust, specifically designed for LLM';


--
-- Name: vchord_bm25; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS vchord_bm25 WITH SCHEMA bm25_catalog;


--
-- Name: EXTENSION vchord_bm25; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION vchord_bm25 IS 'vchord_bm25: A postgresql extension for bm25 ranking algorithm';


--
-- Name: embeddingtype; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.embeddingtype AS ENUM (
    'CODE',
    'TEXT'
);


ALTER TYPE public.embeddingtype OWNER TO postgres;

--
-- Name: enrichmenttype; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.enrichmenttype AS ENUM (
    'UNKNOWN',
    'SUMMARIZATION'
);


ALTER TYPE public.enrichmenttype OWNER TO postgres;

--
-- Name: indexstatustype; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.indexstatustype AS ENUM (
    'PENDING',
    'IN_PROGRESS',
    'COMPLETED',
    'FAILED'
);


ALTER TYPE public.indexstatustype OWNER TO postgres;

--
-- Name: sourcetype; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.sourcetype AS ENUM (
    'UNKNOWN',
    'FOLDER',
    'GIT'
);


ALTER TYPE public.sourcetype OWNER TO postgres;

--
-- Name: tasktype; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.tasktype AS ENUM (
    'INDEX_UPDATE'
);


ALTER TYPE public.tasktype OWNER TO postgres;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: alembic_version; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.alembic_version (
    version_num character varying(32) NOT NULL
);


ALTER TABLE public.alembic_version OWNER TO postgres;

--
-- Name: commit_indexes; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.commit_indexes (
    commit_sha character varying(64) NOT NULL,
    status character varying(255) NOT NULL,
    indexed_at timestamp without time zone,
    error_message text,
    files_processed integer NOT NULL,
    processing_time_seconds double precision NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone NOT NULL
);


ALTER TABLE public.commit_indexes OWNER TO postgres;

--
-- Name: embeddings; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.embeddings (
    snippet_id character varying(64) NOT NULL,
    type public.embeddingtype NOT NULL,
    embedding json NOT NULL,
    id integer NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL
);


ALTER TABLE public.embeddings OWNER TO postgres;

--
-- Name: embeddings_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.embeddings_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.embeddings_id_seq OWNER TO postgres;

--
-- Name: embeddings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.embeddings_id_seq OWNED BY public.embeddings.id;


--
-- Name: enrichment_associations; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.enrichment_associations (
    enrichment_id integer NOT NULL,
    entity_type character varying(50) NOT NULL,
    entity_id character varying(255) NOT NULL,
    id integer NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL
);


ALTER TABLE public.enrichment_associations OWNER TO postgres;

--
-- Name: enrichment_associations_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.enrichment_associations_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.enrichment_associations_id_seq OWNER TO postgres;

--
-- Name: enrichment_associations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.enrichment_associations_id_seq OWNED BY public.enrichment_associations.id;


--
-- Name: enrichments_v2; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.enrichments_v2 (
    content text NOT NULL,
    id integer NOT NULL,
    type character varying(255) NOT NULL,
    subtype character varying(255) NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL
);


ALTER TABLE public.enrichments_v2 OWNER TO postgres;

--
-- Name: enrichments_v2_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.enrichments_v2_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.enrichments_v2_id_seq OWNER TO postgres;

--
-- Name: enrichments_v2_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.enrichments_v2_id_seq OWNED BY public.enrichments_v2.id;


--
-- Name: git_branches; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.git_branches (
    repo_id integer NOT NULL,
    name character varying(255) NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    head_commit_sha character varying(64) NOT NULL
);


ALTER TABLE public.git_branches OWNER TO postgres;

--
-- Name: git_commit_files; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.git_commit_files (
    commit_sha character varying(64) NOT NULL,
    path character varying(1024) NOT NULL,
    blob_sha character varying(64) NOT NULL,
    mime_type character varying(255) NOT NULL,
    extension character varying(255) NOT NULL,
    size integer NOT NULL,
    created_at timestamp with time zone NOT NULL
);


ALTER TABLE public.git_commit_files OWNER TO postgres;

--
-- Name: git_commits; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.git_commits (
    commit_sha character varying(64) NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    repo_id integer NOT NULL,
    date timestamp with time zone NOT NULL,
    message text NOT NULL,
    parent_commit_sha character varying(64),
    author character varying(255) NOT NULL
);


ALTER TABLE public.git_commits OWNER TO postgres;

--
-- Name: git_repos; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.git_repos (
    sanitized_remote_uri character varying(1024) NOT NULL,
    remote_uri character varying(1024) NOT NULL,
    cloned_path character varying(1024),
    last_scanned_at timestamp with time zone,
    id integer NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    num_commits integer DEFAULT 0 NOT NULL,
    num_branches integer DEFAULT 0 NOT NULL,
    num_tags integer DEFAULT 0 NOT NULL,
    tracking_type character varying(255) NOT NULL,
    tracking_name character varying(255) NOT NULL
);


ALTER TABLE public.git_repos OWNER TO postgres;

--
-- Name: git_repos_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.git_repos_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.git_repos_id_seq OWNER TO postgres;

--
-- Name: git_repos_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.git_repos_id_seq OWNED BY public.git_repos.id;


--
-- Name: git_tags; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.git_tags (
    repo_id integer NOT NULL,
    name character varying(255) NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    target_commit_sha character varying(64) NOT NULL
);


ALTER TABLE public.git_tags OWNER TO postgres;

--
-- Name: task_status; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.task_status (
    id character varying(255) NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    operation character varying(255) NOT NULL,
    trackable_id integer,
    trackable_type character varying(255),
    parent character varying(255),
    message text NOT NULL,
    state character varying(255) NOT NULL,
    error text NOT NULL,
    total integer NOT NULL,
    current integer NOT NULL
);


ALTER TABLE public.task_status OWNER TO postgres;

--
-- Name: tasks; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tasks (
    id integer NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    dedup_key character varying(255) NOT NULL,
    type character varying(255) NOT NULL,
    payload json NOT NULL,
    priority integer NOT NULL
);


ALTER TABLE public.tasks OWNER TO postgres;

--
-- Name: tasks_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.tasks_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.tasks_id_seq OWNER TO postgres;

--
-- Name: tasks_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.tasks_id_seq OWNED BY public.tasks.id;


--
-- Name: vectorchord_bm25_documents; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.vectorchord_bm25_documents (
    id integer NOT NULL,
    snippet_id character varying(255) NOT NULL,
    passage text NOT NULL,
    embedding bm25_catalog.bm25vector
);


ALTER TABLE public.vectorchord_bm25_documents OWNER TO postgres;

--
-- Name: vectorchord_bm25_documents_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.vectorchord_bm25_documents_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.vectorchord_bm25_documents_id_seq OWNER TO postgres;

--
-- Name: vectorchord_bm25_documents_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.vectorchord_bm25_documents_id_seq OWNED BY public.vectorchord_bm25_documents.id;


--
-- Name: vectorchord_code_embeddings; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.vectorchord_code_embeddings (
    id integer NOT NULL,
    snippet_id character varying(255) NOT NULL,
    embedding public.vector(768) NOT NULL
);


ALTER TABLE public.vectorchord_code_embeddings OWNER TO postgres;

--
-- Name: vectorchord_code_embeddings_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.vectorchord_code_embeddings_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.vectorchord_code_embeddings_id_seq OWNER TO postgres;

--
-- Name: vectorchord_code_embeddings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.vectorchord_code_embeddings_id_seq OWNED BY public.vectorchord_code_embeddings.id;


--
-- Name: vectorchord_text_embeddings; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.vectorchord_text_embeddings (
    id integer NOT NULL,
    snippet_id character varying(255) NOT NULL,
    embedding public.vector(768) NOT NULL
);


ALTER TABLE public.vectorchord_text_embeddings OWNER TO postgres;

--
-- Name: vectorchord_text_embeddings_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.vectorchord_text_embeddings_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.vectorchord_text_embeddings_id_seq OWNER TO postgres;

--
-- Name: vectorchord_text_embeddings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.vectorchord_text_embeddings_id_seq OWNED BY public.vectorchord_text_embeddings.id;


--
-- Name: embeddings id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.embeddings ALTER COLUMN id SET DEFAULT nextval('public.embeddings_id_seq'::regclass);


--
-- Name: enrichment_associations id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.enrichment_associations ALTER COLUMN id SET DEFAULT nextval('public.enrichment_associations_id_seq'::regclass);


--
-- Name: enrichments_v2 id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.enrichments_v2 ALTER COLUMN id SET DEFAULT nextval('public.enrichments_v2_id_seq'::regclass);


--
-- Name: git_repos id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.git_repos ALTER COLUMN id SET DEFAULT nextval('public.git_repos_id_seq'::regclass);


--
-- Name: tasks id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tasks ALTER COLUMN id SET DEFAULT nextval('public.tasks_id_seq'::regclass);


--
-- Name: vectorchord_bm25_documents id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.vectorchord_bm25_documents ALTER COLUMN id SET DEFAULT nextval('public.vectorchord_bm25_documents_id_seq'::regclass);


--
-- Name: vectorchord_code_embeddings id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.vectorchord_code_embeddings ALTER COLUMN id SET DEFAULT nextval('public.vectorchord_code_embeddings_id_seq'::regclass);


--
-- Name: vectorchord_text_embeddings id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.vectorchord_text_embeddings ALTER COLUMN id SET DEFAULT nextval('public.vectorchord_text_embeddings_id_seq'::regclass);


--
-- Data for Name: alembic_version; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.alembic_version (version_num) FROM stdin;
af4c96f50d5a
\.


--
-- Data for Name: commit_indexes; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.commit_indexes (commit_sha, status, indexed_at, error_message, files_processed, processing_time_seconds, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: embeddings; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.embeddings (snippet_id, type, embedding, id, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: enrichment_associations; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.enrichment_associations (enrichment_id, entity_type, entity_id, id, created_at, updated_at) FROM stdin;
1	git_commits	6210867b0b91eaf939414a0efc68fbc3d23a247f	1	2026-02-16 13:31:05.554773+00	2026-02-16 13:31:05.554773+00
2	git_commits	6210867b0b91eaf939414a0efc68fbc3d23a247f	2	2026-02-16 13:31:05.5548+00	2026-02-16 13:31:05.5548+00
3	git_commits	6210867b0b91eaf939414a0efc68fbc3d23a247f	3	2026-02-16 13:31:05.554808+00	2026-02-16 13:31:05.554808+00
4	git_commits	6210867b0b91eaf939414a0efc68fbc3d23a247f	4	2026-02-16 13:31:05.554814+00	2026-02-16 13:31:05.554814+00
5	git_commits	6210867b0b91eaf939414a0efc68fbc3d23a247f	5	2026-02-16 13:31:05.55482+00	2026-02-16 13:31:05.55482+00
6	git_commits	6210867b0b91eaf939414a0efc68fbc3d23a247f	6	2026-02-16 13:31:05.554826+00	2026-02-16 13:31:05.554826+00
7	git_commits	6210867b0b91eaf939414a0efc68fbc3d23a247f	7	2026-02-16 13:31:05.554831+00	2026-02-16 13:31:05.554831+00
8	git_commits	6210867b0b91eaf939414a0efc68fbc3d23a247f	8	2026-02-16 13:31:05.58185+00	2026-02-16 13:31:05.58185+00
9	enrichments_v2	7	9	2026-02-16 13:31:07.539828+00	2026-02-16 13:31:07.539828+00
9	git_commits	6210867b0b91eaf939414a0efc68fbc3d23a247f	10	2026-02-16 13:31:07.544672+00	2026-02-16 13:31:07.544672+00
10	enrichments_v2	6	11	2026-02-16 13:31:07.925211+00	2026-02-16 13:31:07.925211+00
10	git_commits	6210867b0b91eaf939414a0efc68fbc3d23a247f	12	2026-02-16 13:31:07.929004+00	2026-02-16 13:31:07.929004+00
11	enrichments_v2	1	13	2026-02-16 13:31:08.287839+00	2026-02-16 13:31:08.287839+00
11	git_commits	6210867b0b91eaf939414a0efc68fbc3d23a247f	14	2026-02-16 13:31:08.291259+00	2026-02-16 13:31:08.291259+00
12	enrichments_v2	5	15	2026-02-16 13:31:08.337091+00	2026-02-16 13:31:08.337091+00
12	git_commits	6210867b0b91eaf939414a0efc68fbc3d23a247f	16	2026-02-16 13:31:08.340741+00	2026-02-16 13:31:08.340741+00
13	enrichments_v2	2	17	2026-02-16 13:31:08.838137+00	2026-02-16 13:31:08.838137+00
13	git_commits	6210867b0b91eaf939414a0efc68fbc3d23a247f	18	2026-02-16 13:31:08.841763+00	2026-02-16 13:31:08.841763+00
14	enrichments_v2	4	19	2026-02-16 13:31:09.445623+00	2026-02-16 13:31:09.445623+00
14	git_commits	6210867b0b91eaf939414a0efc68fbc3d23a247f	20	2026-02-16 13:31:09.447654+00	2026-02-16 13:31:09.447654+00
15	enrichments_v2	3	21	2026-02-16 13:31:10.684547+00	2026-02-16 13:31:10.684547+00
15	git_commits	6210867b0b91eaf939414a0efc68fbc3d23a247f	22	2026-02-16 13:31:10.687959+00	2026-02-16 13:31:10.687959+00
16	enrichments_v2	8	23	2026-02-16 13:31:12.089686+00	2026-02-16 13:31:12.089686+00
16	git_commits	6210867b0b91eaf939414a0efc68fbc3d23a247f	24	2026-02-16 13:31:12.093295+00	2026-02-16 13:31:12.093295+00
17	git_commits	6210867b0b91eaf939414a0efc68fbc3d23a247f	25	2026-02-16 13:31:14.890148+00	2026-02-16 13:31:14.890148+00
18	git_commits	6210867b0b91eaf939414a0efc68fbc3d23a247f	26	2026-02-16 13:31:14.957396+00	2026-02-16 13:31:14.957396+00
19	git_commits	6210867b0b91eaf939414a0efc68fbc3d23a247f	27	2026-02-16 13:31:22.092219+00	2026-02-16 13:31:22.092219+00
\.


--
-- Data for Name: enrichments_v2; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.enrichments_v2 (content, id, type, subtype, created_at, updated_at) FROM stdin;
def get_field_structure(field):\n    if field.field_type == 'RECORD':\n        return {\n            "name": field.name,\n            "type": field.field_type,\n            "fields": [get_field_structure(f) for f in field.fields]\n        }\n    return {\n        "name": field.name,\n        "type": field.field_type\n    }\n\n# === USAGE EXAMPLES ===\n# From main.get_field_structure:\n    def get_field_structure(field):\n\n# From main.list_bigquery_fields:\n    return str([get_field_structure(field) for field in table_ref.schema])\n	1	development	snippet	2026-02-16 13:31:05.55204+00	2026-02-16 13:31:05.552042+00
def list_bigquery_fields() -> str:\n    print("listing fields")\n    """List the fields in the bigquery-public-data.ga4_obfuscated_sample_ecommerce.events_20210131 table."""\n    client = bigquery.Client()\n    table_ref = client.get_table("bigquery-public-data.ga4_obfuscated_sample_ecommerce.events_20210131")\n    return str([get_field_structure(field) for field in table_ref.schema])\n\n# === DEPENDENCIES ===\n\ndef get_field_structure(field):\n    if field.field_type == 'RECORD':\n        return {\n            "name": field.name,\n            "type": field.field_type,\n            "fields": [get_field_structure(f) for f in field.fields]\n        }\n    return {\n        "name": field.name,\n        "type": field.field_type\n    }	2	development	snippet	2026-02-16 13:31:05.552043+00	2026-02-16 13:31:05.552043+00
def run_query(query: str) -> str:\n    response = asyncio.run(agent.run(query))\n    print("-" * 100)\n    print(f"Q: {query}")\n    print(f"A: {response.output}")\n    print("-" * 100)	3	development	snippet	2026-02-16 13:31:05.552044+00	2026-02-16 13:31:05.552044+00
def query_bigquery(query: str) -> str:\n    """Query the bigquery-public-data.ga4_obfuscated_sample_ecommerce.events_* table with the given query. \n    \n    You must always query the `bigquery-public-data.ga4_obfuscated_sample_ecommerce.events_*` table.\n    The query **MUST** be a valid BigQuery SQL query.\n    Timestamps are in microseconds since epoch, hence use TIMESTAMP_MICROS(timestamp) to convert to a timestamp.\n    """\n    print(f"querying with: {query}")\n    client = bigquery.Client()\n    query_job = client.query(query)\n    try:\n        results = query_job.result()\n        return str([dict(row) for row in results])\n    except Exception as e:\n        raise ModelRetry(str(e))	4	development	snippet	2026-02-16 13:31:05.552044+00	2026-02-16 13:31:05.552045+00
def plot_line_chart(x_data: list[str], y_data: list[float], xaxis_title: str, yaxis_title: str) -> str:\n    """Plot a line chart of the given data using plotly express.\n    """\n    # print(f"plotting line chart with: {x_data} and {y_data}")\n    df = pd.DataFrame({\n        xaxis_title: x_data,\n        yaxis_title: y_data\n    })\n\n    fig = px.line(df, x=xaxis_title, y=yaxis_title)\n    fig.update_layout(xaxis_title=xaxis_title, yaxis_title=yaxis_title)\n    fig.show()\n    return "Chart displayed"	5	development	snippet	2026-02-16 13:31:05.552045+00	2026-02-16 13:31:05.552045+00
def main():\n    print("Hello from analytics-ai-agent-demo!")	6	development	snippet	2026-02-16 13:31:05.552046+00	2026-02-16 13:31:05.552046+00
def add(a: int, b: int) -> int:\n    """Add two numbers."""\n    return a + b	7	development	snippet	2026-02-16 13:31:05.552046+00	2026-02-16 13:31:05.552047+00
User: Hi!\nAI: Hello! How can I assist you today?\n\nUser: What is 2+2?\nAI: 2 + 2 equals 4.\n\nUser: Add 5 to that.\nAI: 4 plus 5 equals 9.\n\n----------------------------------------------------------------------------------------------------\nQ: Run a query to find out what time period do you have data for?\nA: The data is available for the following time period:\n\n- **First Event Time:** October 31, 2020\n- **Last Event Time:** February 1, 2021\n----------------------------------------------------------------------------------------------------\nQ: What was the total revenue generated in November 2020?\nA: The total revenue generated in November 2020 was $144,260.00.\n----------------------------------------------------------------------------------------------------\nQ: How many sales (by count) were made in November 2020?\nA: In November 2020, there were a total of 2,054 sales made.\n----------------------------------------------------------------------------------------------------\nQ: Which month had the highest revenue?\nA: The month with the highest revenue was December 2020, with a total revenue of $160,555.00.\n----------------------------------------------------------------------------------------------------\nQ: Which traffic source provided the highest revenue overall?\nA: The traffic source that provided the highest revenue overall is **Google**, with a total revenue of **$104,831.00**.\n----------------------------------------------------------------------------------------------------\nQ: Stratify the revenue in December 2020 by traffic source, then compare that to November 2020.\nA: Here is the stratified revenue by traffic source for December 2020 and November 2020:\n\n### December 2020 Revenue by Traffic Source\n1. **Google**: $46,363\n2. **(Direct)**: $36,233\n3. **<Other>**: $35,029\n4. **(Data Deleted)**: $22,086\n5. **shop.googlemerchandisestore.com**: $20,844\n\n### November 2020 Revenue by Traffic Source\n1. **Google**: $40,597\n2. **<Other>**: $33,055\n3. **(Direct)**: $31,133\n4. **(Data Deleted)**: $21,826\n5. **shop.googlemerchandisestore.com**: $17,649\n\n### Comparison\n- The traffic source "Google" saw an increase in revenue from November to December, rising from $40,597 to $46,363.\n- The "(Direct)" traffic source also increased from $31,133 in November to $36,233 in December.\n- The "<Other>" category showed a small increase from $33,055 to $35,029.\n- The "(Data Deleted)" traffic source remained relatively stable, with a slight increase from $21,826 to $22,086.\n- Conversely, "shop.googlemerchandisestore.com" increased from $17,649 to $20,844.\n\nOverall, December 2020 saw stronger revenue across most traffic sources compared to November 2020.\n----------------------------------------------------------------------------------------------------\nQ: Plot the revenue for each day for all the data you have access to.\nA: The revenue for each day has been successfully plotted. You can see the daily revenue trends over the specified period. If you have any further questions or need additional insights, feel free to ask!\n----------------------------------------------------------------------------------------------------	8	development	example	2026-02-16 13:31:05.5802+00	2026-02-16 13:31:05.580202+00
This Python function `add` is a simple arithmetic function that:\n\n1. **Defines a function** named `add` that takes **two integer parameters** (`a` and `b`).\n2. **Uses type hints** (`: int` for parameters, `-> int` for return type) to indicate that both inputs and the output should be integers.\n3. **Includes a docstring** (`"""Add two numbers."""`) for documentation, explaining the function's purpose.\n4. **Returns the sum** of `a` and `b` (`a + b`).\n\n### Key Features:\n- **Pure function**: No side effects; only computes and returns a result.\n- **Basic arithmetic**: Performs a straightforward addition operation.\n- **Type safety**: Uses type hints for better code clarity and IDE support.\n\nThis is a minimal, well-structured example of a function in Python.	9	development	snippet_summary	2026-02-16 13:31:07.53395+00	2026-02-16 13:31:07.533959+00
This code snippet defines a simple Python function called `main()` that:\n\n1. **Prints a greeting message** to the console:\n   ```python\n   print("Hello from analytics-ai-agent-demo!")\n   ```\n   - The `print()` function outputs the string `"Hello from analytics-ai-agent-demo!"` followed by a newline.\n\n2. **Entry point of execution**:\n   - This function is typically the starting point of a script when run directly (e.g., via `python script.py`).\n   - If this script were executed, it would display the message above.\n\n### Key Notes:\n- The function itself does **not** execute automatically unless explicitly called (e.g., `main()`).\n- This is a minimal example, likely part of a larger project (e.g., a demo for an AI analytics agent). The actual functionality would reside in other parts of the codebase.	10	development	snippet_summary	2026-02-16 13:31:07.920554+00	2026-02-16 13:31:07.920563+00
This code defines a recursive function called `get_field_structure` that generates a nested dictionary representing the structure of a field, particularly useful for handling complex data types like **RECORD** (or nested objects).\n\n### Explanation:\n1. **Function Purpose**:\n   - `get_field_structure(field)` recursively traverses a field's structure and returns a structured dictionary representation.\n   - It handles two cases:\n     - **Simple fields** (non-RECORD): Returns a dictionary with `name` and `type`.\n     - **Nested RECORD fields**: Returns a dictionary with `name`, `type`, and a list of its subfields (recursively processed).\n\n2. **Recursion**:\n   - If `field.field_type == 'RECORD'`, it processes each subfield (`field.fields`) recursively, building a nested structure.\n   - Otherwise, it returns a flat representation.\n\n3. **Usage Examples**:\n   - The first snippet shows the function being used to serialize a field (likely from a database schema, e.g., BigQuery).\n   - The second snippet (`list_bigquery_fields`) demonstrates converting a table's schema (a list of fields) into a structured string representation using `get_field_structure`.\n\n### Key Use Case:\nThis is typically used for **schema introspection** (e.g., BigQuery, SQLAlchemy, or ORM models) to generate a human-readable or machine-friendly representation of nested data structures. The output is useful for logging, debugging, or API documentation.	11	development	snippet_summary	2026-02-16 13:31:08.283485+00	2026-02-16 13:31:08.283494+00
This function `plot_line_chart` generates and displays a **line chart** using **Plotly Express** (`px`). Here's a breakdown:\n\n### **Purpose**\n- Takes two lists (`x_data` and `y_data`) and their corresponding axis titles (`xaxis_title`, `yaxis_title`).\n- Creates a **line plot** where:\n  - `x_data` values are plotted on the **x-axis** (with the given `xaxis_title`).\n  - `y_data` values are plotted on the **y-axis** (with the given `yaxis_title`).\n- Displays the chart interactively (using `fig.show()`).\n- Returns a confirmation string (`"Chart displayed"`).\n\n---\n\n### **Key Steps**\n1. **Data Preparation**\n   - Combines `x_data` and `y_data` into a **Pandas DataFrame** with column names `xaxis_title` and `yaxis_title`.\n\n2. **Plot Creation**\n   - Uses `px.line()` to generate a line chart from the DataFrame.\n   - Specifies `x` and `y` axes using the provided titles.\n\n3. **Layout Customization**\n   - Explicitly sets axis titles again (redundant but ensures clarity).\n\n4. **Display & Return**\n   - Shows the chart (interactive by default).\n   - Returns a simple confirmation message.\n\n---\n\n### **Notes**\n- **Dependencies**: Requires `pandas` (`pd`) and `plotly.express` (`px`).\n- **Side Effect**: The function **displays the chart** (not returns it as an image or object).\n- **Potential Improvement**: Could return the `fig` object instead of a string for further customization.\n\nWould you like an example of how to call this function?	12	development	snippet_summary	2026-02-16 13:31:08.333014+00	2026-02-16 13:31:08.33302+00
This code defines a function `list_bigquery_fields()` that retrieves and formats the schema (field structure) of a BigQuery table. Here's a breakdown:\n\n### **Purpose**\n- Retrieves the schema (column names and types) of the public BigQuery table:\n  `bigquery-public-data.ga4_obfuscated_sample_ecommerce.events_20210131`\n- Returns a structured JSON-like string representing the schema.\n\n---\n\n### **Key Components**\n1. **`list_bigquery_fields()`**\n   - Initializes a BigQuery client (`bigquery.Client()`).\n   - Fetches the table reference (`table_ref`) for the specified table.\n   - Uses a list comprehension to recursively process each field using `get_field_structure()`.\n   - Returns a string representation of the schema as a nested dictionary.\n\n2. **`get_field_structure(field)` (Helper Function)**\n   - Handles two cases:\n     - If the field is a **`RECORD`** (nested structure), it recursively processes its subfields.\n     - Otherwise, it returns a simple dictionary with `name` and `type`.\n\n---\n\n### **Output Structure**\nThe returned string is a **nested dictionary** representing the schema, where:\n- Top-level keys are field names.\n- If a field is a `RECORD`, it contains a `fields` array with its subfields.\n- Example output snippet:\n  ```json\n  [\n      {"name": "event_name", "type": "STRING"},\n      {\n          "name": "user_properties",\n          "type": "RECORD",\n          "fields": [\n              {"name": "key", "type": "STRING"},\n              {"name": "value", "type": "STRING"}\n          ]\n      }\n  ]\n  ```\n\n---\n\n### **Dependencies**\n- Requires the `google-cloud-bigquery` library for BigQuery operations.\n- Assumes the table exists and is accessible.\n\nThis is useful for inspecting complex BigQuery schemas programmatically.	13	development	snippet_summary	2026-02-16 13:31:08.833576+00	2026-02-16 13:31:08.833585+00
This Python function `query_bigquery` executes a BigQuery SQL query on the `bigquery-public-data.ga4_obfuscated_sample_ecommerce.events_*` dataset. Here's a breakdown:\n\n### **Purpose**\n- Runs a **BigQuery SQL query** against a public dataset (`ga4_obfuscated_sample_ecommerce`).\n- Converts results into a **list of dictionaries** (for easy JSON-like parsing).\n\n---\n\n### **Key Steps**\n1. **Input Validation**\n   - The function expects a **valid BigQuery SQL query** (`query: str`).\n   - The query **must** target `bigquery-public-data.ga4_obfuscated_sample_ecommerce.events_*` tables.\n\n2. **Logging**\n   - Prints the query being executed (`print(f"querying with: {query}")`).\n\n3. **BigQuery Client Setup**\n   - Initializes a `bigquery.Client()` to interact with Google BigQuery.\n\n4. **Query Execution**\n   - Runs the query (`client.query(query)`) and waits for results (`query_job.result()`).\n\n5. **Result Conversion**\n   - Converts each row into a dictionary (`dict(row)`) and returns a list of these dictionaries (`str([dict(row) for row in results])`).\n   - **Note:** The `str()` wrapper ensures the output is a string (likely for compatibility with downstream systems).\n\n6. **Error Handling**\n   - If the query fails, it raises a `ModelRetry` exception (likely for retry logic in the calling code).\n\n---\n\n### **Important Notes**\n- **Timestamp Handling**: The docstring mentions using `TIMESTAMP_MICROS(timestamp)` for time conversions (since timestamps are in microseconds).\n- **Public Dataset**: The function is designed for the **GA4 obfuscated e-commerce dataset** (a sample dataset for analytics).\n- **Retry Mechanism**: The `ModelRetry` exception suggests this function is part of a retryable workflow (e.g., in an ORM or API client).\n\n---\n### **Potential Improvements**\n- **Parameterized Queries**: Avoid SQL injection by using `query_job = client.query(query, job_config=bigquery.QueryJobConfig(query_parameters=params))`.\n- **Pagination**: Handle large result sets with `query_job.result(page_size=1000)`.\n- **Type Hints**: Add return type hints for better IDE support (e.g., `List[Dict[str, Any]]`).\n- **Logging**: Replace `print` with a proper logger (e.g., `logging.info`).\n\nWould you like any modifications or clarifications?	14	development	snippet_summary	2026-02-16 13:31:09.442474+00	2026-02-16 13:31:09.442479+00
This code defines a function `run_query` that executes an asynchronous query and prints the results in a formatted way. Here's a breakdown:\n\n1. **Function Signature**:\n   ```python\n   def run_query(query: str) -> str:\n   ```\n   - Takes a string `query` as input.\n   - Returns a string (though the return value isn't explicitly used in the function body).\n\n2. **Asynchronous Execution**:\n   ```python\n   response = asyncio.run(agent.run(query))\n   ```\n   - Runs an asynchronous function `agent.run(query)` using `asyncio.run()`.\n   - `agent.run(query)` is assumed to be an async method that processes the input `query` and returns a response object (likely with an `output` attribute).\n\n3. **Printing Formatted Output**:\n   ```python\n   print("-" * 100)\n   print(f"Q: {query}")  # Prints the input query\n   print(f"A: {response.output}")  # Prints the response's output\n   print("-" * 100)\n   ```\n   - Displays a separator (`-----` line) for visual clarity.\n   - Prints the query and its corresponding response in a structured format (`Q:` for query, `A:` for answer).\n   - Ends with another separator line.\n\n### Key Notes:\n- **Assumptions**:\n  - `agent.run(query)` is an async function (e.g., `async def run(self, query)`).\n  - The `response` object returned by `agent.run(query)` has an `output` attribute (e.g., `response.output`).\n- **Return Value**:\n  The function returns `None` implicitly (no explicit `return` statement), but the docstring suggests it should return a string. This might be a bug or oversight.\n- **Async Context**:\n  `asyncio.run()` should only be called at the top level of a script (not inside other async functions). If this function is called from an async context, it will raise an error.	15	development	snippet_summary	2026-02-16 13:31:10.678594+00	2026-02-16 13:31:10.678604+00
## Services List\n*(No services detected; inferred as monolithic or library)*\n\n## Service Dependencies\n*(No dependencies detected; monolithic or standalone)*\n\n## Mermaid Diagram\n```mermaid\ngraph TD\n    A[Monolith/Unknown] --> B[External/Cloud]\n```\n\n## Key Information\n1. **Databases**: None detected; likely external or abstracted.\n2. **Critical services**: None; architecture appears self-contained.\n3. **Unusual patterns**: None; no inferred service interactions.	17	architecture	physical	2026-02-16 13:31:14.884979+00	2026-02-16 13:31:14.884988+00
This example demonstrates **interactive data analysis and visualization** through a conversational interface, likely using a **SQL-based backend** (e.g., querying a database) and **data aggregation** to answer business questions.\n\n### Key Concepts/Patterns:\n1. **Time-based aggregation** (e.g., filtering by month, comparing periods).\n2. **Grouping and stratification** (e.g., breaking down revenue by traffic source).\n3. **Dynamic query generation** (e.g., translating natural language questions into SQL or analytical logic).\n4. **Visualization integration** (e.g., plotting daily revenue trends programmatically).\n\n### When to Use This Approach:\n- For **business intelligence (BI) dashboards** or **analyst tools** where users ask ad-hoc questions.\n- When working with **time-series data** (e.g., sales, traffic, revenue) requiring flexible filtering.\n- In **data-driven applications** where natural language interfaces (NLU) or SQL-like query parsing are used.\n- For **automated reporting** where insights are generated dynamically from a database.\n\nThis pattern is common in tools like **Power BI, Tableau, or custom analytics platforms** with SQL backends.	16	development	example_summary	2026-02-16 13:31:12.084598+00	2026-02-16 13:31:12.084608+00
# python API Reference\n\n- [91983239377d5cbb-ent-demo.main](#91983239377d5cbb-ent-demo-main)\n- [bigquery.main](#bigquery-main)\n- [simple-memory.main](#simple-memory-main)\n\n## 91983239377d5cbb-ent-demo.main\n\n### Functions\n\n#### main\n\n```py\nmain()\n```\n\n## bigquery.main\n\n### Functions\n\n#### get_field_structure\n\n```py\nget_field_structure()\n```\n\n#### list_bigquery_fields\n\n```py\nlist_bigquery_fields()\n```\n\n#### plot_line_chart\n\n```py\nplot_line_chart()\n```\n\nPlot a line chart of the given data using plotly express.\n\n#### query_bigquery\n\n```py\nquery_bigquery()\n```\n\nQuery the bigquery-public-data.ga4_obfuscated_sample_ecommerce.events_* table with the given query. \n\nYou must always query the `bigquery-public-data.ga4_obfuscated_sample_ecommerce.events_*` table.\nThe query **MUST** be a valid BigQuery SQL query.\nTimestamps are in microseconds since epoch, hence use TIMESTAMP_MICROS(timestamp) to convert to a timestamp.\n\n#### run_query\n\n```py\nrun_query()\n```\n\n## simple-memory.main\n\n### Functions\n\n#### add\n\n```py\nadd()\n```\n\nAdd two numbers.\n\n	18	usage	api_docs	2026-02-16 13:31:14.955849+00	2026-02-16 13:31:14.955851+00
# **Analytics AI Agent Demo Cookbook**\n*A practical guide to querying e-commerce data with AI agents using BigQuery and Pydantic-AI*\n\nThis cookbook demonstrates how to use the **`analytics-ai-agent-demo`** library to interactively query e-commerce data from **Google BigQuery** using AI-driven agents. The examples focus on **core functionalities** provided by the `bigquery` module, including querying structured data, analyzing trends, and generating insights.\n\n---\n\n## **Example 1: Querying BigQuery for Basic E-Commerce Metrics**\n**What it does:** Executes a SQL query against the **`ga4_obfuscated_sample_ecommerce.events_*`** dataset to retrieve revenue and sales data for a specific time period.\n\n**Code:**\n```python\nfrom bigquery.main import query_bigquery\n\n# Define a SQL query to fetch revenue and sales for November 2020\nquery = """\n    SELECT\n        DATE(event_date) AS event_date,\n        SUM(revenue) AS total_revenue,\n        COUNT(*) AS total_sales\n    FROM\n        `bigquery-public-data.ga4_obfuscated_sample_ecommerce.events_*`\n    WHERE\n        DATE(event_date) BETWEEN '2020-11-01' AND '2020-11-30'\n    GROUP BY\n        event_date\n    ORDER BY\n        event_date\n"""\n\n# Execute the query and print results\nresults = query_bigquery(query)\nprint("Revenue and Sales in November 2020:")\nfor row in results:\n    print(f"Date: {row['event_date']}, Revenue: ${row['total_revenue']:.2f}, Sales: {row['total_sales']}")\n```\n\n**Explanation:**\n- Uses `query_bigquery()` (the only explicitly documented query function) to run SQL against the **GA4 obfuscated sample dataset**.\n- Filters for **November 2020** and aggregates **revenue** and **sales count**.\n- **When to use:** When you need **structured data extraction** for time-based analysis.\n\n---\n\n## **Example 2: Listing Available Fields in the BigQuery Dataset**\n**What it does:** Retrieves the schema (column names and types) of the **`events_*`** tables in the GA4 dataset to understand available fields before querying.\n\n**Code:**\n```python\nfrom bigquery.main import list_bigquery_fields\n\n# Get the schema of the dataset\nschema = list_bigquery_fields()\n\nprint("Available fields in the GA4 e-commerce dataset:")\nfor table, fields in schema.items():\n    print(f"\\nTable: {table}")\n    for field in fields:\n        print(f"  - {field['name']} ({field['type']})")\n```\n\n**Explanation:**\n- Uses `list_bigquery_fields()` (documented in the API) to inspect the **schema** of the dataset.\n- Helps avoid **query errors** by confirming field names and data types.\n- **When to use:** When you need to **explore available data** before writing queries.\n\n---\n\n## **Example 3: Generating a Line Chart for Revenue Trends**\n**What it does:** Queries revenue data and visualizes it as a **line chart** using Plotly Express.\n\n**Code:**\n```python\nfrom bigquery.main import query_bigquery, plot_line_chart\nimport pandas as pd\n\n# Query revenue by month for 2020\nquery = """\n    SELECT\n        DATE(event_date) AS month,\n        SUM(revenue) AS monthly_revenue\n    FROM\n        `bigquery-public-data.ga4_obfuscated_sample_ecommerce.events_*`\n    WHERE\n        DATE(event_date) BETWEEN '2020-01-01' AND '2020-12-31'\n    GROUP BY\n        month\n    ORDER BY\n        month\n"""\n\n# Execute query and convert to DataFrame\nresults = query_bigquery(query)\ndf = pd.DataFrame(results)\n\n# Plot the revenue trend\nplot_line_chart(df, x="month", y="monthly_revenue", title="Monthly Revenue Trend (2020)")\n```\n\n**Explanation:**\n- Combines `query_bigquery()` for data retrieval and `plot_line_chart()` for visualization.\n- **When to use:** When you need to **visualize trends** (e.g., seasonal revenue patterns).\n\n---\n\n## **Example 4: Finding the Highest-Revenue Traffic Source**\n**What it does:** Queries and ranks traffic sources by revenue to identify the most profitable channels.\n\n**Code:**\n```python\nfrom bigquery.main import query_bigquery\n\n# Query revenue by traffic source\nquery = """\n    SELECT\n        traffic_source,\n        SUM(revenue) AS total_revenue\n    FROM\n        `bigquery-public-data.ga4_obfuscated_sample_ecommerce.events_*`\n    GROUP BY\n        traffic_source\n    ORDER BY\n        total_revenue DESC\n    LIMIT 5\n"""\n\n# Execute and display results\nresults = query_bigquery(query)\nprint("Top 5 Traffic Sources by Revenue:")\nfor row in results:\n    print(f"{row['traffic_source']}: ${row['total_revenue']:.2f}")\n```\n\n**Explanation:**\n- Uses `query_bigquery()` to **rank traffic sources** by revenue.\n- **When to use:** When you need to **identify high-performing marketing channels**.\n\n---\n\n## **Key Takeaways**\n1. **For structured data analysis**, use `query_bigquery()` with SQL queries.\n2. **Before querying**, inspect the schema with `list_bigquery_fields()`.\n3. **Visualize trends** using `plot_line_chart()` for better insights.\n4. **Focus on business questions** (e.g., revenue by traffic source, monthly trends).\n\n### **Next Steps**\n- Extend queries to **filter by date ranges** or **custom metrics**.\n- Integrate with **AI agents** (e.g., Pydantic-AI) for **natural language querying**.\n- Explore **additional BigQuery datasets** for deeper analysis.\n\n---\n**⚠️ Note:** Ensure you have:\n- **GCP credentials** (`gcloud auth application-default login`).\n- **OPENAI_API_KEY** set (`export OPENAI_API_KEY=...`).\n- **Access to the public dataset** (`bigquery-public-data.ga4_obfuscated_sample_ecommerce`).	19	usage	cookbook	2026-02-16 13:31:22.089892+00	2026-02-16 13:31:22.089895+00
\.


--
-- Data for Name: git_branches; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.git_branches (repo_id, name, created_at, updated_at, head_commit_sha) FROM stdin;
1	main	2026-02-16 13:31:05.446697+00	2026-02-16 13:31:05.446699+00	6210867b0b91eaf939414a0efc68fbc3d23a247f
\.


--
-- Data for Name: git_commit_files; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.git_commit_files (commit_sha, path, blob_sha, mime_type, extension, size, created_at) FROM stdin;
6210867b0b91eaf939414a0efc68fbc3d23a247f	/root/.kodit/clones/91983239377d5cbb-ent-demo/.gitignore	0a197900e25d259ab4af2e31e78501787d7a6daa	application/octet-stream	gitignore	3443	2025-05-20 11:08:54+00
6210867b0b91eaf939414a0efc68fbc3d23a247f	/root/.kodit/clones/91983239377d5cbb-ent-demo/.python-version	e4fba2183587225f216eeada4c78dfab6b2e65f5	application/octet-stream	python-version	5	2025-05-20 11:08:54+00
6210867b0b91eaf939414a0efc68fbc3d23a247f	/root/.kodit/clones/91983239377d5cbb-ent-demo/LICENSE	261eeb9e9f8b2b4b0d119366dda99c6fd7d35c64	application/octet-stream	unknown	11357	2025-05-20 11:08:54+00
6210867b0b91eaf939414a0efc68fbc3d23a247f	/root/.kodit/clones/91983239377d5cbb-ent-demo/README.md	1862a53a16b36216e4c7d0163d1644d4ba94c7dd	text/markdown	md	3733	2025-05-20 11:08:54+00
6210867b0b91eaf939414a0efc68fbc3d23a247f	/root/.kodit/clones/91983239377d5cbb-ent-demo/bigquery/main.py	2f5cec24fbeb1aeea2ed0c6c5a2759cb417a8782	text/x-python	py	3117	2025-05-20 11:08:54+00
6210867b0b91eaf939414a0efc68fbc3d23a247f	/root/.kodit/clones/91983239377d5cbb-ent-demo/docs/revenue.png	564ab478e740ea06038e3821d65db7559d6caab5	image/png	png	89580	2025-05-20 11:08:54+00
6210867b0b91eaf939414a0efc68fbc3d23a247f	/root/.kodit/clones/91983239377d5cbb-ent-demo/main.py	369ec14f72d4cd1ab4b54e049e9564d19e8ca44d	text/x-python	py	101	2025-05-20 11:08:54+00
6210867b0b91eaf939414a0efc68fbc3d23a247f	/root/.kodit/clones/91983239377d5cbb-ent-demo/pyproject.toml	1a4df8197d9f74fef344b73852b5ee8b9aa2df25	application/octet-stream	toml	336	2025-05-20 11:08:54+00
6210867b0b91eaf939414a0efc68fbc3d23a247f	/root/.kodit/clones/91983239377d5cbb-ent-demo/simple-memory/main.py	759166d005ac7c7ac4207b22c69f77e3d51bf8f1	text/x-python	py	711	2025-05-20 11:08:54+00
6210867b0b91eaf939414a0efc68fbc3d23a247f	/root/.kodit/clones/91983239377d5cbb-ent-demo/uv.lock	d041b520f8d1fae82a210b48eb93e839a0ca78ad	application/octet-stream	lock	157800	2025-05-20 11:08:54+00
\.


--
-- Data for Name: git_commits; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.git_commits (commit_sha, created_at, updated_at, repo_id, date, message, parent_commit_sha, author) FROM stdin;
6210867b0b91eaf939414a0efc68fbc3d23a247f	2026-02-16 13:31:05.51057+00	2026-02-16 13:31:05.510572+00	1	2025-05-20 11:08:54+00	add simple query	9e0c00a1f701e573b3e2fea36e87b40708dad00d	Phil Winder <phil@winder.ai>
\.


--
-- Data for Name: git_repos; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.git_repos (sanitized_remote_uri, remote_uri, cloned_path, last_scanned_at, id, created_at, updated_at, num_commits, num_branches, num_tags, tracking_type, tracking_name) FROM stdin;
https://github.com/winderai/analytics-ai-agent-demo	https://github.com/winderai/analytics-ai-agent-demo	/root/.kodit/clones/91983239377d5cbb-ent-demo	2026-02-16 13:31:05.517498+00	1	2026-02-16 13:31:04.135309+00	2026-02-16 13:31:05.518726+00	1	0	0	branch	main
\.


--
-- Data for Name: git_tags; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.git_tags (repo_id, name, created_at, updated_at, target_commit_sha) FROM stdin;
\.


--
-- Data for Name: task_status; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.task_status (id, created_at, updated_at, operation, trackable_id, trackable_type, parent, message, state, error, total, current) FROM stdin;
kodit.root	2026-02-16 13:30:49.701738+00	2026-02-16 13:30:49.701742+00	kodit.root	\N	\N	\N		started		0	0
kodit.repository-1-kodit.repository.clone	2026-02-16 13:30:49.701738+00	2026-02-16 13:31:05.491232+00	kodit.repository.clone	1	kodit.repository	kodit.root		completed		0	0
kodit.repository-1-kodit.commit.scan	2026-02-16 13:30:49.701738+00	2026-02-16 13:31:05.521359+00	kodit.commit.scan	1	kodit.repository	kodit.root		completed		0	0
kodit.repository-1-kodit.commit.create_cookbook	2026-02-16 13:30:49.701738+00	2026-02-16 13:31:22.095898+00	kodit.commit.create_cookbook	1	kodit.repository	kodit.root	Generating cookbook examples with LLM	completed		4	4
kodit.repository-1-kodit.commit.extract_snippets	2026-02-16 13:30:49.701738+00	2026-02-16 13:31:05.559652+00	kodit.commit.extract_snippets	1	kodit.repository	kodit.root	Extracting snippets for python	completed		2	2
kodit.repository-1-kodit.commit.create_example_summary_embeddings	2026-02-16 13:30:49.701738+00	2026-02-16 13:31:14.134187+00	kodit.commit.create_example_summary_embeddings	1	kodit.repository	kodit.root	Creating text embeddings for example summaries	completed		1	1
kodit.repository-1-kodit.commit.extract_examples	2026-02-16 13:30:49.701738+00	2026-02-16 13:31:05.584953+00	kodit.commit.extract_examples	1	kodit.repository	kodit.root	Processing README.md	completed		1	1
kodit.repository-1-kodit.commit.create_bm25_index	2026-02-16 13:30:49.701738+00	2026-02-16 13:31:05.668406+00	kodit.commit.create_bm25_index	1	kodit.repository	kodit.root	BM25 index created for commit	completed		8	8
kodit.repository-1-kodit.commit.create_code_embeddings	2026-02-16 13:30:49.701738+00	2026-02-16 13:31:06.071565+00	kodit.commit.create_code_embeddings	1	kodit.repository	kodit.root	Creating code embeddings for commit	completed		7	7
kodit.repository-1-kodit.commit.create_example_code_embeddings	2026-02-16 13:30:49.701738+00	2026-02-16 13:31:06.163873+00	kodit.commit.create_example_code_embeddings	1	kodit.repository	kodit.root	Creating code embeddings for examples	completed		1	1
kodit.repository-1-kodit.commit.create_architecture_enrichment	2026-02-16 13:30:49.701738+00	2026-02-16 13:31:14.902944+00	kodit.commit.create_architecture_enrichment	1	kodit.repository	kodit.root	Architecture enrichment completed	completed		3	3
kodit.repository-1-kodit.commit.create_public_api_docs	2026-02-16 13:30:49.701738+00	2026-02-16 13:31:14.960518+00	kodit.commit.create_public_api_docs	1	kodit.repository	kodit.root	Extracting API docs for markdown	completed		2	2
kodit.repository-1-kodit.commit.create_summary_enrichment	2026-02-16 13:30:49.701738+00	2026-02-16 13:31:10.698694+00	kodit.commit.create_summary_enrichment	1	kodit.repository	kodit.root	Enriching snippets for commit	completed		7	7
kodit.repository-1-kodit.commit.create_example_summary	2026-02-16 13:30:49.701738+00	2026-02-16 13:31:12.103792+00	kodit.commit.create_example_summary	1	kodit.repository	kodit.root	Enriching examples for commit	completed		1	1
kodit.repository-1-kodit.commit.create_commit_description	2026-02-16 13:30:49.701738+00	2026-02-16 13:31:14.976706+00	kodit.commit.create_commit_description	1	kodit.repository	kodit.root	No diff found for commit	skipped		3	1
kodit.repository-1-kodit.commit.create_summary_embeddings	2026-02-16 13:30:49.701738+00	2026-02-16 13:31:14.039925+00	kodit.commit.create_summary_embeddings	1	kodit.repository	kodit.root	Creating text embeddings for commit	completed		7	7
kodit.repository-1-kodit.commit.create_database_schema	2026-02-16 13:30:49.701738+00	2026-02-16 13:31:14.994376+00	kodit.commit.create_database_schema	1	kodit.repository	kodit.root	No database schemas found in repository	skipped		3	1
\.


--
-- Data for Name: tasks; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.tasks (id, created_at, updated_at, dedup_key, type, payload, priority) FROM stdin;
\.


--
-- Data for Name: vectorchord_bm25_documents; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.vectorchord_bm25_documents (id, snippet_id, passage, embedding) FROM stdin;
1	6	def main():\n    print("Hello from analytics-ai-agent-demo!")	{8:1, 31:1, 420:1, 1300:1, 5201:1, 7968:1, 20191:1, 23882:1, 28412:1, 33600:1, 53397:1}
2	7	def add(a: int, b: int) -> int:\n    """Add two numbers."""\n    return a + b	{8:1, 18:3, 23:3, 420:1, 876:2, 6626:1, 14012:1, 15190:2, 30646:1}
3	5	def plot_line_chart(x_data: list[str], y_data: list[float], xaxis_title: str, yaxis_title: str) -> str:\n    """Plot a line chart of the given data using plotly express.\n    """\n    # print(f"plotting line chart with: {x_data} and {y_data}")\n    df = pd.DataFrame({\n        xaxis_title: x_data,\n        yaxis_title: y_data\n    })\n\n    fig = px.line(df, x=xaxis_title, y=yaxis_title)\n    fig.update_layout(xaxis_title=xaxis_title, yaxis_title=yaxis_title)\n    fig.show()\n    return "Chart displayed"	{5:4, 6:4, 8:1, 18:1, 71:1, 104:2, 113:3, 141:10, 151:5, 257:1, 420:3, 454:19, 915:1, 1022:4, 1238:1, 2053:1, 2256:2, 2459:5, 4527:1, 5259:1, 5303:2, 6056:1, 7704:4, 9254:1, 9613:10, 10135:7, 13315:2, 21917:1, 23577:4, 28412:1, 30646:1, 31374:1, 33102:10, 34475:1, 36510:1, 44116:1, 55334:1, 83671:3, 88684:1, 116287:3, 117008:1}
4	4	def query_bigquery(query: str) -> str:\n    """Query the bigquery-public-data.ga4_obfuscated_sample_ecommerce.events_* table with the given query. \n    \n    You must always query the `bigquery-public-data.ga4_obfuscated_sample_ecommerce.events_*` table.\n    The query **MUST** be a valid BigQuery SQL query.\n    Timestamps are in microseconds since epoch, hence use TIMESTAMP_MICROS(timestamp) to convert to a timestamp.\n    """\n    print(f"querying with: {query}")\n    client = bigquery.Client()\n    query_job = client.query(query)\n    try:\n        results = query_job.result()\n        return str([dict(row) for row in results])\n    except Exception as e:\n        raise ModelRetry(str(e))	{5:7, 6:3, 7:2, 8:1, 13:2, 14:5, 28:3, 41:12, 91:1, 107:1, 144:1, 150:1, 206:1, 208:2, 238:3, 297:2, 308:2, 416:9, 420:1, 433:2, 454:12, 590:4, 617:2, 771:1, 880:1, 944:1, 1238:1, 1294:4, 1733:4, 1927:1, 2053:2, 2109:1, 2676:4, 3299:1, 3522:2, 3835:2, 3996:1, 4460:2, 4527:1, 6957:4, 7136:1, 7514:1, 7704:4, 8110:2, 8311:1, 9030:2, 11948:1, 15555:2, 16750:2, 23282:2, 27227:5, 28412:1, 30646:1, 32976:1, 33209:2, 34475:1, 35604:1, 36791:2, 40129:1, 40494:2, 61669:1, 67779:2, 74563:2, 90141:2, 96760:1, 99247:1, 187840:1, 191633:1}
5	3	def run_query(query: str) -> str:\n    response = asyncio.run(agent.run(query))\n    print("-" * 100)\n    print(f"Q: {query}")\n    print(f"A: {response.output}")\n    print("-" * 100)	{5:3, 8:1, 10:1, 14:1, 41:3, 416:3, 420:1, 454:1, 805:2, 1238:2, 3521:1, 6056:1, 7077:1, 7704:2, 8096:1, 11675:1, 12654:1, 16428:2, 23882:1, 27227:1, 28412:4, 57553:1, 77043:1}
6	2	def list_bigquery_fields() -> str:\n    print("listing fields")\n    """List the fields in the bigquery-public-data.ga4_obfuscated_sample_ecommerce.events_20210131 table."""\n    client = bigquery.Client()\n    table_ref = client.get_table("bigquery-public-data.ga4_obfuscated_sample_ecommerce.events_20210131")\n    return str([get_field_structure(field) for field in table_ref.schema])\n\n# === DEPENDENCIES ===\n\ndef get_field_structure(field):\n    if field.field_type == 'RECORD':\n        return {\n            "name": field.name,\n            "type": field.field_type,\n            "fields": [get_field_structure(f) for f in field.fields]\n        }\n    return {\n        "name": field.name,\n        "type": field.field_type\n    }	{5:13, 7:2, 8:2, 13:2, 14:2, 18:1, 150:1, 192:1, 208:2, 238:1, 297:2, 308:1, 420:2, 433:2, 454:22, 474:3, 617:2, 944:2, 1238:2, 1294:2, 2046:3, 2053:2, 3522:2, 3794:1, 3835:2, 4460:2, 5016:2, 5303:3, 6746:2, 6957:3, 7560:1, 7704:2, 9030:1, 9239:2, 9351:2, 10644:2, 17164:1, 23180:2, 23282:2, 27227:2, 28394:8, 28412:1, 29087:2, 30646:3, 32976:1, 33209:2, 36716:3, 44457:12, 46280:3, 56566:1, 67779:2, 74563:2, 90141:2, 128607:2}
7	1	def get_field_structure(field):\n    if field.field_type == 'RECORD':\n        return {\n            "name": field.name,\n            "type": field.field_type,\n            "fields": [get_field_structure(f) for f in field.fields]\n        }\n    return {\n        "name": field.name,\n        "type": field.field_type\n    }\n\n# === USAGE EXAMPLES ===\n# From main.get_field_structure:\n    def get_field_structure(field):\n\n# From main.list_bigquery_fields:\n    return str([get_field_structure(field) for field in table_ref.schema])\n	{5:9, 8:2, 177:1, 192:1, 420:2, 454:16, 474:5, 944:1, 1238:2, 1294:1, 2046:4, 2424:1, 3794:1, 5201:2, 6562:1, 7560:1, 7704:1, 9239:2, 9351:2, 10644:2, 13280:1, 17164:1, 23180:1, 28394:10, 29087:1, 30646:3, 32976:1, 36716:5, 42276:1, 44457:11, 46280:3}
8	8	User: Hi!\nAI: Hello! How can I assist you today?\n\nUser: What is 2+2?\nAI: 2 + 2 equals 4.\n\nUser: Add 5 to that.\nAI: 4 plus 5 equals 9.\n\n----------------------------------------------------------------------------------------------------\nQ: Run a query to find out what time period do you have data for?\nA: The data is available for the following time period:\n\n- **First Event Time:** October 31, 2020\n- **Last Event Time:** February 1, 2021\n----------------------------------------------------------------------------------------------------\nQ: What was the total revenue generated in November 2020?\nA: The total revenue generated in November 2020 was $144,260.00.\n----------------------------------------------------------------------------------------------------\nQ: How many sales (by count) were made in November 2020?\nA: In November 2020, there were a total of 2,054 sales made.\n----------------------------------------------------------------------------------------------------\nQ: Which month had the highest revenue?\nA: The month with the highest revenue was December 2020, with a total revenue of $160,555.00.\n----------------------------------------------------------------------------------------------------\nQ: Which traffic source provided the highest revenue overall?\nA: The traffic source that provided the highest revenue overall is **Google**, with a total revenue of **$104,831.00**.\n----------------------------------------------------------------------------------------------------\nQ: Stratify the revenue in December 2020 by traffic source, then compare that to November 2020.\nA: Here is the stratified revenue by traffic source for December 2020 and November 2020:\n\n### December 2020 Revenue by Traffic Source\n1. **Google**: $46,363\n2. **(Direct)**: $36,233\n3. **<Other>**: $35,029\n4. **(Data Deleted)**: $22,086\n5. **shop.googlemerchandisestore.com**: $20,844\n\n### November 2020 Revenue by Traffic Source\n1. **Google**: $40,597\n2. **<Other>**: $33,055\n3. **(Direct)**: $31,133\n4. **(Data Deleted)**: $21,826\n5. **shop.googlemerchandisestore.com**: $17,649\n\n### Comparison\n- The traffic source "Google" saw an increase in revenue from November to December, rising from $40,597 to $46,363.\n- The "(Direct)" traffic source also increased from $31,133 in November to $36,233 in December.\n- The "<Other>" category showed a small increase from $33,055 to $35,029.\n- The "(Data Deleted)" traffic source remained relatively stable, with a slight increase from $21,826 to $22,086.\n- Conversely, "shop.googlemerchandisestore.com" increased from $17,649 to $20,844.\n\nOverall, December 2020 saw stronger revenue across most traffic sources compared to November 2020.\n----------------------------------------------------------------------------------------------------\nQ: Plot the revenue for each day for all the data you have access to.\nA: The revenue for each day has been successfully plotted. You can see the daily revenue trends over the specified period. If you have any further questions or need additional insights, feel free to ask!\n----------------------------------------------------------------------------------------------------	{4:15, 5:6, 6:1, 8:10, 18:2, 23:5, 31:1, 34:17, 41:1, 48:1, 56:1, 91:1, 106:3, 116:6, 138:2, 141:5, 158:1, 162:5, 184:3, 185:1, 190:4, 201:4, 206:3, 275:18, 277:3, 387:2, 416:1, 483:1, 502:2, 738:4, 910:2, 1001:1, 1019:2, 1112:2, 1126:2, 1274:1, 1300:3, 1663:4, 1733:4, 1936:1, 1957:1, 1974:3, 1991:3, 2053:6, 2104:1, 2273:2, 2843:1, 3622:5, 3871:1, 3912:2, 4039:2, 4046:3, 4092:1, 4568:1, 5117:1, 5155:2, 5518:2, 5595:10, 7166:7, 7228:2, 7413:1, 7612:5, 7621:2, 7639:1, 7864:1, 8096:7, 8318:2, 8915:3, 8951:3, 9550:2, 9655:1, 11075:14, 11473:2, 11663:1, 11675:1, 12319:1, 12338:1, 12663:2, 12768:1, 13442:1, 14922:3, 15190:1, 17168:1, 17174:1, 17203:1, 18925:1, 19336:1, 19437:10, 19732:2, 19927:3, 20016:1, 21416:2, 21567:2, 23577:2, 24124:2, 24470:2, 25820:1, 26458:1, 27430:2, 28960:1, 30793:1, 30994:1, 31150:2, 33444:1, 33600:1, 36272:1, 36880:1, 37397:2, 37509:1, 37515:1, 38937:3, 41262:1, 45804:3, 47143:1, 48358:2, 48877:2, 51332:1, 52758:17, 54529:1, 58735:1, 58944:1, 59875:3, 64371:1, 66604:1, 82683:1, 83629:10, 103741:3, 105586:2, 105950:2, 110952:1, 125158:2, 127968:2, 143646:10, 146533:1, 149133:1, 149201:1, 162264:1, 167375:4, 167377:2, 171074:2, 171793:1, 180344:2, 191304:2, 225490:1}
\.


--
-- Data for Name: vectorchord_code_embeddings; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.vectorchord_code_embeddings (id, snippet_id, embedding) FROM stdin;
1	6	[-0.008510477,-0.029845383,0.053362794,-0.015754035,0.042327076,-0.037969474,-0.04383684,-0.029718576,0.017140692,-0.051396195,-0.017483959,-0.035879806,0.033004347,-0.013590336,-0.00016510917,0.008329425,0.040808994,0.05157704,-0.07845044,0.07315338,0.02171426,0.0018999144,-0.025487818,-0.025319526,-0.026499413,0.0031776023,-0.03605153,-0.0033359586,-0.006554167,-0.07100734,-0.012739708,0.023827832,-0.008869551,-0.01575378,0.021217147,0.01674862,-0.029254533,-0.007057829,0.026213488,-0.013543172,0.05799759,-0.048051063,0.0002136367,0.049424183,0.029656991,0.015395869,0.012173787,0.054509528,0.026886072,0.0068056732,-0.025902288,0.054906834,0.0055493414,0.036879513,0.043283068,0.03148661,0.03431434,0.007191451,-0.0015772898,-0.02385351,-0.0062260143,0.05434832,-0.040197853,-0.033113793,0.047620412,0.0070551196,0.012255022,-0.014044364,0.023783132,-0.033634078,-0.0336784,0.04408428,0.0018340553,0.0323233,0.009572381,0.015182856,-0.035021197,-0.21830395,-0.013107838,-0.002088115,-0.06929609,-0.027296277,-0.021955369,0.005245792,-0.032748982,0.051505286,0.010521649,0.014700745,0.0033712063,-0.02001371,0.044596046,-0.04054396,0.015348944,0.004623955,0.005771894,-0.0849882,0.023609128,-0.010262775,0.0005410835,0.038778096,-0.01888854,0.012663562,-0.0010216772,-0.038207397,0.020506755,0.013689677,-0.01781261,-0.040776514,-0.01103317,0.042148206,0.03972435,-0.015597538,-0.035969783,0.03414997,-0.056195874,-0.023705294,-0.0017456642,-0.0013816648,-0.017565679,0.041960534,0.0018594965,-0.013821843,-0.03413497,-0.05957344,-0.009192658,0.004051541,-0.001987612,-0.06063714,0.029863458,-0.017121928,0.026262779,-0.023740834,-0.035352763,0.018581878,0.00053159805,0.007422604,0.027875084,0.02192397,-0.007220006,0.041416578,0.017205013,-0.027083643,0.026447328,0.027176714,-0.03102658,0.057678957,-0.032999642,-0.011309453,0.02097666,0.05943191,-0.029027773,-0.03315693,-0.06130886,0.008822332,-0.047031116,0.025728423,-0.058885552,0.030932209,0.02476322,-0.022996495,0.0039690332,0.0104957735,0.020032385,0.023611626,-0.020516371,-0.012636386,-0.025793778,0.0020585486,-0.019539107,0.002474936,-0.043004975,-0.04594428,-0.023570646,0.013678223,-0.04646757,0.04846453,-0.0026037705,0.058354735,0.0031902827,0.03995373,0.015902845,0.002173592,-0.038629342,-0.057103097,0.03609722,-0.007591198,0.04075885,-0.014561125,-0.016025936,0.000491679,-0.015217721,-0.0490239,-0.016817426,0.026071824,0.0062031094,-0.060862843,0.006389983,-0.0101345675,0.018596701,0.012411657,0.0026015635,-0.04205919,0.0433078,-0.022262717,0.04213858,-0.057155333,-0.030771585,0.008850051,0.036323275,0.03646226,-0.01761522,0.00020446104,0.01822754,0.043082815,-0.038927615,0.017912442,0.085480206,0.025977775,0.021894732,0.048635554,-0.02757183,-0.030171422,-0.030620232,0.026371628,-0.051149238,0.0045682406,-0.011155845,-0.0014829409,-0.009310768,0.031933602,-0.0070682643,0.03820721,0.00821569,0.035294153,-0.009177304,-0.036481738,-0.005882703,-0.0027654576,0.011558741,0.007940213,0.0074551054,0.06418091,-0.02943049,0.026890427,0.02370357,-0.07645223,-0.010691003,-0.010122802,0.023752695,0.0064882543,-0.004059534,-0.040011,-0.025390575,-0.021554092,-0.01160813,0.01644046,0.054629195,-0.0012230988,-0.019070255,-0.060281597,-0.038309492,0.07578749,-0.012708092,0.046117187,0.03338906,0.052021977,0.02509872,0.0076644453,0.022267679,-0.00917511,0.032629665,-0.0222684,0.04787311,0.045185003,-0.018084465,0.02201927,0.009134567,0.04278872,-0.0087458985,0.004084474,0.011863725,0.007690607,0.043620776,0.02488708,0.054226834,0.0023778616,0.037885055,-0.007303181,-0.009822525,0.04556229,0.08666669,0.020496164,-0.056162715,0.0028867773,0.04778246,0.01089864,0.010080712,0.028906489,-0.010598996,0.008935185,-0.07265468,-0.02326346,-0.010608902,-0.03924937,0.0123695955,-0.087104455,-0.09581294,0.0730198,0.010216727,0.034932267,-0.01860653,-0.0135346735,0.023537107,-0.0040418706,-0.02309651,0.050739486,0.011970115,0.03407519,0.018007038,-0.04024226,-0.0053860378,-0.04270165,-0.01638229,-0.027159158,0.02315913,0.008051918,-0.002023477,0.069853775,0.01976776,0.05098228,-0.039154515,-0.027333576,-0.02609652,-0.12039698,-0.08767698,-0.050112713,0.0025314558,0.046177976,0.07145005,-0.0181619,0.030751565,0.015294449,0.00471582,0.0015130783,0.047798365,0.0075295637,-0.06865268,-0.04589937,-0.023516234,0.0917592,-0.019046647,-0.063825406,-0.028386585,-0.062490128,0.019843562,0.033068825,-0.0008420116,0.022781761,0.005799673,-0.0349802,0.035647944,0.07901354,0.009999611,-0.051759906,-0.035380945,-0.060097154,0.039943025,-0.0029606943,-0.037827574,-0.017553074,-0.014419212,0.025489504,-0.005302279,-0.015461922,0.060395565,-0.041037098,0.078893386,0.017327359,-0.048482094,-0.0077139,-0.018402312,-0.014152785,-0.0067583504,-0.0063935197,0.021680657,-0.020375257,0.033889182,0.037793107,-0.001309478,-0.04313334,0.054653965,0.06581631,-0.0007626289,0.010427288,0.029527564,0.05855964,-0.0202025,-0.06115683,0.0024977915,0.052131142,-0.02041404,0.026137223,-0.016623858,0.008714486,-0.0331672,0.047754485,0.055383816,0.040838696,-0.018547261,0.07560899,-0.031406227,-0.04370868,0.02366606,-0.01863729,-0.008346475,-0.0141132325,-0.006218609,0.030825285,-0.023906268,0.06913501,-0.03379206,0.0023222426,-0.028142782,-0.05924326,-0.0066834404,-0.0027523877,-0.050803225,0.019938106,0.011997842,-0.028328849,0.033147473,0.0027404956,0.036503132,-0.0018393256,-0.02351767,0.010388096,0.0227723,-0.04233241,-0.0011947266,0.066536866,0.1030096,0.013218134,-0.0077758473,0.017180797,-0.017379021,-0.028103271,-0.0064453897,0.011281927,-0.048294544,-0.044277422,-0.05225508,0.029914077,0.052113026,-0.086473405,-0.04553682,0.06727496,-0.015094905,-0.040849824,0.009874937,-0.05699131,-0.013154524,-0.02322228,0.043066982,0.0191932,-0.013133253,-0.009463911,-0.04662734,0.0013498663,0.01873909,0.014549183,0.00046146184,0.03426857,0.06284942,-0.043780472,0.016691865,-0.04746845,0.058561593,0.019919844,0.02215216,0.01981805,-0.02106508,0.029754886,0.017252706,0.014882266,-0.033268694,-0.013532515,0.014824166,-0.039969787,-0.017374344,-0.0016160714,0.01888209,-0.044363886,-0.013785785,0.01789744,-0.0094722565,0.029073454,-0.026467841,-0.013776417,-0.024666313,-0.011936675,0.023915114,0.021883339,0.0054146713,0.023305472,0.04629247,0.0073442594,-0.02458239,0.022682998,0.010660926,-0.025294887,-0.0016279822,-0.024582813,-0.008037709,-0.04683155,-0.01843141,-0.0105,-0.05713039,0.02203954,0.010204878,0.00079778413,-0.01085805,0.01180854,-0.003319071,-0.017299352,-0.001979174,-0.03709081,-0.014416928,0.012467463,0.034013275,0.029537786,0.0040635117,0.036711268,-0.035052147,0.03133626,0.0074918047,-0.012246428,0.0071778228,-0.016295023,-0.015319959,-0.033743802,0.0876769,-0.011874458,0.026266195,0.013677418,-0.03588396,0.06603825,-0.024460578,0.0067095663,0.019798132,0.01437071,0.023867477,0.09994806,-0.01166426,0.020787442,0.01986188,-0.059323225,-0.024422294,-0.05399474,0.014900761,0.057327576,0.044776503,-0.021383788,-0.0819305,0.029310893,-0.024212273,0.0044380757,0.021982338,-0.058130994,-0.01917684,0.0059207454,0.05902108,-0.04217987,-0.032387655,-0.027676983,-0.033941213,-0.013884789,0.024973046,0.068142496,-0.026354054,0.07604974,-0.041051302,-0.034455333,0.1083088,-0.014833891,0.014007873,0.03396494,0.029125025,0.010009964,0.18159942,-0.048632156,0.010859512,0.013588769,-0.082487114,-0.04624989,-0.006640154,-0.04593978,-0.06855816,0.0028073776,0.04150821,-0.023451949,-0.024145091,-0.057176396,0.049220577,0.02505984,-0.028325194,-0.04600134,-0.0066603445,-0.016315252,0.021000465,0.003073152,0.031546574,0.008428398,-0.0076060374,0.02797653,0.05264244,0.023416251,-0.000804201,0.011133862,-0.023944445,0.020121071,-0.00075417594,0.026665432,-0.06761978,0.034312513,0.08518364,0.04984072,0.020401863,0.043093268,0.043255404,0.020581668,0.0043338463,0.02906818,-0.020367015,0.013622621,-0.02966679,0.044652022,0.043084044,0.04076279,0.015495171,-0.04209953,-0.031388674,0.082812145,-0.0076857144,-0.020373587,-0.02023594,0.0002927351,0.053253803,-0.0050377045,-0.016404605,0.032012284,0.008948009,0.04808625,0.034747645,-0.008649405,-0.004039396,0.039564207,0.010519882,-0.009271017,0.0041583125,-0.005995798,0.04244035,-0.044187304,-0.043886032,-0.010159866,-0.054256205,-0.048874382,0.007604481,-0.015314612,-0.04194362,-0.009232323,-0.009470347,0.06156775,0.0097903665,-0.0041310475,0.060487267,0.052367114,0.01605254,-0.05918799,-0.030681014,-0.030592695,-0.013110075,-0.06042991,-0.043729164,0.027500633,0.051250253,0.019826367,0.0068024374,0.044618,0.015526577,-0.00813504,-0.028695822,0.05671592,0.00019520849,0.03297717,-0.046074025,-0.045574628,0.0767342,0.0049391217,-0.019333672,-0.023574127,0.048484918,-0.045160033,0.0009495115,-0.010316411,-0.015042336,0.039798684,-0.009661251,0.04266497,0.0062705143,0.00938486,0.021343868,0.03499213,-0.013540259,-0.0060042096,0.010877297,-0.04203789,0.022405127,-0.0057604667,0.043205075,0.02519483,-0.00023434103,-0.019547017,0.019576821,-0.049263217,0.015864115,0.04398882,0.010732851,-0.010618232,0.027288254,-0.0023707347,-0.0029360366,0.029897453,0.020609714,-0.034050085,-0.011016037,0.020689707,0.034570776,-0.04331642,-0.04980974,0.038124286,-0.0056306035,0.006835304,-0.032166738,-0.01157478,-0.002380442,-0.040150773,-0.027134342,-0.01306254,-0.023053,0.06388537,0.04216127,0.06614044,0.009833977,-0.018823951,-0.019097907,0.014819489,-0.013021528,0.025343811,0.027762765,-0.029157748,-0.02875612,0.0027610601,-0.040758066,-0.04242646,0.04804689,-0.046690963,-0.028434064,0.03199773]
2	7	[-0.001218469,0.034528326,-0.003478155,0.023567984,-0.063329205,0.0005651192,0.02736338,-0.022443762,-0.034868646,0.009951684,-0.051385995,0.033528227,0.013271089,0.075304046,-0.018484738,0.017226119,0.088228874,0.026728438,0.05819427,0.024093088,-0.003477479,0.04901077,0.053695563,0.017512796,0.000274997,0.010365688,0.069190234,0.035456665,-0.033695836,-0.049494565,-0.06517593,0.009630582,0.009597604,0.016068097,0.024312718,0.037319865,-0.004847115,-0.027022084,0.018650878,0.08448145,-0.0007676994,-0.02920996,0.0069663436,0.017506141,0.019011123,0.019431617,-0.099822216,-0.008592353,0.016846279,-0.005831385,-0.021898016,0.041842815,0.012321411,0.050427876,-0.021880608,0.018172447,0.0008050247,0.041053914,-0.0069662444,-0.045999832,-0.009140246,-0.0597105,0.024888625,0.024520852,0.0013858224,0.03254714,0.0014952121,-0.0636103,-0.006509757,-0.031200482,0.0059386627,0.021431187,0.052112527,-0.051235385,-0.044705346,-0.0034151673,0.0020119767,-0.03200717,0.03128148,-0.016115045,0.009681467,0.02953004,0.013917431,0.044366404,-0.008812907,-0.017963827,-0.045339745,-0.037577465,0.023915995,-0.0114265345,-0.006576583,0.0031343538,0.012901444,0.10651853,-0.042092126,-0.032770544,-0.010124442,0.018992368,-0.0070064333,0.022888524,0.04134415,0.012255461,-0.021801163,-0.033802215,0.039713062,0.03212707,-0.035444625,0.03311303,0.061369095,0.042994104,0.0029191517,-0.015883638,-0.033851977,-0.023648042,0.047874782,-0.027827226,-0.011271468,-0.012093829,-0.09599824,0.037346933,-0.031133011,-0.03528036,-0.019995283,0.040715333,0.046535805,-0.040505018,0.0009071351,-0.014676704,-0.025575364,-0.014686887,-0.05292244,0.01004036,0.023919672,-0.015810868,-0.013819922,0.012527175,0.0018633233,-0.015524301,0.039997086,-0.03465097,-0.0026518626,-0.0012097973,-0.023046497,0.05473495,-0.012286392,-0.008072232,-0.014931085,0.009780489,0.026203755,0.014240932,-0.012683828,0.06891785,-0.02534374,-0.044132236,-0.0611001,0.0067495117,-0.0664359,0.010454404,0.034118537,-0.0025637473,0.023274792,-0.0011055412,-0.06696992,0.0038954152,0.027341709,-0.006905453,0.04957653,-0.047386598,0.058313344,0.03620322,-0.020510474,-0.023708655,-0.0318279,0.01017713,-0.006166312,0.0022615802,0.025362462,0.031203656,0.03293265,-0.031833135,0.033545785,-0.0056191348,-0.017780198,-0.036978252,0.004024008,-0.037357654,0.07181359,0.02866385,0.027026167,0.041448228,0.016906034,-0.068572715,0.034628287,-0.0032138773,0.028083213,0.05431608,-0.019504339,0.011420548,-0.007705403,0.0036541624,0.010171682,-0.05079738,0.027706414,0.03758217,0.00071812695,-0.02940957,-0.02313698,0.012057064,0.05664679,-0.011447747,-0.00039155097,0.011595994,0.011929535,0.055539373,-0.12636426,-0.008219988,-0.06223816,0.05952377,0.081447445,0.034665793,0.016178181,-0.021276899,-0.042775024,0.073714316,0.01035206,-0.001706247,-0.0046841335,-0.021539796,0.0015772651,-0.00018904533,-0.008568652,0.0019714406,-0.014568243,-0.014135919,-0.022720052,0.008099055,-0.023951044,0.022579145,-0.015465639,-0.03787747,-0.06210811,-0.010960711,-0.05317485,0.0008332606,-0.015344314,-0.015241927,-0.040428646,0.11431637,0.007291993,-0.016709173,0.019177012,-0.009873736,-0.008279086,-0.021945877,0.042647503,-0.0044330563,-0.026865853,-0.0023036646,-0.05936189,-0.04679024,-0.05586496,-0.003403925,0.0833237,0.015743094,0.02688829,0.021259584,0.0002662604,-0.029935004,0.013932275,-0.011893893,-0.021915253,-0.063906565,-0.023890987,-0.04090806,0.03872353,0.016974293,0.044385884,-0.016967483,-0.033919547,-0.022342028,0.025755156,-0.060988538,-0.0013413653,0.06521504,0.0720308,0.08634962,-0.04520346,-0.06887034,-0.03691616,0.034424048,0.068989016,-0.0144692715,-0.055228297,0.022574613,0.0069903266,0.003918795,0.08292682,-0.007581293,-0.009646259,-0.021347493,-0.011017256,-0.0033762257,-0.011883913,-0.029717818,0.024914658,0.033382237,-0.08110149,-0.06221126,-0.04065417,-0.028411556,-0.0028563202,0.0028337385,-0.0159906,0.0009871834,-0.00035654855,0.117897905,-0.026256641,0.03507476,-0.008300529,-0.08473971,-0.047888033,0.04432933,0.025180178,-0.02168163,-0.010408882,0.026569115,-0.03367443,0.01967873,0.0023175213,0.034966927,-0.032341715,0.031629473,-0.010245047,-0.037342284,0.01864928,0.010800181,0.05944152,-0.030327203,-0.04051077,0.07007725,0.0027839101,0.050652083,-0.0074872193,-0.00755767,0.0073802876,-0.029473413,-0.024737766,0.07127,0.030457312,-0.07694255,0.0077126985,-0.06834796,0.010412241,0.02034123,0.019368611,-0.031385437,-0.014319907,0.02295125,-0.02179473,-0.10324478,0.0075369366,0.01849566,-0.009120598,-0.009240201,-0.037206445,0.009291789,-0.014941688,0.027221885,-0.074673854,-0.0868116,0.005556527,0.0037892044,-0.02419095,0.07567182,-0.0070709228,-0.02240093,0.011135612,0.04499107,-0.029912638,0.06308909,-0.031074762,-0.009046221,-0.010973053,-0.022269038,0.029164692,-0.002209103,-0.01721995,0.03417462,0.007625106,-0.019108007,0.0942963,-0.0117599,0.03878912,-0.01621306,0.017052818,0.020081665,0.00077231333,-0.04867462,0.011660722,-0.009531124,0.01733874,0.013681485,-0.027047727,0.023632053,-0.0027540713,0.019062491,0.020741355,0.014649185,-0.009945959,0.061341878,0.030461399,0.02125878,0.026423413,0.04189504,0.01711342,-0.03133206,-0.011191145,-0.0059448043,0.02427428,-0.0045527024,-0.05137421,-0.029502517,-0.028226228,0.021288313,-0.030648258,-0.0076907156,0.04913202,0.050266184,-0.012246975,0.008321127,0.023302302,0.032337166,-0.04698473,0.0068934797,-0.022896117,0.0494537,-0.024521012,0.012220726,0.015063902,0.0150507055,0.097418755,0.044546016,0.05167089,-0.017153962,0.005029597,0.0145856235,-0.026644373,0.042708762,-0.036524314,-0.056416903,0.023201587,-0.08713939,-0.054724663,0.013840748,-0.07629351,0.018545449,0.004847375,-0.047175623,0.041501705,0.02206917,0.02676681,-0.030321319,-0.010950575,0.0034341018,0.0031098174,0.021893807,-0.049885426,0.07637167,-0.029526062,-0.011676371,0.032785464,0.05908997,-0.011842349,-0.03648788,0.03381585,0.0013862186,0.08866101,0.008187579,-0.025183503,0.005238075,-0.016765738,0.006681793,0.023646878,-0.062367685,-0.0049207397,0.033439558,0.050191306,-0.011187338,0.011228066,0.023398865,-0.007962894,-0.0065624453,-0.015190488,0.010092614,-0.005364335,0.0038150612,-0.022926975,0.021754865,-0.050045952,-0.039119504,-0.0035651526,0.019799318,-0.012710456,0.025448319,0.030568665,0.07921914,-0.006138492,0.032881524,0.0011525812,-0.0130849155,0.042670447,0.028219437,-0.04523912,-0.011831798,0.0052559786,0.014606242,-0.057388622,0.017372595,0.061038345,0.018280393,0.04932469,-0.00030650187,0.05691216,0.0012129409,-0.025413612,0.02529934,-0.016986553,-9.367826e-05,-0.0013842043,0.01503606,0.010075615,0.05746738,0.001984671,0.035203457,-0.027899368,0.014796218,-0.018540554,-0.009504419,0.020998001,-0.025190659,0.06914914,0.042046323,-0.0068068616,0.010683188,-0.005079337,0.045287598,0.048557207,0.021381868,-0.009059955,0.02141856,0.02666474,-0.054853104,0.006225963,0.0027135664,-0.038718075,-0.019904068,0.0061291596,0.020176932,-0.014555979,0.027327389,-0.00026461174,-0.032299206,-0.023733633,-0.008456695,0.038409524,0.04663798,0.037882924,-0.013817358,0.042732764,0.014177546,0.12555583,-0.04111711,-0.07392046,-0.059156988,-0.063795924,-0.033765122,-0.009483439,0.019985663,-0.080022156,-0.0358181,-0.027618734,0.0022371816,0.01595729,0.07017712,0.0234123,-0.018312518,-0.03802693,0.0014464859,0.09868705,-0.0051150406,-0.015577746,0.005503194,-0.024509234,-0.022315186,-0.010949277,-0.015069683,0.003989282,0.034987185,-0.013799323,-0.03254917,-0.0030632983,0.01789473,0.012967449,0.0058932495,-0.07898128,0.02387178,0.014738225,-0.029862747,-0.013332591,-0.014258421,-0.004871958,0.04626879,0.031014105,0.058224306,0.037762623,-0.010427495,-0.022499494,0.049250547,-0.03597865,-0.07549376,0.032095958,-0.033123028,-0.057590522,-0.07134916,-0.013729893,-0.038588308,-0.0034257064,0.029021397,-0.026404543,-0.0073563172,-0.0377132,-0.041262493,-0.0094932,-0.006430897,-0.05713838,-0.00012314541,-0.06130543,0.0371477,0.027832828,0.079299316,-0.014468243,-0.0183878,-0.02777107,0.012659746,0.012321133,-0.008391569,0.018201292,-9.276513e-05,-0.03448597,-0.03855137,-0.038517352,-0.024388859,0.0011943205,-0.008743337,0.04313957,-0.049405202,-0.00060433446,-0.06874129,-0.030274715,-0.020536697,0.0064623025,0.007335309,0.010311061,-0.07489229,-0.06328367,0.0155316405,-0.014802264,-0.08817967,-0.01504011,0.004191112,0.018372111,0.009178257,-0.030364452,0.039727695,-0.0249776,-0.026973322,-0.021102881,0.0065210275,0.010499955,0.016573373,3.4151755e-05,-0.021237552,0.01211213,0.018670201,0.036099274,0.0018981214,0.06043336,0.06443899,0.013087443,0.025525652,-0.00867093,0.042287372,0.016066669,-0.01918601,-0.018254153,-0.036994096,-0.01718579,-0.04066601,-0.0050689825,-0.027650068,0.016456675,0.121516496,0.009837553,0.0028176606,-0.007884866,-0.005246725,-0.022733158,-0.046064354,0.044394642,0.025214918,0.04202487,0.03296389,-0.03274111,0.029790305,-0.026144309,-0.010013322,0.0021579457,0.066561386,0.029088996,0.021551743,0.025927633,-0.034742903,0.046587545,0.037364952,-0.019323189,0.040077936,-0.02312603,-0.014741686,-0.06766655,0.00343971,-0.011132068,-0.01222498,0.06374906,0.056621358,-0.013119346,-0.01136183,0.060047515,-0.029420314,-0.042741798,0.04232357,-0.008059653,0.0007260452,0.009499545,-0.0022215743,0.019481301,-0.041774694,-0.0067403982,0.01673362,-0.00931477,0.021216875,0.015842088,-0.01823566,0.06540766,-0.009467068,0.02944911,-0.013521535,0.044437602,-0.0037461761,0.014526923,0.03600447,-0.040341236,-0.038398385,-0.069720395,-0.018652774,0.06483693,0.06546466,-0.028782107,0.01743407]
3	5	[0.015572043,-0.013103727,-0.00794245,-0.002868434,0.0017392945,-0.025678776,0.011807164,0.06035904,0.00100698,0.013866765,-0.032087393,-0.030595277,-0.0208787,0.04221346,-0.051242188,-0.010033231,0.081802316,0.025043173,-0.044346884,-0.027876403,-0.016884005,0.06275732,-0.03217111,-0.016268011,-0.038225442,0.029632565,0.0082268575,-0.06158799,0.018507877,-0.073423885,-0.027112477,0.040292267,-0.026053604,-0.020184817,0.068862535,-0.0066152294,0.013074928,0.011110986,-0.04745726,-0.030623704,0.08292962,0.022994574,0.028089048,0.06006057,0.018667396,-0.01703266,0.07835831,-0.019662516,-0.024704918,-0.048372027,-0.00821786,0.09637892,0.036097445,0.023466753,0.06182951,0.03395996,0.008094565,-0.042202007,-0.014290564,-0.0071483306,0.035139963,0.049849585,0.0057113017,0.042965878,0.01913212,0.0772201,0.014195911,0.00648594,0.028056793,0.032371234,-0.013807659,-0.035205603,0.0024382058,-0.054923885,0.013540933,-0.006167547,-0.007996025,-0.04082455,0.07132925,0.028501742,-0.023698173,0.032763176,0.03889751,0.007851442,0.064249985,-0.005069731,0.012543109,0.07427525,0.020426797,-0.039414197,0.030044867,0.04506587,-0.0032186776,0.10398927,0.022326594,-0.046897486,0.018991439,0.053823862,0.053584732,-0.022994218,-0.010391869,-0.029373052,-0.007061096,0.05487274,0.027467294,-0.019101173,0.0025495393,0.014744615,-0.014154548,-0.0020077156,0.017168086,0.05376968,-0.016177481,0.05515858,-0.032943837,-0.058646686,-0.009274351,0.04571829,0.07819793,0.036875993,-0.02879486,0.017970836,0.02004765,0.0091114305,0.016961994,0.012472166,-0.01245905,0.040523797,-0.05997991,0.08292024,-0.017109683,-0.01361894,-0.047211032,-0.032109533,0.04425689,0.0346914,0.0061896886,0.012221535,-0.028618338,-0.011700319,0.04023153,0.01049523,0.009198462,0.01582705,-0.020400124,-0.015352651,-0.042132437,0.035952147,-0.023038602,0.03990958,-0.0024457695,-0.01406925,-0.037266117,-0.011359304,0.031893313,0.025680644,0.026880683,0.05915684,0.02287084,-0.006033748,0.07353895,-0.0816334,-0.008954946,-0.045577593,0.010467188,-0.012617499,-0.04645036,0.036818556,-0.04125672,0.017840888,-0.06013893,0.041675966,-0.05136848,0.019057574,0.0034317828,0.008502853,0.0031207688,0.042457484,-0.010378075,0.0036929504,-0.02070994,0.0014549906,0.0036950298,-0.031678006,-0.0291849,0.011387778,-0.026599776,-0.018234713,-0.029557388,-0.008023292,0.004628397,0.03466332,-0.006063422,0.0243516,-0.0032925778,-0.06151409,-0.008010839,-0.019184683,-0.03737611,-0.030326653,-0.04222112,-0.025849419,0.03622774,0.046729557,-0.029975193,-0.09598461,-0.046569273,0.018824654,0.01668285,0.049310684,0.006317297,0.0039207647,0.0017222079,0.009073952,-0.08240161,0.02388136,0.022618225,0.104783416,0.10957248,0.03228941,0.00012340955,0.039808873,-0.029384408,0.0278908,-0.06460451,0.040201634,0.01633115,0.008406974,-0.021347791,0.049655247,0.018542698,0.008497742,0.02400863,0.027166573,0.042644843,-0.043446507,-0.0015091343,-0.009203365,0.082023345,0.012472606,-0.010875703,-0.0023872901,-0.013479711,0.08250901,-0.02775053,-0.023745833,-0.049658462,-0.0029839976,0.029490769,-0.010592632,0.04294423,0.013407138,-0.020140477,-0.040551726,0.016187327,-0.0062559806,0.025708389,0.016008722,-0.0063720467,-0.09102166,-0.019158574,-0.002402552,0.046940763,0.089767866,0.021267503,0.04505294,-0.030601451,0.0028581414,-0.021332387,-0.008899405,-0.030196656,-0.042443812,0.00915686,0.010882596,0.012373028,-0.02672822,0.11269156,0.023418767,0.10637733,-0.030207241,-0.020281462,-0.02623871,-0.04438521,0.026889103,0.04561883,-0.007932318,-0.023140011,0.07685815,0.02044437,0.018083274,0.0053537693,0.003488961,0.0140523715,0.0238386,-0.008205049,0.000238677,0.04338574,-0.0051320824,0.035856396,-0.003959648,-0.004920584,-0.031663638,0.025453534,0.008240998,-0.03846446,0.0030884745,-0.04187682,0.025998434,-0.021662518,-0.033083394,-0.009877457,-0.0033144879,-6.0652477e-05,0.038227834,0.032680854,-0.0020926269,-0.07572371,0.03704743,0.011390569,0.009527017,-0.041883223,-0.023847014,-0.0026377037,-0.0021382917,-0.06984095,-0.00030025063,-0.06903551,0.0019914433,0.055785626,0.052321788,-0.023955747,-0.03839437,0.0075090276,-0.049643915,-0.002504826,0.018879244,-0.013519322,0.013356125,0.009542174,-0.018625066,-0.020912662,-0.011617808,-0.08725679,0.012681078,0.010288899,-0.0140053,0.0060138283,0.010452767,0.013611948,0.05968193,0.04823181,-0.044464428,-0.0061557037,-0.016282186,0.0040798844,0.06289949,-0.06380129,0.011306172,-0.025546538,0.057086144,0.011140383,-0.004474031,-0.007917131,-0.038003054,-0.030575475,0.010389882,0.021681381,0.041975643,-0.004201766,-0.042086303,0.013427508,0.028037278,0.02806818,-0.04046329,0.010764475,-0.013939578,0.064463764,-0.007986538,0.004872974,-0.016219582,-0.01570715,0.0004198864,0.031954583,-0.0015203307,0.012988185,0.019931061,-0.060608655,0.03480621,0.023449926,-0.0117001105,0.0655545,0.02100614,0.00093418837,0.030895151,0.0033960468,-0.013035624,0.031151224,-0.090178646,-0.014769507,-0.0020265188,-0.012951423,-0.012733942,0.004084684,-0.01827109,0.05252149,-0.0022465116,0.0043739686,0.051344983,0.004648106,-0.03699443,0.017602287,-0.04198511,0.042061366,-0.07258633,-0.04159538,-0.007761165,-0.020374307,-0.06203934,-0.0067522805,-0.05729894,-0.067297086,0.008751715,0.011222321,-0.007583272,-0.10478445,0.037819054,-0.019457715,0.0075790253,-0.07646578,-0.018825177,0.0064900443,-0.02075271,0.02963539,-0.043266613,0.015044519,0.034124043,0.029363874,-0.02455458,-0.03417078,-0.01810619,0.07340705,0.0064550475,-0.0044899117,0.04155743,0.010824201,0.05469206,0.023018371,-0.0050383853,-0.00016828679,0.057653952,0.029172137,-0.03916215,0.011458054,-0.062428888,-0.055823155,0.02935563,-0.02260446,-0.04148394,0.021536995,-0.033599906,0.0024720838,-0.054362535,-0.0003948924,-0.0021633569,-0.035225105,-0.00066492194,-0.028490013,0.029096121,0.0034141606,-0.010096799,-0.012544148,-0.041897986,0.06320333,-0.041021712,0.027117176,0.009079603,0.041964877,0.018030088,-0.002081431,-0.041862514,0.04023915,0.0051606586,-0.023927025,0.041538917,0.023320692,0.008598688,-0.009159528,0.002149626,-0.03446379,-0.027793445,-0.016851822,0.008796722,-0.06451596,0.008652918,0.0020961445,-0.03271614,-0.0014576861,0.05220524,-0.025721427,-0.04010379,-0.0079786405,-0.05200644,-0.008985715,0.028282892,0.02556962,0.0080779055,-0.03815122,0.0012604853,-0.03689233,0.026497534,-0.0012013668,0.05050609,-0.0028036395,-0.017662918,-0.026687201,0.008377743,-0.031902336,0.03340328,-0.039298736,-0.043286067,-0.00472553,-0.011970761,-0.013461422,0.05939969,-0.017091017,-0.0024635114,-0.021826599,0.022702986,0.057430588,0.005096398,-0.07241148,-0.0028773025,-0.00659013,0.007647997,-0.10878728,0.028304344,-0.03290874,-0.014046295,-0.03931567,0.011077509,-0.008720158,-0.03486016,-0.040912177,0.011549415,0.0048966613,0.024424814,-0.028177852,-0.0291076,0.05859184,-0.004249236,0.012077906,-0.0023285423,0.018426381,-0.016212307,-0.020975268,-0.014705529,-0.06740735,-0.038952973,-0.029810306,0.0072065536,-0.014269704,-0.06944478,-0.056235448,-0.037496973,0.07112903,-0.01621504,-0.00482199,0.0277008,0.05902563,0.03748103,-0.01915089,-0.040262792,-0.008863305,-0.022755323,-0.055617712,-0.0109615205,-0.018196994,0.1378092,0.007926411,-0.011394899,-0.11987424,-0.06682744,0.03451527,0.017866695,-0.005713661,0.03749628,-0.02400907,-0.029146899,-0.064916484,0.033129748,0.0022795398,0.022229904,-0.05074595,-0.04382237,-0.029596189,-0.020879997,0.018541384,0.03333369,0.009084563,0.015270976,0.032605343,0.018644184,0.010054008,0.0050222515,-0.008988848,0.019006541,0.008867953,0.0042649796,-0.081532575,-0.022361545,0.028906334,0.02273907,0.039241392,0.0015193917,-0.009966781,0.061751366,-0.020301566,0.0061429553,0.0008758731,0.03947342,-0.010241531,-0.00141592,-0.025756456,0.04351422,-0.031607226,0.0026419144,0.038915534,0.041427054,0.019218415,0.05398653,0.047504943,0.013481287,0.009938175,0.056446016,-0.016443906,0.03912977,0.028144546,-0.045419954,0.023250164,-0.0017086492,-0.009591318,-0.011472501,0.021777777,0.03079157,0.014618125,0.022415817,0.01384529,0.10386895,-0.017245207,-0.014372448,-0.010251118,0.04186181,0.09198383,-0.042079538,0.025722556,-0.042971823,0.020798804,-0.028633159,-0.038834304,-0.017176917,0.040777083,-0.04875984,0.015212431,-0.0927222,-0.015224566,-0.030293962,-0.06127538,-0.011722694,0.004120739,0.0695837,-0.071815036,0.01231341,0.03139107,0.020077005,0.036155745,-0.032549094,-0.012933241,-0.06656472,-0.0216217,-0.029961187,-0.02258933,-0.003945888,-0.03799077,0.0034068364,-0.014230013,0.040403176,-0.015718328,0.051397283,0.02300723,-0.026175268,-0.04262788,0.039970648,0.008017207,0.0145985875,-0.0049874494,-0.045410033,0.08710506,0.018036447,0.0028940924,0.019495286,-0.002397355,-0.013984888,0.014178345,0.03088366,0.03130619,-0.02115879,0.03571105,0.043628987,-0.0040952554,-0.036058065,0.0045660967,0.035971247,-0.05531045,-0.020705745,0.019116383,-0.035329275,0.020285489,0.06280668,-0.006951191,0.010291973,0.027290575,0.0490821,-0.0021129698,-0.02000615,-0.010956161,0.012320317,-0.039962295,-0.018513752,0.0153715545,-0.01577519,-0.023617636,-0.037066694,-0.019092597,0.08323656,-0.02100448,-0.01886332,0.035579428,-0.05554111,0.020771615,0.017137688,-0.029027857,0.0126385335,-0.05352541,-0.01341782,0.005280879,-0.019946333,-0.0059628356,-0.059409626,-0.014411321,-0.0068091913,-0.0072489725,-0.043654524,-0.025042592,-0.007345032,0.001935865,-0.050256815,0.025416164,0.01852629,-0.059926,-0.012243187,0.06093769,-0.028363185,0.026290052,0.0022621057,0.041576788,0.026052304,0.024026906,0.010272503]
4	4	[0.006184551,-0.046439514,0.006832532,0.06577031,-0.0022186516,-0.06853435,-0.0037831126,-0.019768469,-0.046164878,-0.03852382,0.0054557337,-0.039441444,0.013757145,-0.01220216,-0.034811504,0.0127153145,0.06199326,0.017755283,-0.0012055138,-0.014111555,-0.030306552,0.023214333,0.017982211,-0.016003953,-0.007847216,0.046060123,-0.031486604,0.020537553,-0.082344525,-0.045133606,-0.03446342,0.018646503,0.027306182,-0.0016579741,0.13268775,-0.024349269,-0.044545528,-0.03276214,0.04557219,-0.011107183,0.08326384,0.080370836,0.030183606,0.008867436,-0.020118877,-0.038449958,0.00092987483,0.03671016,-0.037418015,0.002941241,0.0071584117,0.038736913,0.027205547,0.051772162,0.019922683,-0.07065161,0.031682074,0.025100684,-0.027741918,-0.023658708,-0.019584376,0.106779926,-0.00916484,0.0137624815,0.024418505,-0.032381777,-0.021119539,-0.07256627,-0.031832695,-0.03373483,0.010681985,-0.026711926,0.041119404,0.054131653,-0.03222775,0.034310427,0.00210923,-0.050997864,0.03376046,0.022364818,-0.00741304,-0.0013134069,0.024904568,0.050074417,-0.012956797,-0.0029646894,0.050835475,0.062263448,0.040554773,-0.013942585,0.01295001,-0.010113724,-0.019683458,0.02924522,0.009458033,-0.037585814,-0.028897107,0.09939594,0.007151996,-0.008209885,-0.0597277,-0.008919896,-0.034588166,-0.04634514,0.013243256,-0.018612567,-0.021051578,-0.02285433,-0.014993161,0.044022027,0.024663012,-0.014920971,0.036836755,0.015902687,0.00449078,-0.039412905,-0.01544065,-0.027636483,0.074926585,-0.014621917,-0.06997477,-0.033746358,-0.093070105,0.0045811385,-0.018836545,0.0005686263,-0.028931214,-0.032533344,-0.009554946,-0.039349936,0.0028137467,-0.0008666568,0.0059790714,0.06132752,-0.020374335,0.014554801,-0.06145955,0.027900385,-0.01250662,-0.0010242994,0.004609666,-0.0014007089,0.03051927,0.009082265,-0.039965257,-0.03610355,-0.014854743,-0.004848438,-0.06995379,0.057460606,0.0016595533,0.020305006,-0.011595057,0.0017984075,0.024161447,0.04333179,0.045205265,0.037333507,-0.034281075,-0.051333524,-0.024171699,-0.013873599,-0.009405463,0.01239325,-0.030733803,0.05199295,0.027344132,-0.0179286,0.010856847,0.05803817,-0.053403307,-0.0034757203,-0.045452066,-0.018418398,-0.02128411,0.030720778,-0.005076745,0.039251007,-0.013210701,0.033638682,0.01729602,0.011104908,-0.013011567,0.01554357,0.0146707,-0.011513538,-0.0019232491,0.04138105,-0.04203975,-0.011379581,-0.020638768,-0.0006142032,0.019894848,0.043084323,0.018172298,-0.11379818,0.0035327412,-0.04496985,0.0071761757,0.00023370243,0.016518323,-0.05821175,-0.044442005,-0.010910527,0.04216041,-0.055889662,0.0082549,0.019265816,0.029579476,0.022676857,0.032787494,-0.03393515,-0.015044103,0.034882735,0.009888028,-0.03229216,0.04515427,0.10816121,0.057916723,0.053904314,-0.007457647,0.005974705,-0.032027885,-0.009733816,-0.0023712835,-0.012098036,-0.0036678964,0.051038858,-0.0036985176,-0.003633368,-0.029279122,0.09968646,0.08746409,0.06443911,0.012078003,0.025678787,0.014470753,-0.02898262,0.016101323,-0.0010273503,-0.06687273,0.02349519,0.037859347,-0.00944743,0.009015504,-0.0485499,-0.04632296,-0.06807957,0.024996122,-0.01938654,0.029650245,-0.026156668,0.021551149,-0.010573,-0.03026833,-0.016707798,0.014167668,-0.003177264,0.090006925,-0.021087749,-0.077061005,0.04537361,0.05079429,0.022238139,-0.03811769,0.074101634,-0.015460788,-0.0074190996,0.036236122,-0.020472355,0.0469488,0.0278324,0.038102478,0.060181815,-0.013513397,0.0063402015,0.012686578,0.055534735,0.02357171,-0.0028123073,-0.02006463,-0.0036801288,0.016588314,0.026540913,0.041610386,-9.811943e-06,-0.038349804,-0.030401051,0.01110997,0.06676394,0.034813005,0.0062094973,-0.10716892,0.009004321,0.018836247,0.007884444,-0.0008716112,0.011416411,0.014162083,-0.008070204,-0.027388966,-0.08540806,-0.021871286,0.03115897,-0.0053096204,0.005055424,-0.0510519,0.07323631,0.026682317,0.056683872,-0.006235417,0.02845454,0.0496115,-0.03565158,-0.0035286038,0.02255037,-0.020054737,0.01715319,0.020338558,-0.020309472,0.005063187,-0.009763676,-0.013084298,0.015855314,0.016700957,0.008444547,-0.007195529,0.019694855,0.03961993,0.0009852026,-0.06835752,-0.050261185,0.039253153,-0.12189597,-0.041302107,-0.0058210017,0.025213497,0.016389491,0.030034443,-0.035448033,0.040195912,0.028160598,0.013812378,-0.0043142075,0.034384947,0.046721734,-0.03929226,-0.031091746,-0.008169354,0.017894253,-0.032329082,-0.06869553,-0.012954095,-0.11866752,-0.031786945,0.013793863,0.008869058,-0.0027475269,0.0037891753,-0.019246956,-0.038089227,-0.003560235,-0.034272835,-0.006215001,-0.021750355,0.040832996,0.08150502,-0.08763046,-0.030545171,-0.030151242,0.009378908,0.045659985,0.0072438973,0.0046845553,-0.011872337,-0.0054063625,-0.030233193,-0.042751543,-0.056966856,0.021348415,0.02604897,0.026251016,0.053016108,-0.027894184,0.022833362,0.01614489,0.021612123,-0.022281403,0.0018696676,0.0017059634,0.0057551144,0.017074274,-0.06282973,0.008986108,0.00877524,0.011279803,0.018830793,-0.005642501,0.033838708,-0.027812274,-0.022041056,0.007258242,-0.014039453,-0.015627269,0.0042043766,-0.014573633,0.059560843,-0.016098293,0.011727046,-0.041435488,0.023935732,0.048695475,-0.014450331,-0.049938865,0.021354087,0.00979182,0.038341705,-0.114301234,-0.04269918,0.031057682,-0.026799392,-0.019126885,0.010262698,0.032957558,0.045006342,-0.006674456,0.01841792,0.01719379,0.0900315,0.016523162,0.01220966,-0.05642222,0.11461624,-0.021604871,-0.053749185,-0.03584234,0.01087421,-0.052052334,0.029403873,0.05071652,0.06913787,0.07927125,-0.017948579,-0.0012789445,-0.057504855,0.016654978,0.015409134,-0.020134255,-0.034699455,-0.051066175,-0.051293936,0.066267945,0.06777421,-0.005910733,-0.026846355,0.03897629,-0.034867067,-0.014881307,-0.039955992,0.0038218168,-0.022297911,-0.0055006784,-0.011052517,0.0054102745,0.008267021,0.024689471,-0.06486823,0.020300712,-0.036316738,0.0045512132,0.008043444,-0.0077892914,-0.001592694,-0.008179212,0.0037822947,-0.0044512423,0.07334133,0.050767444,0.011297921,0.055855088,-0.01962382,0.0108160935,0.015374737,-0.027570182,-0.029587284,-0.014463545,0.06531152,0.039847985,-0.009819417,-0.009714121,0.036625925,-0.010507758,-0.011095108,-0.015080557,0.0011024416,-0.015433799,0.01503197,-0.0001097379,0.046239465,-0.050155856,0.025375698,-0.0034970865,-0.01447269,-0.00083879236,0.024330366,-0.039458536,-0.017573858,-0.008572103,0.0443779,0.048579294,0.0048102923,-0.06676812,0.010168047,-0.00019866538,-0.028721534,0.021724036,0.013919,-0.021595018,-0.020142375,-0.082805455,0.032827366,0.05576898,-0.0039446196,0.025301376,-0.011150414,-0.021053007,-0.0011983613,0.0036535633,0.06810257,0.014231932,-0.0072162314,-0.013144748,-0.033630926,0.040924642,0.0029701875,0.0215249,0.043005288,-0.075137675,0.046815254,0.017918803,0.06322017,-0.060220595,0.008240013,0.022970043,-0.0006564833,0.045576513,-0.07658657,-0.014134741,-0.0107078,-0.043180265,0.02792347,0.010840075,-0.04259846,-0.025005616,0.007671853,-0.042571977,-0.017552195,0.023630202,0.041354384,-0.0034089927,-0.021942297,0.024881018,-0.005143978,0.0017604436,0.020976346,-0.001806899,0.012940964,-0.029368516,0.010612425,0.022952298,0.026098836,-0.020891013,0.006073838,0.0028070007,-0.049857542,-0.018904228,-0.026233012,0.071158364,0.033065014,-0.012173601,0.039181557,-0.01327709,0.03489344,0.025209289,0.03518037,0.008256215,0.10702782,0.05344223,0.010955502,0.014647615,-0.04461494,-0.041384142,-0.04201812,-0.017843965,-0.0017238251,-0.018599635,-0.03419739,-0.0014068201,0.018961657,-0.045907356,-0.019400101,-0.013848878,0.023051392,-0.009826924,-0.010833475,-0.06370284,-0.011368126,0.021852598,0.019328725,-0.05261187,0.03957675,-0.028712561,-0.0032824462,-0.005878053,0.04139948,0.10248501,0.04108399,-0.0060045375,-0.02974048,0.0010808456,-0.046976205,0.00093913445,-0.026152536,-0.01635436,0.05324001,-0.015707707,0.0046377587,0.07612609,0.0016306753,-0.00833312,-0.08683652,-0.0084152715,-0.025296265,0.022316141,-0.027827246,-0.026366286,0.06778813,0.0071791694,0.007224099,-0.01212729,-0.012540388,0.0028316642,-0.0137551585,-0.01430235,-0.004466291,-0.0016798069,0.02355161,-0.09370807,0.011407444,-0.030221784,-0.0008527966,0.04441008,0.002479364,0.021801831,0.0002709577,5.89717e-05,0.036810715,-0.010818326,0.03175584,0.014641625,0.029726379,0.046950314,0.0028721758,-0.013104834,0.0050217193,-0.028717292,-0.010994132,0.0055877822,-0.0059207124,0.010135288,0.02908584,0.0011015743,0.025178218,0.06351358,0.08255619,0.027946766,0.020113943,-0.04658755,0.049445126,-0.037976153,-0.014049502,-0.073506124,0.01778064,0.029661262,-0.020003043,0.034773145,0.035129003,-0.015733197,-0.013596162,0.021151977,-0.056988906,0.024753887,0.007418406,0.074241064,-0.09427384,-0.032608315,0.040596284,-0.007635816,-0.018157601,-0.021414904,-0.008263112,-0.04079469,-0.014750184,-0.015905773,-0.006690955,0.06111273,0.00014891945,0.027836416,0.018548958,-0.017388849,-0.02555171,0.011005669,0.020365642,0.02361155,0.03985224,0.0090455385,0.023674522,0.027300123,-0.013860571,0.06524536,-0.023248807,-0.037094064,-0.019373726,0.06964197,0.049890153,0.0459869,0.03348766,-0.034233358,0.01914368,0.019535465,-0.030247761,-0.0001358275,0.020045083,0.0017106431,-0.032474212,0.017514303,0.05643744,0.019598586,-0.022607846,-0.019399812,0.00857788,0.018292544,-0.025614807,0.0025892365,0.006597865,-0.021755477,0.012080643,0.02489181,-0.008363484,0.06054415,-0.044716544,0.044182397,-0.06314834,0.026934866,-0.050661746,0.019472815,-0.016971905,0.027695805,-0.009041976,-0.03193646,-0.029287463,-0.02896683,-0.057198253,-0.031438105,-0.013239859,-0.014965225,0.005796425,-0.019900363]
5	3	[0.0028773614,-0.039199755,0.03765584,0.029436419,-0.06763316,-0.0753371,-0.021356173,0.021853337,0.0010595492,-0.044120125,-0.0014973128,-0.06778114,0.023407746,0.035853874,-0.027914409,-0.015404118,0.038870823,0.015262494,-0.03101529,-0.010699014,0.00039163078,0.0047107027,-0.023539828,0.010712597,-0.027363742,0.015837826,-0.04791454,0.0008956014,-0.07556636,-0.039538123,-0.05395019,0.027798451,0.047161624,-0.015190699,0.0631337,-0.004316216,-0.0248699,-0.013866368,0.029647296,0.026098136,0.017637959,0.008092097,-0.042169295,0.015039749,-0.016064875,-0.0006200871,0.08920045,0.034109905,0.002929735,-0.027757224,0.029856551,0.04829254,0.016886093,0.038154963,0.029921567,-0.020483384,0.043849867,0.0010087829,-0.051026687,-0.015875464,0.0017731066,0.04637889,-0.0014456476,0.028766185,0.047632664,0.02928481,-0.03466326,0.0036805123,-0.014140061,-0.008416414,-0.007821218,-0.008949552,-0.024709404,0.071997605,-0.009268737,-0.032142058,0.018619396,-0.08709627,0.011676002,0.0019547674,-0.037653126,0.0047205477,-0.02315062,0.023492599,-0.02777171,-0.005343284,0.035236944,-0.0068109874,0.0011516719,-0.035694566,0.022006936,-0.024912085,0.05459381,0.09321223,0.02341267,-0.06762585,0.011752294,-0.043014497,0.014694021,0.0033224882,-0.028896637,-0.023747688,-0.053875577,-0.027095575,0.013605622,-0.018254122,0.011551444,-0.0067953016,-0.015275321,0.019182213,0.0057347463,0.003491226,0.0939257,0.036872193,0.043793734,0.0074051977,0.0077380217,-0.004864221,0.07901493,-0.016793046,-0.056453127,0.022157079,-0.034080297,-0.048526563,0.006636181,-0.000483134,-0.007313234,-0.024834143,-0.032477725,-0.010460939,-0.0071204836,-0.018913161,-0.019222878,0.023409635,-0.05078454,0.057588443,-0.01660554,0.05401254,0.01402185,0.033800807,0.020557934,-0.051645465,-0.00476974,0.015509481,-0.03070321,-0.032445136,-0.019061014,0.009429364,-0.029724311,0.08080255,0.044820942,0.040997267,-0.06949397,0.02809521,0.033509456,0.02453113,-0.07231401,0.027736817,0.0055057798,-0.03077538,-0.029083947,-0.023450617,0.005193156,0.06724678,-0.047211517,0.039057273,-0.018871412,-0.014886095,-0.023306323,0.05263432,-0.082598254,-0.015883435,-0.044401363,-0.022413509,-0.043493506,0.027883144,0.05049082,-0.00079011306,-0.034369674,0.032948527,0.01571924,0.007745097,-0.026140919,-0.054987844,0.021415561,0.017913997,0.033367403,0.018097285,-0.03732144,0.0348212,0.009836141,-0.026962707,-0.011000222,0.003393116,0.012513317,-0.1296119,-0.0036686976,-0.040110063,-0.010031751,0.012588425,-0.0013233008,-0.056349065,0.029867535,0.01177204,0.043448944,-0.01920014,-0.016070638,0.026692767,0.008222119,0.050443653,0.008176409,-0.0035059855,-0.03357526,0.014893503,-0.060343944,-0.014976143,0.052621245,0.0417249,0.0022844605,0.015299473,-0.026749779,-0.016580392,-0.03361932,0.016408188,0.0004474883,-0.06301585,0.029366018,-0.00041457175,-0.03738122,0.0027554852,0.018575955,0.01666609,0.044189427,0.01849274,0.0060055805,0.046581894,-0.024883062,-0.025196515,0.04066722,-0.020811185,-0.10655915,0.038312584,-0.008797169,-0.006398946,0.0240209,-0.05008383,-0.008618379,-0.11525996,0.068927765,0.026070355,0.03144427,0.045871716,0.009064787,-0.029439548,-0.021522805,-0.018396346,0.035708085,-0.024982002,0.087510824,-0.01832534,-0.03408315,0.05678991,0.062762916,0.031755738,0.016704319,0.045335814,0.00046353723,0.022221912,0.011470242,-0.050298993,0.03548515,0.011625042,0.0059667584,0.033064246,0.015336912,0.0052333605,0.031551767,0.031180112,-0.06986785,-0.001633067,0.019800242,-0.0047292444,0.00015516201,0.020455264,0.003099079,0.07847237,-0.05950798,-0.022100208,0.027967528,0.0419212,0.019221194,0.0035875486,-0.11645868,0.0010779225,0.03152065,0.008756974,0.020141758,0.0052151736,0.028641233,-0.049138296,-0.042056542,0.02080562,-0.04914119,-0.00412226,0.0075438526,-0.018524235,-0.030613972,0.056245305,-0.018662628,0.034606956,0.012749562,-0.016372718,0.045584757,-0.0044024684,-0.0077096457,0.0326426,-0.013399914,0.0011313017,0.0075704036,-0.034197755,-0.0019823783,-0.030794434,-0.004359081,0.027131218,0.057821512,0.038680322,0.029838232,0.023405286,0.069999896,0.017275391,-0.07104951,0.008819708,0.0028294292,-0.14641865,-0.042143986,-0.023671394,-0.013361398,0.0125407325,0.053487662,-0.014938819,-0.011400835,-0.0058646267,0.04680415,0.03197365,0.053204563,0.040613595,-0.015206538,-0.041305672,0.0005611613,0.037283007,-0.003950472,-0.033413265,0.03317493,-0.04938507,0.003388912,0.022478445,-0.025247756,-0.008532294,0.029845318,-0.08587382,0.041950777,-0.024384072,0.006375164,-0.0116102025,-0.0091118505,-0.006142887,0.054320082,-0.05187576,-0.019907862,-0.054217394,0.01964829,0.093337096,0.007164761,-0.006426522,0.0051919175,0.0073828516,-0.03827587,-0.0041937414,-0.0238994,0.035109237,-0.011293985,-0.032070577,0.020205818,-0.038220104,0.034001328,-0.040400986,-0.006526057,0.015385469,-0.061711777,-0.02832415,0.0017258176,-0.011321009,-0.0543702,-0.031877674,0.043899707,0.028621405,0.012379224,0.009660135,0.035162654,0.044098906,0.02209823,0.040683065,0.0035856175,0.047832966,-0.01643991,0.05533484,0.054223035,0.0010509478,0.0109678535,0.029357309,0.0014405422,0.059489697,-0.017626751,-0.05475133,-0.054208323,-0.014565192,0.074844345,-0.043546632,-0.035666056,0.015216981,-0.0115723815,0.025870105,-0.015148364,0.011550487,0.021038461,0.0018468709,-0.003121131,0.051674366,0.03866042,-0.00862007,0.0092425,-0.027890597,0.05232396,-0.04948574,-0.040469028,0.0119017195,-0.0077394443,-0.06589008,0.026384361,0.043732863,0.07582021,0.057939008,0.0014310535,0.0024036793,-0.014615453,-0.017093096,0.012934556,-0.07101281,-0.025683742,-0.06911975,-0.027079428,0.022791317,0.008364023,-0.073183,0.014872203,0.06689767,-0.050884612,0.02952543,-0.031226099,-0.0066013695,-0.041336924,-0.05490208,0.013092158,0.0041659856,0.011653603,-0.029485174,0.0040112454,0.015850125,0.016927676,0.03617372,0.0069030696,-0.029393312,0.009953391,-0.06765279,0.028552657,-0.015962498,0.07796131,0.053989977,0.009543297,0.049444657,-0.024678148,0.027332801,0.072715275,-0.01586816,-0.0071697053,-0.005272063,-0.019150225,-0.010204735,-0.023995545,-0.016262801,0.069812946,-0.056323044,-0.03259742,0.000461708,-0.0011659892,-0.021976098,-0.040795736,0.06997337,0.021462753,-0.014202797,0.07200951,-0.00044543383,-0.068006925,-0.002525581,0.031269897,-0.00036293783,-0.008362666,-0.013054171,0.008018812,0.074809894,0.0010876391,-0.049827054,0.02699535,-0.039597493,-0.0048781787,-0.015329842,-0.022235278,0.0044176797,0.0083245775,-0.07819307,-0.014560511,0.05095018,0.007919706,0.04072331,-0.01721673,-0.019746598,-0.05632513,0.032151476,0.02658682,-0.029358985,-0.00773668,0.043575883,-0.046507124,0.040768374,-0.05190042,0.0393965,0.01997317,-0.050165396,0.056162383,-0.005393564,0.030675175,-0.04779752,0.027869273,0.0045004827,-0.023921616,0.04393109,-0.03434327,0.0029823626,0.039609347,-0.026150858,-0.026091319,-0.023703188,-0.032424856,-0.016067594,-0.025934245,-0.06686342,-0.0018532393,-0.021430759,0.0065090503,0.01038315,-0.009200794,-0.028399387,-0.008858279,0.004728361,0.024842765,0.03479156,0.0070139803,-0.01587851,0.045171432,-0.004996019,-0.014025902,-0.024275808,-0.027214533,6.7290144e-05,-0.049106315,-0.028559698,-0.008560189,0.013913044,-0.02872016,0.021626225,0.06788321,0.023394419,0.05312157,0.00834165,0.01770892,-0.0071283467,0.048750818,0.023605045,0.0763935,0.0101768,0.02013754,0.004922971,-0.0141998865,-0.05469826,0.012516367,-0.064050175,-0.045710597,0.02220836,0.035202477,-0.013639712,-0.009655891,-0.022240771,-0.0114694685,0.014267588,-0.0022752748,-0.04721581,0.017816216,-0.041875158,-0.008274063,0.020965854,0.036570273,0.0033226064,0.031047478,0.015811853,0.018611327,0.0736563,0.008370151,-0.041161805,-0.027654057,-0.043583397,0.0039584427,0.009986389,-0.032144167,0.036295407,0.00045392712,0.0005356504,-0.029241499,0.016081065,0.03492118,0.015818965,-0.059192695,-0.03471763,-0.013575569,0.038969405,0.04308715,0.007472722,0.048855267,0.044156108,0.0050286935,-0.022449499,-0.040574305,0.053806268,0.015039292,-0.008382986,-0.01213423,0.015428386,0.01685227,-0.108178996,-0.014910545,-0.025655909,0.0018729784,-0.014419884,-0.04296786,-0.012771504,0.04833314,-0.022808552,0.018096577,-0.06921782,0.038021825,-0.008603118,0.014593957,-0.016351586,-0.077126674,-0.019071015,-0.0433737,-0.029306887,0.007147161,0.022340959,-0.017215189,-0.034324754,0.061985373,-0.0035136242,0.039813474,0.015733335,0.11615053,0.034648616,0.050863832,-0.01574438,0.02052918,-0.040882584,-0.06011609,-0.057071276,0.0006991042,0.030696664,0.03349906,0.0107641425,-0.044016883,0.054130167,-0.002138284,0.05825471,-0.008626255,-0.0061462745,0.022995757,0.07334539,-0.06646787,-0.014292394,0.03675114,-0.026968017,-0.015324676,-0.028145436,-0.029076094,-0.019985465,0.016385252,0.016747866,0.006486351,0.021155482,-0.009501048,0.01960806,0.0050290083,0.015399759,0.036803607,0.025323434,0.017926902,-0.012679775,-0.0019448538,-0.041318286,0.02421359,0.015333563,0.015414657,0.03061284,0.0128041515,-0.018484619,-0.008968306,0.0046734056,0.09751524,0.018091427,0.051511593,0.010415502,0.016247546,0.00405016,-0.013598,-0.035411995,0.051458035,-0.04356675,0.039847974,0.028916806,0.059558384,0.021965276,-0.014504801,0.03642013,0.0163037,0.0153662525,-0.07078879,-0.016656475,0.0091050705,-0.007805126,-0.013058344,-0.0016245567,0.048875388,-0.043857116,-0.029813336,0.08915449,0.0062214327,0.005272784,-0.07195383,0.066259004,0.011382925,0.028441386,0.008765289,-0.0066633914,-0.019591674,-0.04466696,0.0021743658,-0.044504005,0.026487963,-0.064439885,-0.039759796,0.0044623753]
6	2	[0.015967125,0.0010038863,-0.008321471,0.011410707,-0.02426384,-0.02709326,-0.040215585,0.012103611,0.0021005268,-0.0006816356,-0.00075750577,0.047869373,-0.0036068151,0.021391088,-0.061009835,0.021561602,0.06216141,0.051616706,-0.036537148,0.05786066,-0.0021404515,0.10377058,-0.007842803,-0.04364963,-0.0014673619,0.014474371,-0.02759531,-0.052319404,-0.027340857,-0.021581933,-0.063612975,0.040163472,-0.0012381293,-0.043921907,0.046791673,0.01156763,-0.014174876,-0.048773944,0.011528665,-0.0225158,0.037973654,0.041399308,0.022057483,0.023336535,-0.014117181,-0.024144996,-0.032041326,0.055892568,-0.029440485,0.033721514,0.012917175,0.053933747,0.008103458,0.03072936,0.023854153,-0.017414933,0.01746963,-0.045240432,-0.020647733,-0.0041512717,0.017390843,0.07646387,-0.018157052,-0.054096688,0.031430066,-0.004744035,-0.0129364645,-0.0417596,0.017870959,-0.022836206,0.009849917,0.0059644654,0.020228295,-0.009394586,-0.007512079,0.043461964,-0.014784028,-0.103345886,0.044170137,0.008577073,-0.034903374,-0.028146852,0.05565117,0.010239707,0.004018499,0.021823904,0.0033056736,0.05944501,0.048070908,0.0053597633,0.0054896553,-0.0009785701,0.021223163,-0.00075216487,0.026157942,-0.001116638,-0.028398592,0.078478776,0.021245623,-0.036416963,-0.023959722,-0.0038517688,0.0019512038,-0.06254389,0.043801736,-0.0054268404,-0.08008135,-0.019862968,-0.04092595,0.03884961,0.043873265,0.012546478,-0.023249716,0.022236539,-0.05111436,-0.05303333,-0.0068800994,0.0041219415,0.06476998,0.033806175,-0.017395323,-0.03938255,-0.055379543,0.015714357,-0.015940677,0.06643522,-0.009536468,-0.019503975,0.025305279,-0.02059146,0.010720124,0.03833616,0.006902263,0.096323274,-0.034435682,0.012126153,-0.09384392,0.035330016,-0.013737631,0.027254423,0.010924623,0.015964322,0.045401864,0.04107922,-0.030438734,-0.0019866948,-0.0671712,-0.0012170791,-0.021363625,-0.002271738,0.024487656,-0.017925715,0.004264726,0.02810324,0.012110653,0.01753733,0.007610853,0.044922404,0.020971553,0.034940396,0.012636219,-0.04741315,0.04625191,0.031920426,-0.001404397,0.0028353422,-0.025558356,-0.042674154,0.007745096,0.026923977,-0.04831128,-0.005940458,-0.0016761263,-0.019273762,-0.02298753,0.002421488,0.02104438,0.033134162,-0.00020416274,0.018484375,0.052409153,0.022279348,-0.0077651003,0.030938221,0.0024992544,-0.041261304,0.008337731,0.007519932,-0.019019233,-0.025152626,0.019846879,-0.006639846,-0.011504988,-0.038254503,0.012860795,-0.05973306,0.016882975,-0.05072996,-0.008316075,-0.02001905,0.017957358,-0.061589807,0.00064967887,0.03211921,0.057469826,-0.084939815,-0.079560034,0.0026614647,-0.014378288,0.039782528,0.05735516,-0.06257327,-0.008772804,0.04224646,0.0656974,-0.05334825,0.051503427,0.0943302,0.059005804,0.05332766,0.064067,-0.005767176,-0.03200309,-0.006651995,0.04669083,-0.04807181,-0.024102794,0.06259114,0.028866656,-0.0066413092,-0.0032919953,0.06693028,0.051249795,0.0036860306,-0.0032165756,0.011924436,0.018496927,-0.022312934,0.007907222,0.0067304918,0.0405169,-0.031999435,-0.0014802169,-0.0132649625,0.010171889,-0.019142536,-0.018831328,-0.15664767,0.04370891,-0.032835208,0.014251031,-0.029663159,0.035015244,-0.009046729,-0.011391065,0.010853764,0.0005500633,-0.0031947782,0.07069638,-0.03231612,-0.05661021,0.0358459,0.0043467595,0.015680432,-0.0017126814,0.08540111,-0.037454948,0.0063436003,0.011988474,-0.014803636,0.009216592,-0.0037856295,0.07200087,0.08192627,-0.046805643,0.0032133027,0.05205737,0.034369875,0.049975976,-0.044260293,0.04475387,-0.054042414,0.013320386,-0.03961496,0.023741873,-0.0005635436,0.06483189,-0.03037679,0.014527703,0.05079281,0.01507933,0.0065545053,-0.05872798,-0.005738927,0.038255576,-0.008138356,-0.018137965,0.016500501,0.049075726,0.004071838,0.0006868099,-0.0341985,-0.03620329,0.029265031,-0.031248273,-0.020224126,-0.029224813,0.059856854,0.033011198,0.0019592328,-0.0015973361,0.05021536,0.05440645,-0.053675827,-0.020334166,0.010921838,-0.015039058,-0.0050224704,-0.012298418,-0.020955518,-0.012451633,-0.006081749,-0.055414688,-0.031615853,0.010612152,-0.039528176,-0.0023768423,-0.00807115,0.02190677,0.012569371,0.042088777,-0.06215122,0.06938042,-0.11068816,-0.019790206,-0.034361992,0.04311946,0.035394344,0.04855843,-0.020635847,0.009903072,-0.0065256953,-0.053061757,-0.026194599,0.00871833,0.03269717,-0.04817622,0.0038856042,-0.01323431,0.055607468,-0.0039523216,-0.022890264,-0.05892996,-0.064549096,0.024430066,-0.0006884468,-0.02761781,-0.018898375,0.0057449494,-0.05154806,-0.012220334,-0.024973579,-0.08029891,0.032699477,-0.034367602,0.034925163,0.0364454,-0.049036928,-0.02504049,0.0069883107,-0.02212984,0.031275913,0.019963928,0.0013082683,0.027712967,0.022425814,0.054772455,-0.026732882,-0.048746828,0.04810454,-0.013604937,0.0469953,0.06880384,-0.035020955,0.02546586,0.023751197,0.008891776,-0.048203323,0.014985323,-0.0066838474,0.0002313229,0.023493113,-0.0059482865,0.021090275,0.049566668,0.029769829,-0.007269041,-0.007747332,0.023849357,-0.007933643,-0.036724314,-0.050516192,0.0010866609,-0.012527588,0.021553392,-0.039741024,0.06289568,-0.010099116,0.0045190267,-0.017971208,0.006955118,0.04317182,-0.0007501554,-0.037602104,0.050631832,0.040488347,0.028270364,-0.045546915,-0.013979359,-0.011614592,-0.006327523,-0.045780294,0.024452524,0.0039725495,0.05270267,0.01832133,0.01047256,-0.0066843126,0.006781368,0.03050101,0.039435,-0.04733824,0.102709725,-0.015087176,0.020242443,-0.019482384,0.039006967,-0.033336807,0.0409001,0.010345923,0.073457524,0.033501275,-0.0041609434,0.06495795,-0.013442827,-0.012042594,-0.01877009,0.02101044,-0.041103497,-0.014911613,-0.022834273,0.070028774,0.105363786,-0.020266999,0.0042774887,0.055384737,-0.07772914,-0.024222167,0.0011473523,0.0040548937,-0.016276108,-0.011929293,0.007751281,0.03244897,-0.042000756,0.02667623,-0.087749,0.01494148,-0.036072627,-0.00031916416,0.025975768,-0.021477034,-0.013384226,-0.021774113,0.0016754675,-0.014415343,0.044953067,0.05448284,0.0393731,0.054914307,-0.022700865,0.032891445,0.0005814161,-0.0019716152,-0.015043636,0.029688543,0.03303143,-0.016608655,0.0081446245,-0.016117338,0.051671863,0.004550399,-0.014907693,-0.03348936,-0.004676049,-0.0020963284,-0.0053988155,-0.022022404,0.027594393,-0.01648151,-0.01305183,0.028749121,-0.0052013802,0.014901493,-0.010240957,-0.037459932,-0.033364855,0.0114959935,0.050056625,0.0023600506,-0.017583042,-0.01591277,0.0004962516,0.011715969,-0.03256105,-0.01365341,0.04830262,-0.026501765,-0.065090425,-0.008033693,0.018633237,0.037147064,-0.016224591,-0.020638896,0.005071013,-0.019691186,0.01808754,-0.00212197,0.07393129,-0.0018576982,-0.052576758,0.008304124,-0.041979685,0.034955647,0.017790914,0.028507438,0.046995994,-0.045016453,0.00056648033,-0.0027382402,0.02206122,-0.04372848,-0.0030847434,0.051098555,-0.016377999,0.018663207,-0.059857503,0.021457437,-0.008982791,-0.033884373,0.0632001,0.029089538,-0.032000758,-0.019414082,-0.034698986,-0.0718636,0.008853129,-0.0150179425,0.02651114,0.042892125,-0.015010772,0.08730747,-0.02900947,-0.005176656,0.063792974,-0.027005155,0.034520328,-0.061148915,0.046271686,-0.00011532107,0.04111377,-0.04626895,0.008137798,-0.03914515,-0.04878945,-0.0013284176,-0.023304071,0.081276596,-0.02127895,-0.0054823565,-0.04088376,-0.026852163,0.054929856,0.03149957,0.011633756,0.05408212,0.07637377,-0.038657654,0.10757617,-0.03367207,-0.04669092,-0.036778755,-0.024596771,0.0013319165,-0.036054444,-0.035433844,-0.011191699,0.0287038,0.004373238,-0.000864838,-0.0055984654,0.0020307102,0.029561978,-0.05253469,-0.0036717888,-0.04119626,-0.004568334,0.021225234,-0.038123127,-0.06533978,0.025969366,0.026214201,0.034035295,-0.05076006,0.018144801,0.07599106,0.053987566,-0.010085754,0.026135696,0.03592203,-0.008916198,-0.02825382,-0.033346083,-0.00876099,0.028842987,0.007173723,0.058045737,0.028821595,0.01963757,0.0044434736,-0.046321075,0.030663665,-0.020822203,0.0058406885,-0.036387563,-0.043211292,0.05950753,-0.036462255,0.022742182,-0.024716523,-0.037422065,0.033881396,0.023803309,0.009162553,-0.011175353,0.019020451,0.016429393,-0.11577869,0.008024131,-0.02519114,-0.01084277,0.02224492,0.06695422,0.025364986,-0.020271912,0.05492907,-0.011844029,-0.018439673,0.03921615,-0.013760252,0.016605387,0.030381346,0.02809812,-0.039525557,-0.0677843,-0.025055075,-0.011735236,0.035295304,0.0044736434,0.020278031,-0.025585238,0.0014095894,0.015409069,0.016997993,0.051812816,0.0066878805,-0.0030528773,-0.012446571,0.039864715,-0.03681273,-0.047800265,-0.07062136,0.005574176,-0.026052602,-0.025523998,0.041234385,0.027832137,-0.025312977,-0.025121344,0.008216917,-0.037118733,0.034693014,0.013051158,0.039776444,-0.040721618,-0.044116907,-0.00051053107,-0.0006147076,-0.004314287,-0.024154652,0.033472423,-0.027364621,0.017173538,-0.015619117,0.014583469,0.015709247,0.064051956,0.031086782,0.047755707,0.0018946164,-0.030942619,-0.023329675,0.020965112,-0.02185723,0.07635869,-0.029547097,0.02052531,0.02873429,-0.0003373797,0.004280011,-0.033755682,-0.0063379146,-0.047482148,0.04433698,0.058657438,-0.0007295033,0.022568317,-0.057153404,-0.03154232,0.0037614144,-0.039926596,0.0030450567,-0.0020607857,0.05731061,-0.05589623,0.035730474,0.047385994,-0.0014701479,-0.005840975,-0.05919507,-0.019592667,-0.018651607,0.015997035,-0.050225887,-0.02223412,-0.031170066,-0.015072606,-0.027564492,-0.007941977,0.0032434955,0.049067814,0.004458545,-0.07368382,0.019361645,-0.0053078723,0.017833404,0.014067654,0.101848654,-0.05726193,-0.012195525,0.022533251,-0.010869601,-0.041001055,-0.011763316,0.004678302,-0.080934286,-0.024609385,0.023172742]
7	1	[0.005908798,0.034163762,-0.033271223,0.004370228,-0.021886595,-0.05354445,0.012627548,-0.020047443,0.036017697,0.022411214,-0.038172137,0.057621874,-0.011912156,0.04730452,0.0681204,0.004277141,-0.054952346,0.027640391,-0.0033936196,0.062364873,0.019322686,0.043528568,-0.08636793,-0.05958822,0.028561316,0.03957562,-0.010280198,-0.03432995,-0.007555507,0.038578622,-0.023113795,-0.0047040167,-0.024404986,-0.029934667,-0.008951167,0.019424178,-0.010539,-0.02294395,-0.004747942,-0.01114301,-0.05126957,0.020288946,-0.030937586,-0.008711521,0.010414404,0.022299001,-0.07673111,0.024060722,0.010009147,-0.00043530372,-0.042378377,0.04513296,-0.038694784,-0.018591255,-0.027013132,0.030337257,0.0016814253,-0.042244587,-0.0076067485,-0.012149429,0.061086092,0.061428104,-0.041249413,-0.0012981354,-0.040406674,-0.00452669,0.018187877,-0.03928661,-0.025356889,0.018078987,-0.0030240905,0.037490737,0.00084065145,-0.044590563,0.06302925,0.06908806,-0.014064821,-0.08503046,-0.048153784,0.048625175,0.00050154084,0.014812036,-0.07880143,-0.011504188,-0.011377471,0.095249094,-0.0009830988,-0.017226279,0.021450406,-0.0059764585,-0.026598828,-0.0068700444,0.017635655,-0.0033107894,0.037609473,0.03553975,-0.010483166,0.066249415,0.020214517,-0.011535615,-0.0057996926,0.005620224,0.03359586,-0.025650553,0.01373184,0.00030091143,-0.052609246,0.030674051,-0.057464577,0.051812362,0.06514973,-0.0026181405,0.012054298,0.027949687,-0.014635527,0.0011235415,0.01911817,-0.0015421341,0.037238088,0.046713352,0.026689192,0.0038112188,-0.0059091826,0.10861749,-0.046040058,0.0767138,0.032227468,0.003282497,0.048511136,0.014592876,0.004974248,-0.0004558286,-0.029056028,0.061280143,-0.014710576,0.007276563,-0.057515536,-0.049744375,-0.05363084,0.050091416,0.008591236,0.015644293,-0.009830591,0.02502469,-0.044401143,0.008684752,0.0054466324,0.011561863,0.028449308,-0.08580746,0.0551499,-0.007729695,0.038951498,0.0046513616,0.022159895,0.036477406,0.008586089,0.00439258,0.0068483986,0.0412684,-0.0011953149,-0.05089917,0.06870973,0.004840043,0.04045746,-0.017936876,0.0061481236,-0.014680737,0.06646874,-0.0013828976,-0.031503577,0.026115393,0.028706321,0.02997369,-0.04558114,-0.035499085,0.039152965,0.010685588,0.012896341,0.022031212,0.01600934,-0.0076892693,0.03786114,0.044546183,-0.02625306,-0.017497659,0.005980505,-0.036190815,0.01465295,-0.055439733,0.08011471,0.02739368,-0.04335532,-0.066162065,-0.0006024558,0.051703207,0.017626677,-0.003334375,0.027286535,-0.05193595,-0.011837917,-0.038003914,-0.05959648,0.020928642,0.05927685,-0.028260525,-0.087626986,0.010976907,-0.02711676,-0.01778024,0.039293993,-0.07414151,-0.0071095526,0.0016480405,0.0734086,-0.07008165,0.020753808,0.11381726,0.079018906,0.04048067,0.024679668,0.012754978,-0.0063785617,0.034516055,-0.04839244,-0.105396345,-0.048019506,0.11138152,0.037754804,0.008017106,-0.054868765,-0.027430994,0.037943084,-0.074581996,-0.02539323,-0.048771005,-0.011173351,0.006348781,-0.027112825,-0.02691142,0.023883075,0.0071072048,0.012674084,-0.0066203196,0.026941285,-0.014374621,-0.023445094,-0.041321747,0.019670894,-0.034785822,-0.008964541,-0.085018575,-0.017851738,0.030813552,0.02232323,-0.016710501,-0.00938593,0.022890834,0.044237457,0.016536247,0.0048240717,-0.020437375,0.007116906,-0.07141968,-0.03422144,0.088329084,-0.024527485,0.0026295541,-0.036645953,-0.017441675,-0.00021735358,0.013310902,0.02797408,0.04963514,-0.026262628,-0.054216754,0.069354385,-0.03377819,-0.009004759,0.00036492452,0.06767979,-0.034658927,-0.035495162,-0.025439678,-0.029859668,-0.010896467,-0.00030589855,-0.030281952,0.01966934,0.008301232,0.03462691,-0.009514799,-0.006263045,0.01033828,0.030244885,-0.018626148,0.016727189,-0.03590677,0.058929212,-0.0033746096,-0.03170726,0.035761464,0.0023057915,-0.00010014123,-0.04372975,0.035723932,0.055211402,0.024011938,-0.004508023,-0.03211614,0.005599487,-0.02455113,-0.0070194784,0.003918682,-0.0010970891,0.025906216,0.029350594,0.01909408,0.030897187,-0.027116004,0.0068138144,0.054714825,-0.021985786,0.01622481,0.0029676168,-0.07198393,0.021454366,-0.050443504,-0.020953255,-0.015561734,0.07545183,-0.07023045,0.06897424,-0.09939387,-0.0023576526,0.010791672,0.038859185,0.03015799,0.056145426,0.03950697,0.050339684,0.00963845,-0.038472135,0.029389279,0.01432491,-0.016142711,0.0062489607,-0.009906136,-0.041082956,-0.029767513,0.012898792,0.059826624,-0.015198371,0.027627252,0.017309142,0.011016682,-0.05994959,-0.017154321,-0.04290479,-0.049878526,0.016545817,-0.026433753,-0.027492283,-0.017958589,0.029947031,-0.03647193,-0.0018379447,-0.043292984,-0.017428404,-0.015306495,0.025299741,-0.010415753,-0.0025010633,0.03554979,-0.018266404,0.017390037,0.057143807,-0.025076777,0.012700331,-0.0027374974,-0.00092004606,-0.02030911,0.07891148,-0.012748882,0.015226773,-0.002918741,0.0062996564,0.022672642,0.017712615,-0.014994269,0.038304243,0.0049106013,-0.031036418,-0.02240057,0.042086795,-0.0056494167,0.049690664,-0.0313204,-0.036828898,-0.032509625,-0.032605916,0.010099286,-0.025225569,0.011791378,0.017791726,0.08524823,0.011585811,-0.054433372,0.04798753,0.022427322,0.030891053,0.020869425,-0.011752616,-0.020445896,0.007160623,0.013338945,0.013932002,0.005700599,0.017825758,-0.023014065,0.02593805,-0.031151328,0.0109071145,0.025454205,0.0016995904,0.011719207,-0.016090507,-0.029494626,-0.002982631,0.05979207,0.11429224,-0.038049813,0.041850287,-0.015889362,-0.007456321,-0.0076174694,0.019803101,-0.02084307,0.050974563,-0.012537217,0.055846088,-0.05065565,0.003760703,0.03048027,-0.035836525,0.0007188244,-0.014478038,0.014632291,-0.012884477,-0.013851631,-0.038086355,0.043742035,0.020745078,0.08288753,0.03140197,-0.055848245,0.016486442,-0.015823945,0.024275362,0.022373576,0.016983831,0.022689587,0.06491565,0.013157889,-0.040614374,-0.026224952,-0.071638316,0.06734713,0.029709943,0.016604487,0.016929645,-0.041396387,-5.8352383e-05,0.013933747,0.00245208,0.011710768,-0.0023065228,-0.0022786981,0.008744266,0.029553775,-0.014806214,0.016660443,-0.03469377,-0.007903198,-0.05921153,-0.00048470896,0.0279551,-0.016963575,0.0033284654,0.01546224,0.017258681,0.076687716,0.0189076,-0.0470755,-0.0068621184,0.019569758,-0.06674253,-0.0053738933,0.020222457,0.011228413,0.009542649,0.014422224,0.0144338505,0.01756788,-0.015234229,0.017938958,0.013760048,0.022194827,0.030109426,-0.0029022708,0.017488632,-0.039117273,-0.016469672,0.015841177,-0.022930017,-0.011934316,-0.0015053182,-0.012487341,0.051100213,-0.0057407822,0.015280768,-0.02518151,-0.009779917,-0.037883326,0.008574959,-0.01118125,-0.0066365777,0.009040044,0.015101666,-0.027666232,-0.016099103,-0.011179579,-0.017960511,0.030057106,0.033517204,-0.00765772,0.01589714,0.01010727,0.057783954,0.055074096,-0.11003691,0.024680432,0.028803436,6.152848e-05,-0.05976099,0.030088123,-0.0076072416,0.0046526035,0.0036246164,-0.039527092,0.04265811,0.030239116,0.052430533,-0.020716997,-0.03571565,-0.03276609,0.020245453,-0.009209828,-0.021317787,0.030572802,0.017896976,0.05346801,-0.044357173,0.033212654,0.09838502,-0.05052654,-0.024209397,-0.069349095,-0.004016265,0.01970549,-0.04962999,-0.0044968463,0.012083796,0.00622491,-0.02446103,0.0010610612,-0.01749133,0.07547156,-0.0141137745,-0.029525591,-0.046952166,-0.040689908,0.0458135,0.059273977,-0.016445182,0.04702759,-0.044543553,-0.005726446,0.018757122,-0.038973846,-0.03839559,-0.056868486,-0.016622813,-0.005053893,-0.012610523,-0.08106676,0.017506361,0.014193601,0.011344516,0.029774385,-0.034649167,-0.016114948,0.0071462784,-0.060900815,0.04371105,-0.063740194,-0.006755543,0.0024422167,-0.032262914,-0.044293217,-0.018160151,-0.012097028,-0.009861977,-0.06283688,-0.07853998,0.009126227,0.04605777,0.040213656,0.01866823,0.05164602,0.022768103,0.0072292266,-0.034213334,0.002355945,0.0837118,0.024397722,-0.00500398,-0.073416814,0.033580966,0.02545621,-0.0061819633,-0.0099187195,-0.052986033,0.02929999,-0.04412793,0.0009790405,-0.008171632,0.040940482,0.012128715,0.01394924,-0.0043048067,-0.05434558,0.013054242,0.0005357493,-0.014886746,0.021514762,-0.03614123,-0.045374606,0.0104238605,-0.0029986918,-0.05136197,-0.017964745,-0.030419393,-0.013037981,0.0010226758,0.030859595,-0.038289525,-0.023406396,0.0054022684,-0.00040254777,0.020100093,0.004086058,0.06780885,0.010508804,-0.06400687,-0.017571514,-0.013146387,0.038278464,-0.027515775,-0.01036647,0.015858991,0.016584544,0.00049752375,0.027966449,-0.008397922,-0.058883425,-0.060408663,0.0051603997,0.059999455,0.0017199258,0.013613363,0.00260032,0.057211537,-0.06677993,0.002194129,0.026267095,0.0026036967,-0.009420818,0.00839841,-0.027410382,-0.037011635,0.08197677,0.023370387,-0.0015067307,0.058705408,-0.023335343,-0.024981298,0.0042581987,0.0054875226,0.029574612,-0.016236672,-0.023526734,-0.0048857154,0.010798956,-0.0052683414,0.074531384,-0.002651992,0.05305487,0.030698825,-0.02797467,-0.033123158,-0.0020373685,0.007757367,-0.025464581,0.021405073,-0.015772605,0.0016258982,0.015791012,-0.030888246,-0.030826172,-0.04824856,0.013477358,-0.007640438,0.016696673,0.0005890137,0.04283852,-0.009084409,0.0126604205,-0.03967361,0.04709632,-0.0056828912,-0.029355355,-0.042485904,0.027162708,-0.036678605,0.065889694,0.007165314,-0.0070885997,-0.04587416,-0.04027853,0.035152484,-0.072867766,0.0023491895,-0.05942663,-0.06662464,0.051295076,0.017313171,-0.0273626,0.033897717,0.011336504,0.050577186,-0.008676743,-0.08754935,0.014667111,0.04892187,0.05765696,0.05814784,0.06284354,-0.075923935,-0.020172141,-0.043793414,0.06531424,0.042000983,0.03310997,0.01665219,-0.07790132,0.0051379297,-0.022737462]
8	8	[0.011423388,-0.055654377,0.054779693,0.037150767,-0.027034659,-0.057538345,-0.00034610662,0.042826038,0.017924944,-0.05553412,-0.029539963,-0.04713567,0.054443177,0.0043362975,0.004300157,0.038952447,-0.014835547,-0.017530516,0.004374984,0.0072683473,-0.028137617,-0.06546584,0.04862265,0.02338863,-0.01880717,-0.0015688951,0.0032033178,0.028427472,0.0016238214,-0.04657701,0.015371675,-0.043661986,0.0223949,0.014566477,-0.03192813,0.00952367,-0.021280397,-0.042613447,0.02139018,-0.01751874,0.056710526,0.03265618,0.039217006,0.00030375237,0.028261216,0.017646302,0.015008915,0.08268399,-0.031250335,-0.029218767,-0.005237152,0.05074899,0.042972926,0.055039898,0.014085772,-0.0058116084,0.07768293,-0.022055209,-0.018761717,-0.04326125,-0.00769274,-0.03897831,-0.05360264,0.01575587,-0.0073589035,0.037512504,-0.017848624,-0.047598265,-0.031511698,-0.040977314,-0.0042941757,0.030899962,-0.011418511,0.07516677,-0.033765864,0.027345568,-0.0461391,-0.18660626,0.009028412,-0.038312815,0.013978698,-0.019327182,0.00994181,0.062456187,0.018610204,-0.078872345,0.020444365,0.040895,-0.030223638,-0.04346906,-0.015669385,0.035490394,-0.0153434,0.057367675,-0.0016749145,-0.04547018,0.024111839,-0.0256883,-0.022256615,0.03680452,-0.00859007,-0.025700739,0.0029254772,-0.037222482,0.004651813,0.004042152,0.03423017,-0.027932052,-0.05170168,-0.015940152,-0.010317973,0.015810454,0.03232673,-0.036317818,-0.014534038,0.038148273,0.019421099,0.0066158464,0.04956029,0.024586458,-0.038198862,-0.018077737,0.044459354,-0.1090306,0.039541084,0.03928256,-0.014538914,-0.06609775,0.017194537,-0.012742955,-0.05417444,-0.050499313,0.01377715,0.014490696,-0.029281814,-0.04354545,0.063139796,0.05106009,-0.024390373,-0.020012787,-0.033133145,-0.026772432,0.023409765,-0.008467122,-0.010646646,-0.013867411,-0.022797845,-0.020792335,-0.025585525,0.0011901055,-0.0040730154,0.0128780855,-0.010266275,-0.03358479,-0.07251081,-0.021717137,-0.017890224,0.030764766,0.014873116,0.0353363,-0.05611845,-0.016739296,-0.0057898154,0.036012962,-0.02400845,0.025055392,-0.004168875,-0.0016511194,-0.034080338,-0.025714336,0.03047914,-0.025666796,0.014665729,0.054163966,-0.01773805,0.0055744136,-0.019710088,0.040461425,-0.05207299,0.0027525737,0.030600231,-0.012031726,-0.01305247,-0.030151863,0.064285964,-0.069669224,0.05072702,-0.082709186,-0.021799454,0.014110773,0.03255512,0.0039179153,0.015737673,0.0073569673,0.05541412,-0.04651915,0.036218934,-0.0029707816,0.0417268,-0.026788333,-0.00056591287,-0.005085527,0.045972534,-0.017017227,0.039063834,-0.03270819,-0.001872966,-0.002627127,0.037042163,0.036653217,-0.0036685432,-0.02638514,0.017565778,0.052801564,-0.03575808,-0.025744928,0.09511769,-0.001990486,0.06695568,-0.0012119781,0.05438227,-0.018674001,-0.0048048142,0.037164193,-0.010048431,-0.012758352,0.027048297,-0.039814275,-0.05030802,-0.006385276,0.02288228,0.017702,0.04982842,0.099762015,0.01353592,-0.007283468,0.0013331886,0.015186501,0.11786999,0.0055831494,0.11153361,0.03785871,-0.0077791996,0.015889766,-0.0010718281,-0.058684796,-0.04717017,0.0376229,0.02192748,-0.035712313,0.02122246,0.015080397,-0.020186828,0.007713642,-0.057003763,0.04744272,0.043037474,0.017864415,-0.035279624,-0.117057905,-0.06712588,0.02733863,0.01918371,0.03691203,0.02181664,0.03331316,0.016620088,0.020407898,0.042668305,-0.00437822,0.0012005045,0.014004481,0.0033589753,0.014465137,-0.04247793,0.022227028,0.037634734,0.047626205,0.042481475,0.01730642,-0.045881957,0.010610656,-0.025653351,-0.0067640636,-0.022080919,0.043706715,0.057376493,-0.036239505,0.034996692,0.03167065,0.031947143,0.040155128,-0.049548857,0.030452998,-0.02306288,0.004648105,-0.0020356951,0.09146446,0.0058339154,-0.031556457,-0.009734314,-0.047322996,0.025416989,-0.0046451483,0.013280974,0.040613648,-0.008679545,0.003975367,-0.07465428,0.022988558,0.020080477,-0.023526708,0.05269871,0.011191821,0.03249465,0.03134077,-0.02017462,0.02401723,0.02581744,0.0027150454,0.0070434427,-0.038889512,0.017396754,0.027379211,-0.019933578,-0.028084168,-0.0046497793,0.08702853,0.09521588,0.050190587,0.013100447,0.039431177,-0.008932545,-0.03374085,-0.044119947,0.04246884,0.007576906,0.040466767,-0.0044587906,-0.003437847,0.0012848923,0.012563757,0.023712842,-0.067283966,0.0430486,-0.005476242,-0.000764518,-0.03900578,-0.0031434668,0.07352933,-0.069641486,-0.06188428,0.012790383,-0.082514875,-0.06029631,-0.0013058168,-0.05848075,0.034532417,0.04368145,-0.0288728,0.023327533,0.021239819,0.025295949,0.06738714,-0.09055534,-0.025462702,0.04543882,-0.0005446583,-0.016931301,-0.017261656,0.034325317,0.02989379,-0.04348412,-0.012964273,-0.0055643055,0.004169879,0.0300488,0.034029,0.03136219,0.007154113,0.01986135,0.078883626,0.014696657,-0.020598823,0.0018599351,0.034983683,-0.023758957,-0.048176944,0.025092676,-0.03843019,0.0038651528,0.037972827,-0.010558055,0.015427187,0.03299836,0.03559907,0.054917216,0.017041013,0.046719283,0.031401306,-0.035603378,0.020854866,-0.035346687,-0.0088687325,-0.03173561,-0.045773923,0.022002634,-0.050868105,-0.0052456376,-0.015283867,0.020244254,0.025801571,-0.011811809,-0.021148281,0.025226755,0.0015511452,0.059855845,0.027330678,-0.023546763,-0.0023777785,-0.030191714,-0.0068310155,0.0077396743,-0.007175119,0.05751206,0.031583194,-0.008616263,0.008809207,0.014910572,-0.024504822,0.05034916,0.06354898,0.026581338,-0.031666804,-0.056876175,-0.01405395,-0.002478682,-0.012510804,0.05466485,0.037594095,0.0091354335,0.01712078,-0.018593078,-0.05557007,-0.0056707165,-0.031603143,0.040317345,-0.037378456,-0.0152578335,0.013680443,0.04052996,0.011953916,-0.020222826,-0.17022613,-0.07022955,0.043578293,-0.047327306,-0.017144226,-0.021357873,0.006830481,-0.037972003,-0.017281407,-0.030110877,0.012126511,-0.006987968,0.019766973,-0.027841708,0.012838874,-0.025904955,0.01813178,0.007186045,0.011150281,-0.026546204,-0.04616187,-0.052844584,-0.0030135673,0.033221874,0.0098594725,0.041341536,0.014945655,0.01355447,0.031856388,0.01065025,0.03812888,-0.010527024,-0.02328371,0.020824555,0.03694914,0.020769332,0.029047942,0.042608734,-0.017191194,-0.037811093,0.016331594,-0.024037907,0.021957278,-0.028930888,0.0178305,0.072735146,0.028968602,-0.016750474,0.027650516,-0.039735414,0.019435428,-0.010009181,-0.048932843,-0.003682244,-0.04198168,-0.012971079,-0.0032581342,-0.026947707,-0.04987097,0.027593723,-0.03770217,-0.04402063,-0.0052265646,-0.022943988,-0.0059282514,0.00074400223,0.015274254,-0.047783628,0.0035972118,0.021557156,0.0019884186,0.010992963,-0.02493553,0.027956078,0.04419681,0.03158012,-0.00054396875,0.020223573,0.037814014,-0.014030859,0.027461909,0.012281982,0.030021336,-0.050580237,-0.048765007,0.03628633,0.04726903,0.03161383,-0.08485738,-0.039492782,0.005970086,-0.017289663,0.031092655,-0.03049915,0.012187906,0.0009657876,5.4826276e-05,0.059580006,-0.029175542,-0.028252827,-0.014587756,0.051357318,-0.0023030574,-0.030852348,-0.042870585,0.020966977,0.01406206,-0.0072742836,-0.005500618,0.000984772,0.03915627,-0.014294956,-0.041844375,0.0071455967,-0.038435806,0.028260693,-0.0034637903,0.059151232,-0.032709487,-0.03389078,0.03226818,-0.053380378,-0.010999748,0.0013917893,0.0036275422,-0.017507965,0.0018331849,0.045886967,0.0018282083,0.032166988,0.05621914,-0.00014858252,-0.0017574484,0.046261966,0.031826686,0.015947156,-0.017608434,-0.023207568,0.011618173,-0.018583756,0.03251517,0.059683226,-0.031271867,-0.03401224,-0.023006413,0.002188836,-0.033381626,-0.005097282,-0.069431454,0.07019174,0.009760228,0.016443836,0.024830982,-0.010989225,0.023292506,0.0387617,-0.069484994,0.020784814,0.016385037,-0.02917643,0.03113573,0.07348807,0.042583063,-0.008308102,-0.04661001,-0.030853536,-0.011930429,-0.0071952078,-0.06027603,-0.015629733,-0.06875375,0.01538751,-0.0024795956,0.0021760096,0.07102302,0.02925878,-0.06273733,-0.012639818,0.010120179,0.01816015,-0.035780307,-0.031816445,-0.039977513,0.03302456,-0.051658217,-0.036197852,0.011240406,-0.019009925,0.012216716,-0.015636569,-0.032171085,0.025701871,-0.017816419,-0.0014239385,-0.028765073,0.0071185045,0.018539192,0.031108014,0.018274136,0.06825681,-0.05188507,-0.025824158,-0.011265914,0.03343298,0.023401577,0.021104641,-0.043467663,0.027543396,-0.020500047,-0.002494027,-0.029830229,-0.04318623,0.032177214,0.05485892,-0.04365694,0.008609407,0.006866184,-0.011751405,0.06980569,-0.03813828,0.01048755,-0.013700034,0.022088673,-0.033576146,-0.016442753,0.011833391,-0.0025526106,0.009127713,0.015962977,-0.052456196,-0.007860888,0.0064862208,-0.005148873,0.029514113,0.038092,0.041963656,-0.0064399103,0.019151276,0.05089918,-0.009209587,0.010672762,-0.010282619,-0.026026348,0.041138846,0.01661149,-0.06417131,0.033447832,0.0309343,-0.03734807,-0.042926162,0.02349652,0.0075916857,0.025084447,-0.009268098,0.003043124,0.009458732,0.011712384,0.018168997,-0.014124981,0.0263337,0.009757798,0.0454088,-0.026641278,0.04543691,0.085486025,-0.0060397564,0.07443815,0.004332012,0.06573979,-0.013355581,-0.022785738,0.040705454,-0.033479426,0.06901401,-0.0012272256,0.0042719287,0.051229596,-0.012554855,-0.046822723,-0.015381816,0.0030304277,-0.04681605,-0.012255084,0.071970254,0.034564655,0.038402706,0.0007736951,0.055713277,0.056601427,0.023763133,-0.0030069547,0.025842836,-0.05896051,0.005163983,0.018629262,-0.036531623,-0.09087119,-0.057780337,0.052262604,-0.026351918,0.00759475,0.0031856482,0.011286909,0.03728504,-0.039403345,0.06646426,-0.00935295,0.009591458,-4.3319684e-05,-0.032108862,0.024332374,0.019534277,-0.04719105,0.009469092,0.0076424987]
\.


--
-- Data for Name: vectorchord_text_embeddings; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.vectorchord_text_embeddings (id, snippet_id, embedding) FROM stdin;
1	15	[0.0056761997,-0.054195203,0.04095929,0.021960968,-0.05149781,-0.06971693,-0.0015759617,0.037038777,-3.9512805e-05,-0.04374183,-0.014273372,-0.04425709,0.023744788,0.036910333,-0.008078132,-0.033377744,0.0074638193,-0.00734476,-0.038653348,0.009815395,-0.0070785745,-0.01568171,-0.03644178,0.006757723,-0.025763009,0.008157233,-0.029671986,0.016006246,-0.077461034,-0.00624265,-0.034366637,0.030056823,0.07078206,0.0014277147,0.02083185,0.0039250148,-0.048580218,-0.0035930427,0.0142817255,0.036064412,0.0033740385,0.03762636,-0.07228007,0.011488175,-0.016134083,-0.0014129401,0.074767515,0.016786596,0.015284866,-0.026007297,0.06331672,0.03703856,-0.0076275356,0.021557175,0.0034040934,-0.0014972603,0.050077494,0.0068177767,-0.03810751,0.0032057762,-0.01656624,-0.0027812577,-0.024211919,0.05837267,0.036521558,0.009659288,-0.044851385,0.010359128,-0.019132214,-0.010834191,-0.02436403,-0.04462309,-0.021882419,0.035105508,-0.03455691,-0.028728858,0.027893065,0.0014589366,0.015210708,0.006344418,-0.017607125,-0.019814478,-0.04230294,0.06180689,-0.033578843,-0.018361853,0.051390793,0.0042526517,-0.009679617,-0.013829277,-0.00412685,-0.011143888,0.035292674,0.058704272,0.023608798,-0.048546586,0.028326496,-0.042098098,0.03618751,0.0016314447,-0.057888117,-0.0026258738,-0.05592908,0.01524922,0.01923484,-0.042296443,0.006110832,-0.00033224243,-0.018685514,0.00032856135,0.016834209,-0.011027298,0.05833491,0.031216746,0.030857867,-0.024281943,0.022945726,0.0020288376,0.06484021,-0.021237897,-0.05271378,-0.0039335145,-0.045668587,-0.046713304,-0.010630147,0.011943327,-0.010442455,-0.01634554,-0.009579076,-0.0033615946,-0.022000425,0.017723259,-0.028423363,0.016156094,-0.0237507,0.04683184,-0.0104234675,0.06256956,-0.023921197,0.043567874,-0.009676892,-0.032412637,-0.0034692492,0.016024152,0.02593922,-0.012189843,-0.020110486,-0.0029028568,-0.023498194,0.11712348,0.0016887693,0.057302006,-0.0638808,0.043693658,0.035038788,0.030568117,-0.051486056,0.03817832,0.005761998,-0.02020797,-0.036047004,-0.049328987,0.026662378,0.08257607,-0.05611365,0.023116525,-0.012762804,-0.010774855,-0.047359914,0.04243787,-0.059704494,-0.010881203,-0.04804099,-0.009911769,-0.03670016,0.02837552,0.047057737,0.012929901,-0.025777157,0.025716439,-0.015355833,-0.01639281,-0.022332488,-0.05084484,0.011913311,0.041017823,0.028325092,0.049095172,-0.030284878,0.027624376,0.017313868,-0.014213864,-0.0035445513,-0.015728692,0.03633353,-0.15560871,-0.025354462,-0.017357552,-0.009224028,-0.0017862871,-0.0017878315,-0.054292962,0.049236897,-0.0011626006,0.022878984,-0.0015464112,-0.040098567,0.042945724,0.023765953,0.062469013,0.034560014,-0.008603526,-0.041758392,0.019709703,-0.08251725,-0.02838611,0.05956379,0.048597664,0.018777456,0.009080575,-0.019685438,-0.013765354,0.005075966,0.016288832,0.037317898,-0.072996885,0.030088712,0.002728541,-0.035759922,-0.0091903815,0.009210952,-0.028990097,0.06960334,0.019678278,-0.008842012,0.04214524,-0.025794799,-0.0008044744,0.04121159,-0.024396414,-0.10350388,0.03816828,-0.010170772,0.0031165152,0.015969867,-0.086904235,-0.063107304,-0.14831987,0.06718329,0.011892323,0.031361677,0.04663262,0.0034553353,-0.010320795,-0.06386782,-0.013885056,0.008672654,-0.026779555,0.046224926,-0.021498209,-0.026040561,0.008375681,0.0660258,0.011903225,0.030810665,0.029333923,-0.0062808567,0.027091898,0.016458897,-0.05297692,0.029742595,0.010370911,0.04889999,0.026627602,0.034978922,0.0025533875,0.06565165,0.025920186,-0.03347267,0.025404785,0.0037246353,-0.0133749,0.008185501,0.026668824,-0.0066886633,0.068539865,-0.03570897,-0.024879735,0.027819704,0.03141422,0.0060732197,0.0014265315,-0.11445177,0.016286602,0.025038127,0.0018602805,0.02353374,-0.030368587,0.026640298,-0.052104443,-0.0041271937,0.029712511,-0.055528503,0.021932077,0.015041996,-0.0019485911,-0.0756269,0.08334087,-0.018650983,0.018130772,0.002442608,-0.018988768,0.03000548,0.005140581,0.008770585,0.02547405,-0.00923797,-0.014411547,-0.0002975962,-0.026363662,0.021419782,-0.04474236,-0.01000126,0.039889034,0.053546976,0.011415074,-0.0023927547,0.005920027,0.07281135,0.020954909,-0.05331934,0.0033313017,0.013481506,-0.12435293,-0.038301922,-0.050911054,-0.01720495,0.009115507,0.039283533,-0.010939989,-0.022232689,-0.022827877,0.057951074,0.059620634,0.058034733,0.02275217,-0.021365725,-0.045872997,0.0022987435,0.1006325,-0.016883852,-0.011893579,0.05709323,-0.046779007,-0.01709983,0.07162148,-0.029364575,0.0012235816,-0.0035602516,-0.0830991,0.01090983,0.0011610984,0.0147572765,-0.0013576575,-0.014883215,-0.041673962,-0.014963081,-0.065438464,-0.020424359,-0.04317719,0.015633103,0.10571395,0.006318586,-0.024517143,0.00074456155,-0.010607666,-0.025393184,-0.0107711945,0.025786864,0.047166273,-0.021271223,-0.036292374,0.008398288,-0.032123238,0.023865264,-0.04619239,0.0018994756,0.058810893,-0.05743864,-0.025370471,-0.0061317543,-0.018892098,-0.0789024,-0.0309765,0.015888194,0.029071348,0.0011307928,0.009752752,0.0317447,0.03608369,0.026809491,0.04978186,0.007181444,0.019489065,-0.033785585,0.027141215,0.04056394,-0.0007919143,-0.0005403315,0.0356673,0.017143102,0.048121884,0.0031259807,-0.05801122,-0.033060804,-0.016708143,0.048996832,-0.03165843,-0.042411953,0.033237994,-0.006017571,0.024925906,0.025216,-0.0028026204,-0.0052107694,0.0039058912,0.0067250854,0.050893262,0.022914525,-0.0012953317,-0.028701887,-0.031455137,0.041635603,-0.034868173,-0.042395685,0.007665846,-0.02332845,-0.05665117,0.04967504,0.060917247,0.07478021,0.04110298,-0.030344784,0.0032156669,-0.003927925,-0.046151187,0.008510942,-0.05275928,-0.05245638,-0.08677433,-0.027208319,0.027492408,0.05928904,-0.0513428,0.0056425864,0.07493583,-0.035679694,0.028501831,-0.01811517,-0.040636078,-0.04063749,-0.034148708,0.026540207,0.0011528324,0.02181755,-0.055327248,0.0045201406,0.02601173,0.023050457,0.037634674,-0.010830982,-0.027827252,0.01724747,-0.067005716,0.047529157,-0.008266671,0.0624333,0.052348997,0.016981246,0.036703847,-0.01697042,0.04797464,0.041398738,0.0055060303,-0.0008464418,-0.0057914164,0.0055981874,-2.744809e-05,-0.039222196,-0.009418587,0.05673906,-0.038595576,-0.005754582,0.027848732,-0.002947022,-0.011794904,-5.88845e-05,0.07927938,0.0015399159,-0.027675575,0.060537815,-0.014687315,-0.069149375,-0.010267991,0.027896505,0.03932827,0.006374397,-0.020823726,0.016193613,0.08448648,-0.017823633,-0.027949244,0.025488844,-0.04314635,-0.023476329,-0.0011955581,-0.012014366,0.029944979,-0.0054216464,-0.10228974,-0.029107932,0.03793161,0.022961874,0.048521172,-0.00030013706,-0.050078716,-0.060921438,0.022833567,0.017739875,-0.018647978,-0.021788597,0.037275776,-0.034112778,0.037789628,-0.047353595,0.018312776,-1.151005e-05,-0.04131375,0.023058016,0.003411294,0.019427165,-0.044515774,0.016539631,-0.011495012,-0.035098057,0.04023332,-0.026961733,-0.0030867157,0.04167402,-0.016937496,-0.013787106,0.01778531,-0.029592851,-0.008045145,-0.021408876,-0.03905122,0.004384197,-0.012498434,-0.0051419483,-0.0210318,0.028531734,-0.015369425,-0.028435055,0.020145914,0.0015484287,0.02824043,-0.0006477574,-0.017767802,0.061674856,0.02843247,-0.051109508,-0.0054434272,-0.025497952,-0.016533852,-0.015205471,-0.023623655,0.023896893,0.058064263,-0.0138623975,-0.003593572,0.04086681,0.035809923,0.045206133,0.028843183,0.0213719,-0.021055395,0.023365162,0.03247969,-0.00012925034,0.037309196,0.014133587,0.021442108,-0.014343774,-0.029458707,0.0021039953,-0.07444846,-0.045725603,0.024941713,0.02565893,-0.00903227,-0.02867247,-0.0017600546,0.0069974214,0.010647029,-0.04079763,-0.036849793,0.034880005,-0.051910162,-0.013229441,-0.027066974,0.02643727,-0.01639518,0.020129299,-0.004481139,0.032132696,0.02724859,0.01550068,-0.046843067,-0.060954705,-0.028763063,-0.0058056894,-0.008767551,-0.051732577,0.047124054,0.010109388,0.007852387,-0.008342926,0.04708556,0.046454426,0.02612406,-0.054278668,-0.039549276,-0.032976516,0.03498318,0.046884075,0.004032621,0.06294478,0.040276904,0.00823113,-0.02295468,-0.037896767,0.018145027,0.005648199,-0.00514109,-0.020591045,0.0055194455,0.015358645,-0.05504334,-0.0026847231,-0.045670487,0.01225947,-0.017505126,-0.046333693,-0.050508935,0.06931527,0.01831581,0.053752955,-0.025707675,0.041804895,0.008189726,0.027287556,-0.017309237,-0.10958021,-0.040186413,-0.01620283,-0.020132514,0.0072585987,-0.013646849,-0.013526198,-0.047562387,0.015058782,0.01792402,0.03047259,0.04078405,0.09812421,0.031895597,0.089848526,-0.004586653,0.019317716,-0.022936463,-0.02289857,-0.05050865,0.018363975,0.018528545,0.02300443,0.012568008,-0.047137372,0.049370177,-0.0066641932,0.019735254,0.017907798,-0.018691942,0.0049734185,0.06309983,0.0008640302,-0.022156512,0.047365703,-0.04184012,-0.025216281,0.0029844113,-0.017258449,-0.03551623,0.0030951498,0.015956562,-0.0087622,0.010914393,-0.013694661,0.024241451,0.025056364,0.021303238,0.020869622,0.002851509,0.03137888,-0.03508852,0.00014294141,-0.022974154,0.02756799,0.0027929354,-0.01670443,0.023276024,0.01997902,-0.011054523,-0.007614782,-0.014652792,0.089323185,0.017639745,0.055226758,0.01085111,0.015328191,0.027986232,-0.017253,-0.009816236,0.0577168,-0.034039434,0.050089303,-0.016945198,0.06267083,0.035567623,-0.052965138,0.031177504,0.041381884,0.016218122,-0.03041356,-0.010364195,0.008600164,0.011218106,-0.015850367,-0.015604988,0.048274487,-0.006783761,-0.056044053,0.08372389,0.007905615,0.0055293418,-0.060252342,0.064174175,-0.005367391,0.0009315,0.027352,0.0047356924,-0.0050893435,-0.046199284,0.024908764,-0.017398845,0.048325982,-0.06686847,-0.010835596,-0.027233465]
2	14	[-0.0147038745,-0.06401748,0.009191725,0.0671765,0.04366538,-0.08792364,0.006700808,0.0098758275,-0.0011175475,-0.029455857,0.0023468097,-0.021926828,0.031325616,0.0034870869,-0.07046888,0.012152265,0.06602276,0.003600238,-0.037562095,0.011213867,-0.031924423,-0.015520989,0.05081302,-0.0049828095,-0.021389255,0.014838777,-0.025615273,0.027221428,-0.09114421,-0.023892483,-0.034495264,0.02413281,0.030743819,0.016627232,0.03906628,-0.043332256,-0.05547729,-0.03214774,0.008271998,0.0041079903,0.06726013,0.070153706,-0.0008149364,0.02372332,-0.0026860253,-0.024477292,0.0022638023,0.027286455,-0.04561757,0.021656597,0.049074966,0.06479766,0.030384623,0.017498791,0.026416019,-0.053585548,0.08625018,0.0058475714,-0.010279505,-0.030003367,-0.010865004,0.039584473,-0.02236563,-0.0005706481,-0.009873443,-0.013126872,-0.04245848,-0.031455256,-0.021692486,-0.037085917,-0.019485885,-0.01856878,0.043938495,0.038002703,-0.05471803,0.035615824,-0.0010878722,-0.09738146,0.00017114924,-0.00044569024,0.007846259,0.004112368,-0.017285652,0.031356253,-0.005549246,-0.031733625,0.047736604,0.058320396,0.020062068,-0.020278135,-0.034907605,0.030254029,-0.0377182,0.024342576,0.024066953,-0.003462613,-0.050117023,0.044902734,0.005058935,0.006752947,-0.03676657,0.033182096,-0.015606796,-0.018871339,-0.021589188,-0.010734708,-0.03174577,-0.016008485,-0.00030541173,-0.0057776985,0.021061037,0.006375986,0.00020237935,0.0038801252,0.0058951755,-0.034158133,-0.016033307,-0.002970994,0.06804294,0.00027073655,-0.046129715,-0.018435916,-0.099029794,-0.017609324,-0.021516042,-0.016390992,-0.04454888,-0.0034890356,0.02178958,-0.013738239,-0.041123558,-0.0006831544,0.015052586,0.07333837,-0.03645739,0.018240659,0.0021742897,0.06285065,-0.026865972,-0.010912855,-0.060205705,0.0046826964,0.021458926,0.013279809,-0.061614186,-0.040190693,-0.02462363,-0.022328764,-0.08734902,0.06550941,-0.04646387,0.03367414,-0.019040916,-0.02306442,0.0005980846,0.027689591,0.068519086,0.060402688,-0.021493645,-0.032338757,-0.055416454,-0.028661614,-0.021765346,0.031636883,-0.06568055,0.03193234,0.024018358,-0.020247528,-0.02936984,0.019210372,-0.031194024,-0.0069834096,-0.03186571,-0.022324134,-0.016213724,0.029338548,-0.060804147,0.049199272,-0.027999828,0.025808105,0.0407126,-0.005897625,-0.02461482,0.013686362,0.022227515,-0.039679453,0.0021862476,0.046041813,-0.03556549,-0.029423725,-0.0321751,0.016987285,0.024937963,-0.00051449664,0.056294434,-0.10835517,-0.010792592,-0.027886387,0.03201015,-0.01050931,-0.011736165,-0.05533561,-0.04310539,-9.574098e-05,0.06306615,-0.02802905,-0.035085324,-0.017972909,0.025899874,0.008099138,0.047926426,-0.05074269,-0.0014007493,0.05863963,0.006605325,0.005041391,0.0828687,0.05920986,0.077678286,0.01915143,-0.015884241,-0.010880197,0.03639622,-0.012041975,-0.00066651223,-0.03737357,0.046019122,-0.014843407,-0.0055313963,0.02369907,0.021332398,0.089636154,0.118997335,0.074784845,-0.012276267,0.032904867,0.009304777,0.01003962,0.049001172,0.0009271398,0.006126791,0.046060126,0.012732775,0.011519325,0.009173339,-0.063435644,-0.045797363,-0.12533793,-0.0028450715,-0.024272116,0.0072728964,-0.0021662442,0.012884084,0.040437836,-0.046292625,0.024579065,0.009348414,0.0088422,0.007511999,-0.00068744854,-0.06402077,-0.0041575003,0.024547063,0.012386723,-0.010341094,0.036365155,-0.010553596,-0.004731739,0.03362726,-0.02761793,0.018121216,0.042442758,0.0561906,0.023854166,-0.009864904,-0.0024659736,0.038605727,0.056529015,0.08828708,0.04319283,-0.029388456,0.01096944,0.017253071,0.004893392,-0.0080455635,0.037802354,0.024483677,-0.0069837715,0.017329533,0.04444627,0.0056126444,0.034304004,-0.06436903,0.02083613,-0.009348585,0.0015146498,0.012517983,0.01562197,0.009108991,-0.018911222,-0.01708431,-0.07312717,0.029920893,0.06885095,-0.020506809,-0.007829445,-0.057622023,0.0800503,0.019252256,0.033405077,-0.01474127,0.031355117,0.07158141,-0.012176465,0.027481329,0.03657674,-0.01847009,-0.007781904,0.015886812,-0.023070026,0.011346104,-0.046945147,-0.0146858515,0.05120526,0.005250459,-0.03348032,-0.017073533,0.059283327,0.07222423,-0.001370083,-0.08449459,0.0013804542,0.032830324,-0.050445586,-0.04008665,-0.011432069,-0.0047296574,0.021812808,0.038175654,-0.056963295,0.016176974,0.004433076,-7.5679265e-05,-0.0012989365,0.04777743,0.015131253,-0.06208953,-0.020809827,-0.019735258,0.091078684,-0.01880539,-0.050031684,-0.002136453,-0.10041398,-0.04787115,0.069558494,-0.00010196674,0.012850031,0.01868594,0.04646301,-0.012158247,-0.0047991937,-0.028652005,-0.0009837619,-0.024505446,-0.022419384,-0.0036865524,-0.016908545,-0.018948458,0.005709457,0.0047478527,0.039124466,0.009604474,-0.0012218463,0.00397148,0.041396927,0.00030218324,-0.056652408,-0.04387397,0.030692458,0.013019268,0.06845783,0.056663133,-0.0027746416,0.011541547,0.032028176,0.052432656,-0.040828183,-0.029093046,-0.017249584,-0.0117034055,0.04309114,-0.08395844,0.0025551745,-0.0018121161,0.016643755,-0.053568352,0.014603049,-0.0029523289,-0.0063010817,-0.018405573,0.04248987,-0.016732214,0.008531781,-0.019846601,-0.04198962,0.057383716,-0.02252293,-0.028644834,-0.04449439,0.031778745,0.05207059,-0.007275925,-0.020157384,0.027945034,-0.008682077,0.08846657,-0.10356203,-0.04873405,0.04682034,-0.011213959,-0.013451579,0.047511604,0.018530965,0.0667251,0.025953785,0.028311826,0.014730352,0.025923789,-0.017195953,-0.021084175,-0.03876596,0.094797894,-4.5762303e-05,-0.03136734,-0.052149456,0.026676692,-0.024535703,0.030011518,0.03182661,0.037733708,0.077344604,-0.02959907,0.013393654,-0.009535795,-0.026345083,0.024449144,-0.015557124,-0.04319482,-0.08018929,-0.018278701,0.018193142,0.09101771,-0.07142598,-0.06637438,0.055233844,-0.0331783,-0.010752504,-0.04067064,-0.03463729,-0.006824912,-0.0028607165,-0.02558471,-0.004668292,-0.0014066184,0.018555231,-0.03517142,0.023342475,-0.048571166,0.019507335,-0.028987147,-0.016208585,0.003939309,-0.005733611,0.0076917033,0.0071628466,0.01726558,0.04879993,0.014465062,0.013366764,-0.014465134,0.0051556462,-0.0061550946,0.020730397,-0.030931564,-0.020727498,0.03036767,0.030210381,-0.0008046765,-0.008870927,0.025725923,-0.068321064,0.011952995,0.023959767,-0.010160033,-0.0073081586,-0.002909541,0.016918784,0.035464052,-0.04861745,0.0029719267,-0.007837325,-0.012900624,0.0022761505,0.027566195,-0.011107001,-0.017348742,-0.026359497,0.03858667,0.06084163,-0.05417188,-0.05110618,0.024914432,-0.011466671,-0.025870852,0.023532523,0.04270528,0.01380303,-0.033466842,-0.042125776,-0.0001420104,0.04492924,-0.008205716,0.024778755,0.014587796,-0.03724085,-0.031864263,-0.0032660004,0.044833556,0.052335076,-0.06785137,-0.0075547365,-0.043699075,0.025676655,-0.001777671,-0.044617068,0.026657002,-0.05531401,0.0011864882,-0.007891406,0.062123686,-0.09343991,0.006421045,0.0404306,0.025482541,0.019338345,-0.041077342,0.01738002,-0.009735122,-0.06394538,0.009804514,0.02148376,-0.05002755,0.010683605,0.022360697,-0.023275902,-0.023034692,0.04515157,0.005808727,-0.012599875,0.006227865,0.0569345,0.008734087,0.010451495,-0.052191343,0.002123149,0.03492836,-0.03152539,0.033672266,0.05607009,0.032678336,-0.019913482,-0.014568044,-0.03470612,0.011327681,-0.0042245137,0.019115124,0.07516718,0.05556169,-0.04075597,0.016112465,0.0025127963,0.063535795,0.013996076,0.045056384,-0.03334548,0.06027868,0.045952648,0.07513769,0.019153802,-0.05647689,-0.009066383,-0.031508483,0.041561145,0.0015790731,-0.0008245353,-0.0021682782,-0.0026632792,0.011533337,-0.06572592,-0.028676677,-0.010868705,0.04091186,0.024775926,-0.019682791,-0.027216475,-0.025274897,0.00969877,-0.037277456,-0.07023082,0.02722404,-0.009356812,0.007974141,0.015990835,0.07247096,0.025735052,0.020576198,0.014349042,-0.043575384,0.035351362,-0.032846153,-0.060363352,-0.05925331,-0.030234104,0.03206807,0.00030239174,0.00036846558,0.08421506,-0.0003835479,0.009736648,-0.06236821,0.0032168436,0.008436046,-0.007511088,-0.012118589,-0.04311393,0.055566166,-0.04228664,-0.013056188,-0.0035236175,-0.019586599,-0.023068193,-0.007479177,-0.00518337,-0.000570486,-0.0003458152,0.033989962,-0.087277666,0.024398956,-0.003112822,0.008388886,0.018384228,0.033707917,-0.0032888725,0.007504683,0.028515238,0.036173258,0.03929997,0.013080071,0.009557528,0.039376885,0.054467704,-0.019660987,-0.04585036,0.03357095,0.021952318,-0.015768342,0.0015287412,0.038937584,-0.0054587196,0.01010096,0.04012818,0.024761934,0.0723293,0.078563064,0.063425474,-0.0016275776,-0.0057561737,0.020957889,-0.015257518,-0.0025653024,-0.033996243,0.00916331,-0.0029536989,-0.012875822,0.005903962,0.034219112,-0.023458244,-0.013930061,0.018799054,0.02365375,0.023205355,-0.0052976892,0.045828544,-0.046927035,-0.06258961,0.048073385,-0.0018747671,-0.052168664,-0.011462866,-0.016088836,-0.06918857,0.017033616,-0.0070974785,-0.028760185,0.039025754,0.026475584,0.021426594,0.026167758,-0.013798416,-0.004190361,-0.008769722,0.014558015,0.017885778,0.023545722,0.008752498,0.023382599,0.060226154,-0.016985238,0.06524537,0.011907942,-0.010946897,-0.027057085,0.0024134691,0.05193202,-0.010347465,0.06008062,-0.0066633117,-0.039014354,0.022385173,0.030981066,-0.0033660776,0.004017081,0.015708366,-0.0016112634,-0.0043660956,0.07120935,0.015845228,-0.010765481,-0.022616602,-0.010762452,0.013484924,-0.006640179,0.011111386,0.0347298,-0.03723143,0.0029479414,0.0357509,0.038278736,0.00800569,-0.04926555,0.07378249,-0.07358221,0.01827228,-0.026322847,-0.008981138,-0.008287404,-0.0023027977,0.0018075502,-0.017978277,0.002171436,-0.01651863,-0.060526695,-0.015345992,0.045012433,-0.061886925,-0.0016580836,-0.030868482]
3	13	[0.0012451734,-0.040442083,0.008903834,0.036146365,0.01812387,-0.025068916,-0.017947825,0.027205266,0.01386335,-0.012669514,-0.020434044,0.020004746,0.0012574032,0.0005340007,-0.078718245,0.03549598,0.057910714,0.055086475,-0.012104194,0.04895551,-0.033428323,0.08209916,0.03389155,-0.040819634,0.020900719,-0.007363205,-0.026477482,0.0063728373,-0.022007255,-0.018643135,-0.063428745,0.047658317,-0.0025626917,-0.00975742,0.024153959,0.014748464,-0.02794585,-0.041965343,-0.013902901,-0.0025199195,0.023152146,0.043751184,-0.00018270126,0.040425904,-0.0010535453,-0.004071146,-0.031101521,0.08210334,-0.029542562,0.06396408,0.005431924,0.033681847,-0.03700115,0.0051151263,0.010244855,-0.028537288,0.038964678,-0.06313976,-0.020297447,-0.014737738,0.02099169,0.08118649,-0.04965163,-0.01456678,-0.0027830559,-0.030126067,-0.034122366,-0.010427808,-0.029041113,-0.0052822023,-0.0025732385,-0.010370565,0.010887514,0.019873157,-0.0027353533,0.07506317,0.015506529,-0.09491463,-0.016833676,0.011440137,0.014866024,-0.0057593305,-0.010705194,-0.0030052783,0.009641456,0.017692523,0.04249109,0.04521542,0.02488657,-0.0028720521,-0.038027793,0.03338957,0.017197058,0.011244941,0.028940426,0.0370308,-0.04470486,0.04512202,0.031925183,-0.0013682886,-0.01430598,0.0053491504,-0.039658945,-0.05278305,0.023008212,-0.023953645,-0.073960096,0.005141795,-0.038326107,0.022414058,0.046766173,0.059039976,-0.017852833,0.024895739,-0.0296149,-0.054464947,-0.012476358,0.006625259,0.056728948,0.059288353,-0.045136742,-0.012970303,-0.08943838,0.03519237,-0.03915312,0.057790503,-0.011900491,0.03886924,0.022183677,0.0022686156,-0.004062498,0.08376244,0.015006257,0.06200593,-0.038018916,0.013028612,-0.093702145,0.049768478,-0.034687307,0.018208183,-0.02391724,0.02549579,0.040839095,0.025821367,-0.05371214,0.0138229495,-0.039807223,-0.035401125,-0.054590512,0.0039906534,-0.0024783183,-0.004761887,0.018913615,0.007990708,0.022877363,-0.0033589604,0.011610449,0.04798407,-0.0031599503,0.031438287,-0.028118437,-0.02003871,0.061756108,0.04444196,-0.01574605,-0.007012578,-0.016000535,-0.028136915,-0.0067648585,0.015485793,-0.015700659,-0.010667352,0.0100723505,0.005973057,-0.030115306,0.010438298,-0.0023069757,0.032463785,-0.014702211,0.030230746,0.060874563,0.013126066,-0.02451381,0.023304481,0.0119375205,-0.053037014,0.007542789,0.013877581,-0.038396545,-0.052461307,-0.020258605,0.030375285,-0.014861886,-0.04056714,0.027095826,-0.030670395,0.011687293,-0.023858223,-0.0034974024,-0.031870186,-0.0010660161,-0.07886898,-0.034787025,0.043300513,0.050373595,-0.05714742,-0.08150453,0.002398451,-0.02442897,-0.008434714,0.054602772,-0.06782559,-0.0033751805,0.0405015,0.117847085,-0.044033263,0.06847283,0.12783259,0.06814011,0.050581295,0.055787213,-0.013468327,-0.0037644422,-0.012406318,0.022876795,-0.045579102,0.01455443,0.052387357,0.038808145,0.003183879,0.005981591,0.040323336,0.1082589,0.01208283,-0.020226136,0.018574074,-0.026712745,0.0024796114,0.018247604,-0.029196076,0.055442426,-0.0106632225,-0.01428176,-0.021660712,0.0063083163,-0.048465267,-0.035089843,-0.14485152,0.026523475,-0.032140337,0.020312548,0.029531082,0.030156143,0.041653104,0.024137361,0.023591617,0.0018651057,0.015536327,0.046682723,0.00884672,-0.07093377,-0.015310806,0.016777398,-0.026364395,0.0011268392,0.07387338,-0.051702593,-0.021713745,-0.0034089847,-0.009821392,-0.0019417775,0.015581811,0.06880399,0.06835629,-0.016272351,-0.032909445,0.058239654,0.03696525,0.07518358,-0.016621942,0.017046493,-0.039683502,0.04151523,-0.013282354,0.00068581273,0.03739,0.05041873,0.0054100086,0.012415139,0.044468388,-0.011224501,0.021189505,-0.020652505,0.008923724,-0.0062041064,-0.020988213,-0.029154852,8.066058e-05,0.05488153,-0.013936964,-0.00707841,-0.024354244,-0.03376268,0.0375665,-0.05089981,0.0015015361,-0.02395917,0.08042251,0.021579586,0.004204698,-0.001066049,0.025256582,0.031919602,-0.018663889,-0.004843831,0.0038175085,-0.044362154,-0.004865223,-0.0074517285,-0.011528267,0.01376486,-0.008044846,-0.039764665,4.389763e-05,0.010250044,-0.065166146,-0.023502411,0.010687716,0.042006638,-0.008583411,0.005753187,0.006364936,0.10899268,-0.08969775,-0.031502318,-0.0083340425,0.030677278,0.031572565,0.04076243,-0.052133895,0.025118263,0.0010482866,-0.05069972,-0.027815262,0.013245111,0.0030668834,-0.04412519,-0.007032326,-0.019861612,0.050090197,-0.0045645298,0.004591055,-0.030356029,-0.06989256,-0.0109476615,0.022606492,-0.0071898126,-0.020621222,0.00855659,0.00067463645,-0.00049173733,-0.033434965,-0.06447696,0.027687885,0.014823721,-0.010239783,-0.000685977,-0.04371962,-0.03358293,-0.018446332,-0.021005549,0.037715536,0.009823802,0.015474223,0.018404141,0.024396183,0.053316686,-0.056353666,-0.061034083,0.03742352,-0.023555469,0.06903235,0.047364555,0.005078955,0.0038409054,0.04574361,0.043684296,-0.0723464,0.010186914,-0.023074593,-0.021377534,0.024528537,-0.05236801,-0.0050179567,0.04263959,0.017113546,-0.033172056,-0.016422378,-0.011130854,-0.023685656,-0.026772287,-0.0043251636,-0.006810546,-0.023329929,-0.0050595966,-0.017015079,0.033240337,-0.018609907,-0.021575814,-0.0410589,-0.00018650111,0.06710842,0.00092422095,-0.011382209,0.026818508,0.025882734,0.035234917,-0.021415167,-0.026156267,0.028785339,2.3134848e-05,-0.03639685,0.028696913,0.0119669195,0.036063332,0.033032846,0.03524471,-0.03143664,0.015608614,0.0057529947,-0.0135440035,0.0016188844,0.08908698,0.02259166,0.01473625,-0.013670425,0.019802561,-0.035017375,0.04928632,-0.017166343,0.060193535,0.0010394509,-0.0076668877,0.0057148533,-0.005136481,-0.017102832,-0.0027019968,6.7412657e-06,-0.045542967,-0.06834313,-0.0033102422,0.0697814,0.12726139,-0.058330294,-0.015863601,0.033237837,-0.036377434,-0.0031674758,-0.013434675,-0.0010117894,0.0039003238,-0.028721875,0.02965823,0.02427617,-0.026917625,0.03838885,-0.053482424,0.06036016,-0.02093409,-0.0015011496,-0.009334452,-0.0428257,0.0024103539,-0.0031676877,0.014172797,0.02405551,0.00030255335,0.055522878,0.0316644,0.016143799,-0.018834615,0.022539329,-0.02507597,0.010400697,-0.037556548,-0.00064698776,0.054108113,0.016054438,0.009342416,-0.01860573,0.034697037,0.008167499,0.0038304664,-0.0071294648,-0.0077639073,0.0096212495,-0.053409915,-0.036017112,0.008322158,-0.012399565,-0.00022410318,0.00031344136,-0.014197526,0.021909004,-0.008956889,-0.011961032,-0.05234849,0.01190088,0.01675652,0.023822738,-0.037776526,0.0047974163,-0.013467797,0.0054669264,-0.029651357,0.01649528,0.056887712,-0.0012926612,-0.051715568,-0.010472736,-0.015752954,-0.002204975,0.0038860706,-0.004450204,0.025123771,-0.04110517,-0.001150582,-0.010953275,0.07940323,0.0184357,-0.06782292,-0.025847364,-0.050992694,0.017220326,0.0113366535,0.00018498903,0.03069272,-0.035255007,-0.03516931,0.012571163,0.0069479803,-0.079825185,-0.0031410612,0.054608133,0.02127993,0.009813854,-0.02772308,0.009469981,-0.02535916,-0.024925794,0.05744491,0.042205393,-0.037821963,-0.015791252,-0.027051795,-0.038718082,0.021497808,0.020635307,0.012459098,0.028357398,0.0067973677,0.12259316,0.00072680396,0.026176568,0.04715793,-0.027170382,0.020235194,-0.024716547,0.072439514,0.024381425,-0.021919226,-0.043724902,0.015252466,-0.059541482,-0.035333507,0.0147775635,0.008411429,0.12540331,0.0036359818,-0.040965624,-0.041940138,-0.0055063944,0.025038116,0.02115387,0.011182709,0.0052735717,0.016084166,-0.0006997606,0.12053285,-0.037087537,-0.05314795,-0.024218239,-0.0032237512,0.049358048,-0.03938876,-0.04137384,0.012509097,0.021391869,-0.0046441066,-0.011430407,-0.013860434,0.012640882,0.032726143,0.0003015033,0.0017479854,-0.021649955,-0.0063630897,0.0130241765,-0.03683903,-0.06868954,0.01142138,-0.001270058,0.019739313,-0.04871116,0.017756522,0.040722493,0.04393695,0.022686554,-0.003180437,0.028949369,-0.008603361,-0.06506989,-0.041223545,-0.039265264,0.029580487,0.025444554,0.054531638,0.019766258,0.020531936,0.020450417,-0.051092304,0.025339091,-0.023079028,-0.022499247,-0.045404118,-0.053540897,0.01918352,-0.056519553,0.0074395663,0.010889019,-0.01829527,0.05468783,0.017791763,-0.013496007,-0.0010523825,0.025531244,0.016109897,-0.12298727,0.042441692,-0.036968492,-0.0075828717,-0.011471484,0.06854848,0.045931984,-1.3037897e-05,0.06384451,0.027308434,0.055096064,0.019813662,-0.0077430485,0.020546695,0.051701944,0.02005051,-0.034013227,0.0048185783,0.009783264,0.025610607,0.0659737,0.013124943,0.0056042694,-0.050513174,0.026536126,-0.019174853,0.03412574,0.017926537,0.019798184,0.009335103,0.034855474,0.054321267,0.0071533006,-0.025199464,-0.04434525,0.012195351,-0.020334553,0.020316433,0.016383156,0.010974102,-0.04651299,-0.044580977,-0.033867005,-0.0042834757,0.016951472,0.0013025773,0.04103033,-0.04317253,-0.04954345,-0.018808179,-0.01073714,-0.027127905,0.005057789,0.0011785714,-0.052355986,0.0027987415,0.005092152,-0.0069924206,0.020648625,0.061129816,0.026133837,0.0737511,-0.007343929,-0.023545125,-0.03061247,0.039976865,-0.02214016,0.017572729,-0.022176942,0.021574313,0.04801977,-0.022999812,0.011849509,-0.0522939,0.004149745,-0.05136074,0.046004627,0.03037074,-0.008729475,0.0306235,-0.043170176,-0.05029267,0.020419266,-0.0036557452,-0.056640584,-0.03842317,0.026610866,-0.064692765,0.044458535,0.038528588,0.01325762,-0.023723185,-0.087839864,-0.005225491,-0.033009607,0.0070922053,-0.025418289,-0.01333803,0.0051973173,-0.017428888,-0.015757998,0.0018586332,0.0027364693,0.0144548,-0.0044991225,-0.07902776,0.014167169,-0.018379051,0.023132183,0.024677685,0.056690775,-0.026218357,-0.0017691458,0.038959067,0.015595657,-0.040542502,0.002769775,0.011947566,-0.12477006,-0.012604309,0.005004514]
4	12	[-0.0032884246,-0.039841115,-0.0048039625,-0.013683566,-0.0005870079,-0.028353361,-0.0034536992,0.019565452,0.0019218788,0.0066018575,-0.03739091,-0.05808181,0.0066099353,0.051584415,-0.012866119,0.027103413,0.046824373,0.014003107,-0.016871639,0.0064103245,-0.027585603,0.03419818,0.008914504,-0.018580006,0.0037508537,0.025515031,0.0100927325,-0.047376934,0.023182796,-0.061714914,0.0051894328,0.016613703,-0.022491347,-0.013035659,0.03205737,-0.03826129,-0.015014899,0.0076029915,-0.06554227,-0.04190155,0.0780463,0.045828883,0.025610493,0.056536876,0.033113815,0.008586734,0.07116018,0.010123956,-0.041525237,-0.008078238,-0.024021711,0.0814335,0.0058112303,0.010390492,0.051762808,0.035816643,0.005915086,-0.051135484,0.015331618,-0.030045897,-0.004535345,-0.004770474,0.0156764,-0.017326687,0.0276659,0.058916,-0.0043080817,-0.013087188,0.0043249563,0.033420894,0.008026912,-0.03823374,0.0050798017,-0.067469135,0.03580844,0.010772463,0.009986711,-0.02347206,0.083890274,0.019334594,-0.0067841043,0.029477447,-0.03140528,0.0010299273,0.04664243,-0.0007724551,-0.0023690264,0.06423221,0.01270123,-0.03428331,0.060997274,0.04890187,-0.00719998,0.06801739,0.011109679,-0.038489953,0.008951926,0.0070311055,0.03495706,-0.015217863,-0.037866805,-0.019009357,-0.015603887,0.05156282,0.015371782,-0.04198887,0.022763887,0.022342362,0.001526267,-0.03609582,0.022094704,0.07775789,-0.02026913,0.035880055,-0.04998153,-0.06702635,0.008470833,0.03049384,0.073589124,0.040243655,-0.04076507,0.036638375,0.036252327,-0.00055496016,0.03987944,0.032216463,-0.052138,0.031601403,-0.042369127,0.122064315,-0.043110933,-0.034644995,-0.03816709,-0.027866418,0.04327651,0.026653245,0.010838145,0.028416779,-0.049442608,-0.006839733,0.0068253935,0.034203287,0.0139857875,8.591756e-05,-0.020804709,-0.016893897,-0.049091112,0.02278057,-0.024775878,0.03600916,-0.02148993,-0.035232738,-0.018718682,-0.02515887,0.0166434,-0.0093567595,0.009954894,0.061186936,0.010148542,0.02371842,0.044021577,-0.05487347,0.008225712,-0.05317247,-0.01404464,0.0004303726,-0.032096248,0.044783846,-0.052432876,0.008562333,-0.043170407,0.030173559,-0.027919024,0.040787056,-0.015848085,0.034858234,-0.00578294,0.059109934,-0.04294863,0.00085967255,-0.02896996,0.012507738,-0.0070554824,-0.03424107,-0.01184506,-0.027915405,-0.02611791,-0.023141801,-0.016512679,-0.039089646,-0.0075937808,0.04335075,-0.014180618,0.015417544,0.020838052,-0.05478903,0.0013946675,0.006952048,-0.024264025,-0.044891786,-0.069780156,-0.04859793,0.042802773,0.033166002,-0.03628855,-0.051223237,-0.043403212,0.0020867109,0.035513964,0.036028437,0.03973392,0.01674001,0.037074868,-0.008741612,-0.07210497,0.023600709,0.03312197,0.05809159,0.12345233,0.0128915,0.0010038759,0.04181959,-0.025419993,0.02434648,-0.028325722,0.0068254275,0.043581832,0.008759509,-0.0360622,0.06262376,0.014768591,-0.0025698654,0.03563216,0.022250824,0.024667913,-0.06583535,-0.0036981825,-0.0213594,0.04474054,0.007795988,0.049375754,-0.017533356,-0.032182913,0.08352007,-0.021002447,-0.026009038,-0.04374519,-0.02166559,0.023370111,-0.009924998,0.030980961,0.020619426,-0.03129025,-0.004859631,0.047040407,0.018286018,0.033821914,0.028129099,-0.009084297,-0.07881432,-0.0017497833,-0.025835445,0.0350816,0.07131388,0.027782999,0.079717115,-0.052497175,-0.018172089,-0.021487543,0.0023709445,-0.041959934,-0.0019802027,0.017088793,-0.01680373,0.0005797312,-0.019257957,0.11592725,0.04640499,0.10909016,-0.017933263,-0.02711554,-0.009872067,-0.056246396,0.0061636064,0.017770959,0.026514612,-0.011644232,0.099705964,0.016386487,-0.0060998504,0.010750586,0.008056684,0.0479142,0.017926194,-0.015976077,0.00093925255,0.010651882,0.0060908827,0.046776082,0.00152309,0.01684341,-0.02388928,0.01786875,0.021349274,-0.054077018,0.023274919,-0.07068335,0.038045857,-0.03404583,-0.02281953,-0.028969994,-0.0043975795,-0.015463215,0.059327338,0.04952814,-0.011166238,-0.07999146,0.034995336,0.0069290577,0.0065260064,-0.047445774,-0.025020065,0.019647432,0.024689982,-0.06636896,-0.006103495,-0.04565873,0.008865587,0.07364625,0.036273997,-0.055165783,-0.02488659,0.037889384,-0.02221148,-0.017098214,0.0009576802,-0.0136376275,0.027062196,-0.014255728,-0.021137116,-0.011025851,0.004911226,-0.07029607,0.035066392,0.016962044,-0.0015515859,-0.008198191,-0.010958469,0.004509045,0.037490144,-4.0659615e-05,-0.037102968,0.024148205,-0.026424611,-0.010724413,0.077155255,-0.074252404,0.046775676,-0.029510323,0.055946108,0.052824944,0.02924973,0.0057090954,-0.043643586,-0.015526557,0.013290019,-0.0056656674,0.018417489,-0.014599979,-0.046307366,0.021392092,0.04674623,0.030313347,-0.057188053,0.022861615,0.022716478,0.08407921,-0.02750577,0.0028822017,0.0154016595,-0.02400837,0.037493456,0.02832564,-0.004907206,0.015113998,0.0031283996,-0.023366323,0.036527622,0.02527296,-0.03373754,0.041821316,0.050643377,-0.010851179,0.014640107,-0.00097473484,0.002443984,-0.008451883,-0.071795024,-0.033138018,0.0022179584,-0.020761315,-0.009792406,0.0013628161,-0.03747414,0.035790738,-0.013406784,-0.0034086814,0.026728807,-0.021824928,-0.039567966,0.02034559,-0.019955877,0.012513648,-0.07729684,-0.04295874,-0.016374376,-0.04917034,-0.05615714,-0.001452594,-0.0133915935,-0.05362074,0.01527319,0.020297483,-0.018886084,-0.061517462,0.0402676,-0.012987855,0.029635793,-0.10150495,-0.041871227,0.011407417,-0.0056440625,0.078779034,-0.03208113,0.004839334,0.049618237,0.029192824,-0.015090904,-0.030042611,-0.004838202,0.056491572,-0.023204794,-0.013985812,0.021720994,-0.008837579,0.08366106,0.047800813,-0.01830475,-0.002574048,0.03751456,0.021037616,-0.025112243,0.022499293,-0.09610387,-0.08266362,0.032678597,0.027619082,-0.022843704,0.012763838,-0.036508612,-0.007152883,-0.071955524,-0.018760977,-0.019316314,-0.02376983,-0.004493687,-0.042443573,0.031135567,0.009034621,0.01713993,-0.04878967,-0.06755279,0.03740405,-0.022024762,0.020401344,0.022788677,-0.002912835,0.025584029,0.010235287,-0.088971205,0.017902507,0.005979426,-0.03519235,0.058163095,0.02906808,0.016777804,-0.011252016,0.033349816,-0.016855706,-0.032935504,-0.00015583663,0.028153183,-0.065600164,0.021188617,-0.005577638,-0.032160316,-0.0065139923,0.054415062,-0.018142166,-0.0029581848,-0.0056893053,-0.02573306,-0.015720041,0.02806738,0.03578087,0.021877179,-0.011644163,-0.01594686,-0.045122214,0.014577378,-0.011402903,0.031878393,0.011681303,-0.037402708,-0.024425955,0.0073913634,-0.0023497215,0.05167813,-0.028726228,0.032372564,-0.02198823,-0.012587391,-0.011176185,0.07861621,0.007902104,-0.0017030524,-0.055358823,0.020463863,0.048044287,0.027785966,-0.05749006,0.02661475,0.0043856283,-0.031505436,-0.092394784,-0.018985942,-0.034210823,-0.04548793,-0.0062432634,0.0075244233,-0.011645332,-0.04646808,-0.045327395,0.009479793,-0.011448935,0.004987239,0.02930366,-0.039525934,0.03976252,0.009241548,0.027292207,0.037248436,0.013391637,-0.030279953,-0.020212889,0.011961657,-0.061897982,-0.044136766,-0.005878069,0.0232754,-0.002875845,-0.084019005,-0.04937893,-0.02089314,0.06565077,-0.0067924946,0.0061215702,0.030283693,0.054542862,0.0465549,-0.021511775,-0.05350223,-0.0071902988,-0.016920209,-0.01567321,-0.005163517,0.017366795,0.12559831,0.03552052,-0.002300247,-0.110147364,-0.06506575,0.018171608,-0.0077473237,-0.00686482,0.023904106,-0.03221189,0.0026576205,-0.016512085,0.025903763,0.009174933,0.023580989,-0.030769033,0.007980841,-0.03270826,-0.029524114,0.019102423,0.006535384,0.016420659,0.0014251793,-0.016512766,0.0046538245,0.02404034,0.028548252,-0.02705258,-0.009054503,-0.0313739,-0.0045830915,-0.09548979,-0.0010032525,-0.00048714422,0.0088438485,0.004320263,0.019030612,-0.0013903731,0.03116109,-0.017314985,-0.012180161,-0.024392653,-0.004046785,-0.0062444354,0.012931295,-0.048835516,0.041314222,-0.034032136,0.03766483,0.011046392,0.02676435,0.018160569,0.050493076,0.074700736,0.021496681,-0.0059623695,0.035959862,-0.0153784035,0.040520277,0.032020804,-0.047198158,0.012772949,-0.0073230634,0.00014669192,0.001885775,0.031259034,0.01689305,0.03665592,0.0109630665,-0.004530898,0.10416531,-0.0012982088,0.006599605,-0.017366406,0.01654248,0.117384106,-0.020339288,0.03270861,-0.037777588,0.063596085,0.01695857,-0.02985966,-0.00599177,0.046644215,-0.047892176,-0.03331451,-0.054851588,0.009687963,0.029499678,-0.036604404,-0.05047123,-0.020409014,0.07258464,-0.087541476,0.0033329225,0.025004316,0.029555758,-0.0143520925,-0.0033490234,0.018716354,-0.055476584,-0.044342637,0.01578931,-0.010642398,0.013278089,-0.046252236,-0.0076191532,0.005188912,0.043443404,-0.004976446,0.03600397,0.039528694,-0.052046183,-0.016965771,0.040669646,-0.0063110655,-0.017671125,0.0042791986,-0.069172055,0.05009337,0.019947462,0.0020047799,0.045294084,-0.010265692,-0.008387465,-0.011649679,0.024457999,0.0039977403,-0.0022426713,0.06722725,0.061711047,0.009526084,-0.053433802,0.03230784,0.015656803,-0.037586022,-0.019384326,0.002537818,-0.029410096,0.01565921,0.05913754,-0.013341801,-0.005635267,0.053123247,0.06316653,-0.0057381564,-0.036581915,-0.00238345,0.0157821,0.0018696233,-0.04184964,0.037556127,-0.004092486,0.008249633,-0.023434622,-0.032535926,0.066808596,-0.027857084,-0.0101700025,0.027804837,-0.03590696,0.03018416,0.011590562,-0.029853927,0.01531929,-0.07042057,0.003696916,0.0039517656,-0.0026415298,-0.011999368,-0.037165325,-0.02724147,-0.0235087,0.0070875613,-0.010340725,-0.054413874,-0.040246304,-0.016915353,-0.040411334,0.054712266,-0.0042533893,-0.056922756,-0.0079921195,0.055701636,-0.015565756,0.026889253,0.0027446318,0.04422565,0.0054810415,0.046375647,0.009513624]
5	11	[0.012254991,0.016567653,-0.024382124,-0.0032159642,0.0006447402,-0.016535332,0.028384421,0.0055951127,-0.0028148135,0.008495137,-0.06511026,0.0638,-0.001457375,0.03563401,0.046112236,-0.028250884,-0.024781602,0.04429675,0.0094971,0.06916681,-0.020236587,0.03770337,-0.098631546,-0.06261594,0.0139917005,0.04804625,0.006180292,-0.0017328167,0.0070184204,0.056363802,-0.015775196,0.009644023,0.0013454314,-0.0087651415,-0.040971182,-0.0005994041,-0.015583206,-0.024090204,-0.044292517,-0.010023549,-0.05257031,0.042425714,-0.049436457,0.006085945,0.012799914,0.026843924,-0.085836284,0.02352023,-0.005277588,-0.0039251177,-0.0019625442,0.012131975,-0.038063653,-0.028959135,-0.018441232,0.03771253,0.0006541236,-0.016491039,0.0013049826,-0.027190922,0.0583948,0.049642116,-0.0490217,0.040843047,-0.057345677,0.0040902896,-0.0015766476,-0.001756146,-0.018272258,0.040052667,-0.022777477,0.0205225,-0.01160478,-0.049573638,0.050532967,0.07674419,0.01200069,-0.070607916,-0.03758391,0.013810619,0.04916215,-0.0105035845,-0.09010332,-0.018543594,-0.007793398,0.040408097,0.013958408,-0.03593936,0.021150773,-0.0014558212,-0.039537285,0.018393742,0.008031435,0.008515925,0.03825989,0.047542535,-0.014332231,0.00061349414,0.024999991,-0.00047982996,-0.010686814,-0.002382853,-0.0018020306,-0.017886499,-0.005072709,0.0035703203,-0.05594359,0.049647924,-0.026298694,0.029041827,0.04920122,0.0058623357,-0.002237865,0.026452703,-0.008648072,0.008404929,0.017585145,0.02209955,0.038167756,0.06925566,1.7650465e-05,-0.0008743107,-0.015931316,0.1274883,-0.042915594,0.07023647,0.037468445,0.06420858,0.031586487,0.031065965,-0.036115598,-0.01784361,-0.034765817,0.047045723,-0.008436786,0.019160248,-0.0421333,-0.018456629,-0.07928513,0.043599326,-0.021228064,0.012864877,0.0029794609,0.026649967,-0.06365661,0.04061971,0.02143234,0.0009737508,0.018884642,-0.043820683,0.0057617673,0.004120077,0.036764033,0.0010734983,0.023728972,0.03527587,0.017887533,0.059217297,0.029853975,0.059811383,-0.003665644,-0.056054663,0.058067225,0.024155295,0.031372517,-0.028829236,0.004510035,0.011323939,0.01717516,0.01442036,-0.0028812033,0.018522136,-0.0045345384,0.030274067,-0.060719386,-0.032392304,0.03817818,0.01468756,0.011887927,0.0350235,-0.009107168,-0.0020310464,0.023105431,0.038104177,-0.04386138,-0.037643984,0.012973044,-0.0062527624,-0.021381551,-0.03441272,0.052710127,0.044795547,-0.030833215,-0.043998055,0.028957134,0.014014205,0.008164788,0.0051366007,0.0018123667,-0.053632956,-0.043369543,-0.03284515,-0.031312566,0.009582567,0.030582642,0.012556456,-0.07791961,0.007029407,-0.032107178,-0.013306049,0.054466214,-0.12963836,0.0052469457,-0.030005239,0.094247304,-0.041876644,0.017022973,0.13556994,0.12826605,0.0520615,0.018322224,0.010674395,0.017699415,0.022995688,-0.011456603,-0.10204873,-0.02280336,0.06377235,0.035755266,0.029650258,-0.023445725,-0.04908495,0.066633746,-0.04552015,-0.022764582,-0.0062632975,-0.031690497,0.012108235,-0.009126961,-0.05926464,0.03825662,0.011159757,-0.009209054,-0.0058191386,0.0105134845,-0.009050851,-0.048898555,-0.06277169,0.062101636,-0.056886647,-0.0033467826,-0.058730062,-0.035277583,0.057340108,0.019904392,0.007690994,0.00846273,0.021461267,0.01112994,0.008585694,0.0075043947,-0.079446755,0.0040128324,-0.08697996,-0.016281677,0.07694506,-0.04114024,-0.017040739,-0.017631676,-0.027530247,-0.039251674,0.0003429124,0.022242172,0.042299654,-0.016138429,-0.06253854,0.06959749,0.01122366,0.0075241444,0.00865282,0.0678482,-0.041477207,-0.012096775,-0.007118059,-0.0602204,0.013654887,0.014050208,-0.033812635,0.030231174,0.019494403,0.037691396,-0.036935207,0.017727943,0.026893133,-0.029944237,0.0047256416,0.009136656,-0.056632403,0.06697074,-0.02091823,0.011474252,0.0478774,-0.016584946,0.00023667609,-0.051434413,0.027310524,0.012055849,0.019842358,-0.016471691,-0.050569624,0.0065874946,-0.0232118,-0.0008766695,0.04262559,0.011084745,0.046443775,-0.016406072,-0.018898703,0.02873477,-0.020757701,0.02165831,0.037693806,-0.016122196,0.03778488,-0.015559229,-0.077535585,0.013276184,-0.050706387,0.014269039,-0.025505152,0.05287832,-0.035839368,0.098176405,-0.029837249,0.009633298,0.010372033,0.03146946,0.023201939,0.03790708,0.027840137,0.04282791,0.003833713,-0.030510066,0.01992062,0.011403384,-0.023711225,-0.010068593,-0.018276144,-0.041789073,-0.03017638,0.020109674,0.07987899,-0.012743879,0.045688346,0.02802006,-0.0020840631,-0.05210152,-0.01935063,-0.057093456,-0.026342722,0.018018516,0.0029258162,-0.013138411,-0.0035327305,0.01268084,-0.07515421,-0.05924896,-0.017191553,-0.0066257617,-0.03848442,0.02630346,0.038611956,0.015291693,0.04357731,-0.010475589,0.010238047,0.047544807,-0.047935173,0.0465186,0.022371404,-0.042632278,-0.006007698,0.03690664,0.0126765985,0.014155078,0.021873884,0.0017523605,0.036629908,0.031991772,-0.015486794,0.02093181,0.009873374,-0.047070444,-0.028403819,0.026766382,-0.02958443,0.004242538,-0.10219865,-0.039916106,-0.03171817,-0.019790297,0.044283543,-0.024912676,0.019550653,6.7197165e-05,0.0739996,0.007551721,-0.03552373,0.023470808,0.010030417,0.06760127,0.046335403,-0.016951492,-0.037778214,0.02260197,-0.0014494698,0.013836166,0.022889225,0.013056225,-0.042229056,0.019286966,-0.048413567,0.034408018,0.009880295,-0.012385592,0.02044469,0.0033541534,-0.009744144,-0.023180814,0.038570236,0.06320812,0.019054923,0.01992542,0.002065518,-0.0034364723,-0.0012236808,0.006790233,-0.030288141,0.07289554,-0.009683308,0.024820136,-0.05746996,0.00014800613,0.003183572,-0.0011620203,-0.04687097,-0.019559423,-0.00814153,-0.024178807,-0.05435874,-0.0067382357,0.0659978,0.05011271,0.042477,0.0016104373,-0.03918972,0.020838335,-0.01878434,0.023367452,-0.0073618884,0.02618002,0.010804572,0.04676611,-0.0051742094,-0.03402994,-0.020240108,-0.046808705,0.07469453,0.017744161,0.020541625,-0.010954334,-0.065338,0.009317567,0.0038357927,0.012385646,0.016488822,-0.011366874,-0.0037533538,0.013972816,0.0067830407,-0.004513101,0.009923751,-0.015238713,0.021093726,-0.045016132,0.014802439,0.02034342,0.013487637,0.0048356983,0.034356974,-0.007368813,0.048700266,0.047717534,-0.020664265,-0.0036036312,0.016822042,-0.069940366,0.010826703,0.0006258277,0.0056625884,0.006148517,0.00056011125,-0.01030462,0.009389915,-0.009357416,0.028816314,-0.0014258206,0.011011869,0.022689462,0.018729309,-0.006141403,-0.010576948,-0.0052434835,0.024888897,-0.05117525,-0.005399935,-0.010478061,-0.0016653588,0.011616771,0.02161406,-0.0063402383,-0.041342963,0.021279313,-0.012737599,0.037113328,-0.033785984,0.0060443166,0.025089692,0.009964139,-0.03229316,-0.03095458,0.0051483912,-0.01402568,0.0020144612,0.013264772,-0.016346257,0.008523618,0.024446527,0.02224796,0.053951688,-0.1193206,0.010086964,0.015774429,0.014013603,-0.04188565,0.020681268,-0.027962197,0.007948562,0.016302418,-0.026234392,0.05362476,0.09676306,0.030092731,-0.01473837,-0.03714727,-0.011415676,0.023884613,0.050154958,-0.032874614,0.00980003,0.03438529,0.05961391,-0.04329755,0.024989301,0.0931033,-0.044470012,-0.017333359,-0.012732619,0.013023689,0.046439823,-0.11308187,0.0005803659,0.023538288,-0.0072581917,-0.020474913,0.021371702,0.020329531,0.07896615,0.0025412978,-0.033723854,-0.04542341,-0.042811573,0.016850779,0.07789375,-0.013029003,0.026706304,-0.07147412,-0.0008653837,0.01813218,-0.041681208,-0.016196935,-0.009115912,-0.013768445,0.012201114,-0.025946166,-0.0942773,0.035306532,0.013947248,0.0059055192,0.030343374,-0.048679315,-0.005466716,-0.010185843,-0.017112708,-0.031541623,-0.020077402,-0.0061166785,0.013139296,-0.05462679,-0.044545416,-0.011947201,0.009730858,-0.002273085,-0.039341077,-0.05259596,0.0002723705,0.025320651,0.024677748,0.0010363935,0.042293534,0.015200658,-0.02320609,-0.054103613,-0.023131683,0.05514157,0.0057217716,0.01241639,-0.03989977,0.036891617,0.022573778,0.01227504,-0.03344496,-0.02602627,0.002273954,-0.027619496,0.0041263676,-0.029450102,0.0010660294,0.018397216,0.023056675,-0.023203416,-0.029556554,0.003185213,-0.013919264,0.004369835,0.0006817252,-0.023130512,-0.065402515,-0.0041645006,-0.027134662,-0.028141933,-0.021327322,-0.02997069,-0.046449937,0.02844295,0.017978258,0.00012742041,0.0128349215,-0.03284735,-0.012605071,0.039412014,-0.014411318,0.023947267,0.009567159,-0.043154687,0.023779975,0.0030169482,0.03403522,-0.025029317,0.0005286984,-0.026226679,0.029475518,-0.013181531,0.016367901,-0.04713678,-0.032201152,-0.024299514,0.022707304,0.048304822,0.046852455,-0.001629298,0.012591324,0.050832056,-0.038351636,0.020094965,-0.009871613,-0.015526852,-0.028538859,0.021673506,-0.019231217,-0.031818833,0.05654424,0.02038003,0.01848092,0.09642913,-0.027154017,-0.02496488,-0.017382978,-0.0214067,0.07949757,-0.024038373,-0.02371705,-0.00880466,0.011588749,-0.03400386,0.052816447,-0.0016342567,0.038493212,0.054507483,-0.03502474,-0.024769127,-0.0052895895,0.013027451,-0.052145142,-0.023039127,-0.0007819975,0.011850928,0.024621239,-0.042213716,-0.018747684,-0.024564067,0.030604184,-0.01755092,-0.007875456,-0.015464519,0.017732512,0.015627038,0.011490013,-0.06034577,0.048419785,0.0026638508,-0.042787243,-0.058593173,0.04808664,-0.020070912,0.05872994,0.02995886,0.012541004,-0.06400557,-0.035395093,0.024595723,-0.07727267,0.03329485,-0.045526456,-0.05904256,0.07655112,0.017225826,-0.017017132,0.013502845,0.013400756,0.022353858,-0.00945577,-0.12657379,0.010999117,0.05095449,0.037832793,0.0507706,-0.0054611624,-0.052228034,-0.027756851,-0.018774027,0.03911153,0.04017522,0.035454325,0.06668314,-0.08095489,0.04093511,-0.044520844]
6	10	[0.003171286,-0.032439113,0.04587251,-0.0065994593,0.054788217,-0.031579636,-0.020800583,0.014065775,0.03795932,-0.0015663964,0.003974207,0.006803661,0.0038325493,-0.0055590454,-0.019314455,-0.012467919,0.057268973,0.00980185,-0.07239676,0.07501173,0.041434463,-0.022324888,-0.0038809637,-0.039145492,0.001955953,0.0036607832,-0.009027506,0.0224343,-0.011331121,-0.055720508,-0.041935287,-0.0006143746,-0.009410772,-0.010974119,0.004597871,0.020089615,-0.034854334,-0.0055725714,-0.027218526,0.01860263,0.03578346,-0.03715366,-0.046806704,0.0077469745,0.013156449,0.036291804,-0.00671915,0.026651733,0.030721493,-0.03323259,0.004948444,0.017391184,-0.032614518,0.057046834,0.03104297,0.05707186,0.038961083,-0.009180841,0.016257524,-0.0051358263,-0.013958988,0.0037787291,-0.05780901,-0.004271306,0.071069814,0.00806621,-0.013442232,-0.03422631,-0.0064684013,-0.04497413,-0.029645028,-0.0009840379,-0.039705686,0.021540284,0.00726636,-0.026982706,-0.04729304,-0.1368465,0.0003942552,-0.029410085,-0.027014747,-0.01836424,-0.038423583,0.04246217,-0.0031114228,0.021619322,0.03704956,-0.011652212,0.017737294,0.004431282,0.050951436,-0.030122552,-0.009470345,0.013604001,-0.020322803,-0.051545385,0.028274773,-0.019775355,-0.0031971347,0.012384397,-0.035433814,0.036469024,-0.0030698697,-0.0016822055,0.03275399,-0.034509707,0.0040688682,-0.025049798,-0.012914924,0.017217834,0.053876657,-0.02679726,-0.07102332,0.030290205,-0.0286569,-0.04384969,0.022945529,0.00074801466,-0.01923004,0.057886768,0.014804058,0.007919575,-0.039934795,-0.011102823,0.009595597,0.025170377,0.004280353,-0.014526365,0.024940714,0.002923453,0.011610452,-0.06240035,-0.029407088,0.024516629,0.01434087,0.023060396,0.038047243,0.0254578,-0.019713366,0.028300405,0.0013795568,-0.0065526697,0.026733194,0.03776774,-0.04348425,0.03873287,-0.028837742,-0.046442464,0.0398933,0.11055274,-0.023704063,0.0005687881,-0.052162543,0.021763345,-0.027011659,0.008210944,0.0003542204,0.07471911,0.055840734,0.016043363,0.022397015,0.040694956,-0.00043069094,0.04887068,-0.03922625,-0.03327628,-0.010117443,-0.005118157,-0.03156195,-0.0075476286,-0.011063908,-0.05748702,-0.005082725,0.003845494,-0.044804294,0.06843335,0.018970048,0.03355408,-0.0050444044,0.033921182,-0.0040430096,8.4333864e-05,-0.03965895,-0.08461915,-0.01390556,0.023682086,0.036145333,0.00854649,-0.016715895,0.036838807,-0.005654262,-0.050054815,-0.012587692,0.015204682,-0.0017153888,-0.07741926,-0.028360909,0.01787882,0.015766375,0.009177788,-0.021747619,0.023724109,0.077845745,-0.0013075436,-0.0018233754,-0.03315866,-0.041788124,0.014879446,0.01175705,0.027685983,0.009412079,0.004016622,-0.009573652,0.012517904,-0.019140624,0.03460326,0.07314257,0.052040074,0.041849513,0.03710468,-0.042767763,-0.02286566,-0.0013007077,0.006699756,-0.040441185,0.03849576,0.03716019,-0.0003123872,0.0010712771,0.063525476,0.011514077,-0.018971607,0.029529272,0.04854783,-0.008053287,-0.0050757593,-0.03593015,-0.025687205,0.035519097,-0.026844373,0.0029800264,0.056669515,-0.04117813,0.016864963,-0.002950366,-0.078808755,-0.056451548,-0.026042402,0.0125934,-0.02289756,-0.02449445,-0.018287644,-0.029863436,-0.017851016,-0.007644175,0.016915435,0.05019066,-0.008777876,-0.039193213,-0.111539684,-0.033821885,0.04320885,0.0045796656,0.07257094,0.03668287,0.021086482,0.031787395,0.007219871,-0.018912511,0.0047800676,0.015690947,-0.047723576,0.074694775,0.000332654,0.013559292,0.021233512,0.047508217,0.0453964,0.0052423435,0.0025891692,-0.014220856,0.059819166,0.03656329,0.061037946,0.056039393,0.006714898,0.027200459,-0.007116854,0.006333713,0.029493975,0.041784875,0.0026883984,-0.044988103,-0.015186742,0.018566636,0.01521566,0.012479404,-0.011337981,-0.0031785236,-0.02787016,-0.078736,-0.009492663,-0.037880294,-0.041630894,-0.0015910384,-0.0712632,-0.086564966,0.08280631,-0.017708795,0.022616308,-0.016904805,-0.028183864,0.0116237765,0.012397565,0.009107455,0.013606583,0.019302955,-0.00882478,0.0061466335,-0.017953431,0.010075087,-0.04013246,-0.013538319,0.012164004,0.055035178,-0.013355555,-0.017164093,0.03216369,-0.027012257,0.036319487,-0.06764666,-0.0060493983,-0.042407025,-0.09575082,-0.08269038,-0.028500585,-0.023508588,0.008067463,0.028988464,-0.019197054,-0.0039016767,0.02735403,0.015089329,0.025821786,0.05095257,-0.0016413367,-0.056267068,-0.0398828,0.00708472,0.09605367,-0.015641643,-0.038951308,-0.0066017723,-0.05960212,0.017175691,0.07653401,0.011963237,0.0016713489,0.008341118,-0.053277384,0.019876266,0.12039656,0.026109492,-0.023680951,-0.026991338,-0.05137337,-0.032798093,0.03065618,0.0019997058,0.0026851029,-0.039639603,0.047383863,0.019531079,-0.06639158,0.03607499,-0.043034818,0.056438334,-0.0012070077,0.017802259,0.01391416,-0.027651183,-0.0013388195,-0.017393116,-0.015824297,0.027123824,-0.044476394,0.015825773,0.055974428,0.034843516,-0.0055324435,0.04414869,0.019108212,-0.026527453,-0.024034178,0.017122446,0.04821897,-0.059848383,-0.07845907,-0.0250321,0.055929918,0.016880823,0.05597982,-0.026665308,0.0072132684,-0.02953267,0.033917848,0.019641379,0.032382295,-0.055792257,0.045892578,-0.007509757,-0.08049377,0.035910018,-0.032738414,0.0031454107,-0.009170559,-0.024321323,-0.030831397,-0.058482256,0.037405424,-0.03585478,0.0004013482,-0.010961623,-0.06443927,0.034274533,-0.0020353948,-0.030048374,0.026321247,0.02907283,-0.057207197,0.008999521,-0.030706177,0.026481168,-0.026662664,-0.04142598,0.033615917,-0.0009454433,-0.05176305,0.05547171,0.061261274,0.064167686,-0.037935443,-0.01407785,-0.006347599,0.021290852,-0.049049698,-0.014685368,-0.0028894981,-0.038352277,-0.025114538,-0.010337496,0.03145356,0.041194633,-0.08048852,-0.012052324,0.06786247,-0.0030573735,-0.035901945,0.005013431,-0.04864368,-0.013163319,-0.053290218,0.039235078,0.021067949,0.0047933087,0.017145487,-0.030398672,0.029859113,-0.030892784,0.04242988,-0.0067338888,0.020250041,0.027865112,0.0065489253,0.019305779,-0.025993971,0.047834646,0.038136087,0.050324798,-0.0014871417,-0.004558695,0.021136967,0.028451974,0.005863018,-0.039786562,-0.025946764,-0.0055706194,-0.020227114,-0.023128321,-0.0065492913,0.011464743,-0.05869479,-0.009969954,0.049123146,-0.005414633,-0.008175758,0.018223511,0.010720856,-0.06104092,-0.035676435,0.005939218,-0.01811697,-0.023118265,0.013229801,0.032596044,-0.006991462,-0.027442517,-0.0008557786,0.01164011,-0.0038969712,-0.009733976,-0.007136334,-0.03865924,-0.03511966,-0.031980637,-0.01998437,-0.07149005,0.05871396,-0.019972332,0.00070434343,-0.05574108,0.019585151,-0.02633556,0.016666796,-0.02232877,-0.05483184,-0.028849265,-0.0026324855,0.039304756,0.04979168,-0.063179515,0.04358278,-0.018601537,0.0043156794,-0.006104057,-0.003489606,0.00428222,-0.017307375,-0.03251315,-0.009443992,0.056392614,0.037997004,0.015806016,-0.0075961375,0.0023064495,0.06374064,0.027565503,0.014329862,0.017913241,0.029576225,0.06413935,0.103270985,-0.024946656,0.04770855,0.01269316,-0.0005648472,-0.015508748,-0.055629924,-0.03033035,0.030441416,0.0516324,0.013843795,-0.09757065,0.012320188,-0.010065233,-0.015420058,-0.013460399,-0.037596982,0.010118817,-0.0006377526,0.062362745,0.007415247,0.01059072,-0.04009412,-0.051563326,0.008448706,0.038148988,0.08256361,0.008263797,0.041278876,-0.045600567,-0.024385469,0.0915843,-0.0001314885,0.01828374,0.0013341368,0.017182337,0.020345729,0.19308648,-0.039943155,0.024616906,0.047636822,-0.047397416,-0.033825036,-0.02067412,-0.024780754,-0.055543467,-0.008774934,0.036729224,0.0075767045,-0.023335094,-0.023841077,0.062901355,0.050352123,-0.038948484,-0.02988381,-0.0010089636,-0.018295854,-0.021783426,-0.011035166,0.0030947756,0.038769256,0.012634931,0.032626674,0.051509753,-0.05706813,0.017921615,0.006247399,-0.029999293,0.026918288,-0.021917328,0.022819871,-0.044447143,0.0348257,0.036559235,0.018735807,0.01787591,0.06339169,0.03451974,-0.013807962,0.043554008,0.040124606,-0.037659843,-0.0005781556,-0.042723652,0.042850066,0.039363563,0.034048356,0.017296169,-0.03075744,-0.007656672,0.06659086,0.0086740665,0.008669402,-0.013124263,0.0050915284,0.015220815,0.029267471,0.0071264766,0.032033574,-0.0097014345,0.016073147,-0.026981935,-0.0076746186,0.043585498,0.041384503,0.08133476,0.054682948,-0.033768203,0.008092524,0.031371377,-0.07262764,-0.027549483,-0.031571683,-0.07841167,-0.028849535,-0.013681625,-0.0127926245,0.0045607667,-0.016105797,-0.03740289,0.051392395,0.018713491,0.008967115,0.035711862,0.07028562,0.0706177,-0.07387048,-0.0069094454,-0.05291457,-0.04108003,-0.023322083,-0.02397254,0.002382238,0.06161575,0.015175221,0.015200627,0.03729838,0.02691845,-0.053385653,-0.022712722,0.028381117,-0.009215971,0.016583737,-0.021076417,-0.06347747,0.048120912,-0.014440765,-0.011108008,-0.015122424,0.057241563,-0.05951795,0.008362887,-0.038936254,-0.010548456,0.01146168,0.0007313277,0.030727064,0.01964422,0.046478134,-0.003024896,0.008480463,-0.0111181205,-0.032037795,0.0032747036,-0.026911868,0.004694793,0.0071573467,0.004633507,0.0071024885,0.029880473,-0.00819163,0.010300299,-0.062471412,0.03102781,0.0058649867,0.021866769,0.0108441515,0.05694147,-0.0015605123,0.021535955,0.040849492,0.027708624,0.023993794,-0.028194051,-0.023896342,0.035134677,-0.024617633,-0.07150203,-0.005215245,0.010804056,-0.027128728,-0.03781473,0.0133085,0.021383965,-0.049862243,-0.013408525,-0.021798583,-0.010555183,0.12981759,0.011827058,0.053303596,0.017455233,-0.03924528,-0.0056654923,0.0045397584,-0.0159785,0.033892713,0.017301429,0.008940636,0.021751612,0.028680945,-0.02169074,-0.0029292,0.057513952,0.006487085,-0.0463812,0.04296013]
7	9	[-0.010980865,0.0029302945,-0.013747488,0.028561268,-0.045052227,0.019431883,0.035298806,0.020389274,0.0019885239,0.0054986933,-0.009330394,0.01569685,-0.011077741,0.062629975,-0.0022578246,0.060248476,0.08129331,-0.025166713,0.022582889,0.057710975,-0.006132168,0.020350788,0.05829524,0.034860592,-0.0008795065,-0.0053145345,0.07862342,0.0670171,-0.033060446,-0.07981519,-0.042445414,0.018030377,0.021251706,0.027489178,-0.045234494,0.00729832,0.0026201552,-0.041429013,0.0053452966,0.07220116,-0.095698275,0.0056345784,-0.034459926,0.028122662,0.018580573,0.024756622,-0.09727075,0.0029012908,-0.008339401,0.012811472,0.0050056744,0.043683004,0.0076632034,0.024311898,-0.0071517965,0.015290457,0.041719396,0.047975916,0.015527143,-0.07201843,-0.032593023,-0.10437547,-0.01499205,0.025368568,-0.01583742,0.03189329,-0.044445243,-0.075462945,-0.020161163,-0.013679325,0.01864025,0.012666123,0.049272686,-0.054352824,-0.053369593,0.01412679,-0.01337053,0.028756572,0.010001549,-0.012927298,0.030506505,0.02398227,0.0055230195,0.066431984,-0.018964332,-0.02326698,-0.047736675,-0.059982717,-5.705469e-05,-0.022750277,0.006505004,0.029097883,0.0041736644,0.06331667,-0.02399987,-0.023124404,-0.006186174,-0.051944546,-0.008847062,0.03546947,0.040074587,0.041360863,-0.062362302,0.0070096995,0.028630793,0.052884027,-0.03491017,0.015423716,0.05887086,-0.010091097,0.036359783,-0.031814296,-0.07651439,-0.019595547,0.04016974,-0.061788246,0.0074537606,0.0096744895,-0.10180465,0.006079477,-0.01654714,-0.039101813,-0.00066607544,0.029711293,0.0041146516,-0.026943112,-0.03614832,-0.025281569,0.006544168,-0.007591983,-0.08860029,0.029750964,0.023213616,-0.008989137,-0.03612721,-0.02682011,0.026378496,-0.0038149473,0.050584435,-0.021502875,-0.028603252,0.039006352,-0.006610032,0.06846465,0.005041798,-0.009152744,-0.020176344,0.0014777358,0.008098288,0.04957976,-0.06357107,0.061205655,-0.011516385,-0.050460048,-0.034497164,-0.0035031817,-0.019628081,0.054253943,0.015257703,-0.0043725087,0.019750984,-0.016320076,-0.0636539,0.009983775,0.006557006,0.0052747834,0.027781993,-0.020354437,0.009329815,0.05028534,0.0018486325,-0.022645975,-0.044108756,0.006252675,-0.020170722,0.038555156,0.006832739,0.084550366,-0.01061674,0.014411082,0.020274274,0.0047669695,-0.0090776915,-0.0476132,0.0304511,-0.0676793,0.04726203,0.062426727,0.0059828092,0.057656575,0.019739253,-0.064361624,0.026707202,-0.022490125,0.048613485,0.017872497,-0.013335719,0.009057537,-0.025134744,-0.0052722143,-0.030353934,-0.048580635,0.02057904,0.0686058,-0.03474729,0.015278624,-0.061989743,0.0054401066,0.060174033,-0.01154132,-0.0013578703,-0.02441017,0.034150433,0.053131822,-0.051805343,-0.012151762,-0.04058865,0.039695695,0.11294294,-0.0049868794,0.0116097145,0.01374658,0.037044004,0.049035523,0.006365854,-3.158827e-05,0.00846379,-0.04897565,-0.029488057,-0.024155555,0.009294045,-0.020675251,-0.030978354,0.0020779218,-0.014682038,0.021397354,-0.056173988,0.045663163,0.015623006,-0.052647825,-0.0060938527,0.00891614,-0.02639622,0.0073951497,-0.043007683,-0.029092142,-0.055382993,0.048206326,-0.028077759,-0.047516443,-0.02212057,0.0018352254,-0.032006975,0.02444275,0.004417951,0.002635519,-0.029771255,-0.009846935,-0.08590755,-0.050746594,-0.047156535,-0.0251801,0.08301791,0.035159722,0.03548599,0.004778565,0.014325628,-0.029561328,0.010435048,-0.020826725,-0.020030197,0.0065036146,0.0035921761,-0.060459,0.04374945,0.014399972,0.05836712,0.030435855,-0.020279353,-0.01849378,-0.005108835,-0.03965255,-0.017132241,0.03490815,0.04310443,0.099349424,0.022919921,-0.060352232,-0.028031763,0.013667325,0.038610734,0.02691187,-0.026569607,0.037987962,0.013439859,0.020173403,0.05917244,-0.04450236,-0.026166543,-0.003256881,0.04757817,-0.02182651,-0.05747041,-0.038493462,-0.010028747,0.034437813,-0.081209294,-0.019618697,-0.06646202,-0.040036604,0.008706612,-0.012694269,-0.0072513632,0.018767664,0.02144865,0.09255964,-0.02043549,0.03334381,-0.009251112,-0.056041047,-0.031441502,0.027332751,0.038806938,0.015415361,0.00271943,-0.016528305,-0.06636519,0.021114074,0.04269847,0.022318343,-0.017062373,0.06855722,-0.013709531,-0.026664672,0.023805821,-0.0002472758,0.04410154,-0.024424754,-0.08133243,0.0397429,-0.021016555,0.029770354,0.008116515,0.02205284,0.043754455,-0.03704677,-0.036789495,0.06736645,0.027610686,-0.010410518,-0.039717853,-0.075403444,0.036242053,0.0056385603,-0.013343717,0.022479387,-0.019380037,0.0071988134,-0.042380214,0.009613931,0.010872725,0.0070590316,0.012586964,0.030504989,-0.033241414,-0.07835037,-0.07303786,0.006798364,-0.039946653,-0.09870574,-0.017555557,0.01874824,-0.041849863,0.023929646,0.016641052,0.0022890572,0.03651037,0.03277196,-0.0036137493,0.052849475,-0.02141437,0.02627333,-0.01176614,-0.0010543505,-0.013434841,0.005203721,-0.0070924447,0.03986654,0.012849885,-0.0057597454,0.03777507,0.007977415,-0.0043801996,-0.05409284,0.006289188,0.008147643,-0.035080783,-0.07016909,-0.0008541191,-0.015826136,0.030563828,0.039115116,-0.024829686,0.01596666,-0.005697379,-0.03245444,0.026714599,0.023405524,-0.045657903,0.01341916,0.032693934,0.032137875,-0.0015423895,0.029758062,0.048896186,0.0027369882,-0.03481039,0.053073652,0.01675328,0.013239734,-0.04260922,-0.029824013,0.014592167,-0.008141596,0.0075712,0.008513436,0.03987906,0.06545962,0.028738331,-0.014116212,0.007257488,0.069952466,-0.0600778,0.039585963,-0.029473754,0.020616347,-0.029631209,0.015170309,-0.0032006132,0.018439597,0.06586169,0.032944687,-0.0073541957,-0.043102566,0.0101756845,-0.015047573,-0.009856654,0.036669813,-0.028511975,-0.048572086,0.035804056,-0.08224941,-0.060257792,-0.03797528,-0.07960883,0.019188745,-0.008593016,-0.034994263,0.040299285,-0.013611892,0.012869581,-0.016215308,-0.03495196,-0.025240496,0.03138456,-0.004088759,-0.022911321,0.07591285,-0.07156624,-0.01966051,0.016971903,0.034443088,0.00079599663,-0.010192039,0.045042787,0.024288813,0.027255623,-0.009363405,-0.010788922,-0.015407268,-0.009623025,0.005820971,0.011159289,-0.024075575,0.0052362457,-0.00038510922,0.05918827,0.023695191,-0.006162975,0.016824882,4.897707e-06,0.022132367,0.031789888,0.015504613,-0.006553786,0.009501468,0.008431685,0.0070147575,-0.043732364,-0.013527009,-0.0012235363,0.0087559465,-0.07013536,0.041054014,0.014429209,0.0764235,0.0040318538,0.0028196329,0.0063508,0.0066857445,0.0056311944,0.04402164,-0.03595688,-0.007191786,-0.008053694,-0.008660588,-0.05408857,0.023430998,0.054455988,0.036281884,0.04014089,0.0013087797,0.038196314,-0.0032946693,0.022574488,0.020942729,-0.03611263,0.013686445,0.009345536,0.027916497,0.008902703,0.06370102,0.0007010278,0.024627848,0.02443488,-0.011171011,-0.024273098,0.015899254,0.022618337,-0.02783722,0.029088322,0.019284245,0.019610047,0.013033817,0.011015722,0.04565462,0.08008016,0.020675989,-0.019941684,0.025676508,0.009091368,-0.06747299,0.007948198,0.046248425,-0.0052090646,0.0057090404,0.03839929,0.038422633,-0.014130464,0.018636081,0.010746865,-0.01020858,-0.015259573,0.051463977,-0.0040960726,0.03770056,0.023215536,-0.008933082,0.067660905,0.07685456,0.05763144,-0.043516632,-0.061383545,-0.07040634,-0.020888649,-0.014310664,0.030001711,0.030192034,-0.06415559,-0.0732369,-0.025315678,0.0537299,0.033663172,0.04974196,0.042800684,-0.037919473,0.02162752,0.04687242,0.11869635,-0.018011346,-0.013547658,0.01975795,-0.0041904836,0.041877635,-0.011292994,-0.015255844,0.022077989,-0.0066248635,-0.013412951,-0.025844404,-0.029778926,0.040633272,0.024941854,0.028991954,-0.03646226,0.050146848,0.0039089285,-0.042095236,-0.041783746,-0.008177754,-0.019062461,0.048071995,-0.006350305,0.034403693,0.034096118,-0.039427433,0.004472535,0.013923726,-0.047608852,-0.053532723,0.015104811,-0.026798654,-0.10897776,-0.1082948,-0.050839484,-0.033996005,0.018142255,0.029030131,-0.010102286,0.014265549,-0.04492683,-0.039870583,-0.0034108628,-0.026424602,-0.012183464,0.001057611,-0.045396324,-0.004170664,-0.005783691,0.06626416,0.012462831,-0.026701916,-0.012865758,-0.0012616967,0.0006762619,-0.00912782,0.0122340135,0.010236957,-0.01826111,-0.003616675,0.004324348,-0.014015819,0.05845296,-0.053001173,0.060130205,-0.0078124655,0.05404306,-0.017020963,-0.04348471,-0.019973397,0.021918016,-0.011768181,0.024265347,-0.087502964,-0.04167642,0.03511506,-0.043016553,-0.015029159,0.012582649,-0.002739447,-0.024635956,0.016843239,0.0075948336,0.048614264,-0.08598973,0.03757243,0.0053182575,0.0020285991,-0.0060179094,0.030231085,-0.00902786,-0.026623731,-0.012581822,-0.004011081,0.022911448,-0.010969786,0.058042523,0.0297597,0.025447065,0.00042080978,0.018837497,0.0060849697,-0.0030941782,-0.028142607,0.009987745,-0.035802558,-0.041322216,-0.039123412,-0.0028751122,0.046513934,0.013266977,0.08123814,0.0025142282,-0.0043295864,-0.023546,-0.00074563466,-0.017311767,-0.025678096,0.04063925,0.022590924,0.017418813,-0.002516213,-0.011660808,0.035042375,-0.039019644,0.016298909,0.006243185,0.041158546,0.03399771,0.039782047,0.047504082,0.0013467842,0.024050694,-0.016339457,0.011291448,-0.005079332,-0.0012350384,-0.023734327,-0.1202985,0.009119666,0.031539824,-0.024752699,0.058656666,0.05408638,0.009053684,-0.03520713,0.051695794,0.0014425752,-0.047746185,-0.016452245,0.011323866,-0.025610028,0.0045855204,0.009718196,0.01900502,-0.008817995,-0.006942818,-0.003377717,0.013413063,-0.047806855,-0.035554867,-0.033057272,0.06120632,-0.021034561,0.016035385,-0.00015244471,0.03990452,-0.00830541,0.008661377,0.05913079,-0.0039443383,0.0037082853,-0.056827053,0.0043010167,0.06783147,0.042799033,-0.001220756,0.0011633768]
8	16	[-0.00827995,-0.034610346,0.01106324,0.0073036947,0.029679947,-0.027820121,-0.015069431,0.009634274,0.03759529,-0.014114078,-0.035748165,-0.010026761,0.016770149,-0.022198932,-0.037121136,0.02655445,0.04854312,-0.026709199,-0.023663094,0.02688226,-0.024427077,-0.07796299,0.036392555,-0.035210624,0.0074665877,0.011281199,0.023635251,0.04222603,-0.04991115,-0.028519694,-0.0011149712,0.0024436323,0.024942003,-0.019811744,0.0005120754,0.007137555,0.0013354748,-0.014848021,0.027159516,-0.03993776,0.045829512,0.03893079,-0.0056470986,0.032938164,0.04211253,0.0012740114,-0.021305874,0.06129706,-0.037813526,0.006378466,-0.010601217,0.034599148,-0.0010874125,0.006864242,0.03954139,-0.031165814,0.06401004,-0.034431767,0.025631305,-0.032701515,-0.015373929,0.021866351,-0.029281128,-0.01927148,0.0036866996,0.013941444,-0.015425343,-0.060493827,-0.054906838,-0.0126738325,-0.050000414,-0.0061559253,0.0007779732,0.02943274,-0.0105557125,0.020577619,0.0033707982,-0.2104699,-0.017701115,-0.0125993,-0.006871477,0.008968311,-0.053714514,0.04202537,0.0030641945,0.012137319,0.02080651,0.081897624,-0.009096496,-0.044440288,-0.027405562,0.056165308,-0.013444158,-0.0003753559,0.006158644,-0.012929331,0.006047177,-0.052570872,-0.015665857,0.055241775,-0.022404538,0.014894535,0.0061194473,0.003218156,0.007100777,-0.046343904,0.034508973,0.020275176,-0.034488775,-0.007228811,0.029270276,0.0089491205,0.023261735,0.008710898,-0.027189218,0.0147626605,-0.026057698,-0.013510373,0.02989671,0.07232126,-0.034717515,0.018105118,0.027792128,-0.06777407,0.024873342,0.028931571,-0.03702897,0.032437004,0.029567001,0.017694259,-0.046798974,-0.03566073,0.0019147235,0.024232931,-0.0030183892,-0.0043708007,0.034111142,0.046964668,-0.037159406,-0.004270013,-0.048741486,-0.02715091,-0.003152934,-0.017750427,-0.09206444,-0.025170946,-0.0066599404,-0.0486102,-0.02233373,0.044909712,-0.024795683,0.0022987432,-0.032747377,-0.009500912,-0.0054467833,0.014233432,0.014796228,0.041971162,0.009674836,0.0068762405,-0.05106069,0.0063221087,-0.013697886,0.032997888,-0.044499688,0.0040169237,0.02193903,0.010165631,-0.044274468,0.012385289,0.0127277495,-0.056339256,-0.0034199515,0.04973069,-0.031585123,0.015182804,-0.030020392,0.04568061,-0.040623013,0.02479172,0.05113004,-0.008265145,-0.020681662,-0.031700827,0.029340707,-0.02952118,0.010362991,-0.05136367,-0.008139121,-0.017102694,0.017072657,0.018788476,-0.01683912,0.0140084205,0.028484084,-0.10554249,-0.011698896,0.0085175745,0.06617205,-0.030734183,-0.0014133325,-0.07971434,0.029470162,0.012098536,0.029605702,-0.02010599,-0.075516485,-0.03593498,0.0314209,0.030848848,0.0361113,-0.040077496,0.014975683,0.040327672,-0.042048395,-0.0014560517,0.12463245,0.050239082,0.0748412,0.016382284,-0.017137963,-0.026355779,0.059725445,0.038400013,-0.035774652,0.0020398502,0.027879726,-0.0030101314,-0.0105090365,0.04419088,0.03237989,-0.0022990545,0.09698304,0.07234383,-0.021649182,0.016740067,-0.004373622,-0.023684636,0.06973893,0.014015923,0.09117332,0.035076704,-0.031005844,0.034486715,-0.007943956,-0.090414144,-0.028027123,-0.05438641,0.009625402,-0.012782912,0.049976524,0.00373687,-0.004570815,0.0090639535,-0.03705787,0.037023157,0.023593744,0.041539982,-0.043482475,-0.09469259,-0.043471493,0.009000916,-0.0037937555,0.017373823,-0.0036268358,0.010748073,-0.020986544,0.016643818,0.009214289,0.007001612,-0.026705476,0.012909404,0.048909057,-0.019775886,-0.027984852,0.02812908,0.068808526,0.03378029,0.06489579,0.06723202,-0.02213781,0.02787296,0.013669409,0.025833834,-0.015577676,0.030939978,0.09667319,-0.008746305,0.019280575,0.0022844777,0.010486222,0.010499283,-0.02453907,0.026019044,-0.006327684,0.014136153,0.00043331026,0.031185223,0.0040765177,-0.027469805,-0.035681102,0.0013145858,0.029240383,0.029562538,-0.04932975,-0.028113859,-0.053180594,0.06470109,-0.016735058,0.014846862,-0.010840628,-0.0015431591,0.045138087,-0.023328759,0.010086982,0.06820912,-0.02390722,0.020182425,0.021722475,-0.01866628,-0.021079678,-0.034124542,0.001366074,0.030126939,-0.005383268,-0.030479766,0.008044614,0.074610725,0.06786382,-0.018692913,0.058386635,-0.00015586878,0.046492394,-0.054368887,-0.018598123,-0.02712821,-0.04223644,0.033401217,0.044429652,-0.0258572,0.016316641,0.00022469161,-0.0043978402,4.700899e-06,0.04118016,-0.03584669,-0.007252465,-0.025164586,-0.025883196,0.07784902,-0.066638365,-0.0039694184,0.0021339564,-0.07167551,-0.031893324,0.03678021,-0.045494635,0.045835044,-0.0042433725,0.03247294,0.006898415,0.054094706,-0.010497484,-0.02940786,-0.04429659,-0.013431852,-0.019930663,0.044309225,-0.013714081,-0.04627294,0.019896423,0.04101131,0.030311326,-0.05047937,0.008220372,0.026125995,0.03281195,-0.013263692,-0.048791654,0.0011422944,-0.021484377,0.062461693,0.0058463374,0.00756037,0.013513509,0.0019458506,0.03693832,-0.03288721,0.022605436,-0.06556102,0.004576696,0.03427221,-0.04133871,0.025028221,0.011008754,0.017926134,-0.008345028,-0.118678875,-0.015336977,0.03075929,-0.0011377166,0.049308375,-0.01688922,0.017686987,0.012930502,0.011575054,0.06703382,-0.00798882,-0.039335698,0.024447132,0.056489173,0.050662175,0.018598491,0.006838836,0.014386298,-0.008989679,0.037265465,-0.05342417,-0.018484335,0.054196198,-0.004013233,-0.011419861,0.049231812,0.0022520702,0.068639785,-0.0042963275,-0.012573259,-0.0058974638,-0.023031248,-0.055480827,0.014864335,-0.0010021435,0.046298545,-0.017711403,-0.04946004,0.030342486,-0.003019003,-0.025811622,0.045795433,0.03950935,0.05481118,-0.016114937,-0.021108627,-0.008210227,-0.052673023,-0.048595503,0.05201639,-0.006737158,-0.0639483,-0.026666366,0.030232681,0.0055994927,0.02648724,-0.109706946,-0.035788067,0.012392373,-0.012181291,-0.030510781,-0.01844907,-0.028238282,-0.0067627295,-0.043266192,0.017876832,0.015987188,-0.011994735,0.018701782,-0.011928969,0.04492222,0.013237805,0.025792312,-0.008688748,-0.040842433,0.018082742,-0.023771577,0.013295947,-0.0018641034,-0.010716199,0.033005822,0.043519825,-0.026981864,0.021024607,0.031196801,-0.0049111196,0.04178349,-0.042851504,-0.022797452,-0.014860918,0.05344156,0.017042227,0.018457575,-0.0036325231,-0.055252895,0.0038026476,0.029270088,-0.018991482,0.006093374,-0.01876196,-0.017646782,0.01273931,0.022067543,0.01790926,0.003308826,-0.062299497,0.018495256,0.008459034,-0.027687753,-0.022754967,-0.014361067,-0.011027836,0.05590415,-0.06992134,-0.021278935,0.037294544,-0.040982038,-0.027292598,0.019190127,-0.024626236,0.055991624,-0.034910265,0.05088938,-0.053240146,0.03495691,-0.0037107065,-0.0055904873,0.010853273,-0.035187397,-0.060551334,-0.00036187537,0.008492531,0.02791281,-0.009816768,0.029029295,-0.022480112,0.023187375,-0.016530368,-0.006021706,0.019293873,-0.017321073,0.039871022,0.02324111,0.04951193,-0.052875366,-0.002499191,0.027512847,-0.05917426,0.06391969,-0.022287047,-0.011694384,-0.0007674924,0.013700196,0.07914599,0.083587244,-0.04235003,0.017531473,0.039258942,0.0031541584,-0.023269571,-0.019378403,0.016042335,-0.016401356,0.0768848,0.030234106,-0.03778562,0.028921638,-0.003351391,-0.03418286,0.015047688,-0.013182957,0.025809038,0.02254305,0.037209,-0.051980477,-0.022338562,-0.0077382843,-0.042640537,-0.012446553,0.031476077,0.047519736,0.030540438,-0.023306658,-0.013748036,0.012875609,0.07866162,0.04812159,0.043466106,0.0029119193,-0.009331066,0.016297588,0.15520363,0.0040261946,-0.004722769,0.012324494,0.0014958374,0.050956722,-0.0030968003,-0.061052367,-0.00042327557,0.025133291,-0.015196411,-0.06124915,-0.01283789,-0.07064649,0.032423545,0.0048045637,-0.020574605,-0.034371577,-0.017621385,0.0031822599,-0.0044048643,-0.054629643,0.037418798,0.015052654,-0.0384148,0.049613602,0.055699322,-0.050723877,-0.008371216,-0.00711551,-0.009918433,0.04029083,-0.0043397243,-0.061374128,-0.043718934,0.0075903726,-0.017625531,-0.0057659387,0.0084339725,0.054231003,0.01785023,0.0036024393,0.007895175,0.0053510983,-0.04374418,-0.035288427,-0.0026184237,0.02473439,0.016075745,-0.045384243,-0.00863723,-0.011166395,-0.039288733,-0.018126624,0.009142497,0.0067758886,0.034749974,0.021366725,0.021588305,-0.037197445,0.011672907,0.024563467,0.008972169,-0.021392092,0.09566175,-0.029745756,-0.010988343,0.02002131,0.027421586,0.07627296,0.019523902,-0.01481026,0.011902693,0.013823988,-0.1170023,-0.03144113,0.018995717,0.021198357,-0.0018829814,-0.05692905,0.016190875,-0.0064036753,-0.012021276,0.079995774,0.061117426,0.022860482,0.028395534,0.048640553,-0.074574776,0.0121098105,0.019991145,0.014018712,-0.013552512,-0.0039388104,-0.027896738,-0.045190293,0.059623413,-0.016356686,0.05907746,0.015400985,0.018802905,-0.03177482,0.015652457,0.062358506,0.0026949605,0.013295303,0.0072113588,-0.08281002,0.057143405,0.00056008296,-0.045002397,0.011848143,0.03133717,-0.072143935,-0.02972954,0.020925961,0.0088149225,0.047224917,0.0030262063,0.028099053,-0.0072229537,-0.008149978,-0.016028985,0.03262075,0.02239823,-0.030708214,-0.011682246,-0.030691257,0.027514521,0.044814106,-0.04944084,0.041368995,0.034623697,0.05637266,-0.012394835,-0.02806277,0.096631765,-0.013708178,0.035070464,-0.032672442,-0.039062664,0.020557923,0.03719434,0.0053781723,-0.015572469,0.012257215,-0.03743663,0.09847694,-0.00033250116,-0.0014403812,-0.010142688,0.015317654,-0.017536964,-0.0034370418,0.018794456,0.019422732,0.019417427,-0.031909116,0.008106998,0.003075916,-0.029449765,-0.022718776,-0.0036204206,0.06946037,-0.045375396,0.015125693,-0.013684386,-0.014939551,0.045990087,0.015256561,-0.0033984815,-0.03717554,0.007539222,0.019926757,-0.031972192,-0.017704261,0.026187802,-0.07111723,-0.00040031513,-0.03852643]
\.


--
-- Name: embeddings_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.embeddings_id_seq', 1, false);


--
-- Name: enrichment_associations_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.enrichment_associations_id_seq', 27, true);


--
-- Name: enrichments_v2_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.enrichments_v2_id_seq', 19, true);


--
-- Name: git_repos_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.git_repos_id_seq', 1, true);


--
-- Name: tasks_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.tasks_id_seq', 16, true);


--
-- Name: vectorchord_bm25_documents_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.vectorchord_bm25_documents_id_seq', 8, true);


--
-- Name: vectorchord_code_embeddings_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.vectorchord_code_embeddings_id_seq', 8, true);


--
-- Name: vectorchord_text_embeddings_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.vectorchord_text_embeddings_id_seq', 8, true);


--
-- Name: alembic_version alembic_version_pkc; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.alembic_version
    ADD CONSTRAINT alembic_version_pkc PRIMARY KEY (version_num);


--
-- Name: embeddings embeddings_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.embeddings
    ADD CONSTRAINT embeddings_pkey PRIMARY KEY (id);


--
-- Name: enrichment_associations enrichment_associations_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.enrichment_associations
    ADD CONSTRAINT enrichment_associations_pkey PRIMARY KEY (id);


--
-- Name: enrichments_v2 enrichments_v2_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.enrichments_v2
    ADD CONSTRAINT enrichments_v2_pkey PRIMARY KEY (id);


--
-- Name: git_commits git_commits_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.git_commits
    ADD CONSTRAINT git_commits_pkey PRIMARY KEY (commit_sha);


--
-- Name: git_repos git_repos_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.git_repos
    ADD CONSTRAINT git_repos_pkey PRIMARY KEY (id);


--
-- Name: commit_indexes pk_commit_indexes; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.commit_indexes
    ADD CONSTRAINT pk_commit_indexes PRIMARY KEY (commit_sha);


--
-- Name: task_status task_status_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.task_status
    ADD CONSTRAINT task_status_pkey PRIMARY KEY (id);


--
-- Name: tasks tasks_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tasks
    ADD CONSTRAINT tasks_pkey PRIMARY KEY (id);


--
-- Name: git_commit_files uix_commit_file; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.git_commit_files
    ADD CONSTRAINT uix_commit_file PRIMARY KEY (commit_sha, path);


--
-- Name: enrichment_associations uix_entity_enrichment; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.enrichment_associations
    ADD CONSTRAINT uix_entity_enrichment UNIQUE (entity_type, entity_id, enrichment_id);


--
-- Name: git_branches uix_repo_branch; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.git_branches
    ADD CONSTRAINT uix_repo_branch PRIMARY KEY (repo_id, name);


--
-- Name: git_tags uix_repo_tag; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.git_tags
    ADD CONSTRAINT uix_repo_tag PRIMARY KEY (repo_id, name);


--
-- Name: vectorchord_bm25_documents vectorchord_bm25_documents_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.vectorchord_bm25_documents
    ADD CONSTRAINT vectorchord_bm25_documents_pkey PRIMARY KEY (id);


--
-- Name: vectorchord_bm25_documents vectorchord_bm25_documents_snippet_id_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.vectorchord_bm25_documents
    ADD CONSTRAINT vectorchord_bm25_documents_snippet_id_key UNIQUE (snippet_id);


--
-- Name: vectorchord_code_embeddings vectorchord_code_embeddings_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.vectorchord_code_embeddings
    ADD CONSTRAINT vectorchord_code_embeddings_pkey PRIMARY KEY (id);


--
-- Name: vectorchord_code_embeddings vectorchord_code_embeddings_snippet_id_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.vectorchord_code_embeddings
    ADD CONSTRAINT vectorchord_code_embeddings_snippet_id_key UNIQUE (snippet_id);


--
-- Name: vectorchord_text_embeddings vectorchord_text_embeddings_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.vectorchord_text_embeddings
    ADD CONSTRAINT vectorchord_text_embeddings_pkey PRIMARY KEY (id);


--
-- Name: vectorchord_text_embeddings vectorchord_text_embeddings_snippet_id_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.vectorchord_text_embeddings
    ADD CONSTRAINT vectorchord_text_embeddings_snippet_id_key UNIQUE (snippet_id);


--
-- Name: idx_entity_lookup; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_entity_lookup ON public.enrichment_associations USING btree (entity_type, entity_id);


--
-- Name: idx_type_subtype; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_type_subtype ON public.enrichments_v2 USING btree (type, subtype);


--
-- Name: ix_commit_indexes_status; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_commit_indexes_status ON public.commit_indexes USING btree (status);


--
-- Name: ix_embeddings_snippet_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_embeddings_snippet_id ON public.embeddings USING btree (snippet_id);


--
-- Name: ix_embeddings_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_embeddings_type ON public.embeddings USING btree (type);


--
-- Name: ix_enrichment_associations_enrichment_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_enrichment_associations_enrichment_id ON public.enrichment_associations USING btree (enrichment_id);


--
-- Name: ix_enrichment_associations_entity_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_enrichment_associations_entity_id ON public.enrichment_associations USING btree (entity_id);


--
-- Name: ix_enrichment_associations_entity_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_enrichment_associations_entity_type ON public.enrichment_associations USING btree (entity_type);


--
-- Name: ix_enrichments_v2_subtype; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_enrichments_v2_subtype ON public.enrichments_v2 USING btree (subtype);


--
-- Name: ix_enrichments_v2_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_enrichments_v2_type ON public.enrichments_v2 USING btree (type);


--
-- Name: ix_git_branches_head_commit_sha; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_git_branches_head_commit_sha ON public.git_branches USING btree (head_commit_sha);


--
-- Name: ix_git_branches_name; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_git_branches_name ON public.git_branches USING btree (name);


--
-- Name: ix_git_branches_repo_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_git_branches_repo_id ON public.git_branches USING btree (repo_id);


--
-- Name: ix_git_commit_files_blob_sha; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_git_commit_files_blob_sha ON public.git_commit_files USING btree (blob_sha);


--
-- Name: ix_git_commit_files_extension; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_git_commit_files_extension ON public.git_commit_files USING btree (extension);


--
-- Name: ix_git_commit_files_mime_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_git_commit_files_mime_type ON public.git_commit_files USING btree (mime_type);


--
-- Name: ix_git_commits_author; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_git_commits_author ON public.git_commits USING btree (author);


--
-- Name: ix_git_commits_parent_commit_sha; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_git_commits_parent_commit_sha ON public.git_commits USING btree (parent_commit_sha);


--
-- Name: ix_git_commits_repo_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_git_commits_repo_id ON public.git_commits USING btree (repo_id);


--
-- Name: ix_git_repos_sanitized_remote_uri; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX ix_git_repos_sanitized_remote_uri ON public.git_repos USING btree (sanitized_remote_uri);


--
-- Name: ix_git_repos_tracking_name; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_git_repos_tracking_name ON public.git_repos USING btree (tracking_name);


--
-- Name: ix_git_repos_tracking_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_git_repos_tracking_type ON public.git_repos USING btree (tracking_type);


--
-- Name: ix_git_tags_name; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_git_tags_name ON public.git_tags USING btree (name);


--
-- Name: ix_git_tags_repo_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_git_tags_repo_id ON public.git_tags USING btree (repo_id);


--
-- Name: ix_git_tags_target_commit_sha; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_git_tags_target_commit_sha ON public.git_tags USING btree (target_commit_sha);


--
-- Name: ix_task_status_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_task_status_id ON public.task_status USING btree (id);


--
-- Name: ix_task_status_operation; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_task_status_operation ON public.task_status USING btree (operation);


--
-- Name: ix_task_status_parent; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_task_status_parent ON public.task_status USING btree (parent);


--
-- Name: ix_task_status_trackable_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_task_status_trackable_id ON public.task_status USING btree (trackable_id);


--
-- Name: ix_task_status_trackable_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_task_status_trackable_type ON public.task_status USING btree (trackable_type);


--
-- Name: ix_tasks_dedup_key; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX ix_tasks_dedup_key ON public.tasks USING btree (dedup_key);


--
-- Name: ix_tasks_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_tasks_type ON public.tasks USING btree (type);


--
-- Name: vectorchord_bm25_documents_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX vectorchord_bm25_documents_idx ON public.vectorchord_bm25_documents USING bm25 (embedding bm25_catalog.bm25_ops);


--
-- Name: vectorchord_code_embeddings_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX vectorchord_code_embeddings_idx ON public.vectorchord_code_embeddings USING vchordrq (embedding public.vector_l2_ops) WITH (options='
residual_quantization = true
[build.internal]
lists = []
');


--
-- Name: vectorchord_text_embeddings_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX vectorchord_text_embeddings_idx ON public.vectorchord_text_embeddings USING vchordrq (embedding public.vector_l2_ops) WITH (options='
residual_quantization = true
[build.internal]
lists = []
');


--
-- Name: enrichment_associations enrichment_associations_enrichment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.enrichment_associations
    ADD CONSTRAINT enrichment_associations_enrichment_id_fkey FOREIGN KEY (enrichment_id) REFERENCES public.enrichments_v2(id) ON DELETE CASCADE;


--
-- Name: git_branches git_branches_repo_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.git_branches
    ADD CONSTRAINT git_branches_repo_id_fkey FOREIGN KEY (repo_id) REFERENCES public.git_repos(id);


--
-- Name: git_commit_files git_commit_files_commit_sha_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.git_commit_files
    ADD CONSTRAINT git_commit_files_commit_sha_fkey FOREIGN KEY (commit_sha) REFERENCES public.git_commits(commit_sha);


--
-- Name: git_commits git_commits_repo_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.git_commits
    ADD CONSTRAINT git_commits_repo_id_fkey FOREIGN KEY (repo_id) REFERENCES public.git_repos(id);


--
-- Name: git_tags git_tags_repo_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.git_tags
    ADD CONSTRAINT git_tags_repo_id_fkey FOREIGN KEY (repo_id) REFERENCES public.git_repos(id);


--
-- Name: task_status task_status_parent_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.task_status
    ADD CONSTRAINT task_status_parent_fkey FOREIGN KEY (parent) REFERENCES public.task_status(id);


--
-- PostgreSQL database dump complete
--

