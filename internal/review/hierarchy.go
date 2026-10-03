package review

import (
	"strings"

	"github.com/luuuc/council/internal/expert"
)

// Domain represents a review domain in the decision hierarchy.
// When experts disagree, higher-priority domains take precedence.
// Security > Product > Scope > Convention > Performance > Quality
type Domain int

const (
	DomainQuality Domain = iota // Lowest priority
	DomainPerformance
	DomainConvention
	DomainScope
	DomainProduct
	DomainSecurity // Highest priority
)

// domainKeywords maps keywords found in expert Focus fields to domains.
var domainKeywords = map[string]Domain{
	// Security
	"security":       DomainSecurity,
	"cryptography":   DomainSecurity,
	"authentication": DomainSecurity,
	"authorization":  DomainSecurity,
	"vulnerability":  DomainSecurity,
	"threat":         DomainSecurity,
	"encryption":     DomainSecurity,
	"secure":         DomainSecurity,
	"infosec":        DomainSecurity,
	"appsec":         DomainSecurity,
	"penetration":    DomainSecurity,
	"safety":         DomainSecurity,

	// Product
	"product":    DomainProduct,
	"ux":         DomainProduct,
	"user":       DomainProduct,
	"design":     DomainProduct,
	"usability":  DomainProduct,
	"experience": DomainProduct,
	"interface":  DomainProduct,
	"accessible": DomainProduct,

	// Scope
	"scope":        DomainScope,
	"simplicity":   DomainScope,
	"minimalism":   DomainScope,
	"yagni":        DomainScope,
	"lean":         DomainScope,
	"pragmatism":   DomainScope,
	"less is more": DomainScope,

	// Convention
	"convention":  DomainConvention,
	"style":       DomainConvention,
	"consistency": DomainConvention,
	"idiomatic":   DomainConvention,
	"standard":    DomainConvention,
	"readability": DomainConvention,
	"clarity":     DomainConvention,

	// Performance
	"performance":  DomainPerformance,
	"optimization": DomainPerformance,
	"scalability":  DomainPerformance,
	"efficiency":   DomainPerformance,
	"latency":      DomainPerformance,
	"throughput":   DomainPerformance,
	"memory":       DomainPerformance,
	"caching":      DomainPerformance,

	// Quality
	"testing":       DomainQuality,
	"test-driven":   DomainQuality,
	"tdd":           DomainQuality,
	"quality":       DomainQuality,
	"refactoring":   DomainQuality,
	"clean code":    DomainQuality,
	"maintainab":    DomainQuality,
	"reliability":   DomainQuality,
	"observability": DomainQuality,
}

// ExpertDomain determines an expert's primary domain from their Focus field.
// Returns DomainQuality as the default if no keywords match.
func ExpertDomain(e *expert.Expert) Domain {
	focus := strings.ToLower(e.Focus)
	best := DomainQuality
	for keyword, domain := range domainKeywords {
		if strings.Contains(focus, keyword) && domain > best {
			best = domain
		}
	}
	return best
}
