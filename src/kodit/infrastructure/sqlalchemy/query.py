"""Base query for SQLAlchemy repositories."""

from abc import ABC, abstractmethod
from dataclasses import dataclass
from enum import Enum
from typing import Any, Self

from sqlalchemy import Select

from kodit.domain.enrichments.enrichment import EnrichmentV2
from kodit.infrastructure.api.v1.query_params import PaginationParams
from kodit.infrastructure.sqlalchemy import entities as db_entities


class Query(ABC):
    """Base query/specification object for encapsulating query logic."""

    @abstractmethod
    def apply(self, stmt: Select, model_type: type) -> Select:
        """Apply this query's criteria to a SQLAlchemy Select statement."""


class FilterOperator(Enum):
    """SQL filter operators."""

    EQ = "eq"
    NE = "ne"
    GT = "gt"
    GTE = "ge"
    LT = "lt"
    LTE = "le"
    IN = "in_"
    LIKE = "like"
    ILIKE = "ilike"


@dataclass
class FilterCriteria:
    """Filter criteria for a query."""

    field: str
    operator: FilterOperator
    value: Any

    def apply(self, model_type: type, stmt: Select) -> Select:  # noqa: C901
        """Apply filter to statement."""
        column = getattr(model_type, self.field)

        # Convert AnyUrl to string for SQLAlchemy comparison
        value = self.value
        if hasattr(value, "__str__") and type(value).__module__ == "pydantic.networks":
            value = str(value)

        # Use column comparison methods instead of operators module
        condition = None
        match self.operator:
            case FilterOperator.EQ:
                condition = column == value
            case FilterOperator.NE:
                condition = column != value
            case FilterOperator.GT:
                condition = column > value
            case FilterOperator.GTE:
                condition = column >= value
            case FilterOperator.LT:
                condition = column < value
            case FilterOperator.LTE:
                condition = column <= value
            case FilterOperator.IN:
                condition = column.in_(value)
            case FilterOperator.LIKE:
                condition = column.like(value)
            case FilterOperator.ILIKE:
                condition = column.ilike(value)

        return stmt.where(condition)


@dataclass
class SortCriteria:
    """Sort criteria for a query."""

    field: str
    descending: bool = False

    def apply(self, model_type: type, stmt: Select) -> Select:
        """Apply sort to statement."""
        column = getattr(model_type, self.field)
        return stmt.order_by(column.desc() if self.descending else column.asc())


@dataclass
class PaginationCriteria:
    """Pagination criteria for a query."""

    limit: int | None = None
    offset: int = 0

    def apply(self, stmt: Select) -> Select:
        """Apply pagination to statement."""
        stmt = stmt.offset(self.offset)
        if self.limit is not None:
            stmt = stmt.limit(self.limit)
        return stmt


class QueryBuilder(Query):
    """Composable query builder for constructing database queries."""

    DEFAULT_SORT_FIELD = "created_at"
    DEFAULT_SORT_DESCENDING = True

    def __init__(self) -> None:
        """Initialize query builder."""
        self._filters: list[FilterCriteria] = []
        self._sorts: list[SortCriteria] = []
        self._pagination: PaginationCriteria | None = None

    def filter(self, field: str, operator: FilterOperator, value: Any) -> Self:
        """Add a filter criterion."""
        self._filters.append(FilterCriteria(field, operator, value))
        return self

    def sort(self, field: str, *, descending: bool = False) -> Self:
        """Add a sort criterion."""
        self._sorts.append(SortCriteria(field, descending))
        return self

    def paginate(self, pagination: PaginationParams) -> Self:
        """Add pagination."""
        self._pagination = PaginationCriteria(
            limit=pagination.limit, offset=pagination.offset
        )
        return self

    def apply_filters_only(self, stmt: Select, model_type: type) -> Select:
        """Apply only filter criteria to the statement."""
        for filter_criteria in self._filters:
            stmt = filter_criteria.apply(model_type, stmt)
        return stmt

    def apply(self, stmt: Select, model_type: type) -> Select:
        """Apply all criteria to the statement."""
        for filter_criteria in self._filters:
            stmt = filter_criteria.apply(model_type, stmt)

        if not self._sorts:
            self._sorts = [
                SortCriteria(
                    field=self.DEFAULT_SORT_FIELD,
                    descending=self.DEFAULT_SORT_DESCENDING,
                )
            ]
        for sort_criteria in self._sorts:
            stmt = sort_criteria.apply(model_type, stmt)

        if self._pagination:
            stmt = self._pagination.apply(stmt)

        return stmt

    def get_large_in_filters(self, chunk_size: int) -> list[FilterCriteria]:
        """Get filters that have IN clauses larger than chunk_size."""
        return [
            f
            for f in self._filters
            if f.operator == FilterOperator.IN
            and isinstance(f.value, list)
            and len(f.value) > chunk_size
        ]

    def with_replaced_filter(
        self, old_filter: FilterCriteria, new_filter: FilterCriteria
    ) -> Self:
        """Create a copy of this query with a filter replaced."""
        new_query = self.__class__()
        # Copy internal state - accessing private members is intentional for cloning
        new_query._filters = [  # noqa: SLF001
            new_filter if f == old_filter else f for f in self._filters
        ]
        new_query._sorts = self._sorts.copy()  # noqa: SLF001
        new_query._pagination = self._pagination  # noqa: SLF001
        return new_query  # type: ignore[return-value]


class EnrichmentAssociationQueryBuilder(QueryBuilder):
    """Query builder for enrichment association entities."""

    def for_enrichment_ids(self, enrichment_ids: list[int]) -> Self:
        """Build a query for associations by enrichment IDs."""
        self.filter(
            db_entities.EnrichmentAssociation.enrichment_id.key,
            FilterOperator.IN,
            enrichment_ids,
        )
        return self

    def for_enrichments(self, enrichments: list[EnrichmentV2]) -> Self:
        """Build a query for enrichment associations by entity IDs."""
        self.filter(
            db_entities.EnrichmentAssociation.enrichment_id.key,
            FilterOperator.IN,
            [enrichment.id for enrichment in enrichments if enrichment.id is not None],
        )
        return self

    def for_enrichment_type(self) -> Self:
        """Build a query for enrichment types."""
        return self.for_entity_type(db_entities.EnrichmentV2.__tablename__)

    def for_entity_type(self, entity_type: str) -> Self:
        """Build a query for enrichment associations by entity type."""
        self.filter(
            db_entities.EnrichmentAssociation.entity_type.key,
            FilterOperator.EQ,
            entity_type,
        )
        return self

    def for_entity_ids(self, entity_ids: list[str]) -> Self:
        """Build a query for enrichment associations by entity IDs."""
        self.filter(
            db_entities.EnrichmentAssociation.entity_id.key,
            FilterOperator.IN,
            entity_ids,
        )
        return self

    @staticmethod
    def for_enrichment_association(
        entity_type: str,
        entity_id: str,
    ) -> QueryBuilder:
        """Build a query for a specific enrichment association."""
        return EnrichmentAssociationQueryBuilder.for_enrichment_associations(
            entity_type,
            [entity_id],
        )

    @staticmethod
    def for_enrichment_associations(
        entity_type: str, entity_ids: list[str]
    ) -> QueryBuilder:
        """Build a query for enrichment associations by entity type and IDs."""
        return (
            QueryBuilder()
            .filter(
                db_entities.EnrichmentAssociation.entity_type.key,
                FilterOperator.EQ,
                entity_type,
            )
            .filter(
                db_entities.EnrichmentAssociation.entity_id.key,
                FilterOperator.IN,
                entity_ids,
            )
        )

    @staticmethod
    def type_and_ids(
        entity_type: str,
        enrichment_ids: list[int],
    ) -> QueryBuilder:
        """Build a query for enrichment associations by enrichment IDs."""
        return (
            QueryBuilder()
            .filter(
                db_entities.EnrichmentAssociation.entity_type.key,
                FilterOperator.EQ,
                entity_type,
            )
            .filter(
                db_entities.EnrichmentAssociation.enrichment_id.key,
                FilterOperator.IN,
                enrichment_ids,
            )
        )

    @staticmethod
    def associations_pointing_to_these_enrichments(
        enrichment_ids: list[int],
    ) -> QueryBuilder:
        """Build a query for enrichment associations pointing to these enrichments."""
        return EnrichmentAssociationQueryBuilder.type_and_ids(
            entity_type=db_entities.EnrichmentV2.__tablename__,
            enrichment_ids=enrichment_ids,
        )

    def for_commit(self, commit_sha: str) -> Self:
        """Build a query for enrichment associations for a commit."""
        self.filter(
            db_entities.EnrichmentAssociation.entity_type.key,
            FilterOperator.EQ,
            db_entities.GitCommit.__tablename__,
        )
        self.filter(
            db_entities.EnrichmentAssociation.entity_id.key,
            FilterOperator.EQ,
            commit_sha,
        )
        return self


class EnrichmentQueryBuilder(QueryBuilder):
    """Query builder for enrichment entities."""

    def for_ids(self, enrichment_ids: list[int]) -> Self:
        """Build a query for enrichments by their IDs."""
        self.filter(
            db_entities.EnrichmentV2.id.key,
            FilterOperator.IN,
            enrichment_ids,
        )
        return self

    def for_type(self, enrichment_type: str) -> Self:
        """Build a query for enrichments by their type."""
        self.filter(
            db_entities.EnrichmentV2.type.key,
            FilterOperator.EQ,
            enrichment_type,
        )
        return self

    def for_subtype(self, enrichment_subtype: str) -> Self:
        """Build a query for enrichments by their subtype."""
        self.filter(
            db_entities.EnrichmentV2.subtype.key,
            FilterOperator.EQ,
            enrichment_subtype,
        )
        return self


class GitFileQueryBuilder(QueryBuilder):
    """Query builder for git file entities."""

    def for_commit_sha(self, commit_sha: str) -> Self:
        """Build a query for git files by their commit SHA."""
        self.filter(
            db_entities.GitCommitFile.commit_sha.key,
            FilterOperator.EQ,
            commit_sha,
        )
        return self

    def for_blob_sha(self, blob_sha: str) -> Self:
        """Build a query for git files by their blob SHA."""
        self.filter(
            db_entities.GitCommitFile.blob_sha.key,
            FilterOperator.EQ,
            blob_sha,
        )
        return self
