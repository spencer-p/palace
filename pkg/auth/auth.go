// auth implements simple authentication.
package auth

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"time"
)

type UsersDB interface {
	ValidatePassword(user, password string) error
}

type Manager struct {
	db      UsersDB
	block   cipher.Block
	signKey []byte
	now     func() time.Time
}

func New(db UsersDB, encryptKey []byte, signKey []byte) (*Manager, error) {
	block, err := aes.NewCipher(encryptKey)
	if err != nil {
		return nil, err
	}
	return &Manager{
		db:      db,
		block:   block,
		signKey: signKey,
		now:     time.Now,
	}, nil
}

type authToken struct {
	Username string
	Password string
	AuthedAt time.Time
}

func init() {
	gob.Register(time.Time{})
	gob.Register(authToken{})
}

// Validate checks the authenticity of the token and verifies the user and
// password is valid according to the database. The error is nil if the token is
// valid.
func (a *Manager) Validate(token []byte) error {
	data, err := a.decryptToken(token)
	if err != nil {
		return fmt.Errorf("bad token: %w", err)
	}

	if err := a.db.ValidatePassword(data.Username, data.Password); err != nil {
		return fmt.Errorf("encrypted and signed token does not match actual user: %w", err)
	}

	if a.now().Sub(data.AuthedAt) >= 30*24*time.Hour {
		return fmt.Errorf("token is expired")
	}
	return nil
}

func (a *Manager) decryptToken(raw []byte) (authToken, error) {
	var token authToken
	b64encrypted, b64signature, _ := bytes.Cut(raw, []byte{'|'})
	encrypted, err := unbase64([]byte(b64encrypted))
	if err != nil {
		return token, err
	}
	signature, err := unbase64([]byte(b64signature))
	if err != nil {
		return token, err
	}

	h := hmac.New(sha256.New, a.signKey)
	h.Write(encrypted)
	actualSignature := h.Sum(nil)
	if !hmac.Equal(signature, actualSignature) {
		return token, fmt.Errorf("invalid signature")
	}

	decrypted, err := decrypt(a.block, encrypted)
	if err != nil {
		return token, fmt.Errorf("invalid encrypted data")
	}

	buf := bytes.NewBuffer(decrypted)
	if err := gob.NewDecoder(buf).Decode(&token); err != nil {
		return token, fmt.Errorf("failed to decode data: %w", err)
	}
	return token, nil
}

// RefreshToken validates and generates the token with a fresh timestamp.
func (a *Manager) RefreshToken(token []byte) ([]byte, error) {
	if err := a.Validate(token); err != nil {
		return nil, fmt.Errorf("cannot refresh invalid auth: %w", err)
	}
	data, err := a.decryptToken(token)
	if err != nil {
		return nil, err
	}
	return a.Token(data.Username, data.Password)
}

// Token generates a token for the username password pair, if it is valid in the
// database.
func (a *Manager) Token(user, password string) ([]byte, error) {
	if err := a.db.ValidatePassword(user, password); err != nil {
		return nil, fmt.Errorf("username does not match password: %w", err)
	}

	// Gob encode the data into buf.
	data := authToken{
		Username: user,
		Password: password,
		AuthedAt: a.now(),
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(data); err != nil {
		return nil, fmt.Errorf("failed to encode auth data: %w", err)
	}

	// Encrypt the buf.
	encrypted, err := encrypt(a.block, buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt auth data: %w", err)
	}

	// Generate a signature.
	h := hmac.New(sha256.New, a.signKey)
	h.Write(encrypted)
	signature := h.Sum(nil)

	// Reset the buf and write b64(encrypted)+"|"+b64(signature) as the result.
	// Errors are elided here, if this fails it creates an invalid auth.
	buf.Reset()
	buf.Write(tobase64(encrypted))
	buf.Write([]byte{'|'})
	buf.Write(tobase64(signature))
	return buf.Bytes(), nil
}

// encrypt encrypts a value using the given block in counter mode.
//
// A random initialization vector ( https://en.wikipedia.org/wiki/Block_cipher_mode_of_operation#Initialization_vector_(IV) ) with the length of the
// block size is prepended to the resulting ciphertext.
func encrypt(block cipher.Block, value []byte) ([]byte, error) {
	iv := generateRandomKey(block.BlockSize())
	if iv == nil {
		return nil, errors.New("failed to generate IV")
	}
	// Encrypt it.
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(value, value)
	// Return iv + ciphertext.
	return append(iv, value...), nil
}

// decrypt decrypts a value using the given block in counter mode.
//
// The value to be decrypted must be prepended by a initialization vector
// ( https://en.wikipedia.org/wiki/Block_cipher_mode_of_operation#Initialization_vector_(IV) ) with the length of the block size.
func decrypt(block cipher.Block, value []byte) ([]byte, error) {
	size := block.BlockSize()
	if len(value) > size {
		// Extract iv.
		iv := value[:size]
		// Extract ciphertext.
		value = value[size:]
		// Decrypt it.
		stream := cipher.NewCTR(block, iv)
		stream.XORKeyStream(value, value)
		return value, nil
	}
	return nil, errors.New("decryption failed")
}

// tobase64 encodes data to base64 with URL encoding.
func tobase64(value []byte) []byte {
	encoded := make([]byte, base64.URLEncoding.EncodedLen(len(value)))
	base64.URLEncoding.Encode(encoded, value)
	return encoded
}

// unbase64 is the reverse of tobase64.
func unbase64(value []byte) ([]byte, error) {
	decoded := make([]byte, base64.URLEncoding.DecodedLen(len(value)))
	b, err := base64.URLEncoding.Decode(decoded, value)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}
	return decoded[:b], nil
}

func generateRandomKey(length int) []byte {
	k := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		return nil
	}
	return k
}
