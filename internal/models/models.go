package models

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type User struct {
	ID        int
	Email     string
	Name      string
	Password  string
	Role      string
	Bio       string
	AvatarURL string
	CreatedAt time.Time
}

func (u *User) IsAdmin() bool { return u.Role == "admin" }

type Invitation struct {
	ID        int
	Code      string
	Email     string
	CreatedBy int
	UsedBy    sql.NullInt64
	CreatedAt time.Time
	UsedAt    sql.NullTime
}

type Post struct {
	ID          int
	Title       string
	Slug        string
	Excerpt     string
	Body        string
	CoverURL    string
	Status      string
	AuthorID    int
	PublishedAt sql.NullTime
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Author      *User
	Tags        []Tag
	CommentCount int
}

func (p *Post) IsPublished() bool { return p.Status == "published" }

type Tag struct {
	ID   int
	Name string
	Slug string
}

type Comment struct {
	ID        int
	PostID    int
	Author    string
	Body      string
	Status    string
	CreatedAt time.Time
	PostTitle string
}

type Upload struct {
	ID         int
	Filename   string
	Path       string
	Size       int64
	MimeType   string
	UploadedBy sql.NullInt64
	CreatedAt  time.Time
}

var ErrNotFound = errors.New("not found")

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, "/", "")
	s = strings.ReplaceAll(s, "?", "")
	s = strings.ReplaceAll(s, "#", "")
	s = strings.ReplaceAll(s, "&", "")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

func ensureUniqueSlug(db *sql.DB, base string, excludeID int) string {
	slug := slugify(base)
	if slug == "" {
		slug = "untitled"
	}
	candidate := slug
	for i := 1; i < 1000; i++ {
		var id int
		err := db.QueryRow("SELECT id FROM posts WHERE slug = ? AND id != ?", candidate, excludeID).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return candidate
		}
		if err != nil {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", slug, i)
	}
	return candidate
}

func UserByEmail(db *sql.DB, email string) (*User, error) {
	u := &User{}
	err := db.QueryRow(
		"SELECT id, email, name, password, role, COALESCE(bio,''), COALESCE(avatar_url,''), created_at FROM users WHERE email = ?",
		email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.Password, &u.Role, &u.Bio, &u.AvatarURL, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func UserByID(db *sql.DB, id int) (*User, error) {
	u := &User{}
	err := db.QueryRow(
		"SELECT id, email, name, password, role, COALESCE(bio,''), COALESCE(avatar_url,''), created_at FROM users WHERE id = ?",
		id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.Password, &u.Role, &u.Bio, &u.AvatarURL, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func UserCount(db *sql.DB) (int, error) {
	var c int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&c)
	return c, err
}

func CreateUser(db *sql.DB, email, name, hashedPassword, role string) (int, error) {
	res, err := db.Exec(
		"INSERT INTO users (email, name, password, role) VALUES (?, ?, ?, ?)",
		email, name, hashedPassword, role,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return int(id), err
}

func AllUsers(db *sql.DB) ([]User, error) {
	rows, err := db.Query("SELECT id, email, name, role, COALESCE(bio,''), COALESCE(avatar_url,''), created_at FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.Bio, &u.AvatarURL, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func CreateInvitation(db *sql.DB, code string, createdBy int, email string) error {
	_, err := db.Exec("INSERT INTO invitations (code, created_by, email) VALUES (?, ?, ?)", code, createdBy, email)
	return err
}

func InvitationByCode(db *sql.DB, code string) (*Invitation, error) {
	inv := &Invitation{}
	err := db.QueryRow(
		"SELECT id, code, COALESCE(email,''), created_by, used_by, created_at, used_at FROM invitations WHERE code = ?",
		code,
	).Scan(&inv.ID, &inv.Code, &inv.Email, &inv.CreatedBy, &inv.UsedBy, &inv.CreatedAt, &inv.UsedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if inv.UsedBy.Valid {
		return nil, errors.New("invitation already used")
	}
	return inv, nil
}

func UseInvitation(db *sql.DB, code string, userID int) error {
	_, err := db.Exec("UPDATE invitations SET used_by = ?, used_at = NOW() WHERE code = ? AND used_by IS NULL", userID, code)
	return err
}

func AllInvitations(db *sql.DB) ([]Invitation, error) {
	rows, err := db.Query("SELECT id, code, COALESCE(email,''), created_by, used_by, created_at, used_at FROM invitations ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var invs []Invitation
	for rows.Next() {
		var inv Invitation
		if err := rows.Scan(&inv.ID, &inv.Code, &inv.Email, &inv.CreatedBy, &inv.UsedBy, &inv.CreatedAt, &inv.UsedAt); err != nil {
			return nil, err
		}
		invs = append(invs, inv)
	}
	return invs, nil
}

func CreateSession(db *sql.DB, token string, userID int, expires time.Time) error {
	_, err := db.Exec("INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)", token, userID, expires)
	return err
}

func UserBySession(db *sql.DB, token string) (*User, error) {
	u := &User{}
	err := db.QueryRow(`
		SELECT u.id, u.email, u.name, u.password, u.role, COALESCE(u.bio,''), COALESCE(u.avatar_url,''), u.created_at
		FROM users u
		JOIN sessions s ON s.user_id = u.id
		WHERE s.token = ? AND s.expires_at > NOW()
	`, token).Scan(&u.ID, &u.Email, &u.Name, &u.Password, &u.Role, &u.Bio, &u.AvatarURL, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func DeleteSession(db *sql.DB, token string) error {
	_, err := db.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
}

func PublishedPosts(db *sql.DB, page, perPage int) ([]Post, int, error) {
	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM posts WHERE status = 'published'").Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * perPage
	rows, err := db.Query(`
		SELECT p.id, p.title, p.slug, COALESCE(p.excerpt,''), p.body, COALESCE(p.cover_url,''),
		       p.status, p.author_id, p.published_at, p.created_at, p.updated_at,
		       u.name, COALESCE(u.avatar_url,''), (SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id AND c.status = 'approved')
		FROM posts p
		JOIN users u ON u.id = p.author_id
		WHERE p.status = 'published'
		ORDER BY p.published_at DESC
		LIMIT ? OFFSET ?
	`, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanPosts(rows, total)
}

func PostBySlug(db *sql.DB, slug string) (*Post, error) {
	p := &Post{Author: &User{}}
	err := db.QueryRow(`
		SELECT p.id, p.title, p.slug, COALESCE(p.excerpt,''), p.body, COALESCE(p.cover_url,''),
		       p.status, p.author_id, p.published_at, p.created_at, p.updated_at,
		       u.id, u.name, COALESCE(u.avatar_url,'')
		FROM posts p
		JOIN users u ON u.id = p.author_id
		WHERE p.slug = ?
	`, slug).Scan(&p.ID, &p.Title, &p.Slug, &p.Excerpt, &p.Body, &p.CoverURL,
		&p.Status, &p.AuthorID, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt,
		&p.Author.ID, &p.Author.Name, &p.Author.AvatarURL)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := loadPostTags(db, p); err != nil {
		return nil, err
	}
	return p, nil
}

func PostByID(db *sql.DB, id int) (*Post, error) {
	p := &Post{Author: &User{}}
	err := db.QueryRow(`
		SELECT p.id, p.title, p.slug, COALESCE(p.excerpt,''), p.body, COALESCE(p.cover_url,''),
		       p.status, p.author_id, p.published_at, p.created_at, p.updated_at,
		       u.id, u.name, COALESCE(u.avatar_url,'')
		FROM posts p
		JOIN users u ON u.id = p.author_id
		WHERE p.id = ?
	`, id).Scan(&p.ID, &p.Title, &p.Slug, &p.Excerpt, &p.Body, &p.CoverURL,
		&p.Status, &p.AuthorID, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt,
		&p.Author.ID, &p.Author.Name, &p.Author.AvatarURL)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := loadPostTags(db, p); err != nil {
		return nil, err
	}
	return p, nil
}

func AllPosts(db *sql.DB) ([]Post, error) {
	rows, err := db.Query(`
		SELECT p.id, p.title, p.slug, COALESCE(p.excerpt,''), p.body, COALESCE(p.cover_url,''),
		       p.status, p.author_id, p.published_at, p.created_at, p.updated_at,
		       u.name, COALESCE(u.avatar_url,''), 0
		FROM posts p
		JOIN users u ON u.id = p.author_id
		ORDER BY p.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	posts, _, err := scanPosts(rows, 0)
	if err != nil {
		return nil, err
	}
	for i := range posts {
		_ = loadPostTags(db, &posts[i])
	}
	return posts, nil
}

func PostsByTag(db *sql.DB, tagSlug string, page, perPage int) ([]Post, int, error) {
	var total int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM posts p
		JOIN post_tags pt ON pt.post_id = p.id
		JOIN tags t ON t.id = pt.tag_id
		WHERE p.status = 'published' AND t.slug = ?
	`, tagSlug).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * perPage
	rows, err := db.Query(`
		SELECT p.id, p.title, p.slug, COALESCE(p.excerpt,''), p.body, COALESCE(p.cover_url,''),
		       p.status, p.author_id, p.published_at, p.created_at, p.updated_at,
		       u.name, COALESCE(u.avatar_url,''), (SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id AND c.status = 'approved')
		FROM posts p
		JOIN users u ON u.id = p.author_id
		JOIN post_tags pt ON pt.post_id = p.id
		JOIN tags t ON t.id = pt.tag_id
		WHERE p.status = 'published' AND t.slug = ?
		ORDER BY p.published_at DESC
		LIMIT ? OFFSET ?
	`, tagSlug, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanPosts(rows, total)
}

func CreatePost(db *sql.DB, title, excerpt, body, coverURL, status string, authorID int, publishedAt *time.Time, tagNames []string) (int, error) {
	slug := ensureUniqueSlug(db, title, 0)
	var pubAt interface{}
	if status == "published" {
		if publishedAt != nil {
			pubAt = *publishedAt
		} else {
			pubAt = time.Now()
		}
	} else {
		pubAt = nil
	}
	res, err := db.Exec(`
		INSERT INTO posts (title, slug, excerpt, body, cover_url, status, author_id, published_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, title, slug, excerpt, body, coverURL, status, authorID, pubAt)
	if err != nil {
		return 0, err
	}
	postID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := syncTags(db, int(postID), tagNames); err != nil {
		return 0, err
	}
	return int(postID), nil
}

func UpdatePost(db *sql.DB, id int, title, excerpt, body, coverURL, status string, publishedAt *time.Time, tagNames []string) error {
	slug := ensureUniqueSlug(db, title, id)
	var pubAt interface{}
	if status == "published" {
		if publishedAt != nil {
			pubAt = *publishedAt
		} else {
			row := db.QueryRow("SELECT published_at FROM posts WHERE id = ?", id)
			var existing sql.NullTime
			_ = row.Scan(&existing)
			if existing.Valid {
				pubAt = existing.Time
			} else {
				pubAt = time.Now()
			}
		}
	} else {
		pubAt = nil
	}
	_, err := db.Exec(`
		UPDATE posts SET title=?, slug=?, excerpt=?, body=?, cover_url=?, status=?, published_at=? WHERE id=?
	`, title, slug, excerpt, body, coverURL, status, pubAt, id)
	if err != nil {
		return err
	}
	return syncTags(db, id, tagNames)
}

func DeletePost(db *sql.DB, id int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM post_tags WHERE post_id = ?", id); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.Exec("DELETE FROM comments WHERE post_id = ?", id); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.Exec("DELETE FROM posts WHERE id = ?", id); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func AllTags(db *sql.DB) ([]Tag, error) {
	rows, err := db.Query("SELECT id, name, slug FROM tags ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, nil
}

func TagBySlug(db *sql.DB, slug string) (*Tag, error) {
	t := &Tag{}
	err := db.QueryRow("SELECT id, name, slug FROM tags WHERE slug = ?", slug).Scan(&t.ID, &t.Name, &t.Slug)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

func CommentsByPost(db *sql.DB, postID int) ([]Comment, error) {
	rows, err := db.Query("SELECT id, post_id, author, body, status, created_at FROM comments WHERE post_id = ? AND status = 'approved' ORDER BY created_at", postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.PostID, &c.Author, &c.Body, &c.Status, &c.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, nil
}

func CreateComment(db *sql.DB, postID int, author, body string) error {
	_, err := db.Exec("INSERT INTO comments (post_id, author, body) VALUES (?, ?, ?)", postID, author, body)
	return err
}

func AllComments(db *sql.DB) ([]Comment, error) {
	rows, err := db.Query(`
		SELECT c.id, c.post_id, c.author, c.body, c.status, c.created_at, p.title
		FROM comments c
		JOIN posts p ON p.id = c.post_id
		ORDER BY c.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.PostID, &c.Author, &c.Body, &c.Status, &c.CreatedAt, &c.PostTitle); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, nil
}

func UpdateCommentStatus(db *sql.DB, id int, status string) error {
	_, err := db.Exec("UPDATE comments SET status = ? WHERE id = ?", status, id)
	return err
}

func DeleteComment(db *sql.DB, id int) error {
	_, err := db.Exec("DELETE FROM comments WHERE id = ?", id)
	return err
}

func CreateUpload(db *sql.DB, filename, path string, size int64, mimeType string, uploadedBy int) error {
	_, err := db.Exec("INSERT INTO uploads (filename, path, size, mime_type, uploaded_by) VALUES (?, ?, ?, ?, ?)",
		filename, path, size, mimeType, uploadedBy)
	return err
}

func scanPosts(rows *sql.Rows, total int) ([]Post, int, error) {
	var posts []Post
	for rows.Next() {
		var p Post
		p.Author = &User{}
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Excerpt, &p.Body, &p.CoverURL,
			&p.Status, &p.AuthorID, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt,
			&p.Author.Name, &p.Author.AvatarURL, &p.CommentCount); err != nil {
			return nil, 0, err
		}
		posts = append(posts, p)
	}
	if posts == nil {
		posts = []Post{}
	}
	return posts, total, nil
}

func loadPostTags(db *sql.DB, p *Post) error {
	rows, err := db.Query(`
		SELECT t.id, t.name, t.slug FROM tags t
		JOIN post_tags pt ON pt.tag_id = t.id
		WHERE pt.post_id = ?
		ORDER BY t.name
	`, p.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug); err != nil {
			return err
		}
		p.Tags = append(p.Tags, t)
	}
	return nil
}

func syncTags(db *sql.DB, postID int, tagNames []string) error {
	if _, err := db.Exec("DELETE FROM post_tags WHERE post_id = ?", postID); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, name := range tagNames {
		name = strings.TrimSpace(name)
		if name == "" || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		slug := slugify(name)
		var tagID int
		err := db.QueryRow("SELECT id FROM tags WHERE slug = ?", slug).Scan(&tagID)
		if errors.Is(err, sql.ErrNoRows) {
			res, err := db.Exec("INSERT INTO tags (name, slug) VALUES (?, ?)", name, slug)
			if err != nil {
				return err
			}
			id, err := res.LastInsertId()
			if err != nil {
				return err
			}
			tagID = int(id)
		} else if err != nil {
			return err
		}
		if _, err := db.Exec("INSERT IGNORE INTO post_tags (post_id, tag_id) VALUES (?, ?)", postID, tagID); err != nil {
			return err
		}
	}
	return nil
}
