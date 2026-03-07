# Nexio

[![CI](https://github.com/denesbeck/nexio/actions/workflows/main.yml/badge.svg)](https://github.com/denesbeck/nexio/actions/workflows/main.yml)
[![Go Version](https://img.shields.io/badge/Go-1.24.4-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A lightweight version control system inspired by Git, built from scratch in Go. Nexio implements core version control concepts including staging, commits, branching, history tracking, and remote synchronization via AWS S3.

![nexio_showcase_s](https://github.com/user-attachments/assets/dc2aea8f-f95d-45c4-aaf4-cae55433471f)

## Overview

Nexio is an educational project that demonstrates the fundamental principles behind modern version control systems. It provides a simplified implementation of Git-like functionality, making it easier to understand how version control works under the hood.

**Key Features:**
- Stage and commit file changes
- Branch management (create, switch, delete)
- Commit history tracking
- File status monitoring
- Remote synchronization via AWS S3 (push, pull, clone)
- User configuration management
- Isolated testing environment

## Prerequisites

- Go 1.24.4 or higher
- Unix-like environment (Linux, macOS) or Windows with Git Bash
- AWS credentials configured (for remote operations) — see [Remote Sync](#remote-sync-s3)

## Installation

### Homebrew (recommended)

```bash
brew tap denesbeck/nexio
brew install nexio
```

### Build from source

1. Clone the repository:

```bash
git clone https://github.com/denesbeck/nexio.git
cd nexio
```

2. Install dependencies:

```bash
go mod download
```

3. Build the binary:

```bash
go build -o nexio ./cmd/nexio
```

4. (Optional) Add to PATH:

```bash
# Add to your shell profile (.bashrc, .zshrc, etc.)
export PATH="$PATH:/path/to/nexio"
```

## Usage

### Initialize a Repository

```bash
./nexio init
```

### Configure User Settings

```bash
./nexio config set username "Your Name"
./nexio config set email "your.email@example.com"
./nexio config set default-branch "main"
```

### Basic Workflow

```bash
# Check file status
./nexio status

# Stage files for commit
./nexio stage file1.txt file2.txt

# Commit changes
./nexio commit -m "Initial commit"

# View commit history
./nexio history
```

### Branch Management

```bash
# Create and switch to new branch
./nexio branch new feature-branch

# List branches
./nexio branch current

# Switch branches
./nexio branch switch main

# Delete a branch
./nexio branch drop feature-branch
```

### Remote Sync (S3)

Nexio supports remote synchronization via AWS S3, enabling collaboration and backup across machines.

**Setup:**

```bash
# Configure your remote
./nexio config set remote s3://my-bucket/nexio-repo
```

AWS credentials are resolved via the standard chain (`~/.aws/credentials`, environment variables, IAM roles). Nexio does not manage credentials.

**Push, Pull, Clone:**

```bash
# Push local commits to remote
./nexio push

# Pull remote commits into local
./nexio pull

# Clone a remote repository
./nexio clone s3://my-bucket/nexio-repo
./nexio clone s3://my-bucket/nexio-repo ./local-dir
```

**Multi-machine workflow:**

```bash
# Machine A
./nexio stage .
./nexio commit -m "Add feature"
./nexio push

# Machine B
./nexio pull    # or: nexio clone s3://my-bucket/nexio-repo
```

All commands accept a `--remote` flag to override the configured remote for a single operation, and a `--force` flag to override the remote lock.

## Available Commands

| Command    | Description                                                                |
|------------|----------------------------------------------------------------------------|
| `init`     | Initialize the Nexio version control system                                |
| `stage`    | Stage files for commit                                                     |
| `unstage`  | Unstage files from the staging area                                        |
| `commit`   | Commit staged changes with a message                                       |
| `status`   | Display staged, tracked, and untracked files                               |
| `history`  | List all commits for the current branch                                    |
| `branch`   | Manage branches (new, drop, switch, default, current)                      |
| `workdir`  | List files in the current working directory state                          |
| `config`   | Get or set configuration values (username, email, default-branch, remote)  |
| `push`     | Push local commits and blobs to remote (S3)                                |
| `pull`     | Pull remote commits and blobs into local repository                        |
| `clone`    | Clone a remote repository from S3                                          |
| `clean`    | Clean orphaned blobs from the object store                                 |
| `purge`    | Remove Nexio and all its data (irreversible)                               |

For detailed command usage, run:

```bash
./nexio [command] --help
```

## Development

### Development Setup

Install Git hooks to ensure code quality:

```bash
make install-hooks
```

This installs a pre-commit hook that automatically runs:
- `go vet ./...` - Lints code for common issues
- `gofmt -l .` - Checks code formatting

If formatting issues are detected, fix them with:

```bash
gofmt -w .
```

To remove the hooks:

```bash
make uninstall-hooks
```

### Running Tests

Run the test suite using the provided script:

```bash
bash ./scripts/run-tests.sh
```

**Important:** Always use `run-tests.sh` instead of `go test` directly. The script sets the `NEXIO_ENV=test` environment variable, which ensures tests run in an isolated namespace to prevent conflicts with your actual `.nexio` directory.

### Project Structure

```
nexio/
├── cmd/nexio/          # CLI application and commands
│   ├── db.go           # SQLite connection, schema, transactions
│   ├── db_staging.go   # Staging area CRUD operations
│   ├── db_commits.go   # Commit CRUD, metadata, logs
│   ├── db_branches.go  # Branch CRUD operations
│   ├── db_files.go     # File list operations, referenced hash collection
│   ├── remote.go       # S3 client, URL parsing, upload/download helpers
│   ├── remote_lock.go  # Remote lock acquire/release
│   ├── push.go         # Push command
│   ├── pull.go         # Pull command, DB merge, working directory sync
│   ├── clone.go        # Clone command
│   └── ...             # Commands, helpers, UI, tests
├── docs/               # Documentation
├── scripts/            # Build and test scripts
├── .github/workflows/  # CI/CD configuration
└── go.mod              # Go module dependencies
```

## Built With

- [Go](https://go.dev/) - Programming language
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [pterm](https://github.com/pterm/pterm) - Terminal output styling
- [fatih/color](https://github.com/fatih/color) - Color output
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) - Pure Go SQLite driver (no CGO)
- [aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2) - AWS SDK for S3 remote operations

## How It Works

Nexio stores version control data in a `.nexio` directory at the root of your project:

```
.nexio/
├── index.db       # SQLite database (metadata, staging, commits, branches)
├── objects/       # Content-addressable blob storage
│   ├── ab/
│   │   └── cdef1234...   # Compressed file content, keyed by BLAKE3 hash
│   └── ...
└── config.json    # User configuration (name, email, remote)
```

- **`index.db`**: A single SQLite database that stores all metadata -- staging area, commits, branches, file lists, and commit logs. Using SQLite provides atomic operations, indexed lookups, and significantly better performance for large repositories compared to the previous JSON file-based approach.
- **`objects/`**: Content-addressable blob storage. Files are hashed (BLAKE3) and stored in a two-level directory structure (first two hex chars as shard directory). Identical file contents are automatically deduplicated.
- **`config.json`**: Stores user settings (name, email) and remote URL.

Commits are shared across branches -- creating a branch simply points the new branch's head to an existing commit. Branch membership is determined by walking the parent chain from each branch's head commit.

For remote sync, the same structure is mirrored in an S3 bucket under a configurable prefix. Push uploads the local `index.db` and any missing blobs. Pull downloads the remote database and merges it locally using SQLite's `ATTACH DATABASE`. Clone downloads everything and restores the working directory.

## Limitations

Nexio is designed for educational purposes and lacks several features found in production version control systems:

- No merge conflict resolution (diverged histories are rejected)
- No diff visualization
- No delta storage (whole-file deduplication only)
- Remote support limited to AWS S3 (no SSH/HTTP remotes)

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Inspired by Git's architecture and design principles
- Built as a learning project to understand version control internals
