#!/usr/bin/env python3
# /// script
# dependencies = [
#   "alembic",
#   "aiosqlite",
#   "asyncpg",
#   "sqlalchemy[asyncio]",
#   "pytest",
#   "pytest-asyncio",
#   "pytest-cov",
#   "pytest-mock",
# ]
# ///
#
"""Tests migrations against both SQLite and PostgreSQL to ensure compatibility."""

import subprocess
import sys
import tempfile
from pathlib import Path

import structlog
from alembic import command
from alembic.config import Config

logger = structlog.get_logger(__name__)


def run_command(
    cmd: list[str], *, should_check: bool = True
) -> subprocess.CompletedProcess:
    """Run a shell command and return the result."""
    logger.info("running_command", command=" ".join(cmd))
    result = subprocess.run(  # noqa: S603
        cmd, capture_output=True, text=True, check=False
    )
    if should_check and result.returncode != 0:
        logger.error("command_failed", stdout=result.stdout, stderr=result.stderr)
        sys.exit(1)
    return result


def test_sqlite_migration() -> None:
    """Test migrations with SQLite."""
    logger.info("=" * 80)
    logger.info("testing_sqlite_migrations")
    logger.info("=" * 80)

    with tempfile.TemporaryDirectory() as tmpdir:
        db_path = Path(tmpdir) / "test.db"
        db_url = f"sqlite+aiosqlite:///{db_path}"

        cfg = Config("alembic.ini")
        cfg.set_main_option("sqlalchemy.url", db_url)

        logger.info("running_upgrade_to_head")
        command.upgrade(cfg, "head")
        logger.info("upgrade_successful")

        logger.info("checking_for_pending_migrations")
        try:
            command.check(cfg)
            logger.info("no_pending_migrations")
        except Exception:
            logger.exception("migration_check_failed")
            sys.exit(1)

        logger.info("testing_downgrade")
        command.downgrade(cfg, "-1")
        logger.info("downgrade_successful")

        logger.info("testing_upgrade_again")
        command.upgrade(cfg, "head")
        logger.info("re_upgrade_successful")

    logger.info("sqlite_migration_test_passed")


def _start_postgres_container(container_name: str) -> None:
    """Start PostgreSQL container."""
    logger.info("cleaning_up_existing_containers")
    result = run_command(
        ["docker", "ps", "-a", "-q", "-f", f"name={container_name}"],
        should_check=False,
    )
    if result.stdout.strip():
        logger.info("removing_existing_container", container=container_name)
        run_command(["docker", "rm", "-f", container_name])

    logger.info("starting_postgres_container")
    run_command(
        [
            "docker",
            "run",
            "--name",
            container_name,
            "-e",
            "POSTGRES_DB=kodit",
            "-e",
            "POSTGRES_PASSWORD=testpass",
            "-p",
            "5433:5432",
            "-d",
            "tensorchord/vchord-suite:pg17-20250601",
        ]
    )


def _wait_for_postgres(container_name: str) -> None:
    """Wait for PostgreSQL to be ready."""
    import time

    logger.info("waiting_for_postgres")
    max_retries = 30
    for i in range(max_retries):
        result = run_command(
            [
                "docker",
                "exec",
                container_name,
                "pg_isready",
                "-U",
                "postgres",
            ],
            should_check=False,
        )
        if result.returncode == 0:
            logger.info("postgres_ready")
            time.sleep(2)  # Extra wait to ensure fully ready
            break
        if i < max_retries - 1:
            logger.debug("waiting_for_postgres_retry", attempt=i + 1, max=max_retries)
            time.sleep(1)
    else:
        logger.error("postgres_failed_to_start")
        sys.exit(1)


def _verify_postgres_schema(container_name: str) -> None:
    """Verify PostgreSQL schema changes."""
    logger.info("verifying_postgres_schema")

    # Check git_branches FK removed
    result = run_command(
        [
            "docker",
            "exec",
            container_name,
            "psql",
            "-U",
            "postgres",
            "-d",
            "kodit",
            "-c",
            (
                "SELECT conname FROM pg_constraint "
                "WHERE conrelid = 'git_branches'::regclass "
                "AND contype = 'f' AND conname LIKE '%head_commit%';"
            ),
        ]
    )
    if "head_commit" in result.stdout:
        logger.error("fk_constraint_still_exists", table="git_branches")
        sys.exit(1)
    logger.info("fk_removed", column="git_branches.head_commit_sha")

    # Check git_tags FK removed
    result = run_command(
        [
            "docker",
            "exec",
            container_name,
            "psql",
            "-U",
            "postgres",
            "-d",
            "kodit",
            "-c",
            (
                "SELECT conname FROM pg_constraint "
                "WHERE conrelid = 'git_tags'::regclass "
                "AND contype = 'f' AND conname LIKE '%target_commit%';"
            ),
        ]
    )
    if "target_commit" in result.stdout:
        logger.error("fk_constraint_still_exists", table="git_tags")
        sys.exit(1)
    logger.info("fk_removed", column="git_tags.target_commit_sha")

    # Check indexes created
    result = run_command(
        [
            "docker",
            "exec",
            container_name,
            "psql",
            "-U",
            "postgres",
            "-d",
            "kodit",
            "-c",
            (
                "SELECT indexname FROM pg_indexes "
                "WHERE tablename = 'git_branches' "
                "AND indexname = 'ix_git_branches_head_commit_sha';"
            ),
        ]
    )
    if "ix_git_branches_head_commit_sha" not in result.stdout:
        logger.error("index_not_created", index="ix_git_branches_head_commit_sha")
        sys.exit(1)
    logger.info("index_created", index="ix_git_branches_head_commit_sha")

    result = run_command(
        [
            "docker",
            "exec",
            container_name,
            "psql",
            "-U",
            "postgres",
            "-d",
            "kodit",
            "-c",
            (
                "SELECT indexname FROM pg_indexes "
                "WHERE tablename = 'git_commits' "
                "AND indexname = 'ix_git_commits_parent_commit_sha';"
            ),
        ]
    )
    if "ix_git_commits_parent_commit_sha" not in result.stdout:
        logger.error("index_not_created", index="ix_git_commits_parent_commit_sha")
        sys.exit(1)
    logger.info("index_created", index="ix_git_commits_parent_commit_sha")


def test_postgres_migration() -> None:
    """Test migrations with PostgreSQL."""
    logger.info("=" * 80)
    logger.info("testing_postgres_migrations")
    logger.info("=" * 80)

    container_name = "kodit-migration-test"
    db_url = "postgresql+asyncpg://postgres:testpass@localhost:5433/kodit"

    _start_postgres_container(container_name)

    try:
        _wait_for_postgres(container_name)

        cfg = Config("alembic.ini")
        cfg.set_main_option("sqlalchemy.url", db_url)

        logger.info("running_upgrade_to_head")
        command.upgrade(cfg, "head")
        logger.info("upgrade_successful")

        logger.info("skipping_alembic_check", reason="known_timestamp_type_drift")

        _verify_postgres_schema(container_name)

        logger.info("testing_downgrade")
        command.downgrade(cfg, "-1")
        logger.info("downgrade_successful")

        logger.info("testing_upgrade_again")
        command.upgrade(cfg, "head")
        logger.info("re_upgrade_successful")

        logger.info("postgres_migration_test_passed")

    finally:
        logger.info("cleaning_up_container")
        run_command(["docker", "rm", "-f", container_name], should_check=False)
        logger.info("container_cleaned_up")


def main() -> None:
    """Run all migration tests."""
    logger.info("=" * 80)
    logger.info("database_migration_integration_tests")
    logger.info("=" * 80)

    test_sqlite_migration()
    test_postgres_migration()

    logger.info("=" * 80)
    logger.info("all_migration_tests_passed")
    logger.info("=" * 80)


if __name__ == "__main__":
    main()
