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
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func generateTestRSAKey(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test RSA key: %v", err)
	}
	return privateKey, &privateKey.PublicKey
}

func TestSignBetaToken(t *testing.T) {
	privateKey, publicKey := generateTestRSAKey(t)
	deviceID := "test-device-001"

	token, err := SignBetaTokenForTest(deviceID, "TEST-CODE", privateKey, 90*24*time.Hour)
	if err != nil {
		t.Fatalf("SignBetaTokenForTest failed: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}

	claims := &BetaClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(_ *jwt.Token) (interface{}, error) {
		return publicKey, nil
	})
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}
	if !parsed.Valid {
		t.Fatal("token is not valid")
	}
}

func TestSignBetaTokenClaims(t *testing.T) {
	privateKey, publicKey := generateTestRSAKey(t)
	deviceID := "test-device-002"

	token, err := SignBetaTokenForTest(deviceID, "TEST-CODE", privateKey, 90*24*time.Hour)
	if err != nil {
		t.Fatalf("SignBetaTokenForTest failed: %v", err)
	}

	claims := &BetaClaims{}
	_, err = jwt.ParseWithClaims(token, claims, func(_ *jwt.Token) (interface{}, error) {
		return publicKey, nil
	})
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	if claims.DeviceID != deviceID {
		t.Errorf("expected DeviceID=%s, got %s", deviceID, claims.DeviceID)
	}
	if claims.ActivationCode != "TEST-CODE" {
		t.Errorf("expected ActivationCode=TEST-CODE, got %s", claims.ActivationCode)
	}
	if claims.Type != "activation" {
		t.Errorf("expected Type=activation, got %s", claims.Type)
	}
	if claims.Issuer != "casdoor" {
		t.Errorf("expected Issuer=casdoor, got %s", claims.Issuer)
	}

	exp := claims.ExpiresAt.Time
	iat := claims.IssuedAt.Time
	duration := exp.Sub(iat)
	expectedDuration := 90 * 24 * time.Hour
	if duration < expectedDuration-time.Second || duration > expectedDuration+time.Second {
		t.Errorf("expected duration ~%v, got %v", expectedDuration, duration)
	}
}

func TestSignBetaTokenExpired(t *testing.T) {
	privateKey, publicKey := generateTestRSAKey(t)
	deviceID := "test-device-003"

	// Create token with negative duration (already expired)
	token, err := SignBetaTokenForTest(deviceID, "TEST-CODE", privateKey, -time.Hour)
	if err != nil {
		t.Fatalf("SignBetaTokenForTest failed: %v", err)
	}

	claims := &BetaClaims{}
	_, err = jwt.ParseWithClaims(token, claims, func(_ *jwt.Token) (interface{}, error) {
		return publicKey, nil
	})
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
	if !strings.Contains(err.Error(), "expired") && !strings.Contains(err.Error(), "token is expired") {
		t.Errorf("expected 'expired' in error, got: %v", err)
	}
}

func TestBetaTokenTamperProof(t *testing.T) {
	privateKey, publicKey := generateTestRSAKey(t)
	deviceID := "test-device-004"

	token, err := SignBetaTokenForTest(deviceID, "TEST-CODE", privateKey, 90*24*time.Hour)
	if err != nil {
		t.Fatalf("SignBetaTokenForTest failed: %v", err)
	}

	// Split token and modify the payload (middle) section
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 token parts, got %d", len(parts))
	}
	parts[1] = parts[1] + "tampered"
	tamperedToken := strings.Join(parts, ".")

	claims := &BetaClaims{}
	_, err = jwt.ParseWithClaims(tamperedToken, claims, func(_ *jwt.Token) (interface{}, error) {
		return publicKey, nil
	})
	if err == nil {
		t.Fatal("expected error for tampered token, got nil")
	}
}
