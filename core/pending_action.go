package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func newPendingActionToken() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("create approval token: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}
