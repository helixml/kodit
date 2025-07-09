---
title: Sync
description: Automatic synchronization of indexed codebases
weight: 3
---

Kodit includes an automatic sync scheduler that keeps your indexed codebases up-to-date with the latest changes. This is especially useful for server deployments where multiple users are working with the same codebases.

## How Sync Works

The sync scheduler runs as a background service that:

1. **Periodically checks** all existing indexes at a configurable interval
2. **Syncs each index** by pulling the latest changes from the source repository
3. **Updates the search index** with any new or modified code snippets
4. **Handles failures gracefully** with configurable retry attempts
5. **Logs detailed progress** for monitoring and debugging

## Configuration

### Environment Variables

Configure the sync scheduler using these environment variables:

```bash
# Enable/disable periodic sync (default: true)
SYNC_PERIODIC_ENABLED=true

# Sync interval in seconds (default: 1800 = 30 minutes)
SYNC_PERIODIC_INTERVAL_SECONDS=1800

# Number of retry attempts for failed syncs (default: 3)
SYNC_PERIODIC_RETRY_ATTEMPTS=3
```

### Common Configuration Examples

#### Quick Development Setup

For rapid development with frequent changes:

```bash
SYNC_PERIODIC_ENABLED=true
SYNC_PERIODIC_INTERVAL_SECONDS=300  # 5 minutes
SYNC_PERIODIC_RETRY_ATTEMPTS=1
```

#### Production Server Setup

For production deployments with stable codebases:

```bash
SYNC_PERIODIC_ENABLED=true
SYNC_PERIODIC_INTERVAL_SECONDS=3600  # 1 hour
SYNC_PERIODIC_RETRY_ATTEMPTS=3
```

#### Disable Sync

If you prefer to sync manually:

```bash
SYNC_PERIODIC_ENABLED=false
```

## Limitations

- **Only syncs existing indexes**: The sync scheduler does not create new indexes for repositories that haven't been indexed yet
- **Sequential processing**: Indexes are synced one at a time to avoid overwhelming the system
- **No conflict resolution**: If there are conflicts during sync, the operation may fail and require manual intervention

## Manual Sync

You can also manually sync indexes using the CLI:

```bash
# Sync all indexes
kodit index --sync
```
