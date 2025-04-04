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

Create a `.env` file in the project directory with the following variables:

```
MINECRAFT_INSTALLATION=server_side
MINECRAFT_LOADER=fabric
MINECRAFT_VERSION=1.21.5
MODRINTH_API_KEY=your_token_here
USERAGENT=your-user/modrinth-mod-updater (your-email@example.com)
MODRINTH_USER=your-username

# Database path (optional, defaults to mods.db in the current directory)
DATABASE_PATH=mods.db

# Keep old mod versions when updating (optional, defaults to false)
# Set to true to move old versions to mods/versions instead of deleting
KEEP_OLD_VERSIONS=false
```

### Configuration Options

| Variable | Description | Default |
|----------|-------------|---------|
| `MINECRAFT_INSTALLATION` | Type of Minecraft installation (e.g., `server_side`, `client`) | None (required) |
| `MINECRAFT_LOADER` | Mod loader (e.g., `fabric`, `forge`, `quilt`) | None (required) |
| `MINECRAFT_VERSION` | Minecraft version (e.g., `1.21.5`) | None (required) |
| `MODRINTH_API_KEY` | Your Modrinth API key | None (required) |
| `USERAGENT` | User agent for API requests | Default generic agent |
| `MODRINTH_USER` | Your Modrinth username | None (required) |
| `DATABASE_PATH` | Path to SQLite database file | `mods.db` in the current directory |
| `KEEP_OLD_VERSIONS` | Whether to keep old versions of mods when updating | `false` |

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
