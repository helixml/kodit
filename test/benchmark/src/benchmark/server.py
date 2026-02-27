"""Server management for benchmark operations."""

import contextlib
import gzip
import os
import shutil
import signal
import subprocess
import sys
import time
from pathlib import Path

import httpx
import structlog

DEFAULT_PID_FILE = Path("/tmp/kodit-benchmark.pid")
DEFAULT_STARTUP_TIMEOUT = 60

# Relative path from test/benchmark/ to the project build directory
_PROJECT_BUILD_DIR = Path(__file__).resolve().parents[4] / "build"


def _kodit_binary() -> str:
    """Locate the kodit binary on PATH or in the project build directory."""
    on_path = shutil.which("kodit")
    if on_path:
        return on_path
    candidate = _PROJECT_BUILD_DIR / "kodit"
    if candidate.is_file():
        return str(candidate)
    msg = (
        "kodit binary not found. Either add it to PATH or run "
        "'make build' from the project root."
    )
    raise FileNotFoundError(msg)


_COMPOSE_DB_URL = "postgresql://postgres:mysecretpassword@127.0.0.1:5432/kodit"  # noqa: S105

# Project root and compose file paths
_PROJECT_ROOT = Path(__file__).resolve().parents[4]
_COMPOSE_DEV = _PROJECT_ROOT / "docker-compose.dev.yaml"
_COMPOSE_BENCHMARK = (
    _PROJECT_ROOT / "test" / "benchmark" / "docker-compose.benchmark.yaml"
)


class ComposeDatabase:
    """Manages the vectorchord database via docker compose."""

    def __init__(self) -> None:
        """Initialize compose database manager."""
        self._log = structlog.get_logger(__name__)

    @property
    def db_url(self) -> str:
        """Return the database URL for the compose-managed database."""
        return _COMPOSE_DB_URL

    def start(self) -> bool:
        """Tear down any existing service and start a fresh one."""
        self._log.info("Starting vectorchord via docker compose")
        if not self._compose("down", "-v"):
            return False
        return self._compose("up", "-d", "--wait")

    def stop(self) -> bool:
        """Stop and remove the compose-managed database."""
        self._log.info("Stopping vectorchord via docker compose")
        return self._compose("down", "-v")

    def dump(self, path: Path) -> bool:
        """Dump the database to a gzip-compressed PostgreSQL tar file."""
        self._log.info("Dumping database", path=str(path))
        path.parent.mkdir(parents=True, exist_ok=True)
        cmd = [
            "docker",
            "compose",  # noqa: S607
            "-f",
            str(_COMPOSE_DEV),
            "-f",
            str(_COMPOSE_BENCHMARK),
            "--profile",
            "vectorchord",
            "exec",
            "-T",
            "vectorchord",
            "pg_dump",
            "-U",
            "postgres",
            "kodit",
            "-F",
            "t",
        ]
        try:
            result = subprocess.run(  # noqa: S603
                cmd,
                capture_output=True,
                check=True,
                cwd=str(_PROJECT_ROOT),
            )
        except subprocess.CalledProcessError as exc:
            self._log.error(
                "Database dump failed",
                stderr=exc.stderr.decode(errors="replace") if exc.stderr else "",
            )
            return False

        compressed = gzip.compress(result.stdout)
        path.write_bytes(compressed)
        self._log.info(
            "Database dumped",
            path=str(path),
            raw_size=len(result.stdout),
            compressed_size=len(compressed),
        )
        return True

    def restore(self, path: Path) -> bool:
        """Restore the database from a (possibly gzipped) PostgreSQL tar file."""
        self._log.info("Restoring database", path=str(path))
        raw = path.read_bytes()
        # Transparently decompress gzip files (magic bytes 1f 8b)
        if raw[:2] == b"\x1f\x8b":
            data = gzip.decompress(raw)
        else:
            data = raw
        cmd = [
            "docker",
            "compose",  # noqa: S607
            "-f",
            str(_COMPOSE_DEV),
            "-f",
            str(_COMPOSE_BENCHMARK),
            "--profile",
            "vectorchord",
            "exec",
            "-T",
            "vectorchord",
            "pg_restore",
            "-U",
            "postgres",
            "-d",
            "kodit",
            "--clean",
            "--if-exists",
            "--no-owner",
        ]
        result = subprocess.run(  # noqa: S603
            cmd,
            input=data,
            capture_output=True,
            check=False,
            cwd=str(_PROJECT_ROOT),
        )
        # pg_restore returns 0 on success, 1 on warnings (e.g. "relation does
        # not exist" during --clean).  Only exit code >= 2 is a hard failure.
        if result.returncode >= 2:
            self._log.error(
                "Database restore failed",
                returncode=result.returncode,
                stderr=result.stderr.decode(errors="replace") if result.stderr else "",
            )
            return False
        self._log.info("Database restored", path=str(path))
        return self._wait_for_ready()

    def _wait_for_ready(self, timeout: int = 30) -> bool:
        """Wait for postgres to accept connections after a restore."""
        deadline = time.monotonic() + timeout
        cmd = [
            "docker",
            "compose",  # noqa: S607
            "-f",
            str(_COMPOSE_DEV),
            "-f",
            str(_COMPOSE_BENCHMARK),
            "--profile",
            "vectorchord",
            "exec",
            "-T",
            "vectorchord",
            "pg_isready",
            "-U",
            "postgres",
            "-d",
            "kodit",
        ]
        while time.monotonic() < deadline:
            result = subprocess.run(  # noqa: S603
                cmd,
                capture_output=True,
                check=False,
                cwd=str(_PROJECT_ROOT),
            )
            if result.returncode == 0:
                return True
            time.sleep(0.5)
        self._log.error("Database not ready after restore", timeout=timeout)
        return False

    def _compose(self, *args: str) -> bool:
        """Run a docker compose command targeting the vectorchord profile."""
        cmd = [
            "docker",
            "compose",  # noqa: S607
            "-f",
            str(_COMPOSE_DEV),
            "-f",
            str(_COMPOSE_BENCHMARK),
            "--profile",
            "vectorchord",
            *args,
        ]
        self._log.debug("Running compose command", cmd=" ".join(cmd))
        result = subprocess.run(  # noqa: S603
            cmd,
            capture_output=True,
            text=True,
            check=False,
            cwd=str(_PROJECT_ROOT),
        )
        if result.returncode != 0:
            self._log.error(
                "Compose command failed",
                cmd=" ".join(cmd),
                stderr=result.stderr,
            )
            return False
        return True


class ServerProcess:
    """Manages the Kodit server process for benchmarking."""

    def __init__(  # noqa: PLR0913
        self,
        host: str,
        port: int,
        enrichment_base_url: str,
        enrichment_model: str,
        enrichment_api_key: str,
        enrichment_parallel_tasks: int,
        enrichment_timeout: int,
        embedding_base_url: str,
        embedding_model: str,
        embedding_api_key: str,
        embedding_parallel_tasks: int,
        embedding_timeout: int,
        pid_file: Path = DEFAULT_PID_FILE,
        startup_timeout: int = DEFAULT_STARTUP_TIMEOUT,
        extra_env: dict[str, str] | None = None,
    ) -> None:
        """Initialize server process manager."""
        self._host = host
        self._port = port
        self._pid_file = pid_file
        self._startup_timeout = startup_timeout
        self._db = ComposeDatabase()
        self._enrichment_base_url = enrichment_base_url
        self._enrichment_model = enrichment_model
        self._enrichment_api_key = enrichment_api_key
        self._enrichment_parallel_tasks = enrichment_parallel_tasks
        self._enrichment_timeout = enrichment_timeout
        self._embedding_base_url = embedding_base_url
        self._embedding_model = embedding_model
        self._embedding_api_key = embedding_api_key
        self._embedding_parallel_tasks = embedding_parallel_tasks
        self._embedding_timeout = embedding_timeout
        self._extra_env = extra_env or {}
        self._log = structlog.get_logger(__name__)

    @property
    def base_url(self) -> str:
        """Return the base URL for the Kodit server."""
        return f"http://{self._host}:{self._port}"

    @property
    def db(self) -> ComposeDatabase:
        """Return the database manager."""
        return self._db

    def start(self, restore_dump: Path | None = None) -> bool:
        """Start the database and Kodit server.

        If *restore_dump* is provided the database is restored from that
        PostgreSQL tar dump before the Kodit binary is launched.
        """
        if self._is_running():
            self._log.info("Server already running", pid=self._read_pid())
            return True

        # Start database first
        if not self._db.start():
            self._log.error("Failed to start database")
            return False

        # Restore a cached database dump when available
        if restore_dump is not None:
            if not self._db.restore(restore_dump):
                self._log.error("Failed to restore database dump")
                self._db.stop()
                return False

        self._log.info("Starting Kodit server", host=self._host, port=self._port)

        env = self._build_env()
        self._log.info("Using configuration", db_url=self._db.db_url)

        binary = _kodit_binary()
        self._log.info("Using kodit binary", path=binary)

        env["HOST"] = self._host
        env["PORT"] = str(self._port)

        process = subprocess.Popen(  # noqa: S603
            [binary, "serve"],
            env=env,
            stdout=sys.stdout,
            stderr=sys.stderr,
            start_new_session=True,
        )

        self._write_pid(process.pid)
        self._log.info("Server process started", pid=process.pid)

        if not self._wait_for_ready():
            self._log.error("Server failed to start within timeout")
            self.stop()
            return False

        self._log.info("Server is ready", url=self.base_url)
        return True

    def _build_env(self) -> dict[str, str]:
        """Build environment variables for the kodit process."""
        env = os.environ.copy()

        # Point Kodit at the project root .env for API keys
        project_env = _PROJECT_BUILD_DIR.parent / ".env"
        if project_env.is_file():
            env["KODIT_ENV_FILE"] = str(project_env)

        # Disable telemetry for benchmarks
        env["DISABLE_TELEMETRY"] = "true"

        # Point to the ORT library in the project lib/ directory
        project_lib = _PROJECT_BUILD_DIR.parent / "lib"
        if project_lib.is_dir() and "ORT_LIB_DIR" not in env:
            env["ORT_LIB_DIR"] = str(project_lib)

        # Database configuration
        env["DB_URL"] = self._db.db_url
        env["DEFAULT_SEARCH_PROVIDER"] = "vectorchord"

        # Disable periodic sync during benchmarks — the initial indexing is
        # sufficient and re-syncs cause spurious embedding failures.
        env["PERIODIC_SYNC_ENABLED"] = "false"

        # HTTP response caching for OpenRouter requests
        cache_dir = _PROJECT_ROOT / "test" / "benchmark" / ".http_cache"
        cache_dir.mkdir(parents=True, exist_ok=True)
        env["HTTP_CACHE_DIR"] = str(cache_dir)

        # Enrichment endpoint configuration (only override non-empty values
        # so that the project .env defaults can take effect)
        overrides = {
            "ENRICHMENT_ENDPOINT_BASE_URL": self._enrichment_base_url,
            "ENRICHMENT_ENDPOINT_MODEL": self._enrichment_model,
            "ENRICHMENT_ENDPOINT_API_KEY": self._enrichment_api_key,
            "ENRICHMENT_ENDPOINT_NUM_PARALLEL_TASKS": str(
                self._enrichment_parallel_tasks
            ),
            "ENRICHMENT_ENDPOINT_TIMEOUT": str(self._enrichment_timeout),
            "EMBEDDING_ENDPOINT_BASE_URL": self._embedding_base_url,
            "EMBEDDING_ENDPOINT_MODEL": self._embedding_model,
            "EMBEDDING_ENDPOINT_API_KEY": self._embedding_api_key,
            "EMBEDDING_ENDPOINT_NUM_PARALLEL_TASKS": str(
                self._embedding_parallel_tasks
            ),
            "EMBEDDING_ENDPOINT_TIMEOUT": str(self._embedding_timeout),
        }
        for key, value in overrides.items():
            if value:
                env[key] = value

        env.update(self._extra_env)

        return env

    def stop(self) -> bool:
        """Stop the Kodit server and database."""
        success = True
        pid = self._read_pid()

        if pid is not None:
            self._log.info("Stopping Kodit server", pid=pid)
            try:
                os.kill(pid, signal.SIGTERM)
                self._wait_for_shutdown(pid)
                self._log.info("Server stopped", pid=pid)
            except ProcessLookupError:
                self._log.info("Server process not found", pid=pid)
            except PermissionError:
                self._log.error("Permission denied stopping server", pid=pid)
                success = False
            finally:
                self._remove_pid_file()
        else:
            self._log.info("No server running (no PID file)")

        # Always try to stop the database
        if not self._db.stop():
            success = False

        return success

    def _is_running(self) -> bool:
        """Check if the server is currently running."""
        pid = self._read_pid()
        if pid is None:
            return False

        try:
            os.kill(pid, 0)
        except (ProcessLookupError, PermissionError):
            self._remove_pid_file()
            return False
        else:
            return True

    def _wait_for_ready(self) -> bool:
        """Wait for the server to respond to health checks."""
        deadline = time.monotonic() + self._startup_timeout
        health_url = f"{self.base_url}/healthz"
        attempt = 0

        self._log.info("Waiting for server to be ready", url=health_url)

        while time.monotonic() < deadline:
            attempt += 1
            try:
                response = httpx.get(health_url, timeout=2.0)
                if response.status_code == 200:
                    self._log.info("Server health check passed", attempts=attempt)
                    return True
                self._log.debug(
                    "Health check returned non-200", status=response.status_code
                )
            except httpx.RequestError as e:
                self._log.debug("Health check failed", attempt=attempt, error=str(e))
            time.sleep(0.5)

        self._log.error("Server failed to become ready", attempts=attempt)
        return False

    def _wait_for_shutdown(self, pid: int, timeout: int = 10) -> None:
        """Wait for the server process to terminate."""
        deadline = time.monotonic() + timeout

        while time.monotonic() < deadline:
            try:
                os.kill(pid, 0)
                time.sleep(0.1)
            except ProcessLookupError:
                return

        self._log.warning("Server did not terminate gracefully, sending SIGKILL")
        with contextlib.suppress(ProcessLookupError):
            os.kill(pid, signal.SIGKILL)

    def _read_pid(self) -> int | None:
        """Read the PID from the PID file."""
        try:
            content = self._pid_file.read_text().strip()
            return int(content)
        except (FileNotFoundError, ValueError):
            return None

    def _write_pid(self, pid: int) -> None:
        """Write the PID to the PID file."""
        self._pid_file.parent.mkdir(parents=True, exist_ok=True)
        self._pid_file.write_text(str(pid))

    def _remove_pid_file(self) -> None:
        """Remove the PID file."""
        with contextlib.suppress(FileNotFoundError):
            self._pid_file.unlink()
