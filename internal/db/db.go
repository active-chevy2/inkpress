package db

import (
	"database/sql"
	"log"

	_ "github.com/go-sql-driver/mysql"

	"inkpress/internal/config"
)

func Open(cfg *config.Config) (*sql.DB, error) {
	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	log.Println("database connected")
	return db, nil
}

func Migrate(db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    email       VARCHAR(255) NOT NULL UNIQUE,
    name        VARCHAR(255) NOT NULL,
    password    VARCHAR(255) NOT NULL,
    role        VARCHAR(20)  NOT NULL DEFAULT 'author',
    bio         TEXT,
    avatar_url  VARCHAR(500),
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS invitations (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    code        VARCHAR(64)  NOT NULL UNIQUE,
    email       VARCHAR(255),
    created_by  INT,
    used_by     INT,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    used_at     TIMESTAMP    NULL DEFAULT NULL,
    INDEX idx_invitations_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS sessions (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    token       VARCHAR(255) NOT NULL UNIQUE,
    user_id     INT          NOT NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at  TIMESTAMP    NOT NULL,
    INDEX idx_sessions_token (token)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS posts (
    id           INT AUTO_INCREMENT PRIMARY KEY,
    title        VARCHAR(255) NOT NULL,
    slug         VARCHAR(255) NOT NULL UNIQUE,
    excerpt      TEXT,
    body         LONGTEXT     NOT NULL,
    cover_url    VARCHAR(500),
    status       VARCHAR(20)  NOT NULL DEFAULT 'draft',
    author_id    INT          NOT NULL,
    published_at TIMESTAMP    NULL DEFAULT NULL,
    created_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_posts_status (status),
    INDEX idx_posts_published (published_at),
    INDEX idx_posts_slug (slug)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS tags (
    id    INT AUTO_INCREMENT PRIMARY KEY,
    name  VARCHAR(100) NOT NULL UNIQUE,
    slug  VARCHAR(100) NOT NULL UNIQUE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS post_tags (
    post_id INT NOT NULL,
    tag_id  INT NOT NULL,
    PRIMARY KEY (post_id, tag_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS comments (
    id         INT AUTO_INCREMENT PRIMARY KEY,
    post_id    INT          NOT NULL,
    author     VARCHAR(255) NOT NULL,
    body       TEXT         NOT NULL,
    status     VARCHAR(20)  NOT NULL DEFAULT 'approved',
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_comments_post (post_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS uploads (
    id         INT AUTO_INCREMENT PRIMARY KEY,
    filename   VARCHAR(255) NOT NULL,
    path       VARCHAR(500) NOT NULL,
    size       BIGINT       NOT NULL,
    mime_type  VARCHAR(100),
    uploaded_by INT,
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`
	_, err := db.Exec(schema)
	if err != nil {
		return err
	}
	log.Println("database migrated")
	return nil
}
