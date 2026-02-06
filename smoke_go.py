#!/usr/bin/env python3
# /// script
# dependencies = [
#   "httpx",
# ]
# ///
"""Simplified smoke test for Go Kodit - outputs comparable results."""

import json
import os
import socket
import subprocess
import sys
import tempfile
import time
from pathlib import Path

BASE_HOST = "127.0.0.1"
BASE_PORT = 8082
BASE_URL = f"http://{BASE_HOST}:{BASE_PORT}"
TARGET_URI = "https://gist.github.com/philwinder/7aa38185e20433c04c533f2b28f4e217.git"


def port_available(host: str, port: int) -> bool:
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


def main():
    import httpx

    if not port_available(BASE_HOST, BASE_PORT):
        print(f"ERROR: Port {BASE_PORT} already in use")
        sys.exit(1)

    with tempfile.NamedTemporaryFile(delete=False) as f:
        tmpfile = Path(f.name)

    # Check required API keys
    embedding_api_key = os.environ.get("EMBEDDING_ENDPOINT_API_KEY", "")
    enrichment_api_key = os.environ.get("ENRICHMENT_ENDPOINT_API_KEY", "")
    if not embedding_api_key:
        print("ERROR: EMBEDDING_ENDPOINT_API_KEY environment variable is required")
        sys.exit(1)
    if not enrichment_api_key:
        print("ERROR: ENRICHMENT_ENDPOINT_API_KEY environment variable is required")
        sys.exit(1)

    env = os.environ.copy()
    env.update({
        "DISABLE_TELEMETRY": "true",
        "DB_URL": "sqlite:///:memory:",
        "LOG_LEVEL": "debug",  # Enable debug logging
        # Embedding provider (OpenRouter with Codestral Embed)
        "EMBEDDING_ENDPOINT_BASE_URL": "https://openrouter.ai/api/v1",
        "EMBEDDING_ENDPOINT_MODEL": "mistralai/codestral-embed-2505",
        "EMBEDDING_ENDPOINT_API_KEY": embedding_api_key,
        # Enrichment provider (OpenRouter with Ministral 8B)
        "ENRICHMENT_ENDPOINT_BASE_URL": "https://openrouter.ai/api/v1",
        "ENRICHMENT_ENDPOINT_MODEL": "mistralai/ministral-8b-2512",
        "ENRICHMENT_ENDPOINT_API_KEY": enrichment_api_key,
    })
    if "SMOKE_DB_URL" in env:
        env["DB_URL"] = env["SMOKE_DB_URL"]

    go_target_dir = Path(__file__).parent / "go-target"

    # Build the binary first
    print("Building Go binary...")
    build_result = subprocess.run(
        ["go", "build", "-o", "/tmp/kodit-smoke", "./cmd/kodit"],
        cwd=str(go_target_dir),
        capture_output=True,
        text=True,
    )
    if build_result.returncode != 0:
        print(f"Build failed: {build_result.stderr}")
        sys.exit(1)
    print("Build successful")

    cmd = [
        "/tmp/kodit-smoke",
        "serve",
        "--host", BASE_HOST,
        "--port", str(BASE_PORT),
        "--env-file", str(tmpfile),
    ]

    process = subprocess.Popen(
        cmd,
        env=env,
        cwd=str(go_target_dir),
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,  # Merge stderr into stdout
    )

    results = {
        "implementation": "go",
        "repository": TARGET_URI,
        "snippets": [],
        "search_results": [],
        "errors": [],
    }

    def read_output():
        """Read any available output from the process."""
        if process.stdout:
            import select
            while select.select([process.stdout], [], [], 0)[0]:
                line = process.stdout.readline()
                if line:
                    print(f"[GO SERVER] {line.decode().rstrip()}")
                else:
                    break

    try:
        with httpx.Client(timeout=30.0) as client:
            print("Waiting for Go server to start...")
            start = time.time()
            while time.time() - start < 60:
                read_output()
                if not port_available(BASE_HOST, BASE_PORT):
                    break
                time.sleep(1)
            else:
                results["errors"].append("Server failed to start")
                read_output()
                return results

            client.get(f"{BASE_URL}/healthz").raise_for_status()
            print("Go server health check passed")

            # Create repository - Go accepts simpler format
            payload = {"remote_url": TARGET_URI}
            client.post(f"{BASE_URL}/api/v1/repositories", json=payload).raise_for_status()
            print(f"Created repository: {TARGET_URI}")

            # Get repo ID
            resp = client.get(f"{BASE_URL}/api/v1/repositories")
            repos = resp.json()
            repo_id = repos["data"][0]["id"]
            print(f"Repository ID: {repo_id}")

            # Wait for indexing
            print("Waiting for indexing to complete...")

            # Track stable task count to detect when all tasks have been registered
            stable_count = 0
            last_task_count = 0

            def indexing_finished():
                nonlocal stable_count, last_task_count
                response = client.get(f"{BASE_URL}/api/v1/repositories/{repo_id}/status")
                status = response.json()
                terminal = {"completed", "skipped", "failed"}
                tasks = status.get("data", [])
                current_count = len(tasks)

                # Check if task count is stable (no new tasks appearing)
                if current_count == last_task_count:
                    stable_count += 1
                else:
                    stable_count = 0
                    last_task_count = current_count

                # Need at least 6 tasks and count must be stable for 3 iterations
                if current_count < 6 or stable_count < 3:
                    pending = sum(1 for t in tasks if t["attributes"]["state"] == "pending")
                    running = sum(1 for t in tasks if t["attributes"]["state"] in ("running", "started", "in_progress"))
                    print(f"  ... tasks={current_count}, pending={pending}, running={running}, stable_count={stable_count}")
                    return False

                pending = sum(1 for t in tasks if t["attributes"]["state"] == "pending")
                running = sum(1 for t in tasks if t["attributes"]["state"] in ("running", "started", "in_progress"))
                if pending > 0 or running > 0:
                    print(f"  ... waiting for tasks to complete (total={current_count}, pending={pending}, running={running})")
                    return False
                # All tasks in terminal state and count is stable
                print(f"  ... all {current_count} tasks in terminal state (stable for {stable_count} checks)")
                return True

            if not wait_for_condition(indexing_finished, timeout=600):
                results["errors"].append("Indexing timed out")
                return results

            print("Indexing completed")

            # Show final task status
            resp = client.get(f"{BASE_URL}/api/v1/repositories/{repo_id}/status")
            task_status = resp.json()
            print("\nTask Status:")
            for task in task_status.get("data", []):
                step = task["attributes"].get("step", "unknown")
                state = task["attributes"].get("state", "unknown")
                error = task["attributes"].get("error", "")
                print(f"  {step}: {state}" + (f" (error: {error})" if error else ""))

            # Get commits
            resp = client.get(f"{BASE_URL}/api/v1/repositories/{repo_id}/commits")
            commits = resp.json()
            if not commits["data"]:
                results["errors"].append("No commits found")
                return results

            commit_sha = commits["data"][0]["attributes"]["commit_sha"]
            commit_url = f"{BASE_URL}/api/v1/repositories/{repo_id}/commits/{commit_sha}"
            print(f"Commit SHA: {commit_sha}")

            # Get files
            resp = client.get(f"{commit_url}/files")
            files = resp.json()
            print(f"Files count: {len(files['data'])}")
            for f in files["data"]:
                print(f"  - {f['attributes']['path']} ({f['attributes']['size']} bytes)")

            # Get snippets - Go returns JSON directly
            resp = client.get(f"{commit_url}/snippets")
            if resp.status_code == 200:
                snippets = resp.json()
                results["snippets"] = snippets.get("data", [])
                print(f"Snippets count: {len(results['snippets'])}")
                for i, s in enumerate(results["snippets"][:5]):  # Show first 5
                    content = s.get("attributes", {}).get("content", {})
                    lang = content.get("language", "unknown")
                    value = content.get("value", "")[:100]
                    print(f"  Snippet {i}: lang={lang}, preview={value!r}...")
            else:
                results["errors"].append(f"Snippets returned status {resp.status_code}")

            # Get enrichments
            resp = client.get(f"{commit_url}/enrichments")
            enrichments = resp.json()
            print(f"Enrichments count: {len(enrichments.get('data', []))}")
            for e in enrichments.get("data", [])[:3]:
                etype = e.get("attributes", {}).get("type", "unknown")
                subtype = e.get("attributes", {}).get("subtype", "")
                print(f"  - {etype}/{subtype}")

            # Search - keywords (using JSON:API format)
            print("\nSearch: keywords=['main', 'func', 'package']")
            payload = {
                "data": {
                    "type": "search",
                    "attributes": {"keywords": ["main", "func", "package"], "limit": 5},
                }
            }
            resp = client.post(f"{BASE_URL}/api/v1/search", json=payload)
            search_results = resp.json()
            results["search_results"] = search_results.get("data", [])
            print(f"Search results count: {len(results['search_results'])}")
            for i, r in enumerate(results["search_results"]):
                content = r.get("attributes", {}).get("content", {})
                lang = content.get("language", "unknown")
                value = content.get("value", "")[:80]
                derives = r.get("attributes", {}).get("derives_from", [])
                paths = [d.get("path", "") for d in derives]
                print(f"  Result {i}: lang={lang}, paths={paths}")
                print(f"    preview: {value!r}...")

            # Code search
            print("\nSearch: code='func main() { fmt.Println }'")
            payload = {
                "data": {
                    "type": "search",
                    "attributes": {"code": "func main() { fmt.Println }", "limit": 5},
                }
            }
            resp = client.post(f"{BASE_URL}/api/v1/search", json=payload)
            code_results = resp.json().get("data", [])
            print(f"Code search results: {len(code_results)}")
            for i, r in enumerate(code_results):
                content = r.get("attributes", {}).get("content", {})
                value = content.get("value", "")[:80]
                print(f"  Result {i}: {value!r}...")

            # Cleanup
            client.delete(f"{BASE_URL}/api/v1/repositories/{repo_id}").raise_for_status()
            print("\nRepository deleted")

    except Exception as e:
        results["errors"].append(str(e))
        import traceback
        traceback.print_exc()
    finally:
        process.terminate()
        process.wait(timeout=10)
        tmpfile.unlink(missing_ok=True)

    # Output JSON results
    print("\n" + "=" * 60)
    print("GO RESULTS JSON:")
    print("=" * 60)
    print(json.dumps(results, indent=2, default=str))

    return results


if __name__ == "__main__":
    main()
