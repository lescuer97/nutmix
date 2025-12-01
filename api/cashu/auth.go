package cashu

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	gonutsCrypto "github.com/elnosh/gonuts/crypto"
)

var (
	ErrInvalidAuthToken  = errors.New("Invalid auth token")
	ErrClearTokenExpired = errors.New("Clear to token is expired")
)

type ProtectedRoute struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type Nut21Info struct {
	OpenIdDiscovery string           `json:"openid_discovery"`
	ClientId        string           `json:"client_id"`
	ProtectedRoutes []ProtectedRoute `json:"protected_endpoints"`
}

type PostAuthBlindMintRequest struct {
	Outputs []BlindedMessage `json:"outputs"`
}

type AuthProof struct {
	Id     string           `json:"id"`
	Secret string           `json:"secret"`
	C      WrappedPublicKey `json:"C" db:"c"`
	Amount uint64           `json:"amount" db:"amount"`
}

func (a AuthProof) Y() (WrappedPublicKey, error) {
	// Get Hash to curve of secret
	parsedSecret := []byte(a.Secret)

	y, err := gonutsCrypto.HashToCurve(parsedSecret)

	if err != nil {
		return WrappedPublicKey{}, fmt.Errorf("crypto.HashToCurve: %+v", err)
	}

	return WrappedPublicKey{y}, nil
}

// creates a normal proof for storage
func (a AuthProof) Proof(y WrappedPublicKey, state ProofState) Proof {
	var proof Proof

	proof.Amount = a.Amount
	proof.Id = a.Id
	proof.Y = y
	proof.C = a.C
	proof.Secret = a.Secret
	proof.SeenAt = time.Now().Unix()
	proof.State = state

	return proof
}

type AuthClams struct {
	Sub      string    `json:"sub"`
	ClientId string    `json:"client_id"`
	Aud      *[]string `json:"aud"`
}

type PostAuthBlindMintResponse struct {
	Signatures []BlindSignature `json:"signatures"`
}
type Nut22Info struct {
	BatMaxMint      uint64           `json:"bat_max_mint"`
	ProtectedRoutes []ProtectedRoute `json:"protected_endpoints"`
}

func ConvertRouteListToProtectedRouteList(list []string) []ProtectedRoute {

	routes := []ProtectedRoute{}

	for _, v := range list {
		routes = append(routes, ProtectedRoute{
			Method: "POST",
			Path:   v,
		}, ProtectedRoute{
			Method: "GET",
			Path:   v,
		},
		)

	}
	return routes
}

func DecodeAuthToken(tokenstr string) (AuthProof, error) {
	prefixVersion := tokenstr[:5]
	base64Token := tokenstr[5:]
	if prefixVersion != "authA" {
		return AuthProof{}, ErrInvalidAuthToken
	}

	tokenBytes, err := base64.URLEncoding.DecodeString(base64Token)
	if err != nil {
		tokenBytes, err = base64.RawURLEncoding.DecodeString(base64Token)
		if err != nil {
			return AuthProof{}, fmt.Errorf("error decoding token: %v", err)
		}
	}

	var authProof AuthProof
	err = json.Unmarshal(tokenBytes, &authProof)
	if err != nil {
		return AuthProof{}, fmt.Errorf("cbor.Unmarshal: %v", err)
	}

	return authProof, nil
}
