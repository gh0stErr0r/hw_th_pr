package core

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"testing"
)

var testParams = Params{Time: 1, MemoryKiB: 64, Threads: 1}

func hexBytes(t *testing.T, text string) []byte {
	t.Helper()
	b, err := hex.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestRoundTrip(t *testing.T) {
	random := make([]byte, 1<<20)
	if _, err := rand.Read(random); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"one byte", []byte{0xff}},
		{"Russian text", []byte("привет\n")},
		{"1 MiB", random},
	} {
		t.Run(tc.name, func(t *testing.T) {
			password := []byte("password")
			blob, err := Encrypt(tc.data, password, testParams, rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			if len(blob) != len(tc.data)+Overhead {
				t.Fatalf("container has %d bytes, want %d", len(blob), len(tc.data)+Overhead)
			}
			c, err := ParseContainer(blob)
			if err != nil {
				t.Fatal(err)
			}
			got, err := c.Decrypt(password)
			if err != nil || !bytes.Equal(got, tc.data) {
				t.Fatalf("round trip failed: %v, equal = %v", err, bytes.Equal(got, tc.data))
			}
			got, err = c.Decrypt([]byte("wrong"))
			if got != nil || !errors.Is(err, ErrAuthFailed) {
				t.Fatalf("wrong password = %d bytes, %v", len(got), err)
			}
		})
	}
}

func TestRejectTamperingAndTruncation(t *testing.T) {
	password := []byte("password")
	blob, err := Encrypt([]byte("привет"), password, testParams, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	reject := func(input []byte) bool {
		c, err := ParseContainer(input)
		if err != nil {
			return true
		}
		got, err := c.Decrypt(password)
		return got == nil && errors.Is(err, ErrAuthFailed)
	}
	for i := range blob {
		modified := bytes.Clone(blob)
		modified[i] ^= 1
		if !reject(modified) {
			t.Fatalf("accepted mutation at offset %d", i)
		}
	}
	for n := range blob {
		if !reject(blob[:n]) {
			t.Fatalf("accepted container truncated to %d bytes", n)
		}
	}
}

func TestGoldenContainer(t *testing.T) {
	rnd := bytes.NewReader(hexBytes(t, "000102030405060708090a0b0c0d0e0f101112131415161718191a1b"))
	// Independently generated with OpenSSL Argon2id and libsodium AEAD.
	want := hexBytes(t, "47454e4301014000000001000102030405060708090a0b0c0d0e0f101112131415161718191a1b"+
		"0f3b1a5c7ad6c4063c872ea5d439587c3e28186217a46e11c60178d66b")
	got, err := Encrypt([]byte("привет\n"), []byte("password"), testParams, rnd)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("Encrypt = %x, %v; want %x", got, err, want)
	}
}

func TestRFC8439(t *testing.T) {
	plaintext := []byte("Ladies and Gentlemen of the class of '99: If I could offer you only one tip for the future, sunscreen would be it.")
	aad := hexBytes(t, "50515253c0c1c2c3c4c5c6c7")
	key := hexBytes(t, "808182838485868788898a8b8c8d8e8f909192939495969798999a9b9c9d9e9f")
	nonce := hexBytes(t, "070000004041424344454647")
	want := hexBytes(t, "d31a8d34648e60db7b86afbc53ef7ec2a4aded51296e08fea9e2b5a736ee62d6"+
		"3dbea45e8ca9671282fafb69da92728b1a71de0a9e060b2905d6a5b67ecd3b36"+
		"92ddbd7f2d778b8c9803aee328091b58fab324e4fad675945585808b4831d7bc"+
		"3ff4def08e4b7a9de576d26586cec64b61161ae10b594f09e26a7e902ecbd0600691")
	got, err := seal(nil, key, nonce, plaintext, aad)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("seal = %x, %v; want %x", got, err, want)
	}
	got, err = open(nil, key, nonce, want, aad)
	if err != nil || !bytes.Equal(got, plaintext) {
		t.Fatalf("open = %q, %v; want %q", got, err, plaintext)
	}
}

func TestFreshSaltAndNonce(t *testing.T) {
	makeBlob := func() []byte {
		t.Helper()
		blob, err := Encrypt([]byte("text"), []byte("password"), testParams, rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		return blob
	}
	a, b := makeBlob(), makeBlob()
	if bytes.Equal(a[11:27], b[11:27]) || bytes.Equal(a[27:HeaderSize], b[27:HeaderSize]) {
		t.Fatal("encrypt reused salt or nonce")
	}
}

func TestEncryptRejectsInvalidParamsAndShortRandomInput(t *testing.T) {
	if _, err := Encrypt(nil, nil, Params{}, bytes.NewReader(nil)); !errors.Is(err, ErrBadParams) {
		t.Fatalf("zero parameters: %v", err)
	}
	short := bytes.NewReader(make([]byte, SaltSize+NonceSize-1))
	if blob, err := Encrypt(nil, nil, testParams, short); blob != nil || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("short random input = %x, %v", blob, err)
	}
}

func TestDecryptRejectsUninitializedContainer(t *testing.T) {
	for _, c := range []*Container{{}, nil} {
		got, err := c.Decrypt([]byte("password"))
		if got != nil || !errors.Is(err, ErrTooShort) {
			t.Fatalf("Decrypt = %v, %v; want nil and ErrTooShort", got, err)
		}
	}
}

func TestOpenRejectsInvalidKeyLength(t *testing.T) {
	got, err := open(nil, nil, make([]byte, NonceSize), nil, nil)
	if got != nil || err == nil || errors.Is(err, ErrAuthFailed) {
		t.Fatalf("open with invalid key = %v, %v; want AEAD setup error", got, err)
	}
}
