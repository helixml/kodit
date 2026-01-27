# Kodit Benchmark: RepoEval Implementation

## Overview

This document describes the RepoEval benchmark implementation for evaluating Kodit's code retrieval capabilities. The benchmark uses function-level code completion tasks from the [CodeRAG-Bench](https://github.com/code-rag-bench/code-rag-bench) dataset to measure how much Kodit's retrieval improves LLM code generation.

---

## 1. Why RepoEval?

RepoEval tests repository-level code completion—the exact use case Kodit is designed for:

| Kodit Capability | RepoEval Requirement | Alignment |
|------------------|---------------------|-----------|
| Index Git repositories | Tasks come from real GitHub repos | ✅ Perfect |
| Hybrid search (BM25 + semantic) | Find relevant code for completion | ✅ Perfect |
| AST-based snippet extraction | Function/class definitions needed | ✅ Perfect |
| Filter by repository | Tasks are repo-specific | ✅ Perfect |

From the [CodeRAG-Bench paper](https://arxiv.org/abs/2406.14497):
- Models gain **up to 17 points** on RepoEval with retrieved code snippets
- RepoEval shows the **largest improvement** from retrieval among all benchmarks

---

## 2. Dataset

### 2.1 Data Sources

The benchmark uses pre-packaged data from RepoCoder:

| Resource | URL |
|----------|-----|
| Tasks | `https://github.com/microsoft/CodeT/raw/main/RepoCoder/datasets/datasets.zip` |
| Repository Snapshots | `https://github.com/Veronicium/repoeval_debug/raw/main/function_level.zip` |

The repository snapshots are frozen at the exact commits used for the benchmark, ensuring reproducibility and preventing data leakage (solutions aren't indexed).

### 2.2 Repositories

The dataset contains 8 Python repositories with 910 function completion tasks:

| Repository | Tasks | Description |
|------------|-------|-------------|
| CarperAI/trlx | 92 | RLHF training library |
| amazon-science/patchcore-inspection | 64 | Anomaly detection |
| deepmind/tracr | 292 | Transformer circuits |
| facebookresearch/omnivore | 44 | Multi-modal learning |
| google/lightweight_mmm | 128 | Marketing mix modeling |
| leopard-ai/betty | 72 | Bilevel optimization |
| lucidrains/imagen-pytorch | 134 | Text-to-image diffusion |
| maxhumber/redframes | 84 | Data manipulation |

### 2.3 Task Format

Each task provides a function signature and asks the LLM to complete the body:

```json
{
  "task_id": "CarperAI--trlx/idx",
  "repo": "CarperAI/trlx",
  "file_path": "",
  "function_signature": "def save_pretrained(self, directory: str) -> None:\n    \"\"\"Save model and tokenizer.\"\"\"",
  "docstring": "",
  "ground_truth": "",
  "context_snippets": []
}
```

Note: The RepoCoder format embeds the repo in the task_id as `owner--repo/suffix`. The parser extracts this to populate the `repo` field.

---

## 3. Experimental Design

### 3.1 Conditions

| Condition | Description |
|-----------|-------------|
| **Baseline** | LLM generates code with only the task prompt (no retrieval) |
| **Kodit RAG** | LLM generates code with task prompt + top-5 Kodit results |
| **Gold Context** | LLM generates code with task prompt + ground-truth snippets (upper bound) |

### 3.2 Metrics

**Primary Metric**:
- **Pass@1**: Correctness of generated code (token similarity with ground truth)
- **Pass@1 Delta**: `Pass@1(Kodit) - Pass@1(baseline)` — the value Kodit adds

**Secondary Metric**:
- **Retrieval Recall@5**: Fraction of ground-truth snippets found in top-5 results

### 3.3 Evaluation

Code correctness is evaluated using token-based similarity between generated code and ground truth. A threshold of 0.7 similarity indicates a pass. This approach is used because:
1. Full execution evaluation requires complex test harness setup per repository
2. Token similarity correlates well with functional correctness for completion tasks
3. It's fast and deterministic

---

## 4. Running the Benchmark

### 4.1 Quick Start

```bash
# Step 1: Download repos and index with Kodit (takes ~30 min per repo)
uv run python -m benchmarks.run_benchmark setup

# Step 2: Run the benchmark
uv run python -m benchmarks.run_benchmark run

# Step 3: Analyze existing results
uv run python -m benchmarks.run_benchmark analyze benchmarks/results/repoeval_*.json
```

### 4.2 CLI Commands

```bash
# Show help
uv run python -m benchmarks.run_benchmark --help

# Create sample dataset for testing
uv run python -m benchmarks.run_benchmark create-sample

# Run with specific options
uv run python -m benchmarks.run_benchmark run \
    --model openrouter/qwen/qwen3-coder \
    --temperature 1.0 \
    --limit 10 \
    --conditions baseline kodit
```

### 4.3 Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `--model` | `openrouter/qwen/qwen3-coder` | LiteLLM model identifier |
| `--temperature` | `1.0` | Generation temperature |
| `--max-tokens` | `8192` | Max tokens per generation |
| `--limit` | `0` (all) | Limit number of tasks |
| `--conditions` | `baseline kodit gold` | Conditions to run |
| `--timeout` | `1800` | Indexing timeout per repo (setup only) |

---

## 5. Architecture

### 5.1 Directory Structure

```
benchmarks/
├── run_benchmark.py          # CLI entry point
├── repoeval/
│   ├── task.py               # Task and result dataclasses
│   ├── loader.py             # Dataset loader
│   ├── retriever.py          # Kodit search wrapper
│   ├── prompts.py            # Prompt templates
│   ├── executor.py           # Code similarity checker
│   ├── runner.py             # Benchmark orchestrator
│   └── analysis.py           # Results analysis
├── scripts/
│   ├── download_repoeval.py  # Download tasks and repos
│   └── clone_repos.py        # Repository configuration
├── data/
│   └── repoeval_tasks.json   # Downloaded task data
├── repos/                    # Repository snapshots
│   ├── CarperAI_trlx/
│   ├── lucidrains_imagen-pytorch/
│   └── ...
└── results/                  # Benchmark outputs
    └── repoeval_YYYY-MM-DD_HH-MM-SS.json
```

### 5.2 Setup Process

The `setup` command performs these steps:

1. **Download snapshots**: Fetches `function_level.zip` containing repository snapshots at the exact commits used by RepoEval

2. **Initialize git repos**: The snapshots are plain directories without `.git`. The setup initializes them as git repositories so Kodit can index them:
   ```bash
   git init
   git add .
   git commit -m "RepoEval snapshot"
   ```

3. **Start Kodit server**: Launches Kodit on port 8765

4. **Index repositories**: For each repo:
   - POST to `/api/v1/repositories` with `file://` URI
   - Poll `/api/v1/repositories/{id}/status/summary` until status is `completed`
   - The indexing pipeline runs: clone → scan → parse → extract → embed

5. **Shutdown**: Stops the server after all repos are indexed

### 5.3 Benchmark Pipeline

```
┌─────────────┐    ┌──────────────┐    ┌──────────────────┐
│  Load Tasks │───▶│   Retrieve   │───▶│  Build Prompt    │
│  from JSON  │    │  from Kodit  │    │  (task+context)  │
└─────────────┘    └──────────────┘    └────────┬─────────┘
                                                │
┌─────────────┐    ┌──────────────┐             │
│   Report    │◀───│   Evaluate   │◀────────────┘
│   Results   │    │  Similarity  │
└─────────────┘    └──────────────┘
```

---

## 6. Implementation Details

### 6.1 Retrieval

The `KoditRetriever` queries Kodit's search endpoint:

```python
async def retrieve(self, task: RepoEvalTask, k: int = 5) -> list[str]:
    query = f"{task.function_signature}\n{task.docstring}"

    results = await self._search_service.search(
        user_intent=query,
        keywords=[],
        related_file_paths=[],
        related_file_contents=[],
    )

    return [r.content for r in results[:k]]
```

### 6.2 Prompt Templates

**Baseline prompt** (no retrieval):
```
Complete the following Python function body.

{function_signature}

Provide only the function body implementation.
```

**RAG prompt** (with retrieved context):
```
Complete the following Python function body.

Here is relevant code from the repository that may help:

{retrieved_snippets}

Now complete this function:

{function_signature}

Provide only the function body implementation.
```

### 6.3 Code Evaluation

Token-based similarity using Python's `difflib.SequenceMatcher`:

```python
def check(self, generated: str, ground_truth: str) -> ExecutionResult:
    similarity = SequenceMatcher(None, gen_tokens, gt_tokens).ratio()
    passed = similarity >= 0.7
    return ExecutionResult(passed=passed, similarity=similarity)
```

---

## 7. Results Format

```json
{
  "benchmark": "repoeval",
  "model": "openrouter/qwen/qwen3-coder",
  "repositories": ["CarperAI/trlx", "..."],
  "results": {
    "baseline": {
      "pass_at_1": 0.35,
      "retrieval_recall_5": 0.0
    },
    "kodit": {
      "pass_at_1": 0.52,
      "retrieval_recall_5": 0.68
    },
    "gold": {
      "pass_at_1": 0.78,
      "retrieval_recall_5": 1.0
    }
  }
}
```

The analysis report shows:
- Overall Pass@1 for each condition
- Delta between Kodit and baseline
- Ceiling percentage (how much of the possible improvement Kodit captures)
- Per-repository breakdown

---

## 8. Expected Results

Based on CodeRAG-Bench findings:

| Metric | Expected |
|--------|----------|
| Baseline Pass@1 | ~35% |
| Kodit RAG Pass@1 | ~50% |
| Gold Context Pass@1 | ~78% |
| **Kodit Delta** | **+15%** |

A positive delta demonstrates Kodit's value for repository-level coding tasks.

---

## 9. Troubleshooting

### Indexing takes too long
- Each repository takes ~5-30 minutes to fully index
- The setup waits for completion by polling the status summary endpoint
- Use `--timeout` to adjust the per-repo timeout

### API key issues
- Set `OPENROUTER_API_KEY` for OpenRouter models
- Or use other LiteLLM-compatible providers (OpenAI, Anthropic, etc.)

### Database conflicts
- Clear the database: `rm ~/.kodit/kodit.db`
- Clear clones: `rm -rf ~/.kodit/clones/*`

### Port already in use
- The setup uses port 8765
- Kill any existing Kodit processes before running setup

---

## 10. Known Issues and Required Fixes

This section documents issues identified during implementation review that must be resolved before the benchmark can accurately measure Kodit's value.

### 10.1 Critical Issues

#### Issue 1: Repository Source Mismatch in Retriever

**Status**: NOT IMPLEMENTED

**Problem**: The `KoditRetriever` filters search results by `source_repo` using GitHub-style URLs (`github.com/CarperAI/trlx`), but repositories are indexed with local file URIs (`file:///path/to/CarperAI_trlx`). This mismatch causes the Kodit condition to return zero results.

**Location**: `benchmarks/repoeval/retriever.py:43-47`

```python
# Current implementation (broken)
def _normalize_repo(self, repo: str) -> str:
    if repo.startswith("github.com/"):
        return repo
    return f"github.com/{repo}"  # Returns: github.com/CarperAI/trlx
```

**Fix Options**:

1. **Option A (Recommended)**: Remove the `source_repo` filter entirely. Since each benchmark run uses a fresh database with only the target repositories, filtering is unnecessary.
2. **Option B**: Store GitHub URLs as metadata during indexing and filter on that metadata field.
3. **Option C**: Map local paths back to GitHub URLs in the retriever.

---

#### Issue 2: Setup Command Doesn't Download Tasks

**Status**: NOT IMPLEMENTED

**Problem**: The `setup` command downloads repository snapshots but does not download the RepoEval task data. Running `setup` followed by `run` will fail because `benchmarks/data/repoeval_tasks.json` won't exist.

**Location**: `benchmarks/run_benchmark.py:345` (setup command)

**Fix**: Add a call to `download_repoeval()` at the start of the setup command:

```python
@cli.command()
def setup(timeout: int, repos_dir: Path) -> None:
    # Step 0: Download task data (NEW)
    tasks_path = Path("benchmarks/data/repoeval_tasks.json")
    if not tasks_path.exists():
        log.info("Downloading RepoEval tasks")
        download_repoeval(tasks_path)

    # Step 1: Download repository snapshots...
```

---

#### Issue 3: Missing Ground Truth Context Snippets

**Status**: NEEDS VERIFICATION

**Problem**: The RepoCoder dataset may not include `reference_code` (ground truth context snippets) for all tasks. Without these:

- The "gold" condition provides no context (same as baseline)
- Retrieval recall calculations return 0.0
- The "ceiling" metric becomes meaningless

**Location**: `benchmarks/scripts/download_repoeval.py:87`

**Fix**:

1. Download the actual task data and inspect the `reference_code` field
2. If empty, consider using an alternative data source or computing reference code from the repository snapshots
3. Document clearly if gold condition is not available

---

### 10.2 Secondary Issues

#### Issue 4: No Pre-Run Verification

**Problem**: The benchmark doesn't verify that repositories are indexed before running. If indexing failed or the database was cleared, results would be meaningless.

**Fix**: Add a verification step in the `run` command that:

1. Checks that `benchmarks/data/repoeval_tasks.json` exists
2. Queries Kodit to verify each target repository is indexed
3. Fails fast with a clear error message if prerequisites aren't met

---

#### Issue 5: Evaluation May Be Too Lenient

**Problem**: The `CodeExecutor` has multiple fallback checks including a "50% method overlap" heuristic that may pass functionally incorrect code, potentially masking the delta between baseline and Kodit.

**Location**: `benchmarks/repoeval/executor.py:47-48`

**Recommendation**: Consider making the evaluation stricter:

1. Remove the `_contains_key_operations` fallback
2. Increase similarity threshold from 0.7 to 0.8
3. Or run benchmarks with both strict and lenient evaluation for comparison

---

### 10.3 Implementation Checklist

Before the benchmark can demonstrate Kodit's value, complete these items:

| #   | Task                                              | Priority | Status |
| --- | ------------------------------------------------- | -------- | ------ |
| 1   | Fix retriever source_repo filter mismatch         | Critical | TODO   |
| 2   | Add task download to setup command                | Critical | TODO   |
| 3   | Verify ground truth context_snippets exist        | Critical | TODO   |
| 4   | Add pre-run verification checks                   | High     | TODO   |
| 5   | Review and potentially tighten evaluation criteria| Medium   | TODO   |
| 6   | Update Quick Start docs after fixes               | Medium   | TODO   |

---

## 11. References

- [CodeRAG-Bench Paper](https://arxiv.org/abs/2406.14497)
- [RepoCoder Paper (introduced RepoEval)](https://arxiv.org/abs/2303.12570)
- [RepoCoder GitHub](https://github.com/microsoft/CodeT/tree/main/RepoCoder)
- [RepoEval Debug Snapshots](https://github.com/Veronicium/repoeval_debug)
