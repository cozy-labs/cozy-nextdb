package web

import (
	"context"
	"fmt"
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

	DB *pgxpool.Pool
}

// ListenAndServe creates and setups the necessary http server and start it.
func (s *Server) ListenAndServe() error {
	e := Handler(s)

	go func() {
		listenAddr := fmt.Sprintf("%s:%d", s.Host, s.Port)
		var err error
		if s.CertFile != "" && s.KeyFile != "" {
			err = e.StartTLS(listenAddr, s.CertFile, s.KeyFile)
		} else {
			err = e.Start(listenAddr)
		}
		if err != nil && err != http.ErrServerClosed {
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with a
	// timeout of 10 minutes. Use a buffered channel to avoid missing signals
	// as recommended for signal.Notify.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	return e.Shutdown(ctx)
}

// Handler returns the echo handler for HTTP requests.
func Handler(s *Server) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Pre(middleware.RemoveTrailingSlash())

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
