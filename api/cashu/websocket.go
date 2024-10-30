package cashu

type WebRequestMethod string

const Unsubcribe WebRequestMethod = "unsubscribe"
const Subcribe WebRequestMethod = "subscribe"

type SubscriptionKind string

const Bolt11MeltQuote SubscriptionKind = "bolt11_melt_quote"
const Bolt11MintQuote SubscriptionKind = "bolt11_mint_quote"
const ProofStateWs SubscriptionKind = "proof_state"

type WebRequestParams struct {
	Kind    SubscriptionKind `json:"kind,omitempty"`
	SubId   string           `json:"subId"`
	Filters []string         `json:"filters,omitempty"`
	Payload any              `json:"payload,omitempty"`
}

type WsRequest struct {
	JsonRpc string           `json:"jsonrpc"`
	Method  WebRequestMethod `json:"method"`
	Params  WebRequestParams `json:"params"`
	Id      int              `json:"id"`
}

type WsResponseResult struct {
	Status string `json:"status"`
	SubId  string `json:"subId"`
}

type WsResponse struct {
	JsonRpc string           `json:"jsonrpc"`
	Result  WsResponseResult `json:"result"`
	Id      int              `json:"id"`
}

type WsNotification struct {
	JsonRpc string           `json:"jsonrpc"`
	Method  WebRequestMethod `json:"method"`
	Params  WebRequestParams `json:"params"`
	Id      int              `json:"id"`
}

type ErrorMsg struct {
    Code int64 `json:"code"`
    Message string `json:"message"`

}
type WsError struct {
	JsonRpc string           `json:"jsonrpc"`
    Error ErrorMsg `json:"error"`
	Id      int              `json:"id"`
}
