<p align="center">
    <a href="https://docs.helix.ml/kodit/"><img src="https://docs.helix.ml/images/helix-kodit-logo.png" alt="Helix Kodit Logo" width="300"></a>
</p>

<h1 align="center">
Kodit: A Code Indexing MCP Server
</h1>

<p align="center">
Kodit connects your AI coding assistant to external codebases to provide accurate and up-to-date snippets of code.
</p>

<div align="center">

[![Documentation](https://img.shields.io/badge/Documentation-6B46C1?style=for-the-badge&logo=readthedocs&logoColor=white)](https://docs.helix.ml/kodit/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=for-the-badge)](./LICENSE)
[![Discussions](https://img.shields.io/badge/Discussions-181717?style=for-the-badge&logo=github&logoColor=white)](https://github.com/helixml/kodit/discussions)

</div>

**Helix Kodit** is an **MCP server** that connects your AI coding assistant to external codebases. It can:

- Improve your AI-assisted code by providing canonical examples direct from the source
- Index local and public codebases
- Integrates with any AI coding assistant via MCP
- Search using keyword and semantic search
- Integrate with any OpenAI-compatible or custom API/model

If you're an engineer working with AI-powered coding assistants, Kodit helps by
providing relevant and up-to-date examples of your task so that LLMs make less mistakes
and produce fewer hallucinations.

## ✨ Features

### Codebase Indexing

Kodit connects to a variety of local and remote codebases to build an index of your
code. This index is used to build a snippet library, ready for ingestion into an LLM.

- Index local directories and public Git repositories
- Build comprehensive snippet libraries for LLM ingestion
- Support for multiple codebase types and languages
- Efficient indexing and search capabilities

### MCP Server

Relevant snippets are exposed to an AI coding assistant via an MCP server. This allows
the assistant to request relevant snippets by providing keywords, code, and semantic
intent. Kodit has been tested to work well with:

- Seamless integration with popular AI coding assistants
- Tested and verified with:
  - [Cursor](https://docs.helix.ml/kodit/#integration-with-cursor)
  - [Cline](https://docs.helix.ml/kodit/#integration-with-cline)
- Please contribute more instructions! ... any other assistant is likely to work ...

### Enterprise Ready

Out of the box, Kodit works with a local SQLite database and very small, local models.
But enterprises can scale out with performant databases and dedicated models. Everything
can even run securely, privately, with on-premise LLM platforms like
[Helix](https://helix.ml).

Supported databases:

- SQLite
- [Vectorchord](https://github.com/tensorchord/VectorChord)

Supported providers:

- Local (which uses tiny CPU-only open-source models)
- OpenAI
- Secure, private LLM enclave with [Helix](https://helix.ml).
- Any other OpenAI compatible API

## 🚀 Quick Start

1. [Install Kodit](https://docs.helix.ml/kodit/#installation)
2. [Index codebases](https://docs.helix.ml/kodit/#quick-start)
3. [Integrate with your coding assistant](https://docs.helix.ml/kodit/#integrating-kodit-with-coding-assistants)

### Documentation

- [Installation Guide](https://docs.helix.ml/kodit/#installation)
- [Usage Guide](https://docs.helix.ml/kodit/#quick-start)
- [Connecting to Kodit](https://docs.helix.ml/kodit/#integrating-kodit-with-coding-assistants)
- [Configuration Options](https://docs.helix.ml/kodit/#configuring-kodit)
- [Contribution Guidelines](.github/CONTRIBUTING.md)

## Roadmap

The roadmap is currently maintained as a [Github Project](https://github.com/orgs/helixml/projects/4).

## 💬 Support

For commercial support, please contact [Helix.ML](founders@helix.ml). To ask a question,
please [open a discussion](https://github.com/helixml/kodit/discussions).

## License

[Apache 2.0 © 2025 HelixML, Inc.](./LICENSE)
