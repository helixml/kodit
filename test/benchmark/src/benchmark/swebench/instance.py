"""SWE-bench instance representation."""

from dataclasses import dataclass, field


@dataclass(frozen=True)
class SWEBenchInstance:
    """A single SWE-bench task instance."""

    instance_id: str
    repo: str
    base_commit: str
    problem_statement: str
    patch: str
    test_patch: str
    version: str
    environment_setup_commit: str
    hints_text: str = ""
    fail_to_pass: tuple[str, ...] = field(default_factory=tuple)
    pass_to_pass: tuple[str, ...] = field(default_factory=tuple)

    def as_dict(self) -> dict:
        """Convert to dictionary for JSON serialization."""
        return {
            "instance_id": self.instance_id,
            "repo": self.repo,
            "base_commit": self.base_commit,
            "problem_statement": self.problem_statement,
            "hints_text": self.hints_text,
            "patch": self.patch,
            "test_patch": self.test_patch,
            "fail_to_pass": list(self.fail_to_pass),
            "pass_to_pass": list(self.pass_to_pass),
            "version": self.version,
            "environment_setup_commit": self.environment_setup_commit,
        }

    @classmethod
    def from_dict(cls, data: dict) -> "SWEBenchInstance":
        """Create instance from dictionary."""
        return cls(
            instance_id=data["instance_id"],
            repo=data["repo"],
            base_commit=data["base_commit"],
            problem_statement=data["problem_statement"],
            hints_text=data.get("hints_text", ""),
            patch=data["patch"],
            test_patch=data["test_patch"],
            fail_to_pass=tuple(data.get("fail_to_pass", [])),
            pass_to_pass=tuple(data.get("pass_to_pass", [])),
            version=data["version"],
            environment_setup_commit=data["environment_setup_commit"],
        )
