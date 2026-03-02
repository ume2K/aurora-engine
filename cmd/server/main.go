package main

import (
	"context"
	"fmt"
	"gocore/internal/app"
	"gocore/internal/auth"
	"gocore/internal/config"
	"gocore/pkg/framework"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type PageData struct {
	Title string
}

func main() {
	framework.LoadEnv(".env")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	isDev := cfg.AppEnv == "development"

	if isDev {
		startDevAssetWatchers()
	}

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer startupCancel()

	deps, err := app.NewDeps(startupCtx, cfg)
	if err != nil {
		log.Fatalf("initialize dependencies: %v", err)
	}

	jwtManager := auth.NewJWTManager(cfg.JWTSecret, cfg.JWTExpiresIn)
	authRepo := auth.NewPostgresRepository(deps.DB)
	authService := auth.NewService(authRepo, jwtManager)
	authHandler := auth.NewHandler(authService)

	r := framework.NewRouter()
	r.Use(framework.Logger)
	r.Use(framework.Recovery)

	r.LoadHTMLGlob("views/*.html")
	r.LoadHTMLGlob("views/components/*.html")
	r.Static("/assets", "./public")

	r.GET("/", func(c *framework.Context) {
		c.HTML(http.StatusOK, "index.html", PageData{Title: "GoCore"})
	})

	r.GET("/api/health", func(c *framework.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"status": "ok",
			"app":    cfg.AppName,
			"env":    cfg.AppEnv,
		})
	})

	r.GET("/api/health/deps", func(c *framework.Context) {
		healthCtx, cancel := context.WithTimeout(c.R.Context(), 3*time.Second)
		defer cancel()

		if err := deps.DB.Ping(healthCtx); err != nil {
			c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "postgres_down"})
			return
		}
		if err := deps.Redis.Ping(healthCtx).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "redis_down"})
			return
		}
		if _, err := deps.S3.ListBuckets(healthCtx); err != nil {
			c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "rustfs_down"})
			return
		}

		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	api := r.Group("/api")
	api.POST("/auth/register", func(c *framework.Context) { authHandler.Register(c) })
	api.POST("/auth/login", func(c *framework.Context) { authHandler.Login(c) })

	protected := r.Group("/api")
	protected.Use(framework.RequireBearerAuth(auth.FrameworkTokenVerifier(jwtManager)))
	protected.GET("/users/me", func(c *framework.Context) { authHandler.Me(c) })

	addr := ":" + cfg.Port

	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Server running on http://localhost%s (bind: 0.0.0.0%s)", addr, addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	shutdownSignalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErr:
		if err != nil {
			log.Fatalf("listen and serve: %v", err)
		}
	case <-shutdownSignalCtx.Done():
		log.Println("Shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
	deps.Close()
}

func startDevAssetWatchers() {
	fmt.Println("GoCore Framework - Development Mode")
	log.Println("Initializing SCSS Compiler...")
	scss := framework.NewSCSSCompiler(framework.SCSSConfig{
		SourceDir: "assets/scss",
		OutputDir: "public/css",
		Debug:     true,
	})
	if err := scss.Compile(); err != nil {
		log.Println("SCSS compilation failed")
	}

	log.Println("Initializing JS Bundler...")
	js := framework.NewJSCompiler(framework.JSConfig{
		SourceDir: "assets/js",
		OutputDir: "public/js",
	})
	if err := js.Bundle(); err != nil {
		log.Println("JS Bundle failed")
	}

	go scss.Watch()
	go js.Watch()
}
