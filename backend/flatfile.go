package main

import (
	// "bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	// "strings"
)

// FlatFileConfig holds configuration for flat file operations
type FlatFileConfig struct {
	FileName  string `json:"fileName"`
	Delimiter string `json:"delimiter"`
	HasHeader bool   `json:"hasHeader"`
}

// FlatFileClient manages flat file operations
type FlatFileClient struct {
	config FlatFileConfig
}

// NewFlatFileClient creates a new flat file client
func NewFlatFileClient(config FlatFileConfig) *FlatFileClient {
	// Default delimiter is comma if not specified
	if config.Delimiter == "" {
		config.Delimiter = ","
	}
	
	return &FlatFileClient{
		config: config,
	}
}

// GetHeaders reads the header row from a flat file
func (f *FlatFileClient) GetHeaders() ([]string, error) {
	file, err := os.Open(f.config.FileName)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = []rune(f.config.Delimiter)[0]

	// Read header row
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read headers: %w", err)
	}

	return headers, nil
}

// GetSchema infers the schema from the first few rows
func (f *FlatFileClient) GetSchema() ([]Column, error) {
	file, err := os.Open(f.config.FileName)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = []rune(f.config.Delimiter)[0]

	// Read header row
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read headers: %w", err)
	}

	// Read a few rows to infer types
	var columns []Column
	for _, header := range headers {
		columns = append(columns, Column{
			Name: header,
			Type: "String", // Default type
		})
	}

	return columns, nil
}

// ReadData reads all data from the flat file
func (f *FlatFileClient) ReadData(selectedColumns []string) ([]map[string]interface{}, error) {
	file, err := os.Open(f.config.FileName)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = []rune(f.config.Delimiter)[0]

	// Read headers
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read headers: %w", err)
	}

	// Map column indices for selected columns
	colIndices := make(map[string]int)
	if len(selectedColumns) == 0 {
		// If no columns specified, select all
		selectedColumns = headers
		for i, header := range headers {
			colIndices[header] = i
		}
	} else {
		// Map only selected columns
		for i, header := range headers {
			for _, selectedCol := range selectedColumns {
				if header == selectedCol {
					colIndices[header] = i
					break
				}
			}
		}
	}

	// Read data rows
	var data []map[string]interface{}
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading file: %w", err)
		}

		row := make(map[string]interface{})
		for col, idx := range colIndices {
			if idx < len(record) {
				row[col] = record[idx]
			}
		}
		data = append(data, row)
	}

	return data, nil
}

// PreviewData reads the first n rows from the flat file
func (f *FlatFileClient) PreviewData(limit int) ([]map[string]interface{}, error) {
	file, err := os.Open(f.config.FileName)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = []rune(f.config.Delimiter)[0]

	// Read headers
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read headers: %w", err)
	}

	// Read data rows up to limit
	var data []map[string]interface{}
	for count := 0; count < limit; count++ {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading file: %w", err)
		}

		row := make(map[string]interface{})
		for i, value := range record {
			if i < len(headers) {
				row[headers[i]] = value
			}
		}
		data = append(data, row)
	}

	return data, nil
}

// WriteData writes data to a flat file
func (f *FlatFileClient) WriteData(data []map[string]interface{}, selectedColumns []string) (int, error) {
	file, err := os.Create(f.config.FileName)
	if err != nil {
		return 0, fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Comma = []rune(f.config.Delimiter)[0]
	defer writer.Flush()

	// If no columns specified, extract from first data row
	if len(selectedColumns) == 0 && len(data) > 0 {
		for col := range data[0] {
			selectedColumns = append(selectedColumns, col)
		}
	}

	// Write header
	if err := writer.Write(selectedColumns); err != nil {
		return 0, fmt.Errorf("failed to write header: %w", err)
	}

	// Write data rows
	recordCount := 0
	for _, row := range data {
		record := make([]string, len(selectedColumns))
		for i, col := range selectedColumns {
			val, ok := row[col]
			if ok {
				record[i] = fmt.Sprintf("%v", val)
			} else {
				record[i] = "" // Empty value for missing columns
			}
		}

		if err := writer.Write(record); err != nil {
			return recordCount, fmt.Errorf("failed to write record: %w", err)
		}
		recordCount++
	}

	return recordCount, nil
}

// ValidateFile checks if the file exists and is readable
func (f *FlatFileClient) ValidateFile() error {
	_, err := os.Stat(f.config.FileName)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", f.config.FileName)
		}
		return fmt.Errorf("error accessing file: %w", err)
	}

	// Try to open the file to ensure it's readable
	file, err := os.Open(f.config.FileName)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer file.Close()

	return nil
}