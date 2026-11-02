package cashu

import (
	"strings"
	"testing"
)

func TestVerifyProofConditionOrdinarySecretLength(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{name: "short", secret: "x", wantErr: false},
		{name: "empty", secret: "", wantErr: false},
		{name: "empty JSON array", secret: "[]", wantErr: false},
		{name: "unknown JSON kind", secret: `["OTHER"]`, wantErr: false},
		{name: "UTF-8 at byte limit", secret: strings.Repeat("é", 256), wantErr: false},
		{name: "UTF-8 below byte limit", secret: strings.Repeat("é", 255), wantErr: false},
		{name: "too many UTF-8 bytes", secret: strings.Repeat("é", 257), wantErr: true},
		{name: "too many ASCII bytes", secret: strings.Repeat("a", 513), wantErr: true},
		{name: "malformed known condition", secret: `["P2PK"]`, wantErr: true}, //nolint:gosec // Deliberately malformed condition discriminator, not a credential.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyProofCondition(Proof{Secret: tt.secret}) //nolint:exhaustruct // Only Secret is relevant to this validation test.
			if (err != nil) != tt.wantErr {
				t.Fatalf("VerifyProofCondition() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
