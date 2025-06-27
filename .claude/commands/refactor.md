# Refactor Code Command

Safely improve code structure while preserving functionality.

## Core Principles

- **Test First**: Ensure tests exist and pass before starting
- **Small Steps**: Make incremental changes, test after each
- **Preserve Behavior**: External functionality must remain identical

## Refactoring Process

1. **Analyze** - Understand current code and identify improvement areas
2. **Test Coverage** - Verify/add tests before changing anything
3. **Plan** - Choose refactoring technique (extract method, rename, simplify conditionals)
4. **Execute** - Make one small change at a time, running tests after each
5. **Verify** - Ensure all tests pass and performance hasn't degraded

## Key Improvements

- **Clarity**: Better names, simpler logic, shorter methods
- **Structure**: Remove duplication, improve separation of concerns, improve cohesion
- **Maintainability**: Apply patterns where beneficial, standardize error handling
- **Documentation**: Update comments and docs to match changes

## Safety Checklist

- [ ] All tests passing before and after
- [ ] No functionality changes
- [ ] Code reviewed for quality
- [ ] Changes documented

Remember: Working code > perfect code. Commit frequently, refactor incrementally.
