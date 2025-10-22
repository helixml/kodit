"""Base query for SQLAlchemy repositories."""

from abc import ABC, abstractmethod
from dataclasses import dataclass
from enum import Enum
from typing import Any

from sqlalchemy import Select


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

    def apply(self, model_type: type, stmt: Select) -> Select:
        """Apply filter to statement."""
        column = getattr(model_type, self.field)

        # Use column comparison methods instead of operators module
        condition = None
        match self.operator:
            case FilterOperator.EQ:
                condition = column == self.value
            case FilterOperator.NE:
                condition = column != self.value
            case FilterOperator.GT:
                condition = column > self.value
            case FilterOperator.GTE:
                condition = column >= self.value
            case FilterOperator.LT:
                condition = column < self.value
            case FilterOperator.LTE:
                condition = column <= self.value
            case FilterOperator.IN:
                condition = column.in_(self.value)
            case FilterOperator.LIKE:
                condition = column.like(self.value)
            case FilterOperator.ILIKE:
                condition = column.ilike(self.value)

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

    def __init__(self) -> None:
        """Initialize query builder."""
        self._filters: list[FilterCriteria] = []
        self._sorts: list[SortCriteria] = []
        self._pagination: PaginationCriteria | None = None

    def filter(
        self, field: str, operator: FilterOperator, value: Any
    ) -> "QueryBuilder":
        """Add a filter criterion."""
        self._filters.append(FilterCriteria(field, operator, value))
        return self

    def sort(self, field: str, *, descending: bool = False) -> "QueryBuilder":
        """Add a sort criterion."""
        self._sorts.append(SortCriteria(field, descending))
        return self

    def paginate(self, limit: int | None = None, *, offset: int = 0) -> "QueryBuilder":
        """Add pagination."""
        self._pagination = PaginationCriteria(limit, offset)
        return self

    def apply(self, stmt: Select, model_type: type) -> Select:
        """Apply all criteria to the statement."""
        for filter_criteria in self._filters:
            stmt = filter_criteria.apply(model_type, stmt)

        for sort_criteria in self._sorts:
            stmt = sort_criteria.apply(model_type, stmt)

        if self._pagination:
            stmt = self._pagination.apply(stmt)

        return stmt
