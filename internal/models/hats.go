package models

import (
	"context"
	"gitea.deepak.science/deepak/trygo/internal/db"
	"gitea.deepak.science/deepak/trygo/internal/tokens"
	"log"
)

type Hat = db.Hat

func (m *storeModel) Hat(ctx context.Context, id int32, userToken *tokens.UserToken) (*Hat, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}
	getHatParams := &db.GetHatParams{
		ID:     id,
		UserID: &userToken.ID,
	}
	hat, err := querier.GetHat(ctx, getHatParams)
	if err != nil {
		log.Printf("Could not get hat: %v", err)
		return nil, err
	}

	return (*Hat)(hat), nil
}

func (m *storeModel) Hats(ctx context.Context, userToken *tokens.UserToken) ([]*Hat, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}
	hats, err := querier.ListHatsByUser(ctx, &userToken.ID)
	if err != nil {
		log.Printf("Could not get hats: %v", err)
		return nil, err
	}
	log.Printf("Got %d hats", len(hats))
	var retHats []*Hat
	for _, h := range hats {
		retHats = append(retHats, (*Hat)(h))
	}

	return retHats, nil
}

// note that we will ignore fields like hat.UserID on the input field because
// only the authenticated user id should be used
func (m *storeModel) AddHat(ctx context.Context, hat *Hat, userToken *tokens.UserToken) (*Hat, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}

	createHatParams := &db.CreateHatParams{
		Name:        hat.Name,
		Description: hat.Description,
		UserID:      &userToken.ID,
	}
	log.Printf("Adding hat %v", createHatParams)
	returnHat, err := querier.CreateHat(ctx, createHatParams)
	if err != nil {
		log.Printf("Could not create hat: %v", err)
		return nil, err
	}
	return returnHat, nil

}
