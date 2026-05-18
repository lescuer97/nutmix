package lightning

import "testing"

func TestLnbitsPaymentState(t *testing.T) {
	tests := []struct {
		name   string
		status LNBitsPaymentStatus
		wanted PaymentStatus
	}{
		{
			name:   "paid status settles payment",
			status: LNBitsPaymentStatus{Paid: true},
			wanted: SETTLED,
		},
		{
			name:   "top level pending keeps payment pending",
			status: LNBitsPaymentStatus{Pending: true},
			wanted: PENDING,
		},
		{
			name:   "detail status pending keeps payment pending",
			status: LNBitsPaymentStatus{Details: LNBitsPaymentStatusDetail{Status: "pending"}},
			wanted: PENDING,
		},
		{
			name:   "detail pending keeps payment pending",
			status: LNBitsPaymentStatus{Details: LNBitsPaymentStatusDetail{Pending: true}},
			wanted: PENDING,
		},
		{
			name:   "unpaid non pending fails payment",
			status: LNBitsPaymentStatus{},
			wanted: FAILED,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := lnbitsPaymentState(test.status)
			if got != test.wanted {
				t.Fatalf("lnbitsPaymentState() = %v, want %v", got, test.wanted)
			}
		})
	}
}
