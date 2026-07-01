package upgrade

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
	selfupdate "github.com/creativeprojects/go-selfupdate"
)

const (
	repositoryOwner = "epoxsizer"
	repositoryName  = "kan"
	checksumName    = "checksums.txt"
)

var (
	ErrDevelopmentBuild = errors.New("self-upgrade is unavailable for a development build")
	ErrNoRelease        = errors.New("no compatible stable public release was found")
)

type Result struct {
	Current    string `json:"current"`
	Latest     string `json:"latest"`
	Available  bool   `json:"available"`
	ReleaseURL string `json:"release_url,omitempty"`
}

type candidate struct {
	version    string
	releaseURL string
	native     any
}

type backend interface {
	latest(context.Context) (candidate, bool, error)
	apply(context.Context, candidate) error
}

type Service struct {
	backend backend
}

func New() (*Service, error) {
	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{APIToken: githubTokenFromEnv()})
	if err != nil {
		return nil, fmt.Errorf("configure GitHub release source: %w", err)
	}
	return newWithSource(source, "", "")
}

func githubTokenFromEnv() string {
	for _, name := range []string{"KAN_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"} {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func newWithSource(source selfupdate.Source, operatingSystem, architecture string) (*Service, error) {
	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source:    source,
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: checksumName},
		OS:        operatingSystem,
		Arch:      architecture,
	})
	if err != nil {
		return nil, fmt.Errorf("configure updater: %w", err)
	}
	return &Service{backend: &selfUpdateBackend{
		updater:    updater,
		repository: selfupdate.NewRepositorySlug(repositoryOwner, repositoryName),
	}}, nil
}

func (service *Service) Check(ctx context.Context, current string) (Result, error) {
	currentVersion, err := parseCurrentVersion(current)
	if err != nil {
		return Result{}, err
	}
	latest, found, err := service.backend.latest(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("check latest release: %w", err)
	}
	result := Result{Current: currentVersion.String(), Latest: currentVersion.String()}
	if !found {
		return Result{}, ErrNoRelease
	}
	latestVersion, err := semver.NewVersion(latest.version)
	if err != nil {
		return Result{}, fmt.Errorf("parse latest release version %q: %w", latest.version, err)
	}
	result.Latest = latestVersion.String()
	result.Available = latestVersion.GreaterThan(currentVersion)
	result.ReleaseURL = latest.releaseURL
	return result, nil
}

func (service *Service) Upgrade(ctx context.Context, current string) (Result, error) {
	result, latest, err := service.checkCandidate(ctx, current)
	if err != nil || !result.Available {
		return result, err
	}
	if err = service.backend.apply(ctx, latest); err != nil {
		return Result{}, fmt.Errorf("replace executable: %w", err)
	}
	return result, nil
}

func (service *Service) checkCandidate(ctx context.Context, current string) (Result, candidate, error) {
	currentVersion, err := parseCurrentVersion(current)
	if err != nil {
		return Result{}, candidate{}, err
	}
	latest, found, err := service.backend.latest(ctx)
	if err != nil {
		return Result{}, candidate{}, fmt.Errorf("check latest release: %w", err)
	}
	result := Result{Current: currentVersion.String(), Latest: currentVersion.String()}
	if !found {
		return Result{}, candidate{}, ErrNoRelease
	}
	latestVersion, err := semver.NewVersion(latest.version)
	if err != nil {
		return Result{}, candidate{}, fmt.Errorf("parse latest release version %q: %w", latest.version, err)
	}
	result.Latest = latestVersion.String()
	result.Available = latestVersion.GreaterThan(currentVersion)
	result.ReleaseURL = latest.releaseURL
	return result, latest, nil
}

func parseCurrentVersion(value string) (*semver.Version, error) {
	value = strings.TrimSpace(strings.TrimPrefix(value, "v"))
	if value == "" || value == "none" || value == "devel" || value == "dev" {
		return nil, ErrDevelopmentBuild
	}
	version, err := semver.NewVersion(value)
	if err != nil {
		return nil, fmt.Errorf("%w: version %q is not semantic", ErrDevelopmentBuild, value)
	}
	return version, nil
}

type selfUpdateBackend struct {
	updater    *selfupdate.Updater
	repository selfupdate.Repository
}

func (backend *selfUpdateBackend) latest(ctx context.Context) (candidate, bool, error) {
	release, found, err := backend.updater.DetectLatest(ctx, backend.repository)
	if err != nil || !found {
		return candidate{}, found, err
	}
	return candidate{version: release.Version(), releaseURL: release.URL, native: release}, true, nil
}

func (backend *selfUpdateBackend) apply(ctx context.Context, value candidate) error {
	release, ok := value.native.(*selfupdate.Release)
	if !ok || release == nil {
		return errors.New("invalid release candidate")
	}
	executable, err := selfupdate.ExecutablePath()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	return backend.updater.UpdateTo(ctx, release, executable)
}
