"""Tests for the LocalEnrichmentProvider."""

import sys
from types import ModuleType

import pytest

from kodit.enrichment.enrichment_provider.enrichment_provider import (
    ENRICHMENT_SYSTEM_PROMPT,
    EnrichmentRequest,
)
from kodit.enrichment.enrichment_provider.local_enrichment_provider import (
    LocalEnrichmentProvider,
    DEFAULT_ENRICHMENT_MODEL,
)


# ---------------------------------------------------------------------------
# Fixtures & helpers
# ---------------------------------------------------------------------------


class _DummyEncoding(dict):
    """Minimal stand-in for a HuggingFace *BatchEncoding*."""

    def to(self, _device):  # pragma: no cover – trivial
        """HF encodings implement .to(device); return self for compatibility."""
        return self


class _DummyTokenizer:  # pylint: disable=too-few-public-methods
    """Very small stub for *AutoTokenizer* used in tests."""

    def apply_chat_template(self, messages, **_kwargs):  # noqa: D401
        """Return the user message content so tests stay deterministic."""
        user_msg = next(m for m in messages if m["role"] == "user")
        content = user_msg["content"]
        assert isinstance(content, str)
        return str(content)

    def __call__(self, batch, **_kwargs):  # noqa: D401
        # *batch* is a list of EmbeddingRequest objects – ignore the text.
        return _DummyEncoding({"input_ids": [[idx] for idx, _ in enumerate(batch)]})

    def decode(self, _ids, **_kwargs):  # noqa: D401
        return "mocked_enrichment"


class _DummyModel:  # pylint: disable=too-few-public-methods
    """Tiny stand-in for *AutoModelForCausalLM*."""

    device = "cpu"

    def generate(self, input_ids=None, **_kwargs):  # noqa: D401
        """Return *input_ids* with an extra dummy token appended.

        Static analysis complains if *input_ids* might be *None*, so default to an
        empty list in that case.
        """

        class _DummyTensor(list):
            """List-like object that mimics a *torch.Tensor* for our use-case."""

            # `.tolist()` should return the underlying Python list (possibly nested).
            def tolist(self):  # noqa: D401
                return list(self)

            # Preserve behaviour for indexing & slicing so provider code can do
            # `generated_ids.tolist()[0]` and still get a plain Python list.
            def __getitem__(self, key):  # noqa: D401
                result = super().__getitem__(key)
                return _DummyTensor(result) if isinstance(result, list) else result

        # Provider only cares about *generated_ids.tolist()[0]*; simulate that
        # shape by returning a 2-level nested list wrapped in DummyTensor.
        return _DummyTensor([[999]])


@pytest.fixture
def _patch_transformers(request):  # noqa: D401
    """Patch *transformers* modules so no large downloads happen during tests."""
    if "no_patch" in request.keywords:
        return
    # Build module hierarchy expected by the provider.
    transformers_mod = ModuleType("transformers")
    models_mod = ModuleType("transformers.models")
    auto_mod = ModuleType("transformers.models.auto")
    tok_auto_mod = ModuleType("transformers.models.auto.tokenization_auto")
    model_auto_mod = ModuleType("transformers.models.auto.modeling_auto")

    # Inject dummy classes.
    class _DummyAutoTokenizer:  # pylint: disable=too-few-public-methods
        @staticmethod
        def from_pretrained(*_args, **_kwargs):  # noqa: D401
            return _DummyTokenizer()

    class _DummyAutoModelForCausalLM:  # pylint: disable=too-few-public-methods
        @staticmethod
        def from_pretrained(*_args, **_kwargs):  # noqa: D401
            return _DummyModel()

    tok_auto_mod.AutoTokenizer = _DummyAutoTokenizer  # type: ignore[attr-defined]
    model_auto_mod.AutoModelForCausalLM = _DummyAutoModelForCausalLM  # type: ignore[attr-defined]

    # Register the modules so Python's import machinery can find them.
    for mod in [
        ("transformers", transformers_mod),
        ("transformers.models", models_mod),
        ("transformers.models.auto", auto_mod),
        ("transformers.models.auto.tokenization_auto", tok_auto_mod),
        ("transformers.models.auto.modeling_auto", model_auto_mod),
    ]:
        sys.modules[mod[0]] = mod[1]


@pytest.fixture
def provider():  # noqa: D401
    """Return a *LocalEnrichmentProvider* with default settings."""
    return LocalEnrichmentProvider()


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_initialization_default():  # noqa: D401
    """Provider initialises with the correct default model."""
    prov = LocalEnrichmentProvider()
    assert prov.model_name == DEFAULT_ENRICHMENT_MODEL


@pytest.mark.asyncio
async def test_initialization_custom():  # noqa: D401
    """Custom model name propagates correctly."""
    custom = "my-fancy-model"
    prov = LocalEnrichmentProvider(model_name=custom)
    assert prov.model_name == custom


@pytest.mark.asyncio
async def test_enrich_single_text(provider):  # noqa: D401
    text = "def hello(): print('Hello, world!')"
    enriched = [
        resp
        async for resp in provider.enrich([EnrichmentRequest(snippet_id=0, text=text)])
    ]

    assert len(enriched) == 1
    assert isinstance(enriched[0].text, str)
    assert len(enriched[0].text) > 0


@pytest.mark.asyncio
async def test_enrich_multiple_texts(provider):  # noqa: D401
    texts = [
        "def hello(): print('Hello, world!')",
        "def add(a, b): return a + b",
        "def multiply(a, b): return a * b",
    ]
    enriched = [
        resp
        async for resp in provider.enrich(
            [EnrichmentRequest(snippet_id=i, text=t) for i, t in enumerate(texts)]
        )
    ]

    assert len(enriched) == 3
    assert all(isinstance(r.text, str) for r in enriched)
    assert all(len(r.text) > 0 for r in enriched)


@pytest.mark.asyncio
async def test_enrich_empty_list(provider):  # noqa: D401
    enriched = [resp async for resp in provider.enrich([])]
    assert len(enriched) == 0


@pytest.mark.asyncio
async def test_enrich_empty_string_filtered(provider):  # noqa: D401
    """Empty strings should be ignored and return no enrichment."""
    enriched = [
        resp
        async for resp in provider.enrich([EnrichmentRequest(snippet_id=0, text="")])
    ]
    assert len(enriched) == 0


@pytest.mark.asyncio
async def test_enrich_order_consistency(provider):  # noqa: D401
    """Ensure order of outputs matches order of inputs despite batching."""
    requests = [
        EnrichmentRequest(snippet_id=i, text=f"def test_{i}(): pass") for i in range(20)
    ]
    enriched = [resp async for resp in provider.enrich(requests)]

    assert [r.snippet_id for r in enriched] == list(range(20))
    assert all(isinstance(r.text, str) and r.text for r in enriched)


@pytest.mark.asyncio
@pytest.mark.no_patch
async def test_must_not_contain_system_prompt(provider):  # noqa: D401
    """The system prompt must not be included in the output."""
    text = "def hello(): print('Hello, world!')"
    enriched = [
        resp
        async for resp in provider.enrich([EnrichmentRequest(snippet_id=0, text=text)])
    ]
    assert ENRICHMENT_SYSTEM_PROMPT not in enriched[0].text
