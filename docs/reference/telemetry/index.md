---
title: Telemetry
description: Learn about what data is collected and how to disable it.
weight: 99
---

Kodit includes a very limited amount initial telemetry to help guide product
development. At the moment Kodit uses [PostHog](https://posthog.com/) to capture anonymous
usage metrics, although it will soon be migrated to
[RudderStack](https://www.rudderstack.com/) for improved flexibility.

## What Kodit Captures

> _this list will expand over time as we improve coverage_

You can see what metrics are sent by searching for [use of the helper
function](https://github.com/helixml/kodit/blob/main/src/kodit/log.py#L160) in the Kodit
codebase.

Kodit currently captures:

- [Starting of the kodit MCP
  server](https://github.com/helixml/kodit/blob/main/src/kodit/cli.py#L324)

... and that's it!

## Disabling Telemetry

We hope that you will help us improve Kodit by leaving telemetry turned on, but if you'd
like to turn it off, add the following environmental variable (or add it to your .env file):

```sh
DISABLE_TELEMETRY=true
```
