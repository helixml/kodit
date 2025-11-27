"""Simple keyword extraction service for query processing."""

import re

# Common English stop words - a minimal set for simple NLP
STOP_WORDS = frozenset([
    "a", "an", "and", "are", "as", "at", "be", "by", "for", "from",
    "has", "he", "in", "is", "it", "its", "of", "on", "or", "that",
    "the", "to", "was", "were", "will", "with", "what", "which",
    "who", "whom", "this", "these", "those", "am", "been", "being",
    "have", "had", "having", "do", "does", "did", "doing", "would",
    "should", "could", "ought", "i", "me", "my", "myself", "we", "our",
    "ours", "ourselves", "you", "your", "yours", "yourself", "yourselves",
    "she", "her", "hers", "herself", "him", "his", "himself", "they",
    "them", "their", "theirs", "themselves", "can", "cannot", "but",
    "if", "then", "else", "when", "where", "why", "how", "all", "each",
    "every", "both", "few", "more", "most", "other", "some", "such", "no",
    "nor", "not", "only", "own", "same", "so", "than", "too", "very",
    "just", "also", "now", "here", "there", "into", "through", "during",
    "before", "after", "above", "below", "between", "under", "again",
    "further", "once", "any", "about", "out", "over", "up", "down",
    "off", "while", "because", "until", "although", "unless", "since",
    "however", "therefore", "thus", "hence", "yet", "still", "already",
    "always", "never", "often", "sometimes", "usually", "want", "need",
    "find", "show", "tell", "get", "give", "make", "use", "look", "see",
    "go", "come", "take", "know", "think", "let", "put", "say", "may",
    "might", "must", "shall",
])


def extract_keywords(query: str) -> list[str]:
    """Extract meaningful keywords from a natural language query.

    Removes stop words and common filler words to extract the core
    terms that should be used for keyword search.
    """
    # Convert to lowercase and split on non-alphanumeric characters
    words = re.split(r"[^a-zA-Z0-9_]+", query.lower())

    # Filter out stop words and short words
    keywords = [
        word for word in words
        if word and len(word) > 1 and word not in STOP_WORDS
    ]

    # Remove duplicates while preserving order
    seen: set[str] = set()
    unique_keywords = []
    for keyword in keywords:
        if keyword not in seen:
            seen.add(keyword)
            unique_keywords.append(keyword)

    return unique_keywords
