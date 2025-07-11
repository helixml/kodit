---
title: Integration With Coding Assistants
description: How to integrate Kodit with AI coding assistants.
weight: 3
---

The core goal of Kodit is to make your AI coding experience more accurate by providing better context. That means you need to integrate Kodit with your favourite assistant.

## MCP Connection Methods

Kodit supports three different ways to run the MCP server, depending on your integration
needs. Each method exposes the same code search capabilities, but differs in how the
connection is established and which assistants/tools it is compatible with.

See the [MCP Reference](../../reference/mcp/index.md) for comprehensive integration
instructions for popular coding assistants like Cursor, Claude, Cline, etc.

### 1. HTTP Streaming (Recommended)

This is the default and recommended method for most users. Kodit runs an HTTP server
that streams responses to connected AI coding assistants over the `/mcp` endpoint.

1. Start the Kodit server:
  
  ```sh
  kodit serve
  ```

  _The Kodit container runs this command by default._

2. Configure your AI coding assistant to connect to `http://localhost:8080/mcp`

### 2. STDIO Mode

Kodit can run as an MCP server over standard input/output (STDIO) for direct integration
with local AI coding assistants that support MCP stdio transport. No network port is
opened.

1. Configure your AI coding assistant to start Kodit's STDIO server:

  ```sh
  kodit stdio
  ```

### 3. SSE (Server-Sent Events) [Deprecated]

Kodit also supports the older SSE protocol on the `/sse` endpoint. This is provided for
backward compatibility with tools that require SSE.

1. Start the Kodit server:
  
  ```sh
  kodit serve
  ```

  _The Kodit container runs this command by default._

2. Configure your AI coding assistant to connect to `http://localhost:8080/sse`
