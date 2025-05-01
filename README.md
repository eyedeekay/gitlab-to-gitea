# gitlab-to-gitea

Go-based tool for migrating GitLab repositories, users, groups, issues and related data to Gitea instances.

More-or-less a port of [gitlab-to-gitea](https://git.autonomic.zone/kawaiipunk/gitlab-to-gitea) from python to Go because *fixing* python appears to be a thing I just can't get my mind around, but *rewriting* it? I'm actually OK at that.

Also includes:
 - `cmd/forkfix`, for fixing fork relationships between migrated repositories by manipulating the gitea sql database
 - `cmd/unmigrate` to delete everything from a gitea instance except for the admin users
 - `cmd/mirror` to set up a mirror from github to gitea
 - `cmd/orgfix` to establish an organization's repositories as the "parent fork" by manipulating the gitea sql database
 - `cmd/namefix` to discover repositories that have identical initial commit hashes but different names
 - `cmd/johnconnor` which is a super-dangerous script for eliminating spam accounts from gitlab instances.

## Core Functionality

- Migrates users, groups, and their relationships from GitLab to Gitea
- Transfers repositories with labels, milestones, issues, and comments
- Preserves user relationships (collaborators) and SSH keys
- Supports resumable migrations through state tracking
- Handles username normalization and entity mapping between platforms

## Go Implementation Improvements

- Modular package structure instead of monolithic script
- Configuration via environment variables rather than hardcoded values
- Added utility tools (`forkfix`, `unmigrate`, `johnconnor`, `mirror`, `orgfix`, `namefix`) 
- Database connectivity for commit action imports
- Improved error handling with recovery mechanisms
- Separation of API client code from migration logic

## Installation

1. Ensure Go 1.24+ is installed
2. Clone the repository:
   ```bash
   git clone https://github.com/go-i2p/gitlab-to-gitea.git
   cd gitlab-to-gitea
   ```
3. Install dependencies:
   ```bash
   go mod download
   ```
4. Build the executable:
   ```bash
   go build -o gitlab-to-gitea ./cmd/migrate/
   ```

## Configuration

1. Copy the example environment file:
   ```bash
   cp _env.example .env
   ```
2. Edit `.env` with your GitLab and Gitea details:
   ```
   GITLAB_URL=https://your-gitlab-instance.com
   GITLAB_TOKEN=your-gitlab-token
   GITEA_URL=https://your-gitea-instance.com
   GITEA_TOKEN=your-gitea-token
   ```

## Usage

Execute the migration tool after configuration:

```bash
./gitlab-to-gitea
```

The tool will:
1. Connect to both GitLab and Gitea instances
2. Migrate users and groups first
3. Migrate projects with all associated data
4. Track progress in `migration_state.json` (resumable if interrupted)

## Key Dependencies

- github.com/xanzy/go-gitlab: GitLab API client
- github.com/joho/godotenv: Environment variable handling
- github.com/go-sql-driver/mysql: Optional database connectivity for action import
- github.com/mattn/go-sqlite3: forkfix sqlite handling

## Optional Features

For commit action import to Gitea's activity timeline:
1. Configure database details in `.env`
2. Generate a commit log file
3. Use the database import functionality in the `gitea` package

## License

MIT License