package signer

import (
	"errors"

	"github.com/lescuer97/nutmix/api/cashu"
)

var ErrNoKeysetFound = errors.New("no keyset found")

type GetKeysResponse struct {
	Keysets []KeysetResponse `json:"keysets"`
}
type GetKeysetsResponse struct {
	Keysets []cashu.BasicKeysetResponse `json:"keysets"`
}

type KeysetResponse struct {
	Keys        map[uint64]string `json:"keys"`
	Id          string            `json:"id"`
	Unit        string            `json:"unit"`
	InputFeePpk uint              `json:"input_fee_ppk"`
	Active      bool              `json:"active"`
}

type BasicKeysetResponse struct {
	Id          string `json:"id"`
	Unit        string `json:"unit"`
	Active      bool   `json:"active"`
	InputFeePpk uint   `json:"input_fee_ppk"`
}
