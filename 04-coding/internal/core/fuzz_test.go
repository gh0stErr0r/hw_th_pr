package core

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func FuzzParseContainer(f *testing.F) {
	valid := make([]byte, Overhead)
	pack(valid[:HeaderSize], testParams)
	f.Add(valid)
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, blob []byte) {
		c, err := ParseContainer(blob)
		if err != nil {
			if c != nil {
				t.Fatal("parse error returned a container")
			}
			return
		}
		if !c.params.valid() || len(c.sealed) < TagSize {
			t.Fatal("accepted an invalid container")
		}
		if &c.header[0] != &blob[0] || &c.sealed[0] != &blob[HeaderSize] {
			t.Fatal("parser copied input bytes")
		}
	})
}

func FuzzRoundTrip(f *testing.F) {
	f.Add([]byte("привет"), []byte("password"))
	f.Add([]byte{}, []byte("password"))
	f.Fuzz(func(t *testing.T, data, password []byte) {
		if len(data) > 64<<10 || len(password) > 1024 {
			t.Skip()
		}
		blob, err := Encrypt(data, password, testParams, rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		c, err := ParseContainer(blob)
		if err != nil {
			t.Fatal(err)
		}
		out, err := c.Decrypt(password)
		if err != nil || !bytes.Equal(out, data) {
			t.Fatalf("round trip failed: %v, equal = %v", err, bytes.Equal(out, data))
		}
	})
}
