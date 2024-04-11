package core

import "errors"

var (
	ErrInternalServerError = errors.New("internal_server_error")
	ErrIllegalDatabaseName = errors.New("illegal_database_name")
	ErrDatabaseExists      = errors.New("file_exists")
)
