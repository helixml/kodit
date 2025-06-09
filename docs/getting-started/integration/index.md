---
title: Integration With Coding Assistants
description: How to integrate Kodit with AI coding assistants.
weight: 3
---

The core goal of Kodit is to make your AI coding experience more accurate by providing better context. That means you need to integrate Kodit with your favourite assistant.

## Integration with Coding Assistants

Integration with most assistants follows a similar pattern, but each has its own configuration flow.

### Integration With Cursor

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

### Integration With Cline

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

## Forcing AI Assistants to use Kodit

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

### Forcing Cursor to Use Kodit

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

### Forcing Cline to Use Kodit

1. Go to `Settings` -> `API Configuration`
2. At the bottom there is a `Custom Instructions` section.