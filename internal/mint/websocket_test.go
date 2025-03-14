package mint

import (
	"testing"

	"github.com/lescuer97/nutmix/api/cashu"
)

func TestDeleteSubIdKeepOther(t *testing.T) {
	observer := Observer{}
	observer.Proofs = make(map[string][]ProofWatchChannel)

	proofChan1 := make(chan cashu.Proof)
	proofChan2 := make(chan cashu.Proof)
	sub1 := ProofWatchChannel{
		SubId:   "1",
		Channel: proofChan1,
	}
	sub2 := ProofWatchChannel{
		SubId:   "2",
		Channel: proofChan2,
	}
	observer.AddProofWatch("test", sub1)
	observer.AddProofWatch("test", sub2)
	proofs := observer.Proofs["test"]
	if proofs[0].SubId != "1" {
		t.Errorf("\n Sub id is incorrect. %v", proofs[0])
	}
	if proofs[1].SubId != "2" {
		t.Errorf("\n Sub id is incorrect. %v", proofs[1])
	}
	observer.RemoveWatch("1")

	if proofs[0].SubId != "2" {
		t.Errorf("\n didn't remove proof correctly. %v", proofs[0])
	}

}
