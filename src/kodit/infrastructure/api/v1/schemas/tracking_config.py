"""Tracking configuration JSON-API schemas."""

from enum import StrEnum
from typing import Literal

from pydantic import BaseModel, Field

from kodit.domain.entities.git import GitRepo, TrackingConfig, TrackingType


class TrackingMode(StrEnum):
    """Tracking mode for repository configuration."""

    BRANCH = "branch"
    TAG = "tag"


class TrackingConfigAttributes(BaseModel):
    """Tracking configuration attributes following JSON-API spec."""

    mode: TrackingMode = Field(
        ...,
        description="'branch' tracks a specific branch, 'tag' tracks the latest tag",
    )
    value: str | None = Field(
        None,
        description="Branch name when mode is 'branch'. Not used for 'tag' mode.",
    )

    @staticmethod
    def from_tracking_config(
        config: TrackingConfig | None,
    ) -> "TrackingConfigAttributes":
        """Create tracking config attributes from domain TrackingConfig."""
        if not config:
            return TrackingConfigAttributes(mode=TrackingMode.BRANCH, value="main")

        if config.type == TrackingType.TAG:
            return TrackingConfigAttributes(mode=TrackingMode.TAG, value=None)

        return TrackingConfigAttributes(mode=TrackingMode.BRANCH, value=config.name)

    def to_domain(self) -> TrackingConfig:
        """Convert to domain TrackingConfig."""
        if self.mode == TrackingMode.TAG:
            return TrackingConfig(type=TrackingType.TAG, name="")

        return TrackingConfig(type=TrackingType.BRANCH, name=self.value or "main")


class TrackingConfigData(BaseModel):
    """Tracking configuration data following JSON-API spec."""

    type: Literal["tracking-config"] = "tracking-config"
    attributes: TrackingConfigAttributes

    @staticmethod
    def from_git_repo(repo: GitRepo) -> "TrackingConfigData":
        """Create tracking config data from a Git repository."""
        return TrackingConfigData(
            attributes=TrackingConfigAttributes.from_tracking_config(repo.tracking_config),
        )


class TrackingConfigResponse(BaseModel):
    """Tracking configuration response following JSON-API spec."""

    data: TrackingConfigData


class TrackingConfigUpdateAttributes(BaseModel):
    """Tracking configuration update attributes."""

    mode: TrackingMode = Field(
        ...,
        description="'branch' tracks a specific branch, 'tag' tracks the latest tag",
    )
    value: str | None = Field(
        None,
        description="Branch name when mode is 'branch'. Not used for 'tag' mode.",
    )

    def to_domain(self) -> TrackingConfig:
        """Convert to domain TrackingConfig."""
        if self.mode == TrackingMode.TAG:
            return TrackingConfig(type=TrackingType.TAG, name="")

        return TrackingConfig(type=TrackingType.BRANCH, name=self.value or "main")


class TrackingConfigUpdateData(BaseModel):
    """Tracking configuration update data."""

    type: Literal["tracking-config"] = "tracking-config"
    attributes: TrackingConfigUpdateAttributes


class TrackingConfigUpdateRequest(BaseModel):
    """Tracking configuration update request."""

    data: TrackingConfigUpdateData
