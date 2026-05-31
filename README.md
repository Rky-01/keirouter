# KeiRouter

A fast, self-hostable AI gateway. Point your coding tools (Claude Code, Cursor,
Codex, Cline, OpenClaw, and any OpenAI/Anthropic-compatible client) at one local
endpoint, and KeiRouter routes requests across many providers with automatic
fallback, token-saving compression, encrypted credential storage, and spend
controls.

Written in Go for a small footprint (single static binary, ~20–30MB RAM idle,
instant startup) with a React + Tailwind dashboard.

> Status: active development. Implemented and tested: the core proxy; OpenAI /
> Anthropic / Gemini format translation (unary + streaming); token saving;
> capability-aware routing/fallback; metering; budgets; embeddings; a semantic
> response cache; Prometheus metrics; dashboard auth with first-run onboarding;
> CLI tool auto-config; and the React dashboard. Additional providers, OAuth
> flows, and the MCP bridge are in progress.

## Why KeiRouter

- **One endpoint, many providers.** Speak OpenAI or Anthropic; KeiRouter
  translates to whatever the target provider expects.
- **Never stop coding.** Routing chains fall back across accounts and providers
  on rate limits, quota exhaustion, or errors — without silently downgrading to
  a model that lacks a capability your request needs.
- **Spend less.** The Slimmer compresses bulky tool outputs (diffs, greps, file
  listings, build logs) before they reach the model. Terse mode trims output
  tokens. Budgets enforce hard USD caps per key, project, or org.
- **Secure by default.** Provider secrets are encrypted at rest with envelope
  encryption (AES-256-GCM). API keys are stored only as argon2id hashes and
  shown in plaintext exactly once. The dashboard is protected by a password
  (seeded on first run, changed via onboarding) and HMAC session cookies.
- **Observable.** Prometheus metrics at `/metrics` cover request volume,
  latency, tokens, cost, fallbacks, and cache hits.
- **Caches what repeats.** An optional semantic response cache returns stored
  answers for repeated prompts at zero cost and instant latency.

## Quick start

Build and run locally (Go 1.24+):

```bash
cd backend
go build -o ../keirouter ./cmd/keirouter
cd ..

# Start the server (defaults to 127.0.0.1:20180).
./keirouter
```

Then open the dashboard (run `cd frontend && npm install && npm run dev`, or
build and let the backend serve it). On first run, sign in with the default
password `keirouter`; the onboarding flow prompts you to set a new one.

You can also mint an API key from the CLI without the dashboard:

```bash
./keirouter -bootstrap   # prints a kr_ key once
```

Run both backend and dashboard together in development:

```bash
make dev   # backend on :20180, dashboard on :5180
```

Or with Docker:

```bash
docker compose -f deploy/compose.yaml up -d --build
```

Then add a provider account and a routing chain via the admin API (loopback
only by default):

```bash
# Add an OpenAI account.
curl -s localhost:20180/api/accounts -d '{
  "provider": "openai", "label": "personal", "api_key": "sk-..."
}'

# Create a fallback chain: try GPT-4o, then DeepSeek.
curl -s localhost:20180/api/chains -d '{
  "name": "coding",
  "steps": [
    {"provider": "openai", "model": "gpt-4o"},
    {"provider": "deepseek", "model": "deepseek-chat"}
  ]
}'
```

Point your tool at KeiRouter:

```
Base URL: http://localhost:20180/v1
API Key:  <your kr_ key from bootstrap>
Model:    openai/gpt-4o     # direct provider/model
          chain:coding      # or a named routing chain
```

## Routing model strings

The `model` field accepts:

- `provider/model` — a single explicit target, e.g. `openai/gpt-4o`.
- `chain:name` — a named routing chain with ordered fallback steps.
- `name` — shorthand for a chain named `name`.

## Architecture

```
backend/
  cmd/keirouter/        entrypoint
  internal/
    core/               canonical domain model (provider-agnostic)
    config/             koanf config (env + YAML)
    crypto/             envelope encryption + API key & password hashing
    store/              SQLite/Postgres repos + embedded migrations
    transform/          OpenAI / Anthropic / Gemini codecs (unary + streaming)
    connectors/         provider drivers (chat + embeddings) + catalog
    slimmer/            tool-output compression (token saver)
    terse/              terse-mode prompt injection (output token saver)
    capability/         model capability matrix (anti-downgrade guard)
    dispatch/           account selection + fallback + cooldown
    budget/             hard spend enforcement
    meter/              usage + cost recording
    cache/              semantic response cache + embedder
    observ/             Prometheus metrics
    auth/               dashboard password + session tokens
    identity/           API key issuance + authentication
    vault/              encrypted-credential <-> live-credential bridge
    pipeline/           request lifecycle orchestration
    gateway/            HTTP edge: auth, routing, admin API, /metrics
    app/                dependency wiring
frontend/               React + Vite + Tailwind dashboard
deploy/                 Dockerfile + compose
```

A request flows: gateway (auth, parse dialect) → pipeline (slimmer, terse,
budget guard) → dispatch (pick account, capability check) → connector (HTTP to
provider) → transform (translate response) → gateway (render in client dialect)
→ meter (record usage).

## Configuration

Copy `config.example.yaml` and pass it with `-config`, or use environment
variables prefixed `KEIROUTER_` with `__` for nesting (e.g.
`KEIROUTER_SERVER__PORT=8080`). SQLite is the zero-config default; set
`database.driver: postgres` with a DSN for team/VPS deployments.

## Security notes

- The admin API (`/api/*`) is restricted to loopback by default. When exposing
  KeiRouter beyond localhost, place it behind a reverse proxy with access
  control or a trusted network policy, and set a stable `master_key`.
- The master key is the root of trust for all stored credentials. Back it up;
  losing it makes encrypted credentials unrecoverable.

## Development

```bash
cd backend
go test ./...        # run the test suite
go vet ./...         # static checks
```

## License

TBD.