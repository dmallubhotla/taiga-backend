package workouts_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/muktihari/fit/profile/typedef"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitea.deepak.science/deepak/taiga/internal/testsupport"
	"gitea.deepak.science/deepak/taiga/internal/workouts"
)

func TestFITSource(t *testing.T) {
	start := time.Date(2026, 11, 1, 7, 0, 0, 0, time.UTC)
	splits := []testsupport.Split{{Meters: 1000, Seconds: 300}}
	for name, tc := range map[string]struct {
		manufacturer typedef.Manufacturer
		product      uint16
		productName  string
		want         string
	}{
		"product name":         {typedef.ManufacturerCoros, 804, "COROS PACE 3", "COROS PACE 3"},
		"garmin product":       {typedef.ManufacturerGarmin, uint16(typedef.GarminProductFr965), "", "Garmin fr965"},
		"unknown garmin":       {typedef.ManufacturerGarmin, 65000, "", "Garmin"},
		"other manufacturer":   {typedef.ManufacturerStrava, 1, "", "Strava"},
		"nothing about device": {typedef.ManufacturerInvalid, 0, "", ""},
	} {
		run, err := workouts.ReadActivityFile(bytes.NewReader(testsupport.EncodeFITDevice(t, start, splits, tc.manufacturer, tc.product, tc.productName)))
		require.NoError(t, err, name)
		assert.Equal(t, tc.want, run.Summary.Source, name)
	}
}

func TestEndomondoSource(t *testing.T) {
	run := readEndomondo(t, testsupport.NewEndomondo(endomondoStart, []testsupport.Split{{Meters: 1000, Seconds: 300}}, "RUNNING"))
	assert.Equal(t, workouts.EndomondoSource, run.Summary.Source)
}
