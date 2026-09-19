package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"internship-management-system/backend/internal/config"
	"internship-management-system/backend/internal/database"
	"internship-management-system/backend/internal/handler"
	appmiddleware "internship-management-system/backend/internal/middleware"
	"internship-management-system/backend/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}

	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	if err := database.MigrateAndSeed(db); err != nil {
		log.Fatalf("prepare database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("access database connection: %v", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			log.Printf("close database connection: %v", err)
		}
	}()

	healthService := service.NewHealthService(db)
	healthHandler := handler.NewHealthHandler(healthService)
	authHandler := handler.NewAuthHandler(db, cfg.JWT.Secret, time.Duration(cfg.JWT.ExpiresHours)*time.Hour)

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), appmiddleware.CORS(cfg.FrontendOrigin))

	api := router.Group("/api")
	api.GET("/health", healthHandler.Get)
	api.POST("/auth/login", authHandler.Login)
	api.GET("/auth/me", appmiddleware.RequireAuth(cfg.JWT.Secret), authHandler.Me)
	api.GET("/protected", appmiddleware.RequireAuth(cfg.JWT.Secret), authHandler.Protected)

	server := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("backend listening on http://localhost:%s", cfg.AppPort)
		serverErrors <- server.ListenAndServe()
	}()

	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("start HTTP server: %v", err)
		}
	case signal := <-shutdownSignals:
		log.Printf("received %s, shutting down", signal)
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		log.Fatalf("gracefully shut down HTTP server: %v", err)
	}
}
