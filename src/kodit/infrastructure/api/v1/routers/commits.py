"""Commit management router for the REST API."""

from typing import Annotated

from fastapi import APIRouter, Depends, HTTPException, Query

from kodit.infrastructure.api.middleware.auth import api_key_auth
from kodit.infrastructure.api.v1.dependencies import (
    GitCommitRepositoryDep,
    ServerFactoryDep,
)
from kodit.infrastructure.api.v1.schemas.commit import (
    CommitAttributes,
    CommitData,
    CommitListResponse,
    CommitResponse,
    EmbeddingAttributes,
    EmbeddingData,
    EmbeddingListResponse,
    FileAttributes,
    FileData,
    FileListResponse,
    FileResponse,
)
from kodit.infrastructure.api.v1.schemas.snippet import (
    EnrichmentSchema,
    GitFileSchema,
    SnippetAttributes,
    SnippetContentSchema,
    SnippetData,
    SnippetListResponse,
)

router = APIRouter(
    prefix="/api/v1/repositories",
    tags=["commits"],
    dependencies=[Depends(api_key_auth)],
    responses={
        401: {"description": "Unauthorized"},
        422: {"description": "Invalid request"},
    },
)


@router.get("/{repo_id}/commits", summary="List repository commits")
async def list_repository_commits(
    repo_id: str, git_commit_repository: GitCommitRepositoryDep
) -> CommitListResponse:
    """List all commits for a repository."""
    try:
        # Get all commits for the repository directly from commit repository
        commits = await git_commit_repository.get_by_repo_id(int(repo_id))
    except ValueError as e:
        raise HTTPException(status_code=404, detail="Repository not found") from e

    return CommitListResponse(
        data=[
            CommitData(
                type="commit",
                id=commit.commit_sha,
                attributes=CommitAttributes(
                    commit_sha=commit.commit_sha,
                    date=commit.date,
                    message=commit.message,
                    parent_commit_sha=commit.parent_commit_sha or "",
                    author=commit.author,
                ),
            )
            for commit in commits
        ]
    )


@router.get(
    "/{repo_id}/commits/{commit_sha}",
    summary="Get repository commit",
    responses={404: {"description": "Repository or commit not found"}},
)
async def get_repository_commit(
    repo_id: str,  # noqa: ARG001
    commit_sha: str,
    git_commit_repository: GitCommitRepositoryDep,
) -> CommitResponse:
    """Get a specific commit for a repository."""
    try:
        # Get the specific commit directly from commit repository
        commit = await git_commit_repository.get_by_sha(commit_sha)
    except ValueError as e:
        raise HTTPException(status_code=404, detail="Commit not found") from e

    return CommitResponse(
        data=CommitData(
            type="commit",
            id=commit.commit_sha,
            attributes=CommitAttributes(
                commit_sha=commit.commit_sha,
                date=commit.date,
                message=commit.message,
                parent_commit_sha=commit.parent_commit_sha or "",
                author=commit.author,
            ),
        )
    )


@router.get("/{repo_id}/commits/{commit_sha}/files", summary="List commit files")
async def list_commit_files(
    repo_id: str,  # noqa: ARG001
    commit_sha: str,
    git_commit_repository: GitCommitRepositoryDep,
) -> FileListResponse:
    """List all files in a specific commit."""
    try:
        # Get the specific commit directly from commit repository
        commit = await git_commit_repository.get_by_sha(commit_sha)
    except ValueError as e:
        raise HTTPException(status_code=404, detail="Commit not found") from e

    return FileListResponse(
        data=[
            FileData(
                type="file",
                id=file.blob_sha,
                attributes=FileAttributes(
                    blob_sha=file.blob_sha,
                    path=file.path,
                    mime_type=file.mime_type,
                    size=file.size,
                    extension=file.extension,
                ),
            )
            for file in commit.files
        ]
    )


@router.get(
    "/{repo_id}/commits/{commit_sha}/files/{blob_sha}",
    summary="Get commit file",
    responses={404: {"description": "Repository, commit or file not found"}},
)
async def get_commit_file(
    repo_id: str,  # noqa: ARG001
    commit_sha: str,
    blob_sha: str,
    git_commit_repository: GitCommitRepositoryDep,
) -> FileResponse:
    """Get a specific file from a commit."""
    try:
        # Get the specific commit directly from commit repository
        commit = await git_commit_repository.get_by_sha(commit_sha)
    except ValueError as e:
        raise HTTPException(status_code=404, detail="Commit not found") from e

    # Find the specific file
    file = next((f for f in commit.files if f.blob_sha == blob_sha), None)
    if not file:
        raise HTTPException(status_code=404, detail="File not found")

    return FileResponse(
        data=FileData(
            type="file",
            id=file.blob_sha,
            attributes=FileAttributes(
                blob_sha=file.blob_sha,
                path=file.path,
                mime_type=file.mime_type,
                size=file.size,
                extension=file.extension,
            ),
        )
    )


@router.get(
    "/{repo_id}/commits/{commit_sha}/snippets",
    summary="List commit snippets",
    responses={404: {"description": "Repository or commit not found"}},
)
async def list_commit_snippets(
    repo_id: str,
    commit_sha: str,
    server_factory: ServerFactoryDep,
) -> SnippetListResponse:
    """List all snippets in a specific commit."""
    _ = repo_id  # Required by FastAPI route path but not used in function
    snippet_repository = server_factory.snippet_v2_repository()
    snippets = await snippet_repository.get_snippets_for_commit(commit_sha)

    return SnippetListResponse(
        data=[
            SnippetData(
                type="snippet",
                id=snippet.sha,
                attributes=SnippetAttributes(
                    created_at=snippet.created_at,
                    updated_at=snippet.updated_at,
                    derives_from=[
                        GitFileSchema(
                            blob_sha=file.blob_sha,
                            path=file.path,
                            mime_type=file.mime_type,
                            size=file.size,
                        )
                        for file in snippet.derives_from
                    ],
                    content=SnippetContentSchema(
                        value=snippet.content,
                        language=snippet.extension,
                    ),
                    enrichments=[
                        EnrichmentSchema(
                            type=enrichment.type.value,
                            content=enrichment.content,
                        )
                        for enrichment in snippet.enrichments
                    ],
                ),
            )
            for snippet in snippets
        ]
    )


# TODO(Phil): This doesn't return vectorchord embeddings properly because it's
# implemented in a different repo
@router.get(
    "/{repo_id}/commits/{commit_sha}/embeddings",
    summary="List commit embeddings",
    responses={404: {"description": "Repository or commit not found"}},
)
async def list_commit_embeddings(
    repo_id: str,
    commit_sha: str,
    server_factory: ServerFactoryDep,
    full: Annotated[  # noqa: FBT002
        bool,
        Query(
            description="If true, return full vectors. If false, return first 5 values."
        ),
    ] = False,
) -> EmbeddingListResponse:
    """List all embeddings for snippets in a specific commit."""
    _ = repo_id  # Required by FastAPI route path but not used in function
    snippet_repository = server_factory.snippet_v2_repository()
    snippets = await snippet_repository.get_snippets_for_commit(commit_sha)

    if not snippets:
        return EmbeddingListResponse(data=[])

    # Get snippet SHAs
    snippet_shas = [snippet.sha for snippet in snippets]

    # Get embeddings for all snippets in the commit
    embedding_repository = server_factory.embedding_repository()
    embeddings = await embedding_repository.get_embeddings_by_snippet_ids(snippet_shas)

    return EmbeddingListResponse(
        data=[
            EmbeddingData(
                type="embedding",
                id=f"{embedding.snippet_id}_{embedding.type.value}",
                attributes=EmbeddingAttributes(
                    snippet_sha=embedding.snippet_id,
                    embedding_type=embedding.type.name.lower(),
                    embedding=embedding.embedding if full else embedding.embedding[:5],
                ),
            )
            for embedding in embeddings
        ]
    )
