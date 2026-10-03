package expert

import (
	"fmt"
	"os"
)

// legacyAliases maps the retired role-based composite IDs to the real-person
// persona they were mostly modeled on.
// Deprecated: Remove in v2.0.
var legacyAliases = map[string]string{
	"the-go-purist":            "rob-pike",
	"the-ruby-crafter":         "matz",
	"the-rails-monolith":       "dhh",
	"the-activerecord-surgeon": "eileen-uchitelle",
	"the-pythonista":           "raymond-hettinger",
	"the-protocol-thinker":     "chris-lattner",
	"the-kotlin-architect":     "jake-wharton",
	"the-html-fundamentalist":  "jeremy-keith",
	"the-react-philosopher":    "dan-abramov",
	"the-otp-alchemist":        "jose-valim",
	"the-liveview-builder":     "chris-mccord",
	"the-type-guardian":        "anders-hejlsberg",
	"the-api-classicist":       "joshua-bloch",
	"the-net-pragmatist":       "anders-hejlsberg-csharp",
	"the-laravel-artisan":      "taylor-otwell",
	"the-vue-reactivity-nerd":  "evan-you",
	"the-event-loop-guy":       "ryan-dahl",
	"the-borrow-checker":       "steve-klabnik",
	"the-data-thinker":         "rich-hickey",
	"the-edge-deployer":        "guillermo-rauch",
	"the-django-pragmatist":    "simon-willison",
	"the-compiler-whisperer":   "rich-harris",
	"the-widget-composer":      "eric-seidel",
	"the-zero-cost-abstracter": "bjarne-stroustrup",
	"the-type-theorist":        "martin-odersky",
	"the-schema-purist":        "lee-byron",
	"the-revision-hawk":        "william-zinsser",
	"the-startup-realist":      "paul-graham",
	"the-product-skeptic":      "marty-cagan",
	"the-usability-scientist":  "don-norman",
	"the-growth-mechanic":      "brian-balfour",
	"the-revenue-engineer":     "mark-roberge",
	"the-metrics-hawk":         "david-skok",
	"the-eng-manager":          "andy-grove",
	"the-scale-operator":       "elad-gil",
	"the-hiring-bar-raiser":    "geoff-smart",
	"the-data-storyteller":     "avinash-kaushik",
	"the-threat-modeler":       "bruce-schneier",
	"the-license-auditor":      "heather-meeker",
	"the-retention-strategist": "lincoln-murphy",
	"the-bootstrap-realist":    "rob-walling",
	"the-venture-strategist":   "reid-hoffman",
	"the-tdd-advocate":         "kent-beck",
	"the-scope-cutter":         "jason-fried",
	"the-design-minimalist":    "dieter-rams",
	"the-flow-optimizer":       "gene-kim",
	"the-deep-worker":          "cal-newport",
}

// LegacyAlias resolves a deprecated composite ID to its real-person persona.
// Deprecated: Remove in v2.0.
func LegacyAlias(id string) (string, bool) {
	if newID, ok := legacyAliases[id]; ok {
		fmt.Fprintf(os.Stderr, "Warning: %q is deprecated, use %q instead\n", id, newID)
		return newID, true
	}
	return id, false
}
