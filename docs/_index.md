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

### Integrating Kodit with Coding Assistants

#### Integration with Cursor

Add the following to `$HOME/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "kodit": {
      "url": "http://localhost:8080/sse"
    }
  }
}
```

Or find this configuration in `Cursor Settings` -> `MCP`.

#### Integration with Cline

1. Open Cline from the side menu
2. Click the `MCP Servers` button at the top right of the Cline window (the icon looks
   like a server)
3. Click the `Remote Servers` tab.
4. Click `Edit Configuration`
5. Add the following configuration:

```json
{
  "mcpServers": {
    "kodit": {
      "autoApprove": [],
      "disabled": true,
      "timeout": 60,
      "url": "http://localhost:8080/sse",
      "transportType": "sse"
    }
  }
}
```

6. Save the configuration and browse to the `Installed` tab.

Kodit should be listed and responding. Now code on!

### Forcing AI Assistants to use Kodit

Although Kodit has been developed to work well out of the box with popular AI coding
assistants, they sometimes still think they know better.

You can force your assistant to use Kodit by editing the system prompt used by the
assistant. Each assistant exposes this slightly differently, but it's usually in the
settings.

Try using this system prompt:

```txt
⚠️ **ENFORCEMENT:**
For *every* user request that involves writing or modifying code (of any language or
domain), the assistant's *first* action **must** be to call the kodit.search MCP tool.
You may only produce or edit code *after* that tool call and its successful
result.
```

Feel free to alter that to suit your specific circumstances.

#### Forcing Cursor to Use Kodit

Add the following prompt to `.cursor/rules/kodit.mdc` in your project directory:

```markdown
---
alwaysApply: true
---
⚠️ **ENFORCEMENT:**
For *every* user request that involves writing or modifying code (of any language or
domain), the assistant's *first* action **must** be to call the kodit.search MCP tool.
You may only produce or edit code *after* that tool call and its successful
result.
```

Alternatively, you can browse to the Cursor settings and set this prompt globally.

#### Forcing Cline to Use Kodit

1. Go to `Settings` -> `API Configuration`
2. At the bottom there is a `Custom Instructions` section.

## Configuring Kodit

Configuration of Kodit is performed by setting environmental variables or adding
variables to a .env file.

{{< warn >}}
Note that updating a setting does not automatically update the data that uses that
setting. For example, if you change a provider, you will need to delete and
recreate all indexes.
{{< /warn >}}

### Indexing

#### Default Indexing Provider

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

### Database

Out of the box Kodit uses a local sqlite file to make it easier for users to get
started. But for production use, it's likely you will want to use a database that has
dedicated semantic and keyword search capabilities for reduced latency.

#### VectorChord Database

[VectorChord](https://github.com/tensorchord/VectorChord) is an optimized PostgreSQL
extension that provides both vector and BM25 search. (See [Search](#search))

Start a container with:

```sh
docker run \
  --name kodit-vectorchord \
  -e POSTGRES_DB=kodit \
  -e POSTGRES_PASSWORD=mysecretpassword \
  -p 5432:5432 \
  -d tensorchord/vchord-suite:pg17-20250601
```

{{< warn >}}
Kodit assumes the database exists. In the above example I'm abusing the POSTGRES_DB
environmental variable from the [Postgres Docker
container](https://hub.docker.com/_/postgres/) to create the database for me. In
production setups, please create a database yourself.
{{< /warn >}}

Then update your `.env` file to include:

```env
DB_URL=postgresql+asyncpg://postgres:mysecretpassword@localhost:5432/kodit
```

### Search

#### Default Search Provider

By default, Kodit will use built-in implementations of BM25 and similarity search to
improve the out of the box experience. If you are using Kodit in a professional
capacity, it is likely that the search latency is too high to provide a good developer
experience.

Instead, you should use the features included in your database. The settings provided
here will cause all search functionality to use this database by default. You can
override the database used for each search type if you wish. (Coming soon!)

##### VectorChord Search

Configure Kodit to use a [VectorChord database](#vectorchord-database).

Then update your `.env` file to include:

```env
DB_URL=postgresql+asyncpg://postgres:mysecretpassword@localhost:5432/kodit
DEFAULT_SEARCH_PROVIDER=vectorchord
```

### Enrichment

#### Default Enrichment Provider

The default enrichment provider is the same as [the default indexing provider](#default-indexing-provider).

## Managing Kodit

There is limited management functionality at this time. To delete indexes you must
delete the database and/or tables.
