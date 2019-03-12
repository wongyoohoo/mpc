//
// parser.go
//
// Copyright (c) 2019 Markku Rossi
//
// All rights reserved.
//

package circuit

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"

	"github.com/markkurossi/mpc/ot"
)

type Operation byte

const (
	XOR Operation = iota
	AND
	INV
)

var reParts = regexp.MustCompilePOSIX("[[:space:]]+")

func (op Operation) String() string {
	switch op {
	case XOR:
		return "XOR"
	case AND:
		return "AND"
	case INV:
		return "INV"
	default:
		return fmt.Sprintf("{Operation %d}", op)
	}
}

type Circuit struct {
	NumGates int
	NumWires int
	N1       int
	N2       int
	N3       int
	Gates    map[int]*Gate
}

func (c *Circuit) String() string {
	return fmt.Sprintf("#gates=%d, #wires=%d n1=%d, n2=%d, n3=%d",
		c.NumGates, c.NumWires, c.N1, c.N2, c.N3)
}

type Gate struct {
	Inputs  []int
	Outputs []int
	Op      Operation
}

type Enc func(a, b, c []byte) []byte

type TableEntry struct {
	Index int
	Data  []byte
}

type ByIndex []TableEntry

func (a ByIndex) Len() int {
	return len(a)
}

func (a ByIndex) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a ByIndex) Less(i, j int) bool {
	return a[i].Index < a[j].Index
}

func entry(enc Enc, a, b, c *ot.Label) TableEntry {
	var ab []byte
	if a != nil {
		ab = a.Bytes()
	}
	var bb []byte
	if b != nil {
		bb = b.Bytes()
	}
	return TableEntry{
		Index: idx(ab, bb),
		Data:  enc(ab, bb, c.Bytes()),
	}
}

func idx(l0, l1 []byte) int {
	if len(l0) == 0 {
		if len(l1) == 0 {
			return 0
		}
		return int(l1[0]&0x80) >> 7
	} else if len(l1) == 0 {
		return int(l0[0]&0x80) >> 7
	} else {
		return (int(l0[0]&0x80) >> 6) | (int(l1[0]&0x80) >> 7)
	}
}

func (g *Gate) Garble(wires ot.Inputs, enc Enc) ([][]byte, error) {
	var in []ot.Wire
	var out []ot.Wire

	for _, i := range g.Inputs {
		w, ok := wires[i]
		if !ok {
			return nil, fmt.Errorf("Unknown input wire %d", i)
		}
		in = append(in, w)
	}

	for _, o := range g.Outputs {
		w, ok := wires[o]
		if !ok {
			return nil, fmt.Errorf("Unknown output wire %d", o)
		}
		out = append(out, w)
	}

	var table []TableEntry

	switch g.Op {
	case XOR:
		// a b c
		// -----
		// 0 0 0
		// 0 1 1
		// 1 0 1
		// 1 1 0
		a := in[0]
		b := in[1]
		c := out[0]
		table = append(table, entry(enc, a.Label0, b.Label0, c.Label0))
		table = append(table, entry(enc, a.Label0, b.Label1, c.Label1))
		table = append(table, entry(enc, a.Label1, b.Label0, c.Label1))
		table = append(table, entry(enc, a.Label1, b.Label1, c.Label0))

	case AND:
		// a b c
		// -----
		// 0 0 0
		// 0 1 0
		// 1 0 0
		// 1 1 1
		a := in[0]
		b := in[1]
		c := out[0]
		table = append(table, entry(enc, a.Label0, b.Label0, c.Label0))
		table = append(table, entry(enc, a.Label0, b.Label1, c.Label0))
		table = append(table, entry(enc, a.Label1, b.Label0, c.Label0))
		table = append(table, entry(enc, a.Label1, b.Label1, c.Label1))

	case INV:
		// a b c
		// -----
		// 0   1
		// 1   0
		a := in[0]
		c := out[0]
		table = append(table, entry(enc, a.Label0, nil, c.Label1))
		table = append(table, entry(enc, a.Label1, nil, c.Label0))
	}

	sort.Sort(ByIndex(table))

	var result [][]byte
	for _, entry := range table {
		result = append(result, entry.Data)
	}

	return result, nil
}

type Dec func(a, b, data []byte) ([]byte, error)

func (g *Gate) Eval(wires map[int][]byte, dec Dec, garbled [][]byte) (
	[]byte, error) {

	var a []byte
	var aOK bool
	var b []byte
	var bOK bool

	switch g.Op {
	case XOR, AND:
		a, aOK = wires[g.Inputs[0]]
		b, bOK = wires[g.Inputs[1]]

	case INV:
		a, aOK = wires[g.Inputs[0]]
		b = []byte{}
		bOK = true
	}

	if !aOK {
		return nil, fmt.Errorf("No input for wire a found")
	}
	if !bOK {
		return nil, fmt.Errorf("No input for wire b found")
	}

	return dec(a, b, garbled[idx(a, b)])
}

func Parse(in io.Reader) (*Circuit, error) {
	r := bufio.NewReader(in)

	// NumGates NumWires
	line, err := readLine(r)
	if err != nil {
		return nil, err
	}
	if len(line) != 2 {
		fmt.Printf("Line: %v\n", line)
		return nil, errors.New("Invalid 1st line")
	}
	numGates, err := strconv.Atoi(line[0])
	if err != nil {
		return nil, err
	}
	numWires, err := strconv.Atoi(line[1])
	if err != nil {
		return nil, err
	}

	// N1 N2 N3
	line, err = readLine(r)
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	if len(line) != 3 {
		return nil, errors.New("Invalid 2nd line")
	}
	n1, err := strconv.Atoi(line[0])
	if err != nil {
		return nil, err
	}
	n2, err := strconv.Atoi(line[1])
	if err != nil {
		return nil, err
	}
	n3, err := strconv.Atoi(line[2])
	if err != nil {
		return nil, err
	}

	gates := make(map[int]*Gate)
	for gate := 0; ; gate++ {
		line, err = readLine(r)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if len(line) < 3 {
			return nil, fmt.Errorf("Invalid gate: %v", line)
		}
		n1, err := strconv.Atoi(line[0])
		if err != nil {
			return nil, err
		}
		n2, err := strconv.Atoi(line[1])
		if err != nil {
			return nil, err
		}
		if 2+n1+n2+1 != len(line) {
			return nil, fmt.Errorf("Invalid gate: %v", line)
		}

		var inputs []int
		for i := 0; i < n1; i++ {
			v, err := strconv.Atoi(line[2+i])
			if err != nil {
				return nil, err
			}
			inputs = append(inputs, v)
		}

		var outputs []int
		for i := 0; i < n2; i++ {
			v, err := strconv.Atoi(line[2+n1+i])
			if err != nil {
				return nil, err
			}
			outputs = append(outputs, v)
		}
		var op Operation
		switch line[len(line)-1] {
		case "XOR":
			op = XOR
		case "AND":
			op = AND
		case "INV":
			op = INV
		default:
			return nil, fmt.Errorf("Invalid operation '%s'", line[len(line)-1])
		}

		gates[gate] = &Gate{
			Inputs:  inputs,
			Outputs: outputs,
			Op:      op,
		}
	}

	return &Circuit{
		NumGates: numGates,
		NumWires: numWires,
		N1:       n1,
		N2:       n2,
		N3:       n3,
		Gates:    gates,
	}, nil
}

func readLine(r *bufio.Reader) ([]string, error) {
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		if len(line) == 1 {
			continue
		}
		parts := reParts.Split(line[:len(line)-1], -1)
		if len(parts) > 0 {
			return parts, nil
		}
	}
}
