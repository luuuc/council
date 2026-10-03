package review

import (
	"testing"

	"github.com/luuuc/council/internal/expert"
)

func TestExpertDomain(t *testing.T) {
	tests := []struct {
		focus string
		want  Domain
	}{
		{"Application security", DomainSecurity},
		{"Cryptography and encryption", DomainSecurity},
		{"Test-driven development", DomainQuality},
		{"Product simplicity and scope", DomainProduct}, // "product" matches Product (highest wins)
		{"Go performance optimization", DomainPerformance},
		{"Code style and consistency", DomainConvention},
		{"User experience design", DomainProduct},
		{"General programming", DomainQuality}, // default
	}

	for _, tt := range tests {
		t.Run(tt.focus, func(t *testing.T) {
			e := &expert.Expert{Focus: tt.focus}
			got := ExpertDomain(e)
			if got != tt.want {
				t.Errorf("ExpertDomain(%q) = %d, want %d", tt.focus, got, tt.want)
			}
		})
	}
}

func TestDomainOrdering(t *testing.T) {
	if DomainSecurity <= DomainProduct {
		t.Error("Security should be higher priority than Product")
	}
	if DomainProduct <= DomainScope {
		t.Error("Product should be higher priority than Scope")
	}
	if DomainScope <= DomainConvention {
		t.Error("Scope should be higher priority than Convention")
	}
	if DomainConvention <= DomainPerformance {
		t.Error("Convention should be higher priority than Performance")
	}
	if DomainPerformance <= DomainQuality {
		t.Error("Performance should be higher priority than Quality")
	}
}
