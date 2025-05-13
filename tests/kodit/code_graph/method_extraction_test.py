"""Test the method extraction functionality."""

from pathlib import Path

from kodit.code_graph.method_extraction import MethodParser


def test_extract_methods() -> None:
    """Test the method extraction functionality."""
    with Path(__file__).parent.joinpath("python.py").open("rb") as f:
        source_code = f.read()
    parser = MethodParser(source_code, "python")
    extracted_methods = list(parser.extract())

    for method in extracted_methods:
        print(method)  # noqa: T201
        print("-" * 40)  # noqa: T201

    assert len(extracted_methods) == 5

    # Verify each method contains its imports and class context if applicable
    for method in extracted_methods:
        assert "import os" in method
        assert "from typing import List" in method

    # Verify helper_function extraction
    helper_func = next(m for m in extracted_methods if "helper_function" in m)
    assert "def helper_function(x: List[str]) -> str:" in helper_func
    assert 'return " ".join(x)' in helper_func

    # Verify MyClass methods
    class_methods = [m for m in extracted_methods if "MyClass:" in m]
    assert len(class_methods) == 3

    # Verify main function
    main_func = next(m for m in extracted_methods if "main" in m)
    assert "def main():" in main_func
    assert "obj = MyClass(42)" in main_func
    assert "return result" in main_func


def test_extract_csharp_methods() -> None:
    """Test the C# method extraction functionality."""
    with Path(__file__).parent.joinpath("csharp.cs").open("rb") as f:
        source_code = f.read()
    parser = MethodParser(source_code, "csharp")
    extracted_methods = list(parser.extract())

    for method in extracted_methods:
        print(method)  # noqa: T201
        print("-" * 40)  # noqa: T201

    assert (
        len(extracted_methods) == 6
    )  # HelperFunction, constructor, GetValue, PrintValue, Main

    # Verify each method contains its using statements
    for method in extracted_methods:
        assert "using System;" in method
        assert "using System.Collections.Generic;" in method
        assert "using System.IO;" in method

    # Verify HelperFunction extraction
    helper_func = next(m for m in extracted_methods if "HelperFunction" in m)
    assert "public static string HelperFunction(List<string> x)" in helper_func
    assert 'return string.Join(" ", x)' in helper_func

    # Verify MyClass methods
    class_methods = [m for m in extracted_methods if "MyClass" in m]
    assert len(class_methods) == 3  # constructor, GetValue, PrintValue

    # Verify constructor
    constructor = next(m for m in extracted_methods if "MyClass(" in m and "value" in m)
    assert "public MyClass(int value)" in constructor
    assert "this.value = value" in constructor

    # Verify GetValue method
    get_value = next(m for m in extracted_methods if "GetValue" in m)
    assert "public List<string> GetValue()" in get_value
    assert "Directory.GetFiles" in get_value

    # Verify PrintValue method
    print_value = next(m for m in extracted_methods if "PrintValue" in m)
    assert "public void PrintValue()" in print_value
    assert "Console.WriteLine" in print_value

    # Verify Main function
    main_func = next(m for m in extracted_methods if "Main" in m)
    assert "public static string Main()" in main_func
    assert "var obj = new MyClass(42)" in main_func
    assert "return result" in main_func
