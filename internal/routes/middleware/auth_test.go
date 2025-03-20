package middleware

import "testing"


func TestMintMatchPattern(t *testing.T) {
    mintRegexPattern := "^/v1/mint/.*"

    matches, err :=  matchesPattern("/v1/mint/quote/bolt11", mintRegexPattern)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if !matches {
        t.Errorf(`This path should have passed. "%s"`, "/v1/mint/quote/bolt11")
    }
    matches, err =  matchesPattern("/v1/mint/bolt11", mintRegexPattern)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if !matches {
        t.Errorf(`This path should have passed. "%s"`, "/v1/mint/bolt11")
    }

    matches, err =  matchesPattern("/v1/mint/quote/bolt11/12345", mintRegexPattern)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if !matches {
        t.Errorf(`This path should have passed. "%s"`, "/v1/mint/quote/bolt11/12345")
    }

    matches, err =  matchesPattern("/v1/swap", mintRegexPattern)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if matches {
        t.Errorf(`This path should have not passed. "%s"`, "/v1/swap")
    }

    matches, err =  matchesPattern("/v1/melt/bolt11", mintRegexPattern)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if matches {
        t.Errorf(`This path should have not passed. "%s"`, "/v1/melt/bolt11")
    }
    matches, err =  matchesPattern("/v1/blind/mint", mintRegexPattern)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if matches {
        t.Errorf(`This path should have not passed. "%s"`, "/v1/blind/mint")
    }
    

    // change to use non regex
    mintBolt11Patten := "/v1/mint/bolt11"
    matches, err =  matchesPattern("/v1/melt/bolt11", mintBolt11Patten)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if matches {
        t.Errorf(`This path should have not passed. "%s"`, "/v1/melt/bolt11")
    }
    matches, err =  matchesPattern("/v1/mint/bolt11", mintBolt11Patten)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if !matches {
        t.Errorf(`This path should have passed. "%s"`, "/v1/mint/bolt11")
    }

    matches, err =  matchesPattern("/v1/mint/quote/bolt11/12345", mintBolt11Patten)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if matches {
        t.Errorf(`This path should have not passed. "%s"`, "/v1/mint/quote/bolt11/12345")
    }

    matches, err =  matchesPattern("/v1/swap", mintBolt11Patten)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if matches {
        t.Errorf(`This path should have not passed. "%s"`, "/v1/swap")
    }

    matches, err =  matchesPattern("/v1/blind/mint", mintBolt11Patten)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if matches {
        t.Errorf(`This path should have not passed. "%s"`, "/v1/swap")
    }

}

func TestMeltMatchPattern(t *testing.T) {
    meltRegexPattern := "^/v1/melt/.*"

    matches, err :=  matchesPattern("/v1/melt/quote/bolt11", meltRegexPattern)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if !matches {
        t.Errorf(`This path should have passed. "%s"`, "/v1/melt/quote/bolt11")
    }
    matches, err =  matchesPattern("/v1/melt/bolt11", meltRegexPattern)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if !matches {
        t.Errorf(`This path should have passed. "%s"`, "/v1/melt/bolt11")
    }

    matches, err =  matchesPattern("/v1/melt/quote/bolt11/12345", meltRegexPattern)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if !matches {
        t.Errorf(`This path should have passed. "%s"`, "/v1/melt/quote/bolt11/12345")
    }

    matches, err =  matchesPattern("/v1/swap", meltRegexPattern)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if matches {
        t.Errorf(`This path should have not passed. "%s"`, "/v1/swap")
    }

    matches, err =  matchesPattern("/v1/mint/bolt11", meltRegexPattern)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if matches {
        t.Errorf(`This path should have not passed. "%s"`, "/v1/mint/bolt11")
    }
    matches, err =  matchesPattern("/v1/blind/melt", meltRegexPattern)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if matches {
        t.Errorf(`This path should have not passed. "%s"`, "/v1/blind/mint")
    }
    

    // change to use non regex
    mintBolt11Patten := "/v1/melt/bolt11"
    matches, err =  matchesPattern("/v1/mint/bolt11", mintBolt11Patten)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if matches {
        t.Errorf(`This path should have not passed. "%s"`, "/v1/mint/bolt11")
    }
    matches, err =  matchesPattern("/v1/melt/bolt11", mintBolt11Patten)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if !matches {
        t.Errorf(`This path should have passed. "%s"`, "/v1/melt/bolt11")
    }

    matches, err =  matchesPattern("/v1/melt/quote/bolt11/12345", mintBolt11Patten)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if matches {
        t.Errorf(`This path should have not passed. "%s"`, "/v1/melt/quote/bolt11/12345")
    }

    matches, err =  matchesPattern("/v1/swap", mintBolt11Patten)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if matches {
        t.Errorf(`This path should have not passed. "%s"`, "/v1/swap")
    }

    matches, err =  matchesPattern("/v1/blind/mint", mintBolt11Patten)
    if err != nil {
        t.Fatalf("Should not panic")
    }

    if matches {
        t.Errorf(`This path should have not passed. "%s"`, "/v1/swap")
    }

}
