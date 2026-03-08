---
id: hello-endpoint
title: Add hello endpoint
state: To Do
---
Goal
Add a `/hello` endpoint that returns JSON:

```json
{"message":"hello"}
```

Context
- Work only inside the cloned repo in the current workspace.
- Keep the change narrowly scoped.
- Avoid unrelated refactors.

Requirements
- Add or update the handler and route.
- Add or update tests.
- Keep the response format exact.

Validation
- Run `go test ./...`

Done when
- `/hello` returns the expected JSON payload.
- Tests pass.
- The final `task_update` summary explains what changed and what validation ran.
