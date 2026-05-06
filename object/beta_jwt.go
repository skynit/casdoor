// Copyright 2026 The Casdoor Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package object

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/casdoor/casdoor/conf"
	"github.com/golang-jwt/jwt/v5"
)

// BetaClaims represents the JWT claims for a beta activation token.
type BetaClaims struct {
	DeviceID string `json:"device_id"`
	Type     string `json:"type"`
	jwt.RegisteredClaims
}

// parsePrivateKey parses a PEM-encoded RSA private key.
// It tries PKCS#1 first (used by casdoor's generateRsaKeys), then PKCS#8.
func parsePrivateKey(pemData string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Try PKCS#1 first (used by casdoor's generateRsaKeys)
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err == nil {
		return key, nil
	}

	// Try PKCS#8
	keyInterface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	rsaKey, ok := keyInterface.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not an RSA private key")
	}
	return rsaKey, nil
}

// SignBetaTokenForTest signs a beta activation JWT token using an explicitly
// provided RSA private key and duration. Exported for use in tests.
func SignBetaTokenForTest(deviceID string, privateKey *rsa.PrivateKey, duration time.Duration) (string, error) {
	now := time.Now()
	claims := &BetaClaims{
		DeviceID: deviceID,
		Type:     "activation",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "casdoor",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(privateKey)
}

// SignBetaToken signs a beta activation JWT token using the configured
// beta JWT certificate stored in the Cert table.
func SignBetaToken(deviceID string) (string, error) {
	certName := conf.GetBetaJwtCertName()
	if certName == "" {
		return "", fmt.Errorf("beta JWT cert name not configured")
	}

	cert, err := getCert("admin", certName)
	if err != nil {
		return "", fmt.Errorf("failed to load beta JWT cert: %w", err)
	}
	if cert == nil {
		return "", fmt.Errorf("beta JWT cert not found: %s", certName)
	}

	privateKey, err := parsePrivateKey(cert.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("failed to parse beta JWT private key: %w", err)
	}

	duration := time.Duration(conf.GetBetaJwtExpiryHours()) * time.Hour
	return SignBetaTokenForTest(deviceID, privateKey, duration)
}

