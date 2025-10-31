#!/usr/bin/env python3
# /// script
# dependencies = [
#   "httpx",
#   "structlog",
#   "aiosqlite",
# ]
# ///
"""Smoke test for repository deletion functionality.

This test verifies that when a repository is deleted via the API,
all associated data is properly removed from the database.
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
BASE_PORT = 8081
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


def verify_database_empty(db_path: str) -> None:
    """Verify that all repository-related tables are empty using sqlite3."""
    import sqlite3

    tables_to_check = [
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

    try:
        for table in tables_to_check:
            cursor.execute(f"SELECT COUNT(*) FROM {table}")  # noqa: S608
            count = cursor.fetchone()[0]
            log.info("Checked table after deletion", table=table, count=count)
            if count > 0:
                log.error("Table not empty after deletion", table=table, count=count)
                msg = f"Table {table} still has {count} rows after deletion"
                raise AssertionError(msg)

        log.info("All tables are empty after deletion")
    finally:
        conn.close()


def main() -> None:  # noqa: PLR0915
    """Run smoke test for repository deletion."""
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

            # Create a repository
            log.info("Creating repository", uri=TARGET_URI)
            payload = {
                "data": {"type": "repository", "attributes": {"remote_uri": TARGET_URI}}
            }
            client.post(
                f"{BASE_URL}/api/v1/repositories", json=payload
            ).raise_for_status()
            log.info("Repository created successfully")

            # Get the repository ID  # cspell: disable-line
            response = client.get(f"{BASE_URL}/api/v1/repositories")
            repos = response.json()
            repo_id = repos["data"][0]["id"]
            log.info("Retrieved repository ID", repo_id=repo_id)

            # Wait for indexing to complete
            log.info("Waiting for indexing to complete", repo_id=repo_id)

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
            log.info("Indexing completed", repo_id=repo_id)

            # Verify data exists before deletion
            log.info("Verifying data exists before deletion")
            import sqlite3
            conn = sqlite3.connect(db_path)
            cursor = conn.cursor()
            cursor.execute("SELECT COUNT(*) FROM git_repos")
            repo_count = cursor.fetchone()[0]
            cursor.execute("SELECT COUNT(*) FROM git_commits")
            commit_count = cursor.fetchone()[0]
            cursor.execute("SELECT COUNT(*) FROM git_branches")
            branch_count = cursor.fetchone()[0]
            conn.close()

            log.info(
                "Database state before deletion",
                repos=repo_count,
                commits=commit_count,
                branches=branch_count,
            )

            if repo_count == 0:
                log.error("No repository found before deletion")
                raise AssertionError("No repository found before deletion")

            # Delete the repository
            log.info("Deleting repository", repo_id=repo_id)
            client.delete(
                f"{BASE_URL}/api/v1/repositories/{repo_id}"
            ).raise_for_status()
            log.info("Repository deleted successfully", repo_id=repo_id)

            # Verify all data is deleted
            log.info("Verifying all data is deleted from database")
            verify_database_empty(db_path)

            log.info("Deletion smoke test passed successfully")

    finally:
        process.terminate()
        process.wait(timeout=10)
        tmpfile.unlink(missing_ok=True)
        db_path_obj.unlink(missing_ok=True)


if __name__ == "__main__":
    main()
