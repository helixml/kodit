---
title: Installing Kodit
description: How to install Kodit.
weight: 1
---

Kodit is a Python CLI application that can be installed locally or remotely. To install choose the path that is most appropriate for your setup. Most users start by experimenting with Kodit locally and progress to installing it remotely on a server.

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

## Docker

```sh
docker run -it --rm registry.helix.ml/helix/kodit:latest
```

Always replace latest with a specific version.

## pip

Use this if you want to use kodit as a python library:

```sh
pip install kodit
```

(Requires Python 3.12+)
