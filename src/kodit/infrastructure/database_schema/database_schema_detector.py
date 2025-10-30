"""Database schema detector for discovering database schemas in a repository."""

import re
from pathlib import Path


class DatabaseSchemaDetector:
    """Detects database schemas from various sources in a repository."""

    # File patterns to look for
    MIGRATION_PATTERNS = [
        "**/migrations/**/*.sql",
        "**/migrations/**/*.py",
        "**/migrate/**/*.sql",
        "**/migrate/**/*.go",
        "**/db/migrate/**/*.rb",
        "**/alembic/versions/**/*.py",
        "**/liquibase/**/*.xml",
        "**/flyway/**/*.sql",
    ]

    SQL_FILE_PATTERNS = [
        "**/*.sql",
        "**/schema/**/*.sql",
        "**/schemas/**/*.sql",
        "**/database/**/*.sql",
        "**/db/**/*.sql",
    ]

    ORM_MODEL_PATTERNS = [
        "**/models/**/*.py",  # SQLAlchemy, Django
        "**/models/**/*.go",  # GORM
        "**/entities/**/*.py",  # SQLAlchemy
        "**/entities/**/*.ts",  # TypeORM
        "**/entities/**/*.js",  # TypeORM/Sequelize
    ]

    # Regex patterns for schema detection
    CREATE_TABLE_PATTERN = re.compile(
        r"CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?[`\"]?(\w+)[`\"]?",
        re.IGNORECASE,
    )

    SQLALCHEMY_MODEL_PATTERN = re.compile(
        r"class\s+(\w+)\s*\([^)]*(?:Base|Model|db\.Model)[^)]*\):",
        re.MULTILINE,
    )

    GORM_MODEL_PATTERN = re.compile(
        r"type\s+(\w+)\s+struct\s*{[^}]*gorm\.Model",
        re.MULTILINE | re.DOTALL,
    )

    TYPEORM_ENTITY_PATTERN = re.compile(
        r"@Entity\([^)]*\)\s*(?:export\s+)?class\s+(\w+)",
        re.MULTILINE,
    )

    async def discover_schemas(self, repo_path: Path) -> str:
        """Discover database schemas and generate a structured report."""
        findings = {
            "tables": set(),
            "migration_files": [],
            "sql_files": [],
            "orm_models": [],
            "orm_type": None,
        }

        # Detect migration files
        await self._detect_migrations(repo_path, findings)

        # Detect SQL schema files
        await self._detect_sql_files(repo_path, findings)

        # Detect ORM models
        await self._detect_orm_models(repo_path, findings)

        # Generate report
        return self._generate_report(findings)

    async def _detect_migrations(self, repo_path: Path, findings: dict) -> None:
        """Detect migration files."""
        for pattern in self.MIGRATION_PATTERNS:
            for file_path in repo_path.glob(pattern):
                if file_path.is_file():
                    findings["migration_files"].append(str(file_path.relative_to(repo_path)))
                    # Try to extract table names from migrations
                    await self._extract_tables_from_file(file_path, findings)

    async def _detect_sql_files(self, repo_path: Path, findings: dict) -> None:
        """Detect SQL schema files."""
        migration_paths = set(findings["migration_files"])

        for pattern in self.SQL_FILE_PATTERNS:
            for file_path in repo_path.glob(pattern):
                if file_path.is_file():
                    rel_path = str(file_path.relative_to(repo_path))
                    # Skip if already counted as migration
                    if rel_path not in migration_paths:
                        findings["sql_files"].append(rel_path)
                        await self._extract_tables_from_file(file_path, findings)

    async def _detect_orm_models(self, repo_path: Path, findings: dict) -> None:
        """Detect ORM model files."""
        for pattern in self.ORM_MODEL_PATTERNS:
            for file_path in repo_path.glob(pattern):
                if file_path.is_file():
                    rel_path = str(file_path.relative_to(repo_path))
                    models = await self._extract_orm_models(file_path)
                    if models:
                        findings["orm_models"].append({
                            "file": rel_path,
                            "models": models,
                        })
                        findings["tables"].update(models)

    async def _extract_tables_from_file(self, file_path: Path, findings: dict) -> None:
        """Extract table names from SQL or migration files."""
        try:
            content = file_path.read_text(encoding="utf-8", errors="ignore")

            # Look for CREATE TABLE statements
            for match in self.CREATE_TABLE_PATTERN.finditer(content):
                table_name = match.group(1)
                findings["tables"].add(table_name)

        except (OSError, UnicodeDecodeError):
            pass

    async def _extract_orm_models(self, file_path: Path) -> list[str]:
        """Extract ORM model names from model files."""
        models = []

        try:
            content = file_path.read_text(encoding="utf-8", errors="ignore")
            suffix = file_path.suffix

            if suffix == ".py":
                # SQLAlchemy or Django models
                for match in self.SQLALCHEMY_MODEL_PATTERN.finditer(content):
                    models.append(match.group(1))

            elif suffix == ".go":
                # GORM models
                for match in self.GORM_MODEL_PATTERN.finditer(content):
                    models.append(match.group(1))

            elif suffix in [".ts", ".js"]:
                # TypeORM entities
                for match in self.TYPEORM_ENTITY_PATTERN.finditer(content):
                    models.append(match.group(1))

        except (OSError, UnicodeDecodeError):
            pass

        return models

    def _generate_report(self, findings: dict) -> str:
        """Generate a structured report of database schema findings."""
        lines = []

        # Summary
        lines.append("# Database Schema Discovery Report")
        lines.append("")

        if not findings["tables"] and not findings["migration_files"] and not findings["sql_files"] and not findings["orm_models"]:
            lines.append("No database schemas detected in this repository.")
            return "\n".join(lines)

        # Tables/Entities found
        if findings["tables"]:
            lines.append(f"## Detected Tables/Entities ({len(findings['tables'])})")
            lines.append("")
            for table in sorted(findings["tables"]):
                lines.append(f"- {table}")
            lines.append("")

        # Migration files
        if findings["migration_files"]:
            lines.append(f"## Migration Files ({len(findings['migration_files'])})")
            lines.append("")
            lines.append("Database migrations detected, suggesting schema evolution over time:")
            for mig_file in findings["migration_files"][:10]:  # Limit to first 10
                lines.append(f"- {mig_file}")
            if len(findings["migration_files"]) > 10:
                lines.append(f"- ... and {len(findings['migration_files']) - 10} more")
            lines.append("")

        # SQL files
        if findings["sql_files"]:
            lines.append(f"## SQL Schema Files ({len(findings['sql_files'])})")
            lines.append("")
            for sql_file in findings["sql_files"][:10]:  # Limit to first 10
                lines.append(f"- {sql_file}")
            if len(findings["sql_files"]) > 10:
                lines.append(f"- ... and {len(findings['sql_files']) - 10} more")
            lines.append("")

        # ORM models
        if findings["orm_models"]:
            lines.append(f"## ORM Models ({len(findings['orm_models'])} files)")
            lines.append("")
            lines.append("ORM models detected, suggesting an object-relational mapping approach:")
            for orm_info in findings["orm_models"][:10]:  # Limit to first 10
                lines.append(f"- {orm_info['file']}: {', '.join(orm_info['models'][:5])}")
                if len(orm_info["models"]) > 5:
                    lines.append(f"  (and {len(orm_info['models']) - 5} more models)")
            if len(findings["orm_models"]) > 10:
                lines.append(f"- ... and {len(findings['orm_models']) - 10} more files")
            lines.append("")

        # Inferred database type
        lines.append("## Inferred Information")
        lines.append("")

        if "alembic" in str(findings.get("migration_files", [])):
            lines.append("- Migration framework: Alembic (Python/SQLAlchemy)")
        elif "django" in str(findings.get("migration_files", [])) or any("migrations" in f and f.endswith(".py") for f in findings.get("migration_files", [])):
            lines.append("- Migration framework: Django Migrations")
        elif any(".go" in f for f in findings.get("migration_files", [])):
            lines.append("- Migration framework: Go-based migrations (possibly golang-migrate)")
        elif "flyway" in str(findings.get("migration_files", [])):
            lines.append("- Migration framework: Flyway")
        elif "liquibase" in str(findings.get("migration_files", [])):
            lines.append("- Migration framework: Liquibase")

        if findings["orm_models"]:
            py_models = sum(1 for m in findings["orm_models"] if m["file"].endswith(".py"))
            go_models = sum(1 for m in findings["orm_models"] if m["file"].endswith(".go"))
            ts_models = sum(1 for m in findings["orm_models"] if m["file"].endswith((".ts", ".js")))

            if py_models > 0:
                lines.append("- ORM: Python (likely SQLAlchemy or Django ORM)")
            if go_models > 0:
                lines.append("- ORM: Go (likely GORM)")
            if ts_models > 0:
                lines.append("- ORM: TypeScript/JavaScript (likely TypeORM or Sequelize)")

        return "\n".join(lines)
