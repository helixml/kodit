"""Test data factory for creating sample git repositories."""

from datetime import UTC, datetime, timedelta

from pydantic import AnyUrl

from kodit.domain.entities import Author, File, GitBranch, GitCommit, GitFile, GitRepo
from kodit.domain.value_objects import FileProcessingStatus


class GitTestDataFactory:
    """Factory for creating test git repository data."""

    @staticmethod
    def create_sample_commit(
        sha: str = "abc123def456",
        message: str = "Add new feature",
        parent_sha: str = "",
        date: datetime | None = None,
        file_count: int = 3,
        author: str = "Test Author <test@example.com>",
    ) -> GitCommit:
        """Create a sample git commit."""
        if date is None:
            date = datetime.now(UTC)

        files = []
        for i in range(file_count):
            file_entity = GitFile(
                blob_sha=f"blob_sha_{i}",
                path=f"/test/repo/file{i}.py",
                mime_type="text/x-python",
                size=100 + i,  # Some fake size
            )
            files.append(file_entity)

        return GitCommit(
            commit_sha=sha,
            date=date,
            message=message,
            parent_commit_sha=parent_sha,
            files=files,
            author=author,
        )

    @staticmethod
    def create_sample_branch(
        name: str = "main",
        head_commit: GitCommit | None = None,
    ) -> GitBranch:
        """Create a sample git branch."""
        if head_commit is None:
            head_commit = GitTestDataFactory.create_sample_commit()

        return GitBranch(
            name=name,
            head_commit=head_commit,
        )

    @staticmethod
    def create_sample_repository(
        num_branches: int = 3,
        commits_per_branch: int = 5,
    ) -> GitRepo:
        """Create a sample git repository with branches and commits."""
        branches = []
        all_commits = []  # Track all commits across all branches
        base_date = datetime.now(UTC)

        # Create main branch with commit history
        main_commits = []
        for i in range(commits_per_branch):
            commit_date = base_date - timedelta(days=i)
            parent_sha = main_commits[-1].commit_sha if main_commits else ""

            commit = GitTestDataFactory.create_sample_commit(
                sha=f"main_{i:03d}_{'a' * 37}",
                message=f"Main branch commit {i + 1}",
                parent_sha=parent_sha,
                date=commit_date,
                file_count=2 + (i % 3),
            )
            main_commits.append(commit)
            all_commits.append(commit)

        main_branch = GitBranch(
            name="main",
            head_commit=main_commits[0],  # Most recent commit
        )
        branches.append(main_branch)

        # Create feature branches
        for branch_num in range(1, num_branches):
            branch_commits = []

            # Branch off from main at some point
            branch_point = main_commits[min(2, len(main_commits) - 1)]

            for i in range(min(3, commits_per_branch)):
                commit_date = base_date - timedelta(days=branch_num, hours=i * 6)
                parent_sha = (
                    branch_commits[-1].commit_sha
                    if branch_commits
                    else branch_point.commit_sha
                )

                commit = GitTestDataFactory.create_sample_commit(
                    sha=f"feature{branch_num}_{i:03d}_{'b' * 34}",
                    message=f"Feature {branch_num} - change {i + 1}",
                    parent_sha=parent_sha,
                    date=commit_date,
                    file_count=1 + (i % 2),
                )
                branch_commits.append(commit)
                all_commits.append(commit)

            feature_branch = GitBranch(
                name=f"feature-{branch_num}",
                head_commit=branch_commits[0] if branch_commits else branch_point,
            )
            branches.append(feature_branch)

        # Need to provide all required fields for GitRepo
        from pathlib import Path
        
        return GitRepo(
            sanitized_remote_uri=AnyUrl("https://github.com/test/sample.git"),
            branches=branches,
            commits=all_commits,
            tracking_branch=main_branch,
            cloned_path=Path("/tmp/test-repo"),
            remote_uri=AnyUrl("https://github.com/test/sample.git"),
            last_scanned_at=datetime.now(UTC),
            total_unique_commits=len(all_commits),
        )

    @staticmethod
    def create_sample_repositories() -> dict[str, GitRepo]:
        """Create a collection of sample repositories."""
        repos = {}

        # Small repository
        small_repo = GitTestDataFactory.create_sample_repository(
            num_branches=2,
            commits_per_branch=3,
        )
        repos["/home/user/projects/small-project"] = small_repo

        # Medium repository
        medium_repo = GitTestDataFactory.create_sample_repository(
            num_branches=5,
            commits_per_branch=10,
        )
        repos["/home/user/projects/medium-project"] = medium_repo

        # Large repository
        large_repo = GitTestDataFactory.create_sample_repository(
            num_branches=10,
            commits_per_branch=20,
        )
        repos["/home/user/projects/large-project"] = large_repo

        # Customize the large repo to have more realistic branch names
        large_repo = repos["/home/user/projects/large-project"]
        for i, branch in enumerate(large_repo.branches[1:], 1):
            if i <= 3:
                branch.name = f"feature/PROJ-{1000 + i}"
            elif i <= 6:
                branch.name = f"bugfix/issue-{200 + i}"
            elif i <= 8:
                branch.name = f"release/v1.{i - 6}.0"
            else:
                branch.name = f"hotfix/critical-fix-{i}"

        return repos
