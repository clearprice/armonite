# Release Process

This document describes how to create a new release of Armonite.

## Prerequisites

1. Ensure all changes are merged to `main` branch
2. Update version references in documentation if needed
3. Test the build locally

## Creating a Release

### 1. Create and Push a Version Tag

```bash
# Make sure you're on the main branch and up to date
git checkout main
git pull origin main

# Create a new tag (replace v1.0.0 with your version)
git tag -a v1.0.0 -m "Release v1.0.0"

# Push the tag to trigger the release workflow
git push origin v1.0.0
```

### 2. Monitor the Release Workflow

1. Go to the **Actions** tab in GitHub
2. Watch the "Build and Release" workflow
3. The workflow will:
   - Build binaries for all platforms (Linux, macOS, Windows on AMD64 and ARM64)
   - Create archives (`.tar.gz` for Unix, `.zip` for Windows)
   - Generate SHA256 checksums
   - Create a GitHub release with all artifacts
   - Build and push Docker images to GitHub Container Registry

### 3. Verify the Release

Once the workflow completes:

1. Check the **Releases** page for the new release
2. Verify all platform binaries are attached
3. Test download and extraction of at least one archive
4. Verify the Docker image is available:
   ```bash
   docker pull ghcr.io/[your-username]/armonite:v1.0.0
   ```

## Release Artifacts

Each release includes:

### Binaries
- `armonite-v1.0.0-linux-amd64.tar.gz`
- `armonite-v1.0.0-linux-arm64.tar.gz`
- `armonite-v1.0.0-darwin-amd64.tar.gz`
- `armonite-v1.0.0-darwin-arm64.tar.gz`
- `armonite-v1.0.0-windows-amd64.zip`
- `armonite-v1.0.0-windows-arm64.zip`

### Each archive contains:
- `armonite` binary (or `armonite.exe` on Windows)
- `README.md`
- `CONCEPTS.md`
- `armonite.yaml` (configuration example)
- `example-testplan.yaml`

### Docker Images
- `ghcr.io/[your-username]/armonite:v1.0.0`
- `ghcr.io/[your-username]/armonite:v1.0`
- `ghcr.io/[your-username]/armonite:v1`

### Additional Files
- `checksums.txt` - SHA256 checksums for all archives

## Version Numbering

Armonite follows semantic versioning (SemVer):

- **MAJOR** version for incompatible API changes
- **MINOR** version for backwards-compatible functionality additions
- **PATCH** version for backwards-compatible bug fixes

Examples:
- `v1.0.0` - Major release
- `v1.1.0` - Minor feature update
- `v1.0.1` - Patch/bugfix release
- `v2.0.0-beta.1` - Pre-release

## Hotfix Releases

For urgent fixes:

1. Create a hotfix branch from the release tag:
   ```bash
   git checkout v1.0.0
   git checkout -b hotfix/v1.0.1
   ```

2. Make the necessary changes

3. Create a new patch release tag:
   ```bash
   git tag -a v1.0.1 -m "Hotfix v1.0.1"
   git push origin v1.0.1
   ```

## Rollback

If a release has critical issues:

1. Delete the problematic release from GitHub
2. Delete the tag:
   ```bash
   git tag -d v1.0.0
   git push origin :refs/tags/v1.0.0
   ```
3. Fix the issues and create a new release

## Manual Testing Checklist

Before tagging a release, verify:

- [ ] Binary builds and starts correctly
- [ ] Web UI loads and functions
- [ ] Agent can connect to coordinator
- [ ] Test runs execute successfully
- [ ] Results are displayed properly
- [ ] Configuration files are valid
- [ ] Documentation is up to date

## Troubleshooting

### Workflow fails
- Check the Actions logs for specific errors
- Ensure all required secrets are set (GITHUB_TOKEN is automatic)
- Verify Go and Node.js versions are compatible

### Missing artifacts
- Check if the UI build completed successfully
- Verify the embed files are included in the binary

### Docker push fails
- Ensure GitHub Container Registry permissions are set
- Check if the repository has access to GHCR