package cashu

import (
	"encoding/hex"
	"fmt"
	"log"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lescuer97/nutmix/crypto"
	"github.com/tyler-smith/go-bip32"
)


type BlindedMessage struct {
	Amount int32  `json:"amount"`
	Id     string `json:"id"`
	B_     string `json:"B_"`
}

func (b *BlindedMessage) SignBlindedMessage() {

}

type BlindSignature struct {
	Amount int32  `json:"amount"`
	Id     string `json:"id"`
	C_     string `json:"C_"`
}

func GenerateBlindSignature (privateKey *bip32.Key, blindedMessage BlindedMessage) (BlindSignature, error) {

			k := secp256k1.PrivKeyFromBytes(privateKey.Key)

			decodedBlindFactor, err := hex.DecodeString(blindedMessage.B_)

			if err != nil {
				log.Println(fmt.Errorf("DecodeString: %v", err))
                return BlindSignature{}, err
			}

			B_, err := secp256k1.ParsePubKey(decodedBlindFactor)

			if err != nil {
				log.Println(fmt.Errorf("ParsePubKey: %v", err))
                return BlindSignature{}, err
			}

			C_ := crypto.SignBlindedMessage(B_, k)

            return BlindSignature {
				Amount: blindedMessage.Amount,
				Id:     blindedMessage.Id,
				C_:     hex.EncodeToString(C_.SerializeCompressed()),
			}, nil
}


type Proof struct {
	Amount int32  `json:"amount"`
	Id     string `json:"id"`
	Secret string `json:"secret"`
	C     string `json:"C"`
}

type MintError struct {
	Detail string `json:"detail"`
	Code   int8   `json:"code"`
}

type Keyset struct {
	Id        string `json:"id"`
	Active    bool   `json:"active" db:"active"`
	Unit      string `json:"unit"`
	Amount    int    `json:"amount"`
	PrivKey  string `json:"priv_key"`
	CreatedAt int64  `json:"created_at"`
}

func (keyset *Keyset) GetPubKey() ([]byte, error)  {
    privKey, err := bip32.B58Deserialize(keyset.PrivKey)

    if err != nil {
        return nil, err

    }
    return privKey.PublicKey().Key, err


}

type Seed struct {
	Seed      []byte
	Active    bool
	CreatedAt int64
	Unit      string
	Id        string
}

type SwapMintMethod struct {
	Method    string `json:"method"`
	Unit      string `json:"unit"`
	MinAmount int    `json:"min_amount"`
	MaxAmount int    `json:"max_amount"`
}

type SwapMintInfo struct {
	Methods  *[]SwapMintMethod `json:"methods,omitempty"`
	Disabled bool              `json:"disabled"`
}

type GetInfoResponse struct {
	Name            string     `json:"name"`
	Version         string     `json:"version"`
	Pubkey          string     `json:"pubkey"`
	Description     string     `json:"description"`
	DescriptionLong string     `json:"description_long"`
	Contact         [][]string `json:"contact"`
	Motd            string     `json:"motd"`
	Nuts            map[string]SwapMintInfo
}


type KeysetResponse struct {
	Id   string            `json:"id"`
	Unit string            `json:"unit"`
	Keys map[string]string `json:"keys"`
}

type PostMintQuoteBolt11Request struct {
	Amount int64  `json:"amount"`
	Unit   string `json:"unit"`
}

type PostMintQuoteBolt11Response struct {
	Quote string  `json:"quote"`
	Request   string `json:"request"`
	Paid   bool `json:"paid"`
	Expiry   uint64 `json:"expiry"`
}

type PostMintBolt11Request struct {
    Quote string `json:"quote"`
    Outputs []BlindedMessage `json:"outputs"`
}

type PostMintBolt11Response struct {
    Signatures []BlindSignature `json:"signatures"`
}

type BasicKeysetResponse struct {
	Id     string `json:"id"`
	Unit   string `json:"unit"`
	Active bool   `json:"active"`
}

type PostMeltQuoteBolt11Request struct {
    Request string `json:"request"`
    Unit    string `json:"unit"`
}

type PostMeltQuoteBolt11Response struct {
    Quote string `json:"quote"`
    Amount int64 `json:"amount"`
    FeeReserve int64 `json:"fee_reserve"`
    Paid bool `json:"paid"`
    Expiry int64 `json:"expiry"`
}

type PostSwapRequest struct {
    Inputs []Proof `json:"inputs"`
    Outputs []BlindedMessage `json:"outputs"`
}

















