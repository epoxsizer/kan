# Release process

Releases are built by GitLab CI with GoReleaser. Archives contain statically
built `kan` binaries for Linux, macOS, and Windows on amd64 and arm64, plus a SHA-256
checksum file.

## Prepare

1. Move relevant entries from `Unreleased` to a dated version in `CHANGELOG.md`.
2. Run `make check` and `make cross-build`.
3. Optionally run `make snapshot` with GoReleaser v2 installed.
4. Commit the release changes and merge them to `main`.

Before the first release, create a masked and protected GitLab CI/CD variable named
`GORELEASER_GITLAB_TOKEN`. Use a project access token with the `api` scope and a role
allowed to create releases. Protect release tags so the variable is available only
to authorized tag pipelines.

## Build a release

```sh
git tag -s v0.1.0 -m "kan v0.1.0"
git push origin v0.1.0
```

The tag pipeline creates a GitLab release and uploads its archives and checksum
file. Protect release tags in GitLab before the first release. Download and verify
at least one archive and check `kan --version`. If signed tags are unavailable,
use an annotated tag with `git tag -a`.
