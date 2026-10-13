package auth

import (
	"bytes"
	"testing"
)

func TestDigestIsStableAndDoesNotExposePlaintext(t *testing.T) {
	first := Digest("motif_test_secret")
	second := Digest("motif_test_secret")
	if !bytes.Equal(first, second) {
		t.Fatal("Digest returned different values for the same secret")
	}
	if bytes.Contains(first, []byte("motif_test_secret")) {
		t.Fatal("Digest contains the plaintext secret")
	}
}
