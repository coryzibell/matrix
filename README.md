# Matrix

Intelligence tools for the Claude Code identity system.

Matrix is a CLI that analyzes, tracks, and surfaces patterns across identity work stored in `~/.claude/ram/`. Each identity in the system writes to their own directory, and Matrix provides 24 specialized commands to extract insights from that knowledge garden.

## Installation

```bash
# Build and install to GOPATH/bin
go install github.com/coryzibell/matrix/cmd/matrix@latest

# Or build locally
cd /home/w3surf/work/personal/code/matrix
go build -o matrix ./cmd/matrix
```

## Quick Start

```bash
# Discover connections between identity work
matrix garden-paths

# Track task completion velocity
matrix velocity

# Scan a codebase and generate intelligence report
matrix recon /path/to/project

# See all commands
matrix --help
```

## Commands

Matrix provides 24 commands organized by the identity that owns each capability:

### Morpheus - Discovery & Learning
- **garden-paths** - Discover connections between identities in RAM
- **garden-seeds** - Create well-structured RAM files from templates
- **knowledge-gaps** - Find unanswered questions and missing documentation

### Tank - Research & Context
- **recon** - Scan codebases and generate intelligence reports
- **data-harvest** - Scan RAM for data patterns to build better fixtures

### Trinity - Debugging & Crisis
- **incident-trace** - Extract structured post-mortem data from debugging sessions
- **tension-map** - Surface conflicts and tensions across RAM

### Seraph - Strategy & Planning
- **crossroads** - Capture decision points and paths not taken
- **balance-checker** - Detect drift between design docs and implementation

### Oracle - Exploration & Alternatives
- **alt-routes** - Accessibility audit and alternative output formats
- **question** - Surface hidden assumptions behind documented work

### Cypher - Security
- **breach-points** - Audit for security vulnerabilities and exposures
- **vault-keys** - Map authentication, authorization, and security boundaries

### Keymaker - Authentication & Keys
- **vault-keys** - Map authentication, authorization, and security boundaries

### Niobe - DevOps & Deployment
- **flight-check** - Track deployment state across identity work
- **platform-map** - Scan for cross-platform compatibility markers

### Merovingian - APIs & Contracts
- **contract-ledger** - Track data flows and dependencies between identities

### Librarian - Database & Schemas
- **schema-catalog** - Track database schemas across projects

### Twins - Cross-Language
- **phase-shift** - Track cross-language compatibility and migration patterns
- **diff-paths** - Compare two implementations and extract architectural tradeoffs

### Deus Ex Machina - Testing
- **verdict** - Track test results and performance metrics

### Rama-Kandra - Refactoring
- **debt-ledger** - Track technical debt markers and generate remediation tasks

### Persephone - UX
- **friction-points** - Track UX review queue and feedback

### Commander Lock - Compliance
- **spec-verify** - Verify implementations against formal specifications

### Apoc - Dependencies
- **dependency-map** - Map installed toolchains and package dependencies

### The Kid - Execution
- **velocity** - Track task completion velocity by identity

## Usage Examples

### Analyze your garden
```bash
# See all cross-references between identities
matrix garden-paths

# Track which identities are shipping work
matrix velocity --days 7

# Find gaps in documentation
matrix knowledge-gaps
```

### Scan a project
```bash
# Full reconnaissance scan
matrix recon /path/to/project

# Quick overview
matrix recon --quick .

# Focus on security
matrix recon --focus security
```

### Track velocity
```bash
# All-time velocity across all identities
matrix velocity

# Filter by specific identity
matrix velocity --identity smith

# JSON output for tooling
matrix velocity --json
```

## Architecture

Matrix is built with zero external dependencies, stdlib only:

```
matrix/
├── cmd/matrix/           # 24 command implementations + main
├── internal/
│   ├── identity/        # Identity validation and RAM path resolution
│   ├── ram/             # Markdown file scanning across ~/.claude/ram
│   └── output/          # ANSI color output utilities
└── go.mod
```

All data is stored as flat markdown files in `~/.claude/ram/{identity}/`. Matrix reads these files to extract patterns, connections, and insights.

## Development

```bash
# Run without installing
go run ./cmd/matrix garden-paths

# Build for current platform
go build -o matrix ./cmd/matrix

# Run tests
go test ./...
```

## Philosophy

The garden grows through collaboration. Matrix helps you see the connections.

Every identity writes to RAM. Matrix reads RAM and shows you patterns you might have missed: which identities collaborate most, where knowledge gaps exist, what decisions were made, where tensions surface.

Matrix is a lens, not a source of truth. The truth lives in `~/.claude/ram/`. Matrix just helps you see it.

## Status

**v0.0.1** - Core commands implemented, zero external dependencies.

## License

MIT
