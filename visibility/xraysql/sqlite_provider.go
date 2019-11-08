package xraysql

import (
	"context"
	"database/sql/driver"
	"github.com/mattn/go-sqlite3"
)

type SqliteConnector struct {
	connString string
}

func NewSqliteConnector(connString string) *SqliteConnector {
	return &SqliteConnector{connString: connString}
}

func (s *SqliteConnector) Connect(ctx context.Context) (
	driver.Conn, *ConnInfo, error) {

	drv := sqlite3.SQLiteDriver{
		Extensions:  nil,
		ConnectHook: nil,
	}

	conn, err := drv.Open(s.connString)
	if err != nil {
		return nil, nil, err
	}

	return conn, &ConnInfo{
		SanitizedConnString: s.connString,
		DbType:              "sqlite",
		DriverName:          "sqlite3",
		DbVersion:           "sqlitedb",
		DbUser:              "none",
		DbName:              "sqlite_db_name",
	}, nil
}
