package filerepo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"gitea.deepak.science/deepak/taiga/internal/config"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var _ FileRepo = &s3Store{}

type s3Store struct {
	client       *s3.Client
	bucket       string
	prefix       string
	prefixLength int
}

func newS3Store(cfg config.Config) (FileRepo, error) {
	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithRegion(cfg.FileRepo.S3Region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg)

	return &s3Store{
		client:       client,
		bucket:       cfg.FileRepo.S3Bucket,
		prefix:       cfg.FileRepo.S3Prefix,
		prefixLength: cfg.FileRepo.PrefixLength,
	}, nil
}

func (s *s3Store) Store(ctx context.Context, r io.Reader) (hash string, err error) {
	hasher := sha256.New()
	reader := io.TeeReader(r, hasher)

	content, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("issue reading content: %w", err)
	}

	hashBytes := hasher.Sum(nil)
	hash = hex.EncodeToString(hashBytes)

	key := s.buildKey(hash)

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(content),
	})
	if err != nil {
		return "", fmt.Errorf("error uploading to S3: %w", err)
	}

	return hash, nil
}

func (s *s3Store) Exists(ctx context.Context, hash string) (exists bool, err error) {
	key := s.buildKey(hash)

	_, err = s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, fmt.Errorf("error checking S3 object existence: %w", err)
	}

	return true, nil
}

func (s *s3Store) Fetch(ctx context.Context, hash string) (rc io.ReadCloser, err error) {
	key := s.buildKey(hash)

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("error fetching from S3: %w", err)
	}

	return result.Body, nil
}

func (s *s3Store) buildKey(hash string) string {
	parts := []string{}

	if s.prefix != "" {
		parts = append(parts, s.prefix)
	}

	if s.prefixLength > 0 && len(hash) > s.prefixLength {
		parts = append(parts, hash[:s.prefixLength])
		parts = append(parts, hash[s.prefixLength:])
	} else {
		parts = append(parts, hash)
	}

	return strings.Join(parts, "/")
}