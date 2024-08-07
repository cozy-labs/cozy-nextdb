package web

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/trace"
	"strconv"
	"time"

	"github.com/cozy-labs/cozy-nextdb/core"
	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const EchoRequestIDKey = "req_id"

// Server is a struct used to run a web server
type Server struct {
	Host     string
	Port     int
	CertFile string
	KeyFile  string

	Logger *slog.Logger
	PG     *pgxpool.Pool
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
			log.Error("failed", slog.String("error", err.Error()))
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

	e.Pre(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			headers := c.Request().Header
			reqID := cmp.Or(
				headers.Get("X-Request-Id"),
				headers.Get("X-Request-UID"),
				headers.Get("X-Haproxy-Unique-ID"),
			)
			if reqID == "" {
				if uuidv7, err := uuid.NewV7(); err == nil {
					reqID = uuidv7.String()
				}
			}
			c.Set(EchoRequestIDKey, reqID)
			return next(c)
		}
	})
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
				slog.Any("req_id", c.Get(EchoRequestIDKey)),
			)
			return nil
		},
		Skipper: func(c echo.Context) bool {
			path := c.Path()
			return path == "/status"
		},
	}))

	if trace.IsEnabled() {
		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				var err error
				req := c.Request()
				ctx := req.Context()
				trace.WithRegion(ctx, "Echo handler", func() {
					trace.Logf(ctx, "HTTP request", "%s %s", req.Method, req.URL.Path)
					err = next(c)
				})
				return err
			}
		})
	}

	e.GET("/status", s.Status)
	e.HEAD("/status", s.Status)

	e.GET("/_all_dbs", s.GetAllDatabases)
	e.GET("/:db", s.GetDatabase)
	e.HEAD("/:db", s.GetDatabase)
	e.PUT("/:db", s.CreateDatabase)
	e.DELETE("/:db", s.DeleteDatabase)

	e.PUT("/:db/_design/:ddoc", s.CreateDesignDoc)
	e.GET("/:db/_design/:ddoc/_view/:view", s.GetView)

	e.GET("/:db/_all_docs", s.GetAllDocs)
	e.GET("/:db/_changes", s.GetChanges)
	e.POST("/:db", s.CreateDocument)
	e.GET("/:db/:docid", s.GetDocument)
	e.HEAD("/:db/:docid", s.GetDocument)
	e.PUT("/:db/:docid", s.PutDocument)
	e.DELETE("/:db/:docid", s.DeleteDocument)

	e.POST("/:db/_find", s.FindMango)

	return e
}

func newOperator(s *Server, c echo.Context) *core.Operator {
	reqID := c.Get(EchoRequestIDKey)
	logger := s.Logger.With(slog.Any("req_id", reqID))
	ctx := context.WithValue(c.Request().Context(), core.RequestIDKey{}, reqID)
	return &core.Operator{
		PG:     s.PG,
		Logger: logger,
		Ctx:    ctx,
	}
}

// GetAllDatabases is the handler for GET /_all_dbs. It returns the list of the
// databases.
func (s *Server) GetAllDatabases(c echo.Context) error {
	op := newOperator(s, c)
	params := core.AllDocsParams{
		Descending: c.QueryParam("descending") == "true",
	}
	var key string
	if startKey := c.QueryParam("startkey"); startKey != "" {
		if err := json.Unmarshal([]byte(startKey), &key); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error":  "bad_request",
				"reason": err.Error(),
			})
		}
		params.StartKey = key
	}
	if startKey := c.QueryParam("start_key"); startKey != "" {
		if err := json.Unmarshal([]byte(startKey), &key); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error":  "bad_request",
				"reason": err.Error(),
			})
		}
		params.StartKey = key
	}
	if endKey := c.QueryParam("endkey"); endKey != "" {
		if err := json.Unmarshal([]byte(endKey), &key); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error":  "bad_request",
				"reason": err.Error(),
			})
		}
		params.EndKey = key
	}
	if endKey := c.QueryParam("end_key"); endKey != "" {
		if err := json.Unmarshal([]byte(endKey), &key); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error":  "bad_request",
				"reason": err.Error(),
			})
		}
		params.EndKey = key
	}
	if limit := c.QueryParam("limit"); limit != "" {
		nb, err := strconv.Atoi(limit)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error":  "query_parse_error",
				"reason": err.Error(),
			})
		}
		params.Limit = nb
	}
	if skip := c.QueryParam("skip"); skip != "" {
		nb, err := strconv.Atoi(skip)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error":  "query_parse_error",
				"reason": err.Error(),
			})
		}
		params.Skip = nb
	}

	result, err := op.GetAllDatabases(params)
	switch {
	case err == nil:
		return c.JSON(http.StatusOK, result)
	case errors.Is(err, core.ErrNotFound):
		return c.JSON(http.StatusNotFound, map[string]any{
			"error":  err.Error(),
			"reason": "missing",
		})
	case errors.Is(err, core.ErrNotImplemented):
		return c.JSON(http.StatusNotImplemented, map[string]any{
			"error":  "not_implemented",
			"reason": err.Error(),
		})
	default:
		op.Logger.With(slog.Any("error", err.Error())).Error("internal_server_error")
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"error":  "internal_server_error",
			"reason": err.Error(),
		})
	}
}

// GetDatabase is the handler for GET/HEAD /:db. It returns information about
// the given database (number of documents).
func (s *Server) GetDatabase(c echo.Context) error {
	op := newOperator(s, c)
	result, err := op.GetDatabase(c.Param("db"))
	switch {
	case err == nil:
		return c.JSON(http.StatusOK, result)
	case errors.Is(err, core.ErrNotFound):
		return c.JSON(http.StatusNotFound, map[string]any{
			"error":  err.Error(),
			"reason": "Database does not exist.",
		})
	default:
		op.Logger.With(slog.Any("error", err.Error())).Error("internal_server_error")
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"error":  "internal_server_error",
			"reason": err.Error(),
		})
	}
}

// CreateDatabase is the handler for PUT /:db. It creates a database (in the
// CouchDB meaning, not a PostgreSQL database).
func (s *Server) CreateDatabase(c echo.Context) error {
	op := newOperator(s, c)
	err := op.CreateDatabase(c.Param("db"))
	switch {
	case err == nil:
		return c.JSON(http.StatusCreated, map[string]any{"ok": true})
	case errors.Is(err, core.ErrIllegalDatabaseName):
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error":  err.Error(),
			"reason": "Name: '_db'. Only lowercase characters (a-z), digits (0-9), and any of the characters _, $, (, ), +, -, and / are allowed. Must begin with a letter.",
		})
	case errors.Is(err, core.ErrDatabaseExists):
		return c.JSON(http.StatusPreconditionFailed, map[string]any{
			"error":  err.Error(),
			"reason": "The database could not be created, the file already exists.",
		})
	default:
		op.Logger.With(slog.Any("error", err.Error())).Error("internal_server_error")
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"error":  "internal_server_error",
			"reason": err.Error(),
		})
	}
}

// DeleteDatabase is the handler for DELETE /:db. It deletes a database (in the
// CouchDB meaning, not a PostgreSQL database).
func (s *Server) DeleteDatabase(c echo.Context) error {
	op := newOperator(s, c)
	err := op.DeleteDatabase(c.Param("db"))
	switch {
	case err == nil:
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	case errors.Is(err, core.ErrNotFound):
		return c.JSON(http.StatusNotFound, map[string]any{
			"error":  err.Error(),
			"reason": "Database does not exist.",
		})
	default:
		op.Logger.With(slog.Any("error", err.Error())).Error("internal_server_error")
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"error":  "internal_server_error",
			"reason": err.Error(),
		})
	}
}

// CreateDesignDoc is the handler for PUT /:db/_design/:ddoc. It creates a
// view (its design document) in the given database.
func (s *Server) CreateDesignDoc(c echo.Context) error {
	op := newOperator(s, c)
	docID := "_design/" + c.Param("ddoc")
	doc, err := op.CreateDesignDoc(c.Param("db"), docID, c.Request().Body)
	switch {
	case err == nil:
		return c.JSON(http.StatusCreated, map[string]any{
			"ok":  true,
			"id":  doc["_id"],
			"rev": doc["_rev"],
		})
	case errors.Is(err, core.ErrBadRequest):
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error":  err.Error(),
			"reason": "invalid UTF-8 JSON",
		})
	case errors.Is(err, core.ErrNotFound):
		return c.JSON(http.StatusNotFound, map[string]any{
			"error":  err.Error(),
			"reason": "Database does not exist.",
		})
	case errors.Is(err, core.ErrConflict):
		return c.JSON(http.StatusConflict, map[string]any{
			"error":  err.Error(),
			"reason": "Document update conflict.",
		})
	default:
		op.Logger.With(slog.Any("error", err.Error())).Error("internal_server_error")
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"error":  "internal_server_error",
			"reason": err.Error(),
		})
	}
}

// GetView is the handler for GET /:db/_design/:ddoc/_view/:view. It executes
// the specified view function from the specified design document.
func (s *Server) GetView(c echo.Context) error {
	op := newOperator(s, c)
	// TODO params like reduce, startkey, endkey

	docID := "_design/" + c.Param("ddoc")
	result, err := op.GetView(c.Param("db"), docID, c.Param("view"))
	switch {
	case err == nil:
		return c.JSON(http.StatusOK, result)
	case errors.Is(err, core.ErrNotFound):
		return c.JSON(http.StatusNotFound, map[string]any{
			"error":  err.Error(),
			"reason": "missing",
		})
	default:
		op.Logger.With(slog.Any("error", err.Error())).Error("internal_server_error")
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"error":  "internal_server_error",
			"reason": err.Error(),
		})
	}
}

// GetAllDocs is the handler for GET /:db/_all_docs. It returns all of the
// documents in the database (ie normal docs and design docs, but not local
// docs).
func (s *Server) GetAllDocs(c echo.Context) error {
	op := newOperator(s, c)
	params := core.AllDocsParams{
		IncludeDocs: c.QueryParam("include_docs") == "true",
		Descending:  c.QueryParam("descending") == "true",
	}
	var key string
	if startKey := c.QueryParam("startkey"); startKey != "" {
		if err := json.Unmarshal([]byte(startKey), &key); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error":  "bad_request",
				"reason": err.Error(),
			})
		}
		params.StartKey = key
	}
	if startKey := c.QueryParam("start_key"); startKey != "" {
		if err := json.Unmarshal([]byte(startKey), &key); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error":  "bad_request",
				"reason": err.Error(),
			})
		}
		params.StartKey = key
	}
	if endKey := c.QueryParam("endkey"); endKey != "" {
		if err := json.Unmarshal([]byte(endKey), &key); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error":  "bad_request",
				"reason": err.Error(),
			})
		}
		params.EndKey = key
	}
	if endKey := c.QueryParam("end_key"); endKey != "" {
		if err := json.Unmarshal([]byte(endKey), &key); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error":  "bad_request",
				"reason": err.Error(),
			})
		}
		params.EndKey = key
	}
	if limit := c.QueryParam("limit"); limit != "" {
		nb, err := strconv.Atoi(limit)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error":  "query_parse_error",
				"reason": err.Error(),
			})
		}
		params.Limit = nb
	}
	if skip := c.QueryParam("skip"); skip != "" {
		nb, err := strconv.Atoi(skip)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error":  "query_parse_error",
				"reason": err.Error(),
			})
		}
		params.Skip = nb
	}

	result, err := op.GetAllDocs(c.Param("db"), params)
	switch {
	case err == nil:
		return c.JSON(http.StatusOK, result)
	case errors.Is(err, core.ErrNotFound):
		return c.JSON(http.StatusNotFound, map[string]any{
			"error":  err.Error(),
			"reason": "missing",
		})
	default:
		op.Logger.With(slog.Any("error", err.Error())).Error("internal_server_error")
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"error":  "internal_server_error",
			"reason": err.Error(),
		})
	}
}

// GetChanges is the handler for GET /:db/_changes. It returns a sorted list of
// changes made to documents in the database.
func (s *Server) GetChanges(c echo.Context) error {
	op := newOperator(s, c)
	params := core.ChangesParams{
		Limit: -1,
		Since: c.QueryParam("since"),
	}
	if limit := c.QueryParam("limit"); limit != "" {
		nb, err := strconv.Atoi(limit)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error":  "query_parse_error",
				"reason": err.Error(),
			})
		}
		params.Limit = nb
	}

	result, err := op.GetChanges(c.Param("db"), params)
	switch {
	case err == nil:
		return c.JSON(http.StatusOK, result)
	case errors.Is(err, core.ErrNotFound):
		return c.JSON(http.StatusNotFound, map[string]any{
			"error":  err.Error(),
			"reason": "missing",
		})
	default:
		op.Logger.With(slog.Any("error", err.Error())).Error("internal_server_error")
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"error":  "internal_server_error",
			"reason": err.Error(),
		})
	}
}

// CreateDocument is the handler for POST /:db. It creates a document in the
// given database.
func (s *Server) CreateDocument(c echo.Context) error {
	op := newOperator(s, c)
	doc, err := op.CreateDocument(c.Param("db"), c.Request().Body)
	switch {
	case err == nil:
		return c.JSON(http.StatusCreated, map[string]any{
			"ok":  true,
			"id":  doc["_id"],
			"rev": doc["_rev"],
		})
	case errors.Is(err, core.ErrBadRequest):
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error":  err.Error(),
			"reason": "invalid UTF-8 JSON",
		})
	case errors.Is(err, core.ErrNotFound):
		return c.JSON(http.StatusNotFound, map[string]any{
			"error":  err.Error(),
			"reason": "Database does not exist.",
		})
	case errors.Is(err, core.ErrConflict):
		return c.JSON(http.StatusConflict, map[string]any{
			"error":  err.Error(),
			"reason": "Document update conflict.",
		})
	default:
		op.Logger.With(slog.Any("error", err.Error())).Error("internal_server_error")
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"error":  "internal_server_error",
			"reason": err.Error(),
		})
	}
}

// GetDocument is the handler for GET/HEAD /:db/:docid. It returns the given
// document.
func (s *Server) GetDocument(c echo.Context) error {
	op := newOperator(s, c)
	withRevisions := c.QueryParam("revs") == "true"
	result, err := op.GetDocument(c.Param("db"), c.Param("docid"), withRevisions)
	switch {
	case err == nil:
		rev, _ := result["_rev"].(string)
		c.Response().Header().Set("ETag", rev)
		return c.JSON(http.StatusOK, result)
	case errors.Is(err, core.ErrNotFound):
		return c.JSON(http.StatusNotFound, map[string]any{
			"error":  err.Error(),
			"reason": "missing",
		})
	case errors.Is(err, core.ErrDeleted):
		return c.JSON(http.StatusNotFound, map[string]any{
			"error":  "not_found",
			"reason": "deleted",
		})
	default:
		op.Logger.With(slog.Any("error", err.Error())).Error("internal_server_error")
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"error":  "internal_server_error",
			"reason": err.Error(),
		})
	}
}

// PutDocument is the handler for PUT /:db/:docid. It creates a new document or
// a new revision of an existing document.
func (s *Server) PutDocument(c echo.Context) error {
	op := newOperator(s, c)
	docID := c.Param("docid")
	rev := c.QueryParam("rev")
	if rev == "" {
		rev = c.Request().Header.Get("If-Match")
	}
	doc, err := op.PutDocument(c.Param("db"), docID, rev, c.Request().Body)
	switch {
	case err == nil:
		rev, _ := doc["_rev"].(string)
		c.Response().Header().Set("ETag", rev)
		return c.JSON(http.StatusCreated, map[string]any{
			"ok":  true,
			"id":  doc["_id"],
			"rev": doc["_rev"],
		})
	case errors.Is(err, core.ErrBadRequest):
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error":  err.Error(),
			"reason": "invalid UTF-8 JSON",
		})
	case errors.Is(err, core.ErrNotFound):
		return c.JSON(http.StatusNotFound, map[string]any{
			"error":  err.Error(),
			"reason": "Database does not exist.",
		})
	case errors.Is(err, core.ErrConflict):
		return c.JSON(http.StatusConflict, map[string]any{
			"error":  err.Error(),
			"reason": "Document update conflict.",
		})
	default:
		op.Logger.With(slog.Any("error", err.Error())).Error("internal_server_error")
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"error":  "internal_server_error",
			"reason": err.Error(),
		})
	}
}

// DeleteDocument is the handler for DELETE /:db/:docid. It marks the given
// document as deleted.
func (s *Server) DeleteDocument(c echo.Context) error {
	op := newOperator(s, c)
	docID := c.Param("docid")
	rev := c.QueryParam("rev")
	if rev == "" {
		rev = c.Request().Header.Get("If-Match")
	}
	doc, err := op.DeleteDocument(c.Param("db"), docID, rev)
	switch {
	case err == nil:
		rev, _ := doc["_rev"].(string)
		c.Response().Header().Set("ETag", rev)
		return c.JSON(http.StatusOK, map[string]any{
			"ok":  true,
			"id":  docID,
			"rev": rev,
		})
	case errors.Is(err, core.ErrNotFound):
		return c.JSON(http.StatusNotFound, map[string]any{
			"error":  err.Error(),
			"reason": "missing",
		})
	case errors.Is(err, core.ErrConflict):
		return c.JSON(http.StatusConflict, map[string]any{
			"error":  err.Error(),
			"reason": "Document update conflict.",
		})
	default:
		op.Logger.With(slog.Any("error", err.Error())).Error("internal_server_error")
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"error":  "internal_server_error",
			"reason": err.Error(),
		})
	}
}

// FindMango is the handler for POST /:db/_find. It finds documents using a
// declarative JSON querying syntax.
func (s *Server) FindMango(c echo.Context) error {
	op := newOperator(s, c)
	var params core.MangoParams
	if err := json.NewDecoder(c.Request().Body).Decode(&params); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error":  err.Error(),
			"reason": "invalid UTF-8 JSON",
		})
	}

	result, err := op.FindMango(c.Param("db"), params)
	switch {
	case err == nil:
		return c.JSON(http.StatusOK, result)
	case errors.Is(err, core.ErrNotFound):
		return c.JSON(http.StatusNotFound, map[string]any{
			"error":  err.Error(),
			"reason": "missing",
		})
	default:
		op.Logger.With(slog.Any("error", err.Error())).Error("internal_server_error")
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"error":  "internal_server_error",
			"reason": err.Error(),
		})
	}
}

// Status responds with the status of the service:
// - 200 if everything if OK
// - 502 if PostgreSQL is not available
func (s *Server) Status(c echo.Context) error {
	op := newOperator(s, c)
	err := op.Ping()
	switch {
	case err == nil:
		return c.JSON(http.StatusOK, map[string]any{"status": "OK"})
	default:
		s.Logger.Warn("Cannot ping PostgreSQL",
			slog.String("nspace", "status"),
			slog.String("error", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]any{"status": "KO"})
	}
}
