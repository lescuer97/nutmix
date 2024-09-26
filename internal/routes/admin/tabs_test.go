package admin

import "testing"


func TestCheckIntegerFromStringSuccess(t *testing.T) {
    text := "2"
    int, err := checkLimitSat(text)

    if err != nil {
        t.Error("Check limit should have work")
    }

    success := 2
    if int != &success {
        t.Error("Convertion should have occured")
    }
}

func TestCheckIntegerFromStringFailureBool(t *testing.T) {
    text := "2.2"
    _, err := checkLimitSat(text)

    if err == nil {
        t.Error("Check limit should have failed. Because it should not allow float")
    }

}
