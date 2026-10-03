package review

import (
	"fmt"

	"github.com/luuuc/council/internal/expert"
)

// More kinds of turn, used when several councils debate.
const (
	// TurnSpokesperson is a council answering the other councils.
	TurnSpokesperson TurnKind = iota + 100
	// TurnCrossModerator lists where the councils disagree.
	TurnCrossModerator
)

// CouncilsDebate runs several councils one turn at a time: every turn of
// each council's debate, then each council's spokesperson, then the
// moderator across councils. It is the step-by-step form of
// Runner.RunCouncils, for callers whose model takes each turn itself
// (the MCP convene tools).
type CouncilsDebate struct {
	councils  []Council
	sub       Submission
	debates   []*Debate
	current   int // council whose debate is running; len(councils) once all are done
	result    CouncilsResult
	names     map[string]bool
	spoken    int // spokespersons done
	moderated bool
	total     int
	number    int
}

// NewCouncilsDebate plans a debate between councils.
func NewCouncilsDebate(councils []Council, sub Submission, finalWord, moderate bool) *CouncilsDebate {
	d := &CouncilsDebate{councils: councils, sub: sub, names: map[string]bool{}}
	for _, c := range councils {
		deb := NewDebate(c.Inputs, sub, finalWord, moderate)
		d.debates = append(d.debates, deb)
		d.names[c.Name] = true
		d.total += len(deb.plan)
	}
	d.total += len(councils) + 1
	return d
}

// Next returns the next turn, or false when the debate is over. A turn of
// a council's own debate carries that council's name in Council.
func (d *CouncilsDebate) Next() (Turn, bool) {
	for d.current < len(d.councils) {
		if t, ok := d.debates[d.current].Next(); ok {
			d.number++
			t.Council = d.councils[d.current].Name
			t.Number, t.Total = d.number, d.total
			return t, true
		}
		c := d.councils[d.current]
		d.result.Councils = append(d.result.Councils, CouncilOutcome{Name: c.Name, Result: d.debates[d.current].Result()})
		d.current++
	}
	if len(d.result.Councils) < 2 {
		return Turn{}, false
	}

	if d.spoken < len(d.result.Councils) {
		name := d.result.Councils[d.spoken].Name
		d.number++
		return Turn{
			Kind: TurnSpokesperson, Council: name, Number: d.number, Total: d.total,
			Expert:    &expert.Expert{ID: name + "-council", Name: "Spokesperson for the " + name + " council"},
			rawPrompt: buildSpokespersonPrompt(name, d.result.Councils, d.sub),
		}, true
	}

	if !d.moderated {
		d.number++
		return Turn{
			Kind: TurnCrossModerator, Number: d.number, Total: d.total,
			Expert:    &expert.Expert{ID: Moderator.ID, Name: "Moderator across councils"},
			rawPrompt: buildCrossModeratorPrompt(&d.result, d.sub),
		}, true
	}
	return Turn{}, false
}

// RecordVerdict records a member's answer during a council's own debate.
func (d *CouncilsDebate) RecordVerdict(v ExpertVerdict) {
	d.debates[d.current].RecordVerdict(v)
}

// RecordRaw records a raw answer for the current turn of kind k: a council's
// moderator, a spokesperson, or the moderator across councils. It returns
// false if the answer couldn't be read; the turn is consumed either way.
func (d *CouncilsDebate) RecordRaw(k TurnKind, raw string) bool {
	switch k {
	case TurnModerator:
		return d.debates[d.current].RecordModeration(raw)
	case TurnSpokesperson:
		name := d.result.Councils[d.spoken].Name
		d.spoken++
		stmt, ok := parseStatement(raw, name, d.names)
		if !ok {
			d.result.Errors = append(d.result.Errors, fmt.Sprintf("%s spokesperson: response could not be read", name))
			return false
		}
		d.result.Statements = append(d.result.Statements, stmt)
		return true
	case TurnCrossModerator:
		d.moderated = true
		m, ok := ParseModeration(raw, d.names)
		if ok {
			d.result.Agreements, d.result.Disagreements, d.result.Decisions = m.Agreements, m.Disagreements, m.Decisions
		}
		return ok
	}
	return false
}

// Result returns the councils' result so far.
func (d *CouncilsDebate) Result() *CouncilsResult {
	r := d.result
	return &r
}
