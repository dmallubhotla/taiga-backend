package jobs_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitea.deepak.science/deepak/taiga/internal/filerepo"
	"gitea.deepak.science/deepak/taiga/internal/jobs"
	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/testsupport"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"gitea.deepak.science/deepak/taiga/internal/workouts"
)

var (
	start = time.Date(2026, 11, 1, 7, 0, 0, 0, time.UTC)
	fourK = []testsupport.Split{{Meters: 1000, Seconds: 300}, {Meters: 1000, Seconds: 200}, {Meters: 2000, Seconds: 600}}
)

// clock is a settable time for the worker, so retries and leases don't need real waiting.
type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }
func newWorker(m models.Model, repo filerepo.FileRepo) (*jobs.Worker, *clock) {
	c := &clock{t: start}
	w := jobs.NewWorker(m, repo)
	w.Now = c.now
	return w, c
}

func createUser(t *testing.T, m models.Model, email string) *tokens.UserToken {
	t.Helper()
	resp, err := m.CreateUser(t.Context(), &models.CreateUserRequest{Email: email, DisplayName: email, Password: "password123"})
	require.NoError(t, err)
	return &tokens.UserToken{ID: resp.Id, Email: email}
}

// legacyWorkout is a workout linked to a file but never processed, like one from before processor versions existed.
func legacyWorkout(t *testing.T, m models.Model, repo filerepo.FileRepo, user *tokens.UserToken, content []byte) *models.Workout {
	t.Helper()
	af, _, err := m.AddActivityFile(t.Context(), bytes.NewReader(content), start, user, repo)
	require.NoError(t, err)
	w, err := m.AddWorkout(t.Context(), &models.Workout{StartTime: pgtype.Timestamptz{Time: start, Valid: true}, ActivityFileID: &af.ID}, user)
	require.NoError(t, err)
	return w
}

func onlyJob(t *testing.T, m models.Model, user *tokens.UserToken) *models.Job {
	t.Helper()
	all, err := m.Jobs(t.Context(), "", 10, user)
	require.NoError(t, err)
	require.Len(t, all, 1)
	return all[0]
}

func TestStaleWorkoutsAreReprocessed(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")
	stale := legacyWorkout(t, m, repo, alice, testsupport.EncodeFIT(t, start, fourK))
	_, err := m.AddWorkout(ctx, &models.Workout{StartTime: pgtype.Timestamptz{Time: start, Valid: true}}, alice)
	require.NoError(t, err)
	w, c := newWorker(m, repo)

	n, err := m.EnqueueStaleWorkouts(ctx, c.now())
	require.NoError(t, err)
	assert.EqualValues(t, 1, n, "only the file-backed workout; the hand-entered one has nothing to reprocess")
	n, err = m.EnqueueStaleWorkouts(ctx, c.now())
	require.NoError(t, err)
	assert.Zero(t, n, "already queued")

	found, err := w.RunOnce(ctx)
	require.NoError(t, err)
	require.True(t, found)

	job := onlyJob(t, m, alice)
	assert.Equal(t, models.JobDone, job.State)
	assert.Equal(t, stale.ID, *job.WorkoutID)
	got, err := m.Workout(ctx, stale.ID, alice)
	require.NoError(t, err)
	assert.Equal(t, workouts.ProcessorVersion, got.ProcessorVersion)
	segments, err := m.WorkoutSegments(ctx, stale.ID, alice)
	require.NoError(t, err)
	assert.NotEmpty(t, segments)

	found, err = w.RunOnce(ctx)
	require.NoError(t, err)
	assert.False(t, found)
	n, err = m.EnqueueStaleWorkouts(ctx, c.now())
	require.NoError(t, err)
	assert.Zero(t, n, "now current")
}

func TestUnreadableFileGoesStraightToDead(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")
	legacyWorkout(t, m, repo, alice, []byte("not a FIT file"))
	w, c := newWorker(m, repo)
	_, err := m.EnqueueReprocessAll(ctx, c.now(), alice)
	require.NoError(t, err)

	_, err = w.RunOnce(ctx)
	require.NoError(t, err)
	job := onlyJob(t, m, alice)
	assert.Equal(t, models.JobDead, job.State)
	assert.EqualValues(t, 1, job.Attempts)
	require.NotNil(t, job.LastError)
	assert.Contains(t, *job.LastError, workouts.ErrUnreadableFile.Error())

	retried, err := m.RetryDeadJob(ctx, job.ID, c.now(), alice)
	require.NoError(t, err)
	assert.Equal(t, models.JobQueued, retried.State)
	assert.Zero(t, retried.Attempts)
	_, err = m.RetryDeadJob(ctx, job.ID, c.now(), alice)
	assert.ErrorIs(t, err, models.ErrJobNotDead)
}

func TestTransientFailuresBackOffThenDie(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")
	legacyWorkout(t, m, repo, alice, testsupport.EncodeFIT(t, start, fourK))
	// A worker reading from an empty repo can't fetch the content: an outage, not a bad file.
	w, c := newWorker(m, testsupport.NewFileRepo(t))
	_, err := m.EnqueueReprocessAll(ctx, c.now(), alice)
	require.NoError(t, err)

	backoff := 30 * time.Second
	for attempt := 1; attempt < jobs.MaxAttempts; attempt++ {
		found, err := w.RunOnce(ctx)
		require.NoError(t, err)
		require.True(t, found, "attempt %d", attempt)
		job := onlyJob(t, m, alice)
		assert.Equal(t, models.JobQueued, job.State)
		assert.EqualValues(t, attempt, job.Attempts)
		assert.True(t, job.RunAt.Time.Equal(c.now().Add(backoff)), "attempt %d retries at %v, want %v", attempt, job.RunAt.Time, c.now().Add(backoff))

		found, err = w.RunOnce(ctx)
		require.NoError(t, err)
		assert.False(t, found, "not due until the backoff passes")
		c.advance(backoff)
		backoff *= 2
	}

	_, err = w.RunOnce(ctx)
	require.NoError(t, err)
	job := onlyJob(t, m, alice)
	assert.Equal(t, models.JobDead, job.State)
	assert.EqualValues(t, jobs.MaxAttempts, job.Attempts)
}

func TestExpiredLeaseIsClaimedAgain(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")
	legacyWorkout(t, m, repo, alice, testsupport.EncodeFIT(t, start, fourK))
	_, err := m.EnqueueReprocessAll(ctx, start, alice)
	require.NoError(t, err)

	// A worker that claims and then crashes.
	claimed, err := m.ClaimJob(ctx, start, start.Add(jobs.Lease))
	require.NoError(t, err)
	require.NotNil(t, claimed)

	again, err := m.ClaimJob(ctx, start.Add(time.Minute), start.Add(time.Minute+jobs.Lease))
	require.NoError(t, err)
	assert.Nil(t, again, "still leased")

	later := start.Add(jobs.Lease + time.Second)
	again, err = m.ClaimJob(ctx, later, later.Add(jobs.Lease))
	require.NoError(t, err)
	require.NotNil(t, again)
	assert.Equal(t, claimed.ID, again.ID)
	assert.EqualValues(t, 2, again.Attempts)
}

func TestJobsAreScopedToOwner(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice, bob := createUser(t, m, "alice@example.com"), createUser(t, m, "bob@example.com")
	legacyWorkout(t, m, repo, alice, testsupport.EncodeFIT(t, start, fourK))
	_, err := m.EnqueueReprocessAll(ctx, start, alice)
	require.NoError(t, err)
	job := onlyJob(t, m, alice)

	bobs, err := m.Jobs(ctx, "", 10, bob)
	require.NoError(t, err)
	assert.Empty(t, bobs)
	_, err = m.Job(ctx, job.ID, bob)
	assert.True(t, models.IsNotFound(err))
	n, err := m.EnqueueReprocessAll(ctx, start, bob)
	require.NoError(t, err)
	assert.Zero(t, n, "bob has no workouts")
}

func TestDeletingAWorkoutDropsItsJobs(t *testing.T) {
	ctx := context.Background()
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")
	stale := legacyWorkout(t, m, repo, alice, testsupport.EncodeFIT(t, start, fourK))
	_, err := m.EnqueueReprocessAll(ctx, start, alice)
	require.NoError(t, err)

	require.NoError(t, m.DeleteWorkout(ctx, stale.ID, alice))
	remaining, err := m.Jobs(ctx, "", 10, alice)
	require.NoError(t, err)
	assert.Empty(t, remaining)
}

func TestRunStopsWithContext(t *testing.T) {
	m, repo := testsupport.NewModel(t), testsupport.NewFileRepo(t)
	alice := createUser(t, m, "alice@example.com")
	stale := legacyWorkout(t, m, repo, alice, testsupport.EncodeFIT(t, start, fourK))
	w := jobs.NewWorker(m, repo)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()

	// Run enqueues the stale workout on start and processes it straight away.
	require.Eventually(t, func() bool {
		got, err := m.Workout(context.Background(), stale.ID, alice)
		return err == nil && got.ProcessorVersion == workouts.ProcessorVersion
	}, 5*time.Second, 20*time.Millisecond)

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run didn't return after cancel")
	}
}
