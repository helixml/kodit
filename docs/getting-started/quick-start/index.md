---
title: Quick Start
description: The quickest way to get started with Kodit.
weight: 2
---

This guide assumes you have already installed Kodit and have verified it is running.

## 1. View the API Docs

Open: <http://localhost:8000/docs> (replace with your own server URL)

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
      "url": "https://gist.github.com/philwinder/7aa38185e20433c04c533f2b28f4e217.git"
    }
  }
}'
```

## 3. Check the Status of the Indexing Process

```sh
curl http://localhost:8080/api/v1/repositories/1/status
```

Wait for the indexing process to complete. If you see any errors at this stage, check
your logs and consult the [troubleshooting guide](../reference/troubleshooting/index.md).

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
      "text": "code to get all orders",
    }
  }
}'
```

Check the API docs for more endpoints and examples.

## 5. Integrate with your AI coding assistant

Now [add the Kodit MCP server to your AI coding assistant](../integration/index.md).
