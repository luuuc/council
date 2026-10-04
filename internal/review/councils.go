package review

import "github.com/luuuc/council/internal/expert"

// Council is one named group of members, such as the "product" pack. A
// review without a pack has one council with no name: every member.
type Council struct {
	Name   string
	Inputs []ExpertInput
}

// ExpertInput pairs a member with their blocking status from the pack.
type ExpertInput struct {
	Expert   *expert.Expert
	Blocking bool
}

// Experts returns the council's members.
func (c Council) Experts() []*expert.Expert {
	out := make([]*expert.Expert, len(c.Inputs))
	for i, in := range c.Inputs {
		out[i] = in.Expert
	}
	return out
}

// CouncilStatement is a council's spokesperson answering the other councils.
type CouncilStatement struct {
	Council    string  `json:"council"`
	Position   string  `json:"position"`
	Challenges []Reply `json:"challenges,omitempty"` // To is another council's name
}

// CouncilOutcome is one council's own review.
type CouncilOutcome struct {
	Name   string             `json:"name"`
	Result *SynthesizedResult `json:"result"`
}

// CouncilsResult is a recorded review. With several councils, they also
// challenge each other's conclusions: Disagreements are then between
// councils (Side.Experts holds council names); Decisions are for the human.
type CouncilsResult struct {
	Councils      []CouncilOutcome   `json:"councils"`
	Statements    []CouncilStatement `json:"statements,omitempty"`
	Agreements    []string           `json:"agreements,omitempty"`
	Disagreements []Disagreement     `json:"disagreements,omitempty"`
	Decisions     []string           `json:"decisions,omitempty"`
	Errors        []string           `json:"errors,omitempty"`
}
