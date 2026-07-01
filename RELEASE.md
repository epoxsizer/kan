# Release process

Releases are built by GitHub Actions with GoReleaser. Archives contain statically
built `kan` binaries for Linux, macOS, and Windows on amd64 and arm64, plus a SHA-256
checksum file. Published releases and their assets must remain public so installed
binaries can check and upgrade without credentials.

## Prepare

1. Move relevant entries from `Unreleased` to a dated version in `CHANGELOG.md`.
2. Run `make check` and `make cross-build`.
3. Run `make snapshot` with GoReleaser v2 and verify updater-compatible archive
   names such as `kan_linux_amd64.tar.gz` and `kan_windows_arm64.zip`.
4. Commit the release changes and merge them to `main`.

Before the first release, confirm that GitHub Actions is enabled for the
repository. The release workflow uses the repository-provided `GITHUB_TOKEN`.
Protect release tags or restrict workflow permissions according to the repository
policy.

## Build a release

```sh
VERSION=vX.Y.Z
git tag -a "$VERSION" -m "kan $VERSION"
git push origin main
git push origin "$VERSION"
```

The tag workflow creates a GitHub release and uploads its archives and checksum
file. Download and verify at least one archive and check `kan --version`. Use
`git tag -s` instead when a signing key and signed-tag policy are configured.
