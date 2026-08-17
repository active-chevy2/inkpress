package handlers

import (
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Renderer struct {
	tmpls *template.Template
}

func NewRenderer(tmplPath string) (*Renderer, error) {
	funcMap := template.FuncMap{
		"formatDate":      formatDate,
		"formatDateTime":  formatDateTime,
		"truncate":        truncate,
		"join":            strings.Join,
		"hasPrefix":       strings.HasPrefix,
		"safeHTML":        func(s string) template.HTML { return template.HTML(s) },
		"safeURL":         func(s string) template.URL { return template.URL(s) },
		"currentYear":     func() int { return time.Now().Year() },
		"dict":            dict,
		"divf":            divf,
	}
	tmpls := template.New("").Funcs(funcMap)
	err := filepath.Walk(tmplPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		rel, err := filepath.Rel(tmplPath, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		name := strings.TrimSuffix(rel, ".html")
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = tmpls.New(name).Parse(string(content))
		return err
	})
	if err != nil {
		return nil, err
	}
	return &Renderer{tmpls: tmpls}, nil
}

func (r *Renderer) Render(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := r.tmpls.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

type PageData struct {
	Title       string
	Description string
	SiteName    string
	BaseURL     string
	User        interface{}
	ActiveNav   string
	Content     interface{}
	Flash       string
	FlashError  string
	CSRFToken   string // CSRF protection token
}

func formatDate(t time.Time) string {
	return t.Format("January 2, 2006")
}

func formatDateTime(t time.Time) string {
	return t.Format("January 2, 2006 at 3:04 PM")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func divf(a int64, b int64) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func dict(values ...interface{}) map[string]interface{} {
	m := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			continue
		}
		if i+1 < len(values) {
			m[key] = values[i+1]
		}
	}
	return m
}
