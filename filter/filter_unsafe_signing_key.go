package filter

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"slices"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/filter/classification"
)

const UnsafeSigningKeyFilterName = "UnsafeSigningKeyFilter"

var unsafePrivateSigningKeys = make([]ed25519.PrivateKey, 1)

func init() {
	mustRegister(UnsafeSigningKeyFilterName, &UnsafeSigningKeyFilter{})

	// This knowingly uses an unsafe source of randomness. See ELEMENTSEC-2025-1670
	_, priv, err := ed25519.GenerateKey(rand.New(rand.NewSource(0)))
	if err != nil {
		panic(err) // "should never happen"
	}
	unsafePrivateSigningKeys[0] = priv
}

func UnsafeSigningKeys() []ed25519.PublicKey {
	keys := make([]ed25519.PublicKey, len(unsafePrivateSigningKeys))
	for i, priv := range unsafePrivateSigningKeys {
		keys[i] = priv.Public().(ed25519.PublicKey)
	}
	return keys
}

type UnsafeSigningKeyFilter struct {
}

func (u *UnsafeSigningKeyFilter) MakeFor(set *Set) (Instanced, error) {
	return &InstancedUnsafeSigningKeyFilter{
		set: set,
	}, nil
}

type InstancedUnsafeSigningKeyFilter struct {
	set *Set
}

func (f *InstancedUnsafeSigningKeyFilter) Name() string {
	return UnsafeSigningKeyFilterName
}

func (f *InstancedUnsafeSigningKeyFilter) CheckEvent(ctx context.Context, input *Input) ([]classification.Classification, error) {
	// Start by generating our own signature for the event. This requires redaction, so do that first.
	roomVer := gomatrixserverlib.MustGetRoomVersion(input.Event.Version())
	redacted, err := roomVer.RedactEventJSON(input.Event.JSON())
	if err != nil {
		return nil, fmt.Errorf("failed to redact event: %w", err)
	}

	// Sign the redacted event with our unsafe signing keys
	for _, privKey := range unsafePrivateSigningKeys {
		ownServerName := "__policyserv__"
		ownKeyId := gomatrixserverlib.KeyID("ed25519:filter")
		signed, err := gomatrixserverlib.SignJSON(ownServerName, ownKeyId, privKey, redacted)
		if err != nil {
			return nil, fmt.Errorf("failed to sign event: %w", err)
		}

		// Extract both the existing signatures and our new (unsafe) signature
		var object struct {
			Signatures map[string]map[gomatrixserverlib.KeyID]string // server name -> key ID -> signature (unpadded base64)
		}
		if err := json.Unmarshal(signed, &object); err != nil {
			return nil, fmt.Errorf("failed to unmarshal event: %w", err)
		}

		// Convert our signature to raw bytes. We don't compare base64 strings because some servers may (improperly)
		// add padding, which throws off the check. Instead, we just compare the bytes.
		compareSignature, err := base64.RawStdEncoding.DecodeString(object.Signatures[ownServerName][ownKeyId])
		if err != nil {
			return nil, fmt.Errorf("failed to decode own signature: %w", err)
		}
		delete(object.Signatures[ownServerName], ownKeyId) // remove our own signature so we don't generate a false positive

		// Compare the signatures
		for serverName, keyIdToSignatures := range object.Signatures {
			for keyId, signature := range keyIdToSignatures {
				rawSignature, err := base64.RawStdEncoding.DecodeString(signature)
				if err != nil {
					return nil, fmt.Errorf("failed to decode signature (%s|%s): %w", serverName, keyId, err)
				}

				if slices.Equal(compareSignature, rawSignature) {
					log.Printf("[%s | %s] Unsafe signing key used by %s as key ID %s", input.Event.EventID(), input.Event.RoomID().String(), serverName, keyId)
					return []classification.Classification{
						classification.Spam,
						classification.Unsafe,
					}, nil
				}
			}
		}
	}

	return nil, nil
}
