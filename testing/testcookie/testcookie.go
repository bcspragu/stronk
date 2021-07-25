// Package testcookie implements the server.SecureCookie interface for use in
// tests.
package testcookie

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"strings"
)

type SecureCookie struct {
	Secure bool
}

func New() *SecureCookie {
	return &SecureCookie{}
}

func (sc *SecureCookie) Encode(name string, value interface{}) (string, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(value); err != nil {
		return "", fmt.Errorf("failed to gob encode value: %w", err)
	}
	return name + ":" + hex.EncodeToString(buf.Bytes()), nil
}

func (sc *SecureCookie) Decode(name, value string, dst interface{}) error {
	vs := strings.Split(value, ":")
	if n := len(vs); n != 2 {
		return fmt.Errorf("malformed value had %d parts, expected %d", n, 2)
	}
	gotName, enc := vs[0], vs[1]
	if name != gotName {
		return fmt.Errorf("requested name %q, but encoded value was for name %q", name, gotName)
	}
	dat, err := hex.DecodeString(enc)
	if err != nil {
		return fmt.Errorf("encoded value was not a valid hex string: %w", err)
	}
	if err := gob.NewDecoder(bytes.NewReader(dat)).Decode(dst); err != nil {
		return fmt.Errorf("failed to decode val: %w", err)
	}
	return nil
}

func (sc *SecureCookie) UseSecure() bool {
	return sc.Secure
}
