"""Pure domain entities using Pydantic."""

from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from urllib.parse import urlparse, urlunparse

from pydantic import AnyUrl, BaseModel

from kodit.domain.value_objects import (
    SnippetContent,
    SnippetContentType,
    SourceType,
)
from kodit.utils.path_utils import path_from_uri


class Author(BaseModel):
    """Author domain entity."""

    id: int | None = None
    name: str
    email: str


class File(BaseModel):
    """File domain entity."""

    id: int | None = None  # Is populated by repository
    created_at: datetime | None = None  # Is populated by repository
    updated_at: datetime | None = None  # Is populated by repository
    uri: AnyUrl
    sha256: str
    authors: list[Author]
    mime_type: str

    def as_path(self) -> Path:
        """Return the file as a path."""
        return path_from_uri(str(self.uri))

    def extension(self) -> str:
        """Return the file extension."""
        return Path(self.as_path()).suffix.lstrip(".")


class WorkingCopy(BaseModel):
    """Working copy value object representing cloned source location."""

    created_at: datetime | None = None  # Is populated by repository
    updated_at: datetime | None = None  # Is populated by repository
    remote_uri: AnyUrl
    cloned_path: Path
    source_type: SourceType
    files: list[File]

    @classmethod
    def sanitize_local_path(cls, path: str) -> AnyUrl:
        """Sanitize a local path."""
        return AnyUrl(Path(path).resolve().absolute().as_uri())

    @classmethod
    def sanitize_git_url(cls, url: str) -> AnyUrl:
        """Remove credentials from a git URL while preserving the rest of the URL.

        This function handles various git URL formats:
        - HTTPS URLs with username:password@host
        - HTTPS URLs with username@host (no password)
        - SSH URLs (left unchanged)
        - File URLs (left unchanged)

        Args:
            url: The git URL that may contain credentials.

        Returns:
            The sanitized URL with credentials removed.

        Examples:
            >>> sanitize_git_url("https://phil:token@dev.azure.com/org/project/_git/repo")
            "https://dev.azure.com/org/project/_git/repo"
            >>> sanitize_git_url("https://username@github.com/user/repo.git")
            "https://github.com/user/repo.git"
            >>> sanitize_git_url("git@github.com:user/repo.git")
            "ssh://git@github.com/user/repo.git"

        """
        # Handle SSH URLs (they don't have credentials in the URL format)
        if url.startswith("git@"):
            # Convert git@host:path to ssh://git@host/path format for AnyUrl
            # This maintains the same semantic meaning while making it a valid URL
            if ":" in url and not url.startswith("ssh://"):
                host_path = url[4:]  # Remove "git@"
                if ":" in host_path:
                    host, path = host_path.split(":", 1)
                    ssh_url = f"ssh://git@{host}/{path}"
                    return AnyUrl(ssh_url)
            return AnyUrl(url)
        if url.startswith("ssh://"):
            return AnyUrl(url)

        # Handle file URLs
        if url.startswith("file://"):
            return AnyUrl(url)

        try:
            # Parse the URL
            parsed = urlparse(url)

            # If there are no credentials, return the URL as-is
            if not parsed.username:
                return AnyUrl(url)

            # Reconstruct the URL without credentials
            # scheme, netloc (without username/password), path, params, query, fragment
            sanitized_netloc = parsed.hostname
            if parsed.port:
                sanitized_netloc = f"{parsed.hostname}:{parsed.port}"

            return AnyUrl(
                urlunparse(
                    (
                        parsed.scheme,
                        sanitized_netloc,
                        parsed.path,
                        parsed.params,
                        parsed.query,
                        parsed.fragment,
                    )
                )
            )

        except Exception as e:
            raise ValueError(f"Invalid URL: {url}") from e


class Source(BaseModel):
    """Source domain entity."""

    id: int | None = None  # Is populated by repository
    created_at: datetime | None = None  # Is populated by repository
    updated_at: datetime | None = None  # Is populated by repository
    working_copy: WorkingCopy


class Snippet(BaseModel):
    """Snippet domain entity."""

    id: int | None = None  # Is populated by repository
    created_at: datetime | None = None  # Is populated by repository
    updated_at: datetime | None = None  # Is populated by repository
    derives_from: list[File]
    original_content: SnippetContent | None = None
    summary_content: SnippetContent | None = None

    def original_text(self) -> str:
        """Return the original content of the snippet."""
        if self.original_content is None:
            return ""
        return self.original_content.value

    def summary_text(self) -> str:
        """Return the summary content of the snippet."""
        if self.summary_content is None:
            return ""
        return self.summary_content.value

    def add_original_content(self, content: str, language: str) -> None:
        """Add an original content to the snippet."""
        self.original_content = SnippetContent(
            type=SnippetContentType.ORIGINAL,
            value=content,
            language=language,
        )

    def add_summary(self, summary: str) -> None:
        """Add a summary to the snippet."""
        self.summary_content = SnippetContent(
            type=SnippetContentType.SUMMARY,
            value=summary,
            language="markdown",
        )


class Index(BaseModel):
    """Index domain entity."""

    id: int
    created_at: datetime
    updated_at: datetime
    source: Source
    snippets: list[Snippet]


# FUTURE: Remove this type, use the domain to get the required information.
@dataclass(frozen=True)
class SnippetWithContext:
    """Domain model for snippet with associated context information."""

    source: Source
    file: File
    authors: list[Author]
    snippet: Snippet
