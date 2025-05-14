"""Test the method extraction functionality."""

import inspect
from pathlib import Path

from kodit.snippets.languages import detect_language
from kodit.snippets.method_snippets import MethodSnippets


def test_extract_methods() -> None:
    """Test the method extraction functionality."""
    source_code = (Path(__file__).parent / "python.py").read_bytes()

    # Query to capture function definitions, bodies, and imports
    file_path = inspect.getfile(detect_language)
    python_query = (Path(file_path).parent / "python.scm").read_text()

    analyzer = MethodSnippets("python", python_query)
    extracted_methods = analyzer.extract(source_code)

    for method in extracted_methods:
        print(method)  # noqa: T201
        print("-" * 40)  # noqa: T201

    assert len(extracted_methods) == 5

    # Verify each method contains its imports and class context if applicable
    for method in extracted_methods:
        assert "import os" in method
        assert "from typing import List" in method

    # Verify MyClass methods
    class_methods = [m for m in extracted_methods if "MyClass:" in m]
    assert len(class_methods) == 3

    # Verify main function
    main_funcs = [m for m in extracted_methods if "main" in m]
    assert len(main_funcs) == 1
    main_func = main_funcs[0]
    assert "def main():" in main_func
    assert "obj = MyClass(42)" in main_func
    assert "return result" in main_func


def test_extract_csharp_methods() -> None:
    """Test the C# method extraction functionality."""
    source_code = (Path(__file__).parent / "csharp.cs").read_bytes()

    # Query to capture function definitions, bodies, and imports
    file_path = inspect.getfile(detect_language)
    csharp_query = (Path(file_path).parent / "csharp.scm").read_text()

    analyzer = MethodSnippets("csharp", csharp_query)
    extracted_methods = analyzer.extract(source_code)

    for method in extracted_methods:
        print(method)  # noqa: T201
        print("-" * 40)  # noqa: T201

    assert len(extracted_methods) == 4

    # Verify each method contains its using statements
    for method in extracted_methods:
        assert "using System;" in method
        assert "using System.Collections.Generic;" in method
        assert "using System.IO;" in method

    # Verify HelperFunction extraction
    helper_funcs = [m for m in extracted_methods if "HelperFunction" in m]
    assert len(helper_funcs) == 2
    helper_func = helper_funcs[0]
    assert "public static string HelperFunction(List<string> x)" in helper_func
    assert 'return string.Join(" ", x)' in helper_func

    # Verify MyClass methods
    class_methods = [m for m in extracted_methods if "MyClass" in m]
    assert len(class_methods) == 3  # constructor, GetValue, PrintValue

    # # Verify constructor
    # constructors = [m for m in extracted_methods if "MyClass(" in m and "value" in m]
    # assert len(constructors) == 1
    # constructor = constructors[0]
    # assert "public MyClass(int value)" in constructor
    # assert "this.value = value" in constructor

    # Verify GetValue method
    get_value_methods = [
        m for m in extracted_methods if "public List<string> GetValue()" in m
    ]
    assert len(get_value_methods) == 1
    get_value = get_value_methods[0]
    assert "public List<string> GetValue()" in get_value
    assert "Directory.GetFiles" in get_value

    # Verify PrintValue method
    print_value_methods = [m for m in extracted_methods if "PrintValue" in m]
    assert len(print_value_methods) == 1
    print_value = print_value_methods[0]
    assert "public void PrintValue()" in print_value
    assert "Console.WriteLine" in print_value

    # Verify Main function
    main_funcs = [m for m in extracted_methods if "Main" in m]
    assert len(main_funcs) == 1
    main_func = main_funcs[0]
    assert "public static string Main()" in main_func
    assert "var obj = new MyClass(42)" in main_func
    assert "return result" in main_func
