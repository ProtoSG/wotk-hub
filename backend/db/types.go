package db

type Dialect string

const (
	Postgres Dialect = "postgres"
	MySQL    Dialect = "mysql"
)

type ConnectionConfig struct {
	Dialect  Dialect
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

type ColumnInfo struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Nullable     bool    `json:"nullable"`
	Default      *string `json:"default"`
	IsPrimaryKey bool    `json:"isPrimaryKey"`
}

type ForeignKey struct {
	FromTable  string `json:"fromTable"`
	FromColumn string `json:"fromColumn"`
	ToTable    string `json:"toTable"`
	ToColumn   string `json:"toColumn"`
}

type QueryResult struct {
	Columns         []string         `json:"columns"`
	Rows            []map[string]any `json:"rows"`
	RowCount        int              `json:"rowCount"`
	ExecutionTimeMs int64            `json:"executionTimeMs"`
	Truncated       bool             `json:"truncated"`
}

type Adapter interface {
	TestConnection(cfg ConnectionConfig) error
	ListTables(cfg ConnectionConfig) ([]string, error)
	GetSchema(cfg ConnectionConfig, table string) ([]ColumnInfo, error)
	RunQuery(cfg ConnectionConfig, sql string) (QueryResult, error)
	GetForeignKeys(cfg ConnectionConfig) ([]ForeignKey, error)
}
