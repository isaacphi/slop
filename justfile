# Help
help:
  just -l

# Build slop
build:
  go build -o slop

# Install slop locally
install:
  go install

# Generate schema
generate:
  go generate ./...

# Run code audit checks
check:
  go tool staticcheck ./...
  go tool govulncheck ./...
