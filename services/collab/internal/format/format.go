package format

import (
	"context"

	"collab/internal/models"
)

// TODO: formatting support requires dedicated tooling in the sandbox service.
// For now, return the source unchanged.
func Format(ctx context.Context, req models.FormatRequest) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	return req.Code, nil
}
