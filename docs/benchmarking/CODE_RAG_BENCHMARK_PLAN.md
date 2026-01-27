# Kodit Benchmark: SWE-bench Implementation

## Overview

This document describes the SWE-bench benchmark implementation for evaluating Kodit's code retrieval capabilities. The benchmark uses real-world GitHub issues from the [SWE-bench](https://www.swebench.com/) dataset to measure how much Kodit's retrieval improves LLM patch generation.

---

## 1. Why SWE-bench?

SWE-bench tests repository-level issue resolution—a task where retrieval provides significant value:

| Kodit Capability | SWE-bench Requirement | Alignment |
|------------------|----------------------|-----------|
| Index Git repositories | Real GitHub repos at specific commits | ✅ Perfect |
| Hybrid search (BM25 + semantic) | Find relevant code for bug fixing | ✅ Perfect |
| AST-based snippet extraction | Locate functions/classes to modify | ✅ Perfect |
| Filter by repository | Each task targets a specific repo | ✅ Perfect |

**Why SWE-bench over RepoEval?**

| Feature | SWE-bench | RepoEval |
|---------|-----------|----------|
| Exact commit hashes | ✅ `base_commit` field | ❌ Snapshots only |
| Evaluation method | ✅ Real test execution | ⚠️ Token similarity |
| Task complexity | Real bug fixes | Function completion |
| Retrieval impact | High (large repos) | Medium |

From the [SWE-bench leaderboard](https://www.swebench.com/):
- RAG-based approaches (BM25 retrieval + LLM) achieve **4-7% on Lite**
- Agentless-Lite with embedding retrieval achieves **32% on Lite**
- This demonstrates significant headroom for better retrieval

---

## 2. Dataset

### 2.1 Data Source

The benchmark uses the official SWE-bench datasets from Hugging Face:

| Dataset | Size | Use Case |
|---------|------|----------|
| `princeton-nlp/SWE-bench_Lite` | 300 instances | Primary benchmark |
| `princeton-nlp/SWE-bench_Verified` | 500 instances | Extended benchmark |

### 2.2 Repositories

SWE-bench Lite covers 12 popular Python repositories:

| Repository | Instances | Description |
|------------|-----------|-------------|
| django/django | 114 | Web framework |
| sympy/sympy | 77 | Symbolic mathematics |
| matplotlib/matplotlib | 23 | Plotting library |
| scikit-learn/scikit-learn | 23 | Machine learning |
| pytest-dev/pytest | 17 | Testing framework |
| sphinx-doc/sphinx | 16 | Documentation generator |
| astropy/astropy | 6 | Astronomy library |
| psf/requests | 6 | HTTP library |
| pylint-dev/pylint | 6 | Code linter |
| pydata/xarray | 5 | N-D arrays |
| mwaskom/seaborn | 4 | Statistical visualization |
| pallets/flask | 3 | Web microframework |

### 2.3 Instance Format

Each instance contains:

```python
{
    "instance_id": "django__django-11049",        # Unique identifier
    "repo": "django/django",                      # GitHub repository
    "base_commit": "17455e924e24...",            # Exact commit to checkout
    "problem_statement": "...",                   # Issue description (natural language)
    "hints_text": "...",                          # Optional hints
    "patch": "diff --git a/...",                  # Ground truth fix
    "test_patch": "diff --git a/...",            # Test additions
    "FAIL_TO_PASS": ["test_invalid_string..."],  # Tests that should pass after fix
    "PASS_TO_PASS": ["test_other..."],           # Tests that should remain passing
    "version": "3.0",                             # Library version
    "environment_setup_commit": "...",           # Commit for environment setup
}
```

### 2.4 Example Task

```
Instance: django__django-11049
Commit: 17455e924e243e7a55e8a38f45966d8cbb27c273

Problem Statement:
  Correct expected format in invalid DurationField error message.
  The current error message says "[DD] [HH:[MM:]]ss[.uuuuuu]" but should
  be "[DD] [[HH:]MM:]ss[.uuuuuu]" because seconds are mandatory.

Expected Patch:
  diff --git a/django/db/models/fields/__init__.py
  -                     "[DD] [HH:[MM:]]ss[.uuuuuu] format.")
  +                     "[DD] [[HH:]MM:]ss[.uuuuuu] format.")

Tests to Fix:
  ["test_invalid_string (model_fields.test_durationfield.TestValidation)"]
```

---

## 3. Experimental Design

### 3.1 Conditions

| Condition | Description |
|-----------|-------------|
| **Baseline** | LLM generates patch with only the problem statement |
| **BM25** | LLM generates patch with BM25-retrieved context (SWE-bench baseline) |
| **Kodit** | LLM generates patch with Kodit-retrieved context |
| **Oracle** | LLM generates patch with gold file context (upper bound) |

### 3.2 Metrics

**Primary Metric**:
- **Resolve Rate**: Percentage of instances where generated patch makes `FAIL_TO_PASS` tests pass
- **Resolve Rate Delta**: `Resolve(Kodit) - Resolve(BM25)` — the improvement over baseline RAG

**Secondary Metrics**:
- **Retrieval Recall@k**: Fraction of modified files found in top-k results
- **Context Utilization**: How often retrieved context appears in generated patches

### 3.3 Evaluation

Evaluation uses the official SWE-bench harness with Docker containers:

1. Apply generated patch to repository at `base_commit`
2. Run `FAIL_TO_PASS` tests in isolated environment
3. Verify `PASS_TO_PASS` tests still pass (no regressions)
4. Instance is "resolved" only if all conditions met

---

## 4. Running the Benchmark

### 4.1 Quick Start

```bash
# Step 1: Setup - clone repos at specific commits, index with Kodit
uv run kodit benchmark setup --dataset swebench-lite

# Step 2: Run a single instance (for testing)
uv run kodit benchmark run-one django__django-11049

# Step 3: Run full benchmark
uv run kodit benchmark run --dataset swebench-lite --condition kodit

# Step 4: Evaluate predictions
uv run kodit benchmark evaluate results/predictions.jsonl
```

### 4.2 CLI Commands

```bash
# Show available instances
uv run kodit benchmark list --dataset swebench-lite --repo django/django

# Run specific instances
uv run kodit benchmark run \
    --instances django__django-11049 django__django-13447 \
    --model claude-3-5-sonnet-20241022 \
    --condition kodit

# Compare conditions
uv run kodit benchmark compare \
    results/baseline.jsonl \
    results/kodit.jsonl
```

### 4.3 Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `--dataset` | `swebench-lite` | Dataset variant (lite, verified, full) |
| `--model` | `claude-3-5-sonnet-20241022` | LiteLLM model identifier |
| `--condition` | `kodit` | Retrieval condition (baseline, bm25, kodit, oracle) |
| `--top-k` | `5` | Number of files/snippets to retrieve |
| `--instances` | all | Specific instance IDs to run |
| `--repo` | all | Filter to specific repository |

---

## 5. Architecture

### 5.1 Directory Structure

```
benchmarks/
├── __init__.py
├── cli.py                    # CLI commands (setup, run, evaluate)
├── swebench/
│   ├── __init__.py
│   ├── instance.py           # SWEBenchInstance dataclass
│   ├── loader.py             # HuggingFace dataset loader
│   ├── repository.py         # Git clone/checkout management
│   ├── retriever.py          # Kodit retrieval wrapper
│   ├── prompt.py             # Prompt templates
│   ├── generator.py          # LLM patch generation
│   └── evaluator.py          # SWE-bench harness wrapper
├── repos/                    # Cloned repositories (gitignored)
│   └── django__django-11049/ # Instance-specific checkout
├── results/                  # Benchmark outputs
│   └── predictions.jsonl
└── cache/                    # Indexed repository cache
```

### 5.2 Setup Process

The `setup` command prepares repositories for benchmarking:

1. **Load dataset**: Fetch from `princeton-nlp/SWE-bench_Lite`

2. **Clone repositories**: For each unique `(repo, base_commit)` pair:
   ```bash
   git clone https://github.com/{repo} repos/{instance_id}
   cd repos/{instance_id}
   git checkout {base_commit}
   ```

3. **Index with Kodit**: For each cloned repository:
   - POST to `/api/v1/repositories` with `file://` URI
   - Wait for indexing to complete
   - Store mapping: `instance_id → repository_id`

4. **Cache index**: Save Kodit database for reuse

### 5.3 Benchmark Pipeline

```
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│ Load Instance│───▶│   Retrieve   │───▶│ Build Prompt │
│  from HF     │    │  from Kodit  │    │  (issue+ctx) │
└──────────────┘    └──────────────┘    └──────┬───────┘
                                               │
┌──────────────┐    ┌──────────────┐           │
│   Evaluate   │◀───│   Generate   │◀──────────┘
│  with Docker │    │    Patch     │
└──────────────┘    └──────────────┘
```

---

## 6. Implementation Details

### 6.1 Retrieval Strategy

The `KoditRetriever` queries Kodit with the problem statement:

```python
class KoditRetriever:
    async def retrieve(self, instance: SWEBenchInstance, k: int = 5) -> list[RetrievedFile]:
        # Extract key terms from problem statement
        keywords = self._extract_keywords(instance.problem_statement)

        results = await self._search_service.search(
            user_intent=instance.problem_statement,
            keywords=keywords,
            source_repo=f"github.com/{instance.repo}",
        )

        # Group snippets by file, return top-k files
        files = self._group_by_file(results)
        return files[:k]
```

### 6.2 Prompt Template

Following the SWE-bench BM25 baseline format:

```
You will be provided with a partial code base and an issue statement explaining
a problem to resolve.

<issue>
{problem_statement}
</issue>

<code>
[start of {file_path_1}]
{file_content_1}
[end of {file_path_1}]

[start of {file_path_2}]
{file_content_2}
[end of {file_path_2}]
</code>

Generate a patch in unified diff format that resolves the issue.
Only output the patch, no explanations.
```

### 6.3 Prediction Format

Output follows SWE-bench evaluation format:

```jsonl
{"instance_id": "django__django-11049", "model_name_or_path": "kodit-claude", "model_patch": "diff --git a/..."}
{"instance_id": "django__django-13447", "model_name_or_path": "kodit-claude", "model_patch": "diff --git a/..."}
```

---

## 7. Results Format

### 7.1 Per-Instance Results

```json
{
  "instance_id": "django__django-11049",
  "condition": "kodit",
  "retrieved_files": ["django/db/models/fields/__init__.py"],
  "retrieval_recall": 1.0,
  "generated_patch": "diff --git a/...",
  "resolved": true,
  "fail_to_pass_results": {"test_invalid_string": "PASSED"},
  "latency_ms": 2340
}
```

### 7.2 Aggregate Results

```json
{
  "benchmark": "swebench-lite",
  "model": "claude-3-5-sonnet-20241022",
  "timestamp": "2024-01-15T10:30:00Z",
  "conditions": {
    "baseline": {
      "resolve_rate": 0.15,
      "retrieval_recall_5": 0.0,
      "instances_run": 300
    },
    "bm25": {
      "resolve_rate": 0.22,
      "retrieval_recall_5": 0.45,
      "instances_run": 300
    },
    "kodit": {
      "resolve_rate": 0.28,
      "retrieval_recall_5": 0.62,
      "instances_run": 300
    }
  },
  "kodit_delta_vs_baseline": "+13%",
  "kodit_delta_vs_bm25": "+6%"
}
```

---

## 8. Expected Results

Based on SWE-bench leaderboard data and CodeRAG-Bench findings:

| Condition | Expected Resolve Rate |
|-----------|----------------------|
| Baseline (no retrieval) | ~15% |
| BM25 retrieval | ~22% |
| Kodit retrieval | ~28% |
| Oracle (gold files) | ~45% |

**Key Insight**: The gap between BM25 (22%) and Oracle (45%) represents the potential improvement from better retrieval. Kodit's hybrid search should capture more of this potential than pure BM25.

---

## 9. Troubleshooting

### Repository cloning fails
- Ensure network access to GitHub
- Some repos may require authentication for private forks
- Use `--skip-clone` if repos already exist locally

### Indexing takes too long
- Large repos (django, sympy) can take 10-30 minutes
- Use `--repo` flag to test with smaller repos first (flask, requests)
- Pre-indexed caches can be shared across runs

### Docker evaluation fails
- Ensure Docker daemon is running
- SWE-bench requires significant disk space for containers
- Use `--dry-run` to test pipeline without evaluation

### API key issues
- Set appropriate API keys for your LLM provider
- `ANTHROPIC_API_KEY` for Claude models
- `OPENAI_API_KEY` for OpenAI models

---

## 10. Implementation Checklist

| # | Task | Priority | Status |
|---|------|----------|--------|
| 1 | Create `SWEBenchInstance` dataclass | High | TODO |
| 2 | Implement HuggingFace dataset loader | High | TODO |
| 3 | Implement repository clone/checkout | High | TODO |
| 4 | Implement Kodit retrieval wrapper | High | TODO |
| 5 | Implement prompt builder | High | TODO |
| 6 | Implement patch generator | High | TODO |
| 7 | Integrate SWE-bench evaluation harness | High | TODO |
| 8 | Add CLI commands | Medium | TODO |
| 9 | Add result aggregation and reporting | Medium | TODO |
| 10 | Add BM25 baseline comparison | Medium | TODO |

---

## 11. References

- [SWE-bench Website](https://www.swebench.com/)
- [SWE-bench GitHub](https://github.com/SWE-bench/SWE-bench)
- [SWE-bench Paper](https://arxiv.org/abs/2310.06770)
- [SWE-bench Lite Dataset](https://huggingface.co/datasets/princeton-nlp/SWE-bench_Lite)
- [Agentless Paper](https://arxiv.org/abs/2407.01489) - RAG-based approach achieving 32%
- [CodeRAG-Bench Paper](https://arxiv.org/abs/2406.14497) - Analysis of retrieval impact
