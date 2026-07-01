package cli

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/epoxsizer/kan/internal/config"
)

type realS3Uploader struct{}

var s3BackupHTTPClient = http.DefaultClient

type s3ListResult struct {
	Contents []struct {
		Key          string    `xml:"Key"`
		LastModified time.Time `xml:"LastModified"`
	} `xml:"Contents"`
	IsTruncated           bool   `xml:"IsTruncated"`
	NextContinuationToken string `xml:"NextContinuationToken"`
}

func (realS3Uploader) Upload(ctx context.Context, cfg config.S3Backup, sourcePath, key string) error {
	endpointURL, canonicalURI, err := s3PutURL(cfg, key)
	if err != nil {
		return err
	}
	file, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open backup for upload: %w", err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat backup for upload: %w", err)
	}

	payloadHash, err := hashReader(file)
	if err != nil {
		return fmt.Errorf("hash backup for upload: %w", err)
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind backup for upload: %w", err)
	}

	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	shortDate := now.Format("20060102")
	parsed, err := url.Parse(endpointURL)
	if err != nil {
		return fmt.Errorf("parse s3 endpoint: %w", err)
	}
	host := parsed.Host
	headers := map[string]string{
		"host":                 host,
		"x-amz-content-sha256": payloadHash,
		"x-amz-date":           amzDate,
	}
	authorization := s3Authorization("PUT", canonicalURI, "", headers, payloadHash, cfg.Region, cfg.AccessKeyID, cfg.SecretAccessKey, shortDate, amzDate)

	request, err := http.NewRequestWithContext(ctx, http.MethodPut, endpointURL, file)
	if err != nil {
		return fmt.Errorf("create s3 upload request: %w", err)
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("X-Amz-Content-Sha256", payloadHash)
	request.Header.Set("X-Amz-Date", amzDate)
	request.Header.Set("Content-Type", "application/octet-stream")
	request.ContentLength = stat.Size()
	response, err := s3BackupHTTPClient.Do(request)
	if err != nil {
		return fmt.Errorf("send s3 upload request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("s3 upload failed: %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func (realS3Uploader) Rotate(ctx context.Context, cfg config.S3Backup, retention time.Duration, now time.Time) (int, error) {
	if retention == 0 {
		return 0, nil
	}
	prefix := strings.Trim(cfg.Prefix, "/")
	if prefix != "" {
		prefix += "/"
	}
	removed := 0
	token := ""
	for {
		result, err := listS3BackupObjects(ctx, cfg, prefix, token)
		if err != nil {
			return removed, err
		}
		for _, object := range result.Contents {
			if !timestampedBackupPattern.MatchString(filepath.Base(object.Key)) || !object.LastModified.Before(now.Add(-retention)) {
				continue
			}
			if err = deleteS3BackupObject(ctx, cfg, object.Key); err != nil {
				return removed, err
			}
			removed++
		}
		if !result.IsTruncated || result.NextContinuationToken == "" {
			return removed, nil
		}
		token = result.NextContinuationToken
	}
}

func listS3BackupObjects(ctx context.Context, cfg config.S3Backup, prefix, token string) (s3ListResult, error) {
	endpoint, canonicalURI, err := s3PutURL(cfg, "")
	if err != nil {
		return s3ListResult{}, err
	}
	values := url.Values{"list-type": {"2"}, "prefix": {prefix}}
	if token != "" {
		values.Set("continuation-token", token)
	}
	query := strings.ReplaceAll(values.Encode(), "+", "%20")
	endpoint += "?" + query
	request, err := signedS3Request(ctx, http.MethodGet, endpoint, canonicalURI, query, cfg)
	if err != nil {
		return s3ListResult{}, err
	}
	response, err := s3BackupHTTPClient.Do(request)
	if err != nil {
		return s3ListResult{}, fmt.Errorf("list s3 backups: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return s3ListResult{}, fmt.Errorf("list s3 backups: %w", readS3HTTPError(response))
	}
	var result s3ListResult
	if err = xml.NewDecoder(response.Body).Decode(&result); err != nil {
		return s3ListResult{}, fmt.Errorf("decode s3 backup list: %w", err)
	}
	return result, nil
}

func deleteS3BackupObject(ctx context.Context, cfg config.S3Backup, key string) error {
	endpoint, canonicalURI, err := s3PutURL(cfg, key)
	if err != nil {
		return err
	}
	request, err := signedS3Request(ctx, http.MethodDelete, endpoint, canonicalURI, "", cfg)
	if err != nil {
		return err
	}
	response, err := s3BackupHTTPClient.Do(request)
	if err != nil {
		return fmt.Errorf("delete expired s3 backup: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("delete expired s3 backup: %w", readS3HTTPError(response))
	}
	return nil
}

func signedS3Request(ctx context.Context, method, endpoint, canonicalURI, canonicalQuery string, cfg config.S3Backup) (*http.Request, error) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	shortDate := now.Format("20060102")
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse s3 endpoint: %w", err)
	}
	payloadHash := hex.EncodeToString(sha256.New().Sum(nil))
	headers := map[string]string{"host": parsed.Host, "x-amz-content-sha256": payloadHash, "x-amz-date": amzDate}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create s3 backup rotation request: %w", err)
	}
	request.Header.Set("Authorization", s3Authorization(method, canonicalURI, canonicalQuery, headers, payloadHash, cfg.Region, cfg.AccessKeyID, cfg.SecretAccessKey, shortDate, amzDate))
	request.Header.Set("X-Amz-Content-Sha256", payloadHash)
	request.Header.Set("X-Amz-Date", amzDate)
	return request, nil
}

func s3PutURL(cfg config.S3Backup, key string) (string, string, error) {
	return s3ObjectURL(cfg.Bucket, cfg.Region, cfg.Endpoint, cfg.ForcePathStyle, key)
}

func escapeS3Key(key string) string {
	parts := strings.Split(key, "/")
	for index, part := range parts {
		parts[index] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func hashReader(reader io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
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
	mac.Write(data)
	return mac.Sum(nil)
}
