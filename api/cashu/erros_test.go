package cashu

import "testing"

func TestCreatingAnErrorResponse(t *testing.T) {

	response := ErrorCodeToResponse(INSUFICIENT_FEE, nil)

	if response.Code != 11006 {
		t.Errorf("Did not get the correct error node.")
	}

	if response.Error != "Insufficient fee" {
		t.Errorf("Incorrect error string")
	}

}
