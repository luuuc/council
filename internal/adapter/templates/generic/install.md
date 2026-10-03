# Install Council

Set up the council for your project.

## Quick Start

1. Initialize the council:
```bash
council init
```

2. Add members to your council: people you choose (researched from their public work), roles, and customers:
```bash
council add "Jane Doe"
council add --role "SRE"
council add --customer "who your users are"
```

3. Sync to generate AGENTS.md:
```bash
council sync
```

The AGENTS.md file will be created in your project root.
AI tools that support the AGENTS.md convention will use these expert personas.
