"""Server management for benchmark operations."""

import contextlib
import os
import signal
import subprocess
import sys
import time
from pathlib import Path

import httpx
import structlog

DEFAULT_HOST = "127.0.0.1"
DEFAULT_PORT = 8765
DEFAULT_PID_FILE = Path("/tmp/kodit-benchmark.pid")
DEFAULT_STARTUP_TIMEOUT = 60

DEFAULT_DOCKER_IMAGE = "tensorchord/vchord-suite:pg17-20250601"
DEFAULT_DOCKER_NAME = "kodit-benchmark-db"
DEFAULT_DB_PASSWORD = "benchmarkpassword"  # noqa: S105
DEFAULT_DB_NAME = "kodit"
DEFAULT_DB_PORT = 5432

# Default enrichment endpoint (OpenRouter)
DEFAULT_ENRICHMENT_BASE_URL = "https://openrouter.ai/api/v1"
DEFAULT_ENRICHMENT_MODEL = "openrouter/anthropic/claude-haiku-4.5"
DEFAULT_ENRICHMENT_PARALLEL_TASKS = 5
DEFAULT_ENRICHMENT_TIMEOUT = 60


def build_db_url(host: str, port: int, password: str, db_name: str) -> str:
    """Build a PostgreSQL connection URL."""
    return f"postgresql+asyncpg://postgres:{password}@{host}:{port}/{db_name}"


class DatabaseContainer:
    """Manages the PostgreSQL/VectorChord Docker container."""

    def __init__(
        self,
        name: str = DEFAULT_DOCKER_NAME,
        image: str = DEFAULT_DOCKER_IMAGE,
        port: int = DEFAULT_DB_PORT,
        password: str = DEFAULT_DB_PASSWORD,
        db_name: str = DEFAULT_DB_NAME,
    ) -> None:
        """Initialize database container manager."""
        self._name = name
        self._image = image
        self._port = port
        self._password = password
        self._db_name = db_name
        self._log = structlog.get_logger(__name__)

    @property
    def db_url(self) -> str:
        """Return the database URL for this container."""
        return build_db_url("127.0.0.1", self._port, self._password, self._db_name)

    def start(self) -> bool:
        """Start a fresh database container."""
        self._log.info("Removing existing container", name=self._name)
        subprocess.run(  # noqa: S603
            ["docker", "rm", "-f", self._name],  # noqa: S607
            capture_output=True,
            check=False,
        )

        self._log.info(
            "Starting database container", name=self._name, image=self._image
        )
        result = subprocess.run(  # noqa: S603
            [  # noqa: S607
                "docker",
                "run",
                "--name",
                self._name,
                "-e",
                f"POSTGRES_DB={self._db_name}",
                "-e",
                f"POSTGRES_PASSWORD={self._password}",
                "-p",
                f"{self._port}:5432",
                "-d",
                self._image,
            ],
            capture_output=True,
            text=True,
            check=False,
        )

        if result.returncode != 0:
            self._log.error("Failed to start container", error=result.stderr)
            return False

        self._log.info("Container started, waiting for database")
        return self._wait_for_ready()

    def stop(self) -> bool:
        """Stop and remove the database container."""
        self._log.info("Stopping database container", name=self._name)
        result = subprocess.run(  # noqa: S603
            ["docker", "rm", "-f", self._name],  # noqa: S607
            capture_output=True,
            text=True,
            check=False,
        )
        if result.returncode != 0:
            self._log.error("Failed to stop container", error=result.stderr)
            return False
        self._log.info("Container stopped")
        return True

    def _wait_for_ready(self, timeout: int = 30) -> bool:
        """Wait for PostgreSQL to accept connections."""
        deadline = time.monotonic() + timeout
        attempt = 0

        while time.monotonic() < deadline:
            attempt += 1
            result = subprocess.run(  # noqa: S603
                [  # noqa: S607
                    "docker",
                    "exec",
                    self._name,
                    "pg_isready",
                    "-U",
                    "postgres",
                ],
                capture_output=True,
                check=False,
            )
            if result.returncode == 0:
                self._log.info("Database is ready", attempts=attempt)
                return True
            time.sleep(0.5)

        self._log.error("Database failed to become ready", attempts=attempt)
        return False


class ServerProcess:
    """Manages the Kodit server process for benchmarking."""

    def __init__(  # noqa: PLR0913
        self,
        host: str = DEFAULT_HOST,
        port: int = DEFAULT_PORT,
        pid_file: Path = DEFAULT_PID_FILE,
        startup_timeout: int = DEFAULT_STARTUP_TIMEOUT,
        db_port: int = DEFAULT_DB_PORT,
        enrichment_base_url: str | None = None,
        enrichment_model: str | None = None,
        enrichment_api_key: str | None = None,
        enrichment_parallel_tasks: int = DEFAULT_ENRICHMENT_PARALLEL_TASKS,
        enrichment_timeout: int = DEFAULT_ENRICHMENT_TIMEOUT,
    ) -> None:
        """Initialize server process manager."""
        self._host = host
        self._port = port
        self._pid_file = pid_file
        self._startup_timeout = startup_timeout
        self._db = DatabaseContainer(port=db_port)
        self._enrichment_base_url = enrichment_base_url
        self._enrichment_model = enrichment_model
        self._enrichment_api_key = enrichment_api_key
        self._enrichment_parallel_tasks = enrichment_parallel_tasks
        self._enrichment_timeout = enrichment_timeout
        self._log = structlog.get_logger(__name__)

    @property
    def base_url(self) -> str:
        """Return the base URL for the Kodit server."""
        return f"http://{self._host}:{self._port}"

    def start(self) -> bool:
        """Start the database and Kodit server."""
        if self._is_running():
            self._log.info("Server already running", pid=self._read_pid())
            return True

        # Start database first
        if not self._db.start():
            self._log.error("Failed to start database")
            return False

        self._log.info("Starting Kodit server", host=self._host, port=self._port)

        env = self._build_env()
        self._log.info("Using configuration", db_url=self._db.db_url)

        process = subprocess.Popen(  # noqa: S603
            [
                sys.executable,
                "-m",
                "kodit.cli",
                "serve",
                "--host",
                self._host,
                "--port",
                str(self._port),
            ],
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

        # Disable .env file reading by pointing to non-existent file
        env["KODIT_ENV_FILE"] = "/nonexistent/.env.benchmark"

        # Disable telemetry for benchmarks
        env["DISABLE_TELEMETRY"] = "true"

        # Database configuration
        env["DB_URL"] = self._db.db_url
        env["DEFAULT_SEARCH_PROVIDER"] = "vectorchord"

        # Enrichment endpoint configuration
        if self._enrichment_base_url:
            env["ENRICHMENT_ENDPOINT_BASE_URL"] = self._enrichment_base_url
        if self._enrichment_model:
            env["ENRICHMENT_ENDPOINT_MODEL"] = self._enrichment_model
        if self._enrichment_api_key:
            env["ENRICHMENT_ENDPOINT_API_KEY"] = self._enrichment_api_key
        env["ENRICHMENT_ENDPOINT_NUM_PARALLEL_TASKS"] = str(
            self._enrichment_parallel_tasks
        )
        env["ENRICHMENT_ENDPOINT_TIMEOUT"] = str(self._enrichment_timeout)

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
