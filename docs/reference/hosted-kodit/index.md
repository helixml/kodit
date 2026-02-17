---
title: Hosted Kodit Service
description: Information about the hosted version of Kodit.
weight: 4
---

The hosted Kodit service at [https://kodit.helix.ml](https://kodit.helix.ml) provides instant access to Kodit's powerful code search capabilities without requiring any installation or setup. Perfect for teams and individuals who want to start using Kodit immediately.

## Features

### Pre-indexed Popular Repositories

The hosted service comes with a curated collection of popular open-source repositories already indexed and ready to search, including:

- Major frameworks and libraries across multiple languages
- Popular development tools and utilities
- Well-documented codebases that serve as excellent examples
- Regularly [updated indices](../sync/index.md) to reflect the latest code changes

### Zero Configuration

- No installation required
- No database setup needed
- No embedding model configuration
- Just add the MCP server URL to your AI coding assistant and start searching

### Full Search Capabilities

The hosted service provides the same powerful search features as self-hosted Kodit:

- **Hybrid search**: Combines semantic understanding with keyword matching
- **Multi-language support**: Search across 20+ programming languages
- **Advanced filtering**: Filter by language, author, date range, and file paths
- **Context-aware results**: Get relevant code snippets with their dependencies
- **Rich metadata**: Each result includes file path, language, author, and commit information

## Getting Started

To use the hosted Kodit service:

1. Add the MCP server URL to your AI coding assistant:

   ```txt
   https://kodit.helix.ml/mcp
   ```

2. Start searching! Your AI assistant can now access the indexed repositories.

For detailed integration instructions, see our [integration guides](../../getting-started/integration/index.md).

## Requesting New Repositories

We're continuously expanding our indexed repository collection. To request a new repository:

1. **Open a GitHub Discussion**: Visit [our discussions page](https://github.com/helixml/kodit/discussions) and create a new discussion in the "Repository Requests" category

2. **Provide Repository Details**:
   - Repository URL (must be publicly accessible)
   - Why this repository would benefit the community
   - Expected usage and relevance

3. **Evaluation Criteria**: We prioritize repositories that are:
   - Actively maintained
   - Well-documented
   - Widely used in the community
   - Good examples of coding practices
   - Compliant with open-source licenses

Note: We can only index publicly accessible repositories. For private repositories,
consider setting up a [self-hosted Kodit instance](../../getting-started/_index.md).

## Service Level Agreement (SLA)

### Best Effort Service

The hosted Kodit service is provided on a **best-effort basis**:

- No guaranteed uptime SLA
- Maintenance windows may occur without prior notice
- Search performance may vary based on usage

### Enterprise Support

For organizations requiring:

- Guaranteed uptime and SLAs
- Priority support
- Custom repository indexing
- Dedicated infrastructure
- Private repository support

Please contact our team at [founders@helix.ml](mailto:founders@helix.ml) to discuss
enterprise options.

## Limitations

### Compared to Self-Hosted

While the hosted service is convenient, self-hosted Kodit offers:

- **Private repository indexing**: Index your proprietary codebases
- **Custom configuration**: Choose your embedding models and search parameters
- **Data control**: Keep all data within your infrastructure
- **Unlimited repositories**: No restrictions on what you can index

### Usage Guidelines

To ensure fair access for all users:

- Rate limiting may be applied during high usage periods
- Excessive API calls may result in temporary throttling
- The service is intended for development use, not production systems

## Security and Privacy

### Data Protection

- All searches are processed securely over HTTPS
- Search queries are not logged or stored
- No personal data is collected beyond standard web server logs
- The service only indexes publicly available code
- [Telemetry](../telemetry/index.md) is enabled

### Compliance

- Only repositories with appropriate open-source licenses are indexed
- Follows GDPR and privacy regulations

## Performance Expectations

### Search Speed

- Most searches complete in under 2 seconds
- Complex semantic searches may take slightly longer
- Performance depends on query complexity and current load

### Index Freshness

- Popular repositories are re-indexed every 30 minutes
- Less active repositories may be updated monthly

## Benefits of Hosted Service

### For Individuals

- Try Kodit without commitment
- Learn from high-quality code examples
- No infrastructure costs
- Instant access to popular codebases

### For Teams

- Quick evaluation of Kodit's capabilities
- Shared access to indexed repositories
- No maintenance overhead
- Easy onboarding for new developers

## Migration to Self-Hosted

When you're ready to move to a self-hosted instance:

1. Follow our [installation guide](../../getting-started/_index.md)
2. Index your desired repositories
3. Configure your preferred models and settings
4. Update your AI assistant's MCP server URL

All search patterns and workflows remain the same, ensuring a smooth transition.

## Feedback and Support

We value your feedback on the hosted service:

- **Feature requests**: Open a [GitHub discussion](https://github.com/helixml/kodit/discussions)
- **Bug reports**: File an [issue on GitHub](https://github.com/helixml/kodit/issues)
- **Enterprise inquiries**: Contact [founders@helix.ml](mailto:founders@helix.ml)

## Future Enhancements

We're actively working on:

- Expanding the repository collection
- Improving search performance
- Adding more language-specific features
- Enhanced filtering options
- Integrating into Helix
- Providing visibility of indexed repositories

Stay updated by watching our [GitHub repository](https://github.com/helixml/kodit) and following our release notes.
