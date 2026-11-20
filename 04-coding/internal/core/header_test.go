package core

import (
	"errors"
	"testing"
)

func TestUnpackValidationOrder(t *testing.T) {
	blob := make([]byte, Overhead)
	check := func(input []byte, want error) {
		t.Helper()
		got, err := unpack(input)
		if got != nil || !errors.Is(err, want) {
			t.Fatalf("unpack = %v, %v; want %v", got, err, want)
		}
	}
	check(blob[:Overhead-1], ErrTooShort)
	check(blob, ErrBadMagic)
	copy(blob, magic)
	check(blob, ErrUnsupportedVersion)
	blob[4] = version
	check(blob, ErrBadParams)
}

func TestUnpackParamBounds(t *testing.T) {
	for _, tc := range []struct {
		name  string
		p     Params
		valid bool
	}{
		{"min", Params{1, 8, 1}, true},
		{"max", Params{10, 1048576, 16}, true},
		{"time zero", Params{0, 64, 1}, false},
		{"time above max", Params{11, 64, 1}, false},
		{"threads zero", Params{1, 64, 0}, false},
		{"threads above max", Params{1, 136, 17}, false},
		{"memory below threads minimum", Params{1, 127, 16}, false},
		{"memory at threads minimum", Params{1, 128, 16}, true},
		{"memory above max", Params{1, 1048577, 1}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			blob := make([]byte, Overhead)
			pack(blob[:HeaderSize], tc.p)
			c, err := unpack(blob)
			if tc.valid {
				if err != nil || c.params != tc.p {
					t.Fatalf("unpack = %v, %v; want params %v", c, err, tc.p)
				}
			} else if !errors.Is(err, ErrBadParams) {
				t.Fatalf("unpack error = %v; want %v", err, ErrBadParams)
			}
		})
	}
}
