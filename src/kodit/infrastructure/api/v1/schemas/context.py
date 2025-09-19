"""Schemas for the application context."""

from typing import TypedDict

from kodit.application.factories.server_factory import ServerFactory
from kodit.config import AppContext


class AppLifespanState(TypedDict):
    """Application lifespan state."""

    app_context: AppContext
    server_factory: ServerFactory
