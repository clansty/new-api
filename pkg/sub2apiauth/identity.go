package sub2apiauth

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type Identity struct {
	BaseURL  string
	Username string
	Password string
}

func NormalizeRootURL(baseURL string) string {
	trimmedBase := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return strings.TrimSuffix(trimmedBase, "/v1")
}

func (identity Identity) Key() string {
	parts := []string{
		NormalizeRootURL(identity.BaseURL),
		strings.TrimSpace(identity.Username),
		identity.Password,
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(digest[:])
}
