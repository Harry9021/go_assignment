package main

import (
	"context"
	// "log"
	"net/http"
	"time"
)

// handleGetClickHouseTables retrieves tables from ClickHouse
func handleGetClickHouseTables(w http.ResponseWriter, r *http.Request) {
	var config ClickHouseConfig
	if err := ReadJSONBody(r, &config); err != nil {
		WriteJSONResponse(w, http.StatusBadRequest, NewErrorResponse("Invalid request body", err))
		return
	}

	client, err := NewClickHouseClient(config)
	if err != nil {
		WriteJSONResponse(w, http.StatusInternalServerError, NewErrorResponse("Failed to connect to ClickHouse", err))
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tables, err := client.GetTables(ctx)
	if err != nil {
		WriteJSONResponse(w, http.StatusInternalServerError, NewErrorResponse("Failed to get tables", err))
		return
	}

	WriteJSONResponse(w, http.StatusOK, NewSuccessResponse("Retrieved tables successfully", tables, len(tables)))
}

// handleGetClickHouseColumns retrieves columns from a ClickHouse table
func handleGetClickHouseColumns(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Config    ClickHouseConfig `json:"config"`
		TableName string           `json:"tableName"`
	}

	if err := ReadJSONBody(r, &req); err != nil {
		WriteJSONResponse(w, http.StatusBadRequest, NewErrorResponse("Invalid request body", err))
		return
	}

	client, err := NewClickHouseClient(req.Config)
	if err != nil {
		WriteJSONResponse(w, http.StatusInternalServerError, NewErrorResponse("Failed to connect to ClickHouse", err))
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	columns, err := client.GetTableColumns(ctx, req.TableName)
	if err != nil {
		WriteJSONResponse(w, http.StatusInternalServerError, NewErrorResponse("Failed to get table columns", err))
		return
	}

	WriteJSONResponse(w, http.StatusOK, NewSuccessResponse("Retrieved columns successfully", columns, len(columns)))
}

// handleGetFlatFileSchema retrieves the schema from a flat file
func handleGetFlatFileSchema(w http.ResponseWriter, r *http.Request) {
	var config FlatFileConfig
	if err := ReadJSONBody(r, &config); err != nil {
		WriteJSONResponse(w, http.StatusBadRequest, NewErrorResponse("Invalid request body", err))
		return
	}

	client := NewFlatFileClient(config)

	// Validate file exists and is readable
	if err := client.ValidateFile(); err != nil {
		WriteJSONResponse(w, http.StatusBadRequest, NewErrorResponse("File validation failed", err))
		return
	}

	columns, err := client.GetSchema()
	if err != nil {
		WriteJSONResponse(w, http.StatusInternalServerError, NewErrorResponse("Failed to get file schema", err))
		return
	}

	WriteJSONResponse(w, http.StatusOK, NewSuccessResponse("Retrieved file schema successfully", columns, len(columns)))
}

// handlePreviewData previews data from either source
func handlePreviewData(w http.ResponseWriter, r *http.Request) {
	var req IngestionRequest
	if err := ReadJSONBody(r, &req); err != nil {
		WriteJSONResponse(w, http.StatusBadRequest, NewErrorResponse("Invalid request body", err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	limit := req.PreviewLimit
	if limit <= 0 {
		limit = 100 // Default preview limit
	}

	var data []map[string]interface{}
	var err error

	switch req.Source {
	case SourceClickHouse:
		client, err := NewClickHouseClient(req.ClickHouseConf)
		if err != nil {
			WriteJSONResponse(w, http.StatusInternalServerError, NewErrorResponse("Failed to connect to ClickHouse", err))
			return
		}
		defer client.Close()

		// Check if it's a join operation
		if len(req.SelectedTables) > 1 && req.JoinCondition != "" {
			data, err = client.JoinTables(ctx, req.SelectedTables, req.JoinCondition, req.SelectedColumns, limit)
		} else {
			tableName := req.TableName
			if tableName == "" && len(req.SelectedTables) > 0 {
				tableName = req.SelectedTables[0]
			}
			data, err = client.FetchData(ctx, tableName, req.SelectedColumns, limit)
		}

	case SourceFlatFile:
		client := NewFlatFileClient(req.FlatFileConf)
		data, err = client.PreviewData(limit)

	default:
		WriteJSONResponse(w, http.StatusBadRequest, NewErrorResponse("Invalid source type", nil))
		return
	}

	if err != nil {
		WriteJSONResponse(w, http.StatusInternalServerError, NewErrorResponse("Failed to preview data", err))
		return
	}

	WriteJSONResponse(w, http.StatusOK, NewSuccessResponse("Data preview successful", data, len(data)))
}

// handleIngestion handles the data ingestion process
func handleIngestion(w http.ResponseWriter, r *http.Request) {
	var req IngestionRequest
	if err := ReadJSONBody(r, &req); err != nil {
		WriteJSONResponse(w, http.StatusBadRequest, NewErrorResponse("Invalid request body", err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var sourceData []map[string]interface{}
	var err error
	var recordCount int

	// Step 1: Fetch data from source
	switch req.Source {
	case SourceClickHouse:
		// Connect to ClickHouse source
		sourceClient, err := NewClickHouseClient(req.ClickHouseConf)
		if err != nil {
			WriteJSONResponse(w, http.StatusInternalServerError, NewErrorResponse("Failed to connect to ClickHouse source", err))
			return
		}
		defer sourceClient.Close()

		// Check if it's a join operation
		if len(req.SelectedTables) > 1 && req.JoinCondition != "" {
			sourceData, err = sourceClient.JoinTables(ctx, req.SelectedTables, req.JoinCondition, req.SelectedColumns, 0)
		} else {
			tableName := req.TableName
			if tableName == "" && len(req.SelectedTables) > 0 {
				tableName = req.SelectedTables[0]
			}
			sourceData, err = sourceClient.FetchData(ctx, tableName, req.SelectedColumns, 0)
		}

		if err != nil {
			WriteJSONResponse(w, http.StatusInternalServerError, NewErrorResponse("Failed to fetch data from ClickHouse", err))
			return
		}

	case SourceFlatFile:
		// Connect to flat file source
		sourceClient := NewFlatFileClient(req.FlatFileConf)
		sourceData, err = sourceClient.ReadData(req.SelectedColumns)
		if err != nil {
			WriteJSONResponse(w, http.StatusInternalServerError, NewErrorResponse("Failed to read data from flat file", err))
			return
		}

	default:
		WriteJSONResponse(w, http.StatusBadRequest, NewErrorResponse("Invalid source type", nil))
		return
	}

	// If preview only, return the data without ingestion
	if req.PreviewOnly {
		limit := req.PreviewLimit
		if limit <= 0 || limit > len(sourceData) {
			limit = len(sourceData)
		}
		WriteJSONResponse(w, http.StatusOK, NewSuccessResponse("Data preview successful", sourceData[:limit], len(sourceData)))
		return
	}

	// Step 2: Write data to target
	switch req.Target {
	case SourceClickHouse:
		// Connect to ClickHouse target
		targetClient, err := NewClickHouseClient(req.ClickHouseConf)
		if err != nil {
			WriteJSONResponse(w, http.StatusInternalServerError, NewErrorResponse("Failed to connect to ClickHouse target", err))
			return
		}
		defer targetClient.Close()

		recordCount, err = targetClient.ImportDataFromFlatFile(ctx, req.TableName, sourceData)
		if err != nil {
			WriteJSONResponse(w, http.StatusInternalServerError, NewErrorResponse("Failed to import data to ClickHouse", err))
			return
		}

	case SourceFlatFile:
		// Write to flat file target
		targetClient := NewFlatFileClient(req.FlatFileConf)
		recordCount, err = targetClient.WriteData(sourceData, req.SelectedColumns)
		if err != nil {
			WriteJSONResponse(w, http.StatusInternalServerError, NewErrorResponse("Failed to write data to flat file", err))
			return
		}

	default:
		WriteJSONResponse(w, http.StatusBadRequest, NewErrorResponse("Invalid target type", nil))
		return
	}

	WriteJSONResponse(w, http.StatusOK, NewSuccessResponse("Data ingestion completed successfully", nil, recordCount))
}

// enableCORS enables CORS for all requests
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// setupRoutes sets up the API routes
func setupRoutes() http.Handler {
	mux := http.NewServeMux()

	// ClickHouse routes
	mux.HandleFunc("/api/clickhouse/tables", handleGetClickHouseTables)
	mux.HandleFunc("/api/clickhouse/columns", handleGetClickHouseColumns)

	// Flat file routes
	mux.HandleFunc("/api/flatfile/schema", handleGetFlatFileSchema)

	// Data preview and ingestion routes
	mux.HandleFunc("/api/preview", handlePreviewData)
	mux.HandleFunc("/api/ingest", handleIngestion)

	// Static file server for frontend
	fs := http.FileServer(http.Dir("./frontend/build"))
	mux.Handle("/", fs)

	return enableCORS(mux)
}
