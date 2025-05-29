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

## Configuring Kodit

Configuration of Kodit is performed by setting environmental variables or adding
variables to a .env file.

{{< warn >}}
Note that updating a setting does not automatically update the data that uses that
setting. For example, if you change a provider, you will need to delete and
recreate all indexes.
{{< /warn >}}

### Indexing

#### Default Provider

By default, Kodit will use small local models for semantic search and enrichment. If you
are using Kodit in a professional capacity, it is likely that the local model latency is
too high to provide a good developer experience.

Instead, you should use an external provider. The settings provided here will cause all
embedding and enrichments request to be sent to this provider by default. You can
override the provider used for each task if you wish. (Coming soon!)

##### OpenAI

Add the following settings to your .env file, or export them as environmental variables:

```bash
DEFAULT_ENDPOINT_BASE_URL=https://api.openai.com/v1
DEFAULT_ENDPOINT_API_KEY=sk-xxxxxx
```

## Managing Kodit

There is limited management functionality at this time. To delete indexes you must
delete the database and/or tables.
