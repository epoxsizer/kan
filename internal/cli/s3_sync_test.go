package cli

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/epoxsizer/kan/internal/config"
	"github.com/epoxsizer/kan/internal/domain"
	"github.com/epoxsizer/kan/internal/seed"
	storage "github.com/epoxsizer/kan/internal/storage/sqlite"
	"github.com/stretchr/testify/require"
)

type fakeSyncClient struct {
	body            []byte
	etag            string
	exists          bool
	lastIfMatch     string
	lastIfNoneMatch bool
	putCount        int
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func (client *fakeSyncClient) Get(_ context.Context, _ config.S3Sync, _ string, ifNoneMatch string) (s3Object, error) {
	if !client.exists {
		return s3Object{NotFound: true}, nil
	}
	if ifNoneMatch != "" && ifNoneMatch == client.etag {
		return s3Object{NotModified: true, ETag: client.etag}, nil
	}
	return s3Object{Body: append([]byte(nil), client.body...), ETag: client.etag}, nil
}

func (client *fakeSyncClient) Put(_ context.Context, _ config.S3Sync, _ string, body []byte, ifMatch string, ifNoneMatch bool) (string, error) {
	client.lastIfMatch = ifMatch
	client.lastIfNoneMatch = ifNoneMatch
	if ifNoneMatch && client.exists {
		return "", errS3Precondition
	}
	if ifMatch != "" && (!client.exists || ifMatch != client.etag) {
		return "", errS3Precondition
	}
	client.putCount++
	client.exists = true
	client.body = append([]byte(nil), body...)
	client.etag = `"etag-` + time.Unix(int64(client.putCount), 0).UTC().Format("150405") + `"`
	return client.etag, nil
}

func testSyncConfig() config.Sync {
	return config.Sync{
		Enabled: true, Interval: "30m", ObjectKey: "kan/sync.json",
		S3: config.S3Sync{Bucket: "kan-sync", Region: "us-east-1", AccessKeyID: "key", SecretAccessKey: "secret"},
	}
}

func openTestSyncEngine(t *testing.T, client s3SyncClient) *syncEngine {
	t.Helper()
	directory := t.TempDir()
	database := filepath.Join(directory, "kan.db")
	repo, err := storage.Open(context.Background(), database)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, repo.Close()) })
	engine, err := newSyncEngine(repo, database, testSyncConfig(), directory, slog.New(slog.NewTextHandler(io.Discard, nil)), client)
	require.NoError(t, err)
	engine.now = func() time.Time { return time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC) }
	return engine
}

func TestStartupPullsRemoteDataIntoEmptyDatabase(t *testing.T) {
	sourceClient := &fakeSyncClient{}
	source := openTestSyncEngine(t, sourceClient)
	require.NoError(t, seed.Demo(context.Background(), source.repo))
	sourceSnapshot, err := source.snapshot(context.Background())
	require.NoError(t, err)

	remote := &fakeSyncClient{body: sourceSnapshot.JSON, etag: `"remote-1"`, exists: true}
	target := openTestSyncEngine(t, remote)
	require.NoError(t, target.startup(context.Background()))

	projects, err := target.repo.ListProjects(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, projects)
	state, err := target.readState()
	require.NoError(t, err)
	require.Equal(t, sourceSnapshot.DataHash, state.DataHash)
	require.Equal(t, `"remote-1"`, state.ETag)
}

func TestSafePushUsesLastRemoteETag(t *testing.T) {
	remote := &fakeSyncClient{}
	engine := openTestSyncEngine(t, remote)
	require.NoError(t, seed.Demo(context.Background(), engine.repo))
	require.NoError(t, engine.startup(context.Background()))
	baselineETag := remote.etag

	project := domain.Project{Name: "Local change", Position: 4096}
	require.NoError(t, engine.repo.CreateProject(context.Background(), &project))
	require.NoError(t, engine.push(context.Background(), false))

	require.Equal(t, baselineETag, remote.lastIfMatch)
	require.False(t, remote.lastIfNoneMatch)
	require.Equal(t, 2, remote.putCount)
}

func TestManualPullCreatesLocalBackupBeforeReplacingData(t *testing.T) {
	source := openTestSyncEngine(t, &fakeSyncClient{})
	remoteProject := domain.Project{Name: "Remote project", Position: 1024}
	require.NoError(t, source.repo.CreateProject(context.Background(), &remoteProject))
	remoteSnapshot, err := source.snapshot(context.Background())
	require.NoError(t, err)

	remote := &fakeSyncClient{body: remoteSnapshot.JSON, etag: `"remote-1"`, exists: true}
	target := openTestSyncEngine(t, remote)
	localProject := domain.Project{Name: "Local project", Position: 1024}
	require.NoError(t, target.repo.CreateProject(context.Background(), &localProject))
	require.NoError(t, target.pull(context.Background()))

	backups, err := filepath.Glob(filepath.Join(target.workingDirectory, "backup", "kan-pre-sync-*.db"))
	require.NoError(t, err)
	require.Len(t, backups, 1)
	projects, err := target.repo.ListProjects(context.Background())
	require.NoError(t, err)
	require.Len(t, projects, 1)
	require.Equal(t, "Remote project", projects[0].Name)
}

func TestStartupStopsWhenLocalAndRemoteBothChanged(t *testing.T) {
	remote := &fakeSyncClient{}
	engine := openTestSyncEngine(t, remote)
	require.NoError(t, seed.Demo(context.Background(), engine.repo))
	require.NoError(t, engine.startup(context.Background()))

	localProject := domain.Project{Name: "Local change", Position: 4096}
	require.NoError(t, engine.repo.CreateProject(context.Background(), &localProject))

	var document domain.ExportDocument
	require.NoError(t, json.Unmarshal(remote.body, &document))
	now := time.Date(2026, 6, 29, 13, 0, 0, 0, time.UTC)
	document.Projects = append(document.Projects, domain.ExportProject{Project: domain.Project{
		ID: "00000000-0000-4000-8000-000000009999", Name: "Remote change", Position: 5120, CreatedAt: now, UpdatedAt: now,
	}, Boards: []domain.ExportBoard{}})
	remote.body, _ = json.Marshal(document)
	remote.etag = `"remote-2"`

	err := engine.startup(context.Background())
	require.ErrorIs(t, err, errSyncConflict)
	require.ErrorContains(t, err, "both changed")
}

func TestS3SyncClientSendsConditionalHeaders(t *testing.T) {
	var putIfMatch, putIfNoneMatch, getIfNoneMatch string
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		require.NotEmpty(t, request.Header.Get("Authorization"))
		switch request.Method {
		case http.MethodPut:
			putIfMatch = request.Header.Get("If-Match")
			putIfNoneMatch = request.Header.Get("If-None-Match")
			return &http.Response{
				StatusCode: http.StatusOK, Status: "200 OK", Header: http.Header{"Etag": []string{`"new-etag"`}},
				Body: io.NopCloser(strings.NewReader("")),
			}, nil
		case http.MethodGet:
			getIfNoneMatch = request.Header.Get("If-None-Match")
			return &http.Response{
				StatusCode: http.StatusNotModified, Status: "304 Not Modified", Header: make(http.Header),
				Body: io.NopCloser(strings.NewReader("")),
			}, nil
		default:
			return &http.Response{
				StatusCode: http.StatusMethodNotAllowed, Status: "405 Method Not Allowed", Header: make(http.Header),
				Body: io.NopCloser(strings.NewReader("")),
			}, nil
		}
	})}

	cfg := testSyncConfig().S3
	cfg.Endpoint = "https://s3.example.test"
	cfg.ForcePathStyle = true
	client := realS3SyncClient{httpClient: httpClient, now: func() time.Time {
		return time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	}}
	etag, err := client.Put(context.Background(), cfg, "kan/sync.json", []byte(`{"ok":true}`), `"old-etag"`, false)
	require.NoError(t, err)
	require.Equal(t, `"new-etag"`, etag)
	require.Equal(t, `"old-etag"`, putIfMatch)
	require.Empty(t, putIfNoneMatch)

	object, err := client.Get(context.Background(), cfg, "kan/sync.json", `"new-etag"`)
	require.NoError(t, err)
	require.True(t, object.NotModified)
	require.Equal(t, `"new-etag"`, getIfNoneMatch)
}

func TestSyncStateRejectsMemoryAndSQLiteURIPaths(t *testing.T) {
	_, err := syncStatePath(":memory:")
	require.ErrorContains(t, err, "file-backed")
	_, err = syncStatePath("file:kan.db")
	require.ErrorContains(t, err, "URI")
}

func TestS3ObjectURLDoesNotDoubleEscapeCustomEndpointKeys(t *testing.T) {
	endpoint, canonicalURI, err := s3ObjectURL("kan-sync", "us-east-1", "https://s3.example.test/storage", true, "folder/a b.json")
	require.NoError(t, err)
	require.Equal(t, "https://s3.example.test/storage/kan-sync/folder/a%20b.json", endpoint)
	require.Equal(t, "/storage/kan-sync/folder/a%20b.json", canonicalURI)
}
