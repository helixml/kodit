"""Tests for the local enrichment provider."""

from unittest.mock import MagicMock, patch

import pytest

from kodit.domain.value_objects import EnrichmentRequest
from kodit.infrastructure.enrichment.local_enrichment_provider import (
    LocalEnrichmentProvider,
)


class TestLocalEnrichmentProvider:
    """Test the local enrichment provider."""

    def test_init_default_values(self) -> None:
        """Test initialization with default values."""
        provider = LocalEnrichmentProvider()
        assert provider.model_name == "Qwen/Qwen3-0.6B"
        assert provider.context_window == 2048
        assert provider.model is None
        assert provider.tokenizer is None

    def test_init_custom_values(self) -> None:
        """Test initialization with custom values."""
        provider = LocalEnrichmentProvider(model_name="test-model", context_window=1024)
        assert provider.model_name == "test-model"
        assert provider.context_window == 1024

    @pytest.mark.asyncio
    async def test_enrich_empty_requests(self) -> None:
        """Test enrichment with empty requests."""
        provider = LocalEnrichmentProvider()
        requests: list[EnrichmentRequest] = []

        results = [result async for result in provider.enrich(requests)]

        assert len(results) == 0

    @pytest.mark.asyncio
    async def test_enrich_empty_text_requests(self) -> None:
        """Test enrichment with requests containing empty text."""
        provider = LocalEnrichmentProvider()
        requests = [
            EnrichmentRequest(snippet_id=1, text=""),
            EnrichmentRequest(snippet_id=2, text="   "),
        ]

        results = [result async for result in provider.enrich(requests)]

        # The local provider actually processes whitespace-only text
        # So we expect 1 result for the whitespace-only request
        assert len(results) == 1
        assert results[0].snippet_id == 2

    @pytest.mark.asyncio
    @patch("transformers.models.auto.tokenization_auto.AutoTokenizer")
    @patch("transformers.models.auto.modeling_auto.AutoModelForCausalLM")
    async def test_enrich_single_request(
        self, mock_model_class: MagicMock, mock_tokenizer_class: MagicMock
    ) -> None:
        """Test enrichment with a single request."""
        # Mock the tokenizer
        mock_tokenizer = MagicMock()
        mock_tokenizer.apply_chat_template.return_value = "mocked prompt"

        # Create a proper mock for the tokenizer return value with a 'to' method
        mock_tokenizer_output = MagicMock()
        mock_tokenizer_output.to.return_value = mock_tokenizer_output
        mock_tokenizer.return_value = mock_tokenizer_output

        mock_tokenizer.decode.return_value = "This is a test function"
        mock_tokenizer_class.from_pretrained.return_value = mock_tokenizer

        # Mock the model
        mock_model = MagicMock()
        mock_model.device = "cpu"
        mock_model.generate.return_value = MagicMock()
        mock_model_class.from_pretrained.return_value = mock_model

        provider = LocalEnrichmentProvider()
        requests = [EnrichmentRequest(snippet_id=1, text="def test(): pass")]

        results = [result async for result in provider.enrich(requests)]

        assert len(results) == 1
        assert results[0].snippet_id == 1
        assert results[0].text == "This is a test function"

        # Verify the tokenizer was called correctly
        mock_tokenizer.apply_chat_template.assert_called_once()
        # The tokenizer is called with the prompt text, not the return value
        mock_tokenizer.assert_called_once_with(
            "mocked prompt",
            return_tensors="pt",
            padding=True,
            truncation=True,
        )
        mock_model.generate.assert_called_once()

    @pytest.mark.asyncio
    @patch("transformers.models.auto.tokenization_auto.AutoTokenizer")
    @patch("transformers.models.auto.modeling_auto.AutoModelForCausalLM")
    async def test_enrich_multiple_requests(
        self, mock_model_class: MagicMock, mock_tokenizer_class: MagicMock
    ) -> None:
        """Test enrichment with multiple requests."""
        # Mock the tokenizer
        mock_tokenizer = MagicMock()
        mock_tokenizer.apply_chat_template.return_value = "mocked prompt"

        # Create a proper mock for the tokenizer return value with a 'to' method
        mock_tokenizer_output = MagicMock()
        mock_tokenizer_output.to.return_value = mock_tokenizer_output
        mock_tokenizer.return_value = mock_tokenizer_output

        mock_tokenizer.decode.return_value = "Enriched content"
        mock_tokenizer_class.from_pretrained.return_value = mock_tokenizer

        # Mock the model
        mock_model = MagicMock()
        mock_model.device = "cpu"
        mock_model.generate.return_value = MagicMock()
        mock_model_class.from_pretrained.return_value = mock_model

        provider = LocalEnrichmentProvider()
        requests = [
            EnrichmentRequest(snippet_id=1, text="def hello(): pass"),
            EnrichmentRequest(snippet_id=2, text="def world(): pass"),
        ]

        results = [result async for result in provider.enrich(requests)]

        assert len(results) == 2
        assert results[0].snippet_id == 1
        assert results[0].text == "Enriched content"
        assert results[1].snippet_id == 2
        assert results[1].text == "Enriched content"

    @pytest.mark.asyncio
    @patch("transformers.models.auto.tokenization_auto.AutoTokenizer")
    @patch("transformers.models.auto.modeling_auto.AutoModelForCausalLM")
    async def test_enrich_mixed_requests(
        self, mock_model_class: MagicMock, mock_tokenizer_class: MagicMock
    ) -> None:
        """Test enrichment with mixed valid and empty requests."""
        # Mock the tokenizer
        mock_tokenizer = MagicMock()
        mock_tokenizer.apply_chat_template.return_value = "mocked prompt"

        # Create a proper mock for the tokenizer return value with a 'to' method
        mock_tokenizer_output = MagicMock()
        mock_tokenizer_output.to.return_value = mock_tokenizer_output
        mock_tokenizer.return_value = mock_tokenizer_output

        mock_tokenizer.decode.return_value = "Enriched content"
        mock_tokenizer_class.from_pretrained.return_value = mock_tokenizer

        # Mock the model
        mock_model = MagicMock()
        mock_model.device = "cpu"
        mock_model.generate.return_value = MagicMock()
        mock_model_class.from_pretrained.return_value = mock_model

        provider = LocalEnrichmentProvider()
        requests = [
            EnrichmentRequest(snippet_id=1, text=""),  # Empty
            EnrichmentRequest(snippet_id=2, text="def valid(): pass"),  # Valid
            EnrichmentRequest(snippet_id=3, text="   "),  # Whitespace only
        ]

        results = [result async for result in provider.enrich(requests)]

        # Should process the valid and whitespace requests (2 total)
        assert len(results) == 2
        assert results[0].snippet_id == 2
        assert results[0].text == "Enriched content"
        assert results[1].snippet_id == 3
        assert results[1].text == "Enriched content"

    @pytest.mark.asyncio
    @patch("transformers.models.auto.tokenization_auto.AutoTokenizer")
    @patch("transformers.models.auto.modeling_auto.AutoModelForCausalLM")
    async def test_enrich_tokenizer_initialization(
        self, mock_model_class: MagicMock, mock_tokenizer_class: MagicMock
    ) -> None:
        """Test that tokenizer is initialized correctly."""
        # Mock the tokenizer
        mock_tokenizer = MagicMock()
        mock_tokenizer.apply_chat_template.return_value = "mocked prompt"

        # Create a proper mock for the tokenizer return value with a 'to' method
        mock_tokenizer_output = MagicMock()
        mock_tokenizer_output.to.return_value = mock_tokenizer_output
        mock_tokenizer.return_value = mock_tokenizer_output

        mock_tokenizer.decode.return_value = "Enriched content"
        mock_tokenizer_class.from_pretrained.return_value = mock_tokenizer

        # Mock the model
        mock_model = MagicMock()
        mock_model.device = "cpu"
        mock_model.generate.return_value = MagicMock()
        mock_model_class.from_pretrained.return_value = mock_model

        provider = LocalEnrichmentProvider(model_name="custom-model")
        requests = [EnrichmentRequest(snippet_id=1, text="def test(): pass")]

        # First call should initialize tokenizer
        results = [result async for result in provider.enrich(requests)]

        # Verify tokenizer was initialized with correct model
        mock_tokenizer_class.from_pretrained.assert_called_once_with(
            "custom-model", padding_side="left"
        )

        # Second call should reuse existing tokenizer
        mock_tokenizer_class.from_pretrained.reset_mock()
        results = []
        async for result in provider.enrich(requests):
            results.append(result)

        # Should not call from_pretrained again
        mock_tokenizer_class.from_pretrained.assert_not_called()

    @pytest.mark.asyncio
    @patch("transformers.models.auto.tokenization_auto.AutoTokenizer")
    @patch("transformers.models.auto.modeling_auto.AutoModelForCausalLM")
    async def test_enrich_model_initialization(
        self, mock_model_class: MagicMock, mock_tokenizer_class: MagicMock
    ) -> None:
        """Test that model is initialized correctly."""
        # Mock the tokenizer
        mock_tokenizer = MagicMock()
        mock_tokenizer.apply_chat_template.return_value = "mocked prompt"

        # Create a proper mock for the tokenizer return value with a 'to' method
        mock_tokenizer_output = MagicMock()
        mock_tokenizer_output.to.return_value = mock_tokenizer_output
        mock_tokenizer.return_value = mock_tokenizer_output

        mock_tokenizer.decode.return_value = "Enriched content"
        mock_tokenizer_class.from_pretrained.return_value = mock_tokenizer

        # Mock the model
        mock_model = MagicMock()
        mock_model.device = "cpu"
        mock_model.generate.return_value = MagicMock()
        mock_model_class.from_pretrained.return_value = mock_model

        provider = LocalEnrichmentProvider(model_name="custom-model")
        requests = [EnrichmentRequest(snippet_id=1, text="def test(): pass")]

        # First call should initialize model
        results = [result async for result in provider.enrich(requests)]

        # Verify model was initialized with correct parameters
        mock_model_class.from_pretrained.assert_called_once_with(
            "custom-model",
            torch_dtype="auto",
            trust_remote_code=True,
            device_map="auto",
        )

        # Second call should reuse existing model
        mock_model_class.from_pretrained.reset_mock()
        results = []
        async for result in provider.enrich(requests):
            results.append(result)

        # Should not call from_pretrained again
        mock_model_class.from_pretrained.assert_not_called()
