package runner

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/yannick2025-tech/Salvo/internal/pkg/snowflake"
	"github.com/yannick2025-tech/Salvo/internal/store/model"
)

// maxFileSizeBytes is the maximum allowed CSV file size (10MB).
const maxFileSizeBytes = 10 * 1024 * 1024

// validFileNamePattern matches valid data source file names (letters, digits,
// underscores only).
var validFileNamePattern = strings.NewReplacer(
	"[", "",
	"]",
	"",
)

// ParseCSV parses a CSV file content and returns the column names and row data.
// It validates the file size, file name format, and detects duplicate columns.
func ParseCSV(fileName string, reader io.Reader) ([]string, []map[string]string, error) {
	content, err := io.ReadAll(io.LimitReader(reader, maxFileSizeBytes+1))
	if err != nil {
		return nil, nil, fmt.Errorf("read csv file: %w", err)
	}
	if len(content) > maxFileSizeBytes {
		return nil, nil, fmt.Errorf("file size exceeds %d byte limit", maxFileSizeBytes)
	}

	if !isValidFileName(fileName) {
		return nil, nil, fmt.Errorf("file name must contain only letters, digits, and underscores")
	}

	csvReader := csv.NewReader(strings.NewReader(string(content)))
	csvReader.FieldsPerRecord = -1 // allow variable-length records

	headers, err := csvReader.Read()
	if err == io.EOF {
		return nil, nil, fmt.Errorf("csv file is empty")
	}
	if err != nil {
		return nil, nil, fmt.Errorf("parse csv headers: %w", err)
	}
	if len(headers) == 0 {
		return nil, nil, fmt.Errorf("csv has no columns")
	}

	// Check duplicate column names
	headerSet := make(map[string]bool)
	for _, h := range headers {
		if headerSet[h] {
			return nil, nil, fmt.Errorf("duplicate column name detected: %q", h)
		}
		headerSet[h] = true
	}

	var rows []map[string]string
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("parse csv record %d: %w", len(rows)+1, err)
		}

		row := make(map[string]string, len(headers))
		for i, h := range headers {
			if i < len(record) {
				row[h] = record[i]
			} else {
				row[h] = ""
			}
		}
		rows = append(rows, row)
	}

	return headers, rows, nil
}

// isValidFileName checks that the file name is non-empty and has a .csv extension.
// The base name (without extension) must contain only letters, digits, and underscores.
func isValidFileName(name string) bool {
	if name == "" {
		return false
	}
	base := strings.TrimSuffix(name, ".csv")
	if base == "" {
		return false
	}
	for _, r := range base {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

// ToDataSourceModel converts parsed CSV data into a DataSource model ready
// for persistence.
func ToDataSourceModel(sceneID snowflake.ID, fileName string, columns []string, rows []map[string]string) *model.DataSource {
	columnsJSON, _ := json.Marshal(columns)
	rowsJSON, _ := json.Marshal(rows)

	name := strings.TrimSuffix(fileName, ".csv")

	return &model.DataSource{
		SceneID:   sceneID,
		Name:      name,
		FileName:  fileName,
		Columns:   string(columnsJSON),
		Rows:      string(rowsJSON),
		RowCount:  len(rows),
		Source:    "csv",
	}
}

// RowIterator provides thread-safe sequential access to rows of a data source.
// When all rows are consumed, it wraps around to the first row.
type RowIterator struct {
	mu       sync.Mutex
	rows     []map[string]string
	index    atomic.Int64
	rowCount int64
}

// NewRowIterator creates a new RowIterator from the given rows.
func NewRowIterator(rows []map[string]string) *RowIterator {
	it := &RowIterator{
		rows:     rows,
		rowCount: int64(len(rows)),
	}
	it.index.Store(0)
	return it
}

// Next returns the next row as a map of column name to value.
// When all rows are exhausted, it wraps around to the first row.
func (it *RowIterator) Next() map[string]string {
	if it.rowCount == 0 {
		return make(map[string]string)
	}
	idx := it.index.Add(1) - 1
	if idx >= it.rowCount {
		it.mu.Lock()
		current := it.index.Load()
		if current > it.rowCount {
			it.index.Store(1)
			idx = 0
		} else {
			idx = current - 1
		}
		it.mu.Unlock()
	}
	return it.rows[idx]
}

// Current returns the current row without advancing.
func (it *RowIterator) Current() map[string]string {
	if it.rowCount == 0 {
		return make(map[string]string)
	}
	idx := it.index.Load() - 1
	if idx < 0 || idx >= it.rowCount {
		return it.rows[0]
	}
	return it.rows[idx]
}

// RowCount returns the total number of rows in this iterator.
func (it *RowIterator) RowCount() int {
	return len(it.rows)
}
