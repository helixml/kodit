---
title: "Kodit Developer Documentation"
linkTitle: Developer Docs
# next: /helix/getting-started
weight: 99
---

## Database

All database operations are handled by SQLAlchemy and Alembic.

### Creating a Database Migration

1. Make changes to your models
2. Ensure the model is referenced in [alembic's env.py](src/kodit/alembic/env.py)
3. Run `alembic upgrade head` to create a temporary DB to compute the upgrade
4. Run `alembic revision --autogenerate -m "your message"`
5. The new migration will be applied when you next run a kodit command

## Releasing

Performing a release is designed to be fully automated. If you spot opportunities to
improve the CI to help performing an automated release, please do so.

1. Create a new release in GitHub.
2. Set the version number. Use patch versions for bugfixes or minor small improvements.
   Use minor versions when adding significant new functionality. Use major versions for
   overhauls.
3. Generate the release notes. <- this could be improved, because we use a strict
   pr/commit naming structure.
4. Wait for all jobs to succeed, then you should be able to brew install, pipx install, etc.
