// Package scheduler drives time-based jobs using robfig/cron:
// per-activity reminder ticks and the nightly digest job.
// Implemented in Milestone 6.
package scheduler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/enricojoe/dailychecker/internal/occurrences"
	"github.com/enricojoe/dailychecker/internal/telegram"
	"github.com/robfig/cron/v3"
)

// Clock is a function that returns the current wall time. Using a function
// type (rather than time.Now directly) allows tests to inject a fixed instant
// without any global state. Production passes time.Now.
type Clock func() time.Time

// OccurrenceRepository is the subset of occurrences.Repository that the
// scheduler needs. Defined here (in the consumer) so the scheduler depends on
// an interface, not a concrete type — following the dependency-inversion
// principle and leaving room for future optimisations (e.g. FOR UPDATE SKIP
// LOCKED for multi-instance dedup) without touching this interface.
type OccurrenceRepository interface {
	ListDueReminders(ctx context.Context, date time.Time, asOf time.Time) ([]occurrences.ReminderRow, error)
	MarkPerActivityNotified(ctx context.Context, occurrenceID string) error
	ListDigestItems(ctx context.Context, date time.Time) ([]occurrences.DigestRow, error)
	MarkDigestNotified(ctx context.Context, occurrenceIDs []string) error
}

// Scheduler orchestrates the two recurring notification jobs.
// Construct via New; call Start to begin scheduling; call Stop to halt.
type Scheduler struct {
	repo         OccurrenceRepository
	tg           telegram.Client
	loc          *time.Location
	clock        Clock
	digestHour   int
	appPublicURL string
	cron         *cron.Cron
}

// New constructs a Scheduler.
//
//   - repo: occurrence repository (provides ListDueReminders, ListDigestItems, etc.).
//   - tg: Telegram client used to send messages (no real network in tests — inject a mock).
//   - loc: Jakarta *time.Location; all date/time calculations use this zone.
//   - clock: returns the current time; pass time.Now in production, a fixed func in tests.
//   - digestHour: local Jakarta hour at which the nightly digest fires (typically 22).
//   - appPublicURL: included in digest messages so users can open the web app.
func New(
	repo OccurrenceRepository,
	tg telegram.Client,
	loc *time.Location,
	clock Clock,
	digestHour int,
	appPublicURL string,
) *Scheduler {
	return &Scheduler{
		repo:         repo,
		tg:           tg,
		loc:          loc,
		clock:        clock,
		digestHour:   digestHour,
		appPublicURL: appPublicURL,
	}
}

// Start registers cron jobs and starts the cron runner in the background.
// It returns immediately; the jobs run until Stop is called or ctx is cancelled.
//
// Two jobs are registered:
//  1. Every minute: RunReminderTick with the current Jakarta time.
//  2. Daily at digestHour:00 Jakarta: RunDigest with the current Jakarta time.
//
// The cron runner uses the Jakarta location so that spec expressions are
// interpreted in local time, not UTC.
func (s *Scheduler) Start(ctx context.Context) {
	s.cron = cron.New(cron.WithLocation(s.loc))

	// Minute tick for per-activity reminders.
	_, _ = s.cron.AddFunc("* * * * *", func() {
		if err := s.RunReminderTick(ctx, s.clock()); err != nil {
			log.Printf("scheduler: reminder tick error: %v", err)
		}
	})

	// Nightly digest at digestHour:00 Jakarta.
	digestSpec := fmt.Sprintf("0 %d * * *", s.digestHour)
	_, _ = s.cron.AddFunc(digestSpec, func() {
		if err := s.RunDigest(ctx, s.clock()); err != nil {
			log.Printf("scheduler: digest error: %v", err)
		}
	})

	s.cron.Start()
	log.Printf("scheduler: started (digest hour %02d:00 Jakarta)", s.digestHour)
}

// Stop gracefully halts the cron runner, waiting for any running job to finish.
func (s *Scheduler) Stop() {
	if s.cron != nil {
		ctx := s.cron.Stop()
		<-ctx.Done()
		log.Println("scheduler: stopped")
	}
}

// RunReminderTick sends a one-off Telegram reminder for every top-level active
// occurrence whose scheduled time_of_day has been reached by now (Jakarta). It
// is idempotent: the per_activity_notified_at column prevents double-sending
// even if the process restarts between ticks.
//
// Design note: the query uses time_of_day <= now (not ==), so a reminder that
// was missed during downtime still fires on the next tick after restart.
// Exactly-once delivery is guaranteed by the dedup flag, not the tick schedule.
func (s *Scheduler) RunReminderTick(ctx context.Context, now time.Time) error {
	jakartaNow := now.In(s.loc)
	// Use midnight of jakartaNow as the date boundary (DATE portion only).
	dateOnly := time.Date(jakartaNow.Year(), jakartaNow.Month(), jakartaNow.Day(), 0, 0, 0, 0, s.loc)

	rows, err := s.repo.ListDueReminders(ctx, dateOnly, jakartaNow)
	if err != nil {
		return fmt.Errorf("scheduler: list due reminders: %w", err)
	}

	for _, row := range rows {
		msg := reminderText(row.Title)
		if err := s.tg.SendMessage(ctx, row.ChatID, msg); err != nil {
			// Log and continue: one failed send must not block the others.
			log.Printf("scheduler: send reminder to chat %d: %v", row.ChatID, err)
			continue
		}
		if err := s.repo.MarkPerActivityNotified(ctx, row.OccurrenceID); err != nil {
			log.Printf("scheduler: mark per-activity notified %s: %v", row.OccurrenceID, err)
		}
	}
	return nil
}

// RunDigest sends a single nightly summary message per user listing all
// not-done activities for today. Users with nothing outstanding receive no
// message. Once sent, each occurrence's digest_notified_at is stamped so the
// digest is never re-sent even if the job runs again (e.g. due to a restart
// right at the configured hour).
func (s *Scheduler) RunDigest(ctx context.Context, now time.Time) error {
	jakartaNow := now.In(s.loc)
	dateOnly := time.Date(jakartaNow.Year(), jakartaNow.Month(), jakartaNow.Day(), 0, 0, 0, 0, s.loc)

	rows, err := s.repo.ListDigestItems(ctx, dateOnly)
	if err != nil {
		return fmt.Errorf("scheduler: list digest items: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}

	// Group by user. Rows are already ordered by user_id from the repo.
	type userGroup struct {
		chatID int64
		userID string
		titles []string
		ids    []string
	}

	var groups []userGroup
	for _, row := range rows {
		if len(groups) == 0 || groups[len(groups)-1].userID != row.UserID {
			groups = append(groups, userGroup{
				chatID: row.ChatID,
				userID: row.UserID,
			})
		}
		g := &groups[len(groups)-1]
		g.titles = append(g.titles, row.Title)
		g.ids = append(g.ids, row.OccurrenceID)
	}

	for _, g := range groups {
		msg := digestText(g.titles, s.appPublicURL)
		if err := s.tg.SendMessage(ctx, g.chatID, msg); err != nil {
			log.Printf("scheduler: send digest to chat %d: %v", g.chatID, err)
			continue
		}
		if err := s.repo.MarkDigestNotified(ctx, g.ids); err != nil {
			log.Printf("scheduler: mark digest notified for user %s: %v", g.userID, err)
		}
	}
	return nil
}

// reminderText formats the Telegram message for a per-activity reminder.
func reminderText(activityTitle string) string {
	return fmt.Sprintf("Reminder: it's time for \"%s\"!", activityTitle)
}

// digestText formats the nightly digest Telegram message listing remaining items.
func digestText(titles []string, appPublicURL string) string {
	var sb strings.Builder
	sb.WriteString("DailyChecker nightly digest — items still not done today:\n")
	for _, t := range titles {
		sb.WriteString("  - ")
		sb.WriteString(t)
		sb.WriteString("\n")
	}
	if appPublicURL != "" {
		sb.WriteString("\nCheck them off here: ")
		sb.WriteString(appPublicURL)
	}
	return sb.String()
}
