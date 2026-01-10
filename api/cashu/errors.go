package cashu

import (
	"errors"
)

var (
	ErrMeltAlreadyPaid            = errors.New("melt already paid")
	ErrQuoteIsPending             = errors.New("quote is pending")
	ErrUnitNotSupported           = errors.New("unit not supported")
	ErrDifferentInputOutputUnit   = errors.New("different input output unit")
	ErrNotEnoughtProofs           = errors.New("not enough proofs")
	ErrProofSpent                 = errors.New("proof already spent")
	ErrBlindMessageAlreadySigned  = errors.New("blind message already signed")
	ErrCommonSecretNotCorrectSize = errors.New("proof secret is not correct size")
	ErrUnknown                    = errors.New("unknown error")
)

type ErrorCode uint

const (
	PROOF_VERIFICATION_FAILED ErrorCode = 10001

	PROOF_ALREADY_SPENT         ErrorCode = 11001
	PROOFS_PENDING              ErrorCode = 11002
	OUTPUTS_ALREADY_SIGNED      ErrorCode = 11003
	OUTPUTS_PENDING             ErrorCode = 11004
	TRANSACTION_NOT_BALANCED    ErrorCode = 11005
	INSUFICIENT_FEE             ErrorCode = 11006
	DUPLICATE_INPUTS            ErrorCode = 11007
	DUPLICATE_OUTPUTS           ErrorCode = 11008
	MULTIPLE_UNITS_OUTPUT_INPUT ErrorCode = 11009
	INPUT_OUTPUT_NOT_SAME_UNIT  ErrorCode = 11010
	UNIT_NOT_SUPPORTED          ErrorCode = 11013

	KEYSET_NOT_KNOW ErrorCode = 12001
	INACTIVE_KEYSET ErrorCode = 12002

	REQUEST_NOT_PAID         ErrorCode = 20001
	QUOTE_ALREADY_ISSUED     ErrorCode = 20002
	MINTING_DISABLED         ErrorCode = 20003
	LIGHTNING_PAYMENT_FAILED ErrorCode = 20004
	QUOTE_PENDING            ErrorCode = 20005
	INVOICE_ALREADY_PAID     ErrorCode = 20006

	MINT_QUOTE_INVALID_SIG     ErrorCode = 20008
	MINT_QUOTE_INVALID_PUB_KEY ErrorCode = 20009

	ENDPOINT_REQUIRES_CLEAR_AUTH ErrorCode = 30001
	CLEAR_AUTH_FAILED            ErrorCode = 30002

	ENDPOINT_REQUIRES_BLIND_AUTH    ErrorCode = 31001
	BLIND_AUTH_FAILED               ErrorCode = 31002
	MAXIMUM_BAT_MINT_LIMIT_EXCEEDED ErrorCode = 31003
	MAXIMUM_BAT_RATE_LIMIT_EXCEEDED ErrorCode = 31004

	UNKNOWN ErrorCode = 99999
)

func (e ErrorCode) String() string {

	error := ""
	switch e {
	case OUTPUTS_ALREADY_SIGNED:
		error = "Blinded message of output already signed"
	case PROOF_VERIFICATION_FAILED:
		error = "Proof could not be verified"

	case PROOF_ALREADY_SPENT:
		error = "Proof is already spent"
	case PROOFS_PENDING:
		error = "Proofs are pending"
	case OUTPUTS_PENDING:
		error = "Outputs are pending"
	case TRANSACTION_NOT_BALANCED:
		error = "Transaction is not balanced (inputs != outputs)"
	case UNIT_NOT_SUPPORTED:
		error = "Unit in request is not supported"
	case INSUFICIENT_FEE:
		error = "Insufficient fee"
	case DUPLICATE_INPUTS:
		error = "Duplicate inputs provided"
	case DUPLICATE_OUTPUTS:
		error = "Duplicate inputs provided"
	case MULTIPLE_UNITS_OUTPUT_INPUT:
		error = "Inputs/Outputs of multiple units"
	case INPUT_OUTPUT_NOT_SAME_UNIT:
		error = "Inputs and outputs are not same unit"

	case KEYSET_NOT_KNOW:
		error = "Keyset is not known"
	case INACTIVE_KEYSET:
		error = "Keyset is inactive, cannot sign messages"
	case MINT_QUOTE_INVALID_SIG:
		error = "No valid signature was provided"
	case MINT_QUOTE_INVALID_PUB_KEY:
		error = "No public key for mint quote"

	case REQUEST_NOT_PAID:
		error = "Quote request is not paid"
	case QUOTE_ALREADY_ISSUED:
		error = "Quote has already been issued"
	case MINTING_DISABLED:
		error = "Minting is disabled"
	case QUOTE_PENDING:
		error = "Quote is pending"
	case INVOICE_ALREADY_PAID:
		error = "Invoice already paid"

	case ENDPOINT_REQUIRES_CLEAR_AUTH:
		error = "Endpoint requires clear auth"
	case CLEAR_AUTH_FAILED:
		error = "Clear authentification failed"

	case ENDPOINT_REQUIRES_BLIND_AUTH:
		error = "Endpoint requires blind auth"
	case BLIND_AUTH_FAILED:
		error = "Blind authentification failed"
	case MAXIMUM_BAT_MINT_LIMIT_EXCEEDED:
		error = "Maximum Blind auth token amounts execeeded"
	case MAXIMUM_BAT_RATE_LIMIT_EXCEEDED:
		error = "Maximum BAT rate limit execeeded"
	}

	return error
}

type ErrorResponse struct {
	Detail *string   `json:"detail,omitempty"`
	Error  string    `json:"error,omitempty"`
	Code   ErrorCode `json:"code"`
}

func ErrorCodeToResponse(code ErrorCode, detail *string) ErrorResponse {

	return ErrorResponse{
		Code:   code,
		Error:  code.String(),
		Detail: detail,
	}
}
