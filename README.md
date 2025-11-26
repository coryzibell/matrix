# Matrix

A unified CLI for the Claude Code identity system.

## Status

**v0.0.1** - Initial repository structure created.

## Architecture

See the architecture documentation at `~/.claude/ram/architect/matrix-go-architecture.md` for the full system design.

## Structure

```
matrix/
├── cmd/
│   └── matrix/
│       └── main.go          # Entry point
├── internal/
│   ├── identity/
│   ├── ram/
│   ├── output/
│   └── platform/
├── go.mod
└── README.md
```

## Development

```bash
# Run without installing
go run ./cmd/matrix

# Build for current platform
go build -o matrix ./cmd/matrix

# Install to GOPATH/bin
go install ./cmd/matrix
```

## License

MIT
