package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

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
	protocol := "http"
	if config.IsHTTPS {
		protocol = "https"
	}

	log.Printf("hello 1",config)

	options := &clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%s", config.Host, config.Port)},
		Auth: clickhouse.Auth{
			Database: config.Database,
			Username: config.Username,
			Password: config.JWTToken,
		},
		Protocol:       clickhouse.HTTP,
		MaxOpenConns:   10,
		MaxIdleConns:   5,
		ConnMaxLifetime: 3600,
	}

	if config.IsHTTPS {
		// Use the standard tls.Config for TLS configuration
		options.TLS = &tls.Config{
			InsecureSkipVerify: true, // Skip verification for simplicity
		}
	}

	conn, err := clickhouse.Open(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create clickhouse connection: %w", err)
	}

	// Test connection
	if err := conn.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping clickhouse: %w", err)
	}

	// Create SQL DB connection
	dsn := fmt.Sprintf("%s://%s:%s/?database=%s&username=%s&access_token=%s",
		protocol, config.Host, config.Port, config.Database, config.Username, config.JWTToken)
	if config.IsHTTPS {
		dsn += "&secure=true"
	}

	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open clickhouse SQL connection: %w", err)
	}

	log.Printf("Connected to ClickHouse at %s:%s", config.Host, config.Port)

	return &ClickHouseClient{
		conn: conn,
		db:   db,
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

// GetTables returns a list of all tables in the database
func (c *ClickHouseClient) GetTables(ctx context.Context) ([]string, error) {
	rows, err := c.db.QueryContext(ctx, "SHOW TABLES")
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []string
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
	rows, err := c.db.QueryContext(ctx, query)
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

// FetchData retrieves data from a table with selected columns
func (c *ClickHouseClient) FetchData(ctx context.Context, tableName string, selectedColumns []string, limit int) ([]map[string]interface{}, error) {
	if len(selectedColumns) == 0 {
		return nil, errors.New("no columns selected")
	}

	columnsStr := strings.Join(selectedColumns, ", ")
	query := fmt.Sprintf("SELECT %s FROM %s", columnsStr, tableName)
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
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

	// Begin transaction
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create placeholders and query
	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = "?"
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		tableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Insert data in batches
	recordCount := 0
	for _, row := range data {
		values := make([]interface{}, len(columns))
		for i, col := range columns {
			values[i] = row[col]
		}

		if _, err := stmt.ExecContext(ctx, values...); err != nil {
			return recordCount, fmt.Errorf("failed to insert row: %w", err)
		}
		recordCount++
	}

	if err := tx.Commit(); err != nil {
		return recordCount, fmt.Errorf("failed to commit transaction: %w", err)
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

	rows, err := c.db.QueryContext(ctx, query)
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