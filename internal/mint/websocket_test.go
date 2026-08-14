package mint

import (
	"sync"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lescuer97/nutmix/api/cashu"
)

func newObserverForTest() Observer {
	return Observer{
		Proofs:    make(map[string][]ProofWatchChannel),
		MintQuote: make(map[string][]MintQuoteChannel),
		MeltQuote: make(map[string][]MeltQuoteChannel),
		Mutex:     sync.Mutex{},
	}
}

func TestRemoveWatchKeepOtherSubscription(t *testing.T) {
	observer := newObserverForTest()

	proofChan1 := make(chan cashu.Proof)
	proofChan2 := make(chan cashu.Proof)

	observer.AddProofWatch("test", ProofWatchChannel{SubId: "1", Channel: proofChan1})
	observer.AddProofWatch("test", ProofWatchChannel{SubId: "2", Channel: proofChan2})

	observer.RemoveWatch("1")

	proofs := observer.Proofs["test"]
	if len(proofs) != 1 {
		t.Fatalf("expected 1 watcher left, got %d", len(proofs))
	}
	if proofs[0].SubId != "2" {
		t.Fatalf("expected remaining sub id to be 2, got %s", proofs[0].SubId)
	}

	select {
	case _, ok := <-proofChan1:
		if ok {
			t.Fatal("expected removed channel to be closed")
		}
	default:
		t.Fatal("expected removed channel to be closed and readable")
	}
}

func TestRemoveWatchDoesNotCloseSameProofChannelTwice(t *testing.T) {
	observer := Observer{
		Proofs:    make(map[string][]ProofWatchChannel),
		MintQuote: make(map[string][]MintQuoteChannel),
		MeltQuote: make(map[string][]MeltQuoteChannel),
		Mutex:     sync.Mutex{},
	}

	sharedProofChan := make(chan cashu.Proof)
	sharedProofChan2 := make(chan cashu.Proof)
	otherProofChan := make(chan cashu.Proof)

	observer.AddProofWatch("filter-1", ProofWatchChannel{SubId: "same-sub", Channel: sharedProofChan})
	observer.AddProofWatch("filter-2", ProofWatchChannel{SubId: "same-sub", Channel: sharedProofChan2})
	observer.AddProofWatch("filter-1", ProofWatchChannel{SubId: "other-sub", Channel: otherProofChan})

	observer.RemoveWatch("same-sub")

	remaining := observer.Proofs["filter-1"]
	if len(remaining) != 1 || remaining[0].SubId != "other-sub" {
		t.Fatalf("unexpected remaining watchers: %+v", remaining)
	}

	select {
	case _, ok := <-sharedProofChan:
		if ok {
			t.Fatal("expected shared proof channel to be closed")
		}
	default:
		t.Fatal("expected shared proof channel to be closed and readable")
	}
}

func TestRemoveWatchClosesMintAndMeltChannels(t *testing.T) {
	observer := newObserverForTest()

	mintChan := make(chan cashu.MintRequestDB)
	meltChan := make(chan cashu.MeltRequestDB)

	observer.AddMintWatch("mint-filter", MintQuoteChannel{SubId: "sub-1", Channel: mintChan})
	observer.AddMeltWatch("melt-filter", MeltQuoteChannel{SubId: "sub-1", Channel: meltChan})

	observer.RemoveWatch("sub-1")

	select {
	case _, ok := <-mintChan:
		if ok {
			t.Fatal("expected mint channel to be closed")
		}
	default:
		t.Fatal("expected mint channel to be closed and readable")
	}

	select {
	case _, ok := <-meltChan:
		if ok {
			t.Fatal("expected melt channel to be closed")
		}
	default:
		t.Fatal("expected melt channel to be closed and readable")
	}
}

func TestSendEventsDoNotBlockOnFullChannels(t *testing.T) {
	observer := newObserverForTest()
	_, publicKey := btcec.PrivKeyFromBytes([]byte{1})
	proof := cashu.Proof{Y: cashu.WrappedPublicKey{PublicKey: publicKey}}
	mint := cashu.MintRequestDB{Quote: "mint"}
	melt := cashu.MeltRequestDB{Quote: "melt"}
	proofChan := make(chan cashu.Proof, 1)
	mintChan := make(chan cashu.MintRequestDB, 1)
	meltChan := make(chan cashu.MeltRequestDB, 1)

	proofChan <- proof
	mintChan <- mint
	meltChan <- melt
	observer.AddProofWatch(proof.Y.ToHex(), ProofWatchChannel{Channel: proofChan})
	observer.AddMintWatch(mint.Quote, MintQuoteChannel{Channel: mintChan})
	observer.AddMeltWatch(melt.Quote, MeltQuoteChannel{Channel: meltChan})

	done := make(chan struct{})
	go func() {
		observer.SendProofsEvent(cashu.Proofs{proof})
		observer.SendMintEvent(mint)
		observer.SendMeltEvent(melt)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("event delivery blocked on a full subscriber channel")
	}
}
