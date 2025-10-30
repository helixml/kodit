"""Commit management router for the REST API."""

from typing import Annotated

from fastapi import APIRouter, Depends, HTTPException, Query
from fastapi.responses import RedirectResponse

from kodit.domain.enrichments.development.development import ENRICHMENT_TYPE_DEVELOPMENT
from kodit.domain.enrichments.development.snippet.snippet import (
    ENRICHMENT_SUBTYPE_SNIPPET,
)
from kodit.domain.entities.git import GitFile
from kodit.infrastructure.api.middleware.auth import api_key_auth
from kodit.infrastructure.api.v1.dependencies import (
    GitCommitRepositoryDep,
    GitFileRepositoryDep,
    ServerFactoryDep,
)
from kodit.infrastructure.api.v1.query_params import PaginationParamsDep
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
from kodit.infrastructure.api.v1.schemas.enrichment import (
    EnrichmentAssociationData,
    EnrichmentAttributes,
    EnrichmentData,
    EnrichmentListResponse,
    EnrichmentRelationships,
)
from kodit.infrastructure.sqlalchemy.query import (
    EnrichmentAssociationQueryBuilder,
    EnrichmentQueryBuilder,
    FilterOperator,
    GitFileQueryBuilder,
    QueryBuilder,
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
    repo_id: str,
    git_commit_repository: GitCommitRepositoryDep,
    pagination_params: PaginationParamsDep,
) -> CommitListResponse:
    """List all commits for a repository."""
    try:
        # Get all commits for the repository directly from commit repository
        commits = await git_commit_repository.find(
            QueryBuilder()
            .filter("repo_id", FilterOperator.EQ, int(repo_id))
            .paginate(pagination_params)
            .sort("date", descending=True)
        )
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
        commit = await git_commit_repository.get(commit_sha)
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
    git_file_repository: GitFileRepositoryDep,
    pagination: PaginationParamsDep,
) -> FileListResponse:
    """List all files in a specific commit."""
    files = await git_file_repository.find(
        GitFileQueryBuilder().for_commit_sha(commit_sha).paginate(pagination)
    )
    return FileListResponse(
        data=[
            FileData(
                type=GitFile.__name__,
                id=file.blob_sha,
                attributes=FileAttributes(
                    blob_sha=file.blob_sha,
                    path=file.path,
                    mime_type=file.mime_type,
                    size=file.size,
                    extension=file.extension,
                ),
            )
            for file in files
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
    git_file_repository: GitFileRepositoryDep,
) -> FileResponse:
    """Get a specific file from a commit."""
    files = await git_file_repository.find(
        GitFileQueryBuilder().for_commit_sha(commit_sha).for_blob_sha(blob_sha)
    )
    if not files:
        raise HTTPException(status_code=404, detail="File not found")
    if len(files) > 1:
        raise HTTPException(status_code=422, detail="Multiple files found")
    file = files[0]
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
) -> RedirectResponse:
    """List all snippets in a specific commit."""
    return RedirectResponse(
        status_code=308,
        url=f"/api/v1/repositories/{repo_id}/commits/{commit_sha}/enrichments?enrichment_type={ENRICHMENT_TYPE_DEVELOPMENT}&enrichment_subtype={ENRICHMENT_SUBTYPE_SNIPPET}",
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

    enrichment_query_service = server_factory.enrichment_query_service()
    snippets = await enrichment_query_service.get_all_snippets_for_commit(commit_sha)

    # Get snippet SHAs
    snippet_shas = [str(snippet.id) for snippet in snippets]

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


@router.get(
    "/{repo_id}/commits/{commit_sha}/enrichments",
    summary="List commit enrichments",
    responses={404: {"description": "Repository or commit not found"}},
)
async def list_commit_enrichments(
    repo_id: str,  # noqa: ARG001
    commit_sha: str,
    server_factory: ServerFactoryDep,
    pagination_params: PaginationParamsDep,
    enrichment_type: str | None = None,
) -> EnrichmentListResponse:
    """List all enrichments for a specific commit."""
    # TODO(Phil): Should use repo too, it's confusing to the user when they specify the
    # wrong commit and another repo. It's like they are seeing results from the other
    # repo.
    enrichment_query_service = server_factory.enrichment_query_service()
    enrichments = await enrichment_query_service.all_enrichments_for_commit(
        commit_sha=commit_sha,
        pagination=pagination_params,
        enrichment_type=enrichment_type,
    )

    return EnrichmentListResponse(
        data=[
            EnrichmentData(
                type="enrichment",
                id=str(enrichment.id),
                attributes=EnrichmentAttributes(
                    type=enrichment.type,
                    subtype=enrichment.subtype,
                    content=enrichment.content,
                    created_at=enrichment.created_at,
                    updated_at=enrichment.updated_at,
                ),
                relationships=EnrichmentRelationships(
                    associations=[
                        EnrichmentAssociationData(
                            id=association.entity_id,
                            type=association.entity_type,
                        )
                        for association in associations
                    ],
                ),
            )
            for enrichment, associations in enrichments.items()
        ]
    )


@router.delete(
    "/{repo_id}/commits/{commit_sha}/enrichments",
    summary="Delete all commit enrichments",
    responses={404: {"description": "Commit not found"}},
    status_code=204,
)
async def delete_all_commit_enrichments(
    repo_id: str,  # noqa: ARG001
    commit_sha: str,
    server_factory: ServerFactoryDep,
) -> None:
    """Delete all enrichments for a specific commit."""
    enrichment_v2_repository = server_factory.enrichment_v2_repository()
    enrichment_association_repository = (
        server_factory.enrichment_association_repository()
    )
    associations = await enrichment_association_repository.find(
        EnrichmentAssociationQueryBuilder().for_commit(commit_sha)
    )
    enrichments = await enrichment_v2_repository.find(
        EnrichmentQueryBuilder().for_ids(
            enrichment_ids=[association.enrichment_id for association in associations]
        )
    )
    await enrichment_association_repository.delete_by_query(
        EnrichmentAssociationQueryBuilder().for_enrichments(enrichments)
    )
    await enrichment_v2_repository.delete_by_query(
        EnrichmentQueryBuilder().for_ids(
            enrichment_ids=[
                enrichment.id for enrichment in enrichments if enrichment.id
            ]
        )
    )


@router.delete(
    "/{repo_id}/commits/{commit_sha}/enrichments/{enrichment_id}",
    summary="Delete commit enrichment",
    responses={404: {"description": "Enrichment not found"}},
    status_code=204,
)
async def delete_commit_enrichment(
    repo_id: str,  # noqa: ARG001
    commit_sha: str,  # noqa: ARG001
    enrichment_id: int,
    server_factory: ServerFactoryDep,
) -> None:
    """Delete a specific enrichment for a commit."""
    try:
        enrichment_v2_repository = server_factory.enrichment_v2_repository()
        enrichment = await enrichment_v2_repository.get(enrichment_id)
        await enrichment_v2_repository.delete(enrichment)
    except ValueError as e:
        raise HTTPException(status_code=404, detail="Enrichment not found") from e
