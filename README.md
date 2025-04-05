# Modrinth Mod Updater

A tool to automatically download and update Minecraft mods from the Modrinth platform.

## Features

- Fetch followed projects from Modrinth
- Filter by Minecraft version and loader compatibility
- Automatically download compatible mods
- SQLite database tracking of installed mods
- Version comparison to identify and download updates
- Option to archive old versions instead of deleting them

## Configuration

Configuration is managed via environment variables or a `.env` file in the project root. Create a `.env` file by copying `.env.example`:

```bash
cp .env.example .env
```

Then, edit `.env` with your settings.

| Environment Variable          | Description                                                                                                                                                                                             | Default Value |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------- |
| `MODRINTH_API_KEY`            | **Required.** Your personal Modrinth API key. Needed to fetch your followed projects. Obtain from [Modrinth Settings](https://modrinth.com/settings/account).                                          | *None*        |
| `MINECRAFT_VERSION`           | **Required.** The target Minecraft version (e.g., `1.20.1`).                                                                                                                                           | *None*        |
| `MINECRAFT_DIR`               | **Required.** The path to your Minecraft instance directory (e.g., `/home/user/.minecraft` or `./my_instance`). The updater will create `mods`, `shaderpacks`, and `resourcepacks` subdirectories here if needed. The database file (`modrinth-updater.db`) is also stored here. | *None*        |
| `MINECRAFT_LOADER`            | The mod loader to check compatibility against (e.g., `fabric`, `forge`, `neoforge`, `quilt`). Only applies to projects of type `mod`.                                                                                             | `fabric`      |
| `MINECRAFT_INSTALLATION_TYPE` | Filters projects based on side compatibility. Use `client` or `server`.                                                                                                                                     | `client`      |
| `KEEP_OLD_VERSIONS`           | If `true`, keeps old files in a `versions` subdirectory within the respective `mods`, `shaderpacks`, or `resourcepacks` folder.                                                                               | `false`       |
| `USERAGENT`                   | Custom User-Agent string for Modrinth API requests. Recommended to include contact info (e.g., `MyApp/1.0 (contact@example.com)`).                                                               | See code      |
| `LOG_LEVEL`                   | Set logging verbosity (`debug`, `info`, `warn`, `error`).                                                                                                                                              | `info`        |
| `LOG_FORMAT`                  | Set logging output format (`text` or `json`).                                                                                                                                                          | `text`        |

## Database

The tool uses an SQLite database to track installed mods. For each mod, it stores:

- Project slug (unique identifier from Modrinth)
- Version ID (current installed version)
- Filename
- Installation path

When updates are found, the tool will:
1. Check if the mod exists in the database
2. Compare the current installed version with the latest available version
3. Handle the old file (delete or archive based on `KEEP_OLD_VERSIONS` setting)
4. Download the new version
5. Update the database with the new information

## Usage

### Update

```
go run . update
```

This will:
1. Fetch all projects you follow on Modrinth
2. Check for compatible versions with your Minecraft version and loader
3. Download new mods and update existing ones as needed
4. Track everything in the SQLite database
5. Store version history for rollbacks

Flags:
- `--force` or `-f`: Force redownload of all mods regardless of current version

### Rollback

```
go run . rollback [projectSlug]
```

This will:
1. Find the most recent previous version of the mod in the database
2. Remove the current version from your mods directory
3. Restore the previous version from the archive (if available)
4. Update the database to reflect the rollback

For example:
```
go run . rollback sodium
```

## Old Version Archiving

When `KEEP_OLD_VERSIONS=true`, old mod files will be moved to the `mods/versions` directory instead of being deleted when updates are found. If a file with the same name already exists in the versions directory, the tool will add a suffix with the version ID to ensure uniqueness.
