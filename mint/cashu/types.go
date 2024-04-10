package cashu


type BlindedMessage struct {
	Amount int32  `json:"amount"`
	Id     string `json:"id"`
	B_     string `json:"B_"`
}

type BlindSignature struct {
	Amount int32  `json:"amount"`
	Id     string `json:"id"`
	C_     string `json:"C_"`
}

type Proof struct {
	Amount int32  `json:"amount"`
	Id     string `json:"id"`
	Secret string `json:"secret"`
	C_     string `json:"C_"`
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
	PubKey    []byte `json:"pub_key"`
	CreatedAt int64  `json:"created_at"`
}

type Seed struct {
	Seed      []byte
	Active    bool
	CreatedAt int64
	Unit      string
	Id        string
}

