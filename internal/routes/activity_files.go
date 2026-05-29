package routes

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"gitea.deepak.science/deepak/taiga/internal/filerepo"
	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"gitea.deepak.science/deepak/taiga/internal/workouts"
	"github.com/go-chi/chi/v5"
)

func newActivityFileRouter(m models.Model, fileRepo filerepo.FileRepo) http.Handler {
	router := chi.NewRouter()
	router.Get("/", getActivityFilesFunc(m))
	router.Get("/{activityfileid}", getActivityFileFunc(m))
	router.Get("/{activityfileid}/download", downloadActivityFileFunc(m, fileRepo))
	router.Post("/", postActivityFileFunc(m, fileRepo))
	router.Put("/{activityfileid}", putActivityFileFunc(m))
	router.Delete("/{activityfileid}", deleteActivityFileFunc(m))

	return router
}

func getActivityFileFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}

		activityFileID64, err := strconv.ParseInt(chi.URLParam(r, "activityfileid"), 10, 32)
		if err != nil {
			notFoundHandler(w, r)
			return
		}
		activityFileID := int32(activityFileID64)

		activityFile, err := m.ActivityFile(ctx, activityFileID, userToken)
		if err != nil {
			notFoundHandler(w, r)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(activityFile); err != nil {
			serverError(w, err)
		}
	}
}

func getActivityFilesFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			log.Printf("Got error %v", err)
			unauthorizedHandler(w, r)
			return
		}

		activityFiles, err := m.ActivityFiles(ctx, userToken)
		if err != nil {
			notFoundHandler(w, r)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(activityFiles); err != nil {
			serverError(w, err)
		}
	}
}

func postActivityFileFunc(m models.Model, fileRepo filerepo.FileRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}

		// Parse multipart form with max memory of 32MB
		err = r.ParseMultipartForm(32 << 20)
		if err != nil {
			badRequestError(w, fmt.Errorf("failed to parse multipart form: %w", err))
			return
		}

		// Get the file from the form
		file, fileHeader, err := r.FormFile("file")
		if err != nil {
			badRequestError(w, fmt.Errorf("failed to get file from form: %w", err))
			return
		}
		defer func() { _ = file.Close() }()

		log.Printf("Uploading file: %s", fileHeader.Filename)

		// Get timestamp from form, default to now if not provided
		timestampStr := r.FormValue("timestamp")
		var timestamp time.Time
		if timestampStr == "" {
			timestamp = time.Now()
		} else {
			timestamp, err = time.Parse(time.RFC3339, timestampStr)
			if err != nil {
				badRequestError(w, fmt.Errorf("invalid timestamp format, use RFC3339: %w", err))
				return
			}
		}

		log.Printf("Adding activity file for timestamp %v", timestamp)
		activityFile, err := m.AddActivityFile(ctx, file, timestamp, userToken, fileRepo)
		if err != nil {
			log.Printf("Error adding activity file! %v", err)
			serverError(w, err)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(activityFile); err != nil {
			serverError(w, err)
		}
	}
}

func putActivityFileFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}

		activityFileID64, err := strconv.ParseInt(chi.URLParam(r, "activityfileid"), 10, 32)
		if err != nil {
			notFoundHandler(w, r)
			return
		}
		activityFileID := int32(activityFileID64)

		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()

		var updateReq struct {
			Timestamp    *time.Time `json:"timestamp"`
			FileRepoHash *string    `json:"file_repo_hash"`
		}
		err = dec.Decode(&updateReq)
		if err != nil {
			badRequestError(w, err)
			return
		}

		// Get current activity file to get existing values
		currentFile, err := m.ActivityFile(ctx, activityFileID, userToken)
		if err != nil {
			notFoundHandler(w, r)
			return
		}

		// Use provided values or keep existing ones
		timestamp := time.Now()
		if updateReq.Timestamp != nil {
			timestamp = *updateReq.Timestamp
		} else if currentFile.Timestamp.Valid {
			timestamp = currentFile.Timestamp.Time
		}

		fileRepoHash := updateReq.FileRepoHash
		if fileRepoHash == nil {
			fileRepoHash = currentFile.FileRepoHash
		}

		log.Printf("Updating activity file %v", activityFileID)
		activityFile, err := m.UpdateActivityFile(ctx, activityFileID, timestamp, fileRepoHash, userToken)
		if err != nil {
			log.Printf("Error updating activity file! %v", err)
			serverError(w, err)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(activityFile); err != nil {
			serverError(w, err)
		}
	}
}

func deleteActivityFileFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}

		activityFileID64, err := strconv.ParseInt(chi.URLParam(r, "activityfileid"), 10, 32)
		if err != nil {
			notFoundHandler(w, r)
			return
		}
		activityFileID := int32(activityFileID64)

		log.Printf("Deleting activity file %v", activityFileID)
		err = m.DeleteActivityFile(ctx, activityFileID, userToken)
		if err != nil {
			log.Printf("Error deleting activity file! %v", err)
			serverError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func downloadActivityFileFunc(m models.Model, fileRepo filerepo.FileRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		userToken, err := tokens.UserTokenFromContext(ctx)
		if err != nil {
			unauthorizedHandler(w, r)
			return
		}

		activityFileID64, err := strconv.ParseInt(chi.URLParam(r, "activityfileid"), 10, 32)
		if err != nil {
			notFoundHandler(w, r)
			return
		}
		activityFileID := int32(activityFileID64)

		content, err := m.GetActivityFileContent(ctx, activityFileID, userToken, fileRepo)
		if err != nil {
			if err == models.ErrNoFileContent {
				notFoundHandler(w, r)
				return
			}
			log.Printf("Error getting activity file content! %v", err)
			serverError(w, err)
			return
		}
		defer func() { _ = content.Close() }()

		log.Println("retrieved content from repo")
		run, err := workouts.ReadFitFile(content)
		if err != nil {
			log.Printf("Error reading fit file")
			// for now don't care, continue.
			// TODO error handling here once we've decided what the interface is like
		} else {
			log.Printf("Got run with summary %+v", run.Summary)
			log.Printf("%+v", workouts.FastestSegments(run, []float64{1609.344, 5000, 8046.72, 10000}))
		}

		// Set appropriate headers for file download
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"activity_file_%d\"", activityFileID))

		// Copy file content to response
		_, err = io.Copy(w, content)
		if err != nil {
			log.Printf("Error copying file content to response: %v", err)
		}
	}
}
