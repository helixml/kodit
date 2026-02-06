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

    # Check required API keys
    embedding_api_key = os.environ.get("EMBEDDING_ENDPOINT_API_KEY", "")
    enrichment_api_key = os.environ.get("ENRICHMENT_ENDPOINT_API_KEY", "")
    if not embedding_api_key:
        print("ERROR: EMBEDDING_ENDPOINT_API_KEY environment variable is required")
        sys.exit(1)
    if not enrichment_api_key:
        print("ERROR: ENRICHMENT_ENDPOINT_API_KEY environment variable is required")
        sys.exit(1)

    with tempfile.NamedTemporaryFile(delete=False) as f:
        tmpfile = Path(f.name)

    env = os.environ.copy()
    env.update({
        "DISABLE_TELEMETRY": "true",
        "DB_URL": "sqlite+aiosqlite:///:memory:",
        # Embedding provider (OpenRouter with Codestral Embed via LiteLLM)
        # Use LiteLLM's native openrouter/ prefix (avoids encoding_format issue)
        "EMBEDDING_ENDPOINT_MODEL": "openrouter/mistralai/codestral-embed-2505",
        "EMBEDDING_ENDPOINT_API_KEY": embedding_api_key,
        # Enrichment provider (OpenRouter with Ministral 8B via LiteLLM)
        "ENRICHMENT_ENDPOINT_MODEL": "openrouter/mistralai/ministral-8b-2512",
        "ENRICHMENT_ENDPOINT_API_KEY": enrichment_api_key,
    })
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
        stderr=subprocess.STDOUT,  # Merge stderr into stdout
    )

    results = {
        "implementation": "python",
        "repository": TARGET_URI,
        "task_status": [],
        "files": [],
        "snippets": [],
        "enrichments": [],
        "keyword_search": [],
        "code_search": [],
        "errors": [],
    }

    def read_output():
        """Read any available output from the process."""
        if process.stdout:
            import select
            while select.select([process.stdout], [], [], 0)[0]:
                line = process.stdout.readline()
                if line:
                    print(f"[PY SERVER] {line.decode().rstrip()}")
                else:
                    break

    try:
        with httpx.Client(timeout=30.0) as client:
            print("Waiting for Python server to start...")
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
            print("Python server health check passed")

            # Create repository (Python uses JSON:API format)
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
                read_output()
                response = client.get(f"{BASE_URL}/api/v1/repositories/{repo_id}/status")
                status = response.json()
                terminal = {"completed", "skipped", "failed"}
                tasks = status.get("data", [])
                if len(tasks) < 5:
                    pending = sum(1 for t in tasks if t["attributes"]["state"] == "pending")
                    running = sum(1 for t in tasks if t["attributes"]["state"] in ("running", "started", "in_progress"))
                    print(f"  ... tasks={len(tasks)}, pending={pending}, running={running}")
                    return False
                all_done = all(t["attributes"]["state"] in terminal for t in tasks)
                if not all_done:
                    pending = sum(1 for t in tasks if t["attributes"]["state"] == "pending")
                    running = sum(1 for t in tasks if t["attributes"]["state"] in ("running", "started", "in_progress"))
                    print(f"  ... tasks={len(tasks)}, pending={pending}, running={running}")
                return all_done

            if not wait_for_condition(indexing_finished, timeout=600):
                results["errors"].append("Indexing timed out")
                return results

            print("Indexing completed")

            # Show final task status
            resp = client.get(f"{BASE_URL}/api/v1/repositories/{repo_id}/status")
            task_status = resp.json()
            print("\n=== TASK STATUS ===")
            for task in task_status.get("data", []):
                step = task["attributes"].get("step", "unknown")
                state = task["attributes"].get("state", "unknown")
                error = task["attributes"].get("error", "")
                line = f"  {step}: {state}" + (f" (error: {error})" if error else "")
                print(line)
                results["task_status"].append({"step": step, "state": state, "error": error})

            # Get commits
            resp = client.get(f"{BASE_URL}/api/v1/repositories/{repo_id}/commits")
            commits = resp.json()
            if not commits["data"]:
                results["errors"].append("No commits found")
                return results

            commit_sha = commits["data"][0]["attributes"]["commit_sha"]
            commit_url = f"{BASE_URL}/api/v1/repositories/{repo_id}/commits/{commit_sha}"
            print(f"\nCommit SHA: {commit_sha}")

            # Get files
            resp = client.get(f"{commit_url}/files")
            files = resp.json()
            print(f"\n=== FILES ({len(files['data'])}) ===")
            for f in sorted(files["data"], key=lambda x: x["attributes"]["path"]):
                path = f["attributes"]["path"]
                size = f["attributes"]["size"]
                print(f"  {path} ({size} bytes)")
                results["files"].append({"path": path, "size": size})

            # Get snippets (Python redirects to enrichments endpoint)
            resp = client.get(f"{commit_url}/snippets", follow_redirects=True)
            if resp.status_code == 200:
                snippets = resp.json()
                snippet_list = snippets.get("data", [])
                # Python returns enrichments (type=development, subtype=snippet)
                # Content is a plain string, not {value, language}
                print(f"\n=== SNIPPETS ({len(snippet_list)}) ===")
                for i, s in enumerate(snippet_list):
                    attrs = s.get("attributes", {})
                    content = attrs.get("content", "")
                    # Python enrichment content is a string
                    if isinstance(content, dict):
                        value = content.get("value", "")
                        lang = content.get("language", "unknown")
                    else:
                        value = str(content)
                        lang = "unknown"
                    lines = value.count("\n") + 1
                    preview = value[:120].replace("\n", "\\n")
                    print(f"  [{i}] lang={lang}, lines={lines}")
                    print(f"      {preview!r}")
                    results["snippets"].append({
                        "language": lang,
                        "lines": lines,
                        "content": value,
                    })
            else:
                results["errors"].append(f"Snippets returned status {resp.status_code}")

            # Get enrichments
            resp = client.get(f"{commit_url}/enrichments")
            enrichments = resp.json()
            enrichment_list = enrichments.get("data", [])
            print(f"\n=== ENRICHMENTS ({len(enrichment_list)}) ===")
            for e in enrichment_list:
                etype = e.get("attributes", {}).get("type", "unknown")
                subtype = e.get("attributes", {}).get("subtype", "")
                print(f"  {etype}/{subtype}")
                results["enrichments"].append({"type": etype, "subtype": subtype})

            # Keyword search
            print("\n=== KEYWORD SEARCH: ['main', 'func', 'package'] ===")
            payload = {
                "data": {
                    "type": "search",
                    "attributes": {"keywords": ["main", "func", "package"], "limit": 5},
                }
            }
            resp = client.post(f"{BASE_URL}/api/v1/search", json=payload)
            kw_results = resp.json().get("data", [])
            print(f"Results: {len(kw_results)}")
            for i, r in enumerate(kw_results):
                content = r.get("attributes", {}).get("content", {})
                lang = content.get("language", "unknown")
                value = content.get("value", "")
                derives = r.get("attributes", {}).get("derives_from", [])
                paths = [d.get("path", "") for d in derives]
                preview = value[:120].replace("\n", "\\n")
                print(f"  [{i}] lang={lang}, paths={paths}")
                print(f"      {preview!r}")
                results["keyword_search"].append({
                    "language": lang,
                    "paths": paths,
                    "content": value,
                })

            # Code search
            print("\n=== CODE SEARCH: 'func main() { fmt.Println }' ===")
            payload = {
                "data": {
                    "type": "search",
                    "attributes": {"code": "func main() { fmt.Println }", "limit": 5},
                }
            }
            resp = client.post(f"{BASE_URL}/api/v1/search", json=payload)
            code_results = resp.json().get("data", [])
            print(f"Results: {len(code_results)}")
            for i, r in enumerate(code_results):
                content = r.get("attributes", {}).get("content", {})
                lang = content.get("language", "unknown")
                value = content.get("value", "")
                derives = r.get("attributes", {}).get("derives_from", [])
                paths = [d.get("path", "") for d in derives]
                preview = value[:120].replace("\n", "\\n")
                print(f"  [{i}] lang={lang}, paths={paths}")
                print(f"      {preview!r}")
                results["code_search"].append({
                    "language": lang,
                    "paths": paths,
                    "content": value,
                })

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
