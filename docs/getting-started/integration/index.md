---
title: Integration With Coding Assistants
description: How to integrate Kodit with AI coding assistants.
weight: 3
---

The core goal of Kodit is to make your AI coding experience more accurate by providing
better context. That means you need to integrate Kodit with your favourite assistant.

## MCP Connection Methods

Kodit runs an HTTP server that streams responses to connected AI coding assistants over
the `/mcp` endpoint.

See the [MCP Reference](../../reference/mcp/index.md) for comprehensive integration
instructions for popular coding assistants like Cursor, Claude, Cline, etc.

### Hosted

Configure your AI coding assistant to connect to `https://kodit.helix.ml/mcp`

More information about the hosted service is available in the [hosted Kodit documentation](../../reference/hosted-kodit/index.md).

### Local

1. Start the Kodit server:

  ```sh
  kodit serve
  ```

  _The Kodit container runs this command by default._

2. Configure your AI coding assistant to connect to the `/mcp` endpoint, for example:
   `http://localhost:8080/mcp`.

## HTTP API Connection Methods

Helix also exposes a REST API with an `/api/v1/search` endpoint to allow integration
with other tools. See the [Kodit HTTP API documentation](../../reference/api/index.md) for more information.
