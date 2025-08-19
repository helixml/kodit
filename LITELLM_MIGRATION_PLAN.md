# LiteLLM Provider Addition Plan

## Overview

This plan outlines the steps required to add new LiteLLM-based embedding and enrichment providers alongside the existing providers. This will enable support for 100+ LLM providers without changing any existing functionality.

## Benefits

- **Unified Interface**: Single provider implementation for all LLM services
- **Extensive Provider Support**: Support for 100+ providers including OpenAI, Anthropic, Azure, AWS Bedrock, Google Vertex AI, Cohere, Hugging Face, Ollama, and more
- **Simplified Configuration**: Consistent configuration pattern across all providers
- **Built-in Features**: Automatic retry logic, fallbacks, load balancing, and caching
- **Cost Tracking**: Built-in cost tracking and budgeting capabilities
- **Observability**: Better logging and monitoring through LiteLLM's built-in observability features

## Implementation Steps

### Phase 1: Update Configuration Schema

- Modify `src/kodit/config.py`:
  - Extend `EndpointType` to include `"litellm"` alongside existing `"openai"`
  - Add new optional fields to `Endpoint` class for LiteLLM support:
    - `litellm_model`: Optional[str] (full LiteLLM model string, e.g., "azure/gpt-4")
    - `api_version`: Optional[str] (for Azure)
    - `extra_params`: Optional[dict] (for provider-specific parameters)
  - Keep all existing fields and behavior unchanged

### Phase 2: Create New LiteLLM Providers

#### 2.1 Create LiteLLM Embedding Provider

Create `src/kodit/infrastructure/embedding/embedding_providers/litellm_embedding_provider.py`:

- Implement `EmbeddingProvider` interface
- Use `litellm.embedding()` for all embedding operations
- Support batching similar to current OpenAI provider
- Handle provider-specific model names (e.g., "text-embedding-3-small" for OpenAI, "embed-english-v3.0" for Cohere)
- Implement token counting using LiteLLM's built-in tokenizer
- Add support for custom embedding dimensions where applicable

#### 2.2 Create LiteLLM Enrichment Provider

Create `src/kodit/infrastructure/enrichment/litellm_enrichment_provider.py`:

- Implement `EnrichmentProvider` interface
- Use `litellm.acompletion()` for async chat completions
- Support streaming responses for better performance
- Maintain the same enrichment prompt structure
- Add support for function calling where applicable
- Implement cost tracking per enrichment

### Phase 3: Update Factory Methods

#### 3.1 Update Embedding Factory

Modify `src/kodit/infrastructure/embedding/embedding_factory.py`:

- Add new condition: if `endpoint.type == "litellm"`, create `LiteLLMEmbeddingProvider`
- Keep all existing provider logic unchanged (OpenAI, local)
- No changes to existing provider selection logic

#### 3.2 Update Enrichment Factory

Modify `src/kodit/infrastructure/enrichment/enrichment_factory.py`:

- Add new condition: if `endpoint.type == "litellm"`, create `LiteLLMEnrichmentProvider`
- Keep all existing provider logic unchanged
- No changes to existing provider selection logic

### Phase 4: Testing Strategy

#### 4.1 Test New Providers

- Create comprehensive tests for LiteLLM providers
- Mock LiteLLM responses for predictable testing
- Test various provider configurations (OpenAI via LiteLLM, Anthropic, Azure, etc.)
- Ensure existing tests continue to pass unchanged

### Phase 5: Documentation

#### 5.1 Update Configuration Documentation

- Document all supported providers
- Provide example configurations for popular providers
- Document provider-specific requirements and limitations

#### 5.2 Usage Guide

- How to use the new LiteLLM provider
- Common configuration examples for different providers
- Troubleshooting guide

## Configuration Examples

### OpenAI (existing provider - unchanged)

```yaml
default_endpoint:
  type: openai
  api_key: sk-...
  model: gpt-4o-mini
```

### OpenAI via LiteLLM (new option)

```yaml
default_endpoint:
  type: litellm
  api_key: sk-...
  litellm_model: gpt-4o-mini
```

### Anthropic via LiteLLM

```yaml
default_endpoint:
  type: litellm
  api_key: sk-ant-...
  litellm_model: claude-3-opus-20240229
```

### Azure OpenAI via LiteLLM

```yaml
default_endpoint:
  type: litellm
  api_key: ...
  base_url: https://myazure.openai.azure.com/
  api_version: 2024-02-15-preview
  litellm_model: azure/my-deployment-name
```

### AWS Bedrock via LiteLLM

```yaml
default_endpoint:
  type: litellm
  litellm_model: bedrock/anthropic.claude-3-opus-20240229-v1:0
  extra_params:
    aws_access_key_id: ...
    aws_secret_access_key: ...
    aws_region_name: us-east-1
```

### Local Ollama via LiteLLM

```yaml
default_endpoint:
  type: litellm
  base_url: http://localhost:11434
  litellm_model: ollama/llama2
```

## Risks and Mitigations

### Risk 1: Breaking Changes

- **Mitigation**: No changes to existing providers or configurations
- **Mitigation**: LiteLLM is opt-in only via `type: litellm`

### Risk 2: Performance Regression

- **Mitigation**: Benchmark against current implementation
- **Mitigation**: Optimize batching and concurrency settings per provider

### Risk 3: Provider-Specific Issues

- **Mitigation**: Implement provider-specific error handling
- **Mitigation**: Add comprehensive logging for debugging

### Risk 4: Dependency Size

- **Mitigation**: Make additional provider dependencies optional
- **Mitigation**: Document minimal vs full installation options

## Timeline Estimate

- Phase 1: 0.5 day (Update Configuration)
- Phase 2: 2 days (Create LiteLLM Providers)
- Phase 3: 0.5 day (Update Factories)
- Phase 4: 1-2 days (Testing)
- Phase 5: 0.5 day (Documentation)

**Total: 4.5-5.5 days**

## Success Metrics

- All existing tests pass unchanged
- New LiteLLM providers work with at least 5 different backends (OpenAI, Anthropic, Azure, Ollama, Bedrock)
- No changes required for existing users
- New providers accessible via simple `type: litellm` configuration
- Performance comparable to direct provider implementations
