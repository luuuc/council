# Install Council

Your AI tool will read the appropriate instructions for setting up the council.

## Quick Start

1. Check if council is already set up:
```bash
council list
```

2. If not set up, run:
```bash
council init
```

This creates `.council/` and installs the `/council` commands. The council starts with the author's persona as an example member. Propose members for this project (people with documented public positions who will disagree, a role, a customer), let the user choose, then add them.

## Customization

After setup, you can modify your council:

- `council assemble` - the brief for proposing and building members; save each with `council add -`
- `/council-add <who>` - add one member: a person, a role, or a customer
- `/council-add` - interactive expert search
- `council remove <id>` - remove an expert
- `council sync` - sync changes to your AI tool
