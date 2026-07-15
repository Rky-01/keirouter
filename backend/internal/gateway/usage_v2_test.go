package gateway

import (
	"testing"

	"github.com/mydisha/keirouter/backend/internal/store"
)

func TestAggregateModelPricingStatusPreservesLegacyTotals(t *testing.T) {
	tests := []struct {
		name  string
		model store.AccurateModelUsage
		want  string
	}{
		{
			name: "all legacy",
			model: store.AccurateModelUsage{
				PricingEligibleRequests: 2, UnpricedRequests: 2, LegacyPricingRequests: 2,
				PricingStatus: "legacy",
			},
			want: "legacy",
		},
		{
			name: "all missing",
			model: store.AccurateModelUsage{
				PricingEligibleRequests: 2, UnpricedRequests: 2, MissingPricingRequests: 2,
			},
			want: "missing",
		},
		{
			name: "legacy and missing",
			model: store.AccurateModelUsage{
				PricingEligibleRequests: 2, UnpricedRequests: 2,
				LegacyPricingRequests: 1, MissingPricingRequests: 1,
			},
			want: "partial",
		},
		{
			name:  "no eligible usage",
			model: store.AccurateModelUsage{PricingStatus: "missing"},
			want:  "none",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := aggregateModelPricingStatus(tt.model); got != tt.want {
				t.Fatalf("aggregateModelPricingStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOptionalRatioEmptySetIsUnavailable(t *testing.T) {
	if got := optionalRatio(0, 0); got != nil {
		t.Fatalf("optionalRatio(0, 0) = %v, want nil", *got)
	}
	got := optionalRatio(3, 4)
	if got == nil || *got != 0.75 {
		t.Fatalf("optionalRatio(3, 4) = %v, want 0.75", got)
	}
}
