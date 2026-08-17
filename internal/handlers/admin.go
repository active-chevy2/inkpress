package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"

	"inkpress/internal/config"
	"inkpress/internal/markdown"
	"inkpress/internal/middleware"
	"inkpress/internal/models"
)

type AdminHandler struct {
	DB    *sql.DB
	Cfg   *config.Config
	Rendr *Renderer
	Auth  *middleware.AuthMiddleware
}

func (h *AdminHandler) Login(w http.ResponseWriter, r *http.Request) {
	if middleware.GetUser(r) != nil {
		http.Redirect(w, r, "/admin/dashboard", http.StatusFound)
		return
	}
	userCount, _ := models.UserCount(h.DB)
	data := h.baseData(r)
	data.Title = "Sign In"
	if userCount == 0 {
		data.Content = map[string]interface{}{"FirstUser": true}
	} else {
		data.Content = map[string]interface{}{"FirstUser": false}
	}
	if r.URL.Query().Get("error") == "1" {
		data.FlashError = "Invalid email or password."
	}
	h.Rendr.Render(w, "admin/login", data)
}

func (h *AdminHandler) LoginSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	password := r.FormValue("password")
	user, err := models.UserByEmail(h.DB, email)
	if err != nil {
		http.Redirect(w, r, "/admin/login?error=1", http.StatusFound)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		http.Redirect(w, r, "/admin/login?error=1", http.StatusFound)
		return
	}
	if err := h.Auth.LoginUser(w, user.ID); err != nil {
		http.Redirect(w, r, "/admin/login?error=1", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/admin/dashboard", http.StatusFound)
}

func (h *AdminHandler) Logout(w http.ResponseWriter, r *http.Request) {
	h.Auth.LogoutUser(w, r)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *AdminHandler) Register(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("invite")
	userCount, _ := models.UserCount(h.DB)

	if code == "" {
		if userCount > 0 {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}
		data := h.baseData(r)
		data.Title = "Create Admin Account"
		data.Flash = "Welcome! Create the first account — it will be the admin."
		data.Content = map[string]interface{}{"InviteCode": "", "Email": "", "FirstUser": true}
		h.Rendr.Render(w, "admin/register", data)
		return
	}

	inv, err := models.InvitationByCode(h.DB, code)
	if err != nil {
		data := h.baseData(r)
		data.Title = "Invalid Invitation"
		data.FlashError = "This invitation link is invalid or has already been used."
		data.Content = map[string]interface{}{"InviteCode": "", "Email": "", "FirstUser": false}
		h.Rendr.Render(w, "admin/register", data)
		return
	}
	data := h.baseData(r)
	data.Title = "Create Account"
	data.Content = map[string]interface{}{"InviteCode": inv.Code, "Email": inv.Email, "FirstUser": false}
	h.Rendr.Render(w, "admin/register", data)
}

func (h *AdminHandler) RegisterSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	code := strings.TrimSpace(r.FormValue("invite_code"))
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	name := strings.TrimSpace(r.FormValue("name"))
	password := r.FormValue("password")
	passwordConfirm := r.FormValue("password_confirm")

	if email == "" || name == "" || password == "" {
		http.Redirect(w, r, "/admin/register?invite="+code+"&error=missing", http.StatusFound)
		return
	}
	if password != passwordConfirm {
		http.Redirect(w, r, "/admin/register?invite="+code+"&error=mismatch", http.StatusFound)
		return
	}
	if len(password) < 8 {
		http.Redirect(w, r, "/admin/register?invite="+code+"&error=short", http.StatusFound)
		return
	}

	count, err := models.UserCount(h.DB)
	if err != nil {
		http.Redirect(w, r, "/admin/register?invite="+code+"&error=failed", http.StatusFound)
		return
	}

	if count > 0 {
		inv, err := models.InvitationByCode(h.DB, code)
		if err != nil {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}
		code = inv.Code
	}

	role := "author"
	if count == 0 {
		role = "admin"
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Redirect(w, r, "/admin/register?invite="+code+"&error=failed", http.StatusFound)
		return
	}
	userID, err := models.CreateUser(h.DB, email, name, string(hashed), role)
	if err != nil {
		http.Redirect(w, r, "/admin/register?invite="+code+"&error=exists", http.StatusFound)
		return
	}
	if count > 0 {
		_ = models.UseInvitation(h.DB, code, userID)
	}
	if err := h.Auth.LoginUser(w, userID); err != nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/admin/dashboard", http.StatusFound)
}

func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	posts, err := models.AllPosts(h.DB)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	comments, err := models.AllComments(h.DB)
	if err != nil {
		comments = nil
	}
	pendingCount := 0
	for _, c := range comments {
		if c.Status == "pending" {
			pendingCount++
		}
	}
	publishedCount, draftCount := 0, 0
	for _, p := range posts {
		if p.IsPublished() {
			publishedCount++
		} else {
			draftCount++
		}
	}
	data := h.baseData(r)
	data.Title = "Dashboard"
	data.ActiveNav = "dashboard"
	data.Content = map[string]interface{}{
		"Posts":          h.prepareAdminPosts(posts),
		"PublishedCount": publishedCount,
		"DraftCount":     draftCount,
		"CommentCount":   len(comments),
		"PendingComments": pendingCount,
	}
	h.Rendr.Render(w, "admin/dashboard", data)
}

func (h *AdminHandler) PostsList(w http.ResponseWriter, r *http.Request) {
	posts, err := models.AllPosts(h.DB)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	data := h.baseData(r)
	data.Title = "Posts"
	data.ActiveNav = "posts"
	data.Content = map[string]interface{}{"Posts": h.prepareAdminPosts(posts)}
	h.Rendr.Render(w, "admin/posts", data)
}

func (h *AdminHandler) PostNew(w http.ResponseWriter, r *http.Request) {
	tags, _ := models.AllTags(h.DB)
	data := h.baseData(r)
	data.Title = "New Post"
	data.ActiveNav = "posts"
	data.Content = map[string]interface{}{
		"Post":     map[string]interface{}{"Title": "", "Excerpt": "", "Body": "", "CoverURL": "", "Status": "draft", "Tags": ""},
		"AllTags":  tags,
		"IsEdit":   false,
	}
	h.Rendr.Render(w, "admin/post-edit", data)
}

func (h *AdminHandler) PostEdit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	post, err := models.PostByID(h.DB, id)
	if err == models.ErrNotFound {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	tags, _ := models.AllTags(h.DB)
	tagNames := make([]string, 0, len(post.Tags))
	for _, t := range post.Tags {
		tagNames = append(tagNames, t.Name)
	}
	data := h.baseData(r)
	data.Title = "Edit Post"
	data.ActiveNav = "posts"
	pubDate := ""
	if post.PublishedAt.Valid {
		pubDate = post.PublishedAt.Time.Format("2006-01-02T15:04")
	}
	data.Content = map[string]interface{}{
		"Post": map[string]interface{}{
			"ID":          post.ID,
			"Title":       post.Title,
			"Slug":        post.Slug,
			"Excerpt":     post.Excerpt,
			"Body":        post.Body,
			"CoverURL":    post.CoverURL,
			"Status":      post.Status,
			"Tags":        strings.Join(tagNames, ", "),
			"PublishedAt": pubDate,
		},
		"AllTags": tags,
		"IsEdit":  true,
	}
	h.Rendr.Render(w, "admin/post-edit", data)
}

func (h *AdminHandler) PostSave(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		http.Redirect(w, r, "/admin/posts/new?error=title", http.StatusFound)
		return
	}
	excerpt := strings.TrimSpace(r.FormValue("excerpt"))
	body := r.FormValue("body")
	coverURL := strings.TrimSpace(r.FormValue("cover_url"))
	status := r.FormValue("status")
	if status != "published" && status != "scheduled" && status != "draft" {
		status = "draft"
	}
	tagStr := strings.TrimSpace(r.FormValue("tags"))
	var tagNames []string
	if tagStr != "" {
		for _, t := range strings.Split(tagStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tagNames = append(tagNames, t)
			}
		}
	}
	user := middleware.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/admin/login", http.StatusFound)
		return
	}
	var publishedAt *time.Time
	if pubStr := r.FormValue("published_at"); pubStr != "" {
		if t, err := time.ParseInLocation("2006-01-02T15:04", pubStr, time.Local); err == nil {
			publishedAt = &t
			if status == "published" && t.After(time.Now()) {
				status = "scheduled"
			}
		}
	}
	if status == "scheduled" && publishedAt == nil {
		status = "draft"
	}

	idStr := r.FormValue("id")
	if idStr == "" {
		id, err := models.CreatePost(h.DB, title, excerpt, body, coverURL, status, user.ID, publishedAt, tagNames)
		if err != nil {
			http.Redirect(w, r, "/admin/posts/new?error=failed", http.StatusFound)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/admin/posts/%d", id), http.StatusFound)
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Redirect(w, r, "/admin/posts", http.StatusFound)
		return
	}
	if err := models.UpdatePost(h.DB, id, title, excerpt, body, coverURL, status, publishedAt, tagNames); err != nil {
		http.Redirect(w, r, fmt.Sprintf("/admin/posts/%d?error=failed", id), http.StatusFound)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/posts/%d?saved=1", id), http.StatusFound)
}

func (h *AdminHandler) PostDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := models.DeletePost(h.DB, id); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/posts", http.StatusFound)
}

func (h *AdminHandler) CommentsList(w http.ResponseWriter, r *http.Request) {
	comments, err := models.AllComments(h.DB)
	if err != nil {
		comments = nil
	}
	data := h.baseData(r)
	data.Title = "Comments"
	data.ActiveNav = "comments"
	data.Content = map[string]interface{}{"Comments": comments}
	h.Rendr.Render(w, "admin/comments", data)
}

func (h *AdminHandler) CommentAction(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	action := mux.Vars(r)["action"]
	switch action {
	case "approve":
		_ = models.UpdateCommentStatus(h.DB, id, "approved")
	case "pending":
		_ = models.UpdateCommentStatus(h.DB, id, "pending")
	case "spam":
		_ = models.UpdateCommentStatus(h.DB, id, "spam")
	case "delete":
		_ = models.DeleteComment(h.DB, id)
	}
	http.Redirect(w, r, "/admin/comments", http.StatusFound)
}

func (h *AdminHandler) InvitationsList(w http.ResponseWriter, r *http.Request) {
	invs, err := models.AllInvitations(h.DB)
	if err != nil {
		invs = nil
	}
	users, _ := models.AllUsers(h.DB)
	data := h.baseData(r)
	data.Title = "Invitations"
	data.ActiveNav = "invitations"
	if r.URL.Query().Get("created") == "1" {
		data.Flash = "Invitation link created successfully."
	}
	data.Content = map[string]interface{}{"Invitations": invs, "Users": users}
	h.Rendr.Render(w, "admin/invitations", data)
}

func (h *AdminHandler) InvitationCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	user := middleware.GetUser(r)
	if user == nil || !user.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	code := generateInviteCode()
	if err := models.CreateInvitation(h.DB, code, user.ID, email); err != nil {
		http.Redirect(w, r, "/admin/invitations?error=failed", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/admin/invitations?created=1", http.StatusFound)
}

func (h *AdminHandler) UploadsList(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query("SELECT id, filename, path, size, COALESCE(mime_type,''), uploaded_by, created_at FROM uploads ORDER BY id DESC")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var uploads []models.Upload
	for rows.Next() {
		var u models.Upload
		if err := rows.Scan(&u.ID, &u.Filename, &u.Path, &u.Size, &u.MimeType, &u.UploadedBy, &u.CreatedAt); err != nil {
			continue
		}
		uploads = append(uploads, u)
	}
	if uploads == nil {
		uploads = []models.Upload{}
	}
	data := h.baseData(r)
	data.Title = "Uploads"
	data.ActiveNav = "uploads"
	data.Content = map[string]interface{}{"Uploads": uploads, "UploadPath": h.Cfg.BaseURL + "/uploads/"}
	h.Rendr.Render(w, "admin/uploads", data)
}

func (h *AdminHandler) UploadCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	_ = r.ParseMultipartForm(10 << 20)
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Redirect(w, r, "/admin/uploads?error=nofile", http.StatusFound)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".svg": true, ".pdf": true}
	if !allowed[ext] {
		http.Redirect(w, r, "/admin/uploads?error=type", http.StatusFound)
		return
	}
	if header.Size > 10<<20 {
		http.Redirect(w, r, "/admin/uploads?error=size", http.StatusFound)
		return
	}

	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	destPath := filepath.Join(h.Cfg.UploadsDir, filename)
	if err := os.MkdirAll(h.Cfg.UploadsDir, 0755); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	dst, err := os.Create(destPath)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	size, err := io.Copy(dst, file)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	user := middleware.GetUser(r)
	uploadedBy := 0
	if user != nil {
		uploadedBy = user.ID
	}
	_ = models.CreateUpload(h.DB, header.Filename, "/uploads/"+filename, size, header.Header.Get("Content-Type"), uploadedBy)
	http.Redirect(w, r, "/admin/uploads?uploaded=1", http.StatusFound)
}

func (h *AdminHandler) baseData(r *http.Request) PageData {
	user := middleware.GetUser(r)
	return PageData{
		SiteName: "InkPress",
		BaseURL:  h.Cfg.BaseURL,
		User:     user,
	}
}

func (h *AdminHandler) prepareAdminPosts(posts []models.Post) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(posts))
	for _, p := range posts {
		pubDate := ""
		if p.PublishedAt.Valid {
			pubDate = p.PublishedAt.Time.Format("Jan 2, 2006")
		}
		excerpt := p.Excerpt
		if excerpt == "" {
			excerpt = markdown.Excerpt(p.Body, 100)
		}
		result = append(result, map[string]interface{}{
			"ID":           p.ID,
			"Title":        p.Title,
			"Slug":         p.Slug,
			"Excerpt":      excerpt,
			"Status":       p.Status,
			"AuthorName":   p.Author.Name,
			"PublishedDate": pubDate,
			"UpdatedDate":  p.UpdatedAt.Format("Jan 2, 2006"),
			"Tags":         p.Tags,
		})
	}
	return result
}

func generateInviteCode() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
