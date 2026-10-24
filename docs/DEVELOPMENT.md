# Build and verify

Build on Windows with Go 1.27.3 or newer patched compatible toolchain, Node and npm.
These are development tools and are not runtime prerequisites for users.

```
cd web
npm ci
npm run build
cd ..
go test ./...
go vet ./...
pwsh -File scripts/build.ps1 -RuntimeZip /path/to/mycinema-runtime-12.3-8.1.2.zip
```

`scripts/build.ps1` builds the frontend/launcher and appends the verified pinned
portable archive to the executable as a PE data overlay with a bounded length footer. AUI sources generate React
deterministically; edit .aui sources rather than generated output. Frontend server
calls use only same-origin application endpoints.

A plain `go build` produces a development launcher without the runtime overlay;
use the packaging script for portable distribution. The exact vendor download
and checksum are recorded in SOURCE-PROVENANCE.json.

`--data`, `--listen`, `--origin`, `--no-browser`, `--version` follow the native
Workspace apphost contract. `WORKSPACE_OIDC_ISSUER`, `WORKSPACE_OIDC_CLIENT_ID`,
`WORKSPACE_OIDC_CLIENT_SECRET`, `WORKSPACE_OWNER_SUBJECT`, `WORKSPACE_HOME_URL`
configure actual OIDC. The client redirect is the exact app origin plus
`/auth/callback`, with scopes `openid profile email`. Secrets must stay server-side.
The normal Workspace installer provisions these clients; no endpoints are assumed.

`web/scripts/acceptance.mjs` drives an explicitly synthetic standalone fixture.
After packaging the executable with build.ps1, run
`node scripts/native-acceptance.mjs` to create a new synthetic NTFS installation,
generate original media with the bundled FFmpeg, execute the browser checks and
stop its processes. Set CINEMA_QA_PORT if the default test port is occupied.
Never point it at an owner installation: it creates test profiles/libraries and
changes test playback state. Managed acceptance installs a fresh selected suite
and tests actual MyAccounts OIDC separately. All source builds/publications are
local. GitHub Actions remain disabled.

API errors are JSON `{error: string}` with no-store. Authenticated writes require
the same Origin and `X-CSRF-Token` from `/api/me`. Principal profiles are isolated
by apphost's issuer+subject hash. Profile selection is bound to a hashed browser
session. Stream IDs and resource IDs are opaque, bounded and profile/session-bound.
Media authority is rechecked before each manifest, segment, artwork and detail.

See ADR-001-architecture.md and SECURITY-CHECKS.md for the engine and trust model.

Use the pinned runtime assembly asset from the same My Cinema release. The optional Python assemble-runtime.py records how unmodified official server and encoder archives are combined; ZIP deflate bytes can vary with zlib. The published runtime checksum remains mandatory.
