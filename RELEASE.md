# Release process

Releases are built by GitHub Actions with GoReleaser. Archives contain statically
built `kan` binaries for Linux, macOS, and Windows on amd64 and arm64, plus a SHA-256
checksum file.

## Prepare

1. Move relevant entries from `Unreleased` to a dated version in `CHANGELOG.md`.
2. Run `make check` and `make cross-build`.
3. Optionally run `make snapshot` with GoReleaser v2 installed.
4. Commit the release changes and merge them to `main`.

Before the first release, confirm that GitHub Actions is enabled for the
repository. The release workflow uses the repository-provided `GITHUB_TOKEN`.
Protect release tags or restrict workflow permissions according to the repository
policy.

## Build a release

```sh
git tag -a v0.1.5 -m "kan v0.1.5"
git push origin main
git push origin v0.1.5
```

The tag workflow creates a GitHub release and uploads its archives and checksum
file. Download and verify at least one archive and check `kan --version`. Use
`git tag -s` instead when a signing key and signed-tag policy are configured.
