# Install Council

Set up the council for your project.

## Quick Start

1. Check if council is already set up:
```bash
council list
```

2. If not set up, run:
```bash
council init
```

This creates `.council/` and installs the `/council` commands. The council starts empty: propose members for this project (people with documented public positions who will disagree, a role, a customer), let the user choose, then add them.

## Customization

After setup, you can modify your council:

- `council add "Jane Doe" --yes` - add a person, researched from their public work
- `council add --role "SRE" --yes` / `council add --customer "..." --yes` - add a role or a customer
- `/council-add` - interactive expert search
- `council remove <id>` - remove an expert
- `council sync` - sync changes to your AI tool
