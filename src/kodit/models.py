"""Database models for kodit."""

from datetime import datetime

from sqlalchemy import DateTime, String
from sqlalchemy.orm import Mapped, mapped_column

from kodit.database import Base


class Source(Base):
    """Source model for tracking code sources.

    Attributes:
        id: The unique identifier for the source
        name: The name of the source
        path: The path to the source
        created_at: When the source was created
        updated_at: When the source was last updated

    """

    __tablename__ = "sources"

    id: Mapped[int] = mapped_column(primary_key=True)
    name: Mapped[str] = mapped_column(String(255))
    path: Mapped[str] = mapped_column(String(1024))
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow)
    updated_at: Mapped[datetime | None] = mapped_column(
        DateTime,
        default=datetime.utcnow,
        onupdate=datetime.utcnow,
    )
