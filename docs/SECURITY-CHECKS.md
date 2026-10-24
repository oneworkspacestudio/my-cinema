# Security and compatibility

Pinned compatibility: Windows amd64; Jellyfin Server 12.3 (API 12.3.0);
jellyfin-ffmpeg 8.1.3-1 / FFmpeg 8.1.3-Jellyfin. Portable archive checksum is in
THIRD-PARTY-LICENSES.txt and enforced before extraction. Fresh setup requires the
legacy first-user GET before updating the startup user. User policy updates merge
the server's required authentication/password-reset provider identifiers.
Jellyfin uses dashed UUIDs in generated HLS URLs and compact IDs in item DTOs;
authorization normalizes that representation while enforcing the same exact item.

Identity uses native apphost discovery, signatures/audience/issuer, code/PKCE,
state/nonce, immutable subject identity, HttpOnly cookies and bounded expiry.
Cookie writes require the exact Origin and CSRF token. PINs use bcrypt cost 12
and failed selections are rate-limited. Switching profiles cancels previous stream
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
