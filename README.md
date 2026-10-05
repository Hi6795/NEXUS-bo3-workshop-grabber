# NEXUS BO3 Workshop Grabber

Standalone Windows downloader for **Call of Duty: Black Ops III** Steam Workshop maps and mods.

> **BO3 does not need to be installed.**

NEXUS BO3 Workshop Grabber accepts Steam Workshop URLs or raw Published File IDs, downloads the content through Valve SteamCMD, reads public Workshop metadata, and organizes completed items into clean `Maps`, `Mods`, or `Other` folders.

**Current public baseline: v0.2 (dark UI)**

## Confirmed working

v0.2 has been hardware-tested on Windows with both a BO3 Workshop **map** and a BO3 Workshop **mod**. Both completed successfully on the first SteamCMD attempt in that test.

Validation items:

- Map: `1795684736` — Worlds Smallest Map
- Mod: `2801151115` — Reaper Collection

## Features

- Dark/black native Windows UI
- No local BO3 installation required
- No PS4 SDK required
- Completely separate from JokerZz's PS4 Porter
- Accepts Workshop URLs or numeric Workshop item IDs
- Batch input for multiple items
- Deduplicates repeated IDs
- Downloads Valve SteamCMD automatically on first use
- Uses BO3 Steam AppID `311210`
- Retrieves public Workshop title, size, and tags
- Sorts completed items into `Maps`, `Mods`, or `Other`
- Prefers `Maps` when an item has both Map and Mod tags
- Retries/resumes failed or timed-out SteamCMD transfers up to 6 times
- Skips an already-organized destination when it is non-empty
- Writes a small `.nexus.json` metadata sidecar without modifying the Workshop payload
- Cancel and Open Output controls
- Windows-safe output folder names

## Usage

1. Build or download `NEXUS_BO3_Workshop_Grabber.exe`.
2. Run it on Windows.
3. Paste one BO3 Workshop URL or numeric Workshop ID per line.
4. Choose an output folder.
5. Press **Download**.
6. Press **Open Output** when the batch is complete.

Example input:

```text
https://steamcommunity.com/sharedfiles/filedetails/?id=1795684736
2801151115
```

Typical output:

```text
NEXUS_BO3_Workshop/
├── Maps/
│   └── Worlds Smallest Map [1795684736]/
├── Mods/
│   └── Reaper Collection v1.115.1 [2801151115]/
└── Other/
```

## SteamCMD

SteamCMD is **not bundled** with the source code. On first use the app downloads Valve's official SteamCMD package from Valve's CDN and stores it under the current Windows user's cache location.

The current v0.2 downloader uses SteamCMD's anonymous Workshop path. It does not store Steam usernames or passwords and does not attempt to bypass Steam permissions. If Valve denies anonymous access to an item, the application reports the Steam error.

## Building from source

Requires Go **1.23.2+**.

Run the tests and vet:

```bash
go test ./...
go vet ./...
```

Build the Windows x64 GUI executable:

```bash
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-H=windowsgui -s -w" -o NEXUS_BO3_Workshop_Grabber.exe .
```

The checked-in v0.2 source reproduces the hardware-tested executable with SHA-256:

```text
ff8a6552cb4c94197ca01abcf3ee6c7bce36bd1bf7419abb0dd2169182f7c5e9
```

GitHub Actions runs tests/vet and produces a Windows x64 build artifact.

## Project boundaries

This repository is only the **Workshop Grabber**.

It does **not** contain, bundle, modify, or integrate:

- Call of Duty: Black Ops III
- SteamCMD binaries
- Sony/PlayStation SDK files
- JokerZz's PS4 Porter

JokerZz's porter is a separate project/tool.

## Contributing

Bug reports, testing reports, and pull requests are welcome. When reporting a download issue, please include the Workshop item ID, relevant activity-log output, and your Windows version.

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Released under the [MIT License](LICENSE).

## Disclaimer

This is an unofficial community project and is not affiliated with or endorsed by Activision, Treyarch, Valve, Steam, Sony Interactive Entertainment, or JokerZz.

Users are responsible for following the terms and permissions that apply to the content they download.
