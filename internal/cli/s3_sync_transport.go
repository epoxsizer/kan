package cli

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/epoxsizer/kan/internal/config"
)

const maxSyncObjectSize = 64 << 20

var errS3Precondition = errors.New("s3 object precondition failed")

type s3Object struct {
	Body        []byte
	ETag        string
	NotFound    bool
	NotModified bool
}

type s3SyncClient interface {
	Get(context.Context, config.S3Sync, string, string) (s3Object, error)
	Put(context.Context, config.S3Sync, string, []byte, string, bool) (string, error)
}

type s3HTTPError struct {
	StatusCode int
	Status     string
	Body       string
}

func (err *s3HTTPError) Error() string {
	if err.Body == "" {
		return "s3 request failed: " + err.Status
	}
	return "s3 request failed: " + err.Status + ": " + err.Body
}

type realS3SyncClient struct {
	httpClient *http.Client
	now        func() time.Time
}

func (client realS3SyncClient) Get(ctx context.Context, cfg config.S3Sync, key, ifNoneMatch string) (s3Object, error) {
	request, err := client.request(ctx, http.MethodGet, cfg, key, nil)
	if err != nil {
		return s3Object{}, err
	}
	if ifNoneMatch != "" {
		request.Header.Set("If-None-Match", ifNoneMatch)
	}
	response, err := client.do(request)
	if err != nil {
		return s3Object{}, fmt.Errorf("send s3 download request: %w", err)
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusNotModified:
		return s3Object{NotModified: true, ETag: ifNoneMatch}, nil
	case http.StatusNotFound:
		return s3Object{NotFound: true}, nil
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return s3Object{}, readS3HTTPError(response)
	}
	if response.ContentLength > maxSyncObjectSize {
		return s3Object{}, fmt.Errorf("s3 sync object exceeds %d bytes", maxSyncObjectSize)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxSyncObjectSize+1))
	if err != nil {
		return s3Object{}, fmt.Errorf("read s3 sync object: %w", err)
	}
	if len(body) > maxSyncObjectSize {
		return s3Object{}, fmt.Errorf("s3 sync object exceeds %d bytes", maxSyncObjectSize)
	}
	etag := strings.TrimSpace(response.Header.Get("ETag"))
	if etag == "" {
		return s3Object{}, errors.New("s3 sync object response has no ETag")
	}
	return s3Object{Body: body, ETag: etag}, nil
}

func (client realS3SyncClient) Put(ctx context.Context, cfg config.S3Sync, key string, body []byte, ifMatch string, ifNoneMatch bool) (string, error) {
	if len(body) > maxSyncObjectSize {
		return "", fmt.Errorf("s3 sync object exceeds %d bytes", maxSyncObjectSize)
	}
	request, err := client.request(ctx, http.MethodPut, cfg, key, body)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	if ifMatch != "" {
		request.Header.Set("If-Match", ifMatch)
	}
	if ifNoneMatch {
		request.Header.Set("If-None-Match", "*")
	}
	response, err := client.do(request)
	if err != nil {
		return "", fmt.Errorf("send s3 upload request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusConflict || response.StatusCode == http.StatusPreconditionFailed {
		return "", fmt.Errorf("%w: %s", errS3Precondition, response.Status)
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return "", readS3HTTPError(response)
	}
	etag := strings.TrimSpace(response.Header.Get("ETag"))
	if etag == "" {
		return "", errors.New("s3 sync upload response has no ETag")
	}
	return etag, nil
}

func (client realS3SyncClient) request(ctx context.Context, method string, cfg config.S3Sync, key string, body []byte) (*http.Request, error) {
	endpointURL, canonicalURI, err := s3SyncURL(cfg, key)
	if err != nil {
		return nil, err
	}
	payload := body
	if payload == nil {
		payload = []byte{}
	}
	payloadSum := sha256.Sum256(payload)
	payloadHash := hex.EncodeToString(payloadSum[:])
	now := time.Now().UTC()
	if client.now != nil {
		now = client.now().UTC()
	}
	parsed, err := url.Parse(endpointURL)
	if err != nil {
		return nil, fmt.Errorf("parse s3 endpoint: %w", err)
	}
	amzDate := now.Format("20060102T150405Z")
	shortDate := now.Format("20060102")
	headers := map[string]string{
		"host":                 parsed.Host,
		"x-amz-content-sha256": payloadHash,
		"x-amz-date":           amzDate,
	}
	authorization := s3Authorization(method, canonicalURI, "", headers, payloadHash, cfg.Region, cfg.AccessKeyID, cfg.SecretAccessKey, shortDate, amzDate)
	request, err := http.NewRequestWithContext(ctx, method, endpointURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create s3 request: %w", err)
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("X-Amz-Content-Sha256", payloadHash)
	request.Header.Set("X-Amz-Date", amzDate)
	if body != nil {
		request.ContentLength = int64(len(body))
	}
	return request, nil
}

func (client realS3SyncClient) do(request *http.Request) (*http.Response, error) {
	if client.httpClient != nil {
		return client.httpClient.Do(request)
	}
	return http.DefaultClient.Do(request)
}

func s3SyncURL(cfg config.S3Sync, key string) (string, string, error) {
	return s3ObjectURL(cfg.Bucket, cfg.Region, cfg.Endpoint, cfg.ForcePathStyle, key)
}

func s3ObjectURL(bucket, region, endpoint string, forcePathStyle bool, key string) (string, string, error) {
	escapedKey := escapeS3Key(key)
	if endpoint == "" {
		if forcePathStyle {
			return "https://s3." + region + ".amazonaws.com/" + bucket + "/" + escapedKey, "/" + bucket + "/" + escapedKey, nil
		}
		return "https://" + bucket + ".s3." + region + ".amazonaws.com/" + escapedKey, "/" + escapedKey, nil
	}
	base, err := url.Parse(strings.TrimRight(endpoint, "/"))
	if err != nil {
		return "", "", fmt.Errorf("parse s3 endpoint: %w", err)
	}
	if base.Scheme == "" || base.Host == "" {
		return "", "", errors.New("s3 endpoint must include scheme and host")
	}
	canonicalURI := strings.TrimRight(base.EscapedPath(), "/")
	if forcePathStyle {
		canonicalURI += "/" + url.PathEscape(bucket) + "/" + escapedKey
	} else {
		base.Host = bucket + "." + base.Host
		canonicalURI += "/" + escapedKey
	}
	decodedPath, err := url.PathUnescape(canonicalURI)
	if err != nil {
		return "", "", fmt.Errorf("decode s3 object path: %w", err)
	}
	base.Path = decodedPath
	base.RawPath = canonicalURI
	return base.String(), canonicalURI, nil
}

func readS3HTTPError(response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return &s3HTTPError{
		StatusCode: response.StatusCode,
		Status:     response.Status,
		Body:       strings.TrimSpace(string(body)),
	}
}

func isTransientS3Error(err error) bool {
	if err == nil {
		return false
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		return true
	}
	var httpError *s3HTTPError
	if errors.As(err, &httpError) {
		return httpError.StatusCode == http.StatusTooManyRequests || httpError.StatusCode >= 500
	}
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

func escapeS3Key(key string) string {
	parts := strings.Split(key, "/")
	for index, part := range parts {
		parts[index] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func s3Authorization(method, canonicalURI, canonicalQuery string, headers map[string]string, payloadHash, region, accessKeyID, secretAccessKey, shortDate, amzDate string) string {
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := "host:" + headers["host"] + "\n" +
		"x-amz-content-sha256:" + headers["x-amz-content-sha256"] + "\n" +
		"x-amz-date:" + headers["x-amz-date"] + "\n"
	canonicalRequest := strings.Join([]string{method, canonicalURI, canonicalQuery, canonicalHeaders, signedHeaders, payloadHash}, "\n")
	scope := shortDate + "/" + region + "/s3/aws4_request"
	requestHash := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(requestHash[:])
	signature := hex.EncodeToString(hmacSHA256(s3SigningKey(secretAccessKey, shortDate, region), []byte(stringToSign)))
	return "AWS4-HMAC-SHA256 Credential=" + accessKeyID + "/" + scope + ", SignedHeaders=" + signedHeaders + ", Signature=" + signature
}

func s3SigningKey(secretAccessKey, shortDate, region string) []byte {
	dateKey := hmacSHA256([]byte("AWS4"+secretAccessKey), []byte(shortDate))
	dateRegionKey := hmacSHA256(dateKey, []byte(region))
	dateRegionServiceKey := hmacSHA256(dateRegionKey, []byte("s3"))
	return hmacSHA256(dateRegionServiceKey, []byte("aws4_request"))
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}
