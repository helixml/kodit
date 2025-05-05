# Contributing to kodit

Thank you for your interest in contributing to kodit! This document provides guidelines and best practices for contributing to our project.

<blockquote class='warning-note'>
     🔐 <b>Important:</b> If you discover a security vulnerability, please use the <a href="https://github.com/helixml/kodit/security/advisories/new">Github security tool to report it privately</a>.
</blockquote>

## Pull Request Guidelines

### Commit History

- Feel free to use whatever commit history you prefer in your PR
- All PRs will be squashed when merged
- The PR title will become the final commit message

### PR Title Format

Please format your PR titles using [Conventional Commits](https://www.conventionalcommits.org/) notation:

```
<type>(<scope>): <description>
```

Common types include:

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or modifying tests
- `chore`: Maintenance tasks

Examples:

- `feat(api): add user authentication endpoint`
- `fix(ui): resolve button alignment issue`
- `docs(readme): update installation instructions`

### Code Quality

- Ensure your code is properly formatted using `ruff`
- Include tests for new features and bug fixes
- Update documentation as needed
- Keep PRs focused and manageable in size

## License

By submitting a pull request to this project, you agree to license your contribution under the Apache License 2.0. This means that your code will be available under the same license as the [rest of the project](../LICENSE).
