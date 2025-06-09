---
title: Quick Start
description: The quickest way to get started with Kodit.
weight: 2
---

Kodit has two key components. A configuration CLI to manage what gets indexed and an MCP
server to expose your code to an AI coding assistant.

## 1. Index a source

1. Index a local path
  
    ```sh
    kodit index /path/to/your/code
    ```

2. or index a public git repository

    ```sh
    kodit index https://github.com/pydantic/pydantic-ai
    ```

## 2. Manually search your index

1. Search with a semantic description of the code:

    ```sh
    kodit search text "an example function"
    ```

2. or with a keyword
  
    ```sh
    kodit search keyword "test"
    ```

3. or with code
  
    ```sh
    kodit search code "def main()"
    ```

4. or via hybrid search
  
    ```sh
    kodit search code hybrid --keywords "main" --code "def main()" --text "example main function"
    ```

## 3. Start an MCP server

```sh
kodit serve
```

Now [add the Kodit MCP server to your AI coding assistant](../integration/index.md).
