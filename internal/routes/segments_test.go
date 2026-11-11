package routes_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/routes"
	"gitea.deepak.science/deepak/taiga/internal/testsupport"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
)

type apiClient struct {
	t     *testing.T
	srv   *httptest.Server
	token string
}

func newAPI(t *testing.T) (*httptest.Server, models.Model, tokens.Toker) {
	m, repo, toker := testsupport.NewModel(t), testsupport.NewFileRepo(t), testsupport.NewToker(t)
	srv := httptest.NewServer(routes.New(m, toker, repo))
	t.Cleanup(srv.Close)
	return srv, m, toker
}

func newUserClient(t *testing.T, srv *httptest.Server, m models.Model, toker tokens.Toker, email string) *apiClient {
	resp, err := m.CreateUser(t.Context(), &models.CreateUserRequest{Email: email, DisplayName: email, Password: "password123"})
	require.NoError(t, err)
	token, err := toker.EncodeUser(&tokens.UserToken{ID: resp.Id, Email: email})
	require.NoError(t, err)
	return &apiClient{t: t, srv: srv, token: token}
}

func (c *apiClient) do(req *http.Request) (int, []byte) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(c.t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(c.t, err)
	return resp.StatusCode, body
}

func (c *apiClient) get(path string) (int, []byte) {
	req, err := http.NewRequest(http.MethodGet, c.srv.URL+path, nil)
	require.NoError(c.t, err)
	return c.do(req)
}

func (c *apiClient) uploadWorkout(fit []byte) (activityFileID, workoutID int32) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", "run.fit")
	require.NoError(c.t, err)
	_, err = part.Write(fit)
	require.NoError(c.t, err)
	require.NoError(c.t, mw.Close())

	req, err := http.NewRequest(http.MethodPost, c.srv.URL+"/activity_files", &buf)
	require.NoError(c.t, err)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	status, body := c.do(req)
	require.Equal(c.t, http.StatusCreated, status, string(body))
	var af models.ActivityFile
	require.NoError(c.t, json.Unmarshal(body, &af))

	req, err = http.NewRequest(http.MethodPost, fmt.Sprintf("%s/workouts/from-activity-file/%d", c.srv.URL, af.ID), nil)
	require.NoError(c.t, err)
	status, body = c.do(req)
	require.Equal(c.t, http.StatusCreated, status, string(body))
	var w models.Workout
	require.NoError(c.t, json.Unmarshal(body, &w))
	return af.ID, w.ID
}

var routeDay = time.Date(2026, 11, 1, 7, 0, 0, 0, time.UTC)

func fourKFIT(t *testing.T) []byte {
	return testsupport.EncodeFIT(t, routeDay, []testsupport.Split{{Meters: 1000, Seconds: 300}, {Meters: 1000, Seconds: 200}, {Meters: 2000, Seconds: 600}})
}

func TestSegmentRoutes(t *testing.T) {
	srv, m, toker := newAPI(t)
	alice := newUserClient(t, srv, m, toker, "alice@example.com")
	bob := newUserClient(t, srv, m, toker, "bob@example.com")

	_, long := alice.uploadWorkout(fourKFIT(t))
	_, short := alice.uploadWorkout(testsupport.EncodeFIT(t, routeDay.AddDate(0, 0, 1), []testsupport.Split{{Meters: 1200, Seconds: 480}}))

	t.Run("standard distances", func(t *testing.T) {
		status, body := alice.get("/segments")
		require.Equal(t, http.StatusOK, status)
		var distances []struct {
			Name   string  `json:"name"`
			Meters float64 `json:"meters"`
		}
		require.NoError(t, json.Unmarshal(body, &distances))
		require.NotEmpty(t, distances)
		assert.Equal(t, "1k", distances[0].Name)
	})

	t.Run("workout segments", func(t *testing.T) {
		status, body := alice.get(fmt.Sprintf("/workouts/%d/segments", long))
		require.Equal(t, http.StatusOK, status)
		var segments []models.WorkoutSegment
		require.NoError(t, json.Unmarshal(body, &segments))
		require.Len(t, segments, 4, "1k, 1mi, 3k and 2mi")
		assert.Equal(t, "1k", segments[0].Name, "ordered by distance")
		assert.Equal(t, 200.0, segments[0].DurationSeconds)

		status, _ = bob.get(fmt.Sprintf("/workouts/%d/segments", long))
		assert.Equal(t, http.StatusNotFound, status)
	})

	t.Run("best efforts", func(t *testing.T) {
		status, body := alice.get("/segments/1k/best")
		require.Equal(t, http.StatusOK, status)
		var efforts []models.SegmentEffort
		require.NoError(t, json.Unmarshal(body, &efforts))
		require.Len(t, efforts, 2)
		assert.Equal(t, long, efforts[0].WorkoutID)
		assert.True(t, efforts[0].WorkoutStartTime.Time.Equal(routeDay))

		status, body = bob.get("/segments/1k/best")
		require.Equal(t, http.StatusOK, status)
		assert.JSONEq(t, "[]", string(body))

		status, _ = alice.get("/segments/4k/best")
		assert.Equal(t, http.StatusNotFound, status)
		for _, limit := range []string{"0", "101", "x"} {
			status, _ = alice.get("/segments/1k/best?limit=" + limit)
			assert.Equal(t, http.StatusBadRequest, status, "limit=%s", limit)
		}
		status, body = alice.get("/segments/1k/best?limit=1")
		require.Equal(t, http.StatusOK, status)
		require.NoError(t, json.Unmarshal(body, &efforts))
		assert.Len(t, efforts, 1)
	})

	t.Run("compare", func(t *testing.T) {
		status, body := alice.get(fmt.Sprintf("/workouts/compare?ids=%d,%d,%d", short, long, short))
		require.Equal(t, http.StatusOK, status, string(body))
		var resp struct {
			Distances []string `json:"distances"`
			Workouts  []struct {
				Workout  models.Workout                   `json:"workout"`
				Segments map[string]models.WorkoutSegment `json:"segments"`
			} `json:"workouts"`
		}
		require.NoError(t, json.Unmarshal(body, &resp))
		assert.Equal(t, []string{"1k", "1mi", "3k", "2mi"}, resp.Distances)
		require.Len(t, resp.Workouts, 2, "duplicate ids collapse")
		assert.Equal(t, short, resp.Workouts[0].Workout.ID)
		assert.NotContains(t, resp.Workouts[0].Segments, "1mi")
		assert.Contains(t, resp.Workouts[1].Segments, "1mi")

		for _, q := range []string{"", "?ids=", "?ids=1,x", "?ids=1,2,3,4,5,6,7,8,9,10,11"} {
			status, _ = alice.get("/workouts/compare" + q)
			assert.Equal(t, http.StatusBadRequest, status, "query %q", q)
		}
		status, _ = bob.get(fmt.Sprintf("/workouts/compare?ids=%d", long))
		assert.Equal(t, http.StatusNotFound, status)
	})

	t.Run("requires auth", func(t *testing.T) {
		anon := &apiClient{t: t, srv: srv}
		for _, path := range []string{"/segments", "/segments/1k/best", fmt.Sprintf("/workouts/%d/segments", long), "/workouts/compare?ids=1"} {
			status, _ := anon.get(path)
			assert.Equal(t, http.StatusUnauthorized, status, path)
		}
	})
}

func TestDurationSegmentRoutes(t *testing.T) {
	srv, m, toker := newAPI(t)
	alice := newUserClient(t, srv, m, toker, "alice@example.com")

	// 50 minutes with a fast 30 in the middle, and a steady 40.
	_, surge := alice.uploadWorkout(testsupport.EncodeFIT(t, routeDay, []testsupport.Split{{Meters: 1000, Seconds: 600}, {Meters: 6000, Seconds: 1800}, {Meters: 1000, Seconds: 600}}))
	_, steady := alice.uploadWorkout(testsupport.EncodeFIT(t, routeDay.AddDate(0, 0, 1), []testsupport.Split{{Meters: 6000, Seconds: 2400}}))

	t.Run("listed with their kind", func(t *testing.T) {
		status, body := alice.get("/segments")
		require.Equal(t, http.StatusOK, status)
		var segments []struct {
			Name    string  `json:"name"`
			Kind    string  `json:"kind"`
			Meters  float64 `json:"meters"`
			Seconds float64 `json:"seconds"`
		}
		require.NoError(t, json.Unmarshal(body, &segments))
		assert.Equal(t, "distance", segments[0].Kind)
		last := segments[len(segments)-1]
		assert.Equal(t, "1h", last.Name)
		assert.Equal(t, "duration", last.Kind)
		assert.Equal(t, 3600.0, last.Seconds)
	})

	t.Run("workout segments put durations after distances", func(t *testing.T) {
		status, body := alice.get(fmt.Sprintf("/workouts/%d/segments", surge))
		require.Equal(t, http.StatusOK, status)
		var segments []models.WorkoutSegment
		require.NoError(t, json.Unmarshal(body, &segments))
		last := segments[len(segments)-1]
		assert.Equal(t, "30min", last.Name, "an hour is longer than the run")
		assert.Equal(t, "duration", last.Kind)
		assert.InDelta(t, 6000, last.DistanceMeters, 1e-6, "the fast middle")
		assert.InDelta(t, 600, last.StartOffsetSeconds, 1e-6)
	})

	t.Run("best ranks by distance", func(t *testing.T) {
		status, body := alice.get("/segments/30min/best")
		require.Equal(t, http.StatusOK, status)
		var efforts []models.SegmentEffort
		require.NoError(t, json.Unmarshal(body, &efforts))
		require.Len(t, efforts, 2)
		assert.Equal(t, surge, efforts[0].WorkoutID)
		assert.Equal(t, steady, efforts[1].WorkoutID)
		assert.InDelta(t, 4500, efforts[1].DistanceMeters, 1e-6)

		status, body = alice.get("/segments/1h/best")
		require.Equal(t, http.StatusOK, status)
		assert.JSONEq(t, "[]", string(body))
	})

	t.Run("compare lists durations after distances", func(t *testing.T) {
		status, body := alice.get(fmt.Sprintf("/workouts/compare?ids=%d,%d", surge, steady))
		require.Equal(t, http.StatusOK, status, string(body))
		var resp struct {
			Distances []string `json:"distances"`
		}
		require.NoError(t, json.Unmarshal(body, &resp))
		assert.Equal(t, "30min", resp.Distances[len(resp.Distances)-1])
	})
}

func TestDownloadReturnsFullFile(t *testing.T) {
	srv, m, toker := newAPI(t)
	alice := newUserClient(t, srv, m, toker, "alice@example.com")
	fit := fourKFIT(t)
	afID, _ := alice.uploadWorkout(fit)

	status, body := alice.get(fmt.Sprintf("/activity_files/%d/download", afID))
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, fit, body)
}

func TestTimeWindowParams(t *testing.T) {
	srv, m, toker := newAPI(t)
	alice := newUserClient(t, srv, m, toker, "alice@example.com")
	_, earlier := alice.uploadWorkout(fourKFIT(t))
	_, later := alice.uploadWorkout(testsupport.EncodeFIT(t, routeDay.AddDate(1, 0, 0), []testsupport.Split{{Meters: 1200, Seconds: 480}}))
	from := routeDay.AddDate(0, 0, 1).Format(time.DateOnly)

	listed := func(path string) []int32 {
		status, body := alice.get(path)
		require.Equal(t, http.StatusOK, status, string(body))
		var ws []models.Workout
		require.NoError(t, json.Unmarshal(body, &ws))
		ids := make([]int32, 0, len(ws))
		for _, w := range ws {
			ids = append(ids, w.ID)
		}
		return ids
	}
	assert.Equal(t, []int32{later, earlier}, listed("/workouts"))
	assert.Equal(t, []int32{later}, listed("/workouts?from="+from))
	assert.Equal(t, []int32{earlier}, listed("/workouts?to="+url.QueryEscape(routeDay.Add(time.Hour).Format(time.RFC3339))))
	assert.Equal(t, []int32{earlier}, listed("/workouts?limit=1&offset=1"))

	status, body := alice.get("/workouts/totals?from=" + from)
	require.Equal(t, http.StatusOK, status, string(body))
	var totals models.WorkoutTotals
	require.NoError(t, json.Unmarshal(body, &totals))
	require.Len(t, totals.Sports, 1)
	assert.Equal(t, int64(1), totals.Sports[0].Workouts)
	assert.Contains(t, string(body), `"first_start_time":`)

	status, body = alice.get("/segments/1k/best?to=" + from)
	require.Equal(t, http.StatusOK, status, string(body))
	var efforts []models.SegmentEffort
	require.NoError(t, json.Unmarshal(body, &efforts))
	require.Len(t, efforts, 1)
	assert.Equal(t, earlier, efforts[0].WorkoutID)

	for _, bad := range []string{
		"/workouts?from=yesterday",
		"/workouts?from=2026-02-01&to=2026-01-01",
		"/workouts?limit=0",
		"/workouts?limit=1001",
		"/workouts?offset=-1",
		"/workouts/totals?to=soon",
		"/segments/1k/best?from=nope",
	} {
		status, _ := alice.get(bad)
		assert.Equal(t, http.StatusBadRequest, status, bad)
	}
}
