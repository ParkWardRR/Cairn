package receipt

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Receipt is the durable acknowledgment the device stores to confirm the
// server has persisted its bundle.
type Receipt struct {
	ReceiptID   uuid.UUID `json:"receipt_id"`
	UploadID    uuid.UUID `json:"upload_id"`
	ContentHash string    `json:"content_hash"`
	IssuedAt    time.Time `json:"issued_at"`
	Signature   string    `json:"signature"` // hex-encoded Ed25519 signature
}

// Signer creates signed receipts.
type Signer struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

// NewSigner creates a Signer. If keyHex is empty, a new ephemeral keypair is
// generated (suitable for development; production should load a persistent key).
func NewSigner(keyHex string) (*Signer, error) {
	if keyHex != "" {
		seed, err := hex.DecodeString(keyHex)
		if err != nil {
			return nil, fmt.Errorf("decode signing key: %w", err)
		}
		priv := ed25519.NewKeyFromSeed(seed)
		return &Signer{
			privateKey: priv,
			publicKey:  priv.Public().(ed25519.PublicKey),
		}, nil
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate signing key: %w", err)
	}
	return &Signer{privateKey: priv, publicKey: pub}, nil
}

// Issue creates a signed receipt for the given upload.
func (s *Signer) Issue(uploadID uuid.UUID, contentHash string) (*Receipt, error) {
	r := &Receipt{
		ReceiptID:   uuid.New(),
		UploadID:    uploadID,
		ContentHash: contentHash,
		IssuedAt:    time.Now().UTC(),
	}

	// Sign the canonical JSON of the receipt fields (excluding signature).
	payload, err := json.Marshal(struct {
		ReceiptID   uuid.UUID `json:"receipt_id"`
		UploadID    uuid.UUID `json:"upload_id"`
		ContentHash string    `json:"content_hash"`
		IssuedAt    time.Time `json:"issued_at"`
	}{r.ReceiptID, r.UploadID, r.ContentHash, r.IssuedAt})
	if err != nil {
		return nil, fmt.Errorf("marshal receipt payload: %w", err)
	}

	sig := ed25519.Sign(s.privateKey, payload)
	r.Signature = hex.EncodeToString(sig)
	return r, nil
}

// PublicKeyHex returns the hex-encoded public key for verification.
func (s *Signer) PublicKeyHex() string {
	return hex.EncodeToString(s.publicKey)
}
