//
// Copyright (c) 2022-2024 Markku Rossi
//
// All rights reserved.
//

package bmr

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"

	"github.com/markkurossi/mpc/circuit"
	"github.com/markkurossi/mpc/ot"
	"github.com/markkurossi/text/superscript"
	"github.com/markkurossi/text/symbols"
)

const (
	// Security parameter k specifies the label sizes in bits.
	k = 32
)

// Player implements a multi-party player.
type Player struct {
	Verbose    bool
	ot         ot.OT
	id         int
	numPlayers int
	r          Label
	peers      []*Peer
	c          *circuit.Circuit
	lambda     *big.Int
}

// Peer contains information about a protocol peer.
type Peer struct {
	this *Player
	id   int
	conn ot.IO
	ot   ot.OT
}

func (peer *Peer) consumer() {
	peer.this.Debugf("consumer for peer%s\n", superscript.Itoa(peer.id))
	for {
		v, err := peer.conn.ReceiveByte()
		if err != nil {
			if err != io.EOF {
				fmt.Printf("consumer: %v\n", err)
			}
			return
		}
		op := Operand(v)
		peer.this.Debugf("%s\n", op)
		switch op {
		case OpInit:
			err = peer.ot.InitReceiver(peer.conn)
			if err != nil {
				fmt.Printf("%s: %s\n", op, err)
				return
			}
		}
	}
}

// NewPlayer creates a new multi-party player.
func NewPlayer(id, numPlayers int) (*Player, error) {
	return &Player{
		id:         id,
		ot:         ot.NewCO(),
		numPlayers: numPlayers,
		peers:      make([]*Peer, numPlayers),
	}, nil
}

// Debugf prints debugging message if Verbose debugging is enabled for
// this Player.
func (p *Player) Debugf(format string, a ...interface{}) {
	if !p.Verbose {
		return
	}
	fmt.Printf(format, a...)
}

// IDString returns the player ID as string.
func (p *Player) IDString() string {
	return superscript.Itoa(p.id)
}

// SetCircuit sets the circuit that is evaluated.
func (p *Player) SetCircuit(c *circuit.Circuit) error {
	if len(c.Inputs) != p.numPlayers {
		return fmt.Errorf("invalid circuit: #inputs=%d != #players=%d",
			len(c.Inputs), p.numPlayers)
	}
	p.c = c
	return nil
}

// AddPeer adds a peer.
func (p *Player) AddPeer(idx int, peer ot.IO) {
	p.peers[idx] = &Peer{
		this: p,
		id:   idx,
		conn: peer,
		ot:   ot.NewCO(),
	}
}

// Play runs the protocol with the peers.
func (p *Player) Play() error {
	// Init peers.
	for _, peer := range p.peers {
		if peer != nil {
			// Start consumer.
			go peer.consumer()

			// Init protocol.
			err := peer.conn.SendByte(byte(OpInit))
			if err != nil {
				return err
			}
			err = p.ot.InitSender(peer.conn)
			if err != nil {
				return err
			}
		}
	}

	err := p.offlinePhase()
	if err != nil {
		return err
	}

	return nil
}

// offlinePhase implements the BMR Offline Phase (BMR Figure 2 - Page 6).
func (p *Player) offlinePhase() error {
	var count int
	for _, peer := range p.peers {
		if peer != nil {
			count++
		}
	}
	if count != p.numPlayers-1 {
		return fmt.Errorf("invalid number of peers: expected %d, got %d",
			count, p.numPlayers-1)
	}

	// Step 1: each peer chooses a random key offset R^i.
	r, err := NewLabel()
	if err != nil {
		return err
	}
	p.r = r
	p.Debugf("R%s:\t%v\n", p.IDString(), p.r)

	// Step 2.a: create random permutation bits lambda. We set the
	// bits initially for all wires but later reset the output bits of
	// XOR gates.
	p.lambda, err = rand.Int(rand.Reader,
		big.NewInt(int64((1<<p.c.NumWires)-1)))
	if err != nil {
		return err
	}

	// Optimization for Step 6: set input wire lambdas to 0 for other
	// peers' inputs.
	var inputIndex int
	for id, input := range p.c.Inputs {
		if id != p.id {
			for i := 0; i < int(input.Type.Bits); i++ {
				p.lambda.SetBit(p.lambda, inputIndex+i, 0)
			}
		}
		inputIndex += int(input.Type.Bits)
	}

	wires := make([]Wire, p.c.NumWires)

	// Step 2: create label shares for all wires. We will reset the
	// output labels of XOR gates below.
	for i := 0; i < p.c.NumWires; i++ {
		// 2.b: choose 0-garbled label at random.
		wires[i].L0, err = NewLabel()
		if err != nil {
			return err
		}
		// 2.c: set the 1-garbled label to be: k_{w,1} = k_{w,0} ⊕ R^i
		wires[i].L1 = wires[i].L0
		wires[i].L1.Xor(p.r)
	}

	for i := 0; i < len(wires); i++ {
		p.Debugf("W%d:\t%v\n", i, wires[i])
	}
	p.Debugf("%c%s:\t%v\n", symbols.Lambda, p.IDString(), p.lambda.Text(2))

	// Step 3: patch output wires and permutation bits for XOR output
	// wires.
	for i := 0; i < p.c.NumGates; i++ {
		if p.c.Gates[i].Op != circuit.XOR {
			continue
		}
		i0 := int(p.c.Gates[i].Input0)
		i1 := int(p.c.Gates[i].Input1)
		ow := int(p.c.Gates[i].Output)

		// 3.a: set permutation bit: λ_w = λ_u ⊕ λ_v

		li0 := p.lambda.Bit(i0)
		li1 := p.lambda.Bit(i1)

		lo := li0 ^ li1
		p.lambda.SetBit(p.lambda, ow, lo)

		p.Debugf("%c[%d]: %v ^ %v = %v\n", symbols.Lambda, ow, li0, li1, lo)

		// 3.b: set garbled label on wire 0: k_{w,0} = k_{u,0} ⊕ k_{v,0}
		wires[ow].L0 = wires[i0].L0
		wires[ow].L0.Xor(wires[i1].L0)

		// 3.b: set garbled label on wire 1: k_{w,1} = k_{w,0} ⊕ R^i
		wires[ow].L1 = wires[ow].L0
		wires[ow].L1.Xor(p.r)
	}

	for i := 0; i < len(wires); i++ {
		p.Debugf("W%d:\t%v\n", i, wires[i])
	}

	p.Debugf("%c%s:\t%v\n", symbols.Lambda, p.IDString(), p.lambda.Text(2))

	return nil
}
