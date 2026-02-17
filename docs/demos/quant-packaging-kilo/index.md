---
title: "Kodit Demo: Packaging Quant Workflows with Kilo Code"
linkTitle: Quant Packaging
description: A demo of Kodit helping with quantitative researchers using private helper libraries in Kilo Code.
weight: 1
tags:
- demo
- jupyter
- quant
- kilo
slug: quant-packaging-kilo
---

This demo shows how you can use Kodit to help speed up the quantitative research process
by indexing a private library that shows researchers how they should package their code for distribution.

<!--more-->

Our previous demo investigated how Kodit can help with [quantitative
research](../quant-jupyter/index.md) using private helper libraries in Jupyter
notebooks.

This demo adds a packaging step to the process to help researchers package their code
for distribution. It also demonstrates how you can use Kodit with Kilo Code.

<video controls preload="auto" class="html-video">
    <source src="https://s3.eu-west-2.amazonaws.com/assets.winder.ai/helix/251118_Kodit_QuantPackaging_kilo_code.mp4" type="video/mp4">
</video>

## Initial Setup

The demo uses a [mock quant helper](https://github.com/philwinder/quant-helper) library with three key classes:

- `MarketData` - Fetches market data from APIs
- `PerformanceMetrics` - Calculates performance statistics
- `Backtester` - Runs trading strategy backtests

This simulates the common helper libraries that most quant teams maintain. In practice,
you would replace this with your own private repository.

We also use a [mock quant packaging library](https://github.com/philwinder/quant-packaging) that shows researchers how they should package their code for distribution.

This simulates the common packaging libraries that most quant teams maintain. In practice,
you would replace this with your own private repository.

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

   ```sh
   curl --request POST \
   --url http://localhost:8080/api/v1/indexes \
   --header 'Content-Type: application/json' \
   --data '{
   "data": {
      "type": "index",
      "attributes": {
         "uri": "https://github.com/philwinder/quant-packaging"
      }
   }
   }'
   ```

   Wait for indexing to complete.

3. [Connect Kilo Code](/kodit/getting-started/integration/index.md).

## Example: Bitcoin Analysis Notebook

Watch the video above to see this workflow in action. To begin with I asked the AI
assistant to create a demo notebook that fetches Bitcoin market data, creates a simple
trading strategy and runs a backtest.

Then I asked the AI assistant to add a cell that helped package the code for distribution.

With Kodit enabled, Kilo Code correctly:

1. Lists available repositories to find the quant helper library
2. Retrieves the api documentation for the quant helper library
3. Generates a working Jupyter notebook with proper imports and method calls
4. Added a cell that helped package the code for distribution

## Key Benefits

This workflow is particularly valuable for quant teams because:

- **Private Repository Support**: Keep your proprietary trading logic and helpers completely private
- **Reduced Hallucination**: AI assistants use actual method signatures instead of guessing
- **Faster Onboarding**: New team members can leverage AI to learn internal libraries
- **Consistent Patterns**: Generated code follows your team's existing conventions
