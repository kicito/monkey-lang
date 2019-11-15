package evaluator

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/kicito/monkey/ast"
	"github.com/kicito/monkey/lexer"
	"github.com/kicito/monkey/object"
	"github.com/kicito/monkey/parser"
)

var (
	NULL  = &object.Null{}
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
)

func Eval(node ast.Node, env *object.Environment) object.Object {
	switch node := node.(type) {
	// Statements
	case *ast.Program:
		return evalProgram(node, env)
	case *ast.ExpressionStatement:
		return Eval(node.Expression, env)
	case *ast.BlockStatement:
		return evalBlockStatement(node, env)
	case *ast.ReturnStatement:
		val := Eval(node.ReturnValue, env)
		if isError(val) {
			return val
		}
		return &object.ReturnValue{Value: val}
	case *ast.LetStatement:
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		env.Set(node.Name.Value, val)
	case *ast.ImportStatement:
		return evalImportStatement(node, env)
	case *ast.Identifier:
		return evalIdentifier(node, env)

	// Expressions
	case *ast.IntegerLiteral:
		return &object.Integer{Value: node.Value}
	case *ast.FloatLiteral:
		return &object.Float{Value: node.Value}
	case *ast.StringLiteral:
		return &object.String{Value: node.Value}
	case *ast.FunctionLiteral:
		params := node.Parameters
		body := node.Body
		return &object.Function{Parameters: params, Env: env, Body: body}
	case *ast.HashLiteral:
		return evalHashLiteral(node, env)

	case *ast.CallExpression:
		function := Eval(node.Function, env)
		if isError(function) {
			return function
		}
		args := evalExpressions(node.Arguments, env)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}
		return applyFunction(function, args)
	case *ast.Boolean:
		return nativeBoolToBooleanObject(node.Value)

	case *ast.ArrayLiteral:
		elements := evalExpressions(node.Elements, env)
		if len(elements) == 1 && isError(elements[0]) {
			return elements[0]
		}
		return &object.Array{Elements: elements}

	case *ast.PrefixExpression:
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalPrefixExpression(node.Operator, right)
	case *ast.InfixExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		right := Eval(node.Right, env)

		if isError(right) {
			return right
		}
		return evalInfixExpression(node.Operator, left, right)
	case *ast.IndexExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		index := Eval(node.Index, env)
		if isError(index) {
			return index
		}
		return evalIndexExpression(left, index)

	case *ast.IfExpression:
		return evalIfExpression(node, env)

	}
	return nil
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

func evalProgram(program *ast.Program, env *object.Environment) object.Object {
	var result object.Object
	for _, statement := range program.Statements {
		result = Eval(statement, env)
		switch result := result.(type) {
		case *object.ReturnValue:
			return result.Value
		case *object.Error:
			return result
		}
	}
	return result
}

func evalBlockStatement(block *ast.BlockStatement, env *object.Environment) object.Object {
	var result object.Object
	for _, statement := range block.Statements {
		result = Eval(statement, env)
		if result != nil {
			rt := result.Type()
			if rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ {
				return result
			}
		}
	}
	return result
}

func evalImportStatement(node *ast.ImportStatement, env *object.Environment) object.Object {
	var result object.Object
	var targetPath string
	// check whether path is abs
	if filepath.IsAbs(node.Target.Value) {
		targetPath = node.Target.Value
	} else {
		// finding Target directory
		ex, err := os.Executable()
		if err != nil {
			panic(err)
		}
		targetPath = filepath.Join(filepath.Dir(ex), node.Target.Value)
	}
	readByte, err := ioutil.ReadFile(targetPath)
	if err != nil {
		return newError("unable to open module %s on %s", node.Target.Value, targetPath)
	}

	// create new program
	l := lexer.New(string(readByte))
	p := parser.New(l)
	program := p.ParseProgram()
	libEnv := object.NewEnvironment()
	obj := Eval(program, libEnv)
	if isError(obj) {
		return result
	}

	if node.IsNameSpaceImport {
		env.Set(node.LocalName.Value, libEnv.ToHash())
	} else {
		locObj, ok := libEnv.Get(node.LocalName.Value)
		if !ok {
			return newError("unknown identifier %s in %s", node.LocalName, node.Target.Value)
		}
		env.Set(node.LocalName.Value, locObj)
	}
	return TRUE
}

func evalPrefixExpression(operator string, right object.Object) object.Object {
	switch operator {
	case "!":
		return evalBangOperatorExpression(right)
	case "-":
		return evalMinusPrefixOperatorExpression(right)
	default:
		return newError("unknown operator: %s%s", operator, right.Type())
	}
}

func evalInfixExpression(
	operator string,
	left, right object.Object,
) object.Object {
	switch {
	case (left.Type() == object.INTEGER_OBJ || left.Type() == object.FLOAT_OBJ) && (right.Type() == object.INTEGER_OBJ || right.Type() == object.FLOAT_OBJ):
		return evalNumberInfixExpression(operator, left, right)
	case left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ:
		return evalStringInfixExpression(operator, left, right)
	case operator == "==":
		return nativeBoolToBooleanObject(left == right)
	case operator == "!=":
		return nativeBoolToBooleanObject(left != right)
	case left.Type() != right.Type():
		return newError("type mismatch: %s %s %s",
			left.Type(), operator, right.Type())
	default:
		return newError("unknown operator: %s %s %s",
			left.Type(), operator, right.Type())
	}
}

func evalBangOperatorExpression(right object.Object) object.Object {
	switch right {
	case TRUE:
		return FALSE
	case FALSE:
		return TRUE
	case NULL:
		return TRUE
	default:
		return FALSE
	}
}

func evalMinusPrefixOperatorExpression(right object.Object) object.Object {
	if right.Type() == object.INTEGER_OBJ {
		value := right.(*object.Integer).Value
		return &object.Integer{Value: -value}
	} else if right.Type() == object.FLOAT_OBJ {
		value := right.(*object.Float).Value
		return &object.Float{Value: -value}
	}
	return newError("unknown operator: -%s", right.Type())

}

func evalNumberInfixExpression(
	operator string,
	left, right object.Object,
) (returnVal object.Object) {
	if left.Type() == object.FLOAT_OBJ || right.Type() == object.FLOAT_OBJ {
		returnVal = &object.Float{}
		var leftVal float64
		var rightVal float64
		if left.Type() == object.FLOAT_OBJ {
			leftVal = left.(*object.Float).Value
		} else {
			leftVal = float64(left.(*object.Integer).Value)
		}
		if right.Type() == object.FLOAT_OBJ {
			rightVal = right.(*object.Float).Value
		} else {
			rightVal = float64(right.(*object.Integer).Value)
		}
		switch operator {
		case "+":
			returnVal = &object.Float{Value: leftVal + rightVal}
		case "-":
			returnVal = &object.Float{Value: leftVal - rightVal}
		case "*":
			returnVal = &object.Float{Value: leftVal * rightVal}
		case "/":
			returnVal = &object.Float{Value: leftVal / rightVal}
		case "<":
			return nativeBoolToBooleanObject(leftVal < rightVal)
		case ">":
			return nativeBoolToBooleanObject(leftVal > rightVal)
		case "==":
			return nativeBoolToBooleanObject(leftVal == rightVal)
		case "!=":
			return nativeBoolToBooleanObject(leftVal != rightVal)
		default:
			return newError("unknown operator: %s %s %s",
				left.Type(), operator, right.Type())
		}
	} else {
		returnVal = &object.Integer{}
		var leftVal int64
		var rightVal int64
		leftVal = left.(*object.Integer).Value
		rightVal = right.(*object.Integer).Value
		switch operator {
		case "+":
			returnVal = &object.Integer{Value: leftVal + rightVal}
		case "-":
			returnVal = &object.Integer{Value: leftVal - rightVal}
		case "*":
			returnVal = &object.Integer{Value: leftVal * rightVal}
		case "/":
			returnVal = &object.Integer{Value: leftVal / rightVal}
		case "<":
			return nativeBoolToBooleanObject(leftVal < rightVal)
		case ">":
			return nativeBoolToBooleanObject(leftVal > rightVal)
		case "==":
			return nativeBoolToBooleanObject(leftVal == rightVal)
		case "!=":
			return nativeBoolToBooleanObject(leftVal != rightVal)
		default:
			return newError("unknown operator: %s %s %s",
				left.Type(), operator, right.Type())
		}
	}
	return
}

func evalStringInfixExpression(
	operator string,
	left, right object.Object,
) object.Object {
	if operator != "+" {
		return newError("unknown operator: %s %s %s",
			left.Type(), operator, right.Type())
	}
	leftVal := left.(*object.String).Value
	rightVal := right.(*object.String).Value
	return &object.String{Value: leftVal + rightVal}
}

func evalIndexExpression(left, index object.Object) object.Object {
	switch {
	case left.Type() == object.ARRAY_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalArrayIndexExpression(left, index)
	case left.Type() == object.HASH_OBJ:
		return evalHashIndexExpression(left, index)
	default:
		return newError("index operator not supported: %s", left.Type())
	}
}

func evalArrayIndexExpression(array, index object.Object) object.Object {
	arrayObject := array.(*object.Array)
	idx := index.(*object.Integer).Value
	max := int64(len(arrayObject.Elements) - 1)
	if idx < 0 || idx > max {
		return NULL
	}
	return arrayObject.Elements[idx]
}

func evalHashIndexExpression(hash, index object.Object) object.Object {
	hashObject := hash.(*object.Hash)
	key, ok := index.(object.Hashable)
	if !ok {
		return newError("unusable as hash key: %s", index.Type())
	}
	pair, ok := hashObject.Pairs[key.HashKey()]
	if !ok {
		return NULL
	}
	return pair.Value
}

func evalIfExpression(ie *ast.IfExpression, env *object.Environment) object.Object {
	condition := Eval(ie.Condition, env)
	if isError(condition) {
		return condition
	}
	if isTruthy(condition) {
		return Eval(ie.Consequence, env)
	} else if ie.Alternative != nil {
		return Eval(ie.Alternative, env)
	} else {
		return NULL
	}
}

func evalExpressions(
	exps []ast.Expression,
	env *object.Environment,
) []object.Object {
	var result []object.Object
	for _, e := range exps {
		evaluated := Eval(e, env)
		if isError(evaluated) {
			return []object.Object{evaluated}
		}
		result = append(result, evaluated)
	}
	return result
}

func evalHashLiteral(
	node *ast.HashLiteral,
	env *object.Environment,
) object.Object {
	pairs := make(map[object.HashKey]object.HashPair)
	for keyNode, valueNode := range node.Pairs {
		key := Eval(keyNode, env)
		if isError(key) {
			return key
		}
		hashKey, ok := key.(object.Hashable)
		if !ok {
			return newError("unusable as hash key: %s", key.Type())
		}
		value := Eval(valueNode, env)
		if isError(value) {
			return value
		}
		hashed := hashKey.HashKey()
		pairs[hashed] = object.HashPair{Key: key, Value: value}
	}
	return &object.Hash{Pairs: pairs}
}

func applyFunction(fn object.Object, args []object.Object) object.Object {
	switch fn := fn.(type) {
	case *object.Function:
		extendedEnv := extendFunctionEnv(fn, args)
		evaluated := Eval(fn.Body, extendedEnv)
		return unwrapReturnValue(evaluated)
	case *object.Builtin:
		return fn.Fn(args...)
	default:
		return newError("not a function: %s", fn.Type())
	}
}

func extendFunctionEnv(
	fn *object.Function,
	args []object.Object,
) *object.Environment {
	env := object.NewEnclosedEnvironment(fn.Env)
	for paramIdx, param := range fn.Parameters {
		env.Set(param.Value, args[paramIdx])
	}
	return env
}

func unwrapReturnValue(obj object.Object) object.Object {
	if returnValue, ok := obj.(*object.ReturnValue); ok {
		return returnValue.Value
	}
	return obj
}

func isTruthy(obj object.Object) bool {
	switch obj {
	case NULL:
		return false
	case TRUE:
		return true
	case FALSE:
		return false
	default:
		return true
	}
}

func newError(format string, a ...interface{}) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, a...)}
}

func isError(obj object.Object) bool {
	if obj != nil {
		return obj.Type() == object.ERROR_OBJ
	}
	return false
}

func evalIdentifier(
	node *ast.Identifier,
	env *object.Environment,
) object.Object {
	if val, ok := env.Get(node.Value); ok {
		return val
	}
	if builtin, ok := builtins[node.Value]; ok {
		return builtin
	}

	return newError("identifier not found: " + node.Value)

}
