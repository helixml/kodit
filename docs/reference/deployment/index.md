---
title: Deployment
description: Deploying Kodit with Docker Compose and Kubernetes.
weight: 20
---

Kodit is packaged as a Docker container so you can run it on any popular orchestration platform. This page describes how to deploy Kodit as a service.

## Deploying With Docker Compose

Create a [docker-compose file](https://github.com/helixml/kodit/tree/main/docs/reference/deployment/docker-compose.yaml) that specifies Kodit and Vectorchord containers. Replace the latest tag with a version. Replace any API keys with your own or configure internal endpoints.

Then run Kodit with `docker compose -f docker-compose.yaml up -d`. For more instructions see the [Docker Compose documentation](https://docs.docker.com/compose/).

Here is an example:

{{< code file="docker-compose.yaml" >}}

## Deploying With Kubernetes

To deploy with Kubernetes we recommend using a templating solution like Helm or Kustomize.

Here is a simple [raw Kubernetes manifest](https://github.com/helixml/kodit/tree/main/docs/reference/deployment/kubernetes.yaml) to help get you started. Remember to pin the Kodit container at a specific version and update the required API keys.

Deploy with `kubectl -n kodit apply -f kubernetes.yaml`

{{< code file="kubernetes.yaml" >}}

### Deploying With a Kind Kubernetes Cluster

[Kind](https://kind.sigs.k8s.io/) is a k8s cluster that runs in a Docker container. So it's great for k8s development.

1. `kind create cluster`
2. `kubectl -n kodit apply -f kubernetes.yaml`

## Remote CLI Access

Once you have Kodit deployed as a server, you can connect to it remotely using the [REST
API](../api/index.md) or the Kodit CLI (which uses the REST API).

### Configuration

Remote mode is activated when you configure a server URL. You can do this via environment variables or CLI flags:

**Environment Variables:**

```bash
export REMOTE_SERVER_URL=https://your-kodit-server.com
export REMOTE_API_KEY=your-api-key-here # Optional: Only if you have API key's enabled
export REMOTE_TIMEOUT=60.0              # Optional: request timeout in seconds
export REMOTE_MAX_RETRIES=5             # Optional: max retry attempts
export REMOTE_VERIFY_SSL=true           # Optional: verify SSL certificates
```

### Security

- Always use HTTPS in production environments
- Store API keys securely and never commit them to version control
- Use environment variables or secure credential stores for API keys
- The CLI verifies SSL certificates by default (can be disabled with `REMOTE_VERIFY_SSL=false`)
