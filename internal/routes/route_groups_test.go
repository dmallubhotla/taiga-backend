package routes_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/testsupport"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
)

func TestRouteGroupRoutes(t *testing.T) {
	srv, m, toker := newAPI(t)
	alice := newUserClient(t, srv, m, toker, "alice@example.com")
	bob := newUserClient(t, srv, m, toker, "bob@example.com")
	splits := []testsupport.Split{{Meters: 2000, Seconds: 600}}
	_, first := alice.uploadWorkout(testsupport.EncodeFIT(t, routeDay, splits))
	_, second := alice.uploadWorkout(testsupport.EncodeFIT(t, routeDay.AddDate(0, 0, 1), splits))
	_, elsewhere := alice.uploadWorkout(testsupport.EncodeFITHeading(t, routeDay.AddDate(0, 0, 2), splits, 180))

	status, body := alice.get("/routes")
	require.Equal(t, http.StatusOK, status, string(body))
	assert.JSONEq(t, `[]`, string(body), "detection runs in the background")

	var aliceID int32
	{
		status, body := alice.get("/jobs?state=queued")
		require.Equal(t, http.StatusOK, status)
		var queued []models.Job
		require.NoError(t, json.Unmarshal(body, &queued))
		require.Len(t, queued, 1, "one detection however many uploads")
		assert.Equal(t, models.JobDetectRoutes, queued[0].Kind)
		aliceID = queued[0].UserID
	}
	require.NoError(t, m.DetectRoutes(t.Context(), &tokens.UserToken{ID: aliceID}))

	status, body = alice.get("/routes")
	require.Equal(t, http.StatusOK, status, string(body))
	var routes []models.RouteSummary
	require.NoError(t, json.Unmarshal(body, &routes))
	require.Len(t, routes, 1)
	id := routes[0].ID
	assert.Equal(t, 2, routes[0].RunCount)

	status, body = alice.get(fmt.Sprintf("/routes/%d", id))
	require.Equal(t, http.StatusOK, status, string(body))
	var detail models.RouteDetail
	require.NoError(t, json.Unmarshal(body, &detail))
	require.Len(t, detail.Workouts, 2)
	assert.Equal(t, second, detail.Workouts[0].ID)
	assert.NotEmpty(t, detail.Track)

	status, body = alice.get(fmt.Sprintf("/workouts/%d/route", first))
	require.Equal(t, http.StatusOK, status, string(body))
	assert.Contains(t, string(body), fmt.Sprintf(`"id":%d`, id))
	status, _ = alice.get(fmt.Sprintf("/workouts/%d/route", elsewhere))
	assert.Equal(t, http.StatusNotFound, status, "a run matching nothing has no route")

	status, body = alice.put(fmt.Sprintf("/routes/%d", id), `{"name":"  Lakeshore  "}`)
	require.Equal(t, http.StatusOK, status, string(body))
	assert.Contains(t, string(body), `"name":"Lakeshore"`)
	status, body = alice.put(fmt.Sprintf("/routes/%d", id), `{"name":""}`)
	require.Equal(t, http.StatusOK, status, string(body))
	assert.Contains(t, string(body), `"name":null`)
	status, _ = alice.put(fmt.Sprintf("/routes/%d", id), fmt.Sprintf(`{"name":%q}`, strings.Repeat("x", 101)))
	assert.Equal(t, http.StatusBadRequest, status)

	status, _ = bob.get(fmt.Sprintf("/routes/%d", id))
	assert.Equal(t, http.StatusNotFound, status)
	status, _ = bob.put(fmt.Sprintf("/routes/%d", id), `{"name":"mine"}`)
	assert.Equal(t, http.StatusNotFound, status)
	status, _ = bob.get(fmt.Sprintf("/workouts/%d/route", first))
	assert.Equal(t, http.StatusNotFound, status)
	status, _ = alice.get("/routes/9999")
	assert.Equal(t, http.StatusNotFound, status)
}
