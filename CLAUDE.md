# ImageOrganizer: notes for Claude Code

A self-hosted photo library in Go: browse, search and organise photos in the
browser; faces are found, grouped and named. See README.md for the feature
list. This fork (hcfryd-dotcom/imageorganizer) is used on Windows by a
Danish-speaking owner (Henrik). **Talk to him in Danish**, in plain words; he
is not a programmer. Code, comments and commit messages stay in English, like
the rest of the repo.

## The owner's setup (Windows 11)

- Source: a copy of this repo in `C:\Users\heinz\Downloads\imageorganizer`
  (copied over file by file, not a git clone yet). If you work in a fresh
  clone, use the `windows-build-without-cgo` branch (pull request #1), or
  `main` once that is merged.
- Build: `build-windows.bat` (Go 1.27 + MinGW gcc via WinLibs, cgo on), which
  writes `dist\ImageOrganizer\imageorganizer.exe` and
  `dist\ImageOrganizer-windows-x64.zip`. It keeps `dist\ImageOrganizer`, which
  also holds `onnxruntime.dll` (ONNX Runtime **1.29.1**, matching
  onnxruntime_go v1.36.0) and `llama\` (llama-server for search by meaning).
- Run: `dist\ImageOrganizer\Start ImageOrganizer.bat` opens
  http://127.0.0.1:8420. The server runs while its console window is open.
  To rebuild, close that window first (the .exe is locked while running).
- Library data: `C:\Users\heinz\.local\share\imageorganizer` (library.bolt,
  thumbnails, face crops, `models\`). **Never delete or reset it**: it holds
  his naming of ~23,000 faces.
- Face models in `models\`: `scrfd_10g.onnx` and `arcface_w600k_r50.onnx`
  (InsightFace buffalo_l, non-commercial licence). No age model
  (`mivolo_v2_age.onnx`), so ages and genders are unknown.
- Photos: `E:\Onedrive\Billeder` (~23,000; OneDrive, so files may be
  cloud-only until read).
- Search by meaning is off: llama-server is in `dist\ImageOrganizer\llama\`,
  but the EmbeddingGemma 2 model files (`embeddinggemma-2-Q8_0.gguf` and
  `mmproj-Q8_0.gguf` from unsloth/embeddinggemma-2-GGUF on Hugging Face) have
  not been put in `llama\models\` yet. Untested on Windows.

## What this fork adds (branch windows-build-without-cgo, PR #1)

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
