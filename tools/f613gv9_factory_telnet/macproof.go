package main

import "bytes"

const (
	proofModulus = uint32(2537)
	proofPower   = uint32(1271)
)

var proofPreimages [256]uint16

func init() {
	var found [256]bool
	remaining := 256
	for candidate := uint32(0); candidate < proofModulus && remaining > 0; candidate++ {
		value := proofByte(candidate)
		if !found[value] {
			proofPreimages[value] = uint16(candidate)
			found[value] = true
			remaining--
		}
	}
	if remaining != 0 {
		panic("MAC proof preimage table is incomplete")
	}
}

func proofByte(value uint32) byte {
	value %= proofModulus
	result := uint32(1)
	for exponent := proofPower; exponent > 0; exponent >>= 1 {
		if exponent&1 == 1 {
			result = (result * value) % proofModulus
		}
		value = (value * value) % proofModulus
	}
	return byte(result & 0xff)
}

func macMagicPayload(mac [6]byte) []byte {
	values := make([]uint16, 12)
	for i, value := range mac {
		values[i] = proofPreimages[value]
	}
	payload := make([]byte, 0, 46)
	for _, value := range values[:11] {
		payload = append(payload, byte(value), byte(value>>8), 0, 0)
	}
	return append(payload, byte(values[11]), byte(values[11]>>8))
}

func verifyMACPayload(payload []byte, mac [6]byte) bool {
	var proof [12]byte
	for group := range proof {
		var value uint32
		for offset := range 4 {
			index := 4*group + offset
			if index < len(payload) {
				value |= uint32(payload[index]) << (8 * offset)
			}
		}
		proof[group] = proofByte(value)
	}
	return bytes.Equal(proof[:6], mac[:]) || bytes.Equal(proof[6:], mac[:])
}
