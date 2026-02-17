---
title: "Kodit Demo: Quant Workflows with Private Libraries"
linkTitle: Quant Workflows
description: A demo of Kodit helping with quantitative analysis using private helper libraries in Jupyter notebooks.
weight: 1
tags:
- demo
- jupyter
- quant
- cline
---

Most quantitative researchers maintain proprietary helper libraries for common tasks
like data fetching, performance calculations, and backtesting. Without context about
these internal libraries, AI assistants often hallucinate method names or reinvent
functionality that already exists.

<!--more-->

This example demonstrates using Kodit to index a private quant helper library, enabling AI assistants to generate correct Jupyter notebook code that leverages existing internal tools.

You'll see that with Kodit the AI assistant delivers:

- Correct usage of private library APIs
- Faster notebook development
- Fewer hallucinated method names

<video controls preload="auto" class="html-video">
    <source src="https://s3.eu-west-2.amazonaws.com/assets.winder.ai/helix/251109_Kodit_SimpleQuantDemo.mp4" type="video/mp4">
</video>

## Initial Setup

The demo uses a [mock quant helper](https://github.com/philwinder/quant-helper) library with three key classes:

- `MarketData` - Fetches market data from APIs
- `PerformanceMetrics` - Calculates performance statistics
- `Backtester` - Runs trading strategy backtests

This simulates the common helper libraries that most quant teams maintain. In practice, you would replace this with your own private repository.

## Indexing the Quant Helper Library with Kodit

Let's index the quant helper library and create our analysis notebook:

1. (Optional) Make sure the [Kodit server is running](/kodit/getting-started/_index.md) and start afresh.

2. Index the quant helper library via curl:

   ```sh
   curl --request POST \
   --url http://localhost:8080/api/v1/indexes \
   --header 'Content-Type: application/json' \
   --data '{
   "data": {
      "type": "index",
      "attributes": {
         "uri": "https://github.com/philwinder/quant-helper"
      }
   }
   }'
   ```

   Wait for indexing to complete:

   ```sh
   curl --request GET \
   --url http://localhost:8080/api/v1/repositories/1/status \
   ```

3. [Connect Cline](/kodit/getting-started/integration/index.md).

## Example: Bitcoin Analysis Notebook

Here is the prompt used to create the analysis notebook:

> Using the kodit mcp server to help you, create a new file called demo.ipynb to fetch bitcoin from the past 30 days data using the quant-helper library.
> Once you've done that, then write some performance indicators and then write a simple momentum strategy algorithm and run a backtest.
> Look at the mcp methods and think about how they might help you deliver this.

With Kodit enabled, Cline correctly:

1. Lists available repositories to find the quant helper library
2. Retrieves the cookbook enrichment for quick-start guidance
3. Generates a working Jupyter notebook with proper imports and method calls

The AI assistant successfully creates a notebook that:

- Fetches Bitcoin market data using the correct `MarketData` class
- Calculates performance metrics with the `PerformanceMetrics` helper
- Implements and backtests a momentum trading strategy using the `Backtester` class

The notebook also includes proper usage of the performance metrics calculator and backtester, demonstrating that Kodit successfully provided context about the entire library's API surface.

## Key Benefits

This workflow is particularly valuable for quant teams because:

- **Private Repository Support**: Keep your proprietary trading logic and helpers completely private
- **Reduced Hallucination**: AI assistants use actual method signatures instead of guessing
- **Faster Onboarding**: New team members can leverage AI to learn internal libraries
- **Consistent Patterns**: Generated code follows your team's existing conventions

Watch the demo video above to see this workflow in action.
