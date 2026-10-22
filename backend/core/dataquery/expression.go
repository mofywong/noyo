package dataquery

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"strconv"
)

// The expression language deliberately has no calls into Go, loops, reflection,
// network or mutable state. Parsing once per step keeps row evaluation cheap.
func compileExpression(text string) (ast.Expr, error) {
	if len(text) == 0 || len(text) > 1024 {
		return nil, errors.New("Invalid expression length / 表达式长度无效")
	}
	expr, err := parser.ParseExpr(text)
	if err != nil {
		return nil, errors.New("Invalid expression / 表达式无效")
	}
	nodes := 0
	valid := true
	ast.Inspect(expr, func(node ast.Node) bool {
		if node == nil {
			return false
		}
		nodes++
		if nodes > 128 {
			valid = false
			return false
		}
		switch n := node.(type) {
		case *ast.BasicLit:
			if n.Kind != token.INT && n.Kind != token.FLOAT && n.Kind != token.STRING {
				valid = false
			}
		case *ast.Ident:
			if !validName(n.Name) {
				valid = false
			}
		case *ast.ParenExpr, *ast.UnaryExpr, *ast.BinaryExpr:
		case *ast.IndexExpr:
			id, ok := n.X.(*ast.Ident)
			if !ok || (id.Name != "row" && id.Name != "param") {
				valid = false
			}
			if _, ok := n.Index.(*ast.BasicLit); !ok {
				valid = false
			}
		case *ast.CallExpr:
			id, ok := n.Fun.(*ast.Ident)
			if !ok || !oneOf(id.Name, "abs", "round", "coalesce", "ifelse") || len(n.Args) > 3 {
				valid = false
			}
		default:
			valid = false
		}
		return valid
	})
	if !valid {
		return nil, errors.New("Unsupported or complex expression / 表达式不支持或过于复杂")
	}
	return expr, nil
}
func evaluate(expr ast.Expr, row Row, params map[string]interface{}) (interface{}, error) {
	switch n := expr.(type) {
	case *ast.BasicLit:
		if n.Kind == token.STRING {
			return strconv.Unquote(n.Value)
		}
		return strconv.ParseFloat(n.Value, 64)
	case *ast.Ident:
		switch n.Name {
		case "true":
			return true, nil
		case "false":
			return false, nil
		case "null", "nil":
			return nil, nil
		}
		return row[n.Name], nil
	case *ast.IndexExpr:
		key, err := evaluate(n.Index, row, params)
		if err != nil {
			return nil, err
		}
		name, ok := key.(string)
		if !ok {
			return nil, errors.New("Field index must be a string / 字段索引必须是字符串")
		}
		if n.X.(*ast.Ident).Name == "param" {
			return params[name], nil
		}
		return row[name], nil
	case *ast.ParenExpr:
		return evaluate(n.X, row, params)
	case *ast.UnaryExpr:
		v, err := evaluate(n.X, row, params)
		if err != nil {
			return nil, err
		}
		if v == nil {
			return nil, nil
		}
		if n.Op == token.NOT {
			b, ok := v.(bool)
			if ok {
				return !b, nil
			}
		}
		if x, ok := number(v); ok {
			if n.Op == token.SUB {
				return -x, nil
			}
			if n.Op == token.ADD {
				return x, nil
			}
		}
	case *ast.BinaryExpr:
		a, err := evaluate(n.X, row, params)
		if err != nil {
			return nil, err
		}
		if n.Op == token.LAND && a == false {
			return false, nil
		}
		if n.Op == token.LOR && a == true {
			return true, nil
		}
		b, err := evaluate(n.Y, row, params)
		if err != nil {
			return nil, err
		}
		if n.Op == token.EQL || n.Op == token.NEQ {
			equal := compare(a, b) == 0
			if n.Op == token.NEQ {
				equal = !equal
			}
			return equal, nil
		}
		if a == nil || b == nil {
			return nil, nil
		}
		if n.Op == token.LAND || n.Op == token.LOR {
			x, xo := a.(bool)
			y, yo := b.(bool)
			if xo && yo {
				if n.Op == token.LAND {
					return x && y, nil
				}
				return x || y, nil
			}
		}
		if oneOf(n.Op.String(), "<", "<=", ">", ">=") {
			c := compare(a, b)
			switch n.Op {
			case token.LSS:
				return c < 0, nil
			case token.LEQ:
				return c <= 0, nil
			case token.GTR:
				return c > 0, nil
			case token.GEQ:
				return c >= 0, nil
			}
		}
		x, xo := number(a)
		y, yo := number(b)
		if xo && yo {
			var v float64
			switch n.Op {
			case token.ADD:
				v = x + y
			case token.SUB:
				v = x - y
			case token.MUL:
				v = x * y
			case token.QUO:
				if y == 0 {
					return nil, nil
				}
				v = x / y
			case token.REM:
				if y == 0 {
					return nil, nil
				}
				v = math.Mod(x, y)
			default:
				return nil, errors.New("Unsupported operator / 不支持的运算符")
			}
			if math.IsInf(v, 0) || math.IsNaN(v) {
				return nil, nil
			}
			return v, nil
		}
	case *ast.CallExpr:
		name := n.Fun.(*ast.Ident).Name
		if name == "ifelse" && len(n.Args) == 3 {
			test, err := evaluate(n.Args[0], row, params)
			if err != nil {
				return nil, err
			}
			if test == true {
				return evaluate(n.Args[1], row, params)
			}
			return evaluate(n.Args[2], row, params)
		}
		if name == "coalesce" {
			for _, arg := range n.Args {
				v, err := evaluate(arg, row, params)
				if err != nil {
					return nil, err
				}
				if v != nil {
					return v, nil
				}
			}
			return nil, nil
		}
		if len(n.Args) == 1 {
			v, err := evaluate(n.Args[0], row, params)
			if err != nil {
				return nil, err
			}
			if v == nil {
				return nil, nil
			}
			if x, ok := number(v); ok {
				if name == "abs" {
					return math.Abs(x), nil
				}
				if name == "round" {
					return math.Round(x), nil
				}
			}
		}
	}
	return nil, errors.New("Expression type mismatch / 表达式类型不匹配")
}
