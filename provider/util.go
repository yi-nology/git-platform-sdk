package provider

import (
	"encoding/hex"
)

// isCommitSHA checks if a string is a valid 40-character hex SHA.
func isCommitSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
