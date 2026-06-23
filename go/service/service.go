package service

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dbut.dev/x/vanity"
	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"

	"naomi.run/database"
	"naomi.run/strava"
)

type Config struct {
	Port               string  `env:"PORT" envDefault:"8080"`
	BaseURL            string  `env:"BASE_URL,required"`
	StravaClientID     string  `env:"STRAVA_CLIENT_ID,required"`
	StravaClientSecret string  `env:"STRAVA_CLIENT_SECRET,required"`
	WebhookVerifyToken string  `env:"STRAVA_VERIFY_TOKEN"`
	CoachName          string  `env:"COACH_NAME" envDefault:"Naomi"`
	AllowedAthletes    []int64 `env:"STRAVA_ALLOWED_ATHLETES"`
}

func (c Config) athleteAllowed(id int64) bool {
	for _, a := range c.AllowedAthletes {
		if a == id {
			return true
		}
	}
	return false
}

type Service struct {
	cfg   Config
	db    *sql.DB
	q     *database.Queries
	oauth *oauth2.Config
	e     *gin.Engine
}

func New(db *sql.DB, cfg Config) *Service {
	s := &Service{
		cfg:   cfg,
		db:    db,
		q:     database.New(db),
		oauth: strava.Config(cfg.StravaClientID, cfg.StravaClientSecret, "https://strava.dbut.dev/naomi"), // todo: configure redirect
		e:     gin.Default(),
	}
	s.e.Use(vanity.Middleware("github.com/dbut2/coach/go"))

	s.addRoutes()

	return s
}

func (s *Service) addRoutes() {
	s.e.GET("/health", s.health)

	s.e.GET("/", s.index)
	s.e.GET("/auth/strava", s.connectStrava)
	s.e.GET("/auth/strava/callback", s.stravaCallback)
	s.e.POST("/logout", s.logout)

	s.e.GET("/webhook/strava", s.webhookVerify)
	s.e.POST("/webhook/strava", s.webhookEvent)

	authed := s.e.Group("/", s.requireAuth)
	authed.GET("/conversation", s.conversation)
	authed.GET("/settings", s.settings)
}

func (s *Service) health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := s.db.PingContext(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func render(c *gin.Context, status int, comp templ.Component) {
	c.Status(status)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = comp.Render(c.Request.Context(), c.Writer)
}

func (s *Service) Run(ctx context.Context) error {
	server := &http.Server{
		Addr:              ":" + s.cfg.Port,
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
