# Install Council

Set up the council for your project.

## Quick Start

1. Initialize the council:
```bash
council init
```

2. Assemble the council with your AI: it follows the brief from `council assemble` (propose people, roles, and customers; the user chooses; build each persona) and saves each member:
```bash
council assemble
council add persona.md
```

3. Sync to generate AGENTS.md:
```bash
council sync
```

The AGENTS.md file will be created in your project root.
AI tools that support the AGENTS.md convention will use these expert personas.
