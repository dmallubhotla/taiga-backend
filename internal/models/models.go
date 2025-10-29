package models

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"gitea.deepak.science/deepak/trygo/internal/config"
	"gitea.deepak.science/deepak/trygo/internal/filerepo"
	"gitea.deepak.science/deepak/trygo/internal/store"
	"gitea.deepak.science/deepak/trygo/internal/tokens"
)

var ErrNoFileContent = errors.New("activity file has no file content")

type Model interface {
	Healthy(ctx context.Context) error
	Close() error
	CreateUser(ctx context.Context, req *CreateUserRequest) (*CreateUserResponse, error)
	VerifyUserByEmailPassword(ctx context.Context, email string, password string) (*UserNoPassword, error)
	Hat(ctx context.Context, hatID int32, userToken *tokens.UserToken) (*Hat, error)
	Hats(ctx context.Context, userToken *tokens.UserToken) ([]*Hat, error)
	AddHat(ctx context.Context, hat *Hat, userToken *tokens.UserToken) (*Hat, error)
	ActivityFile(ctx context.Context, id int32, userToken *tokens.UserToken) (*ActivityFile, error)
	ActivityFiles(ctx context.Context, userToken *tokens.UserToken) ([]*ActivityFile, error)
	AddActivityFile(ctx context.Context, fileReader io.Reader, timestamp time.Time, userToken *tokens.UserToken, fileRepo filerepo.FileRepo) (*ActivityFile, error)
	UpdateActivityFile(ctx context.Context, id int32, timestamp time.Time, fileRepoHash *string, userToken *tokens.UserToken) (*ActivityFile, error)
	DeleteActivityFile(ctx context.Context, id int32, userToken *tokens.UserToken) error
	GetActivityFileContent(ctx context.Context, id int32, userToken *tokens.UserToken, fileRepo filerepo.FileRepo) (io.ReadCloser, error)
}

type storeModel struct {
	store store.Store
}

func New(cfg *config.Config) (Model, error) {

	s, err := store.GetStore(cfg)
	if err != nil {
		if s != nil {
			s.Close()
		}
		return nil, fmt.Errorf("failed to initialize database :%w", err)
	}

	return &storeModel{
		store: s,
	}, nil
}

// mostly for convenience for testing
func NewFromStore(s store.Store) Model {
	return &storeModel{
		store: s,
	}
}

func (m *storeModel) Close() error {
	return m.store.Close()
}

func (m *storeModel) Healthy(ctx context.Context) error {
	return m.store.Healthy(ctx)
}
