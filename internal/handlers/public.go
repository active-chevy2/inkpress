package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/feeds"
	"github.com/gorilla/mux"

	"inkpress/internal/config"
	"inkpress/internal/markdown"
	"inkpress/internal/middleware"
	"inkpress/internal/models"
)

type PublicHandler struct {
	DB    *sql.DB
	Cfg   *config.Config
	Rendr *Renderer
	Auth  *middleware.AuthMiddleware
}

func (h *PublicHandler) Home(w http.ResponseWriter, r *http.Request) {
	page := getPage(r)
	posts, total, err := models.PublishedPosts(h.DB, page, 9)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	totalPages := (total + 9 - 1) / 9
	data := h.baseData(w, r)
	data.Title = h.Cfg.BaseURL
	data.Content = map[string]interface{}{
		"Posts":      h.preparePosts(posts),
		"Page":       page,
		"TotalPages": totalPages,
		"HasPrev":    page > 1,
		"HasNext":    page < totalPages,
		"PrevPage":   page - 1,
		"NextPage":   page + 1,
	}
	h.Rendr.Render(w, "public/home", data)
}

func (h *PublicHandler) Post(w http.ResponseWriter, r *http.Request) {
	slug := mux.Vars(r)["slug"]
	post, err := models.PostBySlug(h.DB, slug)
	if err == models.ErrNotFound {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !post.IsPublished() {
		if user := middleware.GetUser(r); user == nil || !user.IsAdmin() {
			http.NotFound(w, r)
			return
		}
	}
	comments, err := models.CommentsByPost(h.DB, post.ID)
	if err != nil {
		comments = nil
	}
	data := h.baseData(w, r)
	data.Title = post.Title
	data.Description = post.Excerpt
	data.Content = map[string]interface{}{
		"Post":         h.preparePost(post),
		"HTMLBody":     markdown.Render(post.Body),
		"Comments":     comments,
		"CommentCount": len(comments),
	}
	h.Rendr.Render(w, "public/post", data)
}

func (h *PublicHandler) Tag(w http.ResponseWriter, r *http.Request) {
	slug := mux.Vars(r)["slug"]
	tag, err := models.TagBySlug(h.DB, slug)
	if err == models.ErrNotFound {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	page := getPage(r)
	posts, total, err := models.PostsByTag(h.DB, slug, page, 9)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	totalPages := (total + 9 - 1) / 9
	data := h.baseData(w, r)
	data.Title = tag.Name
	data.Content = map[string]interface{}{
		"Tag":        tag,
		"Posts":      h.preparePosts(posts),
		"Page":       page,
		"TotalPages": totalPages,
		"HasPrev":    page > 1,
		"HasNext":    page < totalPages,
		"PrevPage":   page - 1,
		"NextPage":   page + 1,
	}
	h.Rendr.Render(w, "public/tag", data)
}

func (h *PublicHandler) CommentSubmit(w http.ResponseWriter, r *http.Request) {
	if !h.Auth.ValidateCSRFToken(r) {
		http.Redirect(w, r, "/"+mux.Vars(r)["slug"]+"?error=csrf", http.StatusFound)
		return
	}
	slug := mux.Vars(r)["slug"]
	post, err := models.PostBySlug(h.DB, slug)
	if err == models.ErrNotFound {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !post.IsPublished() {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	author := strings.TrimSpace(r.FormValue("author"))
	body := strings.TrimSpace(r.FormValue("body"))
	if author == "" || body == "" {
		http.Redirect(w, r, "/"+post.Slug+"?error=missing", http.StatusFound)
		return
	}
	if len(body) > 5000 {
		http.Redirect(w, r, "/"+post.Slug+"?error=too_long", http.StatusFound)
		return
	}
	if err := models.CreateComment(h.DB, post.ID, author, body); err != nil {
		http.Redirect(w, r, "/"+post.Slug+"?error=failed", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/"+post.Slug+"?success=1", http.StatusFound)
}

func (h *PublicHandler) RSS(w http.ResponseWriter, r *http.Request) {
	posts, _, err := models.PublishedPosts(h.DB, 1, 20)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	feed := &feeds.Feed{
		Title:       "InkPress",
		Link:        &feeds.Link{Href: h.Cfg.BaseURL},
		Description: "Latest posts",
		Created:     time.Now(),
	}
	for _, p := range posts {
		if !p.PublishedAt.Valid {
			continue
		}
		feed.Items = append(feed.Items, &feeds.Item{
			Title:       p.Title,
			Link:        &feeds.Link{Href: h.Cfg.BaseURL + "/" + p.Slug},
			Description: p.Excerpt,
			Content:     markdown.Render(p.Body),
			Created:     p.PublishedAt.Time,
			Id:          p.Slug,
		})
	}
	rss, err := feed.ToRss()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Write([]byte(rss))
}

func (h *PublicHandler) NotFound(w http.ResponseWriter, r *http.Request) {
	data := h.baseData(w, r)
	data.Title = "Not Found"
	w.WriteHeader(http.StatusNotFound)
	h.Rendr.Render(w, "public/404", data)
}

func (h *PublicHandler) baseData(w http.ResponseWriter, r *http.Request) PageData {
	user := middleware.GetUser(r)
	return PageData{
		SiteName:  "InkPress",
		BaseURL:   h.Cfg.BaseURL,
		User:      user,
		ActiveNav: "",
		CSRFToken: h.Auth.GenerateCSRFToken(w, r),
	}
}

func (h *PublicHandler) preparePost(p *models.Post) map[string]interface{} {
	excerpt := p.Excerpt
	if excerpt == "" {
		excerpt = markdown.Excerpt(p.Body, 200)
	}
	pubDate := ""
	if p.PublishedAt.Valid {
		pubDate = p.PublishedAt.Time.Format("January 2, 2006")
	}
	return map[string]interface{}{
		"ID":            p.ID,
		"Title":         p.Title,
		"Slug":          p.Slug,
		"Excerpt":       excerpt,
		"CoverURL":      p.CoverURL,
		"Status":        p.Status,
		"AuthorName":    p.Author.Name,
		"AuthorAvatar":  p.Author.AvatarURL,
		"PublishedDate": pubDate,
		"Tags":          p.Tags,
		"CommentCount":  p.CommentCount,
	}
}

func (h *PublicHandler) preparePosts(posts []models.Post) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(posts))
	for _, p := range posts {
		excerpt := p.Excerpt
		if excerpt == "" {
			excerpt = markdown.Excerpt(p.Body, 160)
		}
		pubDate := ""
		if p.PublishedAt.Valid {
			pubDate = p.PublishedAt.Time.Format("January 2, 2006")
		}
		result = append(result, map[string]interface{}{
			"ID":            p.ID,
			"Title":         p.Title,
			"Slug":          p.Slug,
			"Excerpt":       excerpt,
			"CoverURL":      p.CoverURL,
			"AuthorName":    p.Author.Name,
			"AuthorAvatar":  p.Author.AvatarURL,
			"PublishedDate": pubDate,
			"Tags":          p.Tags,
			"CommentCount":  p.CommentCount,
		})
	}
	return result
}

func getPage(r *http.Request) int {
	p := r.URL.Query().Get("page")
	if p == "" {
		return 1
	}
	n, err := strconv.Atoi(p)
	if err != nil || n < 1 {
		return 1
	}
	return n
}
