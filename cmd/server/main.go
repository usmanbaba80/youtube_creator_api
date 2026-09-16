package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"youtube-creator-api/internal/config"
	"youtube-creator-api/internal/db"
	"youtube-creator-api/internal/handlers"
	"youtube-creator-api/internal/middleware"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	api := &handlers.API{DB: pool, Cfg: cfg}
	limiter := middleware.NewRateLimiter(cfg.RateLimitRPM)

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.CORS(cfg.CORSAllowedOrigins))
	r.Use(limiter.Middleware)
	r.Use(chimw.Timeout(cfg.WriteTimeout))

	log.Printf("youtube-creator-api starting (ready_only=%v)", cfg.ReadyOnly)
	// Unauthenticated liveness (no DB) for probes.
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(middleware.APIKey(cfg.APIKeys))

			r.Get("/health", api.Health)

			r.Route("/creators/{creatorId}", func(r chi.Router) {
				r.Get("/", api.GetCreator)
				r.Get("/videos", api.ListVideos)
				r.Get("/shorts", api.ListShorts)
				r.Get("/playlists", api.ListPlaylists)
				r.Get("/playlists/{playlistId}", api.GetPlaylist)
			})
		})
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		log.Printf("youtube-creator-api listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
