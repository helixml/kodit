"""Domain service for snippet operations."""

from dataclasses import dataclass

from kodit.domain.entities import Snippet
from kodit.domain.value_objects import Document


@dataclass
class SnippetChangeAnalysis:
    """Result of analyzing changes between existing and new snippets."""

    snippets_to_add: list[Snippet]
    snippet_ids_to_delete: list[int]
    unchanged_snippet_ids: set[int | None]


class SnippetDomainService:
    """Domain service for snippet-related operations."""

    def analyze_snippet_changes(
        self, existing_snippets: list[Snippet], extracted_snippets: list[Snippet]
    ) -> SnippetChangeAnalysis:
        """Analyze changes between existing and extracted snippets.

        Efficiently handles four types of snippet changes:
        1. Deleted: in existing but not in extracted
        2. Added: in extracted but not in existing
        3. Modified: different content hash between existing and extracted
        4. Unchanged: same content hash (no database operation needed)

        Args:
            existing_snippets: List of snippets currently in the database
            extracted_snippets: List of newly extracted snippets

        Returns:
            SnippetChangeAnalysis with categorized changes

        """
        # Create hash-to-existing mapping for O(1) lookup
        existing_by_hash = {
            existing.content_hash: existing for existing in existing_snippets
        }
        all_existing_ids = {
            existing.id for existing in existing_snippets if existing.id
        }

        # Track changes for efficient database operations
        snippets_to_add = []  # New or modified snippets
        snippet_ids_to_delete = []  # Deleted snippet IDs
        matched_existing_ids = set()  # IDs of unchanged snippets

        # Process extracted snippets to identify changes
        for new_snippet in extracted_snippets:
            matching_existing = existing_by_hash.get(new_snippet.content_hash)

            if matching_existing:
                # UNCHANGED: Content hash matches - no database operation needed
                # Just track that this existing snippet is still valid
                matched_existing_ids.add(matching_existing.id)
            else:
                # ADDED or MODIFIED: New content hash - needs to be added
                new_snippet.reset_processing_states()
                snippets_to_add.append(new_snippet)

        # DELETED: Existing snippets that weren't matched
        snippet_ids_to_delete = list(all_existing_ids - matched_existing_ids)

        return SnippetChangeAnalysis(
            snippets_to_add=snippets_to_add,
            snippet_ids_to_delete=snippet_ids_to_delete,
            unchanged_snippet_ids=matched_existing_ids,
        )

    def prepare_documents_for_indexing(self, snippets: list[Snippet]) -> list[Document]:
        """Prepare documents from snippets for search indexing.

        Args:
            snippets: List of snippets to prepare

        Returns:
            List of documents ready for indexing

        """
        return [
            Document(snippet_id=snippet.id, text=snippet.original_text())
            for snippet in snippets
            if snippet.id
        ]

    def prepare_summary_documents(self, snippets: list[Snippet]) -> list[Document]:
        """Prepare documents from snippet summaries for text embedding indexing.

        Args:
            snippets: List of snippets to prepare

        Returns:
            List of documents with summary content ready for text indexing

        """
        documents_with_summaries = []
        for snippet in snippets:
            if snippet.id:
                try:
                    summary_text = snippet.summary_text()
                    if summary_text.strip():  # Only add if summary is not empty
                        documents_with_summaries.append(
                            Document(snippet_id=snippet.id, text=summary_text)
                        )
                except ValueError:
                    # Skip snippets without summary content
                    continue
        return documents_with_summaries
