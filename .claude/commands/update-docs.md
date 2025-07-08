# Update Documentation Based on Recent Code Changes

## Objective

Analyze recent commits in the current Git branch and update relevant documentation to reflect code changes.

## Steps to Complete

### 1. Analyze Recent Commits

- Run `git log --oneline -n 20` to view the last 20 commits on the current branch, or
  all commits if it is not the main branch.
- For each commit, run `git show --name-status <commit-hash>` to see which files were modified
- Focus on commits that modified source code files (`.js`, `.py`, `.ts`, `.java`, `.go`, `.rs`, etc.)

### 2. Identify Code Changes

For each modified source file in recent commits:

- Examine the diff using `git diff <commit-hash>^ <commit-hash> -- <file-path>`
- Identify:
  - New functions or methods added
  - Functions or methods removed or renamed
  - Changes to function signatures (parameters, return types)
  - New classes or modules
  - Changes to public APIs
  - New configuration options or environment variables
  - Breaking changes

### 3. Update README.md

Check if the README.md needs updates for:

- **Installation instructions**: If dependencies or setup steps changed
- **Usage examples**: If APIs or interfaces changed
- **Configuration**: If new environment variables or config options were added
- **Features list**: If new features were implemented
- **Quick start guide**: If the basic usage pattern changed

### 4. Update Documentation in /docs

For each markdown file in the `docs/` folder:

- Check if it references any of the changed code
- Update:
  - API documentation with new/changed function signatures
  - Code examples that may no longer work
  - Configuration guides if settings changed
  - Architecture diagrams if structural changes occurred
  - Migration guides if there are breaking changes

### 5. Create or Update Specific Docs

Based on the changes found:

- If new features were added without documentation, create new doc files
- If breaking changes exist, create or update a migration guide
- If new APIs were added, ensure they have proper documentation

### 6. Verify Documentation Accuracy

- Ensure all code examples in documentation are up-to-date
- Check that any referenced file paths still exist
- Verify that installation and setup instructions still work

## Output Required

1. Summary of commits analyzed and significant changes found
2. List of documentation files updated with brief description of changes
3. Any new documentation files created
4. Warnings about potentially outdated documentation that needs manual review

## Important Notes

- Focus on user-facing changes that affect how people use Kodit
- Don't document internal implementation details unless they affect the public API
- Keep documentation concise and example-driven
- If unsure about a change's impact, flag it for manual review
- Ensure all documentation follows the existing style and format in the repository
