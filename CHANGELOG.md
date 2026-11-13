# Changelog

Notable changes to the kodit Go library and server. Release notes for each
tag are on the [GitHub releases page](https://github.com/helixml/kodit/releases).

## Unreleased

### Changed (behaviour change)

- `NewScopedMCPHandler(client, repoIDs)` with a nil or empty `repoIDs` now
  sees **no** repositories. Previously an empty list produced an unscoped
  handler that saw every repository. Callers that want every repository must
  switch to the new `NewMCPHandler(client)`.
- Symbolic links in repository working copies are no longer followed. Every
  read of working-copy content goes through `internal/workingcopy`, which
  refuses a path that is, or passes through, a symbolic link. This covers
  raster and text document reads (MCP and REST), document indexing, page-image
  extraction, the cookbook, wiki, database-schema, physical-architecture and
  API-docs enrichers, `.noindex`, and scanning, hashing and reading non-git
  directories. A symlinked document now reads as not found and is skipped
  during indexing; plain reads of git repositories already returned a link's
  target path, not its content.
- `kodit_semantic_search`, `kodit_keyword_search` and `kodit_visual_search`
  return at most 100 results, whatever `limit` is requested.
- `service.NewEnrichment` takes a `repository.CommitStore` after the
  association store, and `Enrichment.RepositoryIDs` is replaced by
  `Enrichment.RepositoryMemberships`. It returns every repository an
  enrichment belongs to.

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
- Search results from a commit that two repositories share are visible to
  both repositories. An enrichment belongs to every repository it is directly
  associated with, and to the repository `git_commits` records for its
  commit. The search stores' source-repository filter and the MCP result
  filter now use that same rule. Before, they used different sources, and a
  shared commit could be hidden from both repositories.
- `kodit_keyword_search` with `source_repo` now filters in the search store
  as well as afterwards, so matches from other repositories no longer use up
  the result limit.
- The API-docs enricher reads files from the repository's working copy. It
  previously resolved repository-relative paths against the process's working
  directory.

### Known limitations

- `git_commits` keys commits by SHA alone, so when two repositories contain
  the same commit, only the repository scanned last owns its row. Commit
  lookups for the other repository (its latest commit, `commit_sha`
  validation in the docs and wiki tools, `kodit_ls`) do not find that commit
  until the key includes the repository
  ([#583](https://github.com/helixml/kodit/issues/583)).
