#!/bin/bash

# Refactor Loop Script
# Runs Claude Code repeatedly to continue library refactoring work

set -e

PROMPT='Continue the library-first architecture refactoring. Follow this workflow:

1. **Session Start Report**: Read go-target/REFACTOR.md and report:
   - Current phase (1-6)
   - Progress in current phase (completed vs remaining sub-tasks)
   - Next uncompleted task (in dependency order)
   - Any blockers noted from previous sessions

2. **Execute Tasks**: For each task:
   - Move/restructure files as specified in the mapping
   - Update imports across the codebase
   - Write/update tests as needed
   - Verify: `go build ./...`, `go test ./...`, `golangci-lint run`
   - Mark checkbox complete in REFACTOR.md: `- [x]`

3. **After Each Task**: Update REFACTOR.md immediately so work is not lost.

4. **Session End**: Before stopping, update REFACTOR.md:
   - Add session date and summary to notes
   - Document any decisions made
   - Note any blockers or partial work
   - List recommended next tasks

Start by reading go-target/REFACTOR.md and giving me the Session Start Report. Then start the refactoring as you are running in the background.'

ITERATION=0

while true; do
    ITERATION=$((ITERATION + 1))
    echo "============================================"
    echo "Refactor Loop - Iteration $ITERATION"
    echo "Started at: $(date)"
    echo "============================================"

    # Run Claude Code with the prompt
    # --print runs in non-interactive mode and exits when done
    # --dangerously-skip-permissions skips permission prompts (remove if you want manual approval)
    claude --print "$PROMPT" || {
        echo "Claude exited with error code $?"
        echo "Pausing for 10 seconds before retry..."
        sleep 10
    }

    echo ""
    echo "Claude session completed at: $(date)"
    echo ""

    # Check if there are any changes to commit
    if git diff --quiet && git diff --cached --quiet && [ -z "$(git ls-files --others --exclude-standard)" ]; then
        echo "No changes to commit. Continuing to next iteration..."
    else
        echo "Staging all changes..."
        git add -A

        # Generate a commit message based on date/time
        COMMIT_MSG="refactor: automated session $(date '+%Y-%m-%d %H:%M:%S')

Iteration $ITERATION of automated library-first architecture refactoring.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"

        echo "Committing with message:"
        echo "$COMMIT_MSG"
        echo ""

        git commit -m "$COMMIT_MSG" || echo "Commit failed or nothing to commit"
    fi

    echo ""
    echo "Waiting 5 seconds before next iteration..."
    echo "(Press Ctrl+C to stop)"
    sleep 5
done
