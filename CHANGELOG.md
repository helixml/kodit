# Changelog

Notable changes to the kodit Go library and server. Release notes for each
tag are on the [GitHub releases page](https://github.com/helixml/kodit/releases).

## Unreleased

### Changed (behaviour change)

- `NewScopedMCPHandler(client, repoIDs)` with a nil or empty `repoIDs` now
  sees **no** repositories. Previously an empty list produced an unscoped
  handler that saw every repository. Callers that want every repository must
  switch to the new `NewMCPHandler(client)`. A caller that refuses an empty
  list before calling can keep doing so for a clearer error, but comments
  that describe an empty list as "every repository" no longer hold.
- `file://` repositories are no longer read with git unless the operator
  enables it with `WithLocalGitRepositories()` (server:
  `ALLOW_LOCAL_GIT_REPOSITORIES=true`). Without it, every `file://`
  repository is indexed as a plain directory, even one that contains a git
  repository. Previously kodit detected git in these directories
  automatically and honoured any git configuration and object store they
  contained.
  - With the option on, kodit uses git only for a directory with a real
    `.git` directory (not a symbolic link).
  - A `file://` path inside kodit's clone directory is refused.
  - Plain directories skip `.git` directories when they are indexed.
- Every git command kodit builds runs with
  `-c safe.bareRepository=explicit -c core.fsmonitor=false
  -c core.hooksPath=/dev/null -c protocol.ext.allow=never`. Commit diffs
  and logs pass `--no-textconv --no-ext-diff`. Cloning goes through the same
  command, no longer through Gitea's `Clone`.
- Directory snapshot SHAs are salted with the repository ID, so two
  directories with the same contents get different SHAs. A directory
  repository indexed by an earlier release gets a new snapshot SHA, and is
  re-indexed, on its next sync.
- Symbolic links in repository working copies are no longer followed. Every
  read of working-copy content goes through `internal/workingcopy`, which
  refuses a path that is, or passes through, a symbolic link. This covers:
  - raster and text document reads (MCP and REST)
  - document indexing and page-image extraction
  - the cookbook, wiki, database-schema, physical-architecture and API-docs
    enrichers
  - scanning, hashing and reading non-git directories

  A symlinked document now reads as not found and is skipped during indexing.
  Plain reads of git repositories already returned a link's target path, not
  its content.
- Search scoping uses only an enrichment's direct repository associations.
  Both the search stores' source-repository filter and the MCP result filter
  apply this rule. Previously the store filter also used the repository
  `git_commits` recorded for the enrichment's commit SHA, which is not always
  the repository that indexed it.
  - A commit SHA already recorded for another repository keeps its owner when
    a second repository scans it.
  - Repositories only share indexed content for commits they hold. When a
    commit's chunks and page images already exist, indexing adds the
    repository's association only after reading the commit from that
    repository's own git history. Directory snapshots are never shared.
  - `Enrichment.RepositoryIDs` returns every repository an enrichment is
    associated with (`map[string][]int64`).
  - A rescan of a commit that is not recorded for the repository is
    refused with a not-found error before any task is enqueued.
  - A rescan, the old-commit cleanup after indexing, and repository deletion
    delete only enrichments that no other repository shares. A shared
    enrichment loses only that repository's association. On a rescan, the
    shared commit's files and commit row are also kept.
- `kodit_semantic_search`, `kodit_keyword_search` and `kodit_visual_search`
  return at most 100 results, whatever `limit` is requested.
- Constructor changes:
  - `git.NewGiteaAdapter` and `git.NewRepositoryScanner` take a `git.Layout`.
  - `indexing.NewChunkFiles` and `indexing.NewExtractPageImages` take a
    commit store.
  - `enrichment.NewAPIDocs` takes a repository store.
  - The unused `git.IgnorePattern` is removed.

- `Repositories.Add` (and the REST create endpoint, as a validation error)
  accepts only `http`, `https`, `ssh`, `git` and `file` URLs and scp-style
  SSH addresses (`[user@]host:path`).

### Added

- `NewMCPHandler(client)`: an MCP HTTP handler that sees every repository.
- `WithLocalGitRepositories()` / `ALLOW_LOCAL_GIT_REPOSITORIES`: read
  `file://` repositories that have a `.git` directory with git.

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
- `kodit_keyword_search` with `source_repo` now filters in the search store
  as well as afterwards, so matches from other repositories no longer use up
  the result limit.
- The API-docs enricher reads files from the repository's working copy. It
  previously resolved repository-relative paths against the process's working
  directory.

### Upgrading

- Run a one-off Sync of every directory (`file://`) repository after
  upgrading. This moves the repository to its salted snapshot SHA and
  re-indexes it; until then it keeps the SHA computed by the earlier
  release.

### Known limitations

- `git_commits` keys commits by SHA alone. When two repositories contain the
  same git commit, only the repository that recorded it first owns the row,
  so the other repository's commit lookups do not find that commit. These
  include its latest commit, `commit_sha` validation in the docs and wiki
  tools, and `kodit_ls`. The other repository also cannot rescan the commit.
  Its search results are not affected. The fix is to make the key include
  the repository ([#583](https://github.com/helixml/kodit/issues/583)).
