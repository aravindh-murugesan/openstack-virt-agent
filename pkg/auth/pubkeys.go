package auth

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"fmt"
)

func CreateIOTuneSignature(
	volumeId string,
	ioPolicyKey string,
	ioPolicyValue string,
	privateKey string) (string, error) {

	// Decode OPENSSL Private Key
	privateKeyDer, err := base64.StdEncoding.DecodeString(privateKey)
	if err != nil {
		return "", err
	}

	// Validate Private Key
	privateKeyBytes, err := x509.ParsePKCS8PrivateKey(privateKeyDer)
	if err != nil {
		return "", err
	}
	validatedKey, ok := privateKeyBytes.(ed25519.PrivateKey)
	if !ok {
		return "", fmt.Errorf("invalid ed25519 private key")
	}

	signatureString := fmt.Sprintf("%s-%s-%s", volumeId, ioPolicyKey, ioPolicyValue)

	signature := ed25519.Sign(validatedKey, []byte(signatureString))
	return base64.RawStdEncoding.EncodeToString(signature), nil
}

func ValidateIOTuneSignature(
	volumeId string,
	ioPolicyKey string,
	ioPolicyValue string,
	signature string,
	publicKey string,
) error {

	// Decode Public Key
	pubKeyDer, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		return err
	}

	publicKeyBytes, err := x509.ParsePKIXPublicKey(pubKeyDer)
	if err != nil {
		return err
	}

	validateKey, ok := publicKeyBytes.(ed25519.PublicKey)
	if !ok {
		return fmt.Errorf("invalid ed25519 public key")
	}

	// Validation

	signatureString := fmt.Sprintf("%s-%s-%s", volumeId, ioPolicyKey, ioPolicyValue)

	decodedSignature, err := base64.RawStdEncoding.DecodeString(signature)
	if err != nil {
		return err
	}

	isVerified := ed25519.Verify(validateKey, []byte(signatureString), decodedSignature)

	if !isVerified {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// MatchPrivateKeyToRequestor derives the public key from the given base64-encoded
// Ed25519 private key, and checks if it matches any of the authorized public keys.
// Returns the name of the requestor if found, otherwise an error.
func MatchPrivateKeyToRequestor(privateKeyBase64 string, authorizedKeys map[string]string) (string, error) {
	// Decode OPENSSL Private Key
	privateKeyDer, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode private key base64: %w", err)
	}

	// Validate Private Key
	privateKeyBytes, err := x509.ParsePKCS8PrivateKey(privateKeyDer)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	validatedKey, ok := privateKeyBytes.(ed25519.PrivateKey)
	if !ok {
		return "", fmt.Errorf("invalid ed25519 private key")
	}

	derivedPubKey := validatedKey.Public().(ed25519.PublicKey)

	for name, pubKeyB64 := range authorizedKeys {
		pubKeyDer, err := base64.StdEncoding.DecodeString(pubKeyB64)
		if err != nil {
			continue // skip invalid config keys
		}

		publicKeyBytes, err := x509.ParsePKIXPublicKey(pubKeyDer)
		if err != nil {
			continue // skip invalid config keys
		}

		validateKey, ok := publicKeyBytes.(ed25519.PublicKey)
		if !ok {
			continue // skip non-ed25519 keys
		}

		if derivedPubKey.Equal(validateKey) {
			return name, nil
		}
	}

	return "", fmt.Errorf("private key does not match any authorized public key")
}
