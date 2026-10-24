# Custom streaming application

My Cinema uses an original React/TypeScript/AUI frontend embedded in a Go launcher.
The shared Workspace apphost supplies real MyAccounts OIDC discovery, code/PKCE,
nonce/state checks, bounded sessions, HttpOnly cookies, CSRF and private SQLite.
OneAccount in the brief refers to the existing MyAccounts identity service.
Principal identity is the issuer plus immutable subject, never email.

Jellyfin 12.3 is a private portable sidecar. Every viewing profile has its own
non-administrator Jellyfin identity and restricted policy. Only the broker handles
credentials. Media remains in ordinary approved folders; removing a library never
deletes files. The UI calls only same-origin application endpoints.

Windows DPAPI protects bridge credentials and binds them to the current Windows
account/machine. Relocated installations require re-linking. No community OIDC
plugin is required. Existing upstream services and owner files are not test data.
