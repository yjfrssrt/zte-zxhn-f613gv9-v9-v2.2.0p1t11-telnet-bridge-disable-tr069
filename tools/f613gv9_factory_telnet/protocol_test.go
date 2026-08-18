package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestParseReRandBinaryMAC(t *testing.T) {
	body := append([]byte("re_rand=59&320913&"), []byte{0x1c, 0x67, 0x4a, 0xf5, 0x93, 0x03}...)
	serverRand, seed, serverMAC, err := parseReRand(body)
	if err != nil {
		t.Fatal(err)
	}
	if serverRand != 59 || seed != 320913 || serverMAC != [6]byte{0x1c, 0x67, 0x4a, 0xf5, 0x93, 0x03} {
		t.Fatalf("unexpected parse result: %d %d %x", serverRand, seed, serverMAC)
	}
}

func TestSessionKeyIndex(t *testing.T) {
	key, index := deriveSessionKey(0, 59)
	if index != 59 || len(key) != 24 {
		t.Fatalf("index=%d keylen=%d", index, len(key))
	}
	for i := range key {
		if key[i] != newKeyPool[index+i]^0xA5 {
			t.Fatalf("key byte %d mismatch", i)
		}
	}
}

func TestF613GV9TwelveByteSuffix(t *testing.T) {
	key, _ := deriveSessionKey(0, 59)
	plain := make([]byte, 32)
	copy(plain, []byte("FactoryMode.gch"))
	ciphertext := encryptAlignedForTest(t, plain, key)
	response := append(ciphertext, bytes.Repeat([]byte{0xA7}, 12)...)
	if err := verifyFactoryMarker(response, key); err != nil {
		t.Fatal(err)
	}
}

func TestObservedF613GV9MarkerCapture(t *testing.T) {
	// This 44-byte response is the non-credential CheckLoginAuth response
	// observed on the tested F613GV9. With re_rand=59 the first 32 bytes must
	// decrypt to FactoryMode.gch; the final 12 bytes are the firmware suffix.
	response, err := base64.StdEncoding.DecodeString("0bTI3/GA73nB3eVX5cndffvFkBnuhTFALYkBWk34xQV8rUa2GIO1s8itRrY=")
	if err != nil {
		t.Fatal(err)
	}
	key, index := deriveSessionKey(0, 59)
	if index != 59 {
		t.Fatalf("unexpected key index %d", index)
	}
	if err := verifyFactoryMarker(response, key); err != nil {
		t.Fatal(err)
	}
}

func TestFactoryMarkerRejectsUnknownSuffix(t *testing.T) {
	key, _ := deriveSessionKey(0, 59)
	plain := make([]byte, 32)
	copy(plain, []byte("FactoryMode.gch"))
	ciphertext := encryptAlignedForTest(t, plain, key)
	if err := verifyFactoryMarker(append(ciphertext, 1, 2, 3, 4, 5), key); err == nil {
		t.Fatal("unknown suffix accepted")
	}
}

func TestMACProofReferenceAndOtherMAC(t *testing.T) {
	referenceMAC := [6]byte{0x00, 0x07, 0x29, 0x55, 0x35, 0x57}
	reference, err := base64.StdEncoding.DecodeString("AAAAAGAIAACTBwAAOggAALoAAACQBwAAxAcAAMoGAACVBAAATggAAM0BAAAnCA==")
	if err != nil {
		t.Fatal(err)
	}
	if !verifyMACPayload(reference, referenceMAC) {
		t.Fatal("public reference payload is invalid")
	}
	for _, mac := range [][6]byte{
		{0, 0, 0, 0, 0, 0},
		{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		{0x11, 0x22, 0x33, 0x44, 0x55, 0x66},
		{0xde, 0xad, 0xbe, 0xef, 0, 1},
	} {
		payload := macMagicPayload(mac)
		if len(payload) != 46 || !verifyMACPayload(payload, mac) {
			t.Fatalf("generated payload invalid for %x", mac)
		}
	}
}

func TestTemporaryCredentialParser(t *testing.T) {
	user, pass, err := parseTemporaryCredentials([]byte("FactoryModeAuth.gch?user=Temp_123&pass=Pass-456"))
	if err != nil {
		t.Fatal(err)
	}
	if user != "Temp_123" || pass != "Pass-456" {
		t.Fatalf("unexpected credentials %q %q", user, pass)
	}
}

func TestParseMAC(t *testing.T) {
	mac, err := parseMAC("AA-BB-CC-DD-EE-FF")
	if err != nil {
		t.Fatal(err)
	}
	if mac != [6]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff} {
		t.Fatalf("unexpected MAC %x", mac)
	}
}

func encryptAlignedForTest(t *testing.T, plain, key []byte) []byte {
	t.Helper()
	if len(plain)%16 != 0 {
		t.Fatal("test plaintext is not aligned")
	}
	result, err := encryptZeroPadded(plain[:len(plain)-1], key)
	if err != nil {
		t.Fatal(err)
	}
	// encryptZeroPadded adds one zero byte to the 31-byte slice, producing the
	// exact 32-byte aligned plaintext requested by the test.
	return result
}
