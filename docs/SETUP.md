# Settle into My Cinema

## In One Workspace

Open My Cinema from the app picker. MyAccounts provides your real OneAccount
sign-in. Create your first adult viewing profile and choose a 4–12 digit PIN.
Adult profiles always require the PIN when selected. Each household can have six
profiles. A Kids profile can only be created or changed after unlocking an adult.

The Workspace owner can add libraries in Settings. Give each folder a name, choose
Movies, TV Shows or Home videos, and enter its full path on this PC. Use the same
real directories that My Drive already exposes. No copying or moving is performed.
Directories cannot traverse links or junctions or point into Cinema's private data.

For example: D:\Media\Movies and D:\Media\TV Shows. UNC shares require the current
Windows account to already have read access. Paths appear only in owner settings.

Jellyfin reads ordinary media folders and local NFO metadata. Internet metadata and
artwork lookup is an explicit library option; enabling it can send media filenames
to configured metadata providers. Missing artwork has an original visual fallback.
The watcher is supplemented by 45-second stable-file reconciliation. Scanning does
not block browsing. Removing a library removes configuration and keeps every file.

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
