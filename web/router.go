package web

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Server is a struct used to run a web server
type Server struct {
	Host     string
	Port     int
	CertFile string
	KeyFile  string

	Logger *slog.Logger
	DB     *pgxpool.Pool
}

// ListenAndServe creates and setups the necessary http server and start it.
func (s *Server) ListenAndServe() error {
	e := Handler(s)
	log := s.Logger.With(slog.String("nspace", "http"))

	go func() {
		listenAddr := fmt.Sprintf("%s:%d", s.Host, s.Port)
		var err error
		if s.CertFile != "" && s.KeyFile != "" {
			log.Info(fmt.Sprintf("Start HTTPS server on %d", s.Port))
			err = e.StartTLS(listenAddr, s.CertFile, s.KeyFile)
		} else {
			log.Info(fmt.Sprintf("Start HTTP server on %d", s.Port))
			err = e.Start(listenAddr)
		}
		if err != nil && err != http.ErrServerClosed {
			log.Error("failed", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with a
	// timeout of 10 minutes. Use a buffered channel to avoid missing signals
	// as recommended for signal.Notify.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Info("Received interrupt signal")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	return e.Shutdown(ctx)
}

// Handler returns the echo handler for HTTP requests.
func Handler(s *Server) *echo.Echo {
	log := s.Logger.With(slog.String("nspace", "http"))

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Pre(middleware.RemoveTrailingSlash())
	e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
			msg := string(stack)
			log.Error(msg, slog.Bool("panic", true))
			return nil
		},
	}))
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogMethod: true,
		LogURI:    true,
		LogStatus: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			log.Info("Request",
				slog.String("method", v.Method),
				slog.String("uri", v.URI),
				slog.Int("status", v.Status),
			)
			return nil
		},
		Skipper: func(c echo.Context) bool {
			path := c.Path()
			return path == "/status"
		},
	}))

	e.GET("/status", s.Status)
	e.HEAD("/status", s.Status)

	return e
}

// Status responds with the status of the service:
// - 200 if everything if OK
// - 502 if PostgreSQL is not available
func (s *Server) Status(c echo.Context) error {
	status := "OK"
	code := http.StatusOK
	if err := s.DB.Ping(c.Request().Context()); err != nil {
		code = http.StatusInternalServerError
		status = "KO"
	}
	return c.JSON(code, echo.Map{"status": status})
}
