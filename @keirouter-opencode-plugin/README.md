# @keirouter/opencode-plugin

Minimal OpenCode plugin for KeiRouter.

It only fetches `GET /v1/models`, caches the result in memory, and exposes the
models to OpenCode. It does not call combo, auto-combo, pricing, or MCP endpoints.

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "plugin": [
    ["@keirouter/opencode-plugin", { "baseURL": "http://127.0.0.1:20180" }]
  ]
}
```

Then run:

```sh
opencode connect keirouter
```

Options:

- `providerId` default `keirouter`
- `displayName` default `KeiRouter`
- `baseURL` default `http://127.0.0.1:20180`
- `modelCacheTtl` default `300000`
