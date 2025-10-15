package models

import (
	"context"
	"gitea.deepak.science/deepak/trygo/internal/db"
	"log"
)

type Hat = db.Hat

func (m *storeModel) Hat(ctx context.Context, id int32, userID int32) (*Hat, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}
	getHatParams := &db.GetHatParams{
		ID:     id,
		UserID: &userID,
	}
	hat, err := querier.GetHat(ctx, getHatParams)
	if err != nil {
		log.Printf("Could not get hat: %v", err)
		return nil, err
	}

	return (*Hat)(hat), nil
}

func (m *storeModel) Hats(ctx context.Context, id int32, userID int32) (*Hat, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}
	getHatParams := &db.GetHatParams{
		ID:     id,
		UserID: &userID,
	}
	hat, err := querier.GetHat(ctx, getHatParams)
	if err != nil {
		log.Printf("Could not get hat: %v", err)
		return nil, err
	}

	return (*Hat)(hat), nil
}

// note that we will ignore fields like hat.UserID on the input field because
// only the authenticated user id should be used
func (m *storeModel) AddHat(ctx context.Context, hat *Hat, userID int32) (*Hat, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}

	createHatParams := &db.CreateHatParams{
		Name:        hat.Name,
		Description: hat.Description,
		UserID:      &userID,
	}
	returnHat, err := querier.CreateHat(ctx, createHatParams)
	if err != nil {
		log.Printf("Could not create hat: %v", err)
		return nil, err
	}
	return returnHat, nil

}
