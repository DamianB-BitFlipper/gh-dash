# PipeDiffsHub

Open a piped unified or git diff in a browser using Pierre's diff viewer code.

```sh
bun install
bun run build
git diff | bun run pipediffshub
```

The CLI keeps the diff in memory, starts a localhost Bun server, opens the
browser, and serves the diff to a small React viewer.
