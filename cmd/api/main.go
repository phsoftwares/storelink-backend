package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/phsoftwares/storelink-backend/internal/auth"
	"github.com/phsoftwares/storelink-backend/internal/config"
	database "github.com/phsoftwares/storelink-backend/internal/config/database/migrations"
	"github.com/phsoftwares/storelink-backend/internal/repositories"
	"github.com/phsoftwares/storelink-backend/internal/routes"
	"github.com/phsoftwares/storelink-backend/internal/services"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// @title StoreLink API
// @version 1.0
// @description StoreLink integrates stores. Administrators use JWT Bearer; each FNTS_Server installation uses its own fixed X-API-Key and database-validated X-Agent-ID.
// @BasePath /api
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Bearer token obtained from /auth/login.
// @securityDefinitions.apikey AgentAPIKey
// @in header
// @name X-API-Key
// @description Per-agent fixed key shown once at provisioning or rotation.
func main() {
	if e := run(); e != nil {
		slog.Error("backend stopped", "reason", e.Error())
		os.Exit(1)
	}
}
func run() error {
	c, e := config.Load()
	if e != nil {
		return e
	}
	gin.SetMode(gin.ReleaseMode)
	db, e := database.InitConnection(c.DatabaseURL)
	if e != nil {
		return errors.New("falha ao iniciar banco ou migrations")
	}
	sqlDB, e := db.DB()
	if e != nil {
		return e
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	application := services.New(repositories.New(db), auth.New(c.JWTSecret), c)
	seedCtx, seedCancel := context.WithTimeout(context.Background(), 10*time.Second)
	seeded, e := application.EnsureInitialAdmin(seedCtx)
	seedCancel()
	if e != nil {
		return fmt.Errorf("initialize first administrator: %w", e)
	}
	if seeded {
		slog.Info("initial administrator created", "email", c.AdminEmail)
	}
	srv := &http.Server{Addr: ":" + c.Port, Handler: routes.New(application), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: c.RequestTimeout, WriteTimeout: c.RequestTimeout, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { slog.Info("StoreLink API ready", "port", c.Port); done <- srv.ListenAndServe() }()
	select {
	case e := <-done:
		if !errors.Is(e, http.ErrServerClosed) {
			return e
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if e := srv.Shutdown(shutdown); e != nil {
			return e
		}
		if e := <-done; !errors.Is(e, http.ErrServerClosed) {
			return e
		}
	}
	return nil
}
