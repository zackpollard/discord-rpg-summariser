package discordgo

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"testing"
)

// TestDAVEDecryptorMatchesStdlibGCM verifies that our hand-rolled GHASH
// produces the same auth tag as Go's stdlib GCM (truncated to 8 bytes).
func TestDAVEDecryptorMatchesStdlibGCM(t *testing.T) {
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("hello DAVE over-the-top-voice-frame-payload")

	// Reference: stdlib GCM with 16-byte tag.
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	stdSealed := gcm.Seal(nil, nonce, plaintext, nil)
	stdCT := stdSealed[:len(stdSealed)-16]
	stdTag := stdSealed[len(stdSealed)-16:]

	// Build what our daveDecryptor.Open expects: ciphertext || 8-byte tag.
	ourInput := make([]byte, len(stdCT)+daveTagSize)
	copy(ourInput, stdCT)
	copy(ourInput[len(stdCT):], stdTag[:daveTagSize])

	block2, _ := aes.NewCipher(key)
	dec := &daveDecryptor{block: block2}
	got, err := dec.Open(nil, nonce, ourInput, nil)
	if err != nil {
		t.Fatalf("Open returned error on valid input: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("plaintext mismatch: got %x want %x", got, plaintext)
	}
}

// TestDAVEDecryptorRejectsWrongKey verifies that decrypting with a wrong
// key returns an error (instead of silently producing garbage, which was
// the pre-auth behaviour that caused screech after epoch transitions).
func TestDAVEDecryptorRejectsWrongKey(t *testing.T) {
	realKey := make([]byte, 16)
	if _, err := rand.Read(realKey); err != nil {
		t.Fatal(err)
	}
	wrongKey := make([]byte, 16)
	if _, err := rand.Read(wrongKey); err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("hello")

	// Encrypt with the real key via stdlib.
	block, _ := aes.NewCipher(realKey)
	gcm, _ := cipher.NewGCM(block)
	sealed := gcm.Seal(nil, nonce, plaintext, nil)
	ct := sealed[:len(sealed)-16]
	tag := sealed[len(sealed)-16 : len(sealed)-16+daveTagSize]

	input := append([]byte{}, ct...)
	input = append(input, tag...)

	// Attempt to decrypt with the wrong key.
	wrongBlock, _ := aes.NewCipher(wrongKey)
	dec := &daveDecryptor{block: wrongBlock}
	_, err := dec.Open(nil, nonce, input, nil)
	if err == nil {
		t.Fatalf("expected auth failure for wrong key but Open succeeded")
	}
}
