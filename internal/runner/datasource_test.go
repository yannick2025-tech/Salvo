package runner

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCSV(t *testing.T) {
	csv := `name,age,city
Alice,30,NYC
Bob,25,LA`
	columns, rows, removed, err := ParseCSV("users.csv", strings.NewReader(csv))
	require.NoError(t, err)
	assert.Equal(t, []string{"name", "age", "city"}, columns)
	require.Len(t, rows, 2)
	assert.Equal(t, "Alice", rows[0]["name"])
	assert.Equal(t, "30", rows[0]["age"])
	assert.Equal(t, "Bob", rows[1]["name"])
	assert.Equal(t, 0, removed)
}

func TestParseCSVEmptyFile(t *testing.T) {
	_, _, _, err := ParseCSV("empty.csv", strings.NewReader(""))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestParseCSVDuplicateColumns(t *testing.T) {
	csv := `name,age,name
Alice,30,Bob`
	_, _, _, err := ParseCSV("dup.csv", strings.NewReader(csv))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestParseCSVInvalidFileName(t *testing.T) {
	csv := `a,b
1,2`
	_, _, _, err := ParseCSV("my-file.csv", strings.NewReader(csv))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file name")
}

func TestParseCSVRemovesEmptyRows(t *testing.T) {
	csv := `name,age,city
Alice,30,NYC
,,
Bob,25,LA
,,`
	columns, rows, removed, err := ParseCSV("users.csv", strings.NewReader(csv))
	require.NoError(t, err)
	assert.Equal(t, []string{"name", "age", "city"}, columns)
	require.Len(t, rows, 2)
	assert.Equal(t, "Alice", rows[0]["name"])
	assert.Equal(t, "Bob", rows[1]["name"])
	assert.Equal(t, 2, removed, "should remove 2 empty rows")
}

func TestParseCSVPartialEmptyRow(t *testing.T) {
	csv := `name,age,city
Alice,30,
,25,LA
,,`
	columns, rows, removed, err := ParseCSV("users.csv", strings.NewReader(csv))
	require.NoError(t, err)
	_ = columns
	require.Len(t, rows, 2, "partial empty rows should be kept")
	assert.Equal(t, "Alice", rows[0]["name"])
	assert.Equal(t, "25", rows[1]["age"])
	assert.Equal(t, 1, removed, "only fully empty row should be removed")
}

func TestRowIterator(t *testing.T) {
	rows := []map[string]string{
		{"name": "Alice", "age": "30"},
		{"name": "Bob", "age": "25"},
		{"name": "Charlie", "age": "35"},
	}
	it := NewRowIterator(rows)

	// First iteration
	row := it.Next()
	assert.Equal(t, "Alice", row["name"])

	row = it.Next()
	assert.Equal(t, "Bob", row["name"])

	row = it.Next()
	assert.Equal(t, "Charlie", row["name"])

	// Should wrap around
	row = it.Next()
	assert.Equal(t, "Alice", row["name"])
}

func TestRowIteratorEmpty(t *testing.T) {
	it := NewRowIterator(nil)
	row := it.Next()
	assert.Empty(t, row)
}

func TestToDataSourceModel(t *testing.T) {
	columns := []string{"name", "age"}
	rows := []map[string]string{
		{"name": "Alice", "age": "30"},
	}
	ds := ToDataSourceModel(123, "users.csv", columns, rows)
	assert.Equal(t, "users", ds.Name)
	assert.Equal(t, "users.csv", ds.FileName)
	assert.Equal(t, 1, ds.RowCount)
	assert.Contains(t, ds.Columns, "name")
	assert.Contains(t, ds.Rows, "Alice")
}
