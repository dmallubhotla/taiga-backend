// Package testsupport builds real dependencies (migrated SQLite, local file repo, encoded FIT files) for integration tests.
package testsupport

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/muktihari/fit/encoder"
	"github.com/muktihari/fit/profile/filedef"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"

	"gitea.deepak.science/deepak/taiga/internal/config"
	"gitea.deepak.science/deepak/taiga/internal/filerepo"
	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/store"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
)

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// NewModel returns a model on a fresh file-backed SQLite database with all migrations applied.
// A file rather than :memory: because each pooled connection would otherwise get its own empty database.
func NewModel(t *testing.T) models.Model {
	t.Helper()
	cfg := &config.Config{
		Db: config.DBConfig{
			Driver:        "sqlite",
			FilePath:      filepath.Join(t.TempDir(), "test.db"),
			MigrationPath: filepath.Join(repoRoot(), "migrations-sqlite"),
			AutoMigrateUp: true,
		},
	}
	s, err := store.GetStore(cfg)
	if err != nil {
		t.Fatalf("opening test store: %v", err)
	}
	m := models.NewFromStore(s)
	t.Cleanup(func() { _ = m.Close() })
	return m
}

func NewFileRepo(t *testing.T) filerepo.FileRepo {
	t.Helper()
	repo, err := filerepo.NewFileRepo(config.Config{
		FileRepo: config.FileRepoConfig{Type: "local", AssetPath: t.TempDir(), PrefixLength: 2},
	})
	if err != nil {
		t.Fatalf("creating file repo: %v", err)
	}
	return repo
}

// Split is a stretch of constant pace.
type Split struct {
	Meters  float64
	Seconds float64
}

// EncodeFIT returns a single-session activity FIT file with a record every 10m along the given splits.
// FIT timestamps are whole seconds, so pick splits where 10m takes a whole number of seconds.
func EncodeFIT(t *testing.T, start time.Time, splits []Split) []byte {
	t.Helper()
	return encode(t, buildActivity(start, splits, 0))
}

// EncodeFITHeading is EncodeFIT with the track heading the given compass bearing in degrees (0 is north, 90 east), for runs on different routes.
func EncodeFITHeading(t *testing.T, start time.Time, splits []Split, bearing float64) []byte {
	t.Helper()
	return encode(t, buildActivity(start, splits, bearing))
}

// EncodeFITSport is EncodeFIT recorded as another sport (EncodeFIT records running).
func EncodeFITSport(t *testing.T, start time.Time, splits []Split, sport typedef.Sport) []byte {
	t.Helper()
	activity := buildActivity(start, splits, 0)
	activity.Sessions[0].SetSport(sport)
	return encode(t, activity)
}

// EncodeMultiSessionFIT is EncodeFIT with the session repeated, the shape of a multi-sport (e.g. triathlon) file.
func EncodeMultiSessionFIT(t *testing.T, start time.Time, splits []Split, sessions int) []byte {
	t.Helper()
	activity := buildActivity(start, splits, 0)
	for range sessions - 1 {
		activity.Sessions = append(activity.Sessions, activity.Sessions[0])
	}
	return encode(t, activity)
}

// What EncodeFIT records alongside distance: a steady heart rate, a gentle climb, and a straight track due north from the start.
const (
	RecordHeartRate = 150
	StartLatitude   = 45.0
	StartLongitude  = -93.0
	metersPerDegree = 111_320.0
)

// RecordAltitude climbs 1m per 100m covered.
func RecordAltitude(meters float64) float64 { return 100 + meters/100 }

func buildActivity(start time.Time, splits []Split, bearing float64) *filedef.Activity {
	north, east := math.Cos(bearing*math.Pi/180), math.Sin(bearing*math.Pi/180)
	lonScale := metersPerDegree * math.Cos(StartLatitude*math.Pi/180)
	const step = 10.0

	activity := filedef.NewActivity()
	activity.FileId = *mesgdef.NewFileId(nil).SetType(typedef.FileActivity).SetTimeCreated(start)

	var meters, seconds float64
	addRecord := func() {
		activity.Records = append(activity.Records, mesgdef.NewRecord(nil).
			SetTimestamp(start.Add(time.Duration(seconds*float64(time.Second)))).
			SetDistanceScaled(meters).
			SetHeartRate(RecordHeartRate).
			SetEnhancedAltitudeScaled(RecordAltitude(meters)).
			SetPositionLatDegrees(StartLatitude+north*meters/metersPerDegree).
			SetPositionLongDegrees(StartLongitude+east*meters/lonScale))
	}
	addRecord()
	for _, split := range splits {
		secondsPerMeter := split.Seconds / split.Meters
		for covered := 0.0; covered < split.Meters; covered += step {
			meters += step
			seconds += step * secondsPerMeter
			addRecord()
		}
	}

	end := start.Add(time.Duration(seconds * float64(time.Second)))
	activity.Sessions = append(activity.Sessions, mesgdef.NewSession(nil).
		SetStartTime(start).
		SetTimestamp(end).
		SetTotalDistanceScaled(meters).
		SetTotalElapsedTimeScaled(seconds).
		SetSport(typedef.SportRunning))
	return activity
}

func encode(t *testing.T, activity *filedef.Activity) []byte {
	t.Helper()
	fit := activity.ToFIT(nil)
	var buf bytes.Buffer
	if err := encoder.New(&buf).Encode(&fit); err != nil {
		t.Fatalf("encoding FIT: %v", err)
	}
	return buf.Bytes()
}

// NewToker returns a real RSA toker backed by a throwaway keypair.
func NewToker(t *testing.T) tokens.Toker {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	pub, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshalling public key: %v", err)
	}
	dir := t.TempDir()
	privPath, pubPath := filepath.Join(dir, "test.key"), filepath.Join(dir, "test.pem")
	writePEM(t, privPath, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key))
	writePEM(t, pubPath, "PUBLIC KEY", pub)

	toker, err := tokens.New(config.Config{
		App:    config.AppConfig{Environment: "test"},
		Tokens: config.TokensConfig{PrivateKeyPath: privPath, PublicKeyPath: pubPath},
	})
	if err != nil {
		t.Fatalf("creating toker: %v", err)
	}
	return toker
}

func writePEM(t *testing.T, path, blockType string, der []byte) {
	t.Helper()
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der}), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
