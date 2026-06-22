package service

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dbut.dev/x/vanity"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
	e    *gin.Engine
}

func New(pool *pgxpool.Pool) *Service {
	//gin.SetMode(gin.ReleaseMode) //todo

	s := &Service{
		pool: pool,
		e:    gin.New(),
	}
	s.e.Use(gin.Recovery())
	s.e.Use(vanity.Middleware("github.com/dbut2/coach/go"))

	s.addRoutes()

	return s
}

func (s *Service) addRoutes() {
	s.e.GET("/health", s.health)
}

func (s *Service) health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := s.pool.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Service) Run(ctx context.Context) error {
	server := &http.Server{
		Addr:              ":" + port(),
		Handler:           s.e,
		ReadHeaderTimeout: 5 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func port() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return "8080"
}
