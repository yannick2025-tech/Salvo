package expr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvalMath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
		errMsg  string
	}{
		// Basic operations
		{name: "addition", input: "10 + 20", want: "30"},
		{name: "subtraction", input: "50 - 20", want: "30"},
		{name: "multiplication", input: "6 * 7", want: "42"},
		{name: "division", input: "100 / 4", want: "25"},
		{name: "mixed ops", input: "60 * 50 / 100", want: "30"},
		{name: "operator precedence", input: "2 + 3 * 4", want: "14"},
		{name: "precedence with division", input: "10 - 20 / 4", want: "5"},

		// Parentheses
		{name: "parentheses", input: "(10 + 20) * 3", want: "90"},
		{name: "nested parentheses", input: "((2 + 3) * (4 + 5))", want: "45"},
		{name: "parentheses override precedence", input: "(2 + 3) * 4", want: "20"},

		// Floats
		{name: "float result", input: "1.5 * 2", want: "3"},
		{name: "float division", input: "10.0 / 3.0", want: "3.3333333333333335"},
		{name: "float precision", input: "0.1 + 0.2", want: "0.30000000000000004"},
		{name: "float multiplication", input: "1.5 * 2.5", want: "3.75"},

		// Negative numbers
		{name: "negative result", input: "5 - 10", want: "-5"},
		{name: "negative operand", input: "-5 + 10", want: "5"},
		{name: "double negative", input: "10 - -5", want: "15"},
		{name: "negative in parentheses", input: "(-5 + 10) * 2", want: "10"},

		// Whitespace
		{name: "no spaces", input: "10+20", want: "30"},
		{name: "extra spaces", input: "  10  +  20  ", want: "30"},
		{name: "tabs", input: "10\t+\t20", want: "30"},

		// Edge cases
		{name: "single number", input: "42", want: "42"},
		{name: "zero", input: "0", want: "0"},
		{name: "empty string", input: "", want: "", wantErr: false},
		{name: "whitespace only", input: "   ", want: "", wantErr: false},

		// Errors
		{name: "division by zero", input: "10 / 0", wantErr: true, errMsg: "division by zero"},
		{name: "invalid character", input: "10 # 5", wantErr: true, errMsg: "unexpected character"},
		{name: "incomplete expression", input: "10 +", wantErr: true, errMsg: "unexpected end"},
		{name: "unmatched open paren", input: "(10 + 20", wantErr: true, errMsg: "unmatched"},
		{name: "unmatched close paren", input: "10 + 20)", wantErr: true, errMsg: "unexpected"},
		{name: "double operator", input: "10 + + 20", wantErr: true, errMsg: "unexpected"},
		{name: "leading operator", input: "* 10", wantErr: true, errMsg: "unexpected"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EvalMath(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEvalMathLargeNumbers(t *testing.T) {
	got, err := EvalMath("1000000 * 1000000")
	require.NoError(t, err)
	assert.Equal(t, "1000000000000", got)
}

func TestEvalMathConcurrent(t *testing.T) {
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			got, err := EvalMath("2 + 3 * 4")
			assert.NoError(t, err)
			assert.Equal(t, "14", got)
			done <- true
		}()
	}
	for i := 0; i < 100; i++ {
		<-done
	}
}
