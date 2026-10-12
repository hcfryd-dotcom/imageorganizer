# ImageOrganizer

A self-hosted photo library: point it at folders on your disk or your NAS and
browse, search and organise your photos in the browser. Everything runs on
your own machine; your photos never leave it, and your folders are only read,
never changed.

## Features

- **Folders anywhere**: local folders and SMB shares (NAS), read-only. Moved
  and renamed photos keep their ratings, notes and people.
- **Browse** as a justified photo grid by month, a table, or a map of
  geotagged photos.
- **Search by meaning**: "kids on the beach", "birthday cake", "snow". Years
  ("2015", "2000-2010") narrow the date range. Uses a multimodal embedding
  model, so text and photos are compared directly.
- **People**:
  - faces are detected and grouped, with ages estimated;
  - look-alike siblings are kept apart by their implied birth year;
  - name a group once, and confident matches are filed automatically and
    wait in Review for confirmation;
  - background faces (audiences, passers-by) and non-faces are kept out of
    the way.
- **Search for people**: type a name and pick it from the list ("Anna in the
  garden 2010"), or use `$unknown`, `$known` (`$confirmed` or `$unconfirmed`), `$alone`, and `one(…)` / `many(…)` of a face filter: `one($known) one($unknown)`.
- **Fixes for camera mistakes**: photos saved sideways are detected from the
  faces, or turned by hand; the correction is stored, the file is untouched.

## Requirements

- **Go 1.27** or newer to build.
- **Optional, for search by meaning:** a
  [llama.cpp](https://github.com/ggml-org/llama.cpp) server running a
  multimodal embedding model with its projector, for example EmbeddingGemma 2:

  ```sh
  llama-server -m embeddinggemma-2.gguf --mmproj mmproj.gguf --embeddings --port 8095
  ```

- **Optional, for people:** [ONNX Runtime](https://onnxruntime.ai) (a GPU build
  lets faces run on the graphics card) and three models in the models folder
  (default `~/.local/share/imageorganizer/models`): `scrfd_10g.onnx` (face
  detection), `arcface_w600k_r50.onnx` (recognition) and `mivolo_v2_age.onnx`
  (age). With `mivolo_v2_age_gender.onnx` instead (the same MiVOLO v2 network
  with its head output, `head`, exposed as well), adult faces' gender is
  estimated too; it is preferred when present.

Without the optional parts the app still works: browsing, the map and
filename search, just without search by meaning or people.

## Running

```sh
go build -o imageorganizer .
./imageorganizer
```

Then open <http://127.0.0.1:8420> and add a folder. Most options can be changed
on the **Settings** page (the gear at the bottom of the sidebar). Command-line
flags take priority; `./imageorganizer -h` lists them.

The library (database, thumbnails, face crops) lives in
`~/.local/share/imageorganizer`. The database holds the passwords of network
folders, so the directory is created private to your user.

**There is no login.** By default the server only listens on this machine
(`-listen 127.0.0.1:8420`). Only use `-listen 0.0.0.0:8420` on a network you
trust.

## Development

`scripts/dev.sh` keeps the latest build running: it rebuilds when Go files
change (keeping the old server if the build fails), serves the UI straight
from `web/`, and browsers reload or hot-swap CSS by themselves.

```sh
go test ./...
```

## Licence

MIT; see [LICENSE](LICENSE). Bundled third-party libraries are listed in
[web/vendor/THIRD_PARTY_LICENSES.md](web/vendor/THIRD_PARTY_LICENSES.md).
Place names (for copying photos into folders by place) come from
[GeoNames](https://www.geonames.org) via
[cities.json](https://github.com/lutangar/cities.json), CC BY 4.0; see
[internal/places](internal/places/LICENSE-GeoNames-CC-BY-4.0.txt).
