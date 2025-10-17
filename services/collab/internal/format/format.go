package format

import (
	"context"

	"collab/internal/models"
)

// TODO: formatting support requires dedicated tooling in the sandbox service.
// For now, return the source unchanged.
func Format(_ context.Context, req models.FormatRequest) (string, error) {
	return req.Code, nil
}
