# Embedded UI Documentation

## Overview

As of this version, the Armonite React UI is embedded directly into the Go binary using Go 1.16's `embed` package. This means:

- ✅ **Single Binary**: No need to carry external `ui-build/` folders
- ✅ **Self-Contained**: All UI assets are compiled into the binary
- ✅ **Simplified Deployment**: Just copy the binary file
- ✅ **No Runtime Dependencies**: UI works without external files

## How It Works

### Embedding Process
1. React UI is built from `ui-react/` to `ui-build/` using Vite
2. Go's `//go:embed` directive includes `ui-build/*` files into the binary at compile time
3. HTTP server serves embedded files from memory instead of filesystem

### Key Files
- `embedded_ui.go` - Contains embed directives and UI filesystem helper
- `http_server.go` - Updated to serve from embedded filesystem
- `Makefile` - Modified to build UI before Go binary automatically

## Build Process

### Automatic (Recommended)
```bash
make build          # Builds UI and binary together
make run-coord      # Builds and runs coordinator with UI
```

### Manual Steps
```bash
# 1. Build React UI
cd ui-react && npm run build

# 2. Build Go binary (embeds UI automatically)
go build -o armonite .

# 3. UI is now embedded - can delete ui-build/
rm -rf ui-build
```

## File Structure

### Before (External UI)
```
armonite                    # Binary
ui-build/                   # Required folder
├── index.html             # Main HTML
└── assets/
    ├── index-xyz.js       # React bundle
    └── index-xyz.css      # Styles
```

### After (Embedded UI)
```
armonite                    # Self-contained binary (~34MB)
# No external files needed!
```

## Development vs Production

### Development Mode
```bash
# Start API server only
make run-coord-api

# Start UI dev server (with hot reload)
make dev-ui

# UI available at http://localhost:3000 with proxy to API
```

### Production Mode
```bash
# Build and run with embedded UI
make run-coord

# UI available at http://localhost:8081
# API available at http://localhost:8080
```

## Binary Size Impact

| Component | Size |
|-----------|------|
| Go binary only | ~28MB |
| React UI assets | ~6MB |
| **Total embedded** | **~34MB** |

The React UI adds approximately 6MB to the binary size, which is acceptable for the convenience of a single-file deployment.

## Deployment Benefits

### Before (Multiple Files)
```bash
# Deploy multiple files
scp armonite server:/usr/local/bin/
scp -r ui-build/ server:/usr/local/share/armonite/

# Risk: Missing ui-build folder = broken UI
```

### After (Single File)
```bash
# Deploy single file
scp armonite server:/usr/local/bin/

# Works immediately - no missing dependencies
```

## Technical Implementation

### Embed Directive
```go
//go:embed ui-build/*
var embeddedUI embed.FS
```

### Serving Embedded Files
```go
// Get embedded filesystem
uiFS, _ := GetEmbeddedUI()

// Serve assets
assetsFS, _ := fs.Sub(uiFS, "assets")
router.StaticFS("/assets", http.FS(assetsFS))

// Serve index.html
indexHTML, _ := fs.ReadFile(uiFS, "index.html")
ctx.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
```

## Troubleshooting

### UI Shows 404 Error
```bash
# Check if UI was built before binary compilation
ls ui-build/  # Should exist during build

# Rebuild with UI
make clean
make build
```

### Binary Size Too Large
The 34MB binary size is normal with embedded UI. If size is critical:

```bash
# Build without UI for size-sensitive deployments
go build -tags noui -o armonite-slim .
```

### Development Workflow
```bash
# UI changes only - rebuild quickly
cd ui-react && npm run build && cd .. && go build

# Or use development mode with hot reload
make dev
```

## Migration Notes

### Updating from External UI Version
1. Old deployments with `ui-build/` folder still work
2. New deployments only need the binary
3. Remove external `ui-build/` folders to save space

### Docker Images
```dockerfile
# Before: Copy UI files
COPY ui-build/ /app/ui-build/

# After: Just copy binary (UI embedded)
COPY armonite /app/
```

## Verification

To verify UI is properly embedded:

```bash
# Remove external UI folder
rm -rf ui-build/

# Start coordinator
./armonite coordinator --ui --http-port 8080

# UI should still work at http://localhost:8081
curl http://localhost:8081  # Should return HTML
```