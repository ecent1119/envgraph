# envgraph

**Free and open source** — Environment Variable Dependency Graph for visualizing and analyzing environment variable flow across your project.

## The Problem

Environment variables are scattered across:
- Multiple `.env` files (.env, .env.local, .env.production)
- Docker Compose files
- Source code
- Documentation

It's hard to understand:
- Which services depend on which variables
- Where variables are defined vs where they're used
- What happens when you change one variable

## What It Does

- **Discovers** all env variables from compose, .env files, and source code
- **Builds** a dependency graph showing relationships
- **Analyzes** for issues: undefined vars, circular dependencies, orphans
- **Outputs** text reports, DOT graphs, or interactive TUI
- **Policy checks** — validate against custom rules
- **Baseline comparison** — detect drift from known-good state
- **Graph overlays** — compare multiple projects

## Installation

```bash
# Download for your platform
chmod +x envgraph
sudo mv envgraph /usr/local/bin/
```

## Usage

```bash
# Scan current directory
envgraph scan

# Scan specific path
envgraph scan /path/to/project

# Include source code scanning
envgraph scan --include-source

# Output as DOT graph
envgraph scan --graph dot > dependencies.dot

# Generate tree view
envgraph scan --graph tree

# JSON output
envgraph scan --format json

# Interactive TUI
envgraph scan --tui

# Policy validation
envgraph scan --policy .envgraph-policy.yaml

# Save baseline
envgraph scan --save-baseline .envgraph-baseline.json

# Compare against baseline
envgraph scan --baseline .envgraph-baseline.json

# Overlay multiple projects for comparison
envgraph scan --overlay ../other-project --overlay-mode diff

# Generate cleaned .env.example
envgraph scan --fix
```

## Example Output

```
envgraph v1.0.0

Scanning: /Users/dev/myproject

Sources:
  ✓ docker-compose.yml (12 variables)
  ✓ .env (8 variables)
  ✓ .env.example (10 variables)

Variables: 15 total (12 defined, 3 undefined)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Dependency Graph:
  DATABASE_URL
  ├── api (uses)
  ├── worker (uses)
  └── defined in: .env

  REDIS_URL
  ├── api (uses)
  ├── cache-worker (uses)
  └── defined in: docker-compose.yml

Issues:
  ⚠ SECRET_KEY used by api but not defined anywhere
  ⚠ LEGACY_TOKEN defined in .env but never used
```

## Policy File

Create `.envgraph-policy.yaml` for custom validation:

```yaml
# Required variables must exist
required:
  - DATABASE_URL
  - REDIS_URL
  - SECRET_KEY

# Naming conventions
naming:
  pattern: "^[A-Z][A-Z0-9_]*$"
  message: "Variables must be UPPER_SNAKE_CASE"

# Prefix rules
prefixes:
  DB_: 
    description: "Database-related variables"
    required_in: ["api", "worker"]
  
# Forbidden patterns
forbidden:
  - pattern: ".*PASSWORD.*"
    must_be_in: ".env.local"
    message: "Passwords should only be in .env.local"
```

## Graph Output Formats

### DOT (Graphviz)

```bash
envgraph scan --graph dot > deps.dot
dot -Tpng deps.dot -o deps.png
```

### Tree

```bash
envgraph scan --graph tree

DATABASE_URL
├─ api (consumer)
├─ worker (consumer)
└─ .env (source)
```

## Source Code Scanning

Enable `--include-source` to find env var usage in code:

| Language | Pattern |
|----------|---------|
| Node.js | `process.env.VAR_NAME` |
| Go | `os.Getenv("VAR_NAME")` |
| Python | `os.environ["VAR_NAME"]`, `os.getenv("VAR_NAME")` |
| Java | `System.getenv("VAR_NAME")` |
| Rust | `env::var("VAR_NAME")` |
| C# | `Environment.GetEnvironmentVariable("VAR_NAME")` |

## Related Tools

- [envmerge](https://github.com/ecent1119/envmerge) — Resolve env variable conflicts
- [envdoc](https://github.com/ecent1119/envdoc) — Generate env documentation
- [devcheck](https://github.com/ecent1119/devcheck) — Local project readiness inspector
- [stackgen](https://github.com/ecent1119/stackgen) — Generate local Docker Compose stacks

## Support This Project

**envgraph is free and open source.**

If this tool saved you time, consider sponsoring:

[![Sponsor on GitHub](https://img.shields.io/badge/Sponsor-❤️-red?logo=github)](https://github.com/sponsors/ecent1119)

Your support helps maintain and improve this tool.

## License

MIT License. See LICENSE file.
