package expr

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/scanner"
)

// EvalMath evaluates a simple arithmetic expression string and returns the
// result as a string with appropriate precision. Supports +, -, *, /,
// parentheses, floats, and negative numbers.
func EvalMath(input string) (string, error) {
	input = strings.TrimSpace(input)
	if len(input) == 0 {
		return "", nil
	}

	val, err := parseMath(input)
	if err != nil {
		return "", err
	}

	// Format the result: if it's an integer, return as integer; otherwise
	// return with up to 16 significant digits, removing trailing zeros.
	s := formatFloat(val)
	return s, nil
}

// formatFloat formats a float64 value as a string.
func formatFloat(v float64) string {
	// Check if it's a whole number within integer precision.
	if v == float64(int64(v)) && v < 1<<53 {
		return strconv.FormatInt(int64(v), 10)
	}
	// Format with enough precision, removing trailing zeros.
	s := strconv.FormatFloat(v, 'f', -1, 64)
	return s
}

// token represents a math expression token.
type token int

const (
	tokNumber token = iota
	tokPlus
	tokMinus
	tokMul
	tokDiv
	tokLParen
	tokRParen
	tokEOF
	tokError
)

// lexer scans math expression strings.
type lexer struct {
	s   scanner.Scanner
	val float64
	err string
}

func newLexer(input string) *lexer {
	var s scanner.Scanner
	s.Init(strings.NewReader(input))
	s.Mode = scanner.ScanInts | scanner.ScanFloats
	s.Error = func(s *scanner.Scanner, msg string) {}
	return &lexer{s: s}
}

func (l *lexer) next() token {
	pos := l.s.Position
	switch tok := l.s.Scan(); tok {
	case scanner.EOF:
		return tokEOF
	case scanner.Int, scanner.Float:
		val, err := strconv.ParseFloat(l.s.TokenText(), 64)
		if err != nil {
			l.err = fmt.Sprintf("invalid number %q at Ln%d: %v", l.s.TokenText(), pos.Line, err)
			return tokError
		}
		l.val = val
		return tokNumber
	case '+':
		return tokPlus
	case '-':
		return tokMinus
	case '*':
		return tokMul
	case '/':
		return tokDiv
	case '(':
		return tokLParen
	case ')':
		return tokRParen
	default:
		l.err = fmt.Sprintf("unexpected character %q at Ln%d Col%d", l.s.TokenText(), pos.Line, pos.Column)
		return tokError
	}
}

// parser implements a recursive descent parser for arithmetic expressions.
// Grammar:
//
//	expr    → term { '+' term | '-' term }
//	term    → factor { '*' factor | '/' factor }
//	factor  → '-' factor | '(' expr ')' | number
type parser struct {
	lex *lexer
	cur token
}

func parseMath(input string) (float64, error) {
	p := &parser{lex: newLexer(input)}
	p.cur = p.lex.next()
	if p.cur == tokError {
		return 0, errors.New(p.lex.err)
	}
	val, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	if p.cur != tokEOF {
		return 0, errors.New("unexpected token after expression")
	}
	return val, nil
}

func (p *parser) parseExpr() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for p.cur == tokPlus || p.cur == tokMinus {
		op := p.cur
		p.cur = p.lex.next()
		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		switch op {
		case tokPlus:
			left += right
		case tokMinus:
			left -= right
		}
	}
	return left, nil
}

func (p *parser) parseTerm() (float64, error) {
	left, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for p.cur == tokMul || p.cur == tokDiv {
		op := p.cur
		p.cur = p.lex.next()
		if p.cur == tokError {
			return 0, errors.New(p.lex.err)
		}
		right, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		switch op {
		case tokMul:
			left *= right
		case tokDiv:
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			left /= right
		}
	}
	return left, nil
}

func (p *parser) parseFactor() (float64, error) {
	switch p.cur {
	case tokMinus:
		p.cur = p.lex.next()
		if p.cur == tokError {
			return 0, errors.New(p.lex.err)
		}
		val, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		return -val, nil
	case tokLParen:
		p.cur = p.lex.next()
		if p.cur == tokError {
			return 0, errors.New(p.lex.err)
		}
		val, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		if p.cur != tokRParen {
			return 0, fmt.Errorf("unmatched '(': expected ')'")
		}
		p.cur = p.lex.next()
		if p.cur == tokError {
			return 0, errors.New(p.lex.err)
		}
		return val, nil
	case tokNumber:
		val := p.lex.val
		p.cur = p.lex.next()
		if p.cur == tokError {
			return 0, errors.New(p.lex.err)
		}
		return val, nil
	case tokEOF:
		return 0, fmt.Errorf("unexpected end of expression")
	default:
		return 0, fmt.Errorf("unexpected token in expression")
	}
}
