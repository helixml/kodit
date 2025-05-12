from typing import Dict, Optional
import os

# Mapping of file extensions to programming languages
LANGUAGE_MAP: Dict[str, str] = {
    # JavaScript/TypeScript
    "js": "javascript",
    "jsx": "javascript",
    "ts": "typescript",
    "tsx": "typescript",
    
    # Python
    "py": "python",
    
    # Rust
    "rs": "rust",
    
    # Go
    "go": "go",
    
    # C/C++
    "cpp": "cpp",
    "hpp": "cpp",
    "c": "c",
    "h": "c",
    
    # C#
    "cs": "csharp",
    
    # Ruby
    "rb": "ruby",
    
    # Java
    "java": "java",
    
    # PHP
    "php": "php",
    
    # Swift
    "swift": "swift",
    
    # Kotlin
    "kt": "kotlin"
}

def detect_language(file_path: str) -> Optional[str]:
    """
    Detect the programming language of a file based on its extension.
    
    Args:
        file_path (str): Path to the file
        
    Returns:
        Optional[str]: The detected programming language or None if the extension is not supported
        
    Raises:
        ValueError: If file_path is empty or None
        
    Example:
        >>> detect_language("example.py")
        'python'
        >>> detect_language("unknown.xyz")
        None
    """
    if file_path is None or not file_path.strip():
        raise ValueError("File path cannot be empty or None")
        
    # Get the file extension and convert to lowercase
    _, ext = os.path.splitext(file_path)
    ext = ext.lstrip('.').lower()
    
    # Return the corresponding language or None if not found
    return LANGUAGE_MAP.get(ext)