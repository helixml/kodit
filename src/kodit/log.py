"""Logging configuration for kodit."""

import logging
import platform
import re
import shutil
import subprocess
import sys
import uuid
from functools import lru_cache
from pathlib import Path
from typing import Any

import litellm
import rudderstack.analytics as rudder_analytics  # type: ignore[import-untyped]
import structlog
from structlog.types import EventDict

from kodit import _version
from kodit.config import AppContext, LogFormat

_MAC_RE = re.compile(r"(?:[0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2}")

log = structlog.get_logger(__name__)

rudder_analytics.write_key = "2wm1RmV2GnO92NGSs8yYtmSI0mi"
rudder_analytics.dataPlaneUrl = (
    "https://danbmedefzavzlslreyxjgcjwlf.dataplane.rudderstack.com"
)


def drop_color_message_key(_, __, event_dict: EventDict) -> EventDict:  # noqa: ANN001
    """Drop the `color_message` key from the event dict."""
    event_dict.pop("color_message", None)
    return event_dict


def configure_logging(app_context: AppContext) -> None:
    """Configure logging for the application."""
    timestamper = structlog.processors.TimeStamper(fmt="iso")

    shared_processors: list[structlog.types.Processor] = [
        structlog.contextvars.merge_contextvars,
        structlog.stdlib.add_logger_name,
        structlog.stdlib.add_log_level,
        structlog.stdlib.PositionalArgumentsFormatter(),
        structlog.stdlib.ExtraAdder(),
        drop_color_message_key,
        timestamper,
        structlog.processors.StackInfoRenderer(),
    ]

    if app_context.log_format == LogFormat.JSON:
        # Format the exception only for JSON logs, as we want to pretty-print them
        # when using the ConsoleRenderer
        shared_processors.append(structlog.processors.format_exc_info)

    structlog.configure(
        processors=[
            *shared_processors,
            # Prepare event dict for `ProcessorFormatter`.
            structlog.stdlib.ProcessorFormatter.wrap_for_formatter,
        ],
        logger_factory=structlog.stdlib.LoggerFactory(),
        cache_logger_on_first_use=True,
    )

    log_renderer: structlog.types.Processor
    if app_context.log_format == LogFormat.JSON:
        log_renderer = structlog.processors.JSONRenderer()
    else:
        log_renderer = structlog.dev.ConsoleRenderer()

    formatter = structlog.stdlib.ProcessorFormatter(
        # These run ONLY on `logging` entries that do NOT originate within
        # structlog.
        foreign_pre_chain=shared_processors,
        # These run on ALL entries after the pre_chain is done.
        processors=[
            # Remove _record & _from_structlog.
            structlog.stdlib.ProcessorFormatter.remove_processors_meta,
            log_renderer,
        ],
    )

    handler = logging.StreamHandler()
    # Use OUR `ProcessorFormatter` to format all `logging` entries.
    handler.setFormatter(formatter)
    root_logger = logging.getLogger()
    root_logger.addHandler(handler)
    root_logger.setLevel(app_context.log_level.upper())

    # Configure uvicorn loggers to use our structlog setup
    # Uvicorn spits out loads of exception logs when sse server doesn't shut down
    # gracefully, so we hide them unless in DEBUG mode
    for _log in [
        "uvicorn",
        "uvicorn.error",
        "uvicorn.access",
        "bm25s",
        "sentence_transformers.SentenceTransformer",
        "httpx",
        "LiteLLM",
    ]:
        if root_logger.getEffectiveLevel() == logging.DEBUG:
            logging.getLogger(_log).handlers.clear()
            logging.getLogger(_log).propagate = True
        else:
            logging.getLogger(_log).disabled = True

    # More litellm logging cruft
    litellm.suppress_debug_info = True

    # Configure SQLAlchemy loggers to use our structlog setup
    for _log in ["sqlalchemy.engine", "alembic"]:
        engine_logger = logging.getLogger(_log)
        engine_logger.setLevel(logging.WARNING)  # Hide INFO logs by default
        if app_context.log_level.upper() == "DEBUG":
            engine_logger.setLevel(
                logging.DEBUG
            )  # Only show all logs when in DEBUG mode

    def handle_exception(
        exc_type: type[BaseException],
        exc_value: BaseException,
        exc_traceback: Any,
    ) -> None:
        """Log any uncaught exception instead of letting it be printed by Python.

        This leaves KeyboardInterrupt untouched to allow users to Ctrl+C to stop.
        See https://stackoverflow.com/a/16993115/3641865
        """
        if issubclass(exc_type, KeyboardInterrupt):
            sys.__excepthook__(exc_type, exc_value, exc_traceback)
            return

        root_logger.error(
            "Uncaught exception", exc_info=(exc_type, exc_value, exc_traceback)
        )

    sys.excepthook = handle_exception


def configure_telemetry(app_context: AppContext) -> None:
    """Configure telemetry for the application."""
    litellm.telemetry = False  # Disable litellm telemetry by default
    if app_context.disable_telemetry:
        structlog.stdlib.get_logger(__name__).info("Telemetry has been disabled")
        rudder_analytics.send = False

    rudder_analytics.identify(
        anonymous_id=get_stable_mac_str(),
        traits={},
    )


def log_event(event: str, properties: dict[str, Any] | None = None) -> None:
    """Log an event to Rudderstack."""
    p = properties or {}
    # Set default posthog properties
    p["$app_name"] = "kodit"
    p["$app_version"] = _version.version
    p["$os"] = sys.platform
    p["$os_version"] = sys.version
    rudder_analytics.track(
        anonymous_id=get_stable_mac_str(),
        event=event,
        properties=properties or {},
    )


# ----------------------------------------------------------------------
# Helper functions
# ----------------------------------------------------------------------
def _mac_int(mac: str) -> int:
    return int(mac.replace(":", "").replace("-", ""), 16)


def _is_globally_administered(mac_int: int) -> bool:
    first_octet = (mac_int >> 40) & 0xFF
    return not (first_octet & 0b11)  # both bits must be 0


def _from_sysfs() -> list[int]:
    base = Path("/sys/class/net")
    if not base.is_dir():
        return []
    macs: list[int] = []
    for iface in base.iterdir():
        try:
            # Skip if iface is not a directory (e.g., bonding_masters is a file)
            if not iface.is_dir():
                continue
            with (base / iface / "address").open() as f:
                content = f.read().strip()
            if _MAC_RE.fullmatch(content):
                macs.append(_mac_int(content))
        except (FileNotFoundError, PermissionError, NotADirectoryError):
            pass
    return macs


def _from_command(cmd: str) -> list[int]:
    try:
        out = subprocess.check_output(  # noqa: S602
            cmd,
            shell=True,
            text=True,
            stderr=subprocess.DEVNULL,
            encoding="utf-8",
        )
    except Exception:  # noqa: BLE001
        return []
    return [_mac_int(m.group()) for m in _MAC_RE.finditer(out)]


@lru_cache(maxsize=1)
def get_stable_mac_int() -> int | None:
    """Return a *hardware* MAC as an int, or None if none can be found.

    Search order:
        1. /sys/class/net (Linux)
        2. `ip link show` (Linux), `ifconfig -a` (Linux+macOS)
        3. `getmac` and `wmic nic` (Windows)
    The first globally-administered, non-multicast address wins.
    """
    system = platform.system()
    candidates: list[int] = []

    if system == "Linux":
        candidates += _from_sysfs()
        if not candidates and shutil.which("ip"):
            candidates += _from_command("ip link show")
        if not candidates:  # fall back to ifconfig
            candidates += _from_command("ifconfig -a")

    elif system == "Darwin":  # macOS
        candidates += _from_command("ifconfig -a")

    elif system == "Windows":
        # getmac is present on every supported Windows version
        candidates += _from_command("getmac /v /fo list")
        # wmic still exists through at least Win 11
        candidates += _from_command(
            'wmic nic where "MACAddress is not null" get MACAddress /format:list'
        )

    # Prefer globally administered, non-multicast addresses
    for mac in candidates:
        if _is_globally_administered(mac):
            return mac

    # If all we saw were locally-administered MACs, just return the first one
    if candidates:
        return candidates[0]

    return None


def get_stable_mac_str() -> str:
    """Return a *stable* 12-digit hex string (lower-case, no separators).

    Falls back to uuid.getnode() if necessary, so it never raises.
    """
    mac_int = get_stable_mac_int()
    if mac_int is None:
        mac_int = uuid.getnode()  # may still be random in VMs
    return f"{mac_int:012x}"
