# Settle into My Cinema

## In One Workspace

Open My Cinema from the app picker and sign in with OneAccount. There is no
additional viewing PIN. If you have one viewing profile, Cinema selects it
automatically; with several profiles, choose who is watching. Create your first
adult viewing profile if this is a new household. A household can have six
profiles. Kids profiles keep their content restrictions while selected.

Cinema automatically creates **Cinema/Movies**, **Cinema/TV Shows** and
**Cinema/Home Videos** inside the ordinary files shown by My Drive, then connects
them as media libraries. Open **Add videos**, choose the destination, select videos
from your computer or phone and press **Upload selected videos**. No folder path
or manual library setup is needed.

In Add videos or Settings, choose **Change folder** next to a library to browse
My Drive. Open folders, use **Up one folder** or **My Drive** to navigate, then
press **Use this folder**. The choice survives restarts. It changes the library's
location without moving or deleting videos from the previous folder. You can also
browse and connect another existing Drive folder. Links/junctions and private app
data cannot be selected. An active upload must finish or be cancelled before its
destination can change.

Keep Cinema open until an upload reports the videos saved. It shows progress and
provides Cancel upload. A lost request can be retried using acknowledged offsets.
Each completed video is saved in its own title folder. Existing files/folders are
never overwritten; rename duplicate uploads instead. Cancelling removes only the
unfinished upload and keeps completed videos. Interrupted staging stays hidden
from playback and expires after 24 hours when the library is available. Supported
uploads: MP4, MKV, WebM, MOV, AVI, M4V and TS, up to 100 GB each.

You can also copy videos into these folders using My Drive. Cinema indexes them
in place. For TV shows, keep the Series / Season / Episode structure. Original
videos on your device stay untouched. Videos saved on the Workspace PC need their
own file backup.

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

Standalone mode uses an explicitly labelled local owner account. Its adult viewing
profiles retain the local PIN; OneAccount profiles never require that extra PIN.
The default media folders are in the adjacent Files/Cinema directory. It does not
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
