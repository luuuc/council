# Direction

Council gives you a room full of Virtual experts, colleagues, critics, and customers who review your work, see each other's arguments, disagree, and force better decisions.

Code review is one use of this. Product, writing, architecture, security, and growth decisions are others.

## What a Council is

A Council is a team of AI reviewers modeled on real people, real roles, and real perspectives.

Personas based on real people carry the **Virtual** prefix: Virtual DHH, Virtual Boris Cherny, Virtual Jason Fried. The prefix makes it clear the persona is a model built from public material, not the person.

A good Council mixes people with different incentives. An engineering Council might hold Virtual DHH, Virtual Boris Cherny, a security engineer, an SRE, and a product-minded CTO. Five variations of the same expert is not a Council.

## How it works

**Real personas over invented ones.** Personas are built from public talks, writing, documented principles, decisions, and opinions. A real person's track record gives the review sharper, more specific positions than a composite can.

**Friction by design.** Personas should not agree by default. Their positions differ, and those differences are where the useful feedback comes from.

**Sequential, not parallel.** Each persona reviews after the previous ones and sees what they said. They react, challenge assumptions, add missing concerns, or disagree. The debate is part of the review, not something summarized away.

**Councils can debate Councils.** A global Council can sit beside specialized ones for engineering, product, security, growth, or architecture. Their conclusions can be put against each other.

**Users belong in the Council.** Product reviews can include personas for real customer types. They judge the work from the side of the people expected to use it.

**The human decides.** Council does not aim for AI consensus. Its job is to surface disagreements, blind spots, trade-offs, and arguments so the builder makes a better call.

## What changes from today

The current CLI moved away from these ideas. These are the changes needed to get back:

- **Real-person personas, with the Virtual prefix.** Done: the library is back to 97 real-person personas named "Virtual X". Roles (security engineer, SRE) can still be added as custom personas.
- **Make review sequential.** Today's collective review asks one LLM call to play everyone at once. Each persona should get its own turn, with the prior reviews in its prompt.
- **Keep the debate in the output.** Show who said what, who disagreed with whom, and where it stayed unresolved, instead of collapsing it into a single recommendation.
- **Add customer personas.** Let a Council include user types alongside experts.
- **Support Councils of Councils.** Run several Councils and let their conclusions challenge each other.
- **Make the CLI interactive.** Building a Council, picking personas, and following a review should feel guided, not like a list of flags.
