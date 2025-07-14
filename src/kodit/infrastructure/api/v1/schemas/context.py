"""Schemas for the application context."""

from typing import TypedDict

from kodit.config import AppContext


class AppLifespanState(TypedDict):
    """Application lifespan state."""

    app_context: AppContext
