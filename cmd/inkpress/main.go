package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	"inkpress/internal/config"
	"inkpress/internal/db"
	"inkpress/internal/handlers"
	"inkpress/internal/middleware"
)

func main() {
	cfg := config.Load()

	database, err := db.Open(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	renderer, err := handlers.NewRenderer("web/templates")
	if err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}

	store := &middleware.DBStore{DB: database}
	authMW := middleware.NewAuthMiddleware(cfg, store)

	publicH := &handlers.PublicHandler{DB: database, Cfg: cfg, Rendr: renderer, Auth: authMW}
	adminH := &handlers.AdminHandler{DB: database, Cfg: cfg, Rendr: renderer, Auth: authMW}

	r := mux.NewRouter()

	r.HandleFunc("/rss.xml", publicH.RSS).Methods("GET")
	r.HandleFunc("/feed", publicH.RSS).Methods("GET")

	r.HandleFunc("/tag/{slug}", publicH.Tag).Methods("GET")
	r.HandleFunc("/page/{page}", publicH.Home).Methods("GET")

	r.HandleFunc("/{slug}/comment", publicH.CommentSubmit).Methods("POST")
	r.HandleFunc("/{slug}", publicH.Post).Methods("GET")

	r.HandleFunc("/", publicH.Home).Methods("GET")

	staticDir, _ := filepath.Abs("web/static")
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	uploadsDir, _ := filepath.Abs(cfg.UploadsDir)
	r.PathPrefix("/uploads/").Handler(http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadsDir))))

	r.HandleFunc("/admin/login", adminH.Login).Methods("GET")
	r.HandleFunc("/admin/login", adminH.LoginSubmit).Methods("POST")
	r.HandleFunc("/admin/logout", adminH.Logout).Methods("GET", "POST")
	r.HandleFunc("/admin/register", adminH.Register).Methods("GET")
	r.HandleFunc("/admin/register", adminH.RegisterSubmit).Methods("POST")

	admin := r.PathPrefix("/admin").Subrouter()
	admin.Use(authMW.RequireAuth)
	admin.HandleFunc("/dashboard", adminH.Dashboard).Methods("GET")
	admin.HandleFunc("/posts", adminH.PostsList).Methods("GET")
	admin.HandleFunc("/posts/new", adminH.PostNew).Methods("GET")
	admin.HandleFunc("/posts/{id}", adminH.PostEdit).Methods("GET")
	admin.HandleFunc("/posts/{id}/delete", adminH.PostDelete).Methods("POST")
	admin.HandleFunc("/posts/save", adminH.PostSave).Methods("POST")

	admin.HandleFunc("/comments", adminH.CommentsList).Methods("GET")
	admin.HandleFunc("/comments/{id}/{action}", adminH.CommentAction).Methods("POST")

	admin.HandleFunc("/invitations", adminH.InvitationsList).Methods("GET")
	admin.HandleFunc("/invitations/create", adminH.InvitationCreate).Methods("POST")

	admin.HandleFunc("/uploads", adminH.UploadsList).Methods("GET")
	admin.HandleFunc("/uploads/create", adminH.UploadCreate).Methods("POST")

	r.NotFoundHandler = http.HandlerFunc(publicH.NotFound)

	go startScheduler(database)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("InkPress listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Println("Server stopped")
}

func startScheduler(database *sql.DB) {
	publishScheduled(database)
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		publishScheduled(database)
	}
}

func publishScheduled(database *sql.DB) {
	_, err := database.Exec(
		"UPDATE posts SET status = 'published' WHERE status = 'scheduled' AND published_at <= NOW()",
	)
	if err != nil {
		log.Printf("scheduler error: %v", err)
	}
}
