package timeutil

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

var LocationVN *time.Location

func init() {
	var err error
	LocationVN, err = time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		LocationVN = time.FixedZone("ICT", 7*3600)
	}
}

// --- UTC helpers (keep for timestamps: CreatedAt, UpdatedAt) ---

// NowUTC returns the current time in UTC.
func NowUTC() time.Time {
	return time.Now().UTC()
}

// TodayUTC returns the current date at midnight UTC.
func TodayUTC() time.Time {
	now := NowUTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// ParseDateUTC parses a date string in "YYYY-MM-DD" format and returns a time.Time at midnight UTC.
func ParseDateUTC(dateStr string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date format: %w", err)
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
}

// FormatDateUTC formats a time.Time as "YYYY-MM-DD" string in UTC.
func FormatDateUTC(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

// StartOfDayUTC returns the given time normalized to midnight UTC.
func StartOfDayUTC(t time.Time) time.Time {
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

// EndExclusiveOfDayUTC returns the start of the next day (exclusive upper bound).
func EndExclusiveOfDayUTC(t time.Time) time.Time {
	return StartOfDayUTC(t).AddDate(0, 0, 1)
}

// DayRangeUTC returns the start (inclusive) and end (exclusive) times for a given date.
func DayRangeUTC(t time.Time) (start time.Time, end time.Time) {
	start = StartOfDayUTC(t)
	end = EndExclusiveOfDayUTC(t)
	return start, end
}

// DateRangeFromStringUTC parses a date string and returns start/end range in UTC.
func DateRangeFromStringUTC(dateStr string) (start time.Time, end time.Time, date string, err error) {
	parsed, err := ParseDateUTC(dateStr)
	if err != nil {
		return time.Time{}, time.Time{}, "", err
	}
	start, end = DayRangeUTC(parsed)
	date = FormatDateUTC(parsed)
	return start, end, date, nil
}

// StartOfWeekUTC returns the start of the week (midnight UTC) for the given time.
func StartOfWeekUTC(t time.Time, weekStart time.Weekday) time.Time {
	utc := t.UTC()
	dayOfWeek := utc.Weekday()
	daysFromStart := int(dayOfWeek - weekStart)
	if daysFromStart < 0 {
		daysFromStart += 7
	}
	weekStartDate := utc.AddDate(0, 0, -daysFromStart)
	return time.Date(weekStartDate.Year(), weekStartDate.Month(), weekStartDate.Day(), 0, 0, 0, 0, time.UTC)
}

// WeekRangeUTC returns the start (inclusive) and end (exclusive) times for the week.
func WeekRangeUTC(t time.Time, weekStart time.Weekday) (start time.Time, end time.Time) {
	start = StartOfWeekUTC(t, weekStart)
	end = start.AddDate(0, 0, 7)
	return start, end
}

// NormalizeToUTC converts any time.Time to UTC location and truncates to midnight.
func NormalizeToUTC(t time.Time) time.Time {
	return StartOfDayUTC(t)
}

// --- VN timezone helpers (for server-side date calculations) ---

// NowVN returns the current time in Asia/Ho_Chi_Minh (+7).
func NowVN() time.Time {
	return time.Now().In(LocationVN)
}

// TodayVN returns the current date at midnight Asia/Ho_Chi_Minh.
func TodayVN() time.Time {
	now := NowVN()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, LocationVN)
}

// ParseDateVN parses a date string in "YYYY-MM-DD" format and returns midnight in VN timezone.
func ParseDateVN(dateStr string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date format: %w", err)
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, LocationVN), nil
}

// FormatDateVN formats a time.Time as "YYYY-MM-DD" string in VN timezone.
func FormatDateVN(t time.Time) string {
	return t.In(LocationVN).Format("2006-01-02")
}

// StartOfDayVN returns the given time normalized to midnight VN timezone.
func StartOfDayVN(t time.Time) time.Time {
	vn := t.In(LocationVN)
	return time.Date(vn.Year(), vn.Month(), vn.Day(), 0, 0, 0, 0, LocationVN)
}

// EndExclusiveOfDayVN returns the start of the next day (exclusive upper bound) in VN.
func EndExclusiveOfDayVN(t time.Time) time.Time {
	return StartOfDayVN(t).AddDate(0, 0, 1)
}

// DayRangeVN returns the start (inclusive) and end (exclusive) times for a given date in VN.
func DayRangeVN(t time.Time) (start time.Time, end time.Time) {
	start = StartOfDayVN(t)
	end = EndExclusiveOfDayVN(t)
	return start, end
}

// StartOfWeekVN returns the start of the week (midnight VN) for the given time.
func StartOfWeekVN(t time.Time, weekStart time.Weekday) time.Time {
	vn := t.In(LocationVN)
	dayOfWeek := vn.Weekday()
	daysFromStart := int(dayOfWeek - weekStart)
	if daysFromStart < 0 {
		daysFromStart += 7
	}
	weekStartDate := vn.AddDate(0, 0, -daysFromStart)
	return time.Date(weekStartDate.Year(), weekStartDate.Month(), weekStartDate.Day(), 0, 0, 0, 0, LocationVN)
}

// WeekRangeVN returns the start (inclusive) and end (exclusive) times for the week in VN.
func WeekRangeVN(t time.Time, weekStart time.Weekday) (start time.Time, end time.Time) {
	start = StartOfWeekVN(t, weekStart)
	end = start.AddDate(0, 0, 7)
	return start, end
}

// --- DateOnly (JSON date serialization) ---

// DateOnly is a time.Time wrapper that serializes as "YYYY-MM-DD" in VN timezone.
type DateOnly time.Time

func (d DateOnly) MarshalJSON() ([]byte, error) {
	s := time.Time(d).In(LocationVN).Format("2006-01-02")
	return json.Marshal(s)
}

func (d *DateOnly) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return err
	}
	*d = DateOnly(time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, LocationVN))
	return nil
}

func (d DateOnly) Value() (driver.Value, error) {
	return time.Time(d), nil
}

func (d *DateOnly) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		*d = DateOnly(v)
	case string:
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return err
		}
		*d = DateOnly(time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, LocationVN))
	}
	return nil
}

func (d DateOnly) Time() time.Time {
	return time.Time(d)
}

func NewDateOnly(t time.Time) DateOnly {
	return DateOnly(StartOfDayVN(t))
}

// ParseISODateTime parses an ISO 8601 datetime string with timezone offset.
// (e.g. "2026-06-02T20:30:00+07:00" or "2026-06-02T20:30:00Z").
func ParseISODateTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
	}
	for _, format := range formats {
		t, err := time.Parse(format, s)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid ISO datetime: %s", s)
}
