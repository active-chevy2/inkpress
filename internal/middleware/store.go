package middleware

import (
	"database/sql"
	"time"

	"inkpress/internal/models"
)

type DBStore struct {
	DB *sql.DB
}

func (s *DBStore) GetUserBySession(token string) (*models.User, error) {
	return models.UserBySession(s.DB, token)
}

func (s *DBStore) CreateSession(token string, userID int, expires time.Time) error {
	return models.CreateSession(s.DB, token, userID, expires)
}

func (s *DBStore) DeleteSession(token string) error {
	return models.DeleteSession(s.DB, token)
}
