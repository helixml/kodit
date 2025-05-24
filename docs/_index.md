---
title: "kodit: Code Indexing MCP Server"
linkTitle: kodit Docs
cascade:
  type: docs
menu:
  main:
    name: kodit Docs
    weight: 3
# next: /helix/getting-started
weight: 1
aliases:
- /coda
---

## Installation

Please choose your preferred installation method. They all ultimately install the kodit
cli, which contains the kodit MCP server and other tools to manage your data sources.

### Docker

```sh
docker run -it --rm registry.helix.ml/helix/kodit:latest
```

Always replace latest with a specific version.

### pipx

```sh
pipx install kodit
```

### homebrew

```sh
brew install helixml/kodit/kodit
```

### uv

```sh
uv tool install kodit
```

### pip

Use this if you want to use kodit as a python library:

```sh
pip install kodit
```

## Quick Start

Kodit has two key parts. A configuration CLI to manage what gets indexed and an MCP
server to expose your code to an AI coding assistant.

1. Index a source:
    1. a local path: `kodit index /path/to/your/code`
    2. or index a public git repository: `kodit index https://github.com/pydantic/pydantic-ai`
2. Manually search your index:
    1. with a keyword: `kodit search keyword "test"`
    2. or with code: `kodit search code "def main()"`
    3. or via hybrid search:  `kodit search code hybrid --keywords "main" --code "def main()"`
3. Start an MCP server: `kodit serve`

Now add the Kodit MCP server to your AI coding assistant.

### Integration with Cursor

Add the following to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "code-search": {
      "url": "http://localhost:8080/sse"
    }
  }
}
```

Now add the following prompt to `.cursor/rules/kodit.mdc` in your project directory:

```markdown
---
description: 
globs: 
alwaysApply: true
---
You MUST use the code-search MCP tool and always include any file context the user has provided in the chat.
```

Alternatively, you can browse to the cursor settings and set this prompt globally.
