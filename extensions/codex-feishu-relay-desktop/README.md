# Codex Feishu Link Desktop companion

This local VS Code extension polls `~/.codex/feishu-desktop/requests` once per second. The broker creates this directory with mode 0700 and atomically publishes mode 0600 `<id>.json` files:

```json
{"version":1,"id":"unique-request-id","cwd":"/absolute/project","threadId":"thread-id","createdAt":"2026-09-28T00:00:00Z","daemonEpoch":"daemon-start-identity","cliExecutable":"/absolute/install/bin/codex-vscode-link"}
```

Only a window containing the canonical project directory handles a request. Requests expire after 120 seconds. Configure the application-scoped trusted launcher before use (its default is empty). The executable must exactly match the application-scoped `codexRemoteDesktop.cliExecutable` setting; a request cannot choose an executable. The launcher must already exist and be executable. The launcher must connect to the shared daemon, including initialization translation if required by the installed Codex extension. This companion never starts/stops the daemon or submits prompts.

The extension configures `chatgpt.cliExecutable` at user scope. An already activated Codex extension requires one window reload when the daemon epoch or launcher changes. A persisted request ticket prevents repeat reload loops; after reload, polling resumes and opens `openai-codex://route/local/<threadId>` through the Codex custom editor. Reloading a window affects all extensions in that window. The broker should request this only when recovering the selected project's unavailable backend.

Results are private `<id>.result.json` files with `id`, `status` (`opened` or `failed`), `threadId`, `cwd`, and optional `error`. `opened` means the editor command completed, not that a model prompt was submitted or completed. A failed request is retried by issuing a new ID with a fresh timestamp; reuse of a completed ID is ignored. Per-request `<id>.claim` files (JSON `id`, `createdAt`, `pid`, mode 0600) prevent two windows handling the same request; abandoned claims recover after 120 seconds. Claims remain present across window reload, and the persisted reload ticket authorizes continuation. The broker should clean old request/result files after collecting results.

Install this directory as a local VS Code extension (or package a VSIX); installation and settings changes are deployment steps. Test with `npm test` or `node --test test/*.test.js`. Source inspection verified the custom editor route against installed `openai.chatgpt` 0.4.56; future versions may require adaptation.
