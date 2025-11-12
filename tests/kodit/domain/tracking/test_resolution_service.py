"""Tests for TrackableResolutionService."""

from collections.abc import Callable
from datetime import UTC, datetime

import pytest
from pydantic import AnyUrl
from sqlalchemy.ext.asyncio import AsyncSession

from kodit.domain.entities.git import GitBranch, GitCommit
from kodit.domain.factories.git_repo_factory import GitRepoFactory
from kodit.domain.tracking.resolution_service import TrackableResolutionService
from kodit.domain.tracking.trackable import Trackable, TrackableReferenceType
from kodit.infrastructure.sqlalchemy.git_branch_repository import (
    create_git_branch_repository,
)
from kodit.infrastructure.sqlalchemy.git_commit_repository import (
    create_git_commit_repository,
)
from kodit.infrastructure.sqlalchemy.git_repository import create_git_repo_repository
from kodit.infrastructure.sqlalchemy.git_tag_repository import (
    create_git_tag_repository,
)


@pytest.fixture
async def test_repo(session_factory: Callable[[], AsyncSession]) -> int:
    """Create a test repository and return its ID."""
    repo_repository = create_git_repo_repository(session_factory)
    repo = GitRepoFactory.create_from_remote_uri(
        AnyUrl("https://github.com/test/repo.git")
    )
    repo = await repo_repository.save(repo)
    assert repo.id is not None
    return repo.id


@pytest.fixture
def resolution_service(
    session_factory: Callable[[], AsyncSession],
) -> TrackableResolutionService:
    """Create a TrackableResolutionService instance."""
    return TrackableResolutionService(
        commit_repo=create_git_commit_repository(session_factory),
        branch_repo=create_git_branch_repository(session_factory),
        tag_repo=create_git_tag_repository(session_factory),
    )


@pytest.mark.asyncio
async def test_walk_commit_history_with_single_commit(
    resolution_service: TrackableResolutionService,
    session_factory: Callable[[], AsyncSession],
    test_repo: int,
) -> None:
    """Test walking history with only one commit in database."""
    commit_repo = create_git_commit_repository(session_factory)
    branch_repo = create_git_branch_repository(session_factory)

    # Create a single commit with no parent
    commit = GitCommit(
        commit_sha="abc123",
        repo_id=test_repo,
        parent_commit_sha=None,
        date=datetime.now(UTC),
        message="Initial commit",
        author="Test Author",
        created_at=datetime.now(UTC),
    )
    await commit_repo.save(commit)

    # Create a branch pointing to this commit
    branch = GitBranch(
        repo_id=test_repo,
        name="main",
        head_commit_sha="abc123",
        created_at=datetime.now(UTC),
    )
    await branch_repo.save(branch)

    # Walk history
    trackable = Trackable(
        type=TrackableReferenceType.BRANCH,
        identifier="main",
        repo_id=test_repo,
    )
    commits = await resolution_service.resolve_to_commits(trackable, limit=100)

    # Should return just the one commit
    assert len(commits) == 1
    assert commits[0] == "abc123"


@pytest.mark.asyncio
async def test_walk_commit_history_stops_at_missing_parent(
    resolution_service: TrackableResolutionService,
    session_factory: Callable[[], AsyncSession],
    test_repo: int,
) -> None:
    """Test that walking history stops gracefully when parent commit is missing."""
    commit_repo = create_git_commit_repository(session_factory)
    branch_repo = create_git_branch_repository(session_factory)

    # Create HEAD commit with a parent SHA that doesn't exist in database
    head_commit = GitCommit(
        commit_sha="def456",
        repo_id=test_repo,
        parent_commit_sha="missing789",  # This parent doesn't exist
        date=datetime.now(UTC),
        message="Feature commit",
        author="Test Author",
        created_at=datetime.now(UTC),
    )
    await commit_repo.save(head_commit)

    # Create branch pointing to HEAD
    branch = GitBranch(
        repo_id=test_repo,
        name="feature",
        head_commit_sha="def456",
        created_at=datetime.now(UTC),
    )
    await branch_repo.save(branch)

    # Walk history - should stop gracefully at missing commit
    trackable = Trackable(
        type=TrackableReferenceType.BRANCH,
        identifier="feature",
        repo_id=test_repo,
    )
    commits = await resolution_service.resolve_to_commits(trackable, limit=100)

    # Should include both the HEAD commit and attempt to follow parent
    # The parent SHA is added to list before checking if it exists
    assert len(commits) == 2
    assert commits[0] == "def456"
    assert commits[1] == "missing789"


@pytest.mark.asyncio
async def test_walk_commit_history_with_complete_chain(
    resolution_service: TrackableResolutionService,
    session_factory: Callable[[], AsyncSession],
    test_repo: int,
) -> None:
    """Test walking history with a complete chain of commits."""
    commit_repo = create_git_commit_repository(session_factory)
    branch_repo = create_git_branch_repository(session_factory)

    # Create a chain: commit3 -> commit2 -> commit1 -> None
    commit1 = GitCommit(
        commit_sha="commit1",
        repo_id=test_repo,
        parent_commit_sha=None,
        date=datetime.now(UTC),
        message="First commit",
        author="Test Author",
        created_at=datetime.now(UTC),
    )
    commit2 = GitCommit(
        commit_sha="commit2",
        repo_id=test_repo,
        parent_commit_sha="commit1",
        date=datetime.now(UTC),
        message="Second commit",
        author="Test Author",
        created_at=datetime.now(UTC),
    )
    commit3 = GitCommit(
        commit_sha="commit3",
        repo_id=test_repo,
        parent_commit_sha="commit2",
        date=datetime.now(UTC),
        message="Third commit",
        author="Test Author",
        created_at=datetime.now(UTC),
    )

    await commit_repo.save(commit1)
    await commit_repo.save(commit2)
    await commit_repo.save(commit3)

    # Create branch pointing to latest commit
    branch = GitBranch(
        repo_id=test_repo,
        name="main",
        head_commit_sha="commit3",
        created_at=datetime.now(UTC),
    )
    await branch_repo.save(branch)

    # Walk history
    trackable = Trackable(
        type=TrackableReferenceType.BRANCH,
        identifier="main",
        repo_id=test_repo,
    )
    commits = await resolution_service.resolve_to_commits(trackable, limit=100)

    # Should return all three commits in order
    assert len(commits) == 3
    assert commits == ["commit3", "commit2", "commit1"]


@pytest.mark.asyncio
async def test_walk_commit_history_with_partial_chain(
    resolution_service: TrackableResolutionService,
    session_factory: Callable[[], AsyncSession],
    test_repo: int,
) -> None:
    """Test walking history stops when middle commit is missing (PR scenario)."""
    commit_repo = create_git_commit_repository(session_factory)
    branch_repo = create_git_branch_repository(session_factory)

    # Simulate PR scenario: we have the PR head commit, but not all parent commits
    # Chain should be: pr_head -> base_commit, but base_commit isn't in DB
    pr_head = GitCommit(
        commit_sha="pr_head",
        repo_id=test_repo,
        parent_commit_sha="base_commit",  # This doesn't exist in DB
        date=datetime.now(UTC),
        message="PR commit",
        author="Test Author",
        created_at=datetime.now(UTC),
    )
    await commit_repo.save(pr_head)

    # Create branch for PR
    pr_branch = GitBranch(
        repo_id=test_repo,
        name="feature-branch",
        head_commit_sha="pr_head",
        created_at=datetime.now(UTC),
    )
    await branch_repo.save(pr_branch)

    # Walk history
    trackable = Trackable(
        type=TrackableReferenceType.BRANCH,
        identifier="feature-branch",
        repo_id=test_repo,
    )
    commits = await resolution_service.resolve_to_commits(trackable, limit=100)

    # Should include PR head and attempt to follow to missing base
    assert len(commits) == 2
    assert commits[0] == "pr_head"
    assert commits[1] == "base_commit"


@pytest.mark.asyncio
async def test_walk_commit_history_respects_limit(
    resolution_service: TrackableResolutionService,
    session_factory: Callable[[], AsyncSession],
    test_repo: int,
) -> None:
    """Test that walking history respects the limit parameter."""
    commit_repo = create_git_commit_repository(session_factory)
    branch_repo = create_git_branch_repository(session_factory)

    # Create a long chain of 10 commits
    commits_to_create = []
    for i in range(10):
        commit = GitCommit(
            commit_sha=f"commit{i}",
            repo_id=test_repo,
            parent_commit_sha=f"commit{i-1}" if i > 0 else None,
            date=datetime.now(UTC),
            message=f"Commit {i}",
            author="Test Author",
            created_at=datetime.now(UTC),
        )
        commits_to_create.append(commit)

    for commit in commits_to_create:
        await commit_repo.save(commit)

    # Create branch pointing to latest
    branch = GitBranch(
        repo_id=test_repo,
        name="main",
        head_commit_sha="commit9",
        created_at=datetime.now(UTC),
    )
    await branch_repo.save(branch)

    # Walk with limit of 5
    trackable = Trackable(
        type=TrackableReferenceType.BRANCH,
        identifier="main",
        repo_id=test_repo,
    )
    commits = await resolution_service.resolve_to_commits(trackable, limit=5)

    # Should return only 5 commits
    assert len(commits) == 5
    assert commits == ["commit9", "commit8", "commit7", "commit6", "commit5"]


@pytest.mark.asyncio
async def test_pr_workflow_with_new_branch_and_commit(
    resolution_service: TrackableResolutionService,
    session_factory: Callable[[], AsyncSession],
    test_repo: int,
) -> None:
    """Test simulating a PR workflow: create branch, add commit, open PR.

    This simulates the real-world scenario where:
    1. Main branch has some commits
    2. A feature branch is created from main
    3. New commits are added to the feature branch
    4. Only the new commits on the feature branch are scanned (not the base)
    5. A PR is opened and we need to walk the feature branch history
    """
    commit_repo = create_git_commit_repository(session_factory)
    branch_repo = create_git_branch_repository(session_factory)

    # Step 1: Create main branch with initial commit (indexed)
    main_commit = GitCommit(
        commit_sha="main_base",
        repo_id=test_repo,
        parent_commit_sha=None,
        date=datetime.now(UTC),
        message="Initial main commit",
        author="Test Author",
        created_at=datetime.now(UTC),
    )
    await commit_repo.save(main_commit)

    main_branch = GitBranch(
        repo_id=test_repo,
        name="main",
        head_commit_sha="main_base",
        created_at=datetime.now(UTC),
    )
    await branch_repo.save(main_branch)

    # Step 2: Simulate feature branch creation and new commit
    # In reality, only the new feature commit would be scanned, not main_base
    feature_commit = GitCommit(
        commit_sha="feature_commit",
        repo_id=test_repo,
        parent_commit_sha="main_base",  # Points to main
        date=datetime.now(UTC),
        message="Add new feature",
        author="Test Author",
        created_at=datetime.now(UTC),
    )
    await commit_repo.save(feature_commit)

    # Step 3: Create feature branch pointing to new commit
    feature_branch = GitBranch(
        repo_id=test_repo,
        name="feature/new-feature",
        head_commit_sha="feature_commit",
        created_at=datetime.now(UTC),
    )
    await branch_repo.save(feature_branch)

    # Step 4: Open PR - walk feature branch history
    # Should successfully walk from feature_commit back to main_base
    trackable = Trackable(
        type=TrackableReferenceType.BRANCH,
        identifier="feature/new-feature",
        repo_id=test_repo,
    )
    commits = await resolution_service.resolve_to_commits(trackable, limit=100)

    # Should return both commits in the feature branch history
    assert len(commits) == 2
    assert commits[0] == "feature_commit"
    assert commits[1] == "main_base"


@pytest.mark.asyncio
async def test_pr_workflow_with_partial_history(
    resolution_service: TrackableResolutionService,
    session_factory: Callable[[], AsyncSession],
    test_repo: int,
) -> None:
    """Test PR workflow when base branch commits aren't fully indexed.

    This simulates the scenario that caused the original bug:
    1. Main branch exists but only HEAD is indexed
    2. Feature branch is created and only its HEAD is indexed
    3. Both branches point to the same commit (just scanned)
    4. Walking feature branch tries to access parent commits that don't exist
    """
    commit_repo = create_git_commit_repository(session_factory)
    branch_repo = create_git_branch_repository(session_factory)

    # Only the latest commit is indexed (simulating incremental indexing)
    latest_commit = GitCommit(
        commit_sha="latest_sha",
        repo_id=test_repo,
        parent_commit_sha="older_sha",  # This parent was NOT indexed
        date=datetime.now(UTC),
        message="Latest commit",
        author="Test Author",
        created_at=datetime.now(UTC),
    )
    await commit_repo.save(latest_commit)

    # Both main and feature branch point to same commit
    # (This happens when feature branch is just created from main)
    main_branch = GitBranch(
        repo_id=test_repo,
        name="main",
        head_commit_sha="latest_sha",
        created_at=datetime.now(UTC),
    )
    await branch_repo.save(main_branch)

    feature_branch = GitBranch(
        repo_id=test_repo,
        name="feature/branch",
        head_commit_sha="latest_sha",
        created_at=datetime.now(UTC),
    )
    await branch_repo.save(feature_branch)

    # Walking either branch should work without errors
    # Should include the commit and its missing parent
    for branch_name in ["main", "feature/branch"]:
        trackable = Trackable(
            type=TrackableReferenceType.BRANCH,
            identifier=branch_name,
            repo_id=test_repo,
        )
        commits = await resolution_service.resolve_to_commits(trackable, limit=100)

        # Should return the indexed commit and stop at the missing parent
        assert len(commits) == 2
        assert commits[0] == "latest_sha"
        assert commits[1] == "older_sha"  # Parent is included but not in DB
