package review

import (
	"fmt"

	"github.com/luuuc/council/internal/expert"
)

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

// SeatCouncils prepares the councils for one room. A member who sits in
// several of them speaks only in the smallest (the most specific group,
// such as "customers" next to "product"); ties go to the first listed.
// Councils left empty are dropped. notes say what changed, and warn about
// councils of one, which have nobody to debate with.
func SeatCouncils(councils []Council) ([]Council, []string) {
	seat := map[string]int{} // member id -> index of the council they speak in
	for i, c := range councils {
		for _, in := range c.Inputs {
			j, ok := seat[in.Expert.ID]
			if !ok || len(c.Inputs) < len(councils[j].Inputs) {
				seat[in.Expert.ID] = i
			}
		}
	}

	var out []Council
	var notes []string
	moved := map[string]bool{}
	for i, c := range councils {
		kept := Council{Name: c.Name}
		for _, in := range c.Inputs {
			if j := seat[in.Expert.ID]; j != i {
				if !moved[in.Expert.ID] {
					notes = append(notes, fmt.Sprintf("%s sits in several of these councils and speaks only in the %s council", in.Expert.Name, councils[j].Name))
					moved[in.Expert.ID] = true
				}
				continue
			}
			kept.Inputs = append(kept.Inputs, in)
		}
		if len(kept.Inputs) == 0 {
			notes = append(notes, fmt.Sprintf("the %s council has no one left to speak, so it's left out", c.Name))
			continue
		}
		out = append(out, kept)
	}
	return out, append(notes, singleMemberNotes(out)...)
}

// singleMemberNotes warns about councils with one member: no debate.
func singleMemberNotes(councils []Council) []string {
	var notes []string
	for _, c := range councils {
		if len(c.Inputs) != 1 {
			continue
		}
		name := "the council"
		if c.Name != "" {
			name = "the " + c.Name + " council"
		}
		notes = append(notes, fmt.Sprintf("%s has one member (%s), so nobody debates inside it; add members with 'council packs add'", name, c.Inputs[0].Expert.Name))
	}
	return notes
}
