#!/usr/bin/env python3
# /// script
# dependencies = [
#   "httpx",
# ]
# ///
"""Simplified smoke test for Python Kodit - outputs comparable results."""

import json
import os
import socket
import subprocess
import sys
import tempfile
import time
from pathlib import Path

BASE_HOST = "127.0.0.1"
BASE_PORT = 8081
BASE_URL = f"http://{BASE_HOST}:{BASE_PORT}"
# Use the same test repo as Go
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

    env = os.environ.copy()
    env.update({"DISABLE_TELEMETRY": "true", "DB_URL": "sqlite+aiosqlite:///:memory:"})
    if "SMOKE_DB_URL" in env:
        env["DB_URL"] = env["SMOKE_DB_URL"]

    prefix = [] if os.getenv("CI") else ["uv", "run"]
    cmd = [
        *prefix,
        "kodit",
        "--env-file", str(tmpfile),
        "serve",
        "--host", BASE_HOST,
        "--port", str(BASE_PORT),
    ]

    process = subprocess.Popen(
        cmd,
        env=env,
        cwd=str(Path(__file__).parent / "python-source"),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )

    results = {
        "implementation": "python",
        "repository": TARGET_URI,
        "snippets": [],
        "search_results": [],
        "errors": [],
    }

    try:
        with httpx.Client(timeout=30.0) as client:
            print("Waiting for Python server to start...")
            if not wait_for_condition(lambda: not port_available(BASE_HOST, BASE_PORT), timeout=60):
                results["errors"].append("Server failed to start")
                return results

            client.get(f"{BASE_URL}/healthz").raise_for_status()
            print("Python server health check passed")

            # Create repository
            payload = {
                "data": {"type": "repository", "attributes": {"remote_uri": TARGET_URI}}
            }
            client.post(f"{BASE_URL}/api/v1/repositories", json=payload).raise_for_status()
            print(f"Created repository: {TARGET_URI}")

            # Get repo ID
            resp = client.get(f"{BASE_URL}/api/v1/repositories")
            repos = resp.json()
            repo_id = repos["data"][0]["id"]
            print(f"Repository ID: {repo_id}")

            # Wait for indexing
            print("Waiting for indexing to complete...")

            def indexing_finished():
                response = client.get(f"{BASE_URL}/api/v1/repositories/{repo_id}/status")
                status = response.json()
                terminal = {"completed", "skipped", "failed"}
                tasks = status.get("data", [])
                if len(tasks) < 5:
                    return False
                return all(t["attributes"]["state"] in terminal for t in tasks)

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

            # Get snippets - Python returns a redirect
            resp = client.get(f"{commit_url}/snippets", follow_redirects=True)
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

            # Search - keywords
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
    print("PYTHON RESULTS JSON:")
    print("=" * 60)
    print(json.dumps(results, indent=2, default=str))

    return results


if __name__ == "__main__":
    main()
