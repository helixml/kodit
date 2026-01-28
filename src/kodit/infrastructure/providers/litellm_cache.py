"""LiteLLM cache configuration."""

from pathlib import Path

import litellm
import structlog
from litellm.caching.caching import Cache, LiteLLMCacheType

_cache_initialized = False


def configure_litellm_cache(cache_dir: Path, *, enabled: bool = True) -> None:
    """Configure LiteLLM disk caching."""
    global _cache_initialized  # noqa: PLW0603

    if _cache_initialized:
        return

    log = structlog.get_logger(__name__)

    if not enabled:
        log.info("LiteLLM cache disabled")
        _cache_initialized = True
        return

    cache_dir.mkdir(parents=True, exist_ok=True)
    litellm.cache = Cache(type=LiteLLMCacheType.DISK, disk_cache_dir=str(cache_dir))
    _cache_initialized = True
    log.info("LiteLLM cache initialized", cache_dir=str(cache_dir))
