package review

import (
	"fmt"

	"github.com/luuuc/council/internal/expert"
)

// TurnKind says what a turn in a debate is for.
type TurnKind int

const (
	// TurnReview is a member's first review, reacting to the earlier ones.
	TurnReview TurnKind = iota
	// TurnFinalWord is an earlier member answering the points made after them.
	TurnFinalWord
	// TurnModerator is the neutral summary of disagreements and decisions.
	TurnModerator
)

// Turn is one step of a debate: who speaks and what they see.
type Turn struct {
	Kind    TurnKind
	Number  int // 1-based position among all turns
	Total   int
	Expert  *expert.Expert // the member, or Moderator
	Sub     Submission     // for member turns: what BuildPrompt renders
	Council string         // set when several councils debate

	rawPrompt string // prompt for council-level turns (spokesperson, cross moderator)
}

// Label names the turn for progress output, e.g. "Virtual Jane Doe (final word)".
// Turns inside one of several councils start with the council's name.
func (t Turn) Label() string {
	var label string
	switch t.Kind {
	case TurnFinalWord:
		label = t.Expert.Name + " (final word)"
	case TurnModerator:
		label = "Moderator (disagreements and decisions)"
	default:
		label = t.Expert.Name
	}
	if t.Council != "" && t.Kind != TurnSpokesperson {
		label = t.Council + " council: " + label
	}
	return label
}

// Prompt renders the turn's prompt.
func (t Turn) Prompt() string {
	if t.rawPrompt != "" {
		return t.rawPrompt
	}
	if t.Kind == TurnModerator {
		return BuildModeratorPrompt(t.Sub)
	}
	return BuildPrompt(t.Expert, t.Sub)
}

// Debate runs a sequential council review one turn at a time. The caller
// asks for the next turn, gets an answer from an AI (a backend call, or the
// MCP client's own model), and records it. The same steps drive the CLI and
// the MCP convene tools.
//
// Turns: each member reviews in order; then, with FinalWord, each earlier
// member answers the points made after them; then, with Moderate, a neutral
// moderator lists disagreements and decisions.
type Debate struct {
	inputs    []ExpertInput
	sub       Submission
	finalWord bool
	moderate  bool

	verdicts   map[int]*ExpertVerdict // by input index
	order      []int                  // input indexes of members who reviewed, in speaking order
	errors     []string
	moderation *Moderation

	plan []planned
	pos  int
}

type planned struct {
	kind  TurnKind
	input int
}

// NewDebate plans a debate for the given members.
func NewDebate(inputs []ExpertInput, sub Submission, finalWord, moderate bool) *Debate {
	d := &Debate{inputs: inputs, sub: sub, finalWord: finalWord, moderate: moderate, verdicts: map[int]*ExpertVerdict{}}
	for i := range inputs {
		d.plan = append(d.plan, planned{TurnReview, i})
	}
	if len(inputs) > 1 {
		if finalWord {
			for i := 0; i < len(inputs)-1; i++ {
				d.plan = append(d.plan, planned{TurnFinalWord, i})
			}
		}
		if moderate {
			d.plan = append(d.plan, planned{TurnModerator, -1})
		}
	}
	return d
}

// Next returns the next turn, or false when the debate is over. Final words
// are skipped for members whose review failed or who have nobody after them.
func (d *Debate) Next() (Turn, bool) {
	for d.pos < len(d.plan) {
		p := d.plan[d.pos]
		turn := Turn{Kind: p.kind, Number: d.pos + 1, Total: len(d.plan)}

		switch p.kind {
		case TurnReview:
			turn.Expert = d.inputs[p.input].Expert
			turn.Sub = d.sub
			turn.Sub.Prior = d.spoken()
			return turn, true

		case TurnFinalWord:
			own, ok := d.verdicts[p.input]
			after := d.spokenAfter(p.input)
			if !ok || len(after) == 0 {
				d.pos++
				continue
			}
			ownCopy := *own
			turn.Expert = d.inputs[p.input].Expert
			turn.Sub = d.sub
			turn.Sub.Own = &ownCopy
			turn.Sub.Prior = after
			return turn, true

		case TurnModerator:
			if len(d.order) < 2 {
				d.pos++
				continue
			}
			turn.Expert = Moderator
			turn.Sub = d.sub
			turn.Sub.Prior = d.spoken()
			return turn, true
		}
	}
	return Turn{}, false
}

// RecordVerdict records a member's answer to the current turn.
func (d *Debate) RecordVerdict(v ExpertVerdict) {
	p := d.plan[d.pos]
	d.pos++
	inp := d.inputs[p.input]

	if p.kind == TurnFinalWord {
		own := d.verdicts[p.input]
		own.FinalWord = v.Replies
		if ValidVerdicts[v.Verdict] && v.Verdict != own.Verdict && v.Error == "" {
			own.ChangedFrom = own.Verdict
			own.Verdict = v.Verdict
			own.ChangeReason = v.ChangeReason
		}
		return
	}

	v.Expert = inp.Expert.ID
	v.Name = inp.Expert.Name
	v.Blocking = inp.Blocking
	d.verdicts[p.input] = &v
	d.order = append(d.order, p.input)
}

// RecordModeration records the moderator's raw answer. It returns false if
// the answer couldn't be read; the turn is consumed either way.
func (d *Debate) RecordModeration(raw string) bool {
	d.pos++
	ids := map[string]bool{}
	for _, i := range d.order {
		ids[d.inputs[i].Expert.ID] = true
	}
	m, ok := ParseModeration(raw, ids)
	if ok {
		d.moderation = &m
	}
	return ok
}

// Fail records that the current turn failed and moves on.
func (d *Debate) Fail(err error) {
	p := d.plan[d.pos]
	d.pos++
	label := "moderator"
	if p.input >= 0 {
		label = d.inputs[p.input].Expert.ID
		if p.kind == TurnFinalWord {
			label += " (final word)"
		}
	}
	d.errors = append(d.errors, fmt.Sprintf("%s: %s", label, err))
}

// Verdicts returns the members' verdicts so far, in speaking order.
func (d *Debate) Verdicts() []ExpertVerdict {
	return d.spoken()
}

// Result synthesizes the debate.
func (d *Debate) Result() *SynthesizedResult {
	experts := make([]*expert.Expert, len(d.inputs))
	for i, inp := range d.inputs {
		experts[i] = inp.Expert
	}
	result := Synthesize(d.spoken(), experts, d.errors)
	if d.moderation != nil {
		result.Disagreements = d.moderation.Disagreements
		result.Decisions = d.moderation.Decisions
		if len(d.moderation.Agreements) > 0 {
			result.Agreements = d.moderation.Agreements
		}
	}
	return result
}

func (d *Debate) spoken() []ExpertVerdict {
	out := make([]ExpertVerdict, 0, len(d.order))
	for _, i := range d.order {
		out = append(out, *d.verdicts[i])
	}
	return out
}

// spokenAfter returns the verdicts of members who reviewed after input i.
func (d *Debate) spokenAfter(i int) []ExpertVerdict {
	var out []ExpertVerdict
	seen := false
	for _, j := range d.order {
		if seen {
			out = append(out, *d.verdicts[j])
		}
		if j == i {
			seen = true
		}
	}
	return out
}

// RecordRaw records a raw answer for a turn of kind k. In a single
// council only the moderator answers raw.
func (d *Debate) RecordRaw(k TurnKind, raw string) bool {
	if k != TurnModerator {
		return false
	}
	return d.RecordModeration(raw)
}
