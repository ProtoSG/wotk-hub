package dbmanager

import (
	"log"
	"net/http"
	"workhub/db"
	"workhub/httpx"

	chi "github.com/go-chi/chi/v5"
)

// connFromBody decodes connection params (including the target DB password)
// from the JSON request body. These routes used to be GET with the params
// in the query string, which put passwords in URLs (browser history, proxy
// access logs, etc). They were switched to POST + body, matching the
// existing Connect/Query handlers, to keep credentials out of the URL.
func connFromBody(w http.ResponseWriter, r *http.Request) (db.ConnectionConfig, bool) {
	var req connRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return db.ConnectionConfig{}, false
	}
	cfg, err := req.toConfig()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return db.ConnectionConfig{}, false
	}
	return cfg, true
}

// Connect tests a connection to an external database with the given
// credentials. Admin only.
//
// @Summary Test a database connection
// @Tags dbmanager
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param body body connRequest true "Connection credentials"
// @Success 200 {object} testConnectionResponse
// @Failure 400 {object} httpx.APIError
// @Failure 500 {object} httpx.APIError
// @Router /db/connect [post]
func Connect(w http.ResponseWriter, r *http.Request) {
	var req connRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	cfg, err := req.toConfig()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	adapter, err := getAdapter(cfg.Dialect)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	if err := adapter.TestConnection(cfg); err != nil {
		// Same exception as Query below: this is the user's own DB connection,
		// so the real error (bad host/credentials/db name) is what they need
		// to see to diagnose it, not a generic message.
		log.Printf("dbmanager: test connection failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, testConnectionResponse{Success: true, Message: "Connection successful"})
}

// Tables lists the tables in the connected external database. Admin only.
//
// @Summary List tables
// @Tags dbmanager
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param body body connRequest true "Connection credentials"
// @Success 200 {object} listTablesResponse
// @Failure 400 {object} httpx.APIError
// @Failure 500 {object} httpx.APIError
// @Router /db/tables [post]
func Tables(w http.ResponseWriter, r *http.Request) {
	cfg, ok := connFromBody(w, r)
	if !ok {
		return
	}
	adapter, err := getAdapter(cfg.Dialect)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	tables, err := adapter.ListTables(cfg)
	if err != nil {
		// Same exception as Query below: this is the user's own DB connection,
		// so the real error is what they need to see to diagnose it.
		log.Printf("dbmanager: list tables failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, err.Error())
		return
	}
	if tables == nil {
		tables = []string{}
	}
	httpx.WriteJSON(w, http.StatusOK, listTablesResponse{Tables: tables})
}

// Schema returns column info for one table in the connected external
// database. Admin only.
//
// @Summary Get table schema
// @Tags dbmanager
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param name path string true "Table name"
// @Param body body connRequest true "Connection credentials"
// @Success 200 {object} tableSchemaResponse
// @Failure 400 {object} httpx.APIError
// @Failure 500 {object} httpx.APIError
// @Router /db/table/{name}/schema [post]
func Schema(w http.ResponseWriter, r *http.Request) {
	cfg, ok := connFromBody(w, r)
	if !ok {
		return
	}
	table := chi.URLParam(r, "name")
	adapter, err := getAdapter(cfg.Dialect)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	cols, err := adapter.GetSchema(cfg, table)
	if err != nil {
		// Same exception as Query below: this is the user's own DB connection,
		// so the real error is what they need to see to diagnose it.
		log.Printf("dbmanager: get schema failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, err.Error())
		return
	}
	if cols == nil {
		cols = []db.ColumnInfo{}
	}
	httpx.WriteJSON(w, http.StatusOK, tableSchemaResponse{Table: table, Columns: cols})
}

// Query runs an arbitrary SQL statement against the connected external
// database and returns the raw result. Admin only.
//
// @Summary Run a SQL query
// @Tags dbmanager
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param body body queryRequest true "Connection credentials and SQL statement"
// @Success 200 {object} db.QueryResult
// @Failure 400 {object} httpx.APIError
// @Failure 500 {object} httpx.APIError
// @Router /db/query [post]
func Query(w http.ResponseWriter, r *http.Request) {
	var req queryRequest
	if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid request body")
		return
	}
	if req.SQL == "" {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "sql is required")
		return
	}
	cfg, err := req.connRequest.toConfig()
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	adapter, err := getAdapter(cfg.Dialect)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	result, err := adapter.RunQuery(cfg, req.SQL)
	if err != nil {
		// Same treatment as the other dbmanager handlers: this is the user's
		// own arbitrary-SQL query editor, running against a DB connection they
		// supplied themselves. The error is the target database's response to
		// their query (e.g. a SQL syntax error), not an internal detail of
		// this server, and is exactly what the query
		// editor UI needs to show to be useful.
		log.Printf("dbmanager: run query failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

// Relationships lists foreign keys across the connected external database.
// Admin only.
//
// @Summary List foreign key relationships
// @Tags dbmanager
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param body body connRequest true "Connection credentials"
// @Success 200 {object} listRelationshipsResponse
// @Failure 400 {object} httpx.APIError
// @Failure 500 {object} httpx.APIError
// @Router /db/relationships [post]
func Relationships(w http.ResponseWriter, r *http.Request) {
	cfg, ok := connFromBody(w, r)
	if !ok {
		return
	}
	adapter, err := getAdapter(cfg.Dialect)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, err.Error())
		return
	}
	fks, err := adapter.GetForeignKeys(cfg)
	if err != nil {
		// Same exception as Query below: this is the user's own DB connection,
		// so the real error is what they need to see to diagnose it.
		log.Printf("dbmanager: get relationships failed: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, listRelationshipsResponse{Relationships: fks})
}
