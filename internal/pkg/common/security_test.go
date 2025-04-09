package common

import (
	"testing"
)

func TestGenerateSymmetricKey(t *testing.T) {
	length := 32 // 256 bits
	key, err := generateSymmetricKey(length)
	if err != nil {
		t.Fatalf("Failed to generate symmetric key: %v", err)
	}
	if len(key) != length {
		t.Fatalf("Generated key length is incorrect: got %d, want %d", len(key), length)
	}
	t.Logf("Generated symmetric key: %x\n", key)
}
