// Package adminauth implements the fixed Argon2id password format used by the
// local provisioning command and the requests service. Parameters are bounded.
package adminauth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"strings"

	"golang.org/x/crypto/argon2"
)

func Hash(password string) string {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		panic(err)
	}
	return encode(salt, password)
}

func encode(salt []byte, password string) string {
	key := argon2.IDKey([]byte(password), salt, 2, 19*1024, 1, 32)
	return "argon2id-v19-m19456-t2-p1$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key)
}

func Verify(hash, password string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 3 || parts[0] != "argon2id-v19-m19456-t2-p1" {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil || len(salt) != 16 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(hash), []byte(encode(salt, password))) == 1
}
