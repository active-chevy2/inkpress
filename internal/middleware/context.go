package middleware

import (
	"context"

	"inkpress/internal/models"
)

func contextWithUser(ctx context.Context, user *models.User) context.Context {
	return context.WithValue(ctx, UserKey, user)
}
