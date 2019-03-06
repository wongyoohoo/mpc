//
// encryption_block.go
//
// Copyright (c) 2019 Markku Rossi
//
// All rights reserved.
//

package pkcs1

import (
	"crypto/rand"
	"errors"
	"io"
)

type EncryptionBlockType byte

const (
	BT0 EncryptionBlockType = iota
	BT1
	BT2
)

const (
	MinPadLen = 8
)

// A block type BT, a padding string PS, and the data D shall be
// formatted into an octet string EB, the encryption block.
//
//            EB = 00 || BT || PS || 00 || D .           (1)
func NewEncryptionBlock(bt EncryptionBlockType, blockLen int, data []byte) (
	[]byte, error) {

	padLen := blockLen - 3 - len(data)
	if padLen < MinPadLen {
		return nil, errors.New("Data too long")
	}

	block := make([]byte, blockLen)
	block[0] = 0
	block[1] = byte(bt)

	switch bt {
	case BT0:
		return nil, errors.New("Block type 0 not supported")

	case BT1:
		for i := 0; i < padLen; i++ {
			block[2+i] = 0xff
		}

	case BT2:
		_, err := io.ReadFull(rand.Reader, block[2:padLen+2])
		if err != nil {
			return nil, err
		}
		for i := 0; i < padLen; i++ {
			for block[2+i] == 0 {
				if _, err := rand.Read(block[2+i : 2+i+1]); err != nil {
					return nil, err
				}
			}
		}
	}
	copy(block[3+padLen:], data)

	return block, nil
}
