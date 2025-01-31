package signer

import "errors"

var ErrNoKeysetFound = errors.New("No keyset found")

type GetKeysResponse struct {
	Keysets []KeysetResponse `json:"keysets"`
}
type GetKeysetsResponse struct {
	Keysets []BasicKeysetResponse `json:"keysets"`
}

type KeysetResponse struct {
	Id          string            `json:"id"`
	Unit        string            `json:"unit"`
	Keys        map[string]string `json:"keys"`
	InputFeePpk uint              `json:"input_fee_ppk"`
}

type BasicKeysetResponse struct {
	Id          string `json:"id"`
	Unit        string `json:"unit"`
	Active      bool   `json:"active"`
	InputFeePpk uint   `json:"input_fee_ppk"`
}
