#!/bin/bash

# Migration Loop Script
# Runs Claude Code repeatedly to continue Python-to-Go migration work

set -e

PROMPT='Continue the Python-to-Go migration. Follow this workflow:

1. **Session Start Report**: Read MIGRATION.md and report:
   - Current phase
   - Count of completed vs remaining tasks in current context
   - Next uncompleted task (in dependency order)
   - Any blockers noted from previous sessions

2. **Execute Tasks**: For each task:
   - Read the Python source file(s) listed
   - Create the Go file in the correct location per CLAUDE.md structure
   - Write tests
   - Verify: `go build ./...`, `go test ./...`, `golangci-lint run`
   - Mark checkbox complete in MIGRATION.md: `- [x]`
   - Update verified status: `[x] builds [x] tests pass`

3. **After Each Task**: Commit progress to MIGRATION.md immediately so work is not lost.

4. **Session End**: Before stopping, update MIGRATION.md:
   - Add session date and summary to Notes > Session Log
   - Document any decisions made
   - Note any blockers or partial work
   - List recommended next tasks

Start by reading MIGRATION.md and giving me the Session Start Report. Then start the migration as you are running in the background.'

ITERATION=0

while true; do
    ITERATION=$((ITERATION + 1))
    echo "============================================"
    echo "Migration Loop - Iteration $ITERATION"
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
        COMMIT_MSG="migration: automated session $(date '+%Y-%m-%d %H:%M:%S')

Iteration $ITERATION of automated Python-to-Go migration.

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
