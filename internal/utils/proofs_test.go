package utils

import (
	"github.com/lescuer97/nutmix/api/cashu"
	"testing"
)

func setListofEmptyBlindMessages(amounts int) []cashu.BlindedMessage {
	var messages []cashu.BlindedMessage
	for i := 0; i < amounts; i++ {
		message := cashu.BlindedMessage{
			Id:     "mockid",
			Amount: 0,
		}
		messages = append(messages, message)

	}

	return messages
}
func TestGetChangeWithEnoughBlindMessages(t *testing.T) {

	emptyBlindMessages := setListofEmptyBlindMessages(10)

	// create change for value of 2
	change := GetChangeOutput(2, emptyBlindMessages)

	if len(change) != 1 {
		t.Errorf("Incorrect size for change slice %v, should be 1", len(change))

	}

	if change[0].Amount != 2 {
		t.Errorf("Incorrect amount for change slice %v, should be 2", change[0].Amount)
	}

	// create change for a 0 amount
	change = GetChangeOutput(0, emptyBlindMessages)

	if len(change) != 0 {
		t.Errorf("Incorrect size for change slice %v, should be 0", len(change))
	}

}

func TestGetChangeWithOutEnoughBlindMessages(t *testing.T) {

	emptyBlindMessages := setListofEmptyBlindMessages(1)

	// create change for value of 2
	change := GetChangeOutput(10, emptyBlindMessages)

	if len(change) != 1 {
		t.Errorf("Incorrect size for change slice %v, should be 1", len(change))
	}

	if change[0].Amount != 2 {
		t.Errorf("Incorrect amount for change slice %v, should be 2", change[0].Amount)
	}

}

func MakeListofMockProofs(amounts int) []cashu.Proof {

	var proofs []cashu.Proof
	for i := 0; i < amounts; i++ {
		proof := cashu.Proof{
			Id:     "mockid",
			Amount: 0,
		}
		proofs = append(proofs, proof)

	}

	return proofs
}

func TestGetValuesFromProofs(t *testing.T) {

	listOfProofs := cashu.Proofs{
		{
			Id:     "mockid",
			Amount: 2,
			Secret: "mockSecret",
		},
		{
			Id:     "mockid",
			Amount: 6,
			Secret: "mockSecret2",
		},
	}

	TotalAmount, secretsList, err := GetAndCalculateProofsValues(&listOfProofs)
	if err != nil {
		t.Fatal("GetAndCalculateProofsValues(&listOfProofs)")
	}

	if TotalAmount != 8 {
		t.Errorf("Incorrect total amount %v. Should be 8", TotalAmount)
	}

	if secretsList[0] != "02aa4a2c024e41bd87e8c2758d5a7c2d81e09afe52f67fc8a69768bd73d515e28f" {
		t.Errorf("Should be mock secret %v. Should be 8", TotalAmount)
	}
	if listOfProofs[0].Y != "02aa4a2c024e41bd87e8c2758d5a7c2d81e09afe52f67fc8a69768bd73d515e28f" {
		t.Errorf("Incorrect Y: %v. ", listOfProofs[0].Y)
	}

}
