from pydantic import AnyUrl

from kodit.domain.entities import GitTag
from kodit.domain.protocols import GitTagRepository


class InMemoryGitTagRepository(GitTagRepository):
    """Simple in-memory implementation of GitTagRepository."""

    def __init__(self) -> None:
        self._tags: dict[str, list[GitTag]] = {}  # repo_uri -> list of tags

    async def save_tags(self, repo_uri: AnyUrl, tags: list[GitTag]) -> None:
        """Save tags for a repository."""
        self._tags[str(repo_uri)] = tags.copy()

    async def get_tags_for_repo(self, repo_uri: AnyUrl) -> list[GitTag]:
        """Get all tags for a repository."""
        return self._tags.get(str(repo_uri), [])

    async def get_tags_for_commit(
        self, repo_uri: AnyUrl, commit_sha: str
    ) -> list[GitTag]:
        """Get all tags pointing to a specific commit."""
        all_tags = await self.get_tags_for_repo(repo_uri)
        return [tag for tag in all_tags if tag.target_commit_sha == commit_sha]

    async def get_version_tags(self, repo_uri: AnyUrl) -> list[GitTag]:
        """Get version tags for a repository."""
        all_tags = await self.get_tags_for_repo(repo_uri)
        return [tag for tag in all_tags if tag.is_version_tag]

    async def get_tag_by_id(self, tag_id: str) -> GitTag:
        """Get a tag by its ID."""
        for tags in self._tags.values():
            for tag in tags:
                if tag.id == tag_id:
                    return tag
        raise ValueError(f"Tag with ID {tag_id} not found")
