package expert

import (
	"fmt"
	"os"
)

// Deprecated: Remove in v2.0.
var legacyAliases = map[string]string{
	// Go
	"rob-pike":    "sable-okoro",
	"dave-cheney": "sable-okoro",
	"antirez":     "sable-okoro",

	// Ruby
	"matz":       "tara-mishkin",
	"sandi-metz": "tara-mishkin",
	"avdi-grimm": "tara-mishkin",

	// Rails
	"dhh":             "diego-valdez",
	"jorge-manrubia":  "diego-valdez",
	"rafael-franca":   "diego-valdez",
	"eileen-uchitelle": "priya-anand",
	"tenderlove":       "priya-anand",

	// Python
	"raymond-hettinger": "nolan-chambers",
	"simon-willison":    "owen-mbeki",
	"carlton-gibson":    "owen-mbeki",

	// Swift
	"chris-lattner":    "linnea-strand",
	"john-sundell":     "linnea-strand",
	"soroush-khanlou": "linnea-strand",

	// Kotlin/Android
	"jake-wharton":    "dmitri-vanek",
	"roman-elizarov":  "dmitri-vanek",
	"gabor-varadi":    "dmitri-vanek",

	// Frontend
	"jeremy-keith":     "rowan-tate",
	"luke-wroblewski":  "rowan-tate",

	// JavaScript/React
	"dan-abramov": "kira-petrov",

	// Elixir
	"jose-valim":  "tomas-carvallo",
	"sasa-juric":  "tomas-carvallo",

	// Phoenix
	"jose-valim-phoenix":   "mira-santos",
	"sophie-debenedetto":   "mira-santos",
	"chris-mccord":         "mira-santos",

	// TypeScript
	"anders-hejlsberg": "vera-lindqvist",
	"matt-pocock":      "vera-lindqvist",

	// Java
	"joshua-bloch":  "haruto-ikeda",
	"martin-fowler": "haruto-ikeda",

	// C#
	"anders-hejlsberg-csharp": "fiona-cross",
	"nick-chapsas":            "fiona-cross",

	// PHP
	"taylor-otwell": "remy-dubois",
	"nuno-maduro":   "remy-dubois",

	// Vue
	"evan-you":    "mei-chen",
	"anthony-fu":  "mei-chen",

	// Node
	"ryan-dahl":       "jonas-ekberg",
	"matteo-collina":  "jonas-ekberg",

	// Rust
	"steve-klabnik": "cleo-ashford",

	// Clojure
	"rich-hickey":  "theo-marchetti",
	"alex-miller":  "theo-marchetti",

	// Next.js
	"guillermo-rauch": "zara-okonkwo",
	"lee-robinson":    "zara-okonkwo",

	// Django
	// simon-willison already mapped above to owen-mbeki

	// Svelte
	"rich-harris": "astrid-holm",
	"tan-li-hau":  "astrid-holm",

	// Flutter
	"eric-seidel":     "ravi-dasgupta",
	"remi-rousselet":  "ravi-dasgupta",

	// C++
	"bjarne-stroustrup": "petra-koenig",
	"john-carmack":      "petra-koenig",

	// Scala
	"martin-odersky": "lucian-voss",
	"john-de-goes":   "lucian-voss",

	// GraphQL
	"lee-byron":          "soren-blume",
	"marc-andre-giroux":  "soren-blume",

	// Writing
	"william-zinsser": "colette-marsh",
	"stephen-king":    "colette-marsh",

	// Business
	"paul-graham": "ines-guerrero",

	// Product
	"marty-cagan":    "lena-dahl",
	"teresa-torres":  "lena-dahl",
	"des-traynor":    "lena-dahl",

	// Design/UX
	"don-norman":    "yuki-tanaka",
	"jakob-nielsen": "yuki-tanaka",
	"julie-zhuo":    "yuki-tanaka",

	// Growth
	"brian-balfour":  "felix-okafor",
	"april-dunford": "felix-okafor",
	"sean-ellis":    "felix-okafor",

	// Sales
	"mark-roberge": "hana-reeves",
	"aaron-ross":   "hana-reeves",

	// Finance
	"david-skok": "vincent-arnaud",
	"brad-feld":  "vincent-arnaud",

	// Leadership
	"andy-grove":           "wren-callister",
	"camille-fournier":     "wren-callister",
	"will-larson":          "wren-callister",

	// Operations
	"elad-gil":              "sol-achebe",
	"ben-horowitz":          "sol-achebe",
	"claire-hughes-johnson": "sol-achebe",

	// Hiring
	"geoff-smart": "maren-engel",
	"laszlo-bock": "maren-engel",

	// Data
	"avinash-kaushik": "juno-patel",
	"dj-patil":        "juno-patel",

	// Security
	"bruce-schneier": "nadia-kowalski",

	// Legal
	"heather-meeker": "ava-whitfield",

	// Customer Success
	"lincoln-murphy": "noah-altman",
	"nick-mehta":     "noah-altman",

	// Bootstrapped
	"rob-walling":   "renzo-cardenas",
	"sahil-lavingia": "renzo-cardenas",
	"arvid-kahl":     "renzo-cardenas",

	// VC
	"reid-hoffman":    "dahlia-amara",
	"keith-rabois":    "dahlia-amara",
	"marc-andreessen": "dahlia-amara",

	// General
	"kent-beck":    "ada-redgrave",
	"jason-fried":  "marcus-torrent",
	"dieter-rams":  "elara-nygaard",
	"gene-kim":     "niall-cassidy",
	"cal-newport":  "iris-vance",
	"taiichi-ohno": "niall-cassidy",
}

// LegacyAlias resolves a deprecated real-name ID to its composite replacement.
// Deprecated: Remove in v2.0.
func LegacyAlias(id string) (string, bool) {
	if newID, ok := legacyAliases[id]; ok {
		fmt.Fprintf(os.Stderr, "Warning: %q is deprecated, use %q instead\n", id, newID)
		return newID, true
	}
	return id, false
}
