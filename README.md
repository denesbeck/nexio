# Nexio

[![CI](https://github.com/denesbeck/nexio/actions/workflows/main.yml/badge.svg)](https://github.com/denesbeck/nexio/actions/workflows/main.yml)
[![Go Version](https://img.shields.io/badge/Go-1.24.4-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A lightweight version control system inspired by Git, built from scratch in Go. Nexio implements core version control concepts including staging, commits, branching, and history tracking.

![nexio_showcase_s](https://github.com/user-attachments/assets/dc2aea8f-f95d-45c4-aaf4-cae55433471f)

## Overview

Nexio is an educational project that demonstrates the fundamental principles behind modern version control systems. It provides a simplified implementation of Git-like functionality, making it easier to understand how version control works under the hood.

**Key Features:**
- Stage and commit file changes
- Branch management (create, switch, delete)
- Commit history tracking
- File status monitoring
- User configuration management
- Isolated testing environment

## Prerequisites

- Go 1.24.4 or higher
- Unix-like environment (Linux, macOS) or Windows with Git Bash

## Installation

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

# Add files to staging area
./nexio add file1.txt file2.txt

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

## Available Commands

| Command    | Description                                                       |
|------------|-------------------------------------------------------------------|
| `init`     | Initialize the Nexio version control system                    |
| `add`      | Add files to the staging area                                     |
| `remove`   | Remove files from the staging area                                |
| `commit`   | Commit staged changes with a message                              |
| `status`   | Display staged, tracked, and untracked files                      |
| `history`  | List all commits for the current branch                           |
| `branch`   | Manage branches (new, drop, switch, default, current)             |
| `workdir`  | List files in the current working directory state                 |
| `config`   | Get or set configuration values (username, email, default-branch) |
| `clean`    | Clean orphaned blobs from the object store                        |
| `purge`    | Remove Nexio and all its data (irreversible)                   |

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
├── scripts/            # Build and test scripts
├── .github/workflows/  # CI/CD configuration
└── go.mod              # Go module dependencies
```

## Built With

- [Go](https://go.dev/) - Programming language
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [pterm](https://github.com/pterm/pterm) - Terminal output styling
- [fatih/color](https://github.com/fatih/color) - Color output

## How It Works

Nexio stores version control data in a `.nexio` directory at the root of your project:

- **Staging area**: Tracks files prepared for commit
- **Commits**: Stores snapshots of file states with metadata
- **Branches**: Maintains separate lines of development
- **Configuration**: Stores user settings and repository configuration

Unlike Git, Nexio uses a simpler file-based storage system and YAML for metadata, making the internals easier to understand and inspect.

## Limitations

Nexio is designed for educational purposes and lacks several features found in production version control systems:

- No remote repository support
- No merge conflict resolution
- No diff visualization
- No file compression or delta storage
- Limited to local repositories

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
