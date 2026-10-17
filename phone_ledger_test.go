package main

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		name    string
		in      event
		want    decision
		invalid bool
	}{
		{"receipt and campaign aggregate", event{Kind: "donation", DonorID: "donor-4", CampaignID: "winter", AmountCents: 2500}, decision{Receipt: true, CampaignID: "winter", ReportedCents: 2500}, false},
		{"shift reminder", event{Kind: "volunteer_shift", DonorID: "volunteer-2"}, decision{Reminder: true}, false},
		{"no receipt for zero donation", event{Kind: "donation", DonorID: "donor-4", CampaignID: "winter"}, decision{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := classify(tt.in)
			if (err != nil) != tt.invalid || got != tt.want {
				t.Fatalf("classify(%+v) = %+v, %v", tt.in, got, err)
			}
		})
	}
}
