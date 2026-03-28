---
title: Quick Start
description: The quickest way to get started with Kodit.
weight: 2
---

This guide assumes you have already installed Kodit and have verified it is running.

## 1. View the API Docs

Open: <http://localhost:8080/docs> (replace with your own server URL)

## 2. Index a Repository

Replace the repository URL with the one you want to index. This is a small, toy
application that should index quickly.

```sh
curl http://localhost:8080/api/v1/repositories \
-X POST \
-H "Content-Type: application/json" \
-d '{
  "data": {
    "type": "repository",
    "attributes": {
      "remote_uri": "https://gist.github.com/philwinder/7aa38185e20433c04c533f2b28f4e217.git"
    }
  }
}'
```

## 3. Check the Status of the Indexing Process

```sh
curl http://localhost:8080/api/v1/repositories/1/status
```

Wait for the indexing process to complete. You can also check the aggregated summary:

```sh
curl http://localhost:8080/api/v1/repositories/1/status/summary
```

If you see any errors at this stage, check your logs and consult the
[troubleshooting guide](../../reference/troubleshooting/index.md).

## 4. Search for Code

When indexing is complete, you can search through the index of the repository:

```sh
curl http://localhost:8080/api/v1/search \
-X POST \
-H "Content-Type: application/json" \
-d '{
  "data": {
    "type": "search",
    "attributes": {
      "keywords": [
        "orders"
      ],
      "code": "func (s *OrderService) GetAllOrders() []Order {",
      "text": "code to get all orders"
    }
  }
}'
```

You can also use the keyword or semantic search endpoints directly:

```sh
# Keyword search (BM25)
curl "http://localhost:8080/api/v1/search/keyword?keywords=orders&limit=5"

# Semantic search
curl "http://localhost:8080/api/v1/search/semantic?query=get+all+orders&limit=5"

# Grep (regex search via git grep)
curl "http://localhost:8080/api/v1/search/grep?repository_id=1&pattern=GetAllOrders"

# List files matching a pattern
curl "http://localhost:8080/api/v1/search/ls?repository_id=1&pattern=**/*.go"
```

Check the [API docs](../../reference/api/index.md) for more endpoints and examples.

## 5. Browse Enrichments

If you configured an enrichment endpoint, Kodit generates AI-powered documentation for
your repository. You can browse it via the API:

```sh
# List enrichments for the latest commit
curl http://localhost:8080/api/v1/repositories/1/enrichments

# View the wiki table of contents
curl http://localhost:8080/api/v1/repositories/1/wiki
```

## 6. Integrate with your AI coding assistant

Now [add the Kodit MCP server to your AI coding assistant](../integration/index.md).
