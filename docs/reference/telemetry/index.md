---
title: Telemetry
description: Learn about what data is collected and how to disable it.
weight: 99
---

Kodit includes a very limited amount anonymous telemetry to help guide product
development. At the moment Kodit uses [Rudderstack](https://rudderstack.com) to capture
anonymous usage metrics.

## What Kodit Captures

You can see what metrics are sent by searching for [use of the helper
functions](https://github.com/helixml/kodit/blob/main/src/kodit/log.py#L169) in the Kodit
codebase.

Kodit currently captures use of the following:

- When a user uses the CLI methods
- When the indexing service is used or queried

No user data is collected, only metadata about Kodit usage.

## Disabling Telemetry

We hope that you will help us improve Kodit by leaving telemetry turned on, but if you'd
like to turn it off, add the following environmental variable (or add it to your .env file):

```sh
DISABLE_TELEMETRY=true
```
