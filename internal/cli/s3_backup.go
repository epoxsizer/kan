package cli

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/epoxsizer/kan/internal/config"
)

type realS3Uploader struct{}

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
	response, err := http.DefaultClient.Do(request)
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

func s3PutURL(cfg config.S3Backup, key string) (string, string, error) {
	escapedKey := escapeS3Key(key)
	if cfg.Endpoint == "" {
		if cfg.ForcePathStyle {
			return "https://s3." + cfg.Region + ".amazonaws.com/" + cfg.Bucket + "/" + escapedKey, "/" + cfg.Bucket + "/" + escapedKey, nil
		}
		return "https://" + cfg.Bucket + ".s3." + cfg.Region + ".amazonaws.com/" + escapedKey, "/" + escapedKey, nil
	}
	base, err := url.Parse(strings.TrimRight(cfg.Endpoint, "/"))
	if err != nil {
		return "", "", fmt.Errorf("parse s3 endpoint: %w", err)
	}
	if base.Scheme == "" || base.Host == "" {
		return "", "", fmt.Errorf("s3 endpoint must include scheme and host")
	}
	if cfg.ForcePathStyle {
		base.Path = strings.TrimRight(base.Path, "/") + "/" + cfg.Bucket + "/" + escapedKey
		return base.String(), base.EscapedPath(), nil
	}
	base.Host = cfg.Bucket + "." + base.Host
	base.Path = strings.TrimRight(base.Path, "/") + "/" + escapedKey
	return base.String(), base.EscapedPath(), nil
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
