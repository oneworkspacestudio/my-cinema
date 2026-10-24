# Settle into My Cinema

## In One Workspace

Open My Cinema from the app picker. MyAccounts provides your real OneAccount
sign-in. Create your first adult viewing profile and choose a 4–12 digit PIN.
Adult profiles always require the PIN when selected. Each household can have six
profiles. A Kids profile can only be created or changed after unlocking an adult.

The Workspace owner can choose **Add videos** in the header after unlocking an
adult viewing profile. First connect a destination folder: give it a name, choose
Movies, TV Shows or Home videos, and enter its full path on the Workspace PC.
Select **Create this folder if it does not exist** to create a new folder inside
an existing parent. Paths refer to the PC running Workspace, including when you
use Cinema from a phone or another computer.

To upload, choose a connected folder, select videos from your device and press
**Upload selected videos**. Keep Cinema open until it reports the videos saved.
The page shows progress and provides Cancel upload. Videos transfer in small
chunks with acknowledged offsets, allowing a lost request to be retried. Each
completed video is saved in its own title folder. Existing files/folders are
never overwritten; rename a duplicate upload instead. Cancelling removes only
the unfinished upload and keeps videos already completed. Interrupted staging
files stay hidden from playback and expire after 24 hours when their library is
available. Uploads support MP4, MKV, WebM, MOV, AVI, M4V and TS, up to 100 GB each.

For videos already on this PC or in My Drive, connect the same real directories
that My Drive exposes. Connecting a folder only indexes it; no copying or moving
is performed. TV libraries should use Series / Season / Episode structure; connect
an existing structured folder for a complete series. Uploaded originals on your
device are retained, and originals saved on the Workspace PC need a file backup.
Directories cannot traverse links or junctions or point into Cinema's private data.

For example: D:\Media\Movies and D:\Media\TV Shows. UNC shares require the current
Windows account to already have read access. Paths appear only in owner settings.

Jellyfin reads ordinary media folders and local NFO metadata. Internet metadata and
artwork lookup is an explicit library option; enabling it can send media filenames
to configured metadata providers. Missing artwork has an original visual fallback.
The watcher is supplemented by 45-second stable-file reconciliation. Scanning does
not block browsing. Removing a library removes configuration and keeps every file.

Use **Scan for new videos** after copying more files into a connected folder, or
wait for automatic discovery. New uploads also trigger a media scan.

Use title details to resume/restart, mark watched, choose audio/subtitles or switch
versions. Native player controls provide volume, pause, seeking and captions;
the toolbar adds quality, speed and fullscreen. Up Next only counts down after an
episode ends and auto-play was enabled. There are no invented intro timestamps.

## Portable Windows

Extract the `MyCinema` folder and double-click `MyCinema.exe`. The launcher contains
the exact pinned portable engine archive and verifies it before extraction. Data lives in
the adjacent `data` directory; a private pinned engine is extracted there. Initial
database migration can take longer than subsequent launches. No Docker, Node,
installer service, global database or system PATH change is required.

Standalone mode uses an explicitly labelled local owner account. It does not
pretend to be OneAccount. For managed real OIDC, use One Workspace; developer
configuration is described in DEVELOPMENT.md. There is no external sharing by
default. LAN/remote access uses an explicitly configured HTTPS Workspace proxy.

## Back up and restore

In a managed installation use Settings → Open Workspace Backups. This is the normal
verified stop/checkpoint/backup/restore flow and includes Cinema state, bridge,
Jellyfin databases, configuration and metadata. Original videos need a separate
file backup. Updates use the normal checksum-verified transaction with a recovery
copy; keep the newest usable rollback.

For a standalone installation, fully close My Cinema, confirm its Jellyfin child
has stopped, then copy the complete `data` directory to a private backup location.
Restore that complete copy into a new folder, keeping the original until verified.
Do not use the shared apphost's generic database-only backup for the media engine.

Windows DPAPI protects bridge credentials. Restoring/copying to the same Windows
account on the original PC retains them. A different PC/account requires a fresh
Cinema connection and OneAccount authentication; do not copy plaintext credentials.
The app fails closed if it cannot decrypt the matching bridge. Library media paths
are preserved, so check moved drive letters before scanning. Video files are never
overwritten by updates or a library removal.

## Troubleshooting

An occupied private port leaves unrelated programs running. Choose another Cinema
application port; its private Jellyfin port is that port plus 100. Missing runtime
or checksum mismatch requires re-extracting the complete verified release. Engine
startup failures appear on the warming-up screen and local logs, without leaking
credentials. Keep diagnostic logs private: Jellyfin's own logs can contain media
paths. A missing drive stays visible in owner settings. Restore a matching complete
backup if the bridge and engine database no longer correspond.
