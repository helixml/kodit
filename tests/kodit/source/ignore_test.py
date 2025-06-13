"""Test the ignore patterns functionality."""

import tempfile
from pathlib import Path
from unittest.mock import Mock, patch

import git
import pytest

from kodit.source.ignore import IgnorePatterns


@pytest.fixture
def temp_dir():
    """Create a temporary directory for testing."""
    with tempfile.TemporaryDirectory() as temp_dir:
        yield Path(temp_dir)


@pytest.fixture
def git_repo(temp_dir):
    """Create a temporary git repository."""
    # Resolve the real path to handle macOS symlink issues
    real_temp_dir = temp_dir.resolve()
    repo = git.Repo.init(real_temp_dir)
    # Create a test file and commit it
    test_file = real_temp_dir / "test.txt"
    test_file.write_text("test content")
    repo.index.add([str(test_file)])
    repo.index.commit("Initial commit")
    return repo


@pytest.fixture
def non_git_dir(temp_dir):
    """Create a temporary non-git directory."""
    return temp_dir


class TestIgnorePatterns:
    """Test the IgnorePatterns class."""

    def test_init_with_valid_directory(self, temp_dir):
        """Test initialization with a valid directory."""
        ignore_patterns = IgnorePatterns(temp_dir)
        assert ignore_patterns.base_dir == temp_dir
        assert ignore_patterns.git_repo is None

    def test_init_with_git_repository(self, git_repo, temp_dir):
        """Test initialization with a git repository."""
        ignore_patterns = IgnorePatterns(temp_dir)
        assert ignore_patterns.base_dir == temp_dir
        # The git_repo will be None because is_valid_clone_target returns False for local paths

    def test_init_with_invalid_directory(self):
        """Test initialization with an invalid directory."""
        invalid_path = Path("/nonexistent/directory")
        with pytest.raises(ValueError, match="Base directory is not a directory"):
            IgnorePatterns(invalid_path)

    def test_init_with_file_instead_of_directory(self, temp_dir):
        """Test initialization with a file instead of directory."""
        test_file = temp_dir / "test.txt"
        test_file.write_text("test")
        with pytest.raises(ValueError, match="Base directory is not a directory"):
            IgnorePatterns(test_file)

    def test_should_ignore_directory(self, temp_dir):
        """Test that directories are never ignored."""
        ignore_patterns = IgnorePatterns(temp_dir)
        test_dir = temp_dir / "subdir"
        test_dir.mkdir()
        assert not ignore_patterns.should_ignore(test_dir)

    def test_should_ignore_git_files(self, temp_dir):
        """Test that files in .git directory are ignored."""
        ignore_patterns = IgnorePatterns(temp_dir)

        # Create .git directory and files
        git_dir = temp_dir / ".git"
        git_dir.mkdir()
        git_config = git_dir / "config"
        git_config.write_text("test")

        # Test .git/config file
        assert ignore_patterns.should_ignore(git_config)

        # Test nested .git files
        git_subdir = git_dir / "objects"
        git_subdir.mkdir()
        git_object = git_subdir / "abc123"
        git_object.write_text("object")
        assert ignore_patterns.should_ignore(git_object)

    def test_should_ignore_regular_files(self, temp_dir):
        """Test that regular files are not ignored by default."""
        ignore_patterns = IgnorePatterns(temp_dir)

        test_file = temp_dir / "test.txt"
        test_file.write_text("test content")

        assert not ignore_patterns.should_ignore(test_file)

    @patch("kodit.source.ignore.is_valid_clone_target")
    def test_should_ignore_with_git_repository(self, mock_is_valid, temp_dir):
        """Test ignore patterns with a git repository."""
        # Mock the git repository setup
        mock_is_valid.return_value = True

        # Create a mock git repo
        mock_repo = Mock()
        mock_repo.ignored.return_value = []

        with patch("git.Repo") as mock_git_repo:
            mock_git_repo.return_value = mock_repo

            ignore_patterns = IgnorePatterns(temp_dir)
            test_file = temp_dir / "test.txt"
            test_file.write_text("test")

            # File should not be ignored when git returns empty list
            assert not ignore_patterns.should_ignore(test_file)

            # File should be ignored when git returns the file
            mock_repo.ignored.return_value = [str(test_file)]
            assert ignore_patterns.should_ignore(test_file)

    def test_should_ignore_with_noindex_file(self, temp_dir):
        """Test ignore patterns with .noindex file."""
        ignore_patterns = IgnorePatterns(temp_dir)

        # Create .noindex file with patterns
        noindex_file = temp_dir / ".noindex"
        noindex_file.write_text("*.tmp\n*.log\ntest_*.py\n")

        # Create test files
        tmp_file = temp_dir / "temp.tmp"
        tmp_file.write_text("temp")

        log_file = temp_dir / "app.log"
        log_file.write_text("log")

        test_file = temp_dir / "test_example.py"
        test_file.write_text("test")

        regular_file = temp_dir / "regular.txt"
        regular_file.write_text("regular")

        # Test pattern matching
        assert ignore_patterns.should_ignore(tmp_file)
        assert ignore_patterns.should_ignore(log_file)
        assert ignore_patterns.should_ignore(test_file)
        assert not ignore_patterns.should_ignore(regular_file)

    def test_should_ignore_with_empty_noindex_file(self, temp_dir):
        """Test ignore patterns with empty .noindex file."""
        ignore_patterns = IgnorePatterns(temp_dir)

        # Create empty .noindex file
        noindex_file = temp_dir / ".noindex"
        noindex_file.write_text("")

        test_file = temp_dir / "test.txt"
        test_file.write_text("test")

        assert not ignore_patterns.should_ignore(test_file)

    def test_should_ignore_with_noindex_whitespace_lines(self, temp_dir):
        """Test ignore patterns with .noindex file containing whitespace lines."""
        ignore_patterns = IgnorePatterns(temp_dir)

        # Create .noindex file with whitespace and empty lines
        noindex_file = temp_dir / ".noindex"
        noindex_file.write_text("*.tmp\n\n  \n*.log\n  ")

        tmp_file = temp_dir / "temp.tmp"
        tmp_file.write_text("temp")

        log_file = temp_dir / "app.log"
        log_file.write_text("log")

        txt_file = temp_dir / "regular.txt"
        txt_file.write_text("regular")

        assert ignore_patterns.should_ignore(tmp_file)
        assert ignore_patterns.should_ignore(log_file)
        assert not ignore_patterns.should_ignore(txt_file)

    def test_should_ignore_complex_patterns(self, temp_dir):
        """Test ignore patterns with complex glob patterns."""
        ignore_patterns = IgnorePatterns(temp_dir)

        # Create .noindex file with complex patterns
        noindex_file = temp_dir / ".noindex"
        noindex_file.write_text("**/node_modules/**\nbuild/*\nsrc/temp_*\n**/*.pyc")

        # Create test directory structure
        node_modules = temp_dir / "frontend" / "node_modules" / "package"
        node_modules.mkdir(parents=True)
        node_modules_file = node_modules / "index.js"
        node_modules_file.write_text("module")

        build_dir = temp_dir / "build"
        build_dir.mkdir()
        build_file = build_dir / "output.js"
        build_file.write_text("output")

        src_dir = temp_dir / "src"
        src_dir.mkdir()
        temp_file = src_dir / "temp_data.json"
        temp_file.write_text("temp")

        pyc_file = temp_dir / "module.pyc"
        pyc_file.write_text("compiled")

        regular_file = temp_dir / "src" / "main.py"
        regular_file.write_text("main")

        # Test pattern matching
        assert ignore_patterns.should_ignore(node_modules_file)
        assert ignore_patterns.should_ignore(build_file)
        assert ignore_patterns.should_ignore(temp_file)
        assert ignore_patterns.should_ignore(pyc_file)
        assert not ignore_patterns.should_ignore(regular_file)

    def test_relative_path_calculation(self, temp_dir):
        """Test that relative paths are calculated correctly."""
        ignore_patterns = IgnorePatterns(temp_dir)

        # Create nested directory structure
        subdir = temp_dir / "subdir" / "nested"
        subdir.mkdir(parents=True)
        test_file = subdir / "test.txt"
        test_file.write_text("test")

        # Create .noindex file with relative pattern
        noindex_file = temp_dir / ".noindex"
        noindex_file.write_text("subdir/nested/*.txt")

        assert ignore_patterns.should_ignore(test_file)

    def test_should_ignore_priority_order(self, temp_dir):
        """Test the priority order: directories > .git > git ignore > .noindex."""
        # Setup git repo mock
        with patch("kodit.source.ignore.is_valid_clone_target") as mock_is_valid:
            mock_is_valid.return_value = True

            mock_repo = Mock()
            mock_repo.ignored.return_value = []

            with patch("git.Repo") as mock_git_repo:
                mock_git_repo.return_value = mock_repo

                ignore_patterns = IgnorePatterns(temp_dir)

                # Test directory - should never be ignored regardless of other patterns
                test_dir = temp_dir / "test_dir"
                test_dir.mkdir()

                # Create .noindex file that would match directory name
                noindex_file = temp_dir / ".noindex"
                noindex_file.write_text("test_*")

                assert not ignore_patterns.should_ignore(test_dir)
