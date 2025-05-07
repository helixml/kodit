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
3. Run `alembic revision --autogenerate -m "your message"`
4. The new migration will be applied when you next run a kodit command
