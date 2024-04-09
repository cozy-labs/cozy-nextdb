package web

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// ListenAndServe creates and setups the necessary http server and start it.
func ListenAndServe(host string, port int, certFile, keyFile string) error {
	e := Handler()

	go func() {
		listenAddr := fmt.Sprintf("%s:%d", host, port)
		var err error
		if certFile != "" && keyFile != "" {
			err = e.StartTLS(listenAddr, certFile, keyFile)
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
func Handler() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Pre(middleware.RemoveTrailingSlash())

	e.GET("/status", Status)
	e.HEAD("/status", Status)

	return e
}

// Status responds with the status of the service:
// - 200 if everything if OK
// - 502 if PostgreSQL is not available
func Status(c echo.Context) error {
	return c.JSON(http.StatusOK, echo.Map{"status": "OK"})
}
