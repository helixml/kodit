// Package api provides the REST API layer for the Kodit service.
package api

import (
	"errors"
	"fmt"
)

// Base API errors as sentinels.
var (
	// ErrAPI is the base error for all API-related errors.
	ErrAPI = errors.New("api error")

	// ErrAuthentication indicates authentication failure.
	ErrAuthentication = errors.New("authentication failed")

	// ErrConnection indicates connection failure to the API server.
	ErrConnection = errors.New("connection failed")

	// ErrServer indicates the server returned an error response.
	ErrServer = errors.New("server error")
)

// APIError represents a structured API error with additional context.
type APIError struct {
	code    int
	message string
	cause   error
}

// NewAPIError creates a new APIError.
func NewAPIError(code int, message string, cause error) *APIError {
	return &APIError{
		code:    code,
		message: message,
		cause:   cause,
	}
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("api error %d: %s: %v", e.code, e.message, e.cause)
	}
	return fmt.Sprintf("api error %d: %s", e.code, e.message)
}

// Unwrap returns the underlying cause.
func (e *APIError) Unwrap() error {
	return e.cause
}

// Code returns the error code.
func (e *APIError) Code() int {
	return e.code
}

// Message returns the error message.
func (e *APIError) Message() string {
	return e.message
}

// AuthenticationError represents an authentication failure.
type AuthenticationError struct {
	message string
}

// NewAuthenticationError creates a new AuthenticationError.
func NewAuthenticationError(message string) *AuthenticationError {
	return &AuthenticationError{message: message}
}

// Error implements the error interface.
func (e *AuthenticationError) Error() string {
	return fmt.Sprintf("authentication failed: %s", e.message)
}

// Unwrap returns the base authentication error for errors.Is compatibility.
func (e *AuthenticationError) Unwrap() error {
	return ErrAuthentication
}

// ConnectionError represents a connection failure.
type ConnectionError struct {
	host  string
	cause error
}

// NewConnectionError creates a new ConnectionError.
func NewConnectionError(host string, cause error) *ConnectionError {
	return &ConnectionError{
		host:  host,
		cause: cause,
	}
}

// Error implements the error interface.
func (e *ConnectionError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("connection to %s failed: %v", e.host, e.cause)
	}
	return fmt.Sprintf("connection to %s failed", e.host)
}

// Unwrap returns the underlying cause.
func (e *ConnectionError) Unwrap() error {
	return errors.Join(ErrConnection, e.cause)
}

// Host returns the host that failed to connect.
func (e *ConnectionError) Host() string {
	return e.host
}

// ServerError represents a server-side error.
type ServerError struct {
	statusCode int
	message    string
}

// NewServerError creates a new ServerError.
func NewServerError(statusCode int, message string) *ServerError {
	return &ServerError{
		statusCode: statusCode,
		message:    message,
	}
}

// Error implements the error interface.
func (e *ServerError) Error() string {
	return fmt.Sprintf("server error %d: %s", e.statusCode, e.message)
}

// Unwrap returns the base server error for errors.Is compatibility.
func (e *ServerError) Unwrap() error {
	return ErrServer
}

// StatusCode returns the HTTP status code.
func (e *ServerError) StatusCode() int {
	return e.statusCode
}

// Message returns the error message.
func (e *ServerError) Message() string {
	return e.message
}
