"""Re-export all entities from the main entities module."""

# We need to import from the sibling entities.py module
# Use importlib to directly load the entities.py module to avoid circular imports
import importlib.util
from pathlib import Path

# Construct path to the sibling entities.py file
_current_dir = Path(__file__).parent
_entities_py_path = _current_dir.parent / "entities.py"

# Load the entities.py module directly
_spec = importlib.util.spec_from_file_location("_entities", _entities_py_path)
if _spec is None or _spec.loader is None:
    raise ImportError(f"Could not load entities module from {_entities_py_path}")
_entities = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_entities)

# Re-export all the classes from entities.py
Author = _entities.Author
File = _entities.File
IgnorePatternProvider = _entities.IgnorePatternProvider
Index = _entities.Index
Snippet = _entities.Snippet
SnippetWithContext = _entities.SnippetWithContext
Source = _entities.Source
Task = _entities.Task
TaskStatus = _entities.TaskStatus
WorkingCopy = _entities.WorkingCopy

__all__ = [
    "Author",
    "File",
    "IgnorePatternProvider",
    "Index",
    "Snippet",
    "SnippetWithContext",
    "Source",
    "Task",
    "TaskStatus",
    "WorkingCopy",
]
