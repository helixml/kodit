---
title: Installing Kodit
description: How to install Kodit.
weight: 1
---

Unlike many MCP tools, Kodit is designed to run as a server. You're welcome to use the
[public server](../_index.md), but if you want to index your own private repositories
then you should deploy your own server.

Kodit is a Python-based webserver that hosts a REST API and an MCP server. Although most
people install Kodit remotely in a container, you can also install it as a package.

## Docker

```sh
docker run -it --rm registry.helix.ml/helix/kodit:latest
```

Always replace latest with a specific version.

## homebrew

```sh
brew install helixml/kodit/kodit
```

## uv

```sh
uv tool install kodit
```

## pipx

```sh
pipx install kodit
```

## pip

Use this if you want to use kodit as a python library:

```sh
pip install kodit
```

(Requires Python 3.12+)
