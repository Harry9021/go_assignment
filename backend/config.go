package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// SourceType represents the data source type
type SourceType string

const (
	SourceClickHouse SourceType = "clickhouse"
	SourceFlatFile   SourceType = "flatfile"
)

// IngestionRequest holds the parameters for data ingestion
type IngestionRequest struct {
	Source         SourceType       `json:"source"`
	Target         SourceType       `json:"target"`
	ClickHouseConf ClickHouseConfig `json:"clickhouseConfig"`
	FlatFileConf   FlatFileConfig   `json:"flatFileConfig"`
	TableName      string           `json:"tableName"`
	SelectedTables []string         `json:"selectedTables"`
	JoinCondition  string           `json:"joinCondition"`
	SelectedColumns []string         `json:"selectedColumns"`
	PreviewOnly    bool             `json:"previewOnly"`
	PreviewLimit   int              `json:"previewLimit"`
}

// Response represents a standard API response
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Count   int         `json:"count,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// NewSuccessResponse creates a success response
func NewSuccessResponse(message string, data interface{}, count int) Response {
	return Response{
		Success: true,
		Message: message,
		Data:    data,
		Count:   count,
	}
}

// NewErrorResponse creates an error response
func NewErrorResponse(message string, err error) Response {
	errorMsg := ""
	if err != nil {
		errorMsg = err.Error()
	}
	
	return Response{
		Success: false,
		Message: message,
		Error:   errorMsg,
	}
}

// WriteJSONResponse writes a JSON response to the HTTP writer
func WriteJSONResponse(w http.ResponseWriter, statusCode int, resp Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// If encoding fails, write a simple error
		http.Error(w, fmt.Sprintf("Failed to encode response: %s", err), http.StatusInternalServerError)
	}
}

// ReadJSONBody reads and parses JSON from a request body
func ReadJSONBody(r *http.Request, v interface{}) error {
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	
	return decoder.Decode(v)
}