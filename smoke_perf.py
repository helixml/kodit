#!/usr/bin/env python3
# /// script
# dependencies = [
#   "httpx",
# ]
# ///
"""Performance comparison between Python and Go Kodit implementations.

Indexes a repository, benchmarks keyword search (BM25) and API endpoints.
Semantic search is tested once with a short timeout since it depends on external APIs.

Run with: uv run smoke_perf.py
"""

import json
import os
import socket
import statistics
import subprocess
import sys
import tempfile
import time
from pathlib import Path

sys.stdout.reconfigure(line_buffering=True)
sys.stderr.reconfigure(line_buffering=True)

BASE_HOST = "127.0.0.1"
TARGET_URI = "https://gist.github.com/philwinder/7aa38185e20433c04c533f2b28f4e217.git"

# BM25 keyword searches - fast, no external API calls
KEYWORD_QUERIES = [
    {"keywords": ["main", "func", "package"], "limit": 10},
    {"keywords": ["agent", "query"], "limit": 10},
    {"keywords": ["import", "print"], "limit": 10},
    {"keywords": ["def", "return"], "limit": 10},
]

# Semantic searches - require embedding API, tested once with short timeout
SEMANTIC_QUERIES = [
    {"code": "func main() { fmt.Println }", "limit": 5},
    {"text": "AI agent that queries analytics data", "limit": 5},
]

LOAD_ROUNDS = 3
SEARCH_ROUNDS = 1  # search may call embedding APIs, so keep rounds low


def log(msg):
    print(f"[{time.strftime('%H:%M:%S')}] {msg}", flush=True)


def port_available(host, port):
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        try:
            s.bind((host, port))
        except OSError:
            return False
        return True


def percentile(data, p):
    s = sorted(data)
    k = (len(s) - 1) * (p / 100)
    f = int(k)
    c = f + 1
    if c >= len(s):
        return s[f]
    return s[f] + (k - f) * (s[c] - s[f])


def latency_stats(latencies):
    if not latencies:
        return {}
    result = {
        "n": len(latencies),
        "mean_ms": round(statistics.mean(latencies), 1),
        "min_ms": round(min(latencies), 1),
        "max_ms": round(max(latencies), 1),
    }
    if len(latencies) > 1:
        result["median_ms"] = round(statistics.median(latencies), 1)
        result["p95_ms"] = round(percentile(latencies, 95), 1)
        result["stddev_ms"] = round(statistics.stdev(latencies), 1)
    return result


def timed(client, method, url, **kwargs):
    t0 = time.time()
    resp = getattr(client, method)(url, **kwargs)
    return resp, (time.time() - t0) * 1000


def start_server(name, port, cmd, env, cwd, log_file):
    import httpx

    if not port_available(BASE_HOST, port):
        log(f"ERROR: Port {port} already in use")
        return None

    log(f"Starting {name} server (logs -> {log_file})...")
    log_fh = open(log_file, "w")
    process = subprocess.Popen(
        cmd, env=env, cwd=cwd,
        stdout=log_fh, stderr=subprocess.STDOUT,
    )
    process._log_fh = log_fh  # stash so we can close later

    deadline = time.time() + 30
    while time.time() < deadline:
        if process.poll() is not None:
            log_fh.close()
            log(f"ERROR: {name} exited early (code {process.returncode}), see {log_file}")
            return None
        if not port_available(BASE_HOST, port):
            break
        time.sleep(0.3)
    else:
        log_fh.close()
        log(f"ERROR: {name} failed to bind within 30s, see {log_file}")
        process.kill()
        process.wait(timeout=5)
        return None

    try:
        httpx.get(f"http://{BASE_HOST}:{port}/healthz", timeout=5).raise_for_status()
    except Exception as e:
        log(f"ERROR: {name} health check failed: {e}")
        process.kill()
        process.wait(timeout=5)
        return None

    log(f"{name} server ready on :{port}")
    return process


def stop_server(process):
    if not process:
        return
    process.terminate()
    try:
        process.wait(timeout=5)
    except subprocess.TimeoutExpired:
        process.kill()
        process.wait(timeout=5)
    if hasattr(process, "_log_fh"):
        process._log_fh.close()


def index_repo(name, port, client):
    base_url = f"http://{BASE_HOST}:{port}"

    if name == "python":
        payload = {"data": {"type": "repository", "attributes": {"remote_uri": TARGET_URI}}}
    else:
        payload = {"remote_url": TARGET_URI}

    t0 = time.time()
    client.post(f"{base_url}/api/v1/repositories", json=payload).raise_for_status()

    repos = client.get(f"{base_url}/api/v1/repositories").json()
    repo_id = repos["data"][0]["id"]
    log(f"  repo_id={repo_id}, waiting for indexing...")

    stable = 0
    last_count = 0
    deadline = time.time() + 300

    while time.time() < deadline:
        tasks = client.get(f"{base_url}/api/v1/repositories/{repo_id}/status").json().get("data", [])
        current = len(tasks)
        stable = stable + 1 if current == last_count else 0
        last_count = current

        pending = sum(1 for t in tasks if t["attributes"]["state"] == "pending")
        running = sum(1 for t in tasks if t["attributes"]["state"] in ("running", "started", "in_progress"))

        if current >= 5 and stable >= 3 and pending == 0 and running == 0:
            break

        log(f"  tasks={current} pending={pending} running={running}")
        time.sleep(2)

    indexing_time = time.time() - t0

    tasks = client.get(f"{base_url}/api/v1/repositories/{repo_id}/status").json().get("data", [])
    states = {}
    for t in tasks:
        states[t["attributes"].get("step", "?")] = t["attributes"].get("state", "?")

    return {
        "repo_id": repo_id,
        "indexing_time_s": round(indexing_time, 2),
        "total_tasks": len(tasks),
        "completed": sum(1 for s in states.values() if s == "completed"),
        "skipped": sum(1 for s in states.values() if s == "skipped"),
        "failed": sum(1 for s in states.values() if s == "failed"),
        "tasks": states,
    }


def bench_endpoint(client, method, url, rounds, **kwargs):
    latencies = []
    count = None
    for i in range(rounds):
        try:
            resp, ms = timed(client, method, url, **kwargs)
            latencies.append(ms)
            if i == 0 and resp.status_code == 200:
                data = resp.json().get("data", [])
                count = len(data) if isinstance(data, list) else 0
        except Exception as e:
            log(f"  WARNING: {url} attempt {i}: {e}")
            break
    if not latencies:
        return {"count": count, "error": "all requests failed"}
    return {"count": count, **latency_stats(latencies)}


def bench_keyword_search(port):
    """Benchmark keyword search with a per-query 15s timeout."""
    import httpx

    base_url = f"http://{BASE_HOST}:{port}"
    latencies = []
    per_query = []
    short = httpx.Client(timeout=15.0)

    try:
        for rnd in range(SEARCH_ROUNDS):
            for attrs in KEYWORD_QUERIES:
                payload = {"data": {"type": "search", "attributes": attrs}}
                try:
                    resp, ms = timed(short, "post", f"{base_url}/api/v1/search", json=payload)
                    latencies.append(ms)
                    n = len(resp.json().get("data", [])) if resp.status_code == 200 else -1
                    if rnd == 0:
                        per_query.append({
                            "query": f"kw:{attrs['keywords']}",
                            "results": n,
                            "latency_ms": round(ms, 1),
                        })
                except Exception as e:
                    log(f"  TIMEOUT: {attrs.get('keywords', '?')}")
                    if rnd == 0:
                        per_query.append({
                            "query": f"kw:{attrs['keywords']}",
                            "results": -1,
                            "latency_ms": -1,
                            "error": "timeout (15s)",
                        })
    finally:
        short.close()

    if not latencies:
        return {"queries": per_query, "error": "all requests timed out"}
    return {"queries": per_query, **latency_stats(latencies)}


def bench_semantic_search(port, client):
    """Test semantic search once per query with a 15s timeout."""
    import httpx

    base_url = f"http://{BASE_HOST}:{port}"
    results = []
    short_client = httpx.Client(timeout=15.0)

    try:
        for attrs in SEMANTIC_QUERIES:
            payload = {"data": {"type": "search", "attributes": attrs}}
            try:
                resp, ms = timed(short_client, "post", f"{base_url}/api/v1/search", json=payload)
                n = len(resp.json().get("data", [])) if resp.status_code == 200 else -1
                label = attrs.get("code", attrs.get("text", "?"))
                results.append({"query": label[:50], "results": n, "latency_ms": round(ms, 1), "status": "ok"})
            except Exception as e:
                label = attrs.get("code", attrs.get("text", "?"))
                results.append({"query": label[:50], "results": -1, "latency_ms": -1, "status": str(e)[:60]})
    finally:
        short_client.close()

    return results


def test_one(name, port, cmd, env, cwd, log_file):
    import httpx

    log(f"{'=' * 50}")
    log(f"PERFORMANCE TEST: {name.upper()}")
    log(f"{'=' * 50}")

    process = start_server(name, port, cmd, env, cwd, log_file)
    if not process:
        return {"implementation": name, "error": "server failed to start"}

    results = {"implementation": name, "errors": []}
    base_url = f"http://{BASE_HOST}:{port}"

    try:
        with httpx.Client(timeout=60.0) as client:
            # Indexing
            log("Phase 1: Index repository")
            idx = index_repo(name, port, client)
            results["indexing"] = idx
            log(f"  Indexed in {idx['indexing_time_s']}s "
                f"({idx['completed']} ok, {idx['skipped']} skip, {idx['failed']} fail)")

            repo_id = idx["repo_id"]
            commits = client.get(f"{base_url}/api/v1/repositories/{repo_id}/commits").json()
            commit_sha = commits["data"][0]["attributes"]["commit_sha"] if commits.get("data") else None
            commit_url = f"{base_url}/api/v1/repositories/{repo_id}/commits/{commit_sha}"

            # API endpoints
            log("Phase 2: API endpoint latency")
            results["repos_api"] = bench_endpoint(client, "get", f"{base_url}/api/v1/repositories", LOAD_ROUNDS)
            log(f"  /repositories: {results['repos_api'].get('mean_ms', 'ERR')}ms")

            results["commits_api"] = bench_endpoint(
                client, "get", f"{base_url}/api/v1/repositories/{repo_id}/commits", LOAD_ROUNDS)
            log(f"  /commits: {results['commits_api'].get('mean_ms', 'ERR')}ms")

            if commit_sha:
                results["files_api"] = bench_endpoint(client, "get", f"{commit_url}/files", LOAD_ROUNDS)
                log(f"  /files: {results['files_api'].get('count', '?')} items, {results['files_api'].get('mean_ms', 'ERR')}ms")

                results["snippets_api"] = bench_endpoint(
                    client, "get", f"{commit_url}/snippets", LOAD_ROUNDS, follow_redirects=True)
                log(f"  /snippets: {results['snippets_api'].get('count', '?')} items, {results['snippets_api'].get('mean_ms', 'ERR')}ms")

                results["enrichments_api"] = bench_endpoint(client, "get", f"{commit_url}/enrichments", LOAD_ROUNDS)
                log(f"  /enrichments: {results['enrichments_api'].get('count', '?')} items, {results['enrichments_api'].get('mean_ms', 'ERR')}ms")

            # Keyword search (uses per-query 15s timeout)
            total = len(KEYWORD_QUERIES) * SEARCH_ROUNDS
            log(f"Phase 3: Keyword search ({total} queries, 15s timeout each)")
            results["keyword_search"] = bench_keyword_search(port)
            ks = results["keyword_search"]
            log(f"  mean={ks.get('mean_ms', 'ERR')}ms median={ks.get('median_ms', '?')}ms p95={ks.get('p95_ms', '?')}ms")

            # Semantic search (single round, short timeout)
            log("Phase 4: Semantic search (1 round, 15s timeout)")
            results["semantic_search"] = bench_semantic_search(port, client)
            for sq in results["semantic_search"]:
                log(f"  {sq['query'][:40]}: {sq['latency_ms']}ms ({sq['results']} results) [{sq['status']}]")

            # Cleanup
            client.delete(f"{base_url}/api/v1/repositories/{repo_id}").raise_for_status()
            log("Cleanup done")

    except Exception as e:
        results["errors"].append(str(e))
        import traceback
        traceback.print_exc()
    finally:
        stop_server(process)

    return results


def print_comparison(py, go):
    print(f"\n{'=' * 72}", flush=True)
    print("  PERFORMANCE COMPARISON: PYTHON vs GO", flush=True)
    print(f"{'=' * 72}", flush=True)

    def hdr():
        print(f"    {'Metric':<35} {'Python':>12} {'Go':>12} {'Py/Go':>8}")
        print(f"    {'-' * 67}")

    def row(label, pv, gv, show_ratio=True):
        r = ""
        if show_ratio and isinstance(pv, (int, float)) and isinstance(gv, (int, float)) and gv > 0:
            r = f"{pv / gv:.1f}x"
        print(f"    {label:<35} {str(pv):>12} {str(gv):>12} {r:>8}")

    # Indexing
    pi, gi = py.get("indexing", {}), go.get("indexing", {})
    print("\n  INDEXING:")
    hdr()
    row("Total time (s)", pi.get("indexing_time_s", 0), gi.get("indexing_time_s", 0))
    row("Tasks completed", pi.get("completed", 0), gi.get("completed", 0), False)
    row("Tasks skipped", pi.get("skipped", 0), gi.get("skipped", 0), False)
    row("Tasks failed", pi.get("failed", 0), gi.get("failed", 0), False)

    # API endpoints
    for key, label in [
        ("repos_api", "GET /repositories"),
        ("commits_api", "GET /commits"),
        ("files_api", "GET /files"),
        ("snippets_api", "GET /snippets"),
        ("enrichments_api", "GET /enrichments"),
    ]:
        pa, ga = py.get(key, {}), go.get(key, {})
        if pa or ga:
            print(f"\n  {label}:")
            hdr()
            row("Mean (ms)", pa.get("mean_ms", 0), ga.get("mean_ms", 0))
            row("Min (ms)", pa.get("min_ms", 0), ga.get("min_ms", 0))
            row("Max (ms)", pa.get("max_ms", 0), ga.get("max_ms", 0))
            row("Item count", pa.get("count", 0), ga.get("count", 0), False)

    # Keyword search
    pk, gk = py.get("keyword_search", {}), go.get("keyword_search", {})
    print("\n  KEYWORD SEARCH (BM25):")
    hdr()
    for m in ("mean_ms", "median_ms", "p95_ms", "min_ms", "max_ms", "stddev_ms"):
        label = m.replace("_ms", "").replace("_", " ").title() + " (ms)"
        row(label, pk.get(m, 0), gk.get(m, 0))

    # Per-query keyword detail
    pqs, gqs = pk.get("queries", []), gk.get("queries", [])
    if pqs and gqs:
        print(f"\n  KEYWORD QUERY DETAIL (first round):")
        print(f"    {'Query':<38} {'Py ms':>7} {'Py#':>4} {'Go ms':>7} {'Go#':>4}")
        print(f"    {'-' * 60}")
        for p, g in zip(pqs, gqs):
            print(f"    {p['query'][:37]:<38} {p['latency_ms']:>7.1f} {p['results']:>4} "
                  f"{g['latency_ms']:>7.1f} {g['results']:>4}")

    # Semantic search
    pss, gss = py.get("semantic_search", []), go.get("semantic_search", [])
    if pss and gss:
        print(f"\n  SEMANTIC SEARCH (single round, 15s timeout):")
        print(f"    {'Query':<38} {'Py ms':>7} {'Py#':>4} {'Go ms':>7} {'Go#':>4}")
        print(f"    {'-' * 60}")
        for p, g in zip(pss, gss):
            pm = f"{p['latency_ms']:.0f}" if p['latency_ms'] >= 0 else "TIMEOUT"
            gm = f"{g['latency_ms']:.0f}" if g['latency_ms'] >= 0 else "TIMEOUT"
            print(f"    {p['query'][:37]:<38} {pm:>7} {p['results']:>4} {gm:>7} {g['results']:>4}")

    # Errors
    pe, ge = py.get("errors", []), go.get("errors", [])
    if pe or ge:
        print("\n  ERRORS:")
        for e in pe:
            print(f"    Python: {e}")
        for e in ge:
            print(f"    Go: {e}")

    sys.stdout.flush()


def main():
    import httpx  # noqa: F401

    log("Performance comparison starting")

    embedding_key = os.environ.get("EMBEDDING_ENDPOINT_API_KEY", "")
    enrichment_key = os.environ.get("ENRICHMENT_ENDPOINT_API_KEY", "")
    if not embedding_key or not enrichment_key:
        log("ERROR: EMBEDDING_ENDPOINT_API_KEY and ENRICHMENT_ENDPOINT_API_KEY required")
        sys.exit(1)

    script_dir = Path(__file__).parent
    with tempfile.NamedTemporaryFile(delete=False) as f:
        tmpfile = Path(f.name)

    # Python
    py_port = 8081
    py_log = "/tmp/smoke_perf_python.log"
    py_env = os.environ.copy()
    py_env.update({
        "DISABLE_TELEMETRY": "true",
        "LOG_LEVEL": "debug",
        "DB_URL": "sqlite+aiosqlite:///:memory:",
        "EMBEDDING_ENDPOINT_MODEL": "openrouter/mistralai/codestral-embed-2505",
        "EMBEDDING_ENDPOINT_API_KEY": embedding_key,
        "ENRICHMENT_ENDPOINT_MODEL": "openrouter/mistralai/ministral-8b-2512",
        "ENRICHMENT_ENDPOINT_API_KEY": enrichment_key,
    })
    prefix = [] if os.getenv("CI") else ["uv", "run"]
    py_cmd = [*prefix, "kodit", "--env-file", str(tmpfile), "serve",
              "--host", BASE_HOST, "--port", str(py_port)]
    py_cwd = str(script_dir / "python-source")

    # Go
    go_port = 8082
    go_log = "/tmp/smoke_perf_go.log"
    go_dir = script_dir / "go-target"
    go_env = os.environ.copy()
    go_env.update({
        "DISABLE_TELEMETRY": "true",
        "DB_URL": "sqlite:///:memory:",
        "LOG_LEVEL": "debug",
        "EMBEDDING_ENDPOINT_BASE_URL": "https://openrouter.ai/api/v1",
        "EMBEDDING_ENDPOINT_MODEL": "mistralai/codestral-embed-2505",
        "EMBEDDING_ENDPOINT_API_KEY": embedding_key,
        "ENRICHMENT_ENDPOINT_BASE_URL": "https://openrouter.ai/api/v1",
        "ENRICHMENT_ENDPOINT_MODEL": "mistralai/ministral-8b-2512",
        "ENRICHMENT_ENDPOINT_API_KEY": enrichment_key,
    })

    log("Building Go binary...")
    build = subprocess.run(
        ["go", "build", "-tags", "fts5", "-o", "/tmp/kodit-perf", "./cmd/kodit"],
        cwd=str(go_dir), capture_output=True, text=True, timeout=120,
    )
    if build.returncode != 0:
        log(f"Go build failed:\n{build.stderr}")
        sys.exit(1)
    log("Go build OK")

    go_cmd = ["/tmp/kodit-perf", "serve", "--host", BASE_HOST,
              "--port", str(go_port), "--env-file", str(tmpfile)]

    # Run
    py_results = test_one("python", py_port, py_cmd, py_env, py_cwd, py_log)
    go_results = test_one("go", go_port, go_cmd, go_env, str(go_dir), go_log)

    print_comparison(py_results, go_results)

    output_path = "/tmp/smoke_perf.json"
    with open(output_path, "w") as fout:
        json.dump({"python": py_results, "go": go_results}, fout, indent=2, default=str)
    log(f"Full JSON saved to {output_path}")
    log(f"Python server log: {py_log}")
    log(f"Go server log: {go_log}")

    tmpfile.unlink(missing_ok=True)


if __name__ == "__main__":
    main()
