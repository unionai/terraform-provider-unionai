package provider

import (
	"encoding/base64"
	"fmt"
	"strings"
)

func decodeCredentials(encodedStr string) (endpoint, clientID, clientSecret, org string, err error) {
	// Base64 decode
	decodedBytes, err := base64.StdEncoding.DecodeString(encodedStr)
	if err != nil {
		return "", "", "", "", err
	}

	// Convert to string and split on ":"
	parts := strings.Split(string(decodedBytes), ":")
	if len(parts) != 4 {
		return "", "", "", "", fmt.Errorf("expected 4 parts, got %d", len(parts))
	}

	return parts[0], parts[1], parts[2], parts[3], nil
}
