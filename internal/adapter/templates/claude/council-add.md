# Add a Council Member

Add to the council: $ARGUMENTS

This can be a person's name, a role ("SRE"), a description of users (a customer), or a need ("someone for API design").

1. Run `council assemble` and read the persona format and rules (steps 4 and 5).
2. If $ARGUMENTS names someone or a role, build that persona. If it describes a need, propose 3 to 4 options with a reason each and let the user choose with **AskUserQuestion**.
3. Save with `council add -` as the brief shows. If Council rejects it, fix what it says and retry. Confirm with `council list`.
