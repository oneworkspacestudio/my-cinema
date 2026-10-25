# Cinema Drive folders and OneAccount profiles

The user requested automatic Cinema/Movies, Cinema/TV Shows and Cinema/Home Videos
folders in My Drive, a real Drive folder browser instead of typed server paths,
and no extra viewing PIN after OneAccount sign-in. Original authorization to push,
release and install on both PCs persists. No subagents or GitHub Actions.
Previous source/release/delivery details are in
2026-10-25-cinema-uploads-compacted.md; both PCs started on Workspace86/Cinema4.

## Implementation

Cinema5 receives MYCINEMA_DRIVE_ROOT=${FILES} through its managed app catalog Env.
The existing manager expands the actual Drive root, so no manager rebuild is
needed. Managed mode fails closed if the root is missing/unavailable/overlaps
private state. Standalone defaults to adjacent Files. After verified engine
bootstrap, default directories and libraries are provisioned once per type.
cinema_settings markers preserve later owner choices/removals across restarts.
Existing libraries/files stay intact; metadata providers default off.

Owner/adult-only folder browsing returns rooted directory entries, excluding
links/junctions, traversal/absolute paths and private-data overlap. UI in AUI has
My Drive, nested folders, Up, Use this folder and Cancel. Settings/Add videos
provide Change folder and Browse My Drive; no editable server path. Changing
an existing library retains its identity, adds the new engine path/removes the
old with compensation on failure, blocks during unfinished uploads and leaves
all media files where they are. Legacy external libraries remain connected.

Only server-validated sessions from the configured OneAccount issuer replace
PIN checks, creation requirements and adult PIN expiry. Existing PIN hashes
remain for local standalone compatibility but are not requested by OneAccount.
A single OneAccount viewing profile opens automatically; multiple are one-click
choices. Account expiry, household ownership, owner/adult administration,
selected child policy, session/profile resource binding and CSRF stay enforced.
Local standalone adults retain PINs. No database schema change (still4).

## Acceptance

Go tests/vet and frontend build passed. New synthetic HTTP regression covers
default provisioning/retry, owner choice persistence, old-file preservation,
folder traversal/junction/anonymous/child/member denial, active-upload blocking,
compensated engine failure, existing PIN bypass for verified OneAccount only,
local PIN retention and expired account-session rejection.

Exact candidate EXE SHA256
3c65129c3705a0a73cbf856eee8fa8bf316c3ae0f23bf1c89347d901951f21ed.
Seven fresh upload/UI checks passed: automatic defaults, nested desktop/phone
picker, real destination change, lost chunk/completion replies, byte equality,
real indexing/browser playback, duplicate preservation/cancel/nonvideo rejection.
The first picker test read settings before its asynchronous save completed;
waiting for the picker to close fixed that test timing; production unchanged.

Seven fresh managed checks passed with real MyAccounts code/PKCE, actual shared
authenticated My Drive access, native stop/restart choice persistence, independent
browser PIN-free selection, child denial and another real subject's denial.
Fixture uses historical85 installer only as a fresh Accounts/Drive provider,
then exact5 executable and production Env contract. Do not claim a new whole5
installer, clean host or physical reboot. Synthetic protected NTFS only; owner
Accounts/Drive/media/PINs were not functional fixtures. Reports/screenshots in
docs and task tooling drive5-evidence / managed-drive5-evidence. Managed fixture
stopped normally; standalone acceptance fixture awaits cleanup.

## Remaining at source freeze

Publish immutable5 with schema2/integration3, minimumManager86 and exact Env
contract; regenerate/compare via Workspace SDK, validate minimal/full layouts,
promote/verify unauthenticated channel. Then normal backed-up app-only updates
on both PCs, independent final hashes/readiness/default-folder audits, preserve
all other apps and latest rollback, clean named fixtures and push final handoffs.
Never repeat an active update. Runtime pin unchanged: Jellyfin12.3/FFmpeg8.1.2-1,
archive5de2eb4e17fe77132c3f681fcc80cd63846f9ce78d8e7b75d7f9ed7e292b16b3.
