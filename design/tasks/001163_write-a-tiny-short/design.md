# Design: Repository Description

## Overview

Create a simple markdown file containing a short description of the kodit repository.

## Architecture

No architecture needed - this is a single static markdown file.

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| File location | `kodit/DESCRIPTION.md` | Root level for easy discovery |
| Format | Plain markdown | Simple, universal, no tooling required |
| Length | 1-3 sentences | Meets "tiny short" requirement |

## Description Content

Based on the existing README.md, the description should capture:

> Kodit is an MCP (Model Context Protocol) server that indexes codebases and provides semantic and keyword search to AI coding assistants, helping them produce more accurate code with fewer hallucinations.

## Constraints

- Must not duplicate the full README content
- Should be immediately understandable to newcomers