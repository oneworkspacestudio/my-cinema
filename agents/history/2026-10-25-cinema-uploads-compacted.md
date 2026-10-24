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
