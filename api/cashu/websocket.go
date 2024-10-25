package cashu

type WebRequestMethod string

const Unsubcribe WebRequestMethod = "unsubscribe"
const Subcribe WebRequestMethod = "subscribe"

type SubscriptionKind string

const Bolt11MeltQuote SubscriptionKind = "bolt11_melt_quote"
const Bolt11MintQuote SubscriptionKind = "bolt11_mint_quote"
const ProofStateWs SubscriptionKind = "proof_state"

type WebRequestParams struct {
	Kind    SubscriptionKind
	SubId   string `json:"subId"`
	Filters []string
}

type WsRequest struct {
	JsonRpc string `json:"jsonrpc"`
	Method  WebRequestMethod
	Params  WebRequestParams
	Id      int
}

