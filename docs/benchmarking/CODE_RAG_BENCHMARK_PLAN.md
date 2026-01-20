# Kodit Benchmark Plan: RepoEval Adaptation

## Executive Summary

This document outlines a plan to benchmark Kodit's code retrieval capabilities using the RepoEval dataset from [CodeRAG-Bench](https://github.com/code-rag-bench/code-rag-bench). RepoEval is the ideal benchmark for Kodit because it tests repository-level code completion—the exact use case Kodit is designed for. We will start with 5 repositories to validate the approach before expanding.

---

## 1. Why RepoEval?

### 1.1 Alignment with Kodit's Purpose

Kodit indexes external codebases to help AI coding assistants. RepoEval tests exactly this:

| Kodit Capability | RepoEval Requirement | Alignment |
|------------------|---------------------|-----------|
| Index Git repositories | Tasks come from real GitHub repos | ✅ Perfect |
| Hybrid search (BM25 + semantic) | Find relevant code for completion | ✅ Perfect |
| AST-based snippet extraction | Function/class definitions needed | ✅ Perfect |
| Filter by repository | Tasks are repo-specific | ✅ Perfect |

### 1.2 CodeRAG-Bench Findings

From the [CodeRAG-Bench paper](https://arxiv.org/abs/2406.14497):
- Models gain **up to 17 points** on RepoEval with retrieved code snippets
- RepoEval shows the **largest improvement** from retrieval among all benchmarks
- This directly validates Kodit's value proposition

### 1.3 RepoEval Task Types

RepoEval covers three completion scenarios:

| Type | Description | Example |
|------|-------------|---------|
| **Line Completion** | Complete a single line of code | `result = model.` → `model.forward(x)` |
| **API Invocation** | Complete an API call | `torch.` → `torch.tensor([1,2,3])` |
| **Function Body** | Complete entire function body | Given signature + docstring, write implementation |

We will focus on **function body completion** as it best demonstrates Kodit's ability to provide relevant context.

---

## 2. Selected Repositories (Initial 5)

From the [RepoCoder dataset](https://github.com/microsoft/CodeT/tree/main/RepoCoder), we select 5 repositories for initial benchmarking:

| # | Repository | Description | Why Selected |
|---|------------|-------------|--------------|
| 1 | [CarperAI/trlx](https://github.com/CarperAI/trlx) | RLHF training library | Popular ML library, moderate size |
| 2 | [lucidrains/imagen-pytorch](https://github.com/lucidrains/imagen-pytorch) | Text-to-image diffusion | Well-documented, clear APIs |
| 3 | [maxhumber/redframes](https://github.com/maxhumber/redframes) | Data manipulation library | Small, fast to index |
| 4 | [huggingface/evaluate](https://github.com/huggingface/evaluate) | ML evaluation metrics | Familiar domain, good docs |
| 5 | [google/vizier](https://github.com/google/vizier) | Hyperparameter optimization | Clean Python, clear structure |

### 2.1 Selection Criteria

1. **Python-focused**: Kodit's best-supported language
2. **Moderate size**: Indexable without excessive time
3. **Well-structured**: Clear function/class definitions for AST extraction
4. **Diverse domains**: ML training, image gen, data manipulation, evaluation, optimization

---

## 3. Benchmark Design

### 3.1 Evaluation Framework

```
┌─────────────────────────────────────────────────────────────────┐
│                    RepoEval Benchmark Pipeline                   │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐    ┌──────────────┐    ┌──────────────────┐   │
│  │  RepoEval   │───▶│    Index     │───▶│     Retrieve     │   │
│  │   Tasks     │    │   w/ Kodit   │    │    w/ Kodit      │   │
│  └─────────────┘    └──────────────┘    └────────┬─────────┘   │
│                                                   │             │
│                                                   ▼             │
│  ┌─────────────┐    ┌──────────────┐    ┌──────────────────┐   │
│  │  Execution  │◀───│  Generation  │◀───│  Prompt Assembly │   │
│  │  Evaluation │    │    (LLM)     │    │  (Task + Context)│   │
│  └─────────────┘    └──────────────┘    └──────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 3.2 Experimental Conditions

| Condition | Description |
|-----------|-------------|
| **Baseline** | LLM generates code with only the task prompt (no retrieval) |
| **Kodit RAG** | LLM generates code with task prompt + top-5 Kodit results |
| **Gold Context** | LLM generates code with task prompt + ground-truth snippets (upper bound) |

### 3.3 Metrics

**Primary Metric**:
- **Pass@1**: Execution correctness of generated code
- **Pass@1 Delta**: `Pass@1(with Kodit) - Pass@1(baseline)` — the value Kodit adds

**Secondary Metric**:
- **Retrieval Recall@5**: Fraction of ground-truth snippets found in top-5 results

---

## 4. Implementation Plan

### Phase 1: Data Preparation

#### 4.1.1 Download RepoEval Dataset

```python
# scripts/download_repoeval.py

from huggingface_hub import hf_hub_download
import json

def download_repoeval():
    """Download RepoEval function completion tasks."""
    # RepoEval is part of CodeRAG-Bench on HuggingFace
    # Or download from Microsoft's RepoCoder repo
    pass
```

#### 4.1.2 Clone Target Repositories

```python
# scripts/clone_repos.py

REPOS = [
    ("CarperAI/trlx", "main"),
    ("lucidrains/imagen-pytorch", "main"),
    ("maxhumber/redframes", "main"),
    ("huggingface/evaluate", "main"),
    ("google/vizier", "main"),
]

async def clone_repos(output_dir: Path):
    """Clone repos at specific commits matching RepoEval checkpoints."""
    for repo, ref in REPOS:
        # Clone to output_dir/repo_name
        pass
```

#### 4.1.3 Filter Tasks for Selected Repos

```python
# scripts/filter_tasks.py

def filter_tasks_for_repos(
    all_tasks: list[dict],
    target_repos: list[str],
) -> list[dict]:
    """Keep only tasks from our 5 selected repositories."""
    return [t for t in all_tasks if t["repo"] in target_repos]
```

### Phase 2: Index with Kodit

#### 4.2.1 Index Each Repository

```bash
# Index each repository
for repo in trlx imagen-pytorch redframes evaluate vizier; do
    uv run kodit index --path ./repos/$repo
done
```

#### 4.2.2 Verify Indexing

```python
# scripts/verify_index.py

async def verify_index(repo_path: str) -> IndexStats:
    """Verify repository was indexed correctly."""
    return IndexStats(
        total_snippets=...,
        functions=...,
        classes=...,
        languages=...,
    )
```

### Phase 3: Run Benchmark

#### 4.3.1 Benchmark Task Structure

```python
# benchmarks/repoeval/task.py

@dataclass
class RepoEvalTask:
    task_id: str
    repo: str                      # e.g., "CarperAI/trlx"
    file_path: str                 # e.g., "trlx/trainer/base.py"
    function_signature: str        # Function to complete
    docstring: str                 # Function docstring
    ground_truth: str              # Expected function body
    context_snippets: list[str]    # Known relevant code (for gold condition)
```

#### 4.3.2 Retrieval with Kodit

```python
# benchmarks/repoeval/retriever.py

class KoditRetriever:
    """Retrieve relevant code for a RepoEval task."""

    async def retrieve(
        self,
        task: RepoEvalTask,
        k: int = 5,
    ) -> list[Snippet]:
        """
        Query Kodit for relevant snippets.

        Uses function signature + docstring as the query,
        filtered to the task's repository.
        """
        query = f"{task.function_signature}\n{task.docstring}"

        results = await self.kodit.search(
            user_intent=query,
            filters={"source_repo": task.repo},
            limit=k,
        )
        return results
```

#### 4.3.3 Prompt Assembly

```python
# benchmarks/repoeval/prompts.py

BASELINE_TEMPLATE = '''Complete the following Python function.

```python
{function_signature}
    """{docstring}"""
```

Return only the function body (no signature or docstring).
'''

RAG_TEMPLATE = '''Complete the following Python function.

## Relevant Code from Repository

{retrieved_snippets}

## Function to Complete

```python
{function_signature}
    """{docstring}"""
```

Return only the function body (no signature or docstring).
'''

def build_prompt(
    task: RepoEvalTask,
    retrieved: list[Snippet] | None = None,
) -> str:
    if retrieved:
        snippets_text = "\n\n".join(
            f"```python\n{s.content}\n```"
            for s in retrieved
        )
        return RAG_TEMPLATE.format(
            retrieved_snippets=snippets_text,
            function_signature=task.function_signature,
            docstring=task.docstring,
        )
    else:
        return BASELINE_TEMPLATE.format(
            function_signature=task.function_signature,
            docstring=task.docstring,
        )
```

#### 4.3.4 Generation and Evaluation

```python
# benchmarks/repoeval/runner.py

async def run_benchmark(
    tasks: list[RepoEvalTask],
    model: str = "gpt-4o",
    conditions: list[str] = ["baseline", "kodit", "gold"],
) -> BenchmarkResults:
    """Run benchmark across all conditions."""

    results = {}

    for condition in conditions:
        condition_results = []

        for task in tasks:
            # Get context based on condition
            if condition == "baseline":
                retrieved = None
            elif condition == "kodit":
                retrieved = await kodit_retriever.retrieve(task, k=5)
            elif condition == "gold":
                retrieved = task.context_snippets

            # Build prompt and generate
            prompt = build_prompt(task, retrieved)
            generated = await llm.generate(prompt, model=model)

            # Execute and check correctness
            passed = await executor.check(
                generated_code=generated,
                ground_truth=task.ground_truth,
                file_path=task.file_path,
            )

            condition_results.append(TaskResult(
                task_id=task.task_id,
                passed=passed,
                generated=generated,
            ))

        results[condition] = condition_results

    return BenchmarkResults(results)
```

### Phase 4: Analyze Results

#### 4.4.1 Output Format

```json
{
  "benchmark": "repoeval",
  "model": "gpt-4o",
  "repositories": [
    "CarperAI/trlx",
    "lucidrains/imagen-pytorch",
    "maxhumber/redframes",
    "huggingface/evaluate",
    "google/vizier"
  ],
  "results": {
    "baseline": {
      "pass_at_1": 0.35,
      "tasks_evaluated": 50
    },
    "kodit_rag": {
      "pass_at_1": 0.52,
      "retrieval_recall_5": 0.68,
      "tasks_evaluated": 50
    },
    "gold_context": {
      "pass_at_1": 0.78,
      "tasks_evaluated": 50
    }
  },
  "delta": {
    "kodit_vs_baseline": 0.17,
    "gold_vs_baseline": 0.43,
    "kodit_ceiling_pct": 39.5
  },
  "per_repo": {
    "CarperAI/trlx": { "baseline": 0.30, "kodit": 0.48, "gold": 0.75 },
    "lucidrains/imagen-pytorch": { "baseline": 0.32, "kodit": 0.50, "gold": 0.80 },
    "maxhumber/redframes": { "baseline": 0.45, "kodit": 0.60, "gold": 0.82 },
    "huggingface/evaluate": { "baseline": 0.38, "kodit": 0.55, "gold": 0.78 },
    "google/vizier": { "baseline": 0.30, "kodit": 0.47, "gold": 0.75 }
  }
}
```

---

## 5. Directory Structure

```
benchmarks/
├── repoeval/
│   ├── __init__.py
│   ├── task.py              # RepoEvalTask dataclass
│   ├── loader.py            # Load tasks from dataset
│   ├── retriever.py         # Kodit retrieval wrapper
│   ├── prompts.py           # Prompt templates
│   ├── executor.py          # Code execution sandbox
│   ├── runner.py            # Main benchmark orchestrator
│   └── analysis.py          # Results analysis and reporting
├── scripts/
│   ├── download_repoeval.py # Download dataset
│   ├── clone_repos.py       # Clone target repositories
│   ├── filter_tasks.py      # Filter to 5 repos
│   └── verify_index.py      # Check indexing worked
├── repos/                   # Cloned repositories
│   ├── trlx/
│   ├── imagen-pytorch/
│   ├── redframes/
│   ├── evaluate/
│   └── vizier/
└── results/                 # Benchmark outputs
    └── repoeval_YYYY-MM-DD.json
```

---

## 6. Success Criteria

### 6.1 Minimum Viable Benchmark

- [ ] Clone and index all 5 repositories
- [ ] Extract at least 10 tasks per repository (50 total)
- [ ] Run baseline and Kodit RAG conditions
- [ ] Calculate Pass@1 for both conditions
- [ ] **Show positive delta** (Kodit RAG > baseline)

### 6.2 Expected Results

Based on CodeRAG-Bench findings for RepoEval:

| Metric | Expected Value |
|--------|---------------|
| Baseline Pass@1 | ~35% |
| Kodit RAG Pass@1 | ~50% |
| Gold Context Pass@1 | ~78% |
| **Kodit Delta** | **+15%** |

The +15% improvement would demonstrate Kodit's value for repository-level coding tasks.

---

## 7. Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Repository checkpoints don't match RepoEval | Use exact commit SHAs from RepoCoder dataset |
| Task filtering leaves too few tasks | Expand to more repos or use line/API completion too |
| Code execution is flaky | Use Docker isolation; retry failed executions |
| Ground-truth snippets hard to map | Start with tasks that have clear snippet references |

---

## 8. Future Extensions

After validating with 5 repos:

1. **Expand to all 16 RepoEval repositories**
2. **Add line and API completion tasks** (not just function body)
3. **Compare retrieval modes**: BM25-only vs semantic-only vs hybrid
4. **Test multiple LLMs**: GPT-4o, Claude, Gemini
5. **Ablation on k**: Test k=1, 3, 5, 10 retrieved snippets

---

## 9. References

- [CodeRAG-Bench Paper](https://arxiv.org/abs/2406.14497)
- [RepoCoder Paper (introduced RepoEval)](https://arxiv.org/abs/2303.12570)
- [RepoCoder GitHub (contains RepoEval data)](https://github.com/microsoft/CodeT/tree/main/RepoCoder)
- [CodeRAG-Bench HuggingFace](https://huggingface.co/code-rag-bench)

---

## Appendix: RepoEval Task Example

```json
{
  "task_id": "trlx_func_001",
  "repo": "CarperAI/trlx",
  "file_path": "trlx/trainer/accelerate_base_trainer.py",
  "function_signature": "def save_pretrained(self, directory: str) -> None:",
  "docstring": "Save the model and tokenizer to the given directory.",
  "ground_truth": "self.model.save_pretrained(directory)\nself.tokenizer.save_pretrained(directory)",
  "context_snippets": [
    "# From trlx/models/modeling_ppo.py\ndef save_pretrained(self, save_directory):\n    self.base_model.save_pretrained(save_directory)\n    ...",
    "# From trlx/utils/loading.py\ndef get_checkpoint_path(config):\n    ..."
  ]
}
```
