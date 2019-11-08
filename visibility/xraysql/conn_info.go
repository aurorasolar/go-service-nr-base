package xraysql

import (
	"context"
	"database/sql/driver"
)

// Driver implements a database/sql/driver.Driver, needed to satisfy the
// driver.Connector interface for sql.DB
type FakeDriver struct {
}

func (w *FakeDriver) Open(dsn string) (driver.Conn, error) {
	panic("Not implemented")
}

// Connection provider that also returns driver-specific connection metadata
type ConnectionProvider interface {
	Connect(ctx context.Context) (driver.Conn, *ConnInfo, error)
}

// General connection metainformation for tracing purposes
type ConnInfo struct {
	SanitizedConnString       string
	DbType, DriverName        string
	DbVersion, DbUser, DbName string
}

