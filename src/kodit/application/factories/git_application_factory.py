"""Factory for creating GitApplicationService instances."""

from kodit.application.services.git_application_service import GitApplicationService
from kodit.config import AppContext
from kodit.domain.services.git_repository_scanner import (
    GitRepositoryScanner,
    RepositoryCloner,
)
from kodit.infrastructure.cloning.git.git_python_adaptor import GitPythonAdapter
from kodit.infrastructure.memory.in_memory_git_repository import (
    InMemoryGitBranchRepository,
    InMemoryGitCommitRepository,
    InMemoryGitRepoRepository,
)


def create_git_application_service(
    app_context: AppContext,
) -> GitApplicationService:
    """Create a GitApplicationService instance."""
    repo_repository = InMemoryGitRepoRepository()
    branch_repository = InMemoryGitBranchRepository()
    commit_repository = InMemoryGitCommitRepository(branch_repository)
    git_adapter = GitPythonAdapter()
    scanner = GitRepositoryScanner(git_adapter)
    cloner = RepositoryCloner(git_adapter, app_context.get_clone_dir())
    return GitApplicationService(
        repo_repository=repo_repository,
        commit_repository=commit_repository,
        branch_repository=branch_repository,
        scanner=scanner,
        cloner=cloner,
        git_adapter=git_adapter,
    )
