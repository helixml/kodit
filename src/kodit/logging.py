"""Logging configuration for kodit."""

import logging
import sys
import uuid
from enum import Enum
from functools import lru_cache
from typing import Any

import structlog
from posthog import Posthog
from structlog.types import EventDict

from kodit.config import AppContext

log = structlog.get_logger(__name__)


def drop_color_message_key(_, __, event_dict: EventDict) -> EventDict:  # noqa: ANN001
    """Drop the `color_message` key from the event dict."""
    event_dict.pop("color_message", None)
    return event_dict


class LogFormat(Enum):
    """The format of the log output."""

    PRETTY = "pretty"
    JSON = "json"


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
    ]:
        if root_logger.getEffectiveLevel() == logging.DEBUG:
            logging.getLogger(_log).handlers.clear()
            logging.getLogger(_log).propagate = True
        else:
            logging.getLogger(_log).disabled = True

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


posthog = Posthog(
    project_api_key="phc_JsX0yx8NLPcIxamfp4Zc7xyFykXjwmekKUQz060cSt3",
    host="https://eu.i.posthog.com",
)


@lru_cache(maxsize=1)
def get_mac_address() -> str:
    """Get the MAC address of the primary network interface.

    Returns:
        str: The MAC address or a fallback UUID if not available

    """
    # Get the MAC address of the primary network interface
    mac = uuid.getnode()
    return f"{mac:012x}" if mac != uuid.getnode() else str(uuid.uuid4())


def configure_telemetry(app_context: AppContext) -> None:
    """Configure telemetry for the application."""
    if app_context.disable_telemetry:
        structlog.stdlib.get_logger(__name__).info("Telemetry has been disabled")
        posthog.disabled = True


def log_event(event: str, properties: dict[str, Any] | None = None) -> None:
    """Log an event to PostHog."""
    log.debug(
        "Logging event", id=get_mac_address(), ph_event=event, ph_properties=properties
    )
    posthog.capture(get_mac_address(), event, properties or {})
