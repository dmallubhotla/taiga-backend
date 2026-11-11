package routes_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/testsupport"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"gitea.deepak.science/deepak/taiga/internal/workouts"
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
		"junk":        {[]byte("definitely not a FIT file"), "not a readable activity file"},
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

func (c *apiClient) put(path, body string) (int, []byte) {
	req, err := http.NewRequest(http.MethodPut, c.srv.URL+path, strings.NewReader(body))
	require.NoError(c.t, err)
	req.Header.Set("Content-Type", "application/json")
	return c.do(req)
}

func TestReprocessRoutes(t *testing.T) {
	srv, m, toker := newAPI(t)
	alice := newUserClient(t, srv, m, toker, "alice@example.com")
	bob := newUserClient(t, srv, m, toker, "bob@example.com")
	_, workoutID := alice.uploadWorkout(fourKFIT(t))

	status, body := alice.post(fmt.Sprintf("/workouts/%d/reprocess", workoutID))
	require.Equal(t, http.StatusOK, status, string(body))
	var workout models.Workout
	require.NoError(t, json.Unmarshal(body, &workout))
	assert.Equal(t, workouts.ProcessorVersion, workout.ProcessorVersion)

	status, _ = bob.post(fmt.Sprintf("/workouts/%d/reprocess", workoutID))
	assert.Equal(t, http.StatusNotFound, status)

	status, body = alice.post("/workouts/reprocess")
	require.Equal(t, http.StatusAccepted, status, string(body))
	assert.JSONEq(t, `{"enqueued":1}`, string(body))

	status, body = alice.get("/jobs?state=queued")
	require.Equal(t, http.StatusOK, status, string(body))
	var all []models.Job
	require.NoError(t, json.Unmarshal(body, &all))
	var queued []models.Job
	for _, j := range all {
		// Uploading and reprocessing also queued route detection.
		if j.Kind == models.JobProcessWorkout {
			queued = append(queued, j)
		}
	}
	require.Len(t, queued, 1)
	assert.Equal(t, workoutID, *queued[0].WorkoutID)

	status, _ = alice.get(fmt.Sprintf("/jobs/%d", queued[0].ID))
	assert.Equal(t, http.StatusOK, status)
	status, _ = bob.get(fmt.Sprintf("/jobs/%d", queued[0].ID))
	assert.Equal(t, http.StatusNotFound, status)
	status, body = alice.post(fmt.Sprintf("/jobs/%d/retry", queued[0].ID))
	assert.Equal(t, http.StatusConflict, status, "only dead jobs retry")
	assert.Contains(t, string(body), "dead")

	status, body = bob.get("/jobs")
	require.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `[]`, string(body))
	status, _ = alice.get("/jobs?state=bogus")
	assert.Equal(t, http.StatusBadRequest, status)
}

func TestFileBackedWorkoutsAreNotEditable(t *testing.T) {
	srv, m, toker := newAPI(t)
	alice := newUserClient(t, srv, m, toker, "alice@example.com")
	_, workoutID := alice.uploadWorkout(fourKFIT(t))

	status, body := alice.put(fmt.Sprintf("/workouts/%d", workoutID), `{"distance_miles": 3}`)
	assert.Equal(t, http.StatusUnprocessableEntity, status)
	assert.Contains(t, string(body), "reprocess")

	status, body = alice.post(fmt.Sprintf("/workouts/%d/reprocess", workoutID))
	require.Equal(t, http.StatusOK, status, string(body))
}

func idsOf(files []models.ActivityFile) []int32 {
	ids := make([]int32, 0, len(files))
	for _, f := range files {
		ids = append(ids, f.ID)
	}
	return ids
}

func TestWorkoutSeries(t *testing.T) {
	srv, m, toker := newAPI(t)
	alice := newUserClient(t, srv, m, toker, "alice@example.com")
	bob := newUserClient(t, srv, m, toker, "bob@example.com")
	_, workoutID := alice.uploadWorkout(fourKFIT(t))

	status, body := alice.get(fmt.Sprintf("/workouts/%d/series", workoutID))
	require.Equal(t, http.StatusOK, status, string(body))
	var series struct {
		TimeSeconds    []float64  `json:"time_s"`
		DistanceMeters []float64  `json:"distance_m"`
		HeartRate      []float64  `json:"heart_rate"`
		AltitudeMeters []float64  `json:"altitude_m"`
		Latitude       []*float64 `json:"lat"`
		Longitude      []*float64 `json:"lon"`
		SpeedMPS       []*float64 `json:"speed_mps"`
	}
	require.NoError(t, json.Unmarshal(body, &series))
	require.Len(t, series.TimeSeconds, 401, "a record every 10m over 4km, plus the start")
	assert.Equal(t, 4000.0, series.DistanceMeters[400])
	assert.Equal(t, 1100.0, series.TimeSeconds[400])
	assert.Equal(t, float64(testsupport.RecordHeartRate), series.HeartRate[200])
	assert.InDelta(t, testsupport.RecordAltitude(4000), series.AltitudeMeters[400], 0.2)
	require.NotNil(t, series.Latitude[0])
	assert.InDelta(t, testsupport.StartLatitude, *series.Latitude[0], 1e-5)
	assert.Nil(t, series.SpeedMPS, "the synthetic file records no speed, so the column is omitted")

	status, _ = bob.get(fmt.Sprintf("/workouts/%d/series", workoutID))
	assert.Equal(t, http.StatusNotFound, status)

	manual, err := m.AddWorkout(t.Context(), &models.Workout{}, &tokens.UserToken{ID: 1})
	require.NoError(t, err)
	status, body = alice.get(fmt.Sprintf("/workouts/%d/series", manual.ID))
	assert.Equal(t, http.StatusNotFound, status)
	assert.Contains(t, string(body), "entered by hand")
}

func TestEndomondoUploads(t *testing.T) {
	srv, m, toker := newAPI(t)
	alice := newUserClient(t, srv, m, toker, "alice@example.com")
	splits := []testsupport.Split{{Meters: 2000, Seconds: 1200}}

	_, walkID := alice.uploadWorkout(testsupport.NewEndomondo(routeDay, splits, "WALKING").Encode(t))
	status, body := alice.get(fmt.Sprintf("/workouts/%d", walkID))
	require.Equal(t, http.StatusOK, status, string(body))
	var walk models.Workout
	require.NoError(t, json.Unmarshal(body, &walk))
	assert.Equal(t, "walking", walk.Sport)
	assert.Empty(t, walk.ExcludedReason)
	assert.Equal(t, "Endomondo", walk.Source)
	status, body = alice.get(fmt.Sprintf("/workouts/%d/segments", walkID))
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(body), `"name":"1k"`)
	status, _ = alice.get(fmt.Sprintf("/workouts/%d/series", walkID))
	assert.Equal(t, http.StatusOK, status)

	manual := testsupport.NewEndomondo(routeDay.AddDate(0, 0, 1), splits, "WALKING")
	manual.Source, manual.Points = "INPUT_MANUAL", nil
	_, manualID := alice.uploadWorkout(manual.Encode(t))
	status, body = alice.get(fmt.Sprintf("/workouts/%d", manualID))
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(body), `"excluded_reason":"hand-entered"`)
	status, body = alice.get(fmt.Sprintf("/workouts/%d/segments", manualID))
	require.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `[]`, string(body))

	status, body = alice.get("/segments/1k/best?sport=walking")
	require.Equal(t, http.StatusOK, status)
	var best []models.SegmentEffort
	require.NoError(t, json.Unmarshal(body, &best))
	require.Len(t, best, 1, "the hand-entered walk doesn't count")
	assert.Equal(t, walkID, best[0].WorkoutID)
}

func TestGoogleFitUploads(t *testing.T) {
	srv, m, toker := newAPI(t)
	alice := newUserClient(t, srv, m, toker, "alice@example.com")
	splits := []testsupport.Split{{Meters: 2000, Seconds: 600}}

	_, runID := alice.uploadWorkout(testsupport.NewTCX(routeDay, splits, "Running").Encode(t))
	status, body := alice.get(fmt.Sprintf("/workouts/%d", runID))
	require.Equal(t, http.StatusOK, status, string(body))
	var run models.Workout
	require.NoError(t, json.Unmarshal(body, &run))
	assert.Equal(t, "running", run.Sport)
	assert.Equal(t, "Google Fit", run.Source)
	assert.Empty(t, run.ExcludedReason)

	_, mixedID := alice.uploadWorkout(testsupport.NewTCX(routeDay.AddDate(0, 0, 1), splits, "Walking", "Running").Encode(t))
	status, body = alice.get(fmt.Sprintf("/workouts/%d", mixedID))
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(body), `"excluded_reason":"mixed sports"`)

	status, body = alice.get("/segments/1k/best")
	require.Equal(t, http.StatusOK, status)
	var best []models.SegmentEffort
	require.NoError(t, json.Unmarshal(body, &best))
	require.Len(t, best, 1, "the mixed session doesn't count")
	assert.Equal(t, runID, best[0].WorkoutID)
}
