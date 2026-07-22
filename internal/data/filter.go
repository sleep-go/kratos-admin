package data

import (
	"fmt"
	"time"

	"go.einride.tech/aip/filtering"
	"go.einride.tech/aip/ordering"
	expr "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ApplyFilter 将 AIP filtering.Filter 翻译为 GORM 查询作用域。
// 翻译失败时静默跳过（与 ent 适配器行为一致），避免影响请求可用性。
func ApplyFilter(filter filtering.Filter) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if filter.CheckedExpr == nil || filter.CheckedExpr.Expr == nil {
			return db
		}
		cond, err := exprToExpression(filter.CheckedExpr.Expr)
		if err != nil || cond == nil {
			return db
		}
		return db.Where(cond)
	}
}

// ApplyOrderBy 将 AIP ordering.OrderBy 翻译为 GORM 排序作用域。
func ApplyOrderBy(orderBy ordering.OrderBy) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		for _, field := range orderBy.Fields {
			if field.Desc {
				db = db.Order(clause.OrderByColumn{Column: clause.Column{Name: field.Path}, Desc: true})
			} else {
				db = db.Order(clause.OrderByColumn{Column: clause.Column{Name: field.Path}})
			}
		}
		return db
	}
}

// exprToExpression 将 CEL 表达式递归转换为 GORM clause.Expression。
func exprToExpression(e *expr.Expr) (clause.Expression, error) {
	if e == nil {
		return nil, fmt.Errorf("expression is nil")
	}

	switch kind := e.GetExprKind().(type) {
	case *expr.Expr_CallExpr:
		call := kind.CallExpr
		function := call.GetFunction()
		args := call.GetArgs()

		switch function {
		case filtering.FunctionAnd, filtering.FunctionFuzzyAnd:
			return combineLogicalExpr(args, clause.And)
		case filtering.FunctionOr:
			return combineLogicalExpr(args, clause.Or)
		case filtering.FunctionNot:
			if len(args) != 1 {
				return nil, fmt.Errorf("NOT expects 1 argument, got %d", len(args))
			}
			pred, err := exprToExpression(args[0])
			if err != nil {
				return nil, err
			}
			return clause.Not(pred), nil
		case filtering.FunctionEquals,
			filtering.FunctionNotEquals,
			filtering.FunctionLessThan,
			filtering.FunctionLessEquals,
			filtering.FunctionGreaterThan,
			filtering.FunctionGreaterEquals:
			return comparisonExpr(function, args)
		case filtering.FunctionHas:
			return hasExpr(args)
		default:
			return nil, fmt.Errorf("unsupported call expression: %s", function)
		}

	case *expr.Expr_ConstExpr:
		value, err := constToValue(kind.ConstExpr)
		if err != nil {
			return nil, err
		}
		boolean, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("non-boolean constant cannot form predicate: %v", value)
		}
		if boolean {
			return clause.Expr{SQL: "TRUE"}, nil
		}
		return clause.Expr{SQL: "FALSE"}, nil

	case *expr.Expr_IdentExpr:
		return clause.Expr{SQL: fmt.Sprintf("`%s` IS TRUE", kind.IdentExpr.GetName())}, nil

	case *expr.Expr_SelectExpr:
		column, ok := exprToColumn(kind.SelectExpr)
		if !ok {
			return nil, fmt.Errorf("invalid select expression")
		}
		return clause.Expr{SQL: fmt.Sprintf("`%s` IS TRUE", column)}, nil
	}

	return nil, fmt.Errorf("unsupported expression kind %T", e.GetExprKind())
}

// combineLogicalExpr 将多个子表达式用 AND/OR 合并。
func combineLogicalExpr(args []*expr.Expr, combine func(...clause.Expression) clause.Expression) (clause.Expression, error) {
	if len(args) == 0 {
		return clause.Expr{SQL: "TRUE"}, nil
	}
	exprs := make([]clause.Expression, 0, len(args))
	for _, arg := range args {
		e, err := exprToExpression(arg)
		if err != nil {
			return nil, err
		}
		if e != nil {
			exprs = append(exprs, e)
		}
	}
	if len(exprs) == 0 {
		return clause.Expr{SQL: "TRUE"}, nil
	}
	return combine(exprs...), nil
}

// comparisonExpr 构建比较表达式，支持列-值、列-列两种模式。
func comparisonExpr(function string, args []*expr.Expr) (clause.Expression, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("%s expects 2 arguments, got %d", function, len(args))
	}

	leftCol, leftIsCol := exprToColumnExpr(args[0])
	rightCol, rightIsCol := exprToColumnExpr(args[1])

	switch {
	case leftIsCol && rightIsCol:
		return columnToColumnExpr(function, leftCol, rightCol)
	case leftIsCol:
		val, err := exprToValue(args[1])
		if err != nil {
			return nil, err
		}
		return columnToValueExpr(function, leftCol, val)
	case rightIsCol:
		val, err := exprToValue(args[0])
		if err != nil {
			return nil, err
		}
		return valueToColumnExpr(function, val, rightCol)
	default:
		return nil, fmt.Errorf("comparison requires at least one column operand")
	}
}

// hasExpr 构建 ":" (has) 表达式，等价于 LIKE '%value%'。
func hasExpr(args []*expr.Expr) (clause.Expression, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("%s expects 2 arguments, got %d", filtering.FunctionHas, len(args))
	}
	column, ok := exprToColumnExpr(args[0])
	if !ok {
		return nil, fmt.Errorf("left operand of ':' must be a column")
	}
	value, err := exprToValue(args[1])
	if err != nil {
		return nil, err
	}
	str, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("':' right operand must resolve to string, got %T", value)
	}
	return clause.Like{Column: column, Value: "%" + str + "%"}, nil
}

// columnToColumnExpr 构建列与列的比较表达式。
func columnToColumnExpr(function, left, right string) (clause.Expression, error) {
	var op string
	switch function {
	case filtering.FunctionEquals:
		op = "="
	case filtering.FunctionNotEquals:
		op = "!="
	case filtering.FunctionLessThan:
		op = "<"
	case filtering.FunctionLessEquals:
		op = "<="
	case filtering.FunctionGreaterThan:
		op = ">"
	case filtering.FunctionGreaterEquals:
		op = ">="
	default:
		return nil, fmt.Errorf("unsupported column comparison: %s", function)
	}
	return clause.Expr{SQL: fmt.Sprintf("`%s` %s `%s`", left, op, right)}, nil
}

// columnToValueExpr 构建列与值的比较表达式。
func columnToValueExpr(function, column string, value any) (clause.Expression, error) {
	switch function {
	case filtering.FunctionEquals:
		return clause.Eq{Column: column, Value: value}, nil
	case filtering.FunctionNotEquals:
		return clause.Neq{Column: column, Value: value}, nil
	case filtering.FunctionLessThan:
		return clause.Lt{Column: column, Value: value}, nil
	case filtering.FunctionLessEquals:
		return clause.Lte{Column: column, Value: value}, nil
	case filtering.FunctionGreaterThan:
		return clause.Gt{Column: column, Value: value}, nil
	case filtering.FunctionGreaterEquals:
		return clause.Gte{Column: column, Value: value}, nil
	default:
		return nil, fmt.Errorf("unsupported comparison operator %s", function)
	}
}

// valueToColumnExpr 构建值与列的比较表达式，操作数交换后比较方向需翻转。
func valueToColumnExpr(function string, value any, column string) (clause.Expression, error) {
	switch function {
	case filtering.FunctionEquals:
		return clause.Eq{Column: column, Value: value}, nil
	case filtering.FunctionNotEquals:
		return clause.Neq{Column: column, Value: value}, nil
	case filtering.FunctionLessThan:
		// value < column 等价于 column > value
		return clause.Gt{Column: column, Value: value}, nil
	case filtering.FunctionLessEquals:
		return clause.Gte{Column: column, Value: value}, nil
	case filtering.FunctionGreaterThan:
		return clause.Lt{Column: column, Value: value}, nil
	case filtering.FunctionGreaterEquals:
		return clause.Lte{Column: column, Value: value}, nil
	default:
		return nil, fmt.Errorf("unsupported comparison operator %s", function)
	}
}

// exprToColumnExpr 将表达式解析为列名字符串。
func exprToColumnExpr(e *expr.Expr) (string, bool) {
	switch kind := e.GetExprKind().(type) {
	case *expr.Expr_IdentExpr:
		return kind.IdentExpr.GetName(), true
	case *expr.Expr_SelectExpr:
		return exprToColumn(kind.SelectExpr)
	default:
		return "", false
	}
}

// exprToColumn 将 Select 表达式解析为列名（支持 a.b 形式）。
func exprToColumn(sel *expr.Expr_Select) (string, bool) {
	if sel == nil {
		return "", false
	}
	if operand := sel.GetOperand(); operand != nil {
		if prefix, ok := exprToColumnExpr(operand); ok {
			return prefix + "." + sel.GetField(), true
		}
	}
	return sel.GetField(), sel.GetField() != ""
}

// constToValue 将 CEL 常量转换为 Go 值。
func constToValue(constExpr *expr.Constant) (any, error) {
	switch constExpr.ConstantKind.(type) {
	case *expr.Constant_BoolValue:
		return constExpr.GetBoolValue(), nil
	case *expr.Constant_StringValue:
		return constExpr.GetStringValue(), nil
	case *expr.Constant_Int64Value:
		return constExpr.GetInt64Value(), nil
	case *expr.Constant_DoubleValue:
		return constExpr.GetDoubleValue(), nil
	case *expr.Constant_DurationValue:
		if d := constExpr.GetDurationValue(); d != nil {
			return d.AsDuration(), nil
		}
	case *expr.Constant_TimestampValue:
		if ts := constExpr.GetTimestampValue(); ts != nil {
			return ts.AsTime(), nil
		}
	}
	return nil, fmt.Errorf("unsupported constant expression %v", constExpr.ConstantKind)
}

// exprToValue 将表达式解析为 Go 值，支持 timestamp()、duration() 函数调用。
func exprToValue(e *expr.Expr) (any, error) {
	switch kind := e.GetExprKind().(type) {
	case *expr.Expr_ConstExpr:
		return constToValue(kind.ConstExpr)
	case *expr.Expr_CallExpr:
		call := kind.CallExpr
		switch call.GetFunction() {
		case filtering.FunctionTimestamp:
			if len(call.Args) != 1 {
				return nil, fmt.Errorf("timestamp expects 1 argument, got %d", len(call.Args))
			}
			argVal, err := exprToValue(call.Args[0])
			if err != nil {
				return nil, err
			}
			str, ok := argVal.(string)
			if !ok {
				return nil, fmt.Errorf("timestamp argument must be string, got %T", argVal)
			}
			return time.Parse(time.RFC3339, str)
		case filtering.FunctionDuration:
			if len(call.Args) != 1 {
				return nil, fmt.Errorf("duration expects 1 argument, got %d", len(call.Args))
			}
			argVal, err := exprToValue(call.Args[0])
			if err != nil {
				return nil, err
			}
			str, ok := argVal.(string)
			if !ok {
				return nil, fmt.Errorf("duration argument must be string, got %T", argVal)
			}
			return time.ParseDuration(str)
		default:
			return nil, fmt.Errorf("unsupported value function: %s", call.GetFunction())
		}
	default:
		return nil, fmt.Errorf("unsupported value expression %T", e.GetExprKind())
	}
}
