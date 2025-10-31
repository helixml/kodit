#!/usr/bin/env python3
# /// script
# dependencies = [
#   "httpx",
#   "structlog",
#   "aiosqlite",
# ]
# ///
"""Test that indexing operations are idempotent.

This test verifies that running the indexing process multiple times
on the same repository does not create duplicate database entries.
"""

import os
import socket
import subprocess
import sys
import tempfile
import time
from pathlib import Path

import httpx
import structlog

BASE_HOST = "127.0.0.1"
BASE_PORT = 8082
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
    func,  # noqa: ANN001
    timeout: float = 600,
    retry_delay: float = 1,
) -> None:
    """Keep trying a function until it succeeds or timeout is reached."""
    start_time = time.time()

    while time.time() - start_time < timeout:
        try:
            if func():
                return
        except Exception as e:
            if time.time() - start_time >= timeout:
                raise TimeoutError(f"Failed after {timeout} seconds: {e}") from e

        time.sleep(retry_delay)

    raise TimeoutError(f"Timed out after {timeout} seconds")


def get_database_counts(db_path: str) -> dict[str, int]:
    """Get row counts for all relevant tables."""
    import sqlite3

    tables = [
        "git_repos",
        "git_commits",
        "git_branches",
        "git_tags",
        "git_commit_files",
        "embeddings",
        "enrichments_v2",
        "enrichment_associations",
    ]

    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()

    counts = {}
    try:
        for table in tables:
            cursor.execute(f"SELECT COUNT(*) FROM {table}")  # noqa: S608
            counts[table] = cursor.fetchone()[0]
    finally:
        conn.close()

    return counts


def main() -> None:  # noqa: PLR0915
    """Run idempotency test for repository indexing."""
    # Check there is nothing running on the port already
    if not is_port_available(BASE_HOST, BASE_PORT):
        log.error("Port is already in use", host=BASE_HOST, port=BASE_PORT)
        sys.exit(1)

    # Create a temporary database file
    db_fd, db_path = tempfile.mkstemp(suffix=".db")
    os.close(db_fd)
    db_path_obj = Path(db_path)

    # Create a temporary file for the environment file
    with tempfile.NamedTemporaryFile(delete=False) as f:
        tmpfile = Path(f.name)

    env = os.environ.copy()
    env.update({
        "DISABLE_TELEMETRY": "true",
        "DB_URL": f"sqlite+aiosqlite:///{db_path}",
    })

    prefix = [] if os.getenv("CI") else ["uv", "run"]
    cmd = [
        *prefix,
        "kodit",
        "--env-file",
        str(tmpfile),
        "serve",
        "--host",
        BASE_HOST,
        "--port",
        str(BASE_PORT),
    ]
    process = subprocess.Popen(cmd, env=env)  # noqa: S603

    try:
        with httpx.Client(timeout=30.0) as client:
            log.info("Waiting for server to start listening", port=BASE_PORT)
            while is_port_available(BASE_HOST, BASE_PORT):
                time.sleep(1)
            client.get(f"{BASE_URL}/healthz").raise_for_status()  # cspell: disable-line
            log.info("Server health check passed")

            # Create and index a repository
            log.info("Creating repository", uri=TARGET_URI)
            payload = {
                "data": {"type": "repository", "attributes": {"remote_uri": TARGET_URI}}
            }
            client.post(
                f"{BASE_URL}/api/v1/repositories", json=payload
            ).raise_for_status()
            log.info("Repository created successfully")

            # Get the repository ID
            response = client.get(f"{BASE_URL}/api/v1/repositories")
            repos = response.json()
            repo_id = repos["data"][0]["id"]
            log.info("Retrieved repository ID", repo_id=repo_id)

            # Wait for first indexing to complete
            log.info("Waiting for first indexing to complete", repo_id=repo_id)

            def indexing_finished() -> bool:
                response = client.get(
                    f"{BASE_URL}/api/v1/repositories/{repo_id}/status"
                )
                status = response.json()
                return (
                    all(
                        task["attributes"]["state"] == "completed"
                        or task["attributes"]["state"] == "skipped"
                        for task in status["data"]
                    )
                    and len(status["data"]) > 5
                )

            retry_with_timeout(indexing_finished)
            log.info("First indexing completed", repo_id=repo_id)

            # Get database counts after first indexing
            first_counts = get_database_counts(db_path)
            log.info("Database counts after first indexing", counts=first_counts)

            # Try to create the same repository again
            # This should return 200 OK and trigger re-indexing
            log.info("Attempting to create the same repository again", uri=TARGET_URI)
            response = client.post(
                f"{BASE_URL}/api/v1/repositories", json=payload
            )
            if response.status_code != 200:
                log.error(
                    "Expected 200 OK when re-indexing existing repository",
                    status_code=response.status_code,
                )
                raise AssertionError(
                    f"Expected 200 OK for duplicate POST, got {response.status_code}"
                )
            log.info("Received expected 200 OK response for duplicate POST")

            # Get database counts after attempting to create duplicate
            second_counts = get_database_counts(db_path)
            log.info("Database counts after duplicate POST", counts=second_counts)

            # Verify that counts have not increased (idempotency)
            log.info("Verifying idempotency - all tables should have same counts")
            errors = []
            for table in first_counts:
                if second_counts[table] != first_counts[table]:
                    msg = (
                        f"Table {table} count changed from "
                        f"{first_counts[table]} to {second_counts[table]} "
                        f"- this indicates duplicate data was created!"
                    )
                    log.error(msg)
                    errors.append(msg)
                else:
                    log.info(
                        "Table count unchanged (correct)",
                        table=table,
                        count=first_counts[table],
                    )

            if errors:
                log.error("Idempotency test failed", errors=errors)
                raise AssertionError("\n".join(errors))

            log.info("Idempotency test passed successfully")

    finally:
        process.terminate()
        process.wait(timeout=10)
        tmpfile.unlink(missing_ok=True)
        db_path_obj.unlink(missing_ok=True)


if __name__ == "__main__":
    main()
