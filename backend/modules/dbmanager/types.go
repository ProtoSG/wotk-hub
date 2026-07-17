package dbmanager

import (
	"fmt"
	"workhub/db"
)

type connRequest struct {
	Dialect  string `json:"dialect"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

type queryRequest struct {
	connRequest
	SQL string `json:"sql"`
}

type testConnectionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type listTablesResponse struct {
	Tables []string `json:"tables"`
}

type tableSchemaResponse struct {
	Table   string          `json:"table"`
	Columns []db.ColumnInfo `json:"columns"`
}

type listRelationshipsResponse struct {
	Relationships []db.ForeignKey `json:"relationships"`
}

func (r connRequest) toConfig() (db.ConnectionConfig, error) {
	d := db.Dialect(r.Dialect)
	if d != db.Postgres && d != db.MySQL {
		return db.ConnectionConfig{}, fmt.Errorf("invalid dialect: %s", r.Dialect)
	}
	if r.Host == "" || r.User == "" || r.Database == "" || r.Port == 0 {
		return db.ConnectionConfig{}, fmt.Errorf("missing required connection fields")
	}
	return db.ConnectionConfig{
		Dialect:  d,
		Host:     r.Host,
		Port:     r.Port,
		User:     r.User,
		Password: r.Password,
		Database: r.Database,
	}, nil
}
