package worker

import (
	"log"
	"os"
	"sync"
	"time"
)

// defaultSchedulerTimezone is the IANA timezone used for all time-of-day based
// cron expressions (e.g. "send report at 09:00 every day").
// Overridable at runtime via the SCHEDULER_TIMEZONE environment variable.
// DB timestamps and logs are always stored in UTC — this setting only affects
// how local-time cron patterns are interpreted by the scheduler.
const defaultSchedulerTimezone = "Africa/Cairo"

var (
	schedulerLoc     *time.Location
	schedulerLocOnce sync.Once
)

// SchedulerLocation returns the *time.Location used by the Asynq scheduler
// for cron-based tasks. It is loaded once from SCHEDULER_TIMEZONE (default:
// Africa/Cairo) and cached for the lifetime of the process.
func SchedulerLocation() *time.Location {
	schedulerLocOnce.Do(func() {
		tzName := os.Getenv("SCHEDULER_TIMEZONE")
		if tzName == "" {
			tzName = defaultSchedulerTimezone
		}
		loc, err := time.LoadLocation(tzName)
		if err != nil {
			log.Printf("[Scheduler] WARNING: could not load timezone %q (%v); falling back to UTC", tzName, err)
			loc = time.UTC
		} else {
			log.Printf("[Scheduler] Using timezone: %s", tzName)
		}
		schedulerLoc = loc
	})
	return schedulerLoc
}

// NowLocal returns time.Now() expressed in the scheduler's local timezone.
// Use this for any business logic that must be aware of local time (e.g.
// "is the exam submission window open in Cairo time?").
func NowLocal() time.Time {
	return time.Now().In(SchedulerLocation())
}
