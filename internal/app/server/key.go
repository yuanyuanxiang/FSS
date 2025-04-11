package server

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
)

// getOrCreatePrivateKey retrieves an existing P384 private key from the specified path or generates a new one if it doesn't exist.
// It returns the private key and any error encountered during the process.
// The function first checks if the file exists at the given path. If it does, it attempts to load the private key from that file.
func getOrCreatePrivateKey(path string) (*ecdh.PrivateKey, error) {
	if _, err := os.Stat(path); err == nil {
		return loadPrivateKey(path)
	}

	// Use P384 curve for ECDH
	privateKey, err := ecdh.P384().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate P384 private key: %w", err)
	}

	if err := savePrivateKey(path, privateKey); err != nil {
		// Try to save in a current directory if the original path fails
		basePath := path[strings.LastIndex(path, "/")+1:]
		if saveErr := savePrivateKey(basePath, privateKey); saveErr != nil {
			return nil, fmt.Errorf("failed to save in both locations: %w", errors.Join(err, saveErr))
		}
		path = basePath
	}

	return privateKey, nil
}

// Load P384 private key (secure version)
func loadPrivateKey(path string) (*ecdh.PrivateKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, errors.New("invalid PEM block")
	}

	// Verify PEM block type
	if block.Type != "ECDH PRIVATE KEY" && block.Type != "EC PRIVATE KEY" {
		return nil, fmt.Errorf("unexpected PEM type: %s", block.Type)
	}

	privKey, err := ecdh.P384().NewPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse P384 key: %w", err)
	}

	return privKey, nil
}

// Save P384 private key (secure version)
func savePrivateKey(path string, privKey *ecdh.PrivateKey) error {
	// Specify file permissions
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	defer func() {
		if r := recover(); r != nil {
			os.Remove(path)
		}
	}()

	pemBlock := &pem.Block{
		Type:  "ECDH PRIVATE KEY",
		Bytes: privKey.Bytes(),
	}

	if err := pem.Encode(file, pemBlock); err != nil {
		return fmt.Errorf("PEM encoding failed: %w", err)
	}

	// Make sure the file is written and closed properly
	return file.Sync()
}
