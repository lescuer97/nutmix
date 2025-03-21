package cashu

import "errors"

var (
	ErrMeltAlreadyPaid           = errors.New("Melt already Paid")
	ErrQuoteIsPending            = errors.New("Quote is pending")
	ErrUnitNotSupported          = errors.New("Unit not supported")
	ErrNotEnoughtProofs          = errors.New("Not enought proofs")
	ErrProofSpent                = errors.New("Proof already spent")
	ErrBlindMessageAlreadySigned = errors.New("Blind message already signed")
)

type ErrorCode int

const (
	OUTPUT_BLINDED_MESSAGE_ALREADY_SIGNED = 10002
	TOKEN_NOT_VERIFIED                    = 10003

	TOKEN_ALREADY_SPENT      = 11001
	TRANSACTION_NOT_BALANCED = 11002
	UNIT_NOT_SUPPORTED       = 11005
	INSUFICIENT_FEE          = 11006
	// AMOUNT_OUTSIDE_OF_LIMIT = 11006
	OVERPAID_FEE = 11007

	KEYSET_NOT_KNOW = 12001
	INACTIVE_KEYSET = 12002

	REQUEST_NOT_PAID     = 20001
	TOKEN_ALREADY_ISSUED = 20002
	MINTING_DISABLED     = 20003
	QUOTE_PENDING        = 20005
	INVOICE_ALREADY_PAID = 20006

	UNKNOWN = 99999
)

func (e ErrorCode) String() string {

	error := ""
	switch e {
	case OUTPUT_BLINDED_MESSAGE_ALREADY_SIGNED:
		error = "Blinded message of output already signed"
	case TOKEN_NOT_VERIFIED:
		error = "Proof could not be verified"

	case TOKEN_ALREADY_SPENT:
		error = "Token is already spent"
	case TRANSACTION_NOT_BALANCED:
		error = "Transaction is not balanced (inputs != outputs)"
	case UNIT_NOT_SUPPORTED:
		error = "Unit in request is not supported"
	case INSUFICIENT_FEE:
		error = "Insufficient fee"
	case OVERPAID_FEE:
		error = "Fee over paid"

	case KEYSET_NOT_KNOW:
		error = "Keyset is not known"
	case INACTIVE_KEYSET:
		error = "Keyset is inactive, cannot sign messages"

	case REQUEST_NOT_PAID:
		error = "Quote request is not paid"
	case TOKEN_ALREADY_ISSUED:
		error = "Tokens have already been issued for quote"
	case MINTING_DISABLED:
		error = "Minting is disabled"
	case QUOTE_PENDING:
		error = "Quote is pending"
	case INVOICE_ALREADY_PAID:
		error = "Invoice already paid"
	}

	return error
}

type ErrorResponse struct {
	// integer code
	Code ErrorCode `json:"code"`
	// Human readable error
	Error string `json:"error"`
	// Extended explanation of error
	Detail *string `json:"detail"`
}

func ErrorCodeToResponse(code ErrorCode, detail *string) ErrorResponse {

	return ErrorResponse{
		Code:   code,
		Error:  code.String(),
		Detail: detail,
	}
}
