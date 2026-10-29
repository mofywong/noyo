package inspection

import (
	"sort"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// ScheduleRule is the user-facing calendar/interval contract. Legacy cron
// strings remain readable for existing plans; new rules do not require cron.
type ScheduleRule struct {
	Kind     string `json:"kind"`
	Time     string `json:"time,omitempty"`
	Weekdays []int  `json:"weekdays,omitempty"`  // Sunday=0, Saturday=6.
	MonthDay int    `json:"month_day,omitempty"` // -1 means the final calendar day.
	At       string `json:"date_time,omitempty"`
	Every    int    `json:"every,omitempty"`
	Unit     string `json:"unit,omitempty"`
	StartAt  string `json:"start_at,omitempty"` // Fixed interval anchor in plan timezone.
}

func ServerTimezoneName() string { return time.Local.String() }

// Browser timezone identifiers cannot represent Windows' "Local" location.
// An empty identifier explicitly selects the accompanying server UTC offset.
func ServerBrowserTimezone() string {
	name := ServerTimezoneName()
	if name == "Local" {
		return ""
	}
	if _, err := time.LoadLocation(name); err != nil {
		return ""
	}
	return name
}

func parseSchedule(_ string, expr string) (cron.Schedule, error) {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "TZ=") || strings.HasPrefix(expr, "CRON_TZ=") {
		return nil, ErrInvalid
	}
	schedule, err := cron.ParseStandard(expr)
	if calendar, ok := schedule.(*cron.SpecSchedule); ok {
		calendar.Location = time.Local
	}
	return schedule, err
}
func localScheduleTime(value string, loc *time.Location) (time.Time, error) {
	parsed, err := time.ParseInLocation("2006-01-02T15:04", value, loc)
	if err != nil || parsed.Format("2006-01-02T15:04") != value {
		return time.Time{}, ErrInvalid
	}
	return parsed, nil
}
func intervalDuration(rule ScheduleRule) time.Duration {
	unit := time.Minute
	if rule.Unit == "hours" {
		unit = time.Hour
	}
	if rule.Unit == "days" {
		unit = 24 * time.Hour
	}
	return time.Duration(rule.Every) * unit
}
func validateSchedules(spec *Spec) error {
	if len(spec.Schedules)+len(spec.ScheduleRules) > 12 {
		return ErrInvalid
	}
	if spec.Mode == "manual" && (len(spec.Schedules)+len(spec.ScheduleRules) > 0 || spec.AutoOrder) {
		return ErrInvalid
	}
	for _, expr := range spec.Schedules {
		if _, err := parseSchedule(spec.Timezone, expr); err != nil {
			return ErrInvalid
		}
	}
	loc := time.Local
	seen := map[string]bool{}
	for i := range spec.ScheduleRules {
		rule := &spec.ScheduleRules[i]
		switch rule.Kind {
		case "daily", "weekly", "monthly":
			parsed, err := time.Parse("15:04", rule.Time)
			if err != nil || parsed.Format("15:04") != rule.Time {
				return ErrInvalid
			}
			if rule.Kind == "weekly" {
				if len(rule.Weekdays) == 0 || len(rule.Weekdays) > 7 {
					return ErrInvalid
				}
				days := map[int]bool{}
				for _, day := range rule.Weekdays {
					if day < 0 || day > 6 || days[day] {
						return ErrInvalid
					}
					days[day] = true
				}
				sort.Ints(rule.Weekdays)
			} else {
				rule.Weekdays = nil
			}
			if rule.Kind == "monthly" && (rule.MonthDay == 0 || rule.MonthDay < -1 || rule.MonthDay > 31) {
				return ErrInvalid
			}
			if rule.Kind != "monthly" {
				rule.MonthDay = 0
			}
			rule.At, rule.StartAt, rule.Unit, rule.Every = "", "", "", 0
		case "once":
			if _, err := localScheduleTime(rule.At, loc); err != nil {
				return ErrInvalid
			}
			*rule = ScheduleRule{Kind: "once", At: rule.At}
		case "interval":
			if rule.Every < 1 || rule.Every > 10000 || (rule.Unit != "minutes" && rule.Unit != "hours" && rule.Unit != "days") {
				return ErrInvalid
			}
			if _, err := localScheduleTime(rule.StartAt, loc); err != nil {
				return ErrInvalid
			}
			*rule = ScheduleRule{Kind: "interval", Every: rule.Every, Unit: rule.Unit, StartAt: rule.StartAt}
		default:
			return ErrInvalid
		}
		key := encoded(rule)
		if seen[key] {
			return ErrInvalid
		}
		seen[key] = true
	}
	return nil
}
func nextRule(rule ScheduleRule, loc *time.Location, now time.Time) time.Time {
	if rule.Kind == "once" {
		value, _ := localScheduleTime(rule.At, loc)
		if value.After(now) {
			return value
		}
		return time.Time{}
	}
	if rule.Kind == "interval" {
		start, err := localScheduleTime(rule.StartAt, loc)
		period := intervalDuration(rule)
		if err != nil || period <= 0 {
			return time.Time{}
		}
		if start.After(now) {
			return start
		}
		// Anchor arithmetic avoids drift after slow jobs or restart catch-up.
		seconds := int64(period / time.Second)
		steps := (now.Unix()-start.Unix())/seconds + 1
		return time.Unix(start.Unix()+steps*seconds, 0).In(loc)
	}
	clock, err := time.Parse("15:04", rule.Time)
	if err != nil {
		return time.Time{}
	}
	local := now.In(loc)
	for offset := 0; offset <= 366; offset++ {
		day := time.Date(local.Year(), local.Month(), local.Day()+offset, 12, 0, 0, 0, loc)
		if rule.Kind == "weekly" {
			matches := false
			for _, weekday := range rule.Weekdays {
				matches = matches || weekday == int(day.Weekday())
			}
			if !matches {
				continue
			}
		}
		if rule.Kind == "monthly" {
			desired := rule.MonthDay
			if desired == -1 {
				desired = time.Date(day.Year(), day.Month()+1, 0, 12, 0, 0, 0, loc).Day()
			}
			if day.Day() != desired {
				continue
			}
		}
		value := time.Date(day.Year(), day.Month(), day.Day(), clock.Hour(), clock.Minute(), 0, 0, loc)
		// Skip nonexistent DST wall times rather than running at a different hour.
		if value.Day() == day.Day() && value.Hour() == clock.Hour() && value.Minute() == clock.Minute() && value.After(now) {
			return value
		}
	}
	return time.Time{}
}
func nextDue(spec Spec, now time.Time) *time.Time {
	if spec.Mode != "ai" || !spec.Enabled {
		return nil
	}
	var next *time.Time
	consider := func(value time.Time) {
		if !value.IsZero() && (next == nil || value.Before(*next)) {
			v := value.UTC()
			next = &v
		}
	}
	loc := time.Local
	for _, rule := range spec.ScheduleRules {
		consider(nextRule(rule, loc, now))
	}
	for _, expr := range spec.Schedules {
		if schedule, err := parseSchedule(spec.Timezone, expr); err == nil {
			consider(schedule.Next(now.In(time.Local)))
		}
	}
	return next
}

func normalizeScheduleTimes(db *gorm.DB) error {
	var plans []Plan
	return db.Model(&Plan{}).FindInBatches(&plans, 100, func(tx *gorm.DB, batch int) error {
		for _, plan := range plans {
			spec := plan.Spec()
			if spec.Timezone != ServerTimezoneName() {
				spec.Timezone = ServerTimezoneName()
				if err := tx.Model(&Plan{}).Where("id = ?", plan.ID).Updates(map[string]any{"spec_json": encoded(spec), "next_due_at": nextDue(spec, time.Now())}).Error; err != nil {
					return err
				}
			} else if plan.NextDueAt != nil {
				_, offset := plan.NextDueAt.Zone()
				if offset != 0 {
					if err := tx.Model(&Plan{}).Where("id = ?", plan.ID).UpdateColumn("next_due_at", plan.NextDueAt.UTC()).Error; err != nil {
						return err
					}
				}
			}
		}
		return nil
	}).Error
}
