// Copyright 2021 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cosign

import (
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pkg/errors"
	"github.com/sigstore/cosign/pkg/cosign"
	"github.com/sigstore/cosign/pkg/oci"
	ociremote "github.com/sigstore/cosign/pkg/oci/remote"
	"github.com/sigstore/sigstore/pkg/signature"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	validLog = logf.Log.WithName("cosign/validation.go")
)

func Valid(ctx context.Context, ref name.Reference, signer []string, keys []crypto.PublicKey, opts ...ociremote.Option) ([]oci.Signature, error) {
	if len(keys) == 0 {
		// If there are no keys,
		msg := "There are no keys for valid"
		return nil, errors.Errorf(msg)
	}
	// We return nil if ANY key matches
	var lastErr error
	for _, k := range keys {
		verifier, err := signature.LoadVerifier(k, crypto.SHA256)
		if err != nil {
			msg := fmt.Sprintf("Error creating verifier: %v", err)
			validLog.Error(err, msg)
			lastErr = err
			continue
		}

		sps, err := validSignatures(ctx, ref, signer, verifier, opts...)
		if err != nil {
			msg := fmt.Sprintf("Error validating signatures: %v", err)
			validLog.Error(err, msg)
			lastErr = err
			continue
		}
		return sps, nil
	}
	validLog.Info("No valid signatures were found.")
	return nil, lastErr
}

// For testing
var cosignVerifySignatures = cosign.VerifyImageSignatures

func validSignatures(ctx context.Context, ref name.Reference, policySigners []string, verifier signature.Verifier, opts ...ociremote.Option) ([]oci.Signature, error) {
	// allow insecure registry [x509 error fix]
	opts = append(opts, ociremote.WithRemoteOptions(remote.WithTransport(&http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}})))

	var lastErr error
	var lastSig []oci.Signature

	for _, signer := range policySigners {
		// do cosign verify signature & signer annotations
		sigs, _, err := cosignVerifySignatures(ctx, ref, &cosign.CheckOpts{
			RegistryClientOpts: opts,
			RootCerts:          nil,
			SigVerifier:        verifier,
			ClaimVerifier:      cosign.SimpleClaimVerifier,
			Annotations: map[string]interface{}{
				"signer": signer,
			},
		})
		msg := fmt.Sprintf("%v", sigs)
		validLog.Info(msg)
		// if signature is valid & signer is valid, return sig
		if err == nil {
			return sigs, nil
		}
		lastErr = err
		lastSig = sigs
	}
	// return error because it is invalid
	return lastSig, lastErr
}

func GetPublicKey(cfg map[string][]byte) ([]crypto.PublicKey, error) {
	keys := []crypto.PublicKey{}
	errs := []error{}

	validLog.Info("Get Public key...")
	pems := parsePems(cfg["cosign.pub"])
	for _, p := range pems {
		// TODO: check header
		key, err := x509.ParsePKIXPublicKey(p.Bytes)
		if err != nil {
			errs = append(errs, err)
		} else {
			keys = append(keys, key.(crypto.PublicKey))
		}
	}
	if keys == nil {
		msg := fmt.Sprintf("malformed cosign.pub: %v", errs)
		return nil, errors.Wrap(errs[0], msg)
	}
	return keys, nil
}

func parsePems(b []byte) []*pem.Block {
	p, rest := pem.Decode(b)
	if p == nil {
		return nil
	}
	pems := []*pem.Block{p}

	if rest != nil {
		return append(pems, parsePems(rest)...)
	}
	return pems
}
