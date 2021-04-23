package server

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Signature is a sign info of an image
type Signature struct {
	Name               string      `json:"Name"`
	SignedTags         []SignedTag `json:"SignedTags"`
	Signers            []Signer    `json:"Signer"`
	AdministrativeKeys []AdminKey  `json:"AdministrativeKeys"`
}

// SignedTag is a tag-signature info
type SignedTag struct {
	SignedTag string   `json:"SignedTag"`
	Digest    string   `json:"Digest"`
	Signers   []string `json:"Signers"`
}

// Signer is a signer struct
type Signer struct {
}

// AdminKey is a key for admin
type AdminKey struct {
	Name string `json:"Name"`
	Keys []Key  `json:"Keys"`
}

// Key is a key struct
type Key struct {
	ID string `json:"ID"`
}

func (s *Signature) getRepoAdminKey() string {
	for _, key := range s.AdministrativeKeys {
		if key.Name == "Repository" {
			return key.Keys[0].ID
		}
	}

	return ""
}

func getSignatures(raw string) ([]Signature, error) {
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	signaturePart := raw[start:(end + 1)]

	if start < 0 || end < 0 {
		return nil, fmt.Errorf("raw signature format invalid")
	}

	var signatures []Signature
	if err := json.Unmarshal([]byte(signaturePart), &signatures); err != nil {
		return nil, err
	}

	return signatures, nil
}
