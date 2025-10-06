package models

import (
	"context"
	"fmt"
	"gitea.deepak.science/deepak/trygo/internal/db"
	"golang.org/x/crypto/bcrypt"
	"log"
)

type CreateUserRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

type CreateUserResponse struct {
	Email string `json:"email"`
	Id    int32  `json:"id"`
}

func (m *storeModel) CreateUser(ctx context.Context, req *CreateUserRequest) (*CreateUserResponse, error) {
	if req.Email == "" {
		return nil, fmt.Errorf("No email provided")
	}

	hashedPw, err := hashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("Hashing password failed!: %w", err)
	}

	params := &db.CreateUserParams{
		Email:       req.Email,
		DisplayName: req.DisplayName,
		Password:    hashedPw,
	}
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("Could not get a querier: %w", err)
		return nil, err
	}

	user, err := querier.CreateUser(ctx, params)
	if err != nil {
		log.Printf("Error creating user: %w", err)
		return nil, err
	}

	resp := &CreateUserResponse{
		Email: user.Email,
		Id:    user.ID,
	}
	return resp, nil

}

type UserNoPassword struct {
	ID          int32  `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

func noPassword(u *db.SelectEmailPasswordForAuthRow) *UserNoPassword {
	return &UserNoPassword{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
	}
}

// time equalise this method, needs to have a hash before every return in some way
func (m *storeModel) VerifyUserByEmailPassword(ctx context.Context, email string, password string) (*UserNoPassword, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		// may need to pad
		hashPassword(password)
		return nil, fmt.Errorf("Shouldn't have issue getting querier!: %w", err)
	}

	userWithPassword, err := querier.SelectEmailPasswordForAuth(ctx, email)
	if err != nil {
		hashPassword(password)
		return nil, fmt.Errorf("Couldn't select a user")
	}

	err = bcrypt.CompareHashAndPassword(userWithPassword.Password, []byte(password))
	if err != nil {
		return nil, fmt.Errorf("Error with compare: %w", err)
	}

	userNoPass := noPassword(userWithPassword)
	return userNoPass, nil

}

func hashPassword(password string) ([]byte, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 11)
	return bytes, err
}
