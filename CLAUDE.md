# ImageOrganizer: notes for Claude Code

A self-hosted photo library in Go: browse, search and organise photos in the
browser; faces are found, grouped and named. See README.md for the feature
list. This fork (hcfryd-dotcom/imageorganizer) is used on Windows by a
Danish-speaking owner (Henrik). **Talk to him in Danish**, in plain words; he
is not a programmer. Code, comments and commit messages stay in English, like
the rest of the repo.

## The owner's setup (Windows 11)

- Source: `C:\Users\heinz\Downloads\imageorganizer`, a git checkout of
  `main` (pull request #1 is merged), remote `origin` = GitHub; he has
  asked to push his work there. His GitHub login is stored by Git Credential
  Manager (a push from Claude's shell works; if it asks for a login, run it
  in the Terminal panel so the browser sign-in can open). Git for Windows is in
  `C:\Program Files\Git\cmd`; a shell opened before it was installed may
  need that added to PATH. `modeldele\` (the face model split in parts, with
  `saml-model.bat` to join it) and `ONNXRUNTIME-LICENSE.txt` are his local
  files, kept out of git via `.git\info\exclude`.
- Build: `build-windows.bat` (Go 1.27 + MinGW gcc via WinLibs, cgo on), which
  writes `dist\ImageOrganizer\imageorganizer.exe` and
  `dist\ImageOrganizer-windows-x64.zip`. It keeps `dist\ImageOrganizer`, which
  also holds `onnxruntime.dll` (ONNX Runtime **1.29.1**, matching
  onnxruntime_go v1.36.0) and `llama\` (llama-server for search by meaning).
- Run: `dist\ImageOrganizer\Start ImageOrganizer.bat` opens
  http://127.0.0.1:8420. The server runs while its console window is open.
  To rebuild, close that window first (the .exe is locked while running).
  Started from Claude's shell (Start-Process), the .bat exits at once
  without the program; start `dist\ImageOrganizer\imageorganizer.exe`
  directly instead (Start-Process with that working directory), which
  opens its own console window.
- Library data: `C:\Users\heinz\.local\share\imageorganizer` (library.bolt,
  thumbnails, face crops, `models\`). **Never delete or reset it**: it holds
  his naming of ~23,000 faces.
- Face models in `models\`: `scrfd_10g.onnx` and `arcface_w600k_r50.onnx`
  (InsightFace buffalo_l, non-commercial licence) and `mivolo_v2_age.onnx`
  (ages), `genderage.onnx` (gender) and `2d106det.onnx` (eyes).
- Photos: the library's one folder is `E:\Onedrive\Billeder ryddet` (~39,000
  files, 92 GB, inside his OneDrive, pinned "always keep on this device" with
  `attrib +P`). Made on 2026-10-14 by the tidy copy from `E:\Onedrive\Billeder`
  and `E:\Old_Onedrive` into `E:\Billeder ryddet`; moved into OneDrive on
  2026-10-17 at his wish (each copy checked by SHA-256 before its original
  was deleted; log `E:\ImageOrganizer-flytning.log`; library backup from just
  before: `E:\ImageOrganizer-backup\library.bolt.before-onedrive-20261017`;
  folder id 8, the old id 5 removed). The two old folders are still on
  disk, untouched, no longer in the library; deleting them is his call.
  Backup of the library from just before the switch:
  `E:\ImageOrganizer-backup\library.bolt.before-switch-20261014`.
  `C:` is nearly full (~3 GB free): put big temporary files on `E:`.
- Search by meaning is on since 2026-10-14: llama-server is in
  `dist\ImageOrganizer\llama\`; `llama\models` is a junction to
  `E:\ImageOrganizer-models` (EmbeddingGemma 2 `embeddinggemma-2-Q8_0.gguf`
  and `mmproj-Q8_0.gguf` from unsloth/embeddinggemma-2-GGUF, checksums
  verified). The Start .bat starts llama-server on port 8095; when Claude
  starts the .exe directly, start llama-server too (minimized, working dir
  `llama`, `-m models\embeddinggemma-2-Q8_0.gguf --mmproj
  models\mmproj-Q8_0.gguf --embeddings --port 8095 --host 127.0.0.1 -ngl 99`).
  Setting "embed_url" = `http://127.0.0.1:8095`. It runs on his Radeon
  RX 570 through Vulkan: `ggml-vulkan.dll` from the same llama.cpp build
  (b11574, `llama-b11574-bin-win-vulkan-x64.zip`, unpacked in
  `E:\ImageOrganizer-llama-vulkan`) was added to `llama\`, and the .bat
  passes `-ngl 99`; ~0.4 s per photo against ~5 s on the CPU. Vectors are
  pinned to the model's file name (`embed.modelName`), not its path.

## What this fork adds (on main; PR #1 and after)

- Builds without cgo (`internal/faces/engine_nocgo.go`: no faces), and on
  Windows looks for `onnxruntime.dll` next to the program.
- Faces are grouped even without an age model (`Store.NoAgeModel`); before,
  no face was ever grouped without one.
- Find face: no match gave `null` and a blank card that saved to group
  "undefined"; fixed.
- Review is in the sidebar, with a bar to file every inbox group at least
  N% like someone named ("GO", `/api/review/confident`); groups as alike to
  two people are left out.
- Person page: "Copy to folder…" copies a person's photos to
  `<folder>/<year>/` and optionally `/<area or town>/` from GPS
  (`internal/places`: GeoNames towns, embedded, offline).
- Delete: the viewer's trash button / Del key moves a photo's files to a
  deleted-photos folder (default `Pictures\ImageOrganizer - Deleted`), never
  erases them; the library keeps the photo's data.
- Places for photos without GPS, kept only in the library (`GeoSrc` on
  the image; `internal/store/geo.go`, `/api/geo/*`): Map → "More places…"
  lends a photo the position of one taken within N hours (`near`);
  Photos → Select → "Set place…" sets one by town search or map click
  (`user`). A position the file gets later wins. Files are never written.
- Folders → "Collect everything in one tidy folder" (`/api/tidy`,
  `internal/server/tidy.go`, `store/tidy.go`): copies every photo of all
  library folders once into `<dir>/<year>/<MM Month>/<source>/` (source:
  iPhone, camera brand, Screenshots, Unknown camera; names in the UI's
  language). Copies of one photo (same second, same pixels, size within
  2%, near-identical perceptual hash) are left out; the biggest is kept.
  Refuses a folder inside or around a library folder. Read-only on the
  library folders; copying again skips files already there.
- Same panel, "Fetch new photos" (`/api/tidy/fetch`, `server/fetch.go`):
  copies photos the library doesn't have (by the importer's content hash)
  from an outside folder, e.g. `E:\Onedrive\Billeder` where his iPhone
  uploads, into the tidy folder sorted the same way, then rescans.
  Files looked at are remembered in `<data>/fetch-seen-*.json`, so only the
  first run reads everything.
- Videos (`media/video.go`): indexed only when ffmpeg + ffprobe are found
  (`dist\ImageOrganizer\ffmpeg\`, BtbN LGPL 8.1 build); a frame near the
  start is the thumbnail and goes through faces; the viewer plays the
  original (.mov served as video/mp4) and "Save picture" writes the paused
  frame next to the video (`server/frame.go`).
- Albums (`store/albums.go`, `/api/albums`, sidebar "Album"). "Album from a
  folder…" (`POST /api/albums/from-folder`) makes an album of a folder's
  photos, found in the library by content hash (the folder is only read).
  On 2026-10-22 51 albums were made so from his named folders in
  `E:\Onedrive\Billeder\Billeder`, then deleted again the same day at his
  wish (only his own "Mathilde 18 år" kept); don't make them again unasked.
  An album button on each photo in
  the grid and in the viewer (key A) adds that photo to an album; with a
  default album (☆ in "Add to album…", localStorage `defaultAlbum`, its
  name in the top bar) it goes straight in, Shift chooses another.
  An album can have a folder of its own ("Copy to folder…" on the album
  page, `albumRec.CopyTo`, `POST /api/albums/{id}/copy-to`,
  `server/albumcopy.go`): its photos, and each one added later, are copied
  there in the background, into <year>/<MM Month>/ folders like the tidy
  copy (names in the UI's language, `CopyLang`); copies are never taken back.
  A photo can have a number and a title in an album (`albumRec.Marks`,
  `POST /api/albums/{id}/mark`; set when adding one photo, or by the badge
  on it in the album). An album opens sorted "album" (`store.SortAlbum`:
  numbered photos by number, then the rest oldest first), and a photo book
  of an album with marks follows that order: a title is the caption, one
  starting with "Kapitel"/"Chapter" a page of text before its photo, and
  marked photos are always in it.
- Videos: sidebar "Videos" shows only the videos (`video=1`, or `$video`
  typed in the search; `Filter.Video`), where they lie in the folders;
  nothing is moved. The library stats count them (`videos`).
- Daily backups of library.bolt (`internal/backup`), set to
  `E:\ImageOrganizer-backup`, 14 kept; Settings shows the last one.
- Eyes: `models\2d106det.onnx` (InsightFace buffalo_l 106 landmarks,
  non-commercial, from the official buffalo_l.zip v0.7 on 2026-10-15).
  Gender: `models\genderage.onnx` from the same pack
  (`faces/genderage.go`), used because MiVOLO here gives age only; its
  readings are moved by `gaBias` (tested on 120 adult faces of eight
  known people: 98 right). Gender-only jobs skip MiVOLO. The "eyes" job judges each face
  crop once (`faces/eyes.go`: the more open eye's height over width;
  under 0.16 is shut or looking down); photo books can leave them out.
- Age model `models\mivolo_v2_age.onnx` (Sam-Apostel/mivolo-v2-age-onnx,
  Apache-2.0; age only, no gender) since 2026-10-15.
- Review has a slider for the card/face size.
- Photo books (`web/book.html`, `store/books.go`, `server/books.go`,
  sidebar "Photo books"): saved in the library (`books` bucket; Source is
  the query the photos come by: album, person, people=a,b, from/to,
  near=lat,lon&km, chapters=year|month, pick=N; Data is the page's own
  JSON, v2: pages with photo ids). "The best N" is picked in the page
  (no blurry, screenshots or bursts; spread over chapters or events).
  `/api/books/info` gives town and names for captions. Books once kept
  in the browser's localStorage are moved into the library when opened.
  In the page: undo/redo, a dpi warning per photo (checkPrint), print
  shop sizes and 3 mm bleed, page overview and spreads, cropping, text
  styles, frames, map pages (Leaflet, OSM tiles from the internet) and
  calendars (`calendar=YEAR`: a page a month, ISO weeks, Danish days off).
  "Choices…" (book.filters) leave out blurry photos, faces cut by the
  edge (`/api/books/info` face list), people only small far off,
  screenshots, near copies (within 2 min and perceptual hash within 10
  bits) and, if asked, dark ones (thumbnail brightness, `media.Looks`).
  Automatic straightening was tried (edge directions) and dropped: it
  was wrong on a third of real photos; photos are turned by hand (✂).
  Also: "Fill the boxes" crops around the faces (`/api/books/info` sends
  face boxes), ⇄ swaps a photo for one taken near the same time, "Better
  light" lifts dark photos with a CSS filter (files untouched), chapters
  by events (12 h gaps, merged to about one per six photos, titled by
  town and dates) and panoramas over two pages. Picking the best also
  weighs open eyes and balances how often each person is in it.
  He prints at Lasertryk.dk (standard books, from 50 copies; FotoSjov.dk
  is their own-editor photo-book shop, no PDF upload): "For
  Lasertryk.dk…" sets 3 mm bleed, their formats, spine width and cover
  edge, and `server/bookpdf.go` prints `book.html?pdf=inner|cover` to
  "<title> - indmad.pdf" / "- omslag.pdf" with headless Edge (its own
  throw-away profile next to the files), default folder `E:\Fotobøger`.
  "Check the book" lists what may go wrong in print. Also: "Find
  photos…" searches the library and drags photos in (`book.extra`),
  own captions (`book.captions`), cover layouts (photo, full, collage),
  page layouts (grid, one big, scattered prints), page backgrounds
  (`pg.bg`), page numbers and a contents page (both choices), and
  "Copy" on the Photo books page. The bar is one line. One floating
  toolbox holds everything else in parts opened and closed by their
  headings (`TB_PARTS`): the page clicked (shown big), book settings,
  which photos, find photos, check the book, Lasertryk; the old panels
  are moved into it and `panel()` opens a part. The cover is edited like
  a page (`book.coverPage`: design, background, text boxes, stickers;
  its photo any of the source, cropped under key "c"+id; title typed on
  it); ⇄ can swap a photo for any photo of the source (bigger with a
  slider, shown big under the mouse). One click picks a photo: drag in it
  to move it in its box, eight handles size the box (a grid page becomes
  free as it lies, `gridHandles` + `resume`), zoom and turn are sliders
  in the toolbox (`photoRow`); Escape or a click beside it lets go.
  "Photo boxes" offers ready-made box layouts (`BOX_LAYOUTS`, 1 to 6
  photos; the page becomes free and its photos fill their boxes,
  `pg.fillBoxes`); "+ Photo box" adds the next photo in a box of its
  own. The cover photo can be placed freely too (`book.coverLayout`
  "free", `book.coverPos`). Each photo's caption can be turned
  off (T on the photo, or "Caption" in the toolbox; `book.noCap`), and
  its date, place and who shown or not apart from the book (`book.capOwn`). Texts: a font
  of 17, a size in points (`style.pt`, typed or − +), bold, italic, a
  colour of the row or any, and alignment. "Unused photos" opens a floating box
  (moved, folded and sized like the toolbox) with the source's photos not in the book, of the year of the
  page picked (or one chosen), with a size slider; they are dragged onto
  a photo or a page, or put in by date (＋).
  A dark frame all round a photo (a scan's edge; `media.Frame`, sent by
  `/api/books/info`) is cut off with object-view-box. "Optimise the book"
  (toolbox part) lists look-alike photos filling pages of their own, or
  the same photo twice: keep the best, together on one page, or keep
  (`book.optKeep`); the best-of choice also drops the same photo twice.
- Cleanup (sidebar "Cleanup", `store/cleanup.go`, `server/cleanup.go`,
  `/api/cleanup/{receipts|papers|shots|blurry|copies}`): receipts and
  papers by meaning (mean likeness to a few texts minus likeness to
  screenshots, people and places, at least `cleanupLeast` = 0.05; photos
  with a face of 5% width or more and screenshots left out), screenshots (PNG
  with no camera, or named so), blurry photos, and near copies (within
  60 s and perceptual hash within 10 bits; a favourite or rated one, else
  the sharpest, is kept). The page only lists; the photos chosen go
  through `/api/images/delete` (moved to the deleted-photos folder) after
  a confirm. Favourites and rated photos are never chosen by themselves.
- Desktop shortcut `ImageOrganizer.lnk` → the Start .bat, which only opens
  the browser if the program already runs. The program listens from its first
  moment and, until the library is open, every page is a start-up page
  (`startup.go`) with a bar of what it is doing (`store.LoadProgress` steps,
  then face models); the .bat opens the browser 2 s after starting it.
- Look "Nordic" (the default since 2026-10-22, after he asked for something
  like Bang & Olufsen's style; no brand marks): light warm off-white and
  aluminium greys, black main buttons, bronze touches, thin type, small
  spaced capitals for labels, nearly square corners, hairlines; a block at
  the end of `web/app.css`, a few lines in `book.html`, and the start-up page.
- Looks: the macOS-style block in `web/app.css`, and after it the
  "Studio" look (dark and soft, glowing colour behind, frosted glass, no
  hairlines, words in normal case, headings in the Sitka serif, rounded
  photos with air between, lime main buttons; made lighter on 2026-10-16
  after he found the first version too square), the default from 2026-10-16 to 10-22;
  `<html data-look="studio"|"classic">`, chosen in Settings → Look and
  kept in localStorage.
- Danish UI: `web/i18n.js` translates the page as it is drawn (exact strings
  plus patterns); English stays the source text. Add a Danish entry there
  for every new user-facing string. Language is chosen in Settings.

## Working rules

- Go 1.27.1 (`go.mod`). `go vet ./...` and `go test ./...` before commits;
  also `CGO_ENABLED=0 GOOS=windows go vet ./...`.
  `TestDrivePathRoundTrip` fails on Linux (expects Windows paths) and passes
  on Windows; it is not ours to fix.
- The library folders are only read, except by Delete, which moves files
  to the folder the user chose. Don't add anything that changes or erases
  his photos without asking him first.
- Ask before restarting the program if he may be using it (e.g. pressing GO
  in Review): a restart cuts off what's running.
- After a change, rebuild with `build-windows.bat`, restart, and check it in
  the browser before telling him it works.
