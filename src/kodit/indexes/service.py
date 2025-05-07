"""Source repository."""

import mimetypes
from datetime import UTC, datetime
from hashlib import sha256
from pathlib import Path

import aiofiles
import pydantic
import structlog
from sqlalchemy import func, select
from sqlalchemy.ext.asyncio import AsyncSession
from tqdm.asyncio import tqdm

from kodit.indexes.models import File, Snippet
from kodit.indexes.models import Index as IndexModel
from kodit.sources.models import FolderSource, GitSource, Source

MIME_WHITELIST = [
    "text/plain",
    "text/markdown",
    "text/x-python",
    "text/x-shellscript",
    "text/x-sql",
]


class Index(pydantic.BaseModel):
    """Index model."""

    id: int
    created_at: datetime
    updated_at: datetime | None = None
    source_uri: str | None = None
    num_files: int | None = None
    num_snippets: int | None = None


class IndexService:
    """Index repository."""

    def __init__(self, session: AsyncSession) -> None:
        """Initialize the index repository."""
        self.session = session
        self.log = structlog.get_logger(__name__)

    async def create(self, source_id: int) -> Index:
        """Create an index."""
        # First, check if the source exists
        source = await self.session.execute(
            select(Source).where(Source.id == source_id)
        )
        if not source.scalar_one_or_none():
            msg = f"Source not found, please create it first: {source_id}"
            raise ValueError(msg)

        # Now check if there is already an index on this source
        index = await self.session.execute(
            select(IndexModel).where(IndexModel.source_id == source_id)
        )
        if index.scalar_one_or_none():
            msg = f"Index already exists on this source: {source_id}"
            raise ValueError(msg)

        index = IndexModel(source_id=source_id)
        self.session.add(index)
        await self.session.commit()
        return Index(
            id=index.id, created_at=index.created_at, source_id=index.source_id
        )

    async def list(self) -> list[Index]:
        """List indexes."""
        query = (
            select(
                IndexModel,
                Source,
                GitSource,
                FolderSource,
                func.count(File.id).label("file_count"),
                func.count(Snippet.id).label("snippet_count"),
            )
            .join(Source, IndexModel.source_id == Source.id)
            .outerjoin(GitSource, Source.id == GitSource.source_id)
            .outerjoin(FolderSource, Source.id == FolderSource.source_id)
            .outerjoin(File, Source.id == File.source_id)
            .outerjoin(Snippet, File.id == Snippet.file_id)
            .group_by(IndexModel.id, Source.id, GitSource.id, FolderSource.id)
        )
        result = await self.session.execute(query)
        rows = result.all()

        # Map to Pydantic model
        return [
            Index(
                id=index.id,
                created_at=index.created_at,
                updated_at=index.updated_at,
                source_uri=git_source.uri
                if git_source
                else folder_source.path
                if folder_source
                else None,
                num_files=file_count,
                num_snippets=snippet_count,
            )
            for index, source, git_source, folder_source, file_count, snippet_count in rows
        ]

    async def run(self, index_id: int) -> None:
        """Run an index."""
        # Get the index
        index = await self.session.execute(
            select(IndexModel).where(IndexModel.id == index_id)
        )
        index = index.scalar_one_or_none()

        if not index:
            msg = f"Index not found: {index_id}"
            raise ValueError(msg)

        # Now find out what kind of source this is
        result = await self.session.execute(
            select(Source, GitSource, FolderSource)
            .where(Source.id == index.source_id)
            .outerjoin(GitSource, Source.id == GitSource.source_id)
            .outerjoin(FolderSource, Source.id == FolderSource.source_id)
        )
        row = result.first()

        if not row:
            msg = f"Source not found: {index.source_id}"
            raise ValueError(msg)

        source, git_source, folder_source = row

        # Build a list of sha's that have already been indexed
        existing_files = await self.session.execute(
            select(File.sha256).where(File.source_id == index.source_id)
        )
        existing_files = existing_files.scalars().all()
        existing_files_set = set(existing_files)

        if git_source:
            msg = "Git source indexing is not implemented yet"
            raise NotImplementedError(msg)
        if folder_source:
            # Count how many files there are in the folder, recursively
            file_count = 0
            for file_path in Path(folder_source.path).rglob("*"):
                if file_path.is_file():
                    file_count += 1

            # Find all files in the folder
            for file_path in tqdm(
                Path(folder_source.path).rglob("*"), total=file_count
            ):
                if file_path.is_file():
                    # Read the file content
                    async with aiofiles.open(file_path, "rb") as f:
                        content = await f.read()

                        # Detect the mime type of the file
                        mime_type = mimetypes.guess_type(file_path)

                        sha = sha256(content).hexdigest()

                        # Check if the file already exists
                        if sha in existing_files_set:
                            self.log.debug("File already exists", file_path=file_path)
                            continue

                        # Create a file model
                        file = File(
                            index_id=index.id,
                            source_id=index.source_id,
                            mime_type=mime_type[0]
                            if mime_type and mime_type[0]
                            else "application/octet-stream",
                            path=str(file_path),
                            sha256=sha,
                            size_bytes=len(content),
                        )

                        self.log.debug("Adding file", file=file)
                        self.session.add(file)

            await self.session.commit()

            # Now create snippets, based on these files
            # This will be improved in due course

            files = await self.session.execute(
                select(File).where(File.source_id == index.source_id)
            )
            files = files.scalars().all()

            # Create a list of all snippets that have already been created
            existing_snippets = await self.session.execute(
                select(Snippet).where(Snippet.index_id == index.id)
            )
            existing_snippets = existing_snippets.scalars().all()
            existing_snippets_set = {snippet.file_id for snippet in existing_snippets}

            for file in tqdm(files, total=len(files)):
                # Only keep files that match the whitelist
                if file.mime_type not in MIME_WHITELIST:
                    self.log.debug("Skipping mime type", mime_type=file.mime_type)
                    continue

                # Check if the snippet has already been created
                if file.id in existing_snippets_set:
                    self.log.debug("Snippet already exists", file_id=file.id)
                    continue

                # Read the file content
                async with aiofiles.open(file.path, "rb") as f:
                    content = await f.read()

                    # Create a snippet
                    snippet = Snippet(
                        index_id=index.id,
                        file_id=file.id,
                        content=content,
                    )
                    self.session.add(snippet)

            await self.session.commit()

            # Touch the index to indicate that it has been updated
            index.updated_at = datetime.now(UTC)

            await self.session.commit()
        else:
            msg = f"Unsupported source type: {type(source)}"
            raise TypeError(msg)
