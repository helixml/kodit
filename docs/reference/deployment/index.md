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

## Configuration

Please see the [configuration reference](/kodit/reference/configuration/index.md) for
full details and examples.
