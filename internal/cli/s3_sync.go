package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/epoxsizer/kan/internal/config"
	"github.com/epoxsizer/kan/internal/domain"
	storage "github.com/epoxsizer/kan/internal/storage/sqlite"
)

const syncStateVersion = 1

var errSyncConflict = errors.New("s3 sync conflict")

type syncState struct {
	Version   int       `json:"version"`
	Bucket    string    `json:"bucket"`
	ObjectKey string    `json:"object_key"`
	Region    string    `json:"region"`
	Endpoint  string    `json:"endpoint,omitempty"`
	PathStyle bool      `json:"force_path_style"`
	ETag      string    `json:"etag"`
	DataHash  string    `json:"data_hash"`
	SyncedAt  time.Time `json:"synced_at"`
}

type syncSnapshot struct {
	Document domain.ExportDocument
	JSON     []byte
	DataHash string
	Empty    bool
}

type syncEngine struct {
	repo             *storage.Repository
	config           config.Sync
	client           s3SyncClient
	statePath        string
	workingDirectory string
	logger           *slog.Logger
	now              func() time.Time
}

func newSyncEngine(repo *storage.Repository, database string, syncConfig config.Sync, workingDirectory string, logger *slog.Logger, client s3SyncClient) (*syncEngine, error) {
	if !syncConfig.Enabled {
		return nil, errors.New("sync is disabled; set sync.enabled = true")
	}
	statePath, err := syncStatePath(database)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = realS3SyncClient{}
	}
	return &syncEngine{
		repo:             repo,
		config:           syncConfig,
		client:           client,
		statePath:        statePath,
		workingDirectory: workingDirectory,
		logger:           logger,
		now:              time.Now,
	}, nil
}

func syncStatePath(database string) (string, error) {
	if database == ":memory:" || strings.Contains(database, "mode=memory") {
		return "", errors.New("s3 sync requires a file-backed SQLite database")
	}
	if strings.HasPrefix(database, "file:") {
		return "", errors.New("s3 sync does not support SQLite URI database paths")
	}
	absolute, err := filepath.Abs(database)
	if err != nil {
		return "", fmt.Errorf("resolve sync state path: %w", err)
	}
	return absolute + ".sync-state.json", nil
}

func (engine *syncEngine) startup(ctx context.Context) error {
	local, err := engine.snapshot(ctx)
	if err != nil {
		return err
	}
	state, err := engine.readState()
	if err != nil {
		return err
	}
	if state != nil && !engine.matchesTarget(*state) {
		return syncConflict("sync target differs from the target recorded in %s", engine.statePath)
	}
	remote, err := engine.client.Get(ctx, engine.config.S3, engine.config.ObjectKey, "")
	if err != nil {
		return err
	}
	if remote.NotFound {
		if state != nil {
			return syncConflict("remote object was deleted after the last successful sync")
		}
		return engine.upload(ctx, local, "", true)
	}
	remoteDocument, remoteHash, err := decodeSyncDocument(ctx, remote.Body)
	if err != nil {
		return fmt.Errorf("validate remote sync object: %w", err)
	}
	if state == nil {
		switch {
		case local.Empty:
			return engine.importRemote(ctx, remoteDocument, remoteHash, remote.ETag, false)
		case local.DataHash == remoteHash:
			return engine.writeState(remote.ETag, remoteHash)
		default:
			return syncConflict("local and remote data differ and no previous sync state exists")
		}
	}
	if local.DataHash == remoteHash {
		return engine.writeState(remote.ETag, remoteHash)
	}
	localChanged := local.DataHash != state.DataHash
	remoteChanged := remoteHash != state.DataHash
	switch {
	case localChanged && remoteChanged:
		return syncConflict("local and remote data both changed since the last successful sync")
	case localChanged:
		return engine.upload(ctx, local, remote.ETag, false)
	case remoteChanged:
		return engine.importRemote(ctx, remoteDocument, remoteHash, remote.ETag, !local.Empty)
	default:
		return engine.writeState(remote.ETag, remoteHash)
	}
}

func (engine *syncEngine) push(ctx context.Context, force bool) error {
	local, err := engine.snapshot(ctx)
	if err != nil {
		return err
	}
	if force {
		return engine.upload(ctx, local, "", false)
	}
	state, err := engine.readState()
	if err != nil {
		return err
	}
	if state != nil && !engine.matchesTarget(*state) {
		return syncConflict("sync target differs from the target recorded in %s", engine.statePath)
	}
	if state == nil {
		remote, getErr := engine.client.Get(ctx, engine.config.S3, engine.config.ObjectKey, "")
		if getErr != nil {
			return getErr
		}
		if remote.NotFound {
			return engine.upload(ctx, local, "", true)
		}
		_, remoteHash, decodeErr := decodeSyncDocument(ctx, remote.Body)
		if decodeErr != nil {
			return fmt.Errorf("validate remote sync object: %w", decodeErr)
		}
		if remoteHash != local.DataHash {
			return syncConflict("local and remote data differ and no previous sync state exists")
		}
		return engine.writeState(remote.ETag, remoteHash)
	}

	remote, err := engine.client.Get(ctx, engine.config.S3, engine.config.ObjectKey, state.ETag)
	if err != nil {
		return err
	}
	if remote.NotFound {
		return syncConflict("remote object was deleted after the last successful sync")
	}
	if remote.NotModified {
		if local.DataHash == state.DataHash {
			return nil
		}
		return engine.upload(ctx, local, state.ETag, false)
	}
	_, remoteHash, err := decodeSyncDocument(ctx, remote.Body)
	if err != nil {
		return fmt.Errorf("validate remote sync object: %w", err)
	}
	if remoteHash != state.DataHash {
		return syncConflict("remote data changed; restart kan to reconcile before pushing")
	}
	if local.DataHash == state.DataHash {
		return engine.writeState(remote.ETag, remoteHash)
	}
	return engine.upload(ctx, local, remote.ETag, false)
}

func (engine *syncEngine) pull(ctx context.Context) error {
	remote, err := engine.client.Get(ctx, engine.config.S3, engine.config.ObjectKey, "")
	if err != nil {
		return err
	}
	if remote.NotFound {
		return errors.New("remote sync object does not exist")
	}
	document, dataHash, err := decodeSyncDocument(ctx, remote.Body)
	if err != nil {
		return fmt.Errorf("validate remote sync object: %w", err)
	}
	local, err := engine.snapshot(ctx)
	if err != nil {
		return err
	}
	return engine.importRemote(ctx, document, dataHash, remote.ETag, !local.Empty)
}

func (engine *syncEngine) upload(ctx context.Context, snapshot syncSnapshot, ifMatch string, ifNoneMatch bool) error {
	etag, err := engine.client.Put(ctx, engine.config.S3, engine.config.ObjectKey, snapshot.JSON, ifMatch, ifNoneMatch)
	if errors.Is(err, errS3Precondition) {
		return syncConflict("remote object changed while uploading")
	}
	if err != nil {
		return err
	}
	return engine.writeState(etag, snapshot.DataHash)
}

func (engine *syncEngine) importRemote(ctx context.Context, document domain.ExportDocument, dataHash, etag string, backup bool) error {
	if backup {
		path := filepath.Join(storage.BackupDirectory(engine.workingDirectory), "kan-pre-sync-"+engine.currentTime().Format("20060102-150405")+".db")
		if err := engine.repo.Backup(ctx, path); err != nil {
			return fmt.Errorf("create backup before sync import: %w", err)
		}
		if engine.logger != nil {
			engine.logger.Info("pre-sync database backup created", "path", path)
		}
	}
	if err := engine.repo.ImportDocument(ctx, document, true); err != nil {
		return fmt.Errorf("import remote sync data: %w", err)
	}
	return engine.writeState(etag, dataHash)
}

func (engine *syncEngine) snapshot(ctx context.Context) (syncSnapshot, error) {
	directory, err := os.MkdirTemp("", "kan-sync-snapshot-*")
	if err != nil {
		return syncSnapshot{}, fmt.Errorf("create sync snapshot directory: %w", err)
	}
	defer os.RemoveAll(directory)
	path := filepath.Join(directory, "kan.db")
	if err = engine.repo.Backup(ctx, path); err != nil {
		return syncSnapshot{}, fmt.Errorf("create sync snapshot: %w", err)
	}
	snapshotRepo, err := storage.Open(ctx, path)
	if err != nil {
		return syncSnapshot{}, fmt.Errorf("open sync snapshot: %w", err)
	}
	document, exportErr := buildExport(ctx, snapshotRepo, engine.currentTime())
	closeErr := snapshotRepo.Close()
	if exportErr != nil {
		return syncSnapshot{}, fmt.Errorf("export sync snapshot: %w", exportErr)
	}
	if closeErr != nil {
		return syncSnapshot{}, fmt.Errorf("close sync snapshot: %w", closeErr)
	}
	contents, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return syncSnapshot{}, fmt.Errorf("encode sync snapshot: %w", err)
	}
	contents = append(contents, '\n')
	dataHash, err := syncDocumentHash(document)
	if err != nil {
		return syncSnapshot{}, err
	}
	return syncSnapshot{Document: document, JSON: contents, DataHash: dataHash, Empty: len(document.Projects) == 0}, nil
}

func decodeSyncDocument(ctx context.Context, contents []byte) (domain.ExportDocument, string, error) {
	if len(contents) > maxSyncObjectSize {
		return domain.ExportDocument{}, "", fmt.Errorf("sync object exceeds %d bytes", maxSyncObjectSize)
	}
	var document domain.ExportDocument
	decoder := json.NewDecoder(strings.NewReader(string(contents)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return document, "", fmt.Errorf("decode JSON: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return document, "", err
	}
	if document.Format != "kan" || document.Version != domain.ExportVersion {
		return document, "", fmt.Errorf("sync requires format kan version %d", domain.ExportVersion)
	}
	validationRepo, err := storage.Open(ctx, ":memory:")
	if err != nil {
		return document, "", fmt.Errorf("open validation database: %w", err)
	}
	importErr := validationRepo.ImportDocument(ctx, document, true)
	closeErr := validationRepo.Close()
	if importErr != nil {
		return document, "", importErr
	}
	if closeErr != nil {
		return document, "", closeErr
	}
	dataHash, err := syncDocumentHash(document)
	return document, dataHash, err
}

func syncDocumentHash(document domain.ExportDocument) (string, error) {
	document.ExportedAt = time.Time{}
	contents, err := json.Marshal(document)
	if err != nil {
		return "", fmt.Errorf("encode canonical sync data: %w", err)
	}
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:]), nil
}

func (engine *syncEngine) readState() (*syncState, error) {
	contents, err := os.ReadFile(engine.statePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read sync state: %w", err)
	}
	var state syncState
	decoder := json.NewDecoder(strings.NewReader(string(contents)))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&state); err != nil {
		return nil, fmt.Errorf("decode sync state: %w", err)
	}
	if err = ensureJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("decode sync state: %w", err)
	}
	if state.Version != syncStateVersion || state.ETag == "" || state.DataHash == "" {
		return nil, errors.New("sync state is invalid or unsupported")
	}
	return &state, nil
}

func (engine *syncEngine) writeState(etag, dataHash string) error {
	state := syncState{
		Version: syncStateVersion, Bucket: engine.config.S3.Bucket, ObjectKey: engine.config.ObjectKey,
		Region: engine.config.S3.Region, Endpoint: engine.config.S3.Endpoint, PathStyle: engine.config.S3.ForcePathStyle,
		ETag: etag, DataHash: dataHash, SyncedAt: engine.currentTime(),
	}
	contents, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode sync state: %w", err)
	}
	contents = append(contents, '\n')
	if err = config.EnsureParent(engine.statePath); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(engine.statePath), ".kan-sync-state-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary sync state: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(contents)
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil {
		return fmt.Errorf("write sync state: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close sync state: %w", closeErr)
	}
	if err = replaceFile(temporaryPath, engine.statePath); err != nil {
		return fmt.Errorf("publish sync state: %w", err)
	}
	return nil
}

func (engine *syncEngine) matchesTarget(state syncState) bool {
	return state.Bucket == engine.config.S3.Bucket &&
		state.ObjectKey == engine.config.ObjectKey &&
		state.Region == engine.config.S3.Region &&
		state.PathStyle == engine.config.S3.ForcePathStyle &&
		strings.TrimRight(state.Endpoint, "/") == strings.TrimRight(engine.config.S3.Endpoint, "/")
}

func (engine *syncEngine) currentTime() time.Time {
	if engine.now == nil {
		return time.Now().UTC()
	}
	return engine.now().UTC()
}

func syncConflict(format string, arguments ...any) error {
	return fmt.Errorf("%w: %s", errSyncConflict, fmt.Sprintf(format, arguments...))
}
