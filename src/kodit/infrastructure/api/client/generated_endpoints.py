"""API endpoint constants generated from OpenAPI specification.

This file is auto-generated. Do not edit manually.
Run `make generate-api-paths` to regenerate.
"""

# ruff: noqa: E501

from typing import Final


class APIEndpoints:
    """API endpoint constants extracted from OpenAPI specification."""

    # /api/v1/queue
    API_V1_QUEUE: Final[str] = "/api/v1/queue"

    # /api/v1/queue/{task_id}
    API_V1_QUEUE_TASK_ID: Final[str] = "/api/v1/queue/{task_id}"

    # /api/v1/repositories
    API_V1_REPOSITORIES: Final[str] = "/api/v1/repositories"

    # /api/v1/repositories/{repo_id}
    API_V1_REPOSITORIES_REPO_ID: Final[str] = "/api/v1/repositories/{repo_id}"

    # /api/v1/repositories/{repo_id}/commits
    API_V1_REPOSITORIES_REPO_ID_COMMITS: Final[str] = "/api/v1/repositories/{repo_id}/commits"

    # /api/v1/repositories/{repo_id}/commits/{commit_sha}
    API_V1_REPOSITORIES_REPO_ID_COMMITS_COMMIT_SHA: Final[str] = "/api/v1/repositories/{repo_id}/commits/{commit_sha}"

    # /api/v1/repositories/{repo_id}/commits/{commit_sha}/embeddings
    API_V1_REPOSITORIES_REPO_ID_COMMITS_COMMIT_SHA_EMBEDDINGS: Final[str] = "/api/v1/repositories/{repo_id}/commits/{commit_sha}/embeddings"

    # /api/v1/repositories/{repo_id}/commits/{commit_sha}/enrichments
    API_V1_REPOSITORIES_REPO_ID_COMMITS_COMMIT_SHA_ENRICHMENTS: Final[str] = "/api/v1/repositories/{repo_id}/commits/{commit_sha}/enrichments"

    # /api/v1/repositories/{repo_id}/commits/{commit_sha}/enrichments/{enrichment_id}
    API_V1_REPOSITORIES_REPO_ID_COMMITS_COMMIT_SHA_ENRICHMENTS_ENRICHMENT_ID: Final[str] = "/api/v1/repositories/{repo_id}/commits/{commit_sha}/enrichments/{enrichment_id}"

    # /api/v1/repositories/{repo_id}/commits/{commit_sha}/files
    API_V1_REPOSITORIES_REPO_ID_COMMITS_COMMIT_SHA_FILES: Final[str] = "/api/v1/repositories/{repo_id}/commits/{commit_sha}/files"

    # /api/v1/repositories/{repo_id}/commits/{commit_sha}/files/{blob_sha}
    API_V1_REPOSITORIES_REPO_ID_COMMITS_COMMIT_SHA_FILES_BLOB_SHA: Final[str] = "/api/v1/repositories/{repo_id}/commits/{commit_sha}/files/{blob_sha}"

    # /api/v1/repositories/{repo_id}/commits/{commit_sha}/snippets
    API_V1_REPOSITORIES_REPO_ID_COMMITS_COMMIT_SHA_SNIPPETS: Final[str] = "/api/v1/repositories/{repo_id}/commits/{commit_sha}/snippets"

    # /api/v1/repositories/{repo_id}/enrichments
    API_V1_REPOSITORIES_REPO_ID_ENRICHMENTS: Final[str] = "/api/v1/repositories/{repo_id}/enrichments"

    # /api/v1/repositories/{repo_id}/status
    API_V1_REPOSITORIES_REPO_ID_STATUS: Final[str] = "/api/v1/repositories/{repo_id}/status"

    # /api/v1/repositories/{repo_id}/tags
    API_V1_REPOSITORIES_REPO_ID_TAGS: Final[str] = "/api/v1/repositories/{repo_id}/tags"

    # /api/v1/repositories/{repo_id}/tags/{tag_id}
    API_V1_REPOSITORIES_REPO_ID_TAGS_TAG_ID: Final[str] = "/api/v1/repositories/{repo_id}/tags/{tag_id}"

    # /api/v1/search
    API_V1_SEARCH: Final[str] = "/api/v1/search"

    # /healthz
    HEALTHZ: Final[str] = "/healthz"


# Generated from: openapi.json
# Total endpoints: 18
