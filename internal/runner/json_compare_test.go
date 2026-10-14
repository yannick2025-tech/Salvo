package runner

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatJSONValue(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		// json.Number cases — the core fix for scientific notation
		{"json.Number large integer", json.Number("700000028"), "700000028"},
		{"json.Number small integer", json.Number("42"), "42"},
		{"json.Number negative integer", json.Number("-100"), "-100"},
		{"json.Number zero", json.Number("0"), "0"},
		{"json.Number float", json.Number("3.14"), "3.14"},
		{"json.Number very large", json.Number("9007199254740993"), "9007199254740993"},
		{"json.Number decimal", json.Number("0.001"), "0.001"},

		// float64 cases — expected values from config
		{"float64 integer", float64(700000028), "700000028"},
		{"float64 small", float64(42), "42"},
		{"float64 pi", float64(3.14), "3.14"},
		{"float64 zero", float64(0), "0"},
		{"float64 large power of 10", float64(1e8), "100000000"},

		// int cases
		{"int value", 42, "42"},
		{"int zero", 0, "0"},
		{"int negative", -7, "-7"},

		// int64 cases
		{"int64 value", int64(700000028), "700000028"},

		// string cases
		{"string value", "hello", "hello"},
		{"string empty", "", ""},

		// bool cases
		{"bool true", true, "true"},
		{"bool false", false, "false"},

		// nil
		{"nil value", nil, "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatJSONValue(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCompareJSONValues(t *testing.T) {
	tests := []struct {
		name     string
		actual   any
		expected any
		want     bool
	}{
		// The primary use case: json.Number from response vs float64 from config
		{"large int: json.Number vs float64", json.Number("700000028"), float64(700000028), true},
		{"small int: json.Number vs float64", json.Number("42"), float64(42), true},
		{"zero: json.Number vs float64", json.Number("0"), float64(0), true},
		{"negative: json.Number vs float64", json.Number("-1"), float64(-1), true},
		{"decimal: json.Number vs float64", json.Number("3.14"), float64(3.14), true},

		// json.Number vs json.Number
		{"both json.Number equal", json.Number("700000028"), json.Number("700000028"), true},
		{"both json.Number different", json.Number("700000028"), json.Number("700000029"), false},

		// json.Number vs int
		{"json.Number vs int", json.Number("42"), 42, true},

		// json.Number vs int64
		{"json.Number vs int64", json.Number("700000028"), int64(700000028), true},

		// string comparisons
		{"string equal", "hello", "hello", true},
		{"string different", "hello", "world", false},
		{"json.Number string vs string", json.Number("hello"), "hello", true},

		// bool comparisons
		{"bool equal", true, true, true},
		{"bool different", true, false, false},

		// nil comparisons
		{"nil equal", nil, nil, true},
		{"nil vs non-nil", nil, 0, false},

		// mismatch
		{"type mismatch number vs string", json.Number("42"), "42", true},  // both format to "42"
		{"type mismatch bool vs int", true, 1, false},

		// very large integer that would become scientific notation with float64
		{"very large integer", json.Number("9007199254740993"), json.Number("9007199254740993"), true},

		// float64 precision edge case: MaxInt53+1
		{"float64 max safe integer", json.Number("9007199254740991"), float64(math.MaxInt64), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareJSONValues(tt.actual, tt.expected)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestJSONDecoderUseNumberPreservesIntegers(t *testing.T) {
	// End-to-end test: decode a JSON body with UseNumber and verify
	// that large integers are preserved as json.Number strings,
	// not converted to float64 scientific notation.
	body := []byte(`{"errorCode":700000028,"message":"success","count":0,"price":3.14}`)

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// errorCode should be json.Number "700000028", not float64 7.00000028e+08
	errCode, ok := m["errorCode"]
	if !ok {
		t.Fatal("errorCode not found")
	}
	jn, isJSONNumber := errCode.(json.Number)
	assert.True(t, isJSONNumber, "errorCode should be json.Number, got %T", errCode)
	if isJSONNumber {
		assert.Equal(t, "700000028", jn.String())
	}

	// Verify compareJSONValues works end-to-end
	assert.True(t, compareJSONValues(m["errorCode"], float64(700000028)))
	assert.True(t, compareJSONValues(m["count"], float64(0)))
	assert.True(t, compareJSONValues(m["price"], float64(3.14)))
	assert.True(t, compareJSONValues(m["message"], "success"))
}
