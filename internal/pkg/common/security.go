package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"time"

	"golang.org/x/crypto/hkdf"
)

const (
	SymmetricKey = "2fc08d8662e87cab5b38045e22797a162af67143dcf4f7c5ac2961f30714da8c"
)

func generateSymmetricKey(length int) ([]byte, error) {
	key := make([]byte, length)
	_, err := rand.Read(key)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func SignSignature(challenge string, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(challenge))
	return hex.EncodeToString(h.Sum(nil))
}

func VerifySignature(challenge, secret, signature string) bool {
	expected := SignSignature(challenge, secret)
	return hmac.Equal([]byte(signature), []byte(expected))
}

func GenerateChallenge() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func GetCurrentTimestamp() int64 {
	return time.Now().Unix()
}

func PublicKeyToBase64(publicKey *ecdh.PublicKey) string {
	return base64.StdEncoding.EncodeToString(publicKey.Bytes())
}

func Base64ToPublicKey(base64Str string) (*ecdh.PublicKey, error) {
	pubKeyASN1, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return nil, err
	}
	pubKey, err := ecdh.P384().NewPublicKey(pubKeyASN1)
	if err != nil {
		return nil, err
	}
	return pubKey, nil
}

func PrivateKeyToBase64(privateKey *ecdh.PrivateKey) string {
	return base64.StdEncoding.EncodeToString(privateKey.Bytes())
}

func Base64ToPrivateKey(base64Str string) (*ecdh.PrivateKey, error) {
	privKeyASN1, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return nil, err
	}

	privKey, err := ecdh.P384().NewPrivateKey(privKeyASN1)
	if err != nil {
		return nil, err
	}
	return privKey, nil
}

func DeriveKeys(sharedSecret []byte) (encKey, authKey []byte) {
	// 使用HKDF-SHA256进行密钥派生
	hkdf := hkdf.New(sha256.New, sharedSecret, nil, []byte("FIRMWARE_UPDATE_KEYS"))

	encKey = make([]byte, 32)  // AES-256密钥
	authKey = make([]byte, 32) // HMAC密钥
	io.ReadFull(hkdf, encKey)
	io.ReadFull(hkdf, authKey)

	return
}

func EncryptData(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func DecryptData(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func GenerateRandomStringBase64(length int) (string, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func GenerateRandomStringHex(length int) (string, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
