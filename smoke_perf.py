#!/usr/bin/env python3
# /// script
# dependencies = [
#   "httpx",
# ]
# ///
"""Performance comparison between Python and Go Kodit implementations.

Indexes a repository, then runs search load tests measuring latency and throughput.
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

BASE_HOST = "127.0.0.1"
TARGET_URI = "https://github.com/winderai/analytics-ai-agent-demo"

SEARCH_QUERIES = [
    {"keywords": ["agent", "query", "bigquery"], "limit": 10},
    {"keywords": ["pydantic", "model", "ai"], "limit": 10},
    {"keywords": ["main", "import", "async"], "limit": 10},
    {"code": "async def main():", "limit": 10},
    {"code": "from pydantic_ai import Agent", "limit": 10},
    {"code": "def run_query(sql: str)", "limit": 10},
    {"text": "AI agent that queries analytics data", "limit": 10},
    {"text": "bigquery integration with pydantic", "limit": 10},
]

LOAD_ROUNDS = 5  # repeat all queries this many times


def port_available(host, port):
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        try:
            s.bind((host, port))
        except OSError:
            return False
        return True


def wait_for_condition(condition, timeout=600, interval=1):
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            if condition():
                return True
        except Exception:
            pass
        time.sleep(interval)
    return False


def percentile(data, p):
    sorted_data = sorted(data)
    k = (len(sorted_data) - 1) * (p / 100)
    f = int(k)
    c = f + 1
    if c >= len(sorted_data):
        return sorted_data[f]
    return sorted_data[f] + (k - f) * (sorted_data[c] - sorted_data[f])


def run_server(name, port, cmd, env, cwd):
    """Start a server process and return it."""
    import httpx

    if not port_available(BASE_HOST, port):
        print(f"  ERROR: Port {port} already in use")
        return None

    process = subprocess.Popen(
        cmd, env=env, cwd=cwd,
        stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
    )

    base_url = f"http://{BASE_HOST}:{port}"
    start = time.time()
    while time.time() - start < 60:
        if not port_available(BASE_HOST, port):
            break
        time.sleep(0.5)
    else:
        print(f"  ERROR: {name} server failed to start")
        process.terminate()
        try:
            process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait(timeout=5)
        return None

    try:
        httpx.get(f"{base_url}/healthz", timeout=10).raise_for_status()
    except Exception as e:
        print(f"  ERROR: {name} health check failed: {e}")
        process.terminate()
        try:
            process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait(timeout=5)
        return None

    return process


def index_repo(name, port, client):
    """Create repo, wait for indexing, return timing info."""
    base_url = f"http://{BASE_HOST}:{port}"

    # Create repository (different payloads for Python vs Go)
    if name == "python":
        payload = {"data": {"type": "repository", "attributes": {"remote_uri": TARGET_URI}}}
    else:
        payload = {"remote_url": TARGET_URI}

    t0 = time.time()
    client.post(f"{base_url}/api/v1/repositories", json=payload).raise_for_status()

    # Get repo ID
    repos = client.get(f"{base_url}/api/v1/repositories").json()
    repo_id = repos["data"][0]["id"]

    # Wait for indexing
    stable_count = 0
    last_task_count = 0

    def indexing_finished():
        nonlocal stable_count, last_task_count
        response = client.get(f"{base_url}/api/v1/repositories/{repo_id}/status")
        status = response.json()
        terminal = {"completed", "skipped", "failed"}
        tasks = status.get("data", [])
        current = len(tasks)

        if current == last_task_count:
            stable_count += 1
        else:
            stable_count = 0
            last_task_count = current

        if current < 5 or stable_count < 3:
            return False

        pending = sum(1 for t in tasks if t["attributes"]["state"] == "pending")
        running = sum(1 for t in tasks if t["attributes"]["state"] in ("running", "started", "in_progress"))
        return pending == 0 and running == 0

    wait_for_condition(indexing_finished, timeout=600)
    indexing_time = time.time() - t0

    # Collect task details
    resp = client.get(f"{base_url}/api/v1/repositories/{repo_id}/status")
    tasks = resp.json().get("data", [])
    task_summary = {}
    for t in tasks:
        step = t["attributes"].get("step", "unknown")
        state = t["attributes"].get("state", "unknown")
        task_summary[step] = state

    return {
        "repo_id": repo_id,
        "indexing_time_s": round(indexing_time, 2),
        "total_tasks": len(tasks),
        "completed": sum(1 for s in task_summary.values() if s == "completed"),
        "skipped": sum(1 for s in task_summary.values() if s == "skipped"),
        "failed": sum(1 for s in task_summary.values() if s == "failed"),
        "tasks": task_summary,
    }


def run_search_load_test(name, port, client):
    """Run search queries and measure latency."""
    base_url = f"http://{BASE_HOST}:{port}"
    latencies = []
    results_by_query = []

    for round_num in range(LOAD_ROUNDS):
        for qi, query_attrs in enumerate(SEARCH_QUERIES):
            payload = {"data": {"type": "search", "attributes": query_attrs}}

            try:
                t0 = time.time()
                resp = client.post(f"{base_url}/api/v1/search", json=payload)
                elapsed_ms = (time.time() - t0) * 1000
                status = resp.status_code
                result_count = len(resp.json().get("data", [])) if status == 200 else -1
            except Exception as e:
                elapsed_ms = (time.time() - t0) * 1000
                status = -1
                result_count = -1
                print(f"    WARNING: query {qi} round {round_num} failed: {e}")

            latencies.append(elapsed_ms)

            # Only record result counts on first round
            if round_num == 0:
                query_label = next(
                    (f"kw:{v}" for k, v in query_attrs.items() if k == "keywords"),
                    next(
                        (f"code:{v}" for k, v in query_attrs.items() if k == "code"),
                        next(
                            (f"text:{v}" for k, v in query_attrs.items() if k == "text"),
                            "unknown",
                        ),
                    ),
                )
                results_by_query.append({
                    "query": query_label,
                    "status": status,
                    "results": result_count,
                    "latency_ms": round(elapsed_ms, 1),
                })

    return {
        "total_requests": len(latencies),
        "mean_ms": round(statistics.mean(latencies), 1),
        "median_ms": round(statistics.median(latencies), 1),
        "p95_ms": round(percentile(latencies, 95), 1),
        "p99_ms": round(percentile(latencies, 99), 1),
        "min_ms": round(min(latencies), 1),
        "max_ms": round(max(latencies), 1),
        "stddev_ms": round(statistics.stdev(latencies), 1) if len(latencies) > 1 else 0,
        "queries": results_by_query,
        "all_latencies": [round(x, 1) for x in latencies],
    }


def run_snippet_benchmark(name, port, client, repo_id):
    """Benchmark fetching snippets and enrichments."""
    base_url = f"http://{BASE_HOST}:{port}"

    # Get commit SHA
    resp = client.get(f"{base_url}/api/v1/repositories/{repo_id}/commits")
    commits = resp.json().get("data", [])
    if not commits:
        return {"error": "no commits found"}

    commit_sha = commits[0]["attributes"]["commit_sha"]
    commit_url = f"{base_url}/api/v1/repositories/{repo_id}/commits/{commit_sha}"

    timings = {}
    snippet_count = 0
    enrichment_count = 0
    file_count = 0

    # Snippets
    try:
        t0 = time.time()
        resp = client.get(f"{commit_url}/snippets", follow_redirects=True)
        timings["snippets_ms"] = round((time.time() - t0) * 1000, 1)
        snippet_count = len(resp.json().get("data", [])) if resp.status_code == 200 else 0
    except Exception as e:
        timings["snippets_ms"] = round((time.time() - t0) * 1000, 1)
        timings["snippets_error"] = str(e)
        print(f"    WARNING: snippets timed out ({timings['snippets_ms']:.0f}ms)")

    # Enrichments
    try:
        t0 = time.time()
        resp = client.get(f"{commit_url}/enrichments")
        timings["enrichments_ms"] = round((time.time() - t0) * 1000, 1)
        enrichment_count = len(resp.json().get("data", [])) if resp.status_code == 200 else 0
    except Exception as e:
        timings["enrichments_ms"] = round((time.time() - t0) * 1000, 1)
        timings["enrichments_error"] = str(e)
        print(f"    WARNING: enrichments timed out ({timings['enrichments_ms']:.0f}ms)")

    # Files
    try:
        t0 = time.time()
        resp = client.get(f"{commit_url}/files")
        timings["files_ms"] = round((time.time() - t0) * 1000, 1)
        file_count = len(resp.json().get("data", [])) if resp.status_code == 200 else 0
    except Exception as e:
        timings["files_ms"] = round((time.time() - t0) * 1000, 1)
        timings["files_error"] = str(e)
        print(f"    WARNING: files timed out ({timings['files_ms']:.0f}ms)")

    return {
        "snippet_count": snippet_count,
        "enrichment_count": enrichment_count,
        "file_count": file_count,
        **timings,
    }


def test_implementation(name, port, cmd, env, cwd):
    """Run full performance test for one implementation."""
    import httpx

    print(f"\n{'='*60}")
    print(f"  PERFORMANCE TEST: {name.upper()}")
    print(f"{'='*60}")

    print(f"  Starting {name} server on port {port}...")
    process = run_server(name, port, cmd, env, cwd)
    if not process:
        return {"error": "server failed to start"}

    results = {"implementation": name, "errors": []}

    try:
        with httpx.Client(timeout=120.0) as client:
            # Phase 1: Indexing
            print(f"  Indexing {TARGET_URI}...")
            idx = index_repo(name, port, client)
            results["indexing"] = idx
            print(f"  Indexing completed in {idx['indexing_time_s']}s "
                  f"({idx['completed']} completed, {idx['skipped']} skipped, {idx['failed']} failed)")

            # Phase 2: Endpoint benchmarks
            print(f"  Running endpoint benchmarks...")
            bench = run_snippet_benchmark(name, port, client, idx["repo_id"])
            results["endpoints"] = bench
            print(f"  Snippets: {bench.get('snippet_count', '?')} in {bench.get('snippets_ms', '?')}ms | "
                  f"Enrichments: {bench.get('enrichment_count', '?')} in {bench.get('enrichments_ms', '?')}ms | "
                  f"Files: {bench.get('file_count', '?')} in {bench.get('files_ms', '?')}ms")

            # Phase 3: Search load test
            total_queries = len(SEARCH_QUERIES) * LOAD_ROUNDS
            print(f"  Running search load test ({total_queries} requests, "
                  f"{len(SEARCH_QUERIES)} queries x {LOAD_ROUNDS} rounds)...")
            load = run_search_load_test(name, port, client)
            results["search_load"] = load
            print(f"  Search latency: mean={load['mean_ms']}ms, "
                  f"median={load['median_ms']}ms, p95={load['p95_ms']}ms, p99={load['p99_ms']}ms")

            # Cleanup
            base_url = f"http://{BASE_HOST}:{port}"
            client.delete(f"{base_url}/api/v1/repositories/{idx['repo_id']}").raise_for_status()

    except Exception as e:
        results["errors"].append(str(e))
        import traceback
        traceback.print_exc()
    finally:
        process.terminate()
        try:
            process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait(timeout=5)

    return results


def print_comparison(py_results, go_results):
    """Print side-by-side comparison."""
    print(f"\n{'='*60}")
    print("  COMPARISON SUMMARY")
    print(f"{'='*60}")

    # Indexing
    py_idx = py_results.get("indexing", {})
    go_idx = go_results.get("indexing", {})
    print(f"\n  INDEXING:")
    print(f"    {'Metric':<30} {'Python':>10} {'Go':>10} {'Diff':>12}")
    print(f"    {'-'*62}")
    py_t = py_idx.get("indexing_time_s", 0)
    go_t = go_idx.get("indexing_time_s", 0)
    speedup = f"{py_t / go_t:.1f}x" if go_t > 0 else "n/a"
    print(f"    {'Total time (s)':<30} {py_t:>10.1f} {go_t:>10.1f} {speedup:>12}")
    print(f"    {'Tasks completed':<30} {py_idx.get('completed', 0):>10} {go_idx.get('completed', 0):>10}")
    print(f"    {'Tasks skipped':<30} {py_idx.get('skipped', 0):>10} {go_idx.get('skipped', 0):>10}")

    # Endpoints
    py_ep = py_results.get("endpoints", {})
    go_ep = go_results.get("endpoints", {})
    print(f"\n  ENDPOINT LATENCY:")
    print(f"    {'Metric':<30} {'Python':>10} {'Go':>10} {'Diff':>12}")
    print(f"    {'-'*62}")
    for key, label in [("snippets_ms", "Snippets (ms)"), ("enrichments_ms", "Enrichments (ms)"), ("files_ms", "Files (ms)")]:
        py_v = py_ep.get(key, 0)
        go_v = go_ep.get(key, 0)
        speedup = f"{py_v / go_v:.1f}x" if go_v > 0 else "n/a"
        print(f"    {label:<30} {py_v:>10.1f} {go_v:>10.1f} {speedup:>12}")
    for key, label in [("snippet_count", "Snippet count"), ("enrichment_count", "Enrichment count"), ("file_count", "File count")]:
        print(f"    {label:<30} {py_ep.get(key, 0):>10} {go_ep.get(key, 0):>10}")

    # Search load
    py_sl = py_results.get("search_load", {})
    go_sl = go_results.get("search_load", {})
    print(f"\n  SEARCH LOAD TEST ({py_sl.get('total_requests', 0)} requests each):")
    print(f"    {'Metric':<30} {'Python':>10} {'Go':>10} {'Diff':>12}")
    print(f"    {'-'*62}")
    for key, label in [("mean_ms", "Mean (ms)"), ("median_ms", "Median (ms)"),
                        ("p95_ms", "P95 (ms)"), ("p99_ms", "P99 (ms)"),
                        ("min_ms", "Min (ms)"), ("max_ms", "Max (ms)"),
                        ("stddev_ms", "Stddev (ms)")]:
        py_v = py_sl.get(key, 0)
        go_v = go_sl.get(key, 0)
        speedup = f"{py_v / go_v:.1f}x" if go_v > 0 else "n/a"
        print(f"    {label:<30} {py_v:>10.1f} {go_v:>10.1f} {speedup:>12}")

    # Per-query results
    py_queries = py_sl.get("queries", [])
    go_queries = go_sl.get("queries", [])
    if py_queries and go_queries:
        print(f"\n  PER-QUERY DETAIL (first round):")
        print(f"    {'Query':<45} {'Py ms':>7} {'Py #':>5} {'Go ms':>7} {'Go #':>5}")
        print(f"    {'-'*69}")
        for pq, gq in zip(py_queries, go_queries):
            label = pq["query"][:44]
            print(f"    {label:<45} {pq['latency_ms']:>7.1f} {pq['results']:>5} "
                  f"{gq['latency_ms']:>7.1f} {gq['results']:>5}")

    # Errors
    py_errs = py_results.get("errors", [])
    go_errs = go_results.get("errors", [])
    if py_errs or go_errs:
        print(f"\n  ERRORS:")
        for e in py_errs:
            print(f"    Python: {e}")
        for e in go_errs:
            print(f"    Go: {e}")


def main():
    import httpx  # noqa: F401 - validate import early

    embedding_api_key = os.environ.get("EMBEDDING_ENDPOINT_API_KEY", "")
    enrichment_api_key = os.environ.get("ENRICHMENT_ENDPOINT_API_KEY", "")
    if not embedding_api_key or not enrichment_api_key:
        print("ERROR: EMBEDDING_ENDPOINT_API_KEY and ENRICHMENT_ENDPOINT_API_KEY are required")
        sys.exit(1)

    script_dir = Path(__file__).parent

    with tempfile.NamedTemporaryFile(delete=False) as f:
        tmpfile = Path(f.name)

    # --- Python setup ---
    py_port = 8081
    py_env = os.environ.copy()
    py_env.update({
        "DISABLE_TELEMETRY": "true",
        "DB_URL": "sqlite+aiosqlite:///:memory:",
        "EMBEDDING_ENDPOINT_MODEL": "openrouter/mistralai/codestral-embed-2505",
        "EMBEDDING_ENDPOINT_API_KEY": embedding_api_key,
        "ENRICHMENT_ENDPOINT_MODEL": "openrouter/mistralai/ministral-8b-2512",
        "ENRICHMENT_ENDPOINT_API_KEY": enrichment_api_key,
    })
    prefix = [] if os.getenv("CI") else ["uv", "run"]
    py_cmd = [*prefix, "kodit", "--env-file", str(tmpfile), "serve",
              "--host", BASE_HOST, "--port", str(py_port)]
    py_cwd = str(script_dir / "python-source")

    # --- Go setup ---
    go_port = 8082
    go_target_dir = script_dir / "go-target"
    go_env = os.environ.copy()
    go_env.update({
        "DISABLE_TELEMETRY": "true",
        "DB_URL": "sqlite:///:memory:",
        "LOG_LEVEL": "warn",
        "EMBEDDING_ENDPOINT_BASE_URL": "https://openrouter.ai/api/v1",
        "EMBEDDING_ENDPOINT_MODEL": "mistralai/codestral-embed-2505",
        "EMBEDDING_ENDPOINT_API_KEY": embedding_api_key,
        "ENRICHMENT_ENDPOINT_BASE_URL": "https://openrouter.ai/api/v1",
        "ENRICHMENT_ENDPOINT_MODEL": "mistralai/ministral-8b-2512",
        "ENRICHMENT_ENDPOINT_API_KEY": enrichment_api_key,
    })

    # Build Go binary
    print("Building Go binary...")
    build = subprocess.run(
        ["go", "build", "-tags", "fts5", "-o", "/tmp/kodit-perf", "./cmd/kodit"],
        cwd=str(go_target_dir), capture_output=True, text=True,
    )
    if build.returncode != 0:
        print(f"Go build failed: {build.stderr}")
        sys.exit(1)
    print("Go build successful")

    go_cmd = ["/tmp/kodit-perf", "serve", "--host", BASE_HOST,
              "--port", str(go_port), "--env-file", str(tmpfile)]
    go_cwd = str(go_target_dir)

    # --- Run tests ---
    py_results = test_implementation("python", py_port, py_cmd, py_env, py_cwd)
    go_results = test_implementation("go", go_port, go_cmd, go_env, go_cwd)

    # --- Print comparison ---
    print_comparison(py_results, go_results)

    # --- Save JSON ---
    combined = {"python": py_results, "go": go_results}
    output_path = "/tmp/smoke_perf.json"
    with open(output_path, "w") as f:
        json.dump(combined, f, indent=2, default=str)
    print(f"\nFull results saved to {output_path}")

    tmpfile.unlink(missing_ok=True)


if __name__ == "__main__":
    main()
