package lightning

import "testing"

func TestLnbitsPaymentState(t *testing.T) {
	tests := []struct {
		name   string
		status LNBitsPaymentStatus
		wanted PaymentStatus
	}{
		{
			name: "paid status settles payment",
			status: LNBitsPaymentStatus{
				Preimage: "",
				Details: LNBitsPaymentStatusDetail{
					Memo:    "",
					Status:  "",
					Fee:     0,
					Pending: false,
				},
				Paid:    true,
				Pending: false,
			},
			wanted: SETTLED,
		},
		{
			name: "top level pending keeps payment pending",
			status: LNBitsPaymentStatus{
				Preimage: "",
				Details: LNBitsPaymentStatusDetail{
					Memo:    "",
					Status:  "",
					Fee:     0,
					Pending: false,
				},
				Paid:    false,
				Pending: true,
			},
			wanted: PENDING,
		},
		{
			name: "detail status pending keeps payment pending",
			status: LNBitsPaymentStatus{
				Preimage: "",
				Details: LNBitsPaymentStatusDetail{
					Memo:    "",
					Status:  "pending",
					Fee:     0,
					Pending: false,
				},
				Paid:    false,
				Pending: false,
			},
			wanted: PENDING,
		},
		{
			name: "detail pending keeps payment pending",
			status: LNBitsPaymentStatus{
				Preimage: "",
				Details: LNBitsPaymentStatusDetail{
					Memo:    "",
					Status:  "",
					Fee:     0,
					Pending: true,
				},
				Paid:    false,
				Pending: false,
			},
			wanted: PENDING,
		},
		{
			name: "unpaid non pending fails payment",
			status: LNBitsPaymentStatus{
				Preimage: "",
				Details: LNBitsPaymentStatusDetail{
					Memo:    "",
					Status:  "",
					Fee:     0,
					Pending: false,
				},
				Paid:    false,
				Pending: false,
			},
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
