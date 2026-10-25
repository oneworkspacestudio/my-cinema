# Add videos follow-up

Context: `2026-10-24-my-cinema-compacted.md` records the original architecture,
runtime corrections and verified release/install of Cinema2 with Workspace85.
The user reported no way to add videos. Settings only connected existing folder
paths; there was no upload action. Prior request still authorizes push, release
and normal backed-up installation on both PCs. No subagents or Actions.

Implemented visible Add videos navigation, empty-state and Settings entry;
original AUI screen with browser file selection, destination, progress/cancel,
existing-folder connection and explicit new-folder creation. Uploads require the
owner and an unlocked adult profile. 8 MiB raw chunks avoid normal proxy request
limits and use server-acknowledged offsets; completion receipts recover lost
responses. 100 GiB/file bound, safe filenames/container signatures, rooted file
operations and Windows atomic directory publication preserve existing titles.
Partial staging is hidden; cancel/24-hour expiry removes only recorded partials.
Original source files stay untouched. Existing folders remain indexed in place.
TV structure and separate original-file backups are documented. Schema4 adds
uploads and completion receipts; normal updater rollback must restore its backup.

Go tests/vet and frontend build pass. Six browser acceptance checks pass on the
exact packaged candidate, including real runtime provisioning, new destination,
two-chunk video transfer with lost chunk AND completion replies, exact bytes,
real Jellyfin indexing and browser playback, duplicate protection, cancellation,
renamed nonvideo rejection, desktop/phone layout and zero browser errors. Synthetic
fixture only; no owner media/credentials used as functional fixtures. The first
test padded an MP4 with zeros and stalled playback; a real encoded 30-second
synthetic clip fixed that fixture. A placeholder escaping mismatch was fixed.

Candidate3 SHA256 eea5c0e69468968de03a8221e2c68c41fad3964cbc8ad546180f983f1fad3e4e.
Runtime pin remains 5de2eb4e17fe77132c3f681fcc80cd63846f9ce78d8e7b75d7f9ed7e292b16b3
(Jellyfin12.3, official FFmpeg8.1.2-1). Artifacts under the existing Cinema task
tooling, candidate-app3 and upload3-evidence; synthetic data on named NTFS C
fixture. Native process receipt is upload3-process.json. Stop parent and verified
children before cleanup; retain source/evidence/release and owner rollback.

At this checkpoint: source3 commit/tag, public release3 and channel promotion,
then both normal app-only updates to3, actual version/media-readiness and final
delivery receipts are pending. Workspace85 manager remains sufficient; preserve
all unrelated app versions/pins. Do not claim installs finished until verified.

## Corrected managed contract

Preview3 source292aefc was published with all15 asset hashes verified, but its
standalone update manifest inherited a schema1/integration2 minimumManager field
from preview2. Native85 correctly rejects that before any installation mutation
(manager requirements require schema2/integration3). Neither PC installed3.
Do not alter published3 assets/tag; withhold it and supersede with4. The media/UI
implementation is unchanged. Preview4 uses schema2/integration3 and minimum85,
with a new repository prepublication checker for all ZIP member hashes and the
requirements header. Also regenerate/compare using the manager's exported
MakeAppUpdateManifest + RequireManager before publication. Complete both normal
app-only installs and final evidence. Keep source/withholdings/history honest.

## Published correction and installed primary

Cinema4 source d8ff391f2ef88a510efd3cdbb588e20a56741e2e is public; all15
published asset sizes/digests and the unauthenticated channel were verified.
Its EXE is eb175cb610957b585c75467b9369397af47420319c57a4b5488a6888dbb44c37.
Six fresh browser checks passed on this exact packaged4, including actual indexed
playback, lost-response recovery and byte preservation. Named synthetic NTFS data
and its verified child processes were removed after acceptance; evidence remains.

The second PC's normal default catalog tracks four Cinema runtime/licence
packages, while full catalogs track ten including optional guides. Native85
rejects untracked guides in the valid4 manifest on the minimal layout. Workspace86
allows only the exact known missing Cinema guide/provenance ID/path pairs, using
the existing compatibility-document boundary. Runtime/foreign/moved/missing
tracked-document regression guards remain. Missing guides are not installed.
Both public minimal/full catalog checks pass. Never mutate published4 assets.

Workspace86 source d6a78d1f5c6147ab695f2c5524027b2cf4b08497 is public; all28
asset sizes/digests and the unauthenticated channel verified. Manager SHA256
514cc6d88ad96e40a05efc5897e9e026cb15dd6b65e7bd647fca75b22ce2dcdc. Channel
promotion a187fd8 retains all unrelated records and withheld3. No Actions.

Primary normal public app-only update to Cinema4 and then normal manager-only
update to86 completed. The online manager attempt safely failed its free-space
preflight and restarted85; the successful attempt used the exact public86 bundle
staged outside C, with all normal checksum/backup/rollback/startup guards. Both
installed EXE hashes, actual Cinema readiness, other app/package/executable bytes,
identity/OIDC/recovery credentials, paths/ports/preferences were verified. Latest
verified rollback retained; unknown cleanup items preserved. Receipts in task
tooling: upload4-primary-delivery.json and manager86-primary-delivery.json.

## Second PC completion and final delivery

The user-selected remote desktop PC completed one normal public Workspace86 +
Cinema4 transaction in23m59s. Other updates were unchecked. All apps healthy and
latest verified85 rollback retained; older automatic rollback cleaned normally.
Independent read-only remote PowerShell audit compared before/after app contracts,
package pins,25 other process hashes, owner/paths/ports/preferences and app count.
All passed; both final EXE hashes match public pins; actual Cinema4 readiness and
private Jellyfin12.3 startup complete. Four Cinema packages remain tracked.
This follow-up DOES include an independent remote filesystem hash audit, unlike
the prior2 delivery. A current86 public check reports Cinema4/Workspace86 up to
date. Social6 remains unchanged; its separate7 offer and old My3D review remain.

Remote audit receipt stays protected on that PC; task tooling retains sanitized
audit/version/completion screenshots and remote86-after.txt. Public aggregate:
docs/upload-delivery.json. Completed updater closed, task terminal closed and
remote input toggle restored. Existing unrelated services left running. Cinema
is open at its normal adult profile PIN gate; no owner fixtures were created.
First use: unlock adult owner profile, Add videos, connect a full server-side
destination path, then select/upload videos. Source, releases, promotion and both
installations complete. No running delivery operation or required feature work.
