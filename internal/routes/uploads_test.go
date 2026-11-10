package routes_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/testsupport"
)

func (c *apiClient) post(path string) (int, []byte) {
	req, err := http.NewRequest(http.MethodPost, c.srv.URL+path, nil)
	require.NoError(c.t, err)
	return c.do(req)
}

func (c *apiClient) delete(path string) (int, []byte) {
	req, err := http.NewRequest(http.MethodDelete, c.srv.URL+path, nil)
	require.NoError(c.t, err)
	return c.do(req)
}

// uploadFile posts content to /activity_files and returns the status and the file it describes.
func (c *apiClient) uploadFile(content []byte) (int, models.ActivityFile) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", "upload.fit")
	require.NoError(c.t, err)
	_, err = part.Write(content)
	require.NoError(c.t, err)
	require.NoError(c.t, mw.Close())

	req, err := http.NewRequest(http.MethodPost, c.srv.URL+"/activity_files", &buf)
	require.NoError(c.t, err)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	status, body := c.do(req)
	var af models.ActivityFile
	require.NoError(c.t, json.Unmarshal(body, &af), string(body))
	return status, af
}

func TestUnusableFilesAre422WithAReason(t *testing.T) {
	srv, m, toker := newAPI(t)
	alice := newUserClient(t, srv, m, toker, "alice@example.com")

	for name, tc := range map[string]struct {
		content []byte
		reason  string
	}{
		"junk":        {[]byte("definitely not a FIT file"), "not a readable FIT file"},
		"multi-sport": {testsupport.EncodeMultiSessionFIT(t, routeDay, []testsupport.Split{{Meters: 1000, Seconds: 300}}, 3), "3 sessions"},
	} {
		t.Run(name, func(t *testing.T) {
			status, af := alice.uploadFile(tc.content)
			require.Equal(t, http.StatusCreated, status)

			status, body := alice.post(fmt.Sprintf("/workouts/from-activity-file/%d", af.ID))
			assert.Equal(t, http.StatusUnprocessableEntity, status)
			assert.Contains(t, string(body), tc.reason)

			status, body = alice.get("/activity_files?without_workout=true")
			require.Equal(t, http.StatusOK, status)
			var pending []models.ActivityFile
			require.NoError(t, json.Unmarshal(body, &pending))
			assert.Contains(t, idsOf(pending), af.ID)

			status, _ = alice.delete(fmt.Sprintf("/activity_files/%d", af.ID))
			assert.Equal(t, http.StatusNoContent, status)
		})
	}
}

func TestWorkoutFromMissingActivityFileIs404(t *testing.T) {
	srv, m, toker := newAPI(t)
	alice := newUserClient(t, srv, m, toker, "alice@example.com")
	bob := newUserClient(t, srv, m, toker, "bob@example.com")
	bobsFile, _ := bob.uploadWorkout(fourKFIT(t))

	status, _ := alice.post("/workouts/from-activity-file/9999")
	assert.Equal(t, http.StatusNotFound, status)
	status, _ = alice.post(fmt.Sprintf("/workouts/from-activity-file/%d", bobsFile))
	assert.Equal(t, http.StatusNotFound, status, "someone else's file")
}

func TestReuploadReturnsExistingWorkout(t *testing.T) {
	srv, m, toker := newAPI(t)
	alice := newUserClient(t, srv, m, toker, "alice@example.com")
	fit := fourKFIT(t)
	afID, workoutID := alice.uploadWorkout(fit)

	status, af := alice.uploadFile(fit)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, afID, af.ID)

	status, body := alice.post(fmt.Sprintf("/workouts/from-activity-file/%d", af.ID))
	assert.Equal(t, http.StatusOK, status)
	var w models.Workout
	require.NoError(t, json.Unmarshal(body, &w))
	assert.Equal(t, workoutID, w.ID)

	status, body = alice.get("/workouts")
	require.Equal(t, http.StatusOK, status)
	var all []models.Workout
	require.NoError(t, json.Unmarshal(body, &all))
	assert.Len(t, all, 1)
}

func TestDeleteActivityFileWithWorkoutIs409(t *testing.T) {
	srv, m, toker := newAPI(t)
	alice := newUserClient(t, srv, m, toker, "alice@example.com")
	afID, workoutID := alice.uploadWorkout(fourKFIT(t))

	status, body := alice.delete(fmt.Sprintf("/activity_files/%d", afID))
	assert.Equal(t, http.StatusConflict, status)
	assert.Contains(t, string(body), "delete them first")

	status, _ = alice.delete(fmt.Sprintf("/workouts/%d", workoutID))
	require.Equal(t, http.StatusNoContent, status)
	status, _ = alice.delete(fmt.Sprintf("/activity_files/%d", afID))
	assert.Equal(t, http.StatusNoContent, status)
}

func TestRecomputeSegmentRoutes(t *testing.T) {
	srv, m, toker := newAPI(t)
	alice := newUserClient(t, srv, m, toker, "alice@example.com")
	bob := newUserClient(t, srv, m, toker, "bob@example.com")
	_, workoutID := alice.uploadWorkout(fourKFIT(t))

	status, body := alice.post(fmt.Sprintf("/workouts/%d/segments/recompute", workoutID))
	require.Equal(t, http.StatusOK, status, string(body))
	var segments []models.WorkoutSegment
	require.NoError(t, json.Unmarshal(body, &segments))
	assert.Len(t, segments, 2, "1k and 1mi, same as at creation")

	status, _ = bob.post(fmt.Sprintf("/workouts/%d/segments/recompute", workoutID))
	assert.Equal(t, http.StatusNotFound, status)

	status, body = alice.post("/segments/recompute")
	require.Equal(t, http.StatusOK, status)
	var summary models.RecomputeSummary
	require.NoError(t, json.Unmarshal(body, &summary))
	assert.Equal(t, 1, summary.Recomputed)
	assert.Empty(t, summary.Failed)
	assert.Contains(t, string(body), `"failed":[]`)
}

func idsOf(files []models.ActivityFile) []int32 {
	ids := make([]int32, 0, len(files))
	for _, f := range files {
		ids = append(ids, f.ID)
	}
	return ids
}
