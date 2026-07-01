package upgrade

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	selfupdate "github.com/creativeprojects/go-selfupdate"
	"github.com/google/go-github/v30/github"
	"github.com/stretchr/testify/require"
)

type fakeBackend struct {
	candidate candidate
	found     bool
	latestErr error
	applyErr  error
	applied   int
}

func (backend *fakeBackend) latest(context.Context) (candidate, bool, error) {
	return backend.candidate, backend.found, backend.latestErr
}

func (backend *fakeBackend) apply(context.Context, candidate) error {
	backend.applied++
	return backend.applyErr
}

func TestCheckReportsOnlyNewerStableCandidate(t *testing.T) {
	backend := &fakeBackend{candidate: candidate{version: "1.2.0", releaseURL: "https://example.test/v1.2.0"}, found: true}
	service := &Service{backend: backend}

	result, err := service.Check(context.Background(), "v1.1.0")
	require.NoError(t, err)
	require.Equal(t, "1.1.0", result.Current)
	require.Equal(t, "1.2.0", result.Latest)
	require.True(t, result.Available)
	require.Equal(t, "https://example.test/v1.2.0", result.ReleaseURL)
}

func TestUpgradeAppliesNewerCandidateOnly(t *testing.T) {
	backend := &fakeBackend{candidate: candidate{version: "1.2.0"}, found: true}
	service := &Service{backend: backend}

	result, err := service.Upgrade(context.Background(), "1.1.0")
	require.NoError(t, err)
	require.True(t, result.Available)
	require.Equal(t, 1, backend.applied)

	backend.candidate.version = "1.0.0"
	result, err = service.Upgrade(context.Background(), "1.1.0")
	require.NoError(t, err)
	require.False(t, result.Available)
	require.Equal(t, 1, backend.applied)
}

func TestUpgradeRejectsDevelopmentBuildAndPropagatesFailures(t *testing.T) {
	service := &Service{backend: &fakeBackend{}}
	_, err := service.Upgrade(context.Background(), "devel")
	require.ErrorIs(t, err, ErrDevelopmentBuild)

	_, err = service.Check(context.Background(), "1.0.0")
	require.ErrorIs(t, err, ErrNoRelease)

	expected := errors.New("release API unavailable")
	service.backend = &fakeBackend{latestErr: expected}
	_, err = service.Check(context.Background(), "1.0.0")
	require.ErrorIs(t, err, expected)

	service.backend = &fakeBackend{candidate: candidate{version: "1.1.0"}, found: true, applyErr: expected}
	_, err = service.Upgrade(context.Background(), "1.0.0")
	require.ErrorIs(t, err, expected)
}

type fakeReleaseSource struct {
	releases []selfupdate.SourceRelease
}

func (source *fakeReleaseSource) ListReleases(context.Context, selfupdate.Repository) ([]selfupdate.SourceRelease, error) {
	return source.releases, nil
}

func (source *fakeReleaseSource) DownloadReleaseAsset(context.Context, *selfupdate.Release, int64) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func TestReleaseAssetNamingAndChecksumAreRequired(t *testing.T) {
	release := func(assets ...string) selfupdate.SourceRelease {
		githubAssets := make([]*github.ReleaseAsset, 0, len(assets))
		for index, name := range assets {
			githubAssets = append(githubAssets, &github.ReleaseAsset{
				ID: github.Int64(int64(index + 1)), Name: github.String(name),
				BrowserDownloadURL: github.String("https://example.test/" + name),
			})
		}
		return selfupdate.NewGitHubRelease(&github.RepositoryRelease{
			ID: github.Int64(1), TagName: github.String("v1.1.0"), HTMLURL: github.String("https://example.test/v1.1.0"),
			Assets: githubAssets,
		})
	}

	service, err := newWithSource(&fakeReleaseSource{releases: []selfupdate.SourceRelease{
		release("kan_linux_amd64.tar.gz", checksumName),
	}}, "linux", "amd64")
	require.NoError(t, err)
	result, err := service.Check(context.Background(), "1.0.0")
	require.NoError(t, err)
	require.True(t, result.Available)

	service, err = newWithSource(&fakeReleaseSource{releases: []selfupdate.SourceRelease{
		release("kan_linux_amd64.tar.gz"),
	}}, "linux", "amd64")
	require.NoError(t, err)
	_, err = service.Check(context.Background(), "1.0.0")
	require.ErrorIs(t, err, selfupdate.ErrValidationAssetNotFound)
}
