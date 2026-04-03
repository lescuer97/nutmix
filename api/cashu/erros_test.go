package cashu

import "testing"

func TestCreatingAnErrorResponse(t *testing.T) {
	response := ErrorCodeToResponse(INSUFICIENT_OUTSIDE_LIMIT, nil)

	if response.Code != 11006 {
		t.Errorf("Did not get the correct error node.")
	}

	if response.Error != "Amount outside limit" {
		t.Errorf("Incorrect error string")
	}
}
