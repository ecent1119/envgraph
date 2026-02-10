# envgraph

Visualize and validate environment variable usage across local dev projects and Docker Compose stacks.

---

## The problem

- `.env` files drift from reality
- Vars defined but unused
- Vars used but undocumented
- Conflicting names across services
- Hidden overrides via compose / Docker / runtime
- Onboarding failures because "it worked on my machine"

---

## What it does

- Scans `.env` files, compose files, and source code
- Builds a complete variable inventory
- Flags missing, unused, and overridden variables
- Generates readable reports and dependency graphs
- Understands layer precedence (`.env` < `.env.local` < compose)

---

## New in v2.0

- **Multi-language source scanning** — Node.js, Go, Python, Java, Rust, C#
- **Graph overlays** — compare multiple projects or environments
- **Baseline mode** — detect drift from known-good configuration
- **Policy checks** — validate against custom rules (naming, required vars, patterns)

---

## Example output

```
❌ DATABASE_URL used but not defined
⚠️  REDIS_HOST defined in .env but overridden in docker-compose.yml
⚠️  API_KEY defined but never used

Variable Inventory:
───────────────────
DB_HOST
  defined in: .env
  used in: docker-compose.yml, api/config.go

REDIS_URL
  defined in: .env, .env.local
  used in: docker-compose.yml, worker/main.go
  ⚠️  overridden: .env.local takes precedence
```

Service dependency graph:

```
postgres
 ├── POSTGRES_DB
 ├── POSTGRES_USER
 └── POSTGRES_PASSWORD

api
 ├── DATABASE_URL
 └── REDIS_URL

worker
 └── REDIS_URL (shared)
```

---

## Output formats

| Format | Use case |
|--------|----------|
| `--format text` | Human-readable terminal output |
| `--format json` | CI integration, tooling |
| `--format markdown` | Documentation, wikis |
| `--format dot` | Graphviz visualization |

---

## Scope

- **Observes**, does not enforce
- **Reports**, does not guarantee
- **Visualizes**, does not secure
- Read-only by default
- No code execution
- No telemetry

---

## Get it

👉 [Download on Gumroad](https://ecent.gumroad.com/l/hebmsl)

---

## Related tools

| Tool | Purpose |
|------|---------|
| **[stackgen](https://github.com/stackgen-cli/stackgen)** | Generate local dev Docker Compose stacks |
| **[dataclean](https://github.com/stackgen-cli/dataclean)** | Reset local dev data safely |
| **[compose-diff](https://github.com/stackgen-cli/compose-diff)** | Semantic Docker Compose diff |
| **[devcheck](https://github.com/stackgen-cli/devcheck)** | Local project readiness inspector |

---

## License

MIT — this repository contains documentation and examples only.
