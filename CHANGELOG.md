# Changelog

Notable changes to the kodit Go library and server. Release notes for each
tag are on the [GitHub releases page](https://github.com/helixml/kodit/releases).

## Unreleased

### Changed (behaviour change)

- `NewScopedMCPHandler(client, repoIDs)` with a nil or empty `repoIDs` now
  sees **no** repositories. Previously an empty list produced an unscoped
  handler that saw every repository. Callers that want every repository must
  switch to the new `NewMCPHandler(client)`.

### Added

- `NewMCPHandler(client)`: an MCP HTTP handler that sees every repository.

### Fixed

- The scoped MCP handler now applies its repository scope to every read path
  and search, not only to plain file reads and code search:
  - `?mode=raster` and `?mode=text` file reads (PDF, office documents,
    images) resolve their disk path through the scope, both through
    `kodit_read_resource` and the `file://` resource template.
  - `kodit_visual_search` is restricted to the scope.
  - Commit lookups, and enrichment listings behind the docs and wiki tools
    (`kodit_architecture_docs`, `kodit_api_docs`, `kodit_commit_description`,
    `kodit_database_schema`, `kodit_cookbook`, `kodit_wiki`,
    `kodit_wiki_page`), only accept commits from repositories in the scope.
  - Search results are post-filtered against the scope even if a searcher
    ignores its filters, and a search whose requested repositories fall
    entirely outside the scope returns no results instead of running
    unfiltered.
- The docs and wiki tools reject a `commit_sha` that does not belong to the
  repository named by `repo_url`, on scoped and unscoped servers alike.
