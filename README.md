# jsonlv — JSON Log Viewer

A native macOS desktop app for viewing and filtering structured JSON log streams in real time.

## Features

- **Live streaming** — tail one or more files (`-f`) or pipe stdin
- **Auto-detected log formats** — Laravel/Monolog, pino (numeric levels), Datadog, and generic JSON
- **Level filter buttons** — ALL / INFO / WARN / ERROR / CRITICAL / DEBUG with live counts
- **Property filters** — right-click any JSON key in an expanded entry → "Filter hinzufügen"; filter bar appears with per-value counts and AND/OR semantics
- **Custom columns** — right-click any key → "Spalte hinzufügen/entfernen"
- **Copy from context menu** — "Wert kopieren" (raw value) or "Als JSON kopieren" (formatted JSON)
- **Full-text search** — Cmd+F; matching entries auto-expand their details panel; live search applies to incoming entries too
- **File-path linking** — paths like `/var/www/html/…:265` become clickable links that open in PhpStorm; path-mapping dialog for remote→local resolution (persisted)
- **Font scaling** — Cmd+= / Cmd+−
- **Recent files** — native File menu with "Zuletzt geöffnet" submenu (persisted)
- **Light / dark theme**

## Installation

```bash
make install   # builds jsonlv.app → /Applications, shell wrapper → /usr/local/bin/jsonlv
```

Requires macOS and Xcode Command Line Tools (`xcode-select --install`).

## Usage

```bash
# Pipe stdin
./loggen.sh | jsonlv

# Tail a file (last 1000 lines, then follow)
jsonlv -f /var/log/app.log

# Multiple files
jsonlv -f app.log worker.log

# Custom line count
jsonlv -n 500 -f app.log

# Open from Finder — double-click jsonlv.app
# Then use File → Öffnen… (Cmd+O) to choose files
```

## Build

```bash
make build   # produces ./jsonlv binary
make app     # produces ./jsonlv.app bundle
make install # installs bundle + CLI wrapper
make clean   # removes build artefacts
```

## Supported log formats

| Field | Keys tried (in order) |
|---|---|
| Level (string) | `level_name`, `dd_status`, `level` — normalises WARNING→WARN, FATAL→CRITICAL |
| Level (pino numeric) | 60→CRITICAL, 50→ERROR, 40→WARN, 30→INFO, ≤20→DEBUG |
| Message | `message`, `msg`, `error` |
| Service | `channel`, `service`, `logger`, `dd.service` |
| Timestamp | `datetime`, `timestamp`, `time`, `@timestamp` |
| Duration | `duration_ms` |
| Status | `status_code` |

Non-JSON lines are displayed as plain text.

## Path mapping (PhpStorm)

When a log line contains a file path that doesn't exist locally (e.g. a Docker container path), clicking it opens a file-picker dialog. The chosen local file is matched by common suffix to derive a prefix mapping that applies to all future paths automatically. Mappings are stored in `~/.config/jsonlv/mappings.json`.
