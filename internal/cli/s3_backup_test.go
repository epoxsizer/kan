package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/epoxsizer/kan/internal/config"
	"github.com/stretchr/testify/require"
)

func TestS3BackupRotationDeletesOnlyExpiredGeneratedObjects(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	deleted := []string{}
	originalClient := s3BackupHTTPClient
	t.Cleanup(func() { s3BackupHTTPClient = originalClient })
	s3BackupHTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		require.NotEmpty(t, request.Header.Get("Authorization"))
		switch request.Method {
		case http.MethodGet:
			require.Equal(t, "/backups/", request.URL.Path)
			require.Equal(t, "kan/backups/", request.URL.Query().Get("prefix"))
			body := fmt.Sprintf(`<ListBucketResult>
				<IsTruncated>false</IsTruncated>
				<Contents><Key>kan/backups/kan-auto-20260616-120000.db</Key><LastModified>%s</LastModified></Contents>
				<Contents><Key>kan/backups/release-20260618-120000.db</Key><LastModified>%s</LastModified></Contents>
				<Contents><Key>kan/backups/notes.db</Key><LastModified>%s</LastModified></Contents>
			</ListBucketResult>`, now.Add(-15*24*time.Hour).Format(time.RFC3339), now.Add(-13*24*time.Hour).Format(time.RFC3339), now.Add(-30*24*time.Hour).Format(time.RFC3339))
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		case http.MethodDelete:
			deleted = append(deleted, request.URL.Path)
			return &http.Response{StatusCode: http.StatusNoContent, Status: "204 No Content", Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		default:
			return nil, fmt.Errorf("unexpected method %s", request.Method)
		}
	})}

	cfg := config.S3Backup{
		Bucket: "backups", Prefix: "kan/backups", Region: "us-east-1",
		Endpoint: "https://s3.example.test", AccessKeyID: "key", SecretAccessKey: "secret", ForcePathStyle: true,
	}
	removed, err := (realS3Uploader{}).Rotate(context.Background(), cfg, 14*24*time.Hour, now)
	require.NoError(t, err)
	require.Equal(t, 1, removed)
	require.Equal(t, []string{"/backups/kan/backups/kan-auto-20260616-120000.db"}, deleted)
}
