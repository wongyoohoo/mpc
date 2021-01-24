//
// Copyright (c) 2020 Markku Rossi
//
// All rights reserved.
//

package utils

import (
	"io"
)

// Params specify compiler parameters.
type Params struct {
	Verbose   bool
	SSAOut    io.WriteCloser
	SSADotOut io.WriteCloser

	// MaxWireBits specifies the maximum variable width in bits.
	MaxWireBits int

	NoCircCompile bool
	CircOut       io.WriteCloser
	CircDotOut    io.WriteCloser
	CircFormat    string

	CircMultArrayTreshold int

	OptPruneGates bool
}

// NewParams returns new compiler params object, initialized with the
// default values.
func NewParams() *Params {
	return &Params{
		MaxWireBits: 0x20000,
	}
}

// Close closes all open resources.
func (p *Params) Close() {
	if p.SSAOut != nil {
		p.SSAOut.Close()
		p.SSAOut = nil
	}
	if p.SSADotOut != nil {
		p.SSADotOut.Close()
		p.SSADotOut = nil
	}
	if p.CircOut != nil {
		p.CircOut.Close()
		p.CircOut = nil
	}
	if p.CircDotOut != nil {
		p.CircDotOut.Close()
		p.CircDotOut = nil
	}
}
