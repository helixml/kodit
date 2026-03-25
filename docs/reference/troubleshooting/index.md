---
title: Troubleshooting
description: Learn about how to troubleshoot Kodit.
weight: 90
---

## MCP Troubleshooting

- `Error POSTing to endpoint (HTTP 400): Bad Request: No valid session ID provided`: This happens after Kodit server restarts. Reload your MCP client.

## Indexing Troubleshooting

- **Indexing appears stuck**: Check the task queue status at `/api/v1/queue` and the
  repository status at `/api/v1/repositories/{id}/status/summary`. Enable debug logging
  with `LOG_LEVEL=DEBUG` for more detail.
- **No enrichments generated**: Ensure an enrichment endpoint is configured. Without
  `ENRICHMENT_ENDPOINT_*` environment variables, Kodit only performs basic indexing
  (chunking, BM25, and embeddings) without AI-powered documentation.
- **Private repository access fails**: Ensure the repository URL includes authentication
  credentials (e.g. `https://user:token@github.com/org/repo.git`) or that SSH keys are
  configured for SSH URLs.

## Search Troubleshooting

- **No search results**: Verify that indexing has completed by checking the repository
  status. Results only appear after the indexing pipeline finishes.
- **Semantic search returns poor results**: The built-in embedding model is small and
  optimised for code search. For better semantic search quality, configure an external
  embedding provider via `EMBEDDING_ENDPOINT_*` environment variables.

## Debug Logging

Enable debug logging for detailed diagnostics:

```sh
LOG_LEVEL=DEBUG kodit serve
```
