package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/wukfit/equipment-exposure-service/internal/adapters/events"
	apphttp "github.com/wukfit/equipment-exposure-service/internal/adapters/http"
	"github.com/wukfit/equipment-exposure-service/internal/adapters/repository/memory"
	"github.com/wukfit/equipment-exposure-service/internal/app"
	"github.com/wukfit/equipment-exposure-service/internal/app/command"
	"github.com/wukfit/equipment-exposure-service/internal/app/query"
	"github.com/wukfit/equipment-exposure-service/internal/seed"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLevel(getenv("LOG_LEVEL", "info")),
	}))
	slog.SetDefault(logger)

	// Wire the app over in-memory repos preloaded with the seed catalog.
	exposures := memory.NewExposureRepo()
	users := memory.NewUserRepo(seed.Users()...)
	equipment := memory.NewEquipmentRepo(seed.Equipment()...)
	publisher := events.NewSlogPublisher(logger)

	router := apphttp.NewRouter(apphttp.RouterDeps{
		Logger:                 logger,
		Clock:                  app.SystemClock,
		RecordExposure:         command.NewRecordExposure(exposures, users, equipment, publisher, app.SystemClock),
		GetExposure:            query.NewGetExposure(exposures, users, equipment),
		ListExposures:          query.NewListExposures(exposures, users, equipment),
		GetUserExposureSummary: query.NewGetUserExposureSummary(exposures, users),
	})
	srv := &http.Server{
		Addr:              ":" + getenv("PORT", "8080"),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("starting server", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", slog.String("error", err.Error()))
	}
	logger.Info("server stopped")
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
