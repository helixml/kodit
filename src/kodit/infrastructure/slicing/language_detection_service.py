"""Language detection service implementation."""

from pathlib import Path

from kodit.domain.services.index_service import LanguageDetectionService


class FileSystemLanguageDetectionService(LanguageDetectionService):
    """Simple file extension based language detection service."""

    def __init__(self, language_map: dict[str, str]) -> None:
        """Initialize with a mapping of extensions to languages."""
        self._language_map = language_map

    async def detect_language(self, file_path: Path) -> str:
        """Detect language based on file extension."""
        extension = file_path.suffix.lstrip(".")
        return self._language_map.get(extension, "unknown")
