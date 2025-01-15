package status_http

import "net/http"

// Status ...
type Status struct {
	Code          int    `json:"code"`
	Status        string `json:"status"`
	Description   string `json:"description"`
	CustomMessage string `json:"custom_message"`
}

var (
	OK = Status{
		Code:        http.StatusOK,
		Status:      "OK",
		Description: "The request has succeeded",
	}
	Created = Status{
		Code:        http.StatusCreated,
		Status:      "CREATED",
		Description: "The request has been fulfilled and has resulted in one or more new resources being created",
	}
	NoContent = Status{
		Code:        http.StatusNoContent,
		Status:      "NO_CONTENT",
		Description: "There is no content to send for this request, but the headers may be useful",
	}
	BadEnvironment = Status{
		Code:        http.StatusBadRequest,
		Status:      "BAD_ENVIRONMENT",
		Description: "The service has an invalid environment value",
	}
	BadRequest = Status{
		Code:        http.StatusBadRequest,
		Status:      "BAD_REQUEST",
		Description: "The server could not understand the request due to invalid syntax",
	}
	InvalidArgument = Status{
		Code:        http.StatusBadRequest,
		Status:      "INVALID_ARGUMENT",
		Description: "Invalid argument value passed",
	}
	Unauthorized = Status{
		Code:        http.StatusUnauthorized,
		Status:      "UNAUTHORIZED",
		Description: "...",
	}
	Forbidden = Status{
		Code:        http.StatusForbidden,
		Status:      "FORBIDDEN",
		Description: "...",
	}
	TooManyRequests = Status{
		Code:        http.StatusTooManyRequests,
		Status:      "TOO_MANY_REQUESTS",
		Description: "The user has sent too many requests in a given amount of time",
	}
	InternalServerError = Status{
		Code:        http.StatusInternalServerError,
		Status:      "INTERNAL_SERVER_ERROR",
		Description: "The server encountered an unexpected condition that prevented it from fulfilling the request",
	}
	GRPCError = Status{
		Code:        http.StatusInternalServerError,
		Status:      "GRPC_ERROR",
		Description: "The gRPC request failed",
	}
	NotFound = Status{
		Code:        http.StatusNotFound,
		Status:      "NOT_FOUND",
		Description: "The user not found",
	}
	NotImplemented = Status{
		Code:        http.StatusNotImplemented,
		Status:      "NOT_IMPLEMENTED",
		Description: "Not implemented",
	}

	GrpcStatusToHTTP = map[string]Status{
		"Created":         Created,
		"Ok":              OK,
		"InvalidArgument": InvalidArgument,
		"NotFound":        NotFound,
		"Internal":        InternalServerError,
		"NoContent":       NoContent,
	}
)

// Can be added as many as need like belove examples
// 400	BAD_CONTINUATION_TOKEN	Invalid continuation token passed.
// 400	BAD_PAGE	Page number does not exist or is an invalid format (e.g. negative).
// 400	BAD_REQUEST	The resource you’re creating already exists.
// 400	INVALID_ARGUMENT	Invalid argument value passed.
// 400	INVALID_AUTH	Authentication/OAuth token is invalid.
// 400	INVALID_AUTH_HEADER	Authentication header is invalid.
// 400	INVALID_BATCH	Batched request is missing or invalid.
// 400	INVALID_BODY	A request body that was not in JSON format was passed.
// 400	UNSUPPORTED_OPERATION	Requested operation not supported.
// 401	ACCESS_DENIED	Authentication unsuccessful.
// 401	NO_AUTH	Authentication not provided.
// 403	NOT_AUTHORIZED	User has not been authorized to perform that action.
// 404	NOT_FOUND	Invalid URL.
// 405	METHOD_NOT_ALLOWED	Method is not allowed for this endpoint.
// 409	REQUEST_CONFLICT	Requested operation resulted in conflict.
// 429	HIT_RATE_LIMIT	Hourly rate limit has been reached for this token. Default rate limits are 2,000 calls per hour.
// 500	EXPANSION_FAILED	Unhandled error occurred during expansion; the request is likely to succeed if you don’t ask for expansions, but contact Eventbrite support if this problem persists.
// 500	INTERNAL_ERROR	Unhandled error occurred in Eventbrite. contact Eventbrite support if this problem persists.
