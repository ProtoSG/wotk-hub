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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "message": "Connection successful"})
}

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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"tables": tables})
}

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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"table": table, "columns": cols})
}

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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"relationships": fks})
}
