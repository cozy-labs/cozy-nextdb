package core

import "errors"

var (
	ErrBadRequest          = errors.New("bad_request")
	ErrNotFound            = errors.New("not_found")
	ErrConflict            = errors.New("conflict")
	ErrInternalServerError = errors.New("internal_server_error")

	ErrIllegalDatabaseName = errors.New("illegal_database_name")
	ErrDatabaseExists      = errors.New("file_exists")
	ErrDeleted             = errors.New("deleted")

	ErrNotImplemented = errors.New("not_implemented")
)
