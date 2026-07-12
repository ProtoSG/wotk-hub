package dbmanager

import (
	"fmt"
	"workhub/db"
	"workhub/db/mysql"
	"workhub/db/postgres"
)

func getAdapter(dialect db.Dialect) (db.Adapter, error) {
	switch dialect {
	case db.Postgres:
		return postgres.Adapter{}, nil
	case db.MySQL:
		return mysql.Adapter{}, nil
	default:
		return nil, fmt.Errorf("unsupported dialect: %s", dialect)
	}
}
