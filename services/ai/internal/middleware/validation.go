package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"

	"peerprep/ai/internal/models"
	"peerprep/ai/internal/utils"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const validatedRequestKey contextKey = "validated_request"

// request models implement this interface
type Validator interface {
	Validate() error
}

/*
tldr
- reads the JSON body of a request
- deserializes it into a Go struct (specific to that route)
- validates it using the struct's own Validate() method
- stores the validated struct in the request context
- passes control to your actual handler (which can safely assume the request is valid)
*/

// validates JSON requests using generics
func ValidateRequest[T Validator]() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create a new instance of the request type
			var req T
			reqType := reflect.TypeOf(req)
			if reqType.Kind() == reflect.Ptr {
				req = reflect.New(reqType.Elem()).Interface().(T)
			} else {
				req = reflect.New(reqType).Interface().(T)
			}

			// decoding JSON request body
			if err := json.NewDecoder(r.Body).Decode(req); err != nil {
				utils.JSON(w, http.StatusBadRequest, models.ErrorResponse{
					Code:    "invalid_json",
					Message: "Invalid JSON in request body",
				})
				return
			}

			// validation
			if err := req.Validate(); err != nil {
				// error is already an ErrorResponse, we use it directly
				if errResp, ok := err.(*models.ErrorResponse); ok {
					utils.JSON(w, http.StatusBadRequest, *errResp)
				} else {
					utils.JSON(w, http.StatusBadRequest, models.ErrorResponse{
						Code:    "validation_error",
						Message: err.Error(),
					})
				}
				return
			}

			// store validated request in context
			ctx := context.WithValue(r.Context(), validatedRequestKey, req)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetValidatedRequest retrieves the validated request from context
func GetValidatedRequest[T any](r *http.Request) T {
	return r.Context().Value(validatedRequestKey).(T)
}
