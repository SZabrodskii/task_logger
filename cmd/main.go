package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"task_api/internal/handler"
	"task_api/internal/logger"
	"task_api/internal/repository"
	"task_api/internal/service"
	"time"
)

type Config struct {
	Logger logger.Config
}

func loadConfig() Config {
	return Config{
		Logger: logger.Config{
			Level:        logger.ParseLevel(os.Getenv("LOG_LEVEL")),
			IsProduction: os.Getenv("APP_ENV") == "production",
		},
	}
}

func main() {
	cfg := loadConfig()
	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	baseLogger := logger.NewAsyncLogger(appCtx, cfg.Logger)
	baseLogger.Info("Application starting...")

	taskRepo := repository.NewTaskRepository(baseLogger)
	taskService := service.NewTaskService(taskRepo, baseLogger)
	taskHandler := handler.NewTaskHandler(taskService, baseLogger)

	mux := http.NewServeMux()
	taskHandler.RegisterRoutes(mux)

	loggedMux := logger.RequestLogger(baseLogger)(mux)

	server := &http.Server{Addr: ":8080", Handler: loggedMux}

	go func() {
		baseLogger.Info("Server is listening on port 8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			baseLogger.Fatal("Failed to start server", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	baseLogger.Info("Server is shutting down...")

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Failed to shutdown server: %v\n", err)
	}

	baseLogger.Close()

}
