# Feature completion and verification

The brief defines a phased product. This release provides the functional streaming
core and responsive UI, with the following explicit boundaries.

| Capability | Status |
|---|---|
| Original embedded React streaming UI, cinematic hero, rails, details, catalogs | Implemented |
| Real OneAccount/MyAccounts discovery, code + PKCE, nonce/state, subject mapping | Managed integration; acceptance receipt records the exact tested build |
| Separate restricted Jellyfin identities, PIN adult profiles, child unknown-rating deny | Implemented; native restricted metadata/search/stream checks |
| Household authorization | First household creation requires the Workspace owner; no arbitrary member may self-grant library access |
| Per-profile favourites, progress, resume, watched flags | Implemented; real engine writeback |
| Movies, seasons, episodes, next episode | Implemented; specials stay explicitly selectable in the season list |
| Native MP4/WebM and Jellyfin remux/HLS selection | Implemented; H.264/AAC MP4 and MKV transcode acceptance |
| Audio/subtitle/version selection and WebVTT conversion | Implemented; representative acceptance only, not every codec/device |
| Keyboard, touch, TV spatial focus, reduced motion | Implemented; desktop/tablet/phone/TV browser layouts checked |
| My Drive shared filesystem roots, stable polling, scans, non-destructive removal | Implemented; originals are indexed in place |
| Genre similarity suggestions, truthful date-added view | Implemented; no fabricated popularity figures |
| Internet metadata lookup | Explicit library option; local NFO works without it |
| Managed verified updates and complete stop/checkpoint backup/restore | Existing Workspace workflow, linked from Settings |
| Standalone backup | Complete stopped-data-folder copy; no standalone graphical restore wizard |
| Existing remote Jellyfin connector / portable secret export vault | Not included in this first managed-sidecar release |
| Quantum API import picker, copy/move jobs, hashes/undo | Extended phase; shared paths require no duplication and are available now |
| PWA installation/offline downloads, device pairing, native TV package | Not included |
| Trailer autoplay, Skip Intro, global Top 10, likes | Not shown without real licensed data/markers/measurements |
| Curated collections, content requests, viewing-history export | Extended phase |
| Clean Windows host, reboot/autostart, arbitrary TV remote/hardware acceleration | Not certified by workstation-based acceptance |

No stock Jellyfin page is exposed through Cinema. The browser never receives a
Jellyfin administrator key, password or unrestricted upstream media URL. There are
no production fixture rows. Screenshots use original synthetic test films, not
commercial artwork. Library setup and a viewing profile remain necessary before
the user's real collection appears on either installed machine.
