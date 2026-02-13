## New Features

- [ ] Bundle a built-in embedding model in the binary. Bundle a "built-in" embedding mode. Default to this if no embedding provider is configured.

## Refactorings

- [ ] Completely remove go-git and use the gitea library instead.

## Bugs

- [ ] application/handler/enrichment/example_summary.go - Association uses incorrect entity type for enrichment ID reference. The code creates an association using EntityTypeSnippet but the entityID parameter is an enrichment ID (from example.ID()), not a snippet ID. The EntityTypeKey should represent the type of entity the entityID refers to. The domain only defines EntityTypeCommit and EntityTypeSnippet, but neither correctly represents an enrichment-to-enrichment relationship. This mismatch could cause downstream bugs if code assumes the entityID is a snippet and attempts to query or process it accordingly.
- [ ] application/handler/enrichment/example_summary.go - The code creates an association using EntityTypeSnippet but the entityID parameter is an enrichment ID (from example.ID()), not a snippet ID. The EntityTypeKey should represent the type of entity the entityID refers to. The domain only defines EntityTypeCommit and EntityTypeSnippet, but neither correctly represents an enrichment-to-enrichment relationship. This mismatch could cause downstream bugs if code assumes the entityID is a snippet and attempts to query or process it accordingly.