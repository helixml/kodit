#!/usr/bin/env python3
# /// script
# dependencies = [
#   "httpx",
#   "structlog",
# ]
# ///
"""Smoke tests for Kodit API endpoints."""

import os
import socket
import subprocess
import sys
import tempfile
import time
from collections.abc import Callable
from pathlib import Path
from typing import Any

import httpx
import structlog

BASE_HOST = "127.0.0.1"
BASE_PORT = 8080
BASE_URL = f"http://{BASE_HOST}:{BASE_PORT}"
TARGET_URI = "https://gist.github.com/7aa38185e20433c04c533f2b28f4e217.git"

log = structlog.get_logger(__name__)


def is_port_available(host: str, port: int) -> bool:
    """Check if a port is available by trying to bind to it."""
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        try:
            s.bind((host, port))
        except OSError:
            return False  # Port is in use
        else:
            return True


def retry_with_timeout(
    func: Callable[..., bool], timeout: float = 600, retry_delay: float = 1
) -> Any:
    """Keep trying a function until it succeeds or timeout is reached."""
    start_time = time.time()

    while time.time() - start_time < timeout:
        try:
            if func():
                return
        except Exception as e:
            if time.time() - start_time >= timeout:
                raise TimeoutError(f"Failed after {timeout} seconds: {e}") from e

        time.sleep(retry_delay)  # Wait before retrying

    raise TimeoutError(f"Timed out after {timeout} seconds")


def main() -> None:  # noqa: PLR0915
    """Run smoke tests."""
    # Check there is nothing running on port 8080 already:
    if not is_port_available(BASE_HOST, BASE_PORT):
        log.error("Port is already in use", host=BASE_HOST, port=BASE_PORT)
        sys.exit(1)

    # Create a temporary file for the environment file so it doesn't pull any settings
    # from the local .env file
    with tempfile.NamedTemporaryFile(delete=False) as f:
        tmpfile = Path(f.name)

    env = os.environ.copy()
    env.update({"DISABLE_TELEMETRY": "true", "DB_URL": "sqlite+aiosqlite:///:memory:"})
    if "SMOKE_DB_URL" in env:
        env.update({"DB_URL": env["SMOKE_DB_URL"]})

    prefix = [] if os.getenv("CI") else ["uv", "run"]
    cmd = [
        *prefix,
        "kodit",
        "--env-file",
        str(tmpfile),
        "serve",
        "--host",
        "127.0.0.1",
        "--port",
        "8080",
    ]
    process = subprocess.Popen(cmd, env=env)  # noqa: S603

    try:
        with httpx.Client(timeout=30.0) as client:
            log.info("Waiting for server to start listening", port=BASE_PORT)
            while is_port_available(BASE_HOST, BASE_PORT):
                time.sleep(1)
            client.get(f"{BASE_URL}/healthz").raise_for_status()  # cspell: disable-line
            log.info("Server health check passed")

            log.info("Testing repository lifecycle")
            resp = client.get(f"{BASE_URL}/api/v1/repositories")
            repos = resp.json()
            log.info("Listed existing repositories", count=len(repos["data"]))

            payload = {
                "data": {"type": "repository", "attributes": {"remote_uri": TARGET_URI}}
            }
            client.post(
                f"{BASE_URL}/api/v1/repositories", json=payload
            ).raise_for_status()
            log.info("Created repository", uri=TARGET_URI)

            resp = client.get(f"{BASE_URL}/api/v1/repositories")
            repos = resp.json()
            repo_id = repos["data"][0]["id"]
            log.info("Retrieved repository ID", repo_id=repo_id)

            log.info("Testing repository endpoints", repo_id=repo_id)
            client.get(f"{BASE_URL}/api/v1/repositories/{repo_id}").raise_for_status()
            client.get(
                f"{BASE_URL}/api/v1/repositories/{repo_id}/status"
            ).raise_for_status()

            log.info("Waiting for indexing to complete", repo_id=repo_id)

            def indexing_finished() -> bool:
                response = client.get(
                    f"{BASE_URL}/api/v1/repositories/{repo_id}/status"
                )
                status = response.json()
                log.info("Indexing status", status=status)
                return (
                    all(
                        task["attributes"]["state"] == "completed"
                        or task["attributes"]["state"] == "skipped"
                        for task in status["data"]
                    )
                    and len(status["data"]) > 5
                )

            retry_with_timeout(indexing_finished)
            log.info("Indexing completed", repo_id=repo_id)

            log.info("Testing tags", repo_id=repo_id)
            resp = client.get(f"{BASE_URL}/api/v1/repositories/{repo_id}/tags")
            tags = resp.json()
            log.info("Retrieved tags", count=len(tags["data"]))
            if tags["data"]:
                tag_id = tags["data"][0]["id"]
                tag_url = f"{BASE_URL}/api/v1/repositories/{repo_id}/tags/{tag_id}"
                client.get(tag_url).raise_for_status()
                log.info("Retrieved tag details", tag_id=tags["data"][0]["id"])

            log.info("Testing commits", repo_id=repo_id)
            resp = client.get(f"{BASE_URL}/api/v1/repositories/{repo_id}/commits")
            commits = resp.json()
            log.info("Retrieved commits", count=len(commits["data"]))
            if commits["data"]:
                commit_sha = commits["data"][0]["attributes"]["commit_sha"]
                commit_url = (
                    f"{BASE_URL}/api/v1/repositories/{repo_id}/commits/{commit_sha}"
                )
                client.get(commit_url).raise_for_status()
                log.info("Retrieved commit details", commit_sha=commit_sha)

                resp = client.get(f"{commit_url}/files")
                files = resp.json()
                log.info("Retrieved commit files", count=len(files["data"]))
                if files["data"]:
                    blob_sha = files["data"][0]["attributes"]["blob_sha"]
                    client.get(f"{commit_url}/files/{blob_sha}").raise_for_status()
                    log.info("Retrieved file content", blob_sha=blob_sha)

                assert client.get(f"{commit_url}/snippets").is_redirect
                log.info("Retrieved snippets", commit_sha=commit_sha)
                client.get(f"{commit_url}/enrichments").raise_for_status()
                log.info("Retrieved enrichments", commit_sha=commit_sha)
                client.get(f"{commit_url}/embeddings?full=false").raise_for_status()
                log.info("Retrieved embeddings", commit_sha=commit_sha)

            log.info("Testing search API")
            payload = {
                "data": {
                    "type": "search",
                    "attributes": {"keywords": ["test"], "code": "def", "limit": 5},
                }
            }
            client.post(f"{BASE_URL}/api/v1/search", json=payload).raise_for_status()
            log.info("Search completed successfully")

            log.info("Testing queue API")
            client.get(f"{BASE_URL}/api/v1/queue").raise_for_status()
            log.info("Queue API responded successfully")

            log.info("Testing repository deletion", repo_id=repo_id)
            client.delete(
                f"{BASE_URL}/api/v1/repositories/{repo_id}"
            ).raise_for_status()
            log.info("Repository deleted successfully", repo_id=repo_id)
            log.info("All smoke tests passed successfully")

    finally:
        process.terminate()
        process.wait(timeout=10)
        tmpfile.unlink(missing_ok=True)


if __name__ == "__main__":
    main()
