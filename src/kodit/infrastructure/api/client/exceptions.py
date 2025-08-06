"""Exceptions for Kodit API client operations."""


class KoditAPIError(Exception):
    """Base exception for Kodit API errors."""



class AuthenticationError(KoditAPIError):
    """Authentication failed with the API server."""



class KoditConnectionError(KoditAPIError):
    """Connection to API server failed."""



class ServerError(KoditAPIError):
    """Server returned an error response."""

