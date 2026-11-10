package models

import (
	"context"
	"io"
	"log"
	"time"

	"gitea.deepak.science/deepak/taiga/internal/db"
	"gitea.deepak.science/deepak/taiga/internal/filerepo"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"github.com/jackc/pgx/v5/pgtype"
)

type ActivityFile = db.ActivityFile

func (m *storeModel) ActivityFile(ctx context.Context, id int32, userToken *tokens.UserToken) (*ActivityFile, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}
	getActivityFileParams := &db.GetActivityFileParams{
		ID:     id,
		UserID: &userToken.ID,
	}
	activityFile, err := querier.GetActivityFile(ctx, getActivityFileParams)
	if err != nil {
		log.Printf("Could not get activity file: %v", err)
		return nil, err
	}

	return (*ActivityFile)(activityFile), nil
}

func (m *storeModel) ActivityFiles(ctx context.Context, userToken *tokens.UserToken) ([]*ActivityFile, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}
	activityFiles, err := querier.ListActivityFilesByUser(ctx, &userToken.ID)
	if err != nil {
		log.Printf("Could not get activity files: %v", err)
		return nil, err
	}
	log.Printf("Got %d activity files", len(activityFiles))
	var retActivityFiles []*ActivityFile
	for _, af := range activityFiles {
		log.Println(af)
		retActivityFiles = append(retActivityFiles, (*ActivityFile)(af))
	}

	return retActivityFiles, nil
}

func (m *storeModel) ActivityFilesWithoutWorkout(ctx context.Context, userToken *tokens.UserToken) ([]*ActivityFile, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}
	activityFiles, err := querier.ListActivityFilesWithoutWorkout(ctx, &userToken.ID)
	if err != nil {
		log.Printf("Could not get activity files without workouts: %v", err)
		return nil, err
	}
	return activityFiles, nil
}

func (m *storeModel) AddActivityFile(ctx context.Context, fileReader io.Reader, timestamp time.Time, userToken *tokens.UserToken, fileRepo filerepo.FileRepo) (*ActivityFile, bool, error) {
	// Storing is idempotent (content-addressed), and it's what gives us the hash to dedupe on.
	hash, err := fileRepo.Store(ctx, fileReader)
	if err != nil {
		log.Printf("Could not store file in filerepo: %v", err)
		return nil, false, err
	}

	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, false, err
	}

	createActivityFileParams := &db.CreateActivityFileParams{
		Timestamp:    pgtype.Timestamptz{Time: timestamp, Valid: true},
		FileRepoHash: &hash,
		UserID:       &userToken.ID,
	}
	log.Printf("Adding activity file %v", createActivityFileParams)
	created, err := querier.CreateActivityFile(ctx, createActivityFileParams)
	if err == nil {
		return created, true, nil
	}
	// ON CONFLICT DO NOTHING returns no row when the user already has this content, so concurrent re-uploads can't duplicate it.
	if !IsNotFound(err) {
		log.Printf("Could not create activity file: %v", err)
		return nil, false, err
	}
	existing, err := querier.GetActivityFileByHash(ctx, &db.GetActivityFileByHashParams{
		UserID:       &userToken.ID,
		FileRepoHash: &hash,
	})
	if err != nil {
		log.Printf("Could not look up activity file by hash: %v", err)
		return nil, false, err
	}
	log.Printf("Activity file with hash %s already uploaded as %d", hash, existing.ID)
	return existing, false, nil
}

func (m *storeModel) UpdateActivityFile(ctx context.Context, id int32, timestamp time.Time, fileRepoHash *string, userToken *tokens.UserToken) (*ActivityFile, error) {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return nil, err
	}

	// Convert time.Time to pgtype.Timestamptz
	pgTimestamp := pgtype.Timestamptz{
		Time:  timestamp,
		Valid: true,
	}

	updateActivityFileParams := &db.UpdateActivityFileParams{
		ID:           id,
		UserID:       &userToken.ID,
		Timestamp:    pgTimestamp,
		FileRepoHash: fileRepoHash,
	}
	log.Printf("Updating activity file %v", updateActivityFileParams)
	returnActivityFile, err := querier.UpdateActivityFile(ctx, updateActivityFileParams)
	if err != nil {
		log.Printf("Could not update activity file: %v", err)
		return nil, err
	}
	return returnActivityFile, nil
}

func (m *storeModel) DeleteActivityFile(ctx context.Context, id int32, userToken *tokens.UserToken) error {
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("could not get a querier: %v", err)
		return err
	}

	// Postgres would reject this with an FK violation and SQLite would leave the workouts pointing at nothing, so refuse up front either way.
	referencing, err := querier.GetWorkoutsByActivityFile(ctx, &db.GetWorkoutsByActivityFileParams{
		ActivityFileID: &id,
		UserID:         userToken.ID,
	})
	if err != nil {
		log.Printf("Could not check workouts for activity file %d: %v", id, err)
		return err
	}
	if len(referencing) > 0 {
		return ErrActivityFileInUse
	}

	deleteActivityFileParams := &db.DeleteActivityFileParams{
		ID:     id,
		UserID: &userToken.ID,
	}
	log.Printf("Deleting activity file %v", deleteActivityFileParams)
	err = querier.DeleteActivityFile(ctx, deleteActivityFileParams)
	if err != nil {
		log.Printf("Could not delete activity file: %v", err)
		return err
	}
	return nil
}

func (m *storeModel) GetActivityFileContent(ctx context.Context, id int32, userToken *tokens.UserToken, fileRepo filerepo.FileRepo) (io.ReadCloser, error) {
	// First get the activity file to get the hash
	activityFile, err := m.ActivityFile(ctx, id, userToken)
	if err != nil {
		return nil, err
	}

	if activityFile.FileRepoHash == nil {
		log.Printf("Activity file %d has no file_repo_hash", id)
		return nil, ErrNoFileContent
	}

	// Fetch from filerepo
	content, err := fileRepo.Fetch(ctx, *activityFile.FileRepoHash)
	if err != nil {
		log.Printf("Could not fetch file content from filerepo: %v", err)
		return nil, err
	}

	return content, nil
}
