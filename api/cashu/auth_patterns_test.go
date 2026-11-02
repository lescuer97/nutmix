package cashu

import "testing"

func TestValidateAuthEndpointPattern(t *testing.T) {
	for _, pattern := range []string{"/v1/mint/bolt11", "/v1/mint/*", "/v1/mint/quote.v1"} {
		if err := ValidateAuthEndpointPattern(pattern); err != nil {
			t.Errorf("ValidateAuthEndpointPattern(%q): %v", pattern, err)
		}
	}
	for _, pattern := range []string{"", "not/absolute", "^/v1/mint/.*", "^/v1/mint/", "^/v1/mint/bolt11$", "/v1/(mint|melt)", "/v1/mint/*/quote", "/v1/mint/**"} {
		if err := ValidateAuthEndpointPattern(pattern); err == nil {
			t.Errorf("ValidateAuthEndpointPattern(%q) unexpectedly succeeded", pattern)
		}
	}
}
