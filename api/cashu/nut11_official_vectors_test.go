package cashu

import (
	"embed"
	"encoding/json"
	"fmt"
	"testing"
)

//go:embed testdata/nut11/vectors.json
var nut11OfficialVectors embed.FS

type nut11VectorSet struct {
	Source       string      `json:"source"`
	SourceSHA256 string      `json:"source_sha256"`
	Cases        []nut11Case `json:"cases"`
}

//nolint:govet // JSON-oriented field order keeps the fixture manifest readable.
type nut11Case struct {
	Request     json.RawMessage `json:"request"`
	SourceBlock int             `json:"source_block"`
	Name        string          `json:"name"`
	Expect      string          `json:"expect"`
	Kind        string          `json:"kind"`
}

func TestNUT11OfficialVectors(t *testing.T) {
	var vectors nut11VectorSet
	data, err := nut11OfficialVectors.ReadFile("testdata/nut11/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	if len(vectors.Cases) != 20 {
		t.Fatalf("loaded %d NUT-11 vectors, want all 20 source blocks", len(vectors.Cases))
	}

	seenBlocks := make(map[int]bool, len(vectors.Cases))
	for _, tt := range vectors.Cases {
		t.Run(tt.Name, func(t *testing.T) {
			if tt.SourceBlock < 1 || tt.SourceBlock > 20 || seenBlocks[tt.SourceBlock] {
				t.Fatalf("invalid or duplicate source block %d", tt.SourceBlock)
			}
			seenBlocks[tt.SourceBlock] = true
			if tt.Expect != "valid" && tt.Expect != "invalid" && tt.Expect != "condition_only" {
				t.Fatalf("unknown expected result %q", tt.Expect)
			}
			wantValid := tt.Expect == "valid"
			switch tt.Kind {
			case "proof":
				var proof Proof
				if err := json.Unmarshal(tt.Request, &proof); err != nil {
					t.Fatal(err)
				}
				condition, err := proof.parseSpendCondition()
				if err != nil {
					t.Fatalf("parse official spend condition: %v", err)
				}
				if err := condition.CheckValid(); err != nil {
					t.Fatalf("official spend condition rejected: %v", err)
				}
				if tt.Expect == "condition_only" {
					return // Source block 1 is an unsigned condition template, not a spend vector.
				}
				valid, err := proof.VerifyP2PK(condition)
				if valid != wantValid || (wantValid && err != nil) {
					t.Fatalf("VerifyP2PK() = (%v, %v), want valid=%v", valid, err, wantValid)
				}
			case "swap":
				var request PostSwapRequest
				if err := json.Unmarshal(tt.Request, &request); err != nil {
					t.Fatal(err)
				}
				if err := validateNUT11FixtureConditions(request.Inputs); err != nil {
					t.Fatal(err)
				}
				assertNUT11VectorResult(t, request.ValidateSigflag(), wantValid)
			case "melt":
				var request PostMeltBolt11Request
				if err := json.Unmarshal(tt.Request, &request); err != nil {
					t.Fatal(err)
				}
				if err := validateNUT11FixtureConditions(request.Inputs); err != nil {
					t.Fatal(err)
				}
				assertNUT11VectorResult(t, request.ValidateSigflag(), wantValid)
			default:
				t.Fatalf("unknown vector kind %q", tt.Kind)
			}
		})
	}
	if len(seenBlocks) != 20 {
		t.Fatalf("covered %d of 20 NUT-11 source blocks", len(seenBlocks))
	}
}

func validateNUT11FixtureConditions(proofs Proofs) error {
	for i, proof := range proofs {
		condition, err := proof.parseSpendCondition()
		if err != nil {
			return fmt.Errorf("input %d parse spend condition: %w", i, err)
		}
		if err := condition.CheckValid(); err != nil {
			return fmt.Errorf("input %d invalid spend condition: %w", i, err)
		}
	}
	return nil
}

func assertNUT11VectorResult(t *testing.T, err error, wantValid bool) {
	t.Helper()
	if (err == nil) != wantValid {
		t.Fatalf("vector validation error = %v, want valid=%v", err, wantValid)
	}
}
