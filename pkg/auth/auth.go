package auth

import (
	"fmt"
	"net/http"
	"time"
)

// TokenType is HTTP Authorization Header's token type
type TokenType string

const (
	// TokenTypeBasic is Basic type token
	TokenTypeBasic TokenType = "Basic"
	// TokenTypeBearer is Bearer type token
	TokenTypeBearer TokenType = "Bearer"
)

// Token is a spec of Token
type Token struct {
	// Type is "Basic" or "Bearer"
	Type TokenType
	// Value...
	Value string
}

// TokenResponse is a spec of TokenResponse
type TokenResponse struct {
	Token        string    `json:"token"`
	AccessToken  string    `json:"access_token"`
	ExpiresIn    int       `json:"expires_in"`
	IssuedAt     time.Time `json:"issued_at"`
	RefreshToken string    `json:"refresh_token"`
}

// RegistryTransport is a spec of token's roundtripper
type RegistryTransport struct {
	Base  http.RoundTripper
	Token *Token
}

// RoundTrip returns base response of cloned request
func (t *RegistryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clonedReq := cloneRequest(req)
	if t.Token != nil {
		clonedReq.Header.Set("Authorization", fmt.Sprintf("%s %s", t.Token.Type, t.Token.Value))
	}

	baseResp, err := t.Base.RoundTrip(clonedReq)
	if err != nil {
		return nil, err
	}

	return baseResp, err
}

// cloneRequest returns a clone of the provided *http.Request.
// The clone is a shallow copy of the struct and its Header map.
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}

	return r2
}
