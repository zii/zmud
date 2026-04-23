package lmdbwrapper

import "errors"

var (
	// ErrDBClosed 尝试在数据库关闭后执行操作时返回
	ErrDBClosed = errors.New("database is closed")

	// ErrReadOnlyTx 在只读事务中尝试写操作时返回
	ErrReadOnlyTx = errors.New("transaction is read-only")

	// ErrInvalidOpts 提供的选项无效时返回
	ErrInvalidOpts = errors.New("invalid options")
)