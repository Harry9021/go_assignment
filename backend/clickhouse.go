package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// ClickHouseConfig holds connection details for ClickHouse
type ClickHouseConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	JWTToken string `json:"jwtToken"`
	IsHTTPS  bool   `json:"isHttps"`
}

// ClickHouseClient wraps a ClickHouse connection
type ClickHouseClient struct {
	conn driver.Conn
	db   *sql.DB
}

// NewClickHouseClient creates a new client using JWT authentication
func NewClickHouseClient(config ClickHouseConfig) (*ClickHouseClient, error) {
	// Define protocol based on IsHTTPS
	// protocol := "http"
	// if config.IsHTTPS {
	//     protocol = "https"
	// }

	// Define the connection options for the ClickHouse client
	options := &clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%s", config.Host, config.Port)}, // Replace with your host and port
		Auth: clickhouse.Auth{
			Database: config.Database,
			Username: config.Username,
			Password: config.JWTToken, // Use the password provided in the config
		},
		ClientInfo: clickhouse.ClientInfo{
			Products: []struct {
				Name    string
				Version string
			}{
				{Name: "go-client", Version: "0.1"},
			},
		},
		Debugf: func(format string, v ...interface{}) {
			fmt.Printf(format, v)
		},
		TLS: &tls.Config{
			InsecureSkipVerify: true, // Skip verification for simplicity
		},
	}

	// Establish the connection
	conn, err := clickhouse.Open(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create clickhouse connection: %w", err)
	}

	// Test the connection with a simple ping
	ctx := context.Background()
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping ClickHouse: %w", err)
	}
	log.Println("Successfully connected to ClickHouse!")

	// Successfully connected, return the client
	return &ClickHouseClient{
		conn: conn,
	}, nil
}

// Close releases all resources
func (c *ClickHouseClient) Close() error {
	if c.conn != nil {
		err := c.conn.Close()
		if err != nil {
			return err
		}
	}
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

func (c *ClickHouseClient) GetTables(ctx context.Context) ([]string, error) {
	// Execute the query directly using the native protocol connection
	rows, err := c.conn.Query(ctx, "SHOW TABLES;")
	if err != nil {
		log.Printf("Error querying tables: %v", err)
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	// Iterate through the result rows
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, tableName)
	}

	return tables, nil
}

// TableInfo holds column metadata
type TableInfo struct {
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`
}

// Column represents a column in a table
type Column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// GetTableColumns returns the columns of a table
func (c *ClickHouseClient) GetTableColumns(ctx context.Context, tableName string) ([]Column, error) {
	query := fmt.Sprintf("DESCRIBE TABLE %s", tableName)
	rows, err := c.conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to describe table %s: %w", tableName, err)
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var name, typ, defaultType, defaultExpression string
		var comment, codecExpression, ttlExpression sql.NullString
		if err := rows.Scan(&name, &typ, &defaultType, &defaultExpression, &comment, &codecExpression, &ttlExpression); err != nil {
			return nil, fmt.Errorf("failed to scan column info: %w", err)
		}
		columns = append(columns, Column{
			Name: name,
			Type: typ,
		})
	}

	return columns, nil
}

func (c *ClickHouseClient) ValidateColumns(ctx context.Context, tableName string, requestedColumns []string) ([]string, error) {
	// Get all available columns for the table
	query := fmt.Sprintf("DESCRIBE TABLE %s", tableName)
	rows, err := c.conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to describe table: %w", err)
	}
	defer rows.Close()

	availableColumns := make(map[string]bool)
	for rows.Next() {
		var name, typ, defaultType, defaultExpression string
		var comment, codecExpression, ttlExpression sql.NullString
		if err := rows.Scan(&name, &typ, &defaultType, &defaultExpression, &comment, &codecExpression, &ttlExpression); err != nil {
			return nil, fmt.Errorf("failed to scan column info: %w", err)
		}
		availableColumns[name] = true
	}

	// Validate requested columns
	var validColumns []string
	var missingColumns []string
	for _, col := range requestedColumns {
		if availableColumns[col] {
			validColumns = append(validColumns, col)
		} else {
			missingColumns = append(missingColumns, col)
		}
	}

	if len(missingColumns) > 0 {
		return validColumns, fmt.Errorf("columns not found in table %s: %s", tableName, strings.Join(missingColumns, ", "))
	}

	return validColumns, nil
}

// FetchData retrieves data from a table with selected columns
func (c *ClickHouseClient) FetchData(ctx context.Context, tableName string, selectedColumns []string, limit int) ([]map[string]interface{}, error) {
	if len(selectedColumns) == 0 {
		return nil, errors.New("no columns selected")
	}

	// Validate columns before executing query
	validColumns, err := c.ValidateColumns(ctx, tableName, selectedColumns)
	if err != nil {
		return nil, err
	}

	if len(validColumns) == 0 {
		return nil, errors.New("no valid columns to select")
	}

	columnsStr := strings.Join(selectedColumns, ", ")

	// Base query
	query := fmt.Sprintf("SELECT %s FROM %s", columnsStr, tableName)

	// Add LIMIT before SETTINGS
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	// Add SETTINGS at the end
	query += " SETTINGS allow_introspection_functions=1"

	rows, err := c.conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// Get column types
	columnTypes := rows.ColumnTypes()

	results := []map[string]interface{}{}
	for rows.Next() {
		// Create a slice of properly typed values based on column types
		rowValues := make([]interface{}, len(selectedColumns))
		for i, ct := range columnTypes {
			// Create appropriate type based on database type
			typeName := ct.DatabaseTypeName()
			scanType := ct.ScanType()

			// Check if it's a Nullable type
			// isNullable := strings.Contains(typeName, "Nullable")

			switch {
			case strings.Contains(typeName, "UInt8"):
				var val uint8
				rowValues[i] = &val
			case strings.Contains(typeName, "UInt16"):
				var val uint16
				rowValues[i] = &val
			case strings.Contains(typeName, "UInt32"):
				var val uint32
				rowValues[i] = &val
			case strings.Contains(typeName, "UInt64"):
				var val uint64
				rowValues[i] = &val
			case strings.Contains(typeName, "Int8"):
				var val int8
				rowValues[i] = &val
			case strings.Contains(typeName, "Int16"):
				var val int16
				rowValues[i] = &val
			case strings.Contains(typeName, "Int32"):
				var val int32
				rowValues[i] = &val
			case strings.Contains(typeName, "Int64"):
				var val int64
				rowValues[i] = &val
			case strings.Contains(typeName, "UUID"):
				var val string // UUIDs are typically handled as strings
				rowValues[i] = &val
			case strings.Contains(typeName, "DateTime") || strings.Contains(typeName, "Date"):
				var val time.Time
				rowValues[i] = &val
			case scanType.Kind() == reflect.Float64 || strings.Contains(typeName, "Float64"):
				var val float64
				rowValues[i] = &val
			case scanType.Kind() == reflect.Float32 || strings.Contains(typeName, "Float32"):
				var val float32
				rowValues[i] = &val
			case scanType.Kind() == reflect.String || strings.Contains(typeName, "String"):
				var val string
				rowValues[i] = &val
			case strings.Contains(typeName, "Array"):
				var val []interface{} // Handle arrays as slice of interfaces
				rowValues[i] = &val
			case strings.Contains(typeName, "Decimal"):
				var val float64 // Handle decimals as float64
				rowValues[i] = &val
			case strings.Contains(typeName, "Bool") || strings.Contains(typeName, "Boolean"):
				var val bool
				rowValues[i] = &val
			default:
				// For other types, use a string as a fallback
				var val string
				rowValues[i] = &val
			}
		}

		if err := rows.Scan(rowValues...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Create map for this row
		rowMap := make(map[string]interface{})
		for i, col := range selectedColumns {
			// Dereference the pointer to get the actual value
			switch v := rowValues[i].(type) {
			case *uint8:
				rowMap[col] = *v
			case *uint16:
				rowMap[col] = *v
			case *uint32:
				rowMap[col] = *v
			case *uint64:
				rowMap[col] = *v
			case *int8:
				rowMap[col] = *v
			case *int16:
				rowMap[col] = *v
			case *int32:
				rowMap[col] = *v
			case *int64:
				rowMap[col] = *v
			case *float32:
				rowMap[col] = *v
			case *float64:
				rowMap[col] = *v
			case *string:
				rowMap[col] = *v
			case *bool:
				rowMap[col] = *v
			case *time.Time:
				rowMap[col] = *v
			case *[]interface{}:
				rowMap[col] = *v
			case nil:
				rowMap[col] = nil
			default:
				rowMap[col] = v
			}
		}
		results = append(results, rowMap)
	}

	return results, nil
}

// TableExists checks if a table exists in the database
func (c *ClickHouseClient) TableExists(ctx context.Context, tableName string) (bool, error) {
	query := fmt.Sprintf("EXISTS TABLE %s", tableName)
	rows, err := c.conn.Query(ctx, query)
	if err != nil {
		return false, fmt.Errorf("failed to check if table exists: %w", err)
	}
	defer rows.Close()

	var exists uint8
	if rows.Next() {
		if err := rows.Scan(&exists); err != nil {
			return false, fmt.Errorf("failed to scan result: %w", err)
		}
	}

	return exists == 1, nil
}

func sanitizeColumnName(name string) string {
	// Replace spaces and special characters with underscores
	sanitized := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, name)

	// Ensure it starts with a letter or underscore
	if len(sanitized) > 0 && (sanitized[0] >= '0' && sanitized[0] <= '9') {
		sanitized = "_" + sanitized
	}

	// If empty, use a default name
	if sanitized == "" {
		sanitized = "column"
	}

	return sanitized
}

// CreateTable creates a new table based on the provided schema
func (c *ClickHouseClient) CreateTable(ctx context.Context, tableName string, columns []Column) error {
	// Build column definitions with proper escaping
	columnDefs := make([]string, len(columns))
	for i, col := range columns {
		// Escape column names that contain spaces or special characters
		escapedName := sanitizeColumnName(col.Name)
		columnDefs[i] = fmt.Sprintf("%s %s", escapedName, col.Type)
	}

	// Create table query
	query := fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s (%s) ENGINE = MergeTree() ORDER BY tuple()",
		tableName,
		strings.Join(columnDefs, ", "))

	// Log the query for debugging
	log.Printf("Creating table with query: %s", query)

	err := c.conn.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

// ImportDataFromFlatFile imports data from a flat file to ClickHouse
func (c *ClickHouseClient) ImportDataFromFlatFile(ctx context.Context, tableName string, data []map[string]interface{}) (int, error) {
	if len(data) == 0 {
		return 0, errors.New("no data to import")
	}

	// Get all column names from the first row
	columns := make([]string, 0, len(data[0]))
	for col := range data[0] {
		columns = append(columns, col)
	}

	log.Printf("table name: %s", tableName)

	// Check if table exists
	exists, err := c.TableExists(ctx, tableName)
	if err != nil {
		return 0, fmt.Errorf("failed to check if table exists: %w", err)
	}

	// If table doesn't exist, create it
	if !exists {
		log.Printf("Table %s does not exist. Creating it...", tableName)

		// Infer column types from the data
		tableColumns := make([]Column, len(columns))
		for i, col := range columns {
			// Default to String type, but you can improve this by inferring types from data
			colType := "String"

			// Try to infer type from the first non-nil value
			for _, row := range data {
				if val := row[col]; val != nil {
					switch val.(type) {
					case int, int8, int16, int32, int64:
						colType = "Int64"
					case uint, uint8, uint16, uint32, uint64:
						colType = "UInt64"
					case float32, float64:
						colType = "Float64"
					case bool:
						colType = "UInt8" // ClickHouse uses UInt8 for boolean
					case time.Time:
						colType = "DateTime"
					case []interface{}:
						colType = "Array(String)"
					}
					break
				}
			}

			tableColumns[i] = Column{Name: col, Type: colType}
		}

		// Create the table
		if err := c.CreateTable(ctx, tableName, tableColumns); err != nil {
			return 0, err
		}
	}

	// Create the query
	query := fmt.Sprintf("INSERT INTO %s", tableName)

	// Log the query for debugging
	log.Println("Preparing batch with query:", query)

	// Prepare the batch statement using the native connection
	batch, err := c.conn.PrepareBatch(ctx, query)
	if err != nil {
		log.Println("Error preparing batch:", err)
		return 0, fmt.Errorf("failed to prepare batch: %w", err)
	}

	// Insert data in batches
	recordCount := 0
	for _, row := range data {
		// Create values array in the same order as columns
		values := make([]interface{}, len(columns))
		for i, col := range columns {
			values[i] = row[col]
		}

		// Add the row to the batch
		if err := batch.Append(values...); err != nil {
			log.Println("Error appending row to batch:", err)
			return recordCount, fmt.Errorf("failed to append row to batch: %w", err)
		}
		recordCount++
	}

	// Send the batch to the server
	if err := batch.Send(); err != nil {
		log.Println("Error sending batch:", err)
		return recordCount, fmt.Errorf("failed to send batch: %w", err)
	}

	return recordCount, nil
}

// JoinTables executes a join query between multiple tables
func (c *ClickHouseClient) JoinTables(ctx context.Context, tables []string, joinConditions string, selectedColumns []string, limit int) ([]map[string]interface{}, error) {
	if len(tables) < 2 {
		return nil, errors.New("at least two tables are required for a join")
	}
	if len(selectedColumns) == 0 {
		return nil, errors.New("no columns selected")
	}

	columnsStr := strings.Join(selectedColumns, ", ")
	query := fmt.Sprintf("SELECT %s FROM %s JOIN %s ON %s",
		columnsStr,
		tables[0],
		strings.Join(tables[1:], " JOIN "),
		joinConditions)

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	query += " SETTINGS allow_introspection_functions=1"

	rows, err := c.conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute join query: %w", err)
	}
	defer rows.Close()

	results := []map[string]interface{}{}
	for rows.Next() {
		// Create a slice of interface{} to hold the values
		rowValues := make([]interface{}, len(selectedColumns))
		rowValuePtrs := make([]interface{}, len(selectedColumns))
		for i := range rowValues {
			rowValuePtrs[i] = &rowValues[i]
		}

		if err := rows.Scan(rowValuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Create map for this row
		rowMap := make(map[string]interface{})
		for i, col := range selectedColumns {
			rowMap[col] = rowValues[i]
		}
		results = append(results, rowMap)
	}

	return results, nil
}
