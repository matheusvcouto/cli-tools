# Changelog — media-get

## [Unreleased]

## [0.2.1] - 2026-10-02

### Corrigido

- **media-get:** Improve interactive menus with search and compact selected answers, late Referer and bounded fragment concurrency; fix decimal HLS totals, carry selected forecasts into immediate animated bars, account for video/audio streams without duplicate bytes, preserve progress on interruption, avoid redundant quality queries, validate coherent FFmpeg/ffprobe and retain automation flags; discard only current incomplete files by default on failure/cancellation, offer explicit retention in a visible private folder with size and identity-checked cleanup; prefetch estimates alongside initial metadata with bounded workers and in-memory session cache, update sizes in live menus without blocking selection and cancel all probes before download; edit native text fields with cursor arrows and deletion, restore terminal before canceled prompts return and show readable cancellation outcomes while retaining exit 130

## [0.2.0] - 2026-10-01

### Adicionado

- **media-get:** Introduce experimental media-get with declarative CLI Core, optional Referer, video/audio/SRT selections, per-operation system dependency checks, concurrent cached transfer-size previews before selection, loading counters and staged progress, cancellable Unix process groups and confined no-clobber publication with partial-download recovery

- Novo downloader experimental com fluxo interativo, Referer opcional,
  dependências do sistema, estimativa e publicação sem substituir arquivos.

## [0.3.0] - 2026-10-02

### Adicionado

- **media-get:** Use bounded parallel fragment downloads by default, allow 1–256 fragments through validated flags and an environment preference, show an editable final download summary, respect explicit names, preserve exact transfer totals and unify preview estimates.

## [0.3.1] - 2026-10-02

### Corrigido

- **media-get:** Fix missing plain TXT subtitle output and offer validated compatible MP4 output, including explicit filename extensions.

## [0.4.0] - 2026-10-08

### Adicionado

- **media-get:** Import versioned and discovery JSON manifests, validate them offline, review each output and safely download a bounded concurrent batch with typed Origin headers, cancellation and partial-failure handling. Show output format and extension in every download summary, and safely create missing destination directories only when the confirmed download begins. Expose direct output-format editing and a batch-wide MP4 choice; show metadata loading counts and a concurrent progress panel with reusable active slots.
