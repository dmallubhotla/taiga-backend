package filerepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"gitea.deepak.science/deepak/trygo/internal/config"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// stealing a lot from https://github.com/networkteam/filestore/blob/main/filestore.go

type FileRepo interface {
	// returns a hash to use to retrieve later
	Store(ctx context.Context, r io.Reader) (hash string, err error)
	Exists(ctx context.Context, hash string) (exists bool, err error)
	Fetch(ctx context.Context, hash string) (rc io.ReadCloser, err error)
}

var _ FileRepo = &localStore{}

type localStore struct {
	assetPath    string
	prefixLength int
}

func NewFileRepo(cfg config.Config) FileRepo {
	return &localStore{assetPath: cfg.FileRepo.AssetPath, prefixLength: cfg.FileRepo.PrefixLength}
}

func (f *localStore) Store(ctx context.Context, r io.Reader) (hash string, err error) {

	hasher := sha256.New()

	reader := io.TeeReader(r, hasher)

	content, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("Issue reading content: %w", err)
	}

	hashBytes := hasher.Sum(nil)
	hash = hex.EncodeToString(hashBytes)
	outputPath, err := f.outputPath(hash)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(outputPath, content, 0644); err != nil {
		return "", fmt.Errorf("Error writing to outputfile: %w", err)
	}
	return hash, nil
}

// returns this store's output path from the hash
func (f *localStore) outputPath(hash string) (string, error) {

	dir := filepath.Join(f.assetPath, hash[:f.prefixLength])
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("issue making dirs: %w", err)
	}
	return filepath.Join(dir, hash[f.prefixLength:]), nil

}

func (f *localStore) Exists(ctx context.Context, hash string) (exists bool, err error) {
	outputPath, err := f.outputPath(hash)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(outputPath)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("Generic stat file error: %w", err)
	} else {
		return true, nil
	}
}

func (f *localStore) Fetch(ctx context.Context, hash string) (rc io.ReadCloser, err error) {
	outputPath, err := f.outputPath(hash)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(outputPath)
	if err != nil {
		return nil, fmt.Errorf("Issue opening file: %w", err)
	}
	return file, nil
}
