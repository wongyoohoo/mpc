//
// Copyright (c) 2019-2023 Markku Rossi
//
// All rights reserved.
//

package ast

import (
	"fmt"
	"math"

	"github.com/markkurossi/mpc/compiler/mpa"
	"github.com/markkurossi/mpc/compiler/ssa"
	"github.com/markkurossi/mpc/types"
)

const (
	debugEval = false
)

// Eval implements the compiler.ast.AST.Eval for list statements.
func (ast List) Eval(env *Env, ctx *Codegen, gen *ssa.Generator) (
	ssa.Value, bool, error) {
	return ssa.Undefined, false, fmt.Errorf("List.Eval not implemented yet")
}

// Eval implements the compiler.ast.AST.Eval for function definitions.
func (ast *Func) Eval(env *Env, ctx *Codegen, gen *ssa.Generator) (
	ssa.Value, bool, error) {
	return ssa.Undefined, false, nil
}

// Eval implements the compiler.ast.AST.Eval for constant definitions.
func (ast *ConstantDef) Eval(env *Env, ctx *Codegen,
	gen *ssa.Generator) (ssa.Value, bool, error) {
	return ssa.Undefined, false, nil
}

// Eval implements the compiler.ast.AST.Eval for variable definitions.
func (ast *VariableDef) Eval(env *Env, ctx *Codegen,
	gen *ssa.Generator) (ssa.Value, bool, error) {
	return ssa.Undefined, false, nil
}

// Eval implements the compiler.ast.AST.Eval for assignment expressions.
func (ast *Assign) Eval(env *Env, ctx *Codegen, gen *ssa.Generator) (
	ssa.Value, bool, error) {

	var values []interface{}
	for _, expr := range ast.Exprs {
		val, ok, err := expr.Eval(env, ctx, gen)
		if err != nil || !ok {
			return ssa.Undefined, ok, err
		}
		// XXX multiple return values.
		values = append(values, val)
	}

	if len(ast.LValues) != len(values) {
		return ssa.Undefined, false, ctx.Errorf(ast,
			"assignment mismatch: %d variables but %d values",
			len(ast.LValues), len(values))
	}

	arrType := types.Info{
		Type:       types.TArray,
		IsConcrete: true,
		ArraySize:  types.Size(len(values)),
	}

	if ast.Define {
		for idx, lv := range ast.LValues {
			constVal := gen.Constant(values[idx], types.Undefined)
			gen.AddConstant(constVal)
			arrType.ElementType = &constVal.Type

			ref, ok := lv.(*VariableRef)
			if !ok {
				return ssa.Undefined, false,
					ctx.Errorf(ast, "cannot assign to %s", lv)
			}
			// XXX package.name below

			lValue := gen.NewVal(ref.Name.Name, constVal.Type, ctx.Scope())
			env.Set(lValue, &constVal)
		}
	} else {
		for idx, lv := range ast.LValues {
			ref, ok := lv.(*VariableRef)
			if !ok {
				return ssa.Undefined, false,
					ctx.Errorf(ast, "cannot assign to %s", lv)
			}
			// XXX package.name below

			b, ok := env.Get(ref.Name.Name)
			if !ok {
				return ssa.Undefined, false,
					ctx.Errorf(ast, "undefined variable '%s'", ref.Name)
			}
			lValue := gen.NewVal(b.Name, b.Type, ctx.Scope())

			constVal := gen.Constant(values[idx], b.Type)
			gen.AddConstant(constVal)
			arrType.ElementType = &constVal.Type

			env.Set(lValue, &constVal)
		}
	}

	return gen.Constant(values, arrType), true, nil
}

// Eval implements the compiler.ast.AST.Eval for if statements.
func (ast *If) Eval(env *Env, ctx *Codegen, gen *ssa.Generator) (
	ssa.Value, bool, error) {
	return ssa.Undefined, false, nil
}

// Eval implements the compiler.ast.AST.Eval for call expressions.
func (ast *Call) Eval(env *Env, ctx *Codegen, gen *ssa.Generator) (
	ssa.Value, bool, error) {

	if debugEval {
		fmt.Printf("Call.Eval: %s(", ast.Ref)
		for idx, expr := range ast.Exprs {
			if idx > 0 {
				fmt.Print(", ")
			}
			fmt.Printf("%v", expr)
		}
		fmt.Println(")")
	}

	// Resolve called.
	var pkgName string
	if len(ast.Ref.Name.Package) > 0 {
		pkgName = ast.Ref.Name.Package
	} else {
		pkgName = ast.Ref.Name.Defined
	}
	pkg, ok := ctx.Packages[pkgName]
	if !ok {
		return ssa.Undefined, false,
			ctx.Errorf(ast, "package '%s' not found", pkgName)
	}
	_, ok = pkg.Functions[ast.Ref.Name.Name]
	if ok {
		return ssa.Undefined, false, nil
	}
	// Check builtin functions.
	bi, ok := builtins[ast.Ref.Name.Name]
	if ok && bi.Eval != nil {
		return bi.Eval(ast.Exprs, env, ctx, gen, ast.Location())
	}

	// Resolve name as type.
	typeName := &TypeInfo{
		Point: ast.Point,
		Type:  TypeName,
		Name:  ast.Ref.Name,
	}
	typeInfo, err := typeName.Resolve(env, ctx, gen)
	if err != nil {
		return ssa.Undefined, false, err
	}
	if len(ast.Exprs) != 1 {
		return ssa.Undefined, false, nil
	}
	constVal, ok, err := ast.Exprs[0].Eval(env, ctx, gen)
	if err != nil {
		return ssa.Undefined, false, err
	}
	if !ok {
		return ssa.Undefined, false, nil
	}

	switch typeInfo.Type {
	case types.TInt, types.TUint:
		switch constVal.Type.Type {
		case types.TInt, types.TUint:
			if !typeInfo.Concrete() {
				typeInfo.Bits = constVal.Type.Bits
				typeInfo.SetConcrete(true)
			}
			if constVal.Type.MinBits > typeInfo.Bits {
				typeInfo.MinBits = typeInfo.Bits
			} else {
				typeInfo.MinBits = constVal.Type.MinBits
			}
			cast := constVal
			cast.Type = typeInfo
			if constVal.HashCode() != cast.HashCode() {
				panic("const cast changes value HashCode")
			}
			if !constVal.Equal(&cast) {
				panic("const cast changes value equality")
			}
			return cast, true, nil

		default:
			return ssa.Undefined, false,
				ctx.Errorf(ast.Ref, "casting %T not supported", constVal.Type)
		}
	}

	return ssa.Undefined, false, nil
}

// Eval implements the compiler.ast.AST.Eval for return statements.
func (ast *Return) Eval(env *Env, ctx *Codegen, gen *ssa.Generator) (
	ssa.Value, bool, error) {
	return ssa.Undefined, false, nil
}

// Eval implements the compiler.ast.AST.Eval for for statements.
func (ast *For) Eval(env *Env, ctx *Codegen, gen *ssa.Generator) (
	ssa.Value, bool, error) {
	return ssa.Undefined, false, nil
}

// Eval implements the compiler.ast.AST.Eval for binary expressions.
func (ast *Binary) Eval(env *Env, ctx *Codegen, gen *ssa.Generator) (
	ssa.Value, bool, error) {
	l, ok, err := ast.Left.Eval(env, ctx, gen)
	if err != nil || !ok {
		return ssa.Undefined, ok, err
	}
	r, ok, err := ast.Right.Eval(env, ctx, gen)
	if err != nil || !ok {
		return ssa.Undefined, ok, err
	}

	if debugEval {
		fmt.Printf("Binary.Eval: %v[%v] %v %v[%v]\n",
			ast.Left, l.Type, ast.Op, ast.Right, r.Type)
	}

	switch lval := l.ConstValue.(type) {
	case bool:
		rval, ok := r.ConstValue.(bool)
		if !ok {
			return ssa.Undefined, false, ctx.Errorf(ast.Right,
				"invalid types: %s %s %s", l, ast.Op, r)
		}
		switch ast.Op {
		case BinaryEq:
			return gen.Constant(lval == rval, types.Bool), true, nil
		case BinaryNeq:
			return gen.Constant(lval != rval, types.Bool), true, nil
		case BinaryAnd:
			return gen.Constant(lval && rval, types.Bool), true, nil
		case BinaryOr:
			return gen.Constant(lval || rval, types.Bool), true, nil
		default:
			return ssa.Undefined, false, ctx.Errorf(ast.Right,
				"Binary.Eval: '%v %v %v' not supported", l, ast.Op, r)
		}

	case *mpa.Int:
		rval, ok := r.ConstValue.(*mpa.Int)
		if !ok {
			return ssa.Undefined, false, ctx.Errorf(ast.Right,
				"%s %v %s: invalid r-value %v (%T)", l, ast.Op, r, rval, rval)
		}
		switch ast.Op {
		case BinaryMul:
			return gen.Constant(mpa.NewInt(0).Mul(lval, rval), l.Type),
				true, nil
		case BinaryDiv:
			return gen.Constant(mpa.NewInt(0).Div(lval, rval), l.Type),
				true, nil
		case BinaryMod:
			return gen.Constant(mpa.NewInt(0).Mod(lval, rval), l.Type),
				true, nil
		case BinaryLshift:
			return gen.Constant(mpa.NewInt(0).Lsh(lval, uint(rval.Int64())),
				l.Type), true, nil
		case BinaryBor:
			return gen.Constant(mpa.NewInt(0).Or(lval, rval), l.Type),
				true, nil
		case BinaryAdd:
			return gen.Constant(mpa.NewInt(0).Add(lval, rval), l.Type),
				true, nil
		case BinarySub:
			return gen.Constant(mpa.NewInt(0).Sub(lval, rval), l.Type),
				true, nil
		case BinaryEq:
			return gen.Constant(lval.Cmp(rval) == 0, types.Bool), true, nil
		case BinaryNeq:
			return gen.Constant(lval.Cmp(rval) != 0, types.Bool), true, nil
		case BinaryLt:
			return gen.Constant(lval.Cmp(rval) == -1, types.Bool), true, nil
		case BinaryLe:
			return gen.Constant(lval.Cmp(rval) != 1, types.Bool), true, nil
		case BinaryGt:
			return gen.Constant(lval.Cmp(rval) == 1, types.Bool), true, nil
		case BinaryGe:
			return gen.Constant(lval.Cmp(rval) != -1, types.Bool), true, nil

		default:
			return ssa.Undefined, false, ctx.Errorf(ast.Right,
				"Binary.Eval: '%v %s %v' not implemented yet", l, ast.Op, r)
		}

	case string:
		rval, ok := r.ConstValue.(string)
		if !ok {
			return ssa.Undefined, false, ctx.Errorf(ast.Right,
				"invalid types: %s %s %s", l, ast.Op, r)
		}
		return gen.Constant(lval+rval, types.Undefined), true, nil

	default:
		return ssa.Undefined, false, ctx.Errorf(ast.Left,
			"%s %v %s: invalid l-value %v (%T)", l, ast.Op, r, lval, lval)
	}
}

// Eval implements the compiler.ast.AST.Eval for unary expressions.
func (ast *Unary) Eval(env *Env, ctx *Codegen, gen *ssa.Generator) (
	ssa.Value, bool, error) {
	expr, ok, err := ast.Expr.Eval(env, ctx, gen)
	if err != nil || !ok {
		return ssa.Undefined, ok, err
	}
	switch val := expr.ConstValue.(type) {
	case bool:
		switch ast.Type {
		case UnaryNot:
			return gen.Constant(!val, types.Bool), true, nil
		}
	case int32:
		switch ast.Type {
		case UnaryMinus:
			return gen.Constant(-val, types.Int32), true, nil
		}
	case *mpa.Int:
		switch ast.Type {
		case UnaryMinus:
			r := mpa.NewInt(0)
			return gen.Constant(r.Sub(r, val), expr.Type), true, nil
		}
	}
	return ssa.Undefined, false, ctx.Errorf(ast.Expr,
		"invalid unary expression: %s%T", ast.Type, ast.Expr)
}

// Eval implements the compiler.ast.AST.Eval for slice expressions.
func (ast *Slice) Eval(env *Env, ctx *Codegen, gen *ssa.Generator) (
	ssa.Value, bool, error) {

	expr, ok, err := ast.Expr.Eval(env, ctx, gen)
	if err != nil || !ok {
		return ssa.Undefined, ok, err
	}

	from := 0
	to := math.MaxInt32

	if ast.From != nil {
		val, ok, err := ast.From.Eval(env, ctx, gen)
		if err != nil || !ok {
			return ssa.Undefined, ok, err
		}
		from, err = intVal(val)
		if err != nil {
			return ssa.Undefined, false, ctx.Errorf(ast.From, err.Error())
		}
	}
	if ast.To != nil {
		val, ok, err := ast.To.Eval(env, ctx, gen)
		if err != nil || !ok {
			return ssa.Undefined, ok, err
		}
		to, err = intVal(val)
		if err != nil {
			return ssa.Undefined, false, ctx.Errorf(ast.To, err.Error())
		}
	}
	if to <= from {
		return ssa.Undefined, false, ctx.Errorf(ast.Expr,
			"invalid slice range %d:%d", from, to)
	}
	switch val := expr.ConstValue.(type) {
	case int32:
		if to > 32 {
			return ssa.Undefined, false, ctx.Errorf(ast.From,
				"slice bounds out of range [%d:32]", to)
		}
		tmp := uint32(val)
		tmp >>= from
		tmp &^= 0xffffffff << (to - from)
		return gen.Constant(int32(tmp), types.Int32), true, nil

	case []interface{}:
		if to == math.MaxInt32 {
			to = len(val)
		}
		if to > len(val) {
			return ssa.Undefined, false, ctx.Errorf(ast.From,
				"slice bounds out of range [%d:%d]", to, len(val))
		}
		return gen.Constant(val[from:to], types.Undefined), true, nil

	default:
		return ssa.Undefined, false, ctx.Errorf(ast.Expr,
			"Slice.Eval: expr %T not implemented yet", val)
	}
}

func intVal(val interface{}) (int, error) {
	switch v := val.(type) {
	case int32:
		return int(v), nil

	case *mpa.Int:
		return int(v.Int64()), nil

	case ssa.Value:
		if !v.Const {
			return 0, fmt.Errorf("non-const slice index: %v", v)
		}
		return intVal(v.ConstValue)

	default:
		return 0, fmt.Errorf("invalid slice index: %T", v)
	}
}

// Eval implements the compiler.ast.AST.Eval() for index expressions.
func (ast *Index) Eval(env *Env, ctx *Codegen, gen *ssa.Generator) (
	ssa.Value, bool, error) {

	expr, ok, err := ast.Expr.Eval(env, ctx, gen)
	if err != nil || !ok {
		return ssa.Undefined, ok, err
	}

	val, ok, err := ast.Index.Eval(env, ctx, gen)
	if err != nil || !ok {
		return ssa.Undefined, ok, err
	}
	index, err := intVal(val)
	if err != nil {
		return ssa.Undefined, false, ctx.Errorf(ast.Index, err.Error())
	}

	switch val := expr.ConstValue.(type) {
	case string:
		bytes := []byte(val)
		if index < 0 || index >= len(bytes) {
			return ssa.Undefined, false, ctx.Errorf(ast.Index,
				"invalid array index %d (out of bounds for %d-element string)",
				index, len(bytes))
		}
		return gen.Constant(bytes[index], types.Int32), true, nil

	case []interface{}:
		if index < 0 || index >= len(val) {
			return ssa.Undefined, false, ctx.Errorf(ast.Index,
				"invalid array index %d (out of bounds for %d-element array)",
				index, len(val))
		}
		return gen.Constant(val[index], *expr.Type.ElementType), true, nil

	default:
		return ssa.Undefined, false, ctx.Errorf(ast.Expr,
			"Index.Eval: expr %T not implemented yet", val)
	}
}

// Eval implements the compiler.ast.AST.Eval for variable references.
func (ast *VariableRef) Eval(env *Env, ctx *Codegen, gen *ssa.Generator) (
	ssa.Value, bool, error) {

	lrv, ok, err := ctx.LookupVar(nil, gen, env.Bindings, ast)
	if !ok || err != nil {
		return ssa.Undefined, ok, err
	}

	return lrv.ConstValue()
}

// Eval implements the compiler.ast.AST.Eval for constant values.
func (ast *BasicLit) Eval(env *Env, ctx *Codegen, gen *ssa.Generator) (
	ssa.Value, bool, error) {
	return gen.Constant(ast.Value, types.Undefined), true, nil
}

// Eval implements the compiler.ast.AST.Eval for constant values.
func (ast *CompositeLit) Eval(env *Env, ctx *Codegen, gen *ssa.Generator) (
	ssa.Value, bool, error) {

	// XXX the init values might be short so we must pad them with
	// zero values so that we create correctly sized values.

	typeInfo, err := ast.Type.Resolve(env, ctx, gen)
	if err != nil {
		return ssa.Undefined, false, err
	}
	switch typeInfo.Type {
	case types.TStruct:
		// Check if all elements are constants.
		var values []interface{}
		for _, el := range ast.Value {
			// XXX check if el.Key is specified

			v, ok, err := el.Element.Eval(env, ctx, gen)
			if err != nil || !ok {
				return ssa.Undefined, ok, err
			}
			// XXX chck that v is assignment compatible with typeInfo.Struct[i]
			values = append(values, v)
		}
		return gen.Constant(values, typeInfo), true, nil

	case types.TArray:
		// Check if all elements are constants.
		var values []interface{}
		for _, el := range ast.Value {
			// XXX check if el.Key is specified

			v, ok, err := el.Element.Eval(env, ctx, gen)
			if err != nil || !ok {
				return ssa.Undefined, ok, err
			}
			// XXX check that v is assignment compatible with array.
			values = append(values, v)
		}
		typeInfo.ArraySize = types.Size(len(values))
		typeInfo.Bits = typeInfo.ArraySize * typeInfo.ElementType.Bits
		typeInfo.MinBits = typeInfo.Bits
		return gen.Constant(values, typeInfo), true, nil

	default:
		fmt.Printf("CompositeLit.Eval: not implemented yet: %v, Value: %v\n",
			typeInfo, ast.Value)
		return ssa.Undefined, false, nil
	}
}

// Eval implements the compiler.ast.AST.Eval for the builtin function make.
func (ast *Make) Eval(env *Env, ctx *Codegen, gen *ssa.Generator) (
	ssa.Value, bool, error) {
	if len(ast.Exprs) != 1 {
		return ssa.Undefined, false, ctx.Errorf(ast,
			"invalid amount of argument in call to make")
	}
	typeInfo, err := ast.Type.Resolve(env, ctx, gen)
	if err != nil {
		return ssa.Undefined, false, ctx.Errorf(ast.Type, "%s is not a type",
			ast.Type)
	}
	if typeInfo.Type == types.TArray {
		// Arrays are made in Make.SSA.
		return ssa.Undefined, false, nil
	}
	if typeInfo.Bits != 0 {
		return ssa.Undefined, false, ctx.Errorf(ast.Type,
			"can't make specified type %s", typeInfo)
	}
	constVal, _, err := ast.Exprs[0].Eval(env, ctx, gen)
	if err != nil {
		return ssa.Undefined, false, ctx.Error(ast.Exprs[0], err.Error())
	}
	length, err := constVal.ConstInt()
	if err != nil {
		return ssa.Undefined, false, ctx.Errorf(ast.Exprs[0],
			"non-integer (%T) len argument in %s: %s", constVal, ast, err)
	}

	typeInfo.IsConcrete = true
	typeInfo.Bits = length

	// Create typeref constant.
	return gen.Constant(typeInfo, types.Undefined), true, nil
}
