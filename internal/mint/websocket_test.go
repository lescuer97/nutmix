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

	observer.Lock()
	proofs := observer.Proofs["test"]
	observer.Unlock()
	if len(proofs) != 1 {
		t.Fatalf("expected 1 watcher left, got %d", len(proofs))
	}
	if proofs[0].SubId != "2" {
		t.Fatalf("expected remaining sub id to be 2, got %s", proofs[0].SubId)
	}

	assertChannelOpen(t, proofChan1)
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

	observer.Lock()
	remaining := observer.Proofs["filter-1"]
	observer.Unlock()
	if len(remaining) != 1 || remaining[0].SubId != "other-sub" {
		t.Fatalf("unexpected remaining watchers: %+v", remaining)
	}

	assertChannelOpen(t, sharedProofChan)
	assertChannelOpen(t, sharedProofChan2)
}

func TestRemoveWatchDetachesMintAndMeltChannelsWithoutClosing(t *testing.T) {
	observer := newObserverForTest()

	mintChan := make(chan cashu.MintRequestDB)
	meltChan := make(chan cashu.MeltRequestDB)

	observer.AddMintWatch("mint-filter", MintQuoteChannel{SubId: "sub-1", Channel: mintChan})
	observer.AddMeltWatch("melt-filter", MeltQuoteChannel{SubId: "sub-1", Channel: meltChan})

	observer.RemoveWatch("sub-1")

	assertChannelOpen(t, mintChan)
	assertChannelOpen(t, meltChan)
}

func assertChannelOpen[T any](t *testing.T, ch <-chan T) {
	t.Helper()
	select {
	case _, ok := <-ch:
		if !ok {
			t.Fatal("RemoveWatch closed a caller-owned channel")
		}
		t.Fatal("unexpected value on empty channel")
	default:
	}
}

func TestSendEventsDoNotBlockOnFullChannels(t *testing.T) {
	observer := newObserverForTest()
	_, publicKey := btcec.PrivKeyFromBytes([]byte{1})
	proof := cashu.Proof{Y: cashu.WrappedPublicKey{PublicKey: publicKey}} //nolint:exhaustruct // Only the event key matters.
	mint := cashu.MintRequestDB{Quote: "mint"}                            //nolint:exhaustruct // Only the event key matters.
	melt := cashu.MeltRequestDB{Quote: "melt"}                            //nolint:exhaustruct // Only the event key matters.
	proofChan := make(chan cashu.Proof, 1)
	mintChan := make(chan cashu.MintRequestDB, 1)
	meltChan := make(chan cashu.MeltRequestDB, 1)

	proofChan <- proof
	mintChan <- mint
	meltChan <- melt
	observer.AddProofWatch(proof.Y.ToHex(), ProofWatchChannel{Channel: proofChan}) //nolint:exhaustruct // Subscription ID is irrelevant.
	observer.AddMintWatch(mint.Quote, MintQuoteChannel{Channel: mintChan})         //nolint:exhaustruct // Subscription ID is irrelevant.
	observer.AddMeltWatch(melt.Quote, MeltQuoteChannel{Channel: meltChan})         //nolint:exhaustruct // Subscription ID is irrelevant.

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
