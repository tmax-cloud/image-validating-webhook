package server

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Signature struct {
	Name               string      `json:"Name"`
	SignedTags         []SignedTag `json: "SignedTags"`
	Signers            []Signer    `json: "Signer"`
	AdministrativeKeys []AdminKey  `json: "AdministrativeKeys"`
}

type SignedTag struct {
	SignedTag string   `json: "SignedTag"`
	Digest    string   `json: "Digest"`
	Signers   []string `json: "Signers"`
}

type Signer struct {
}

type AdminKey struct {
	Name string `json: "Name"`
	Keys []Key  `json: "Keys"`
}

type Key struct {
	ID string `json: "ID"`
}

func getSignatures(raw string) ([]Signature, error) {
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	signaturePart := raw[start:(end + 1)]

	if start < 0 || end < 0 {
		return nil, fmt.Errorf("Raw signature format invalid")
	}

	var signatures []Signature
	if err := json.Unmarshal([]byte(signaturePart), &signatures); err != nil {
		return nil, err
	}

	return signatures, nil
}
