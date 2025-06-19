"""Infrastructure implementation for language detection."""

from pathlib import Path

from kodit.domain.services.snippet_extraction_service import LanguageDetectionService


class FileSystemLanguageDetectionService(LanguageDetectionService):
    """Infrastructure implementation for language detection."""

    def __init__(self, language_map: dict[str, str]) -> None:
        """Initialize the language detection service.

        Args:
            language_map: Mapping of file extensions to programming languages

        """
        self.language_map = language_map

    async def detect_language(self, file_path: Path) -> str:
        """Detect language based on file extension.

        Args:
            file_path: Path to the file to detect language for

        Returns:
            The detected programming language

        Raises:
            ValueError: If the language is not supported

        """
        suffix = file_path.suffix.removeprefix(".").lower()
        language = self.language_map.get(suffix)

        if language is None:
            raise ValueError(f"Unsupported language for file suffix: {suffix}")

        return language
