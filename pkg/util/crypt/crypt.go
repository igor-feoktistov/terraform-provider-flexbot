package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

func createHash(key string) []byte {
	hasher := sha256.New()
	hasher.Write([]byte(key))
	return hasher.Sum(nil)
}

// Encrypt encrypts byte array
func Encrypt(data []byte, passphrase string) (b []byte, err error) {
	var block cipher.Block
	if block, err = aes.NewCipher(createHash(passphrase)); err != nil {
		return
	}
	var gcm cipher.AEAD
	if gcm, err = cipher.NewGCM(block); err != nil {
		return
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return
	}
	b = gcm.Seal(nonce, nonce, data, nil)
	return
}

// Decrypt decrypts byte array
func Decrypt(data []byte, passphrase string) (b []byte, err error) {
	key := createHash(passphrase)
	var block cipher.Block
	if block, err = aes.NewCipher(key); err != nil {
		return
	}
	var gcm cipher.AEAD
	if gcm, err = cipher.NewGCM(block); err != nil {
		return
	}
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	if b, err = gcm.Open(nil, nonce, ciphertext, nil); err != nil {
		return
	}
	return
}

// EncryptString encrypts string
func EncryptString(decrypted string, passPhrase string) (encrypted string, err error) {
	var b []byte
	if !strings.HasPrefix(decrypted, "base64:") {
		if b, err = Encrypt([]byte(decrypted), passPhrase); err != nil {
			err = fmt.Errorf("Encrypt() failure: %s", err)
			return
		}
		encrypted = "base64:" + base64.StdEncoding.EncodeToString(b)
	} else {
		encrypted = decrypted
	}
	return
}

// DecryptString decrypts string
func DecryptString(encrypted string, passPhrase string) (decrypted string, err error) {
	var b, b64 []byte
	if strings.HasPrefix(encrypted, "base64:") {
		if b64, err = base64.StdEncoding.DecodeString(encrypted[7:]); err != nil {
			err = fmt.Errorf("base64.StdEncoding.DecodeString() failure: %s", err)
			return
		}
		if b, err = Decrypt(b64, passPhrase); err != nil {
			err = fmt.Errorf("Decrypt() failure: %s", err)
			return
		}
		decrypted = string(b)
	} else {
		decrypted = encrypted
	}
	return
}
