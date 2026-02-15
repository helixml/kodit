## To Test

- [ ] MCP

## Verification

- [ ] BM25 search results gone
- [ ] Code search results gone

## Bugs

- [ ] application/handler/enrichment/example_summary.go - Association uses incorrect entity type for enrichment ID reference. The code creates an association using EntityTypeSnippet but the entityID parameter is an enrichment ID (from example.ID()), not a snippet ID. The EntityTypeKey should represent the type of entity the entityID refers to. The domain only defines EntityTypeCommit and EntityTypeSnippet, but neither correctly represents an enrichment-to-enrichment relationship. This mismatch could cause downstream bugs if code assumes the entityID is a snippet and attempts to query or process it accordingly.
- [ ] application/handler/enrichment/example_summary.go - The code creates an association using EntityTypeSnippet but the entityID parameter is an enrichment ID (from example.ID()), not a snippet ID. The EntityTypeKey should represent the type of entity the entityID refers to. The domain only defines EntityTypeCommit and EntityTypeSnippet, but neither correctly represents an enrichment-to-enrichment relationship. This mismatch could cause downstream bugs if code assumes the entityID is a snippet and attempts to query or process it accordingly.
- [ ] The `TruncateDiff` function can panic at runtime if `maxLength` is less than or equal to the length of the truncation notice. When `maxLength - len(truncationNotice)` is negative, the slice operation `diff[:maxLength-len(truncationNotice)]` causes a panic. This violates the "NO PANICS" rule from CLAUDE.md. While the current usage with `MaxDiffLength = 100_000` won't trigger this, the function is unsafe for general use. @application/handler/enrichment/util.go:4-11.
- [ ] The HEALTHCHECK command at line 71 uses wget to verify the application health, but wget is not installed in the final Alpine 3.19 image. The runtime dependencies installed at lines 43-47 do not include wget. This will cause the health check to fail with "wget: not found", making the container unhealthy and causing orchestration systems (Kubernetes, Docker Compose) to mark it as failed and potentially restart it unnecessarily.