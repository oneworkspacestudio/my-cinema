# Security and compatibility

Pinned compatibility: Windows amd64; Jellyfin Server 12.3 (API 12.3.0);
jellyfin-ffmpeg 8.1.2-1 / FFmpeg 8.1.2-Jellyfin. Portable archive checksum is in
THIRD-PARTY-LICENSES.txt and enforced before extraction. Fresh setup requires the
legacy first-user GET before updating the startup user. User policy updates merge
the server's required authentication/password-reset provider identifiers.
Jellyfin uses dashed UUIDs in generated HLS URLs and compact IDs in item DTOs;
authorization normalizes that representation while enforcing the same exact item.

Identity uses native apphost discovery, signatures/audience/issuer, code/PKCE,
state/nonce, immutable subject identity, HttpOnly cookies and bounded expiry.
Cookie writes require the exact Origin and CSRF token. A verified session from the
configured OneAccount issuer replaces the viewing PIN, including for existing
PIN-protected profiles. Account session expiry, owner checks and profile ownership
still apply. Local standalone PINs use bcrypt cost 12 with rate-limited selections.
Switching profiles cancels previous stream
contexts. Child unknown/unrated items are denied by both the broker and engine.
The restricted engine identity applies to arbitrary item IDs and upstream policy.
Profiles, libraries, playback sessions and opaque resources cannot be selected by
caller-supplied filesystem paths or credentials.

The runtime listens only on loopback. Automatic port mapping and discovery are
disabled. No public Jellyfin admin proxy exists. Admin credentials and restricted
tokens use Windows DPAPI. CSP allows only same-origin media; upstream tokens never
reach browser manifests, DTOs or artwork URLs. External redirects/resources in HLS
are rejected. Range requests are authorized before forwarding. No analytics.

Synthetic native acceptance covers indexing, direct video bytes, HLS transcoding,
progress/resume, isolated favourites, child search/detail/playback/admin denial,
PIN rejection, CSRF, anonymous access, UI playback and responsive overflow. Reports
and screenshots are retained alongside each release. This is not a penetration
test, legal certification, clean-host or arbitrary-device performance guarantee.

Jellyfin's own local diagnostics can include filesystem paths and upstream stream
parameters. They remain inside protected app data and are not returned through
the user API. Review and redact logs before sharing them.

Preview.4 delivers owner-only video uploads behind an adult profile. Every
chunk, completion, status and cancellation request verifies the current account,
session and profile; writes retain Origin/CSRF enforcement. Destination IDs map
to existing validated library roots; clients cannot supply arbitrary write paths.
Plain filenames, Windows reserved names, container signatures, total size,
8 MiB chunk bounds and acknowledged offsets are checked. Files stay in a named
staging directory until complete; Windows directory publication cannot replace
an existing title. Cancellation/expiry removes only that recorded staging job.
Completed receipts make a lost completion response recoverable and expire without
deleting the saved video. Go HTTP tests cover these boundaries. The separate
upload acceptance report covers real two-chunk browser transfer, lost replies,
byte equality, actual indexed playback, duplicate/cancel and phone layout using
synthetic media. No existing owner library was used as a fixture.

Cinema5 provisions the three Drive destinations once, with metadata providers off
by default. The owner/adult-only folder browser rejects traversal, absolute paths,
links/junctions, private-data overlap and whole-Drive library selection. Existing
external libraries remain connected. Folder changes retain the library identity,
block during unfinished uploads and compensate failed engine/config changes.
Files are never moved or deleted. OneAccount PIN replacement is based on the
server-validated session issuer, never a client flag or request header.
