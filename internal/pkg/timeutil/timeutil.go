package timeutil

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

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
// Returns an error if the format is invalid.
func ParseDateUTC(dateStr string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date format: %w", err)
	}
	// Ensure the parsed time is in UTC location
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
}

// FormatDateUTC formats a time.Time as "YYYY-MM-DD" string.
func FormatDateUTC(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

// StartOfDayUTC returns the given time normalized to midnight UTC.
func StartOfDayUTC(t time.Time) time.Time {
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

// EndExclusiveOfDayUTC returns the start of the next day (exclusive upper bound).
// Use this for range queries: date >= start AND date < end
func EndExclusiveOfDayUTC(t time.Time) time.Time {
	return StartOfDayUTC(t).AddDate(0, 0, 1)
}

// DayRangeUTC returns the start (inclusive) and end (exclusive) times for a given date.
// Use for SQL queries: WHERE date >= start AND date < end
func DayRangeUTC(t time.Time) (start time.Time, end time.Time) {
	start = StartOfDayUTC(t)
	end = EndExclusiveOfDayUTC(t)
	return start, end
}

// DateRangeFromStringUTC parses a date string and returns:
// - start: beginning of day in UTC (inclusive)
// - end: beginning of next day in UTC (exclusive)
// - date: the original date string formatted as YYYY-MM-DD
// - error: parsing error if any
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
// weekStart specifies which day the week starts on (e.g., time.Sunday or time.Monday).
func StartOfWeekUTC(t time.Time, weekStart time.Weekday) time.Time {
	utc := t.UTC()
	dayOfWeek := utc.Weekday()

	// Calculate days to subtract to get to the start of the week
	daysFromStart := int(dayOfWeek - weekStart)
	if daysFromStart < 0 {
		daysFromStart += 7
	}

	weekStartDate := utc.AddDate(0, 0, -daysFromStart)
	return time.Date(weekStartDate.Year(), weekStartDate.Month(), weekStartDate.Day(), 0, 0, 0, 0, time.UTC)
}

// WeekRangeUTC returns the start (inclusive) and end (exclusive) times for the week containing the given time.
// weekStart specifies which day the week starts on (e.g., time.Sunday or time.Monday).
// The range covers 7 full days from the week start.
func WeekRangeUTC(t time.Time, weekStart time.Weekday) (start time.Time, end time.Time) {
	start = StartOfWeekUTC(t, weekStart)
	end = start.AddDate(0, 0, 7)
	return start, end
}

// NormalizeToUTC converts any time.Time to UTC location and truncates to the date level (midnight).
// Useful for ensuring consistent date comparisons regardless of input timezone.
func NormalizeToUTC(t time.Time) time.Time {
	return StartOfDayUTC(t)
}

// DateOnly is a time.Time wrapper that serializes as "YYYY-MM-DD" in JSON.
// Use this for date-only fields such as due_date in API responses.
type DateOnly time.Time

func (d DateOnly) MarshalJSON() ([]byte, error) {
	s := time.Time(d).Format("2006-01-02")
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
	*d = DateOnly(t)
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
		*d = DateOnly(t)
	}
	return nil
}

func (d DateOnly) Time() time.Time {
	return time.Time(d)
}

func NewDateOnly(t time.Time) DateOnly {
	return DateOnly(StartOfDayUTC(t))
}

// ParseISODateTime parses an ISO 8601 datetime string with timezone offset
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
