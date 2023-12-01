//
// Copyright (c) 2023 Markku Rossi
//
// All rights reserved.
//

package mpa

import (
	"fmt"
	"math/big"

	"github.com/markkurossi/mpc/circuit"
	"github.com/markkurossi/mpc/compiler/circuits"
	"github.com/markkurossi/mpc/compiler/utils"
	"github.com/markkurossi/mpc/types"
)

// Int implements multi-precision integer arithmetics.
type Int struct {
	bits   int
	values *big.Int
}

// NewInt creates a new Int with init value x.
func NewInt(x int64) *Int {
	return &Int{
		bits:   64,
		values: big.NewInt(x),
	}
}

func (z *Int) Debug() {
	fmt.Printf("mpa.Big: val=%v, bits=%v, bitLen=%v\n",
		z, z.bits, z.BitLen())
}

func (z *Int) TypeSize() int {
	return z.bits
}

func (z *Int) SetTypeSize(size int) {
	z.bits = size
}

func (z *Int) Bit(i int) uint {
	return z.values.Bit(i)
}

func (z *Int) BitLen() int {
	return z.values.BitLen()
}

func (z *Int) Cmp(x *Int) int {
	return z.values.Cmp(x.values)
}

// Int64 returns the int64 representation of x. If x cannot be
// represented as int64, the result is undefined.
func (z *Int) Int64() int64 {
	z.setSign()
	return z.values.Int64()
}

func (z *Int) String() string {
	return z.values.String()
}

// Add sets z to x+y and returns z.
func (z *Int) Add(x, y *Int) *Int {
	return z.bin(circuits.NewAdder, x, y)
}

// And sets z to x&y and returns z.
func (z *Int) And(x, y *Int) *Int {
	z.values.And(x.values, y.values)
	z.bits = max(x.bits, y.bits)
	z.setSign()
	return z
}

// Div sets z to x/y and returns z.
func (z *Int) Div(x, y *Int) *Int {
	calloc := circuits.NewAllocator()
	inputs := circuit.IO{
		newIOArg("x", types.TInt, x.bits),
		newIOArg("y", types.TInt, y.bits),
	}
	outputs := circuit.IO{
		newIOArg("q", types.TInt, max(x.bits, y.bits)),
		newIOArg("r", types.TInt, max(x.bits, y.bits)),
	}
	i0w := calloc.Wires(inputs[0].Type.Bits)
	i1w := calloc.Wires(inputs[1].Type.Bits)

	var inputWires []*circuits.Wire
	inputWires = append(inputWires, i0w...)
	inputWires = append(inputWires, i1w...)

	o0w := calloc.Wires(outputs[0].Type.Bits)
	o1w := calloc.Wires(outputs[1].Type.Bits)

	var outputWires []*circuits.Wire
	outputWires = append(outputWires, o0w...)
	outputWires = append(outputWires, o1w...)

	for idx := range outputWires {
		outputWires[idx].SetOutput(true)
	}

	cc, err := circuits.NewCompiler(utils.NewParams(), calloc, inputs, outputs,
		inputWires, outputWires)
	if err != nil {
		panic(err)
	}

	err = circuits.NewDivider(cc, i0w, i1w, o0w, o1w)
	if err != nil {
		panic(err)
	}

	circ := cc.Compile()

	obits, err := circ.Compute([]*big.Int{x.values, y.values})
	if err != nil {
		panic(err)
	}

	z.bits = int(outputs[0].Type.Bits)
	z.values = obits[0]
	z.setSign()

	return z
}

// Lsh sets z to x<<n and returns z.
func (z *Int) Lsh(x *Int, n uint) *Int {
	if z != x {
		z.bits = x.bits
		z.values.Set(x.values)
	}
	z.values.Lsh(z.values, n)
	for i := z.values.BitLen() - 1; i >= z.bits; i-- {
		z.values.SetBit(z.values, i, 0)
	}
	z.setSign()
	return z
}

// Mod sets z to x%y and returns z.
func (z *Int) Mod(x, y *Int) *Int {
	calloc := circuits.NewAllocator()
	inputs := circuit.IO{
		newIOArg("x", types.TInt, x.bits),
		newIOArg("y", types.TInt, y.bits),
	}
	outputs := circuit.IO{
		newIOArg("q", types.TInt, max(x.bits, y.bits)),
		newIOArg("r", types.TInt, max(x.bits, y.bits)),
	}
	i0w := calloc.Wires(inputs[0].Type.Bits)
	i1w := calloc.Wires(inputs[1].Type.Bits)

	var inputWires []*circuits.Wire
	inputWires = append(inputWires, i0w...)
	inputWires = append(inputWires, i1w...)

	o0w := calloc.Wires(outputs[0].Type.Bits)
	o1w := calloc.Wires(outputs[1].Type.Bits)

	var outputWires []*circuits.Wire
	outputWires = append(outputWires, o0w...)
	outputWires = append(outputWires, o1w...)

	for idx := range outputWires {
		outputWires[idx].SetOutput(true)
	}

	cc, err := circuits.NewCompiler(utils.NewParams(), calloc, inputs, outputs,
		inputWires, outputWires)
	if err != nil {
		panic(err)
	}

	err = circuits.NewDivider(cc, i0w, i1w, o0w, o1w)
	if err != nil {
		panic(err)
	}

	circ := cc.Compile()

	obits, err := circ.Compute([]*big.Int{x.values, y.values})
	if err != nil {
		panic(err)
	}

	z.bits = int(outputs[1].Type.Bits)
	z.values = obits[1]
	z.setSign()

	return z
}

// Mul sets z to x*y and returns z.
func (z *Int) Mul(x, y *Int) *Int {
	return z.bin(func(cc *circuits.Compiler, x, y, z []*circuits.Wire) error {
		return circuits.NewMultiplier(cc, 0, x, y, z)
	}, x, y)
}

// Or sets z to x|y and returns z.
func (z *Int) Or(x, y *Int) *Int {
	z.values.Or(x.values, y.values)
	z.bits = max(x.bits, y.bits)
	z.setSign()
	return z
}

// Rsh sets z to x>>n and returns z.
func (z *Int) Rsh(x *Int, n uint) *Int {
	if z != x {
		z.bits = x.bits
		z.values.Set(x.values)
	}
	z.values.Rsh(z.values, n)
	z.setSign()
	return z
}

// SetBig sets z to x and returns z.
func (z *Int) SetBig(x *big.Int) *Int {
	z.bits = x.BitLen()
	if z.bits > 0 && x.Sign() == 1 && x.Bit(z.bits-1) == 1 {
		z.bits++
	}
	z.values = new(big.Int).Set(x)
	return z
}

func (z *Int) SetString(s string, base int) (*Int, bool) {
	i, ok := new(big.Int).SetString(s, base)
	if !ok {
		return nil, false
	}
	z.SetBig(i)
	return z, true
}

func (z *Int) Sign() int {
	return z.values.Sign()
}

// Sub sets z to x-y and returns z.
func (z *Int) Sub(x, y *Int) *Int {
	return z.bin(circuits.NewSubtractor, x, y)
}

// Xor sets z to x^y and returns z.
func (z *Int) Xor(x, y *Int) *Int {
	z.values.Xor(x.values, y.values)
	z.bits = max(x.bits, y.bits)
	z.setSign()
	return z
}

type binaryOp func(cc *circuits.Compiler, x, y, z []*circuits.Wire) error

func (z *Int) bin(op binaryOp, x, y *Int) *Int {
	calloc := circuits.NewAllocator()
	inputs := circuit.IO{
		newIOArg("x", types.TInt, x.bits),
		newIOArg("y", types.TInt, y.bits),
	}
	outputs := circuit.IO{
		newIOArg("z", types.TInt, max(x.bits, y.bits)),
	}
	i0w := calloc.Wires(inputs[0].Type.Bits)
	i1w := calloc.Wires(inputs[1].Type.Bits)
	var inputWires []*circuits.Wire
	inputWires = append(inputWires, i0w...)
	inputWires = append(inputWires, i1w...)

	outputWires := calloc.Wires(outputs[0].Type.Bits)
	for idx := range outputWires {
		outputWires[idx].SetOutput(true)
	}

	cc, err := circuits.NewCompiler(utils.NewParams(), calloc, inputs, outputs,
		inputWires, outputWires)
	if err != nil {
		panic(err)
	}

	err = op(cc, i0w, i1w, outputWires)
	if err != nil {
		panic(err)
	}

	circ := cc.Compile()

	obits, err := circ.Compute([]*big.Int{x.values, y.values})
	if err != nil {
		panic(err)
	}

	z.bits = int(outputs[0].Type.Bits)
	z.values = obits[0]
	z.setSign()

	return z
}

func (z *Int) setSign() {
	var sign int
	if z.values.Bit(z.bits-1) == 1 {
		sign = -1
	} else {
		sign = 1
	}
	zs := z.values.Sign()
	if sign != zs {
		z.values.Neg(z.values)
	}
}

func newIOArg(name string, t types.Type, size int) circuit.IOArg {
	return circuit.IOArg{
		Name: name,
		Type: types.Info{
			Type:       t,
			IsConcrete: true,
			Bits:       types.Size(size),
		},
	}
}
