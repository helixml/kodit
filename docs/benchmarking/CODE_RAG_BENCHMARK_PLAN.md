# Kodit Benchmark Plan: Adapting CodeRAG-Bench

## Executive Summary

This document outlines a plan to benchmark Kodit's code retrieval capabilities by adapting data and methodologies from [CodeRAG-Bench](https://github.com/code-rag-bench/code-rag-bench). The goal is to measure the impact of Kodit's indexing and retrieval on AI code generation quality, comparing performance before and after indexing target repositories.

---

## 1. Background Research Findings

### 1.1 CodeRAG-Bench Overview

CodeRAG-Bench is a comprehensive benchmark for retrieval-augmented code generation (RACG) that includes:

- **8 coding tasks** across three difficulty tiers:
  - Basic: HumanEval (164), MBPP (500), LiveCodeBench (400)
  - Open-Domain: DS-1000 (1,000), ODEX (439)
  - Repository-Level: RepoEval (373), SWE-bench-Lite (300)

- **5 retrieval sources** totaling ~25M documents:
  - Programming solutions (1.1K samples)
  - Online tutorials (76K samples)
  - Library documentation (34K samples)
  - StackOverflow posts (2M samples)
  - GitHub repositories (712K samples)

- **Evaluation methodology**:
  - Retrieval: NDCG@10, Precision, Recall
  - Generation: Pass@1 (execution correctness)

### 1.2 Kodit Capabilities

Kodit provides:

- **Hybrid search** combining three modalities via Reciprocal Rank Fusion (RRF):
  - BM25 keyword search (stemmed, stopwords removed)
  - Semantic code search (code embeddings)
  - Semantic text search (text embeddings over summaries)

- **AST-based snippet extraction** supporting 25+ languages via Tree-sitter

- **MCP server interface** exposing `search()`, `get_api_docs()`, `get_architecture_docs()`, etc.

- **Enrichment layers**: architecture docs, API docs, commit descriptions, cookbook examples

### 1.3 Alignment Analysis

| CodeRAG-Bench Capability | Kodit Support | Notes |
|--------------------------|---------------|-------|
| BM25 retrieval | ✅ Full | LocalBM25Repository or VectorChordBM25Repository |
| Dense embeddings | ✅ Full | LiteLLM or Local embedding providers |
| Repository indexing | ✅ Full | Git-based indexing with commit tracking |
| Library documentation | ⚠️ Partial | Requires preprocessing; not natively extracted |
| StackOverflow/tutorials | ⚠️ Partial | Kodit focuses on code repos, not external docs |
| Code execution eval | ❌ None | Not in scope; use external harness |

**Key insight**: Kodit excels at repository-level tasks (RepoEval, SWE-bench) where the retrieval source is the target repository itself.

---

## 2. Benchmark Design

### 2.1 Evaluation Framework

```
┌─────────────────────────────────────────────────────────────────┐
│                    Benchmark Pipeline                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐    ┌──────────────┐    ┌──────────────────┐   │
│  │   Dataset   │───▶│   Indexing   │───▶│     Retrieval    │   │
│  │   (Tasks)   │    │   (Kodit)    │    │     (Kodit)      │   │
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

### 2.2 Metrics

**Primary Metrics**:
- **Pass@1**: Execution correctness of generated code (matches CodeRAG-Bench)
- **Pass@1 Delta**: `Pass@1(with Kodit) - Pass@1(baseline)` — the value Kodit adds

**Secondary Metrics**:
- **Retrieval Quality**: NDCG@5, Recall@5 against annotated ground-truth docs
- **Search Latency**: End-to-end retrieval time per query
- **Index Build Time**: Time to index each repository

### 2.3 Experimental Conditions

| Condition | Description |
|-----------|-------------|
| **Baseline** | LLM generates code with only the task prompt (no retrieval) |
| **Kodit RAG** | LLM generates code with task prompt + top-k Kodit results |
| **Gold Context** | LLM generates code with task prompt + ground-truth documents (upper bound) |

---

## 3. Selected Initial Benchmarks

Starting with 5 tasks that best align with Kodit's repository indexing strengths:

### 3.1 Task Selection Criteria

1. **Repository-centric**: Task requires knowledge from a specific codebase
2. **Annotated ground truth**: Known correct documents for retrieval evaluation
3. **Execution-based evaluation**: Can measure Pass@1
4. **Reasonable scope**: Can index repository in under 1 hour
5. **Python-focused**: Kodit's best-supported language for testing

### 3.2 Selected Tasks

| # | Task | Source | Size | Repository to Index | Rationale |
|---|------|--------|------|---------------------|-----------|
| 1 | **ODEX** | [HuggingFace](https://huggingface.co/datasets/code-rag-bench/odex) | 439 | Python standard library | Tests API knowledge retrieval; ground-truth docs annotated |
| 2 | **DS-1000** | [HuggingFace](https://huggingface.co/datasets/code-rag-bench/ds1000) | 1,000 | numpy, pandas, scipy, matplotlib, sklearn, pytorch, tensorflow | Tests library usage; known lib functions |
| 3 | **RepoEval** | CodeRAG-Bench | 373 | Original task repos | Repository-level completion; ideal Kodit use case |
| 4 | **HumanEval** | [HuggingFace](https://huggingface.co/datasets/code-rag-bench/humaneval) | 164 | Programming solutions corpus | Baseline; tests if general code helps |
| 5 | **MBPP** | [HuggingFace](https://huggingface.co/datasets/code-rag-bench/mbpp) | 500 | Programming solutions corpus | Similar to HumanEval, larger sample |

### 3.3 Repository Preparation

For each task, we need to prepare indexable corpora:

```
benchmarks/
├── odex/
│   ├── corpus/                    # Python stdlib documentation as code
│   │   └── python-stdlib/         # Cloned or generated code samples
│   ├── tasks.jsonl                # Benchmark tasks from HuggingFace
│   └── qrels.jsonl                # Ground-truth doc mappings
├── ds1000/
│   ├── corpus/
│   │   ├── numpy/                 # Library source + examples
│   │   ├── pandas/
│   │   └── ...
│   ├── tasks.jsonl
│   └── qrels.jsonl
├── repoeval/
│   ├── corpus/                    # Original repos from RepoEval
│   ├── tasks.jsonl
│   └── qrels.jsonl
└── ...
```

---

## 4. Implementation Plan

### Phase 1: Infrastructure Setup (Week 1)

#### 4.1.1 Create Benchmark Module Structure

```
src/kodit/benchmarks/
├── __init__.py
├── runner.py              # Main benchmark orchestrator
├── datasets/
│   ├── __init__.py
│   ├── base.py            # Abstract dataset interface
│   ├── odex.py            # ODEX dataset loader
│   ├── ds1000.py          # DS-1000 dataset loader
│   ├── repoeval.py        # RepoEval dataset loader
│   ├── humaneval.py       # HumanEval dataset loader
│   └── mbpp.py            # MBPP dataset loader
├── indexing/
│   ├── __init__.py
│   └── corpus_builder.py  # Prepare indexable corpora from datasets
├── retrieval/
│   ├── __init__.py
│   └── evaluator.py       # NDCG, Recall calculation
├── generation/
│   ├── __init__.py
│   ├── prompt_builder.py  # Assemble task + context prompts
│   └── executor.py        # Run LLM generation
└── evaluation/
    ├── __init__.py
    └── code_executor.py   # Sandboxed code execution for Pass@1
```

#### 4.1.2 Dataset Loading

```python
# datasets/base.py
from abc import ABC, abstractmethod
from dataclasses import dataclass

@dataclass
class BenchmarkTask:
    task_id: str
    prompt: str                    # Natural language task description
    canonical_solution: str        # Ground truth code
    test_code: str                 # Execution test cases
    entry_point: str               # Function to test
    ground_truth_docs: list[str]   # Known relevant document IDs

class BenchmarkDataset(ABC):
    @abstractmethod
    def load_tasks(self) -> list[BenchmarkTask]:
        """Load all benchmark tasks."""

    @abstractmethod
    def get_corpus_repos(self) -> list[str]:
        """Return repository URLs to index for this benchmark."""
```

### Phase 2: Corpus Indexing (Week 2)

#### 4.2.1 ODEX Corpus: Python Standard Library

```python
# indexing/corpus_builder.py

class ODEXCorpusBuilder:
    """Build indexable corpus from Python stdlib documentation."""

    async def build(self, output_path: Path) -> None:
        # Option A: Index CPython repo directly
        # - Clone https://github.com/python/cpython
        # - Kodit will extract all function/class definitions
        # - Map task library field to extracted snippets

        # Option B: Convert DevDocs.io library-documentation
        # - Download from HuggingFace: code-rag-bench/library-documentation
        # - Convert to fake repo structure with .py files
        # - Each doc becomes a module with docstrings
```

#### 4.2.2 DS-1000 Corpus: Data Science Libraries

```python
class DS1000CorpusBuilder:
    """Index data science library source code."""

    LIBRARIES = {
        'numpy': 'https://github.com/numpy/numpy',
        'pandas': 'https://github.com/pandas-dev/pandas',
        'scipy': 'https://github.com/scipy/scipy',
        'matplotlib': 'https://github.com/matplotlib/matplotlib',
        'sklearn': 'https://github.com/scikit-learn/scikit-learn',
        'pytorch': 'https://github.com/pytorch/pytorch',
        'tensorflow': 'https://github.com/tensorflow/tensorflow',
    }

    async def build(self) -> None:
        # Index each library's source code
        # Focus on: public APIs, docstrings, example code in tests
```

#### 4.2.3 RepoEval Corpus: Task-Specific Repos

RepoEval tasks come from specific repositories. We need to:
1. Download the RepoEval dataset to get repo URLs and commit SHAs
2. Clone each repo at the specified commit
3. Index with Kodit

### Phase 3: Retrieval Integration (Week 3)

#### 4.3.1 Kodit Search Integration

```python
# retrieval/kodit_retriever.py

class KoditRetriever:
    """Retrieve relevant code snippets via Kodit MCP."""

    async def retrieve(
        self,
        task: BenchmarkTask,
        k: int = 5,
    ) -> list[RetrievalResult]:
        """
        Query Kodit for relevant snippets.

        Uses task.prompt as the query, optionally filtered
        to the corpus repository.
        """
        results = await self.kodit_client.search(
            user_intent=task.prompt,
            filters=SearchFilters(
                source_repo=self.corpus_repo_url,
            ),
            limit=k,
        )
        return [
            RetrievalResult(
                doc_id=r.snippet.sha,
                content=r.snippet.content,
                score=sum(r.original_scores),
            )
            for r in results
        ]
```

#### 4.3.2 Retrieval Evaluation

```python
# retrieval/evaluator.py

def calculate_ndcg(
    retrieved: list[str],  # Retrieved doc IDs
    relevant: list[str],   # Ground-truth doc IDs
    k: int = 10,
) -> float:
    """Calculate NDCG@k for a single query."""

def calculate_recall(
    retrieved: list[str],
    relevant: list[str],
    k: int = 5,
) -> float:
    """Calculate Recall@k for a single query."""
```

### Phase 4: Generation Pipeline (Week 4)

#### 4.4.1 Prompt Assembly

```python
# generation/prompt_builder.py

class RAGPromptBuilder:
    """Build prompts with retrieved context."""

    TEMPLATE = '''You are an expert Python programmer. Complete the following function.

{retrieved_context}

## Task
{task_prompt}

## Your Solution
```python
'''

    def build(
        self,
        task: BenchmarkTask,
        retrieved: list[RetrievalResult] | None = None,
    ) -> str:
        context = ""
        if retrieved:
            context = "## Relevant Code Examples\n\n"
            for i, r in enumerate(retrieved, 1):
                context += f"### Example {i}\n```python\n{r.content}\n```\n\n"

        return self.TEMPLATE.format(
            retrieved_context=context,
            task_prompt=task.prompt,
        )
```

#### 4.4.2 LLM Generation

```python
# generation/executor.py

class LLMGenerator:
    """Generate code using LLM."""

    async def generate(
        self,
        prompt: str,
        model: str = "gpt-4o",
        temperature: float = 0.2,
    ) -> str:
        """Generate code completion."""
```

### Phase 5: Execution Evaluation (Week 5)

#### 4.5.1 Code Execution Sandbox

```python
# evaluation/code_executor.py

class CodeExecutor:
    """Execute generated code safely."""

    async def execute(
        self,
        generated_code: str,
        test_code: str,
        entry_point: str,
        timeout: float = 30.0,
    ) -> ExecutionResult:
        """
        Execute code in isolated environment.

        Uses Docker or subprocess with resource limits.
        Returns pass/fail and any error messages.
        """
```

#### 4.5.2 Pass@1 Calculation

```python
def calculate_pass_at_1(results: list[ExecutionResult]) -> float:
    """Calculate Pass@1 metric."""
    passed = sum(1 for r in results if r.passed)
    return passed / len(results)
```

---

## 5. Benchmark Runner

### 5.1 CLI Interface

```python
# runner.py

@click.command()
@click.option('--dataset', type=click.Choice(['odex', 'ds1000', 'repoeval', 'humaneval', 'mbpp']))
@click.option('--model', default='gpt-4o')
@click.option('--k', default=5, help='Number of documents to retrieve')
@click.option('--baseline/--no-baseline', default=True)
@click.option('--kodit/--no-kodit', default=True)
@click.option('--gold/--no-gold', default=False)
async def run_benchmark(dataset, model, k, baseline, kodit, gold):
    """Run Kodit benchmark suite."""
```

### 5.2 Output Format

```json
{
  "benchmark": "odex",
  "model": "gpt-4o",
  "retrieval_k": 5,
  "timestamp": "2026-01-20T12:00:00Z",
  "results": {
    "baseline": {
      "pass_at_1": 0.42,
      "tasks_evaluated": 439
    },
    "kodit_rag": {
      "pass_at_1": 0.58,
      "retrieval_ndcg_10": 0.65,
      "retrieval_recall_5": 0.72,
      "tasks_evaluated": 439,
      "avg_retrieval_latency_ms": 124
    },
    "gold_context": {
      "pass_at_1": 0.81,
      "tasks_evaluated": 439
    }
  },
  "delta": {
    "kodit_vs_baseline": 0.16,
    "gold_vs_baseline": 0.39,
    "kodit_ceiling_pct": 41.0
  }
}
```

---

## 6. Success Criteria

### 6.1 Minimum Viable Benchmark

- [ ] Successfully index at least one corpus (ODEX or HumanEval)
- [ ] Run baseline and Kodit RAG conditions on 100+ tasks
- [ ] Calculate Pass@1 for both conditions
- [ ] Show positive delta (Kodit RAG > baseline)

### 6.2 Full Benchmark Suite

- [ ] All 5 datasets indexed and evaluated
- [ ] Retrieval quality metrics (NDCG, Recall) calculated
- [ ] Gold context upper bound established
- [ ] Results reproducible across runs
- [ ] Latency measurements within acceptable bounds (<500ms per query)

### 6.3 Target Metrics

Based on CodeRAG-Bench findings, reasonable targets:

| Dataset | Baseline Pass@1 | Target Kodit Pass@1 | Expected Delta |
|---------|-----------------|---------------------|----------------|
| HumanEval | ~65% | ~70% | +5% |
| MBPP | ~55% | ~60% | +5% |
| ODEX | ~45% | ~55% | +10% |
| DS-1000 | ~40% | ~48% | +8% |
| RepoEval | ~35% | ~50% | +15% |

Note: Repository-level tasks (RepoEval) should show largest improvement since they directly match Kodit's use case.

---

## 7. Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Corpus preparation complexity | High | Start with HumanEval (simplest); iterate |
| Code execution security | High | Use Docker isolation; limit resources |
| LLM API costs | Medium | Sample subset for rapid iteration; cache responses |
| Ground-truth mapping misalignment | Medium | Manual validation on sample tasks |
| Retrieval returns irrelevant code | Medium | Tune search params; compare hybrid vs individual modes |

---

## 8. Future Extensions

1. **Additional Benchmarks**: SWE-bench-Lite, LiveCodeBench
2. **Multi-language Support**: Extend to Go, TypeScript benchmarks
3. **Ablation Studies**: Compare BM25-only vs semantic-only vs hybrid
4. **Prompt Engineering**: Test different context integration strategies
5. **Fine-tuning Analysis**: Measure if retrieval reduces need for domain fine-tuning

---

## 9. References

- [CodeRAG-Bench Paper](https://arxiv.org/abs/2406.14497)
- [CodeRAG-Bench GitHub](https://github.com/code-rag-bench/code-rag-bench)
- [CodeRAG-Bench Datasets on HuggingFace](https://huggingface.co/code-rag-bench)
- [HumanEval Dataset](https://huggingface.co/datasets/code-rag-bench/humaneval)
- [ODEX Dataset](https://huggingface.co/datasets/code-rag-bench/odex)
- [DS-1000 Dataset](https://huggingface.co/datasets/code-rag-bench/ds1000)

---

## Appendix A: Dataset Field Mappings

### A.1 ODEX Fields → Benchmark Task

| ODEX Field | BenchmarkTask Field |
|------------|---------------------|
| `task_id` | `task_id` |
| `intent` | `prompt` |
| `canonical_solution` | `canonical_solution` |
| `test_start` + `test` | `test_code` |
| `entry_point` | `entry_point` |
| `docs[*].title` | `ground_truth_docs` |

### A.2 HumanEval Fields → Benchmark Task

| HumanEval Field | BenchmarkTask Field |
|-----------------|---------------------|
| `task_id` | `task_id` |
| `prompt` | `prompt` |
| `canonical_solution` | `canonical_solution` |
| `test` | `test_code` |
| `entry_point` | `entry_point` |
| `docs[*].title` | `ground_truth_docs` |

---

## Appendix B: Example Workflow

```bash
# 1. Index the ODEX corpus (Python stdlib)
uv run kodit index --repo https://github.com/python/cpython --tag v3.12.0

# 2. Run benchmark
uv run python -m kodit.benchmarks.runner \
    --dataset odex \
    --model gpt-4o \
    --k 5 \
    --baseline \
    --kodit \
    --output results/odex_benchmark.json

# 3. View results
cat results/odex_benchmark.json | jq '.delta'
```
