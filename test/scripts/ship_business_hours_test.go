package scripts

import (
	"strings"
	"testing"
)

// spec 58: business-hours gate blocks --non-compatible-migration in production (BETA=false).

const businessHoursMarker = "blocked during business hours"

func TestShipNonCompatBlockedDuringBusinessHours(t *testing.T) {
	repo := newTempShipRepo(t)
	repo.env = append(repo.env, "SHIP_BETA=false", "NOW_DOW=3", "NOW_HOUR=10")

	out, err := repo.runShip(t, "--non-compatible-migration")
	if err == nil {
		t.Fatalf("bin/ship exited 0 during business hours; want non-zero.\noutput:\n%s", out)
	}
	if !strings.Contains(out, businessHoursMarker) {
		t.Fatalf("output missing %q.\noutput:\n%s", businessHoursMarker, out)
	}
}

func TestShipNonCompatAllowedAfterHoursWeekday(t *testing.T) {
	repo := newTempShipRepo(t)
	repo.env = append(repo.env, "SHIP_BETA=false", "NOW_DOW=3", "NOW_HOUR=20")

	out, _ := repo.runShip(t, "--non-compatible-migration")
	if strings.Contains(out, businessHoursMarker) {
		t.Fatalf("gate should not fire after hours; %q appeared.\noutput:\n%s", businessHoursMarker, out)
	}
}

func TestShipNonCompatAllowedOnWeekend(t *testing.T) {
	repo := newTempShipRepo(t)
	repo.env = append(repo.env, "SHIP_BETA=false", "NOW_DOW=6", "NOW_HOUR=10")

	out, _ := repo.runShip(t, "--non-compatible-migration")
	if strings.Contains(out, businessHoursMarker) {
		t.Fatalf("gate should not fire on weekends; %q appeared.\noutput:\n%s", businessHoursMarker, out)
	}
}
