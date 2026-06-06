package timeutil

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseeDateUTC(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantYear    int
		wantMonth   time.Month
		wantDay     int
		wantHour    int
		wantMinute  int
		wantSecond  int
		wantZone    string
		expectError bool
	}{
		{
			name:       "valid date 2026-06-02",
			input:      "2026-06-02",
			wantYear:   2026,
			wantMonth:  time.June,
			wantDay:    2,
			wantHour:   0,
			wantMinute: 0,
			wantSecond: 0,
			wantZone:   "UTC",
		},
		{
			name:       "valid date 2025-01-01",
			input:      "2025-01-01",
			wantYear:   2025,
			wantMonth:  time.January,
			wantDay:    1,
			wantHour:   0,
			wantMinute: 0,
			wantSecond: 0,
			wantZone:   "UTC",
		},
		{
			name:        "invalid format",
			input:       "02-06-2026",
			expectError: true,
		},
		{
			name:        "invalid date",
			input:       "2026-13-01",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDateUTC(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("ParseDateUTC() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseDateUTC() unexpected error: %v", err)
				return
			}

			if got.Year() != tt.wantYear {
				t.Errorf("Year = %d, want %d", got.Year(), tt.wantYear)
			}
			if got.Month() != tt.wantMonth {
				t.Errorf("Month = %v, want %v", got.Month(), tt.wantMonth)
			}
			if got.Day() != tt.wantDay {
				t.Errorf("Day = %d, want %d", got.Day(), tt.wantDay)
			}
			if got.Hour() != tt.wantHour {
				t.Errorf("Hour = %d, want %d", got.Hour(), tt.wantHour)
			}
			if got.Minute() != tt.wantMinute {
				t.Errorf("Minute = %d, want %d", got.Minute(), tt.wantMinute)
			}
			if got.Second() != tt.wantSecond {
				t.Errorf("Second = %d, want %d", got.Second(), tt.wantSecond)
			}
			if got.Location().String() != tt.wantZone {
				t.Errorf("Location = %s, want %s", got.Location().String(), tt.wantZone)
			}
		})
	}
}

func TestDayRangeUTC(t *testing.T) {
	tests := []struct {
		name      string
		input     time.Time
		wantStart string
		wantEnd   string
	}{
		{
			name:      "UTC time at midnight",
			input:     time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
			wantStart: "2026-06-02 00:00:00 +0000 UTC",
			wantEnd:   "2026-06-03 00:00:00 +0000 UTC",
		},
		{
			name:      "UTC time at noon",
			input:     time.Date(2026, 6, 2, 12, 30, 45, 0, time.UTC),
			wantStart: "2026-06-02 00:00:00 +0000 UTC",
			wantEnd:   "2026-06-03 00:00:00 +0000 UTC",
		},
		{
			name:      "non-UTC time",
			input:     time.Date(2026, 6, 2, 14, 0, 0, 0, time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)),
			wantStart: "2026-06-02 00:00:00 +0000 UTC",
			wantEnd:   "2026-06-03 00:00:00 +0000 UTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := DayRangeUTC(tt.input)

			if start.String() != tt.wantStart {
				t.Errorf("start = %v, want %v", start.String(), tt.wantStart)
			}
			if end.String() != tt.wantEnd {
				t.Errorf("end = %v, want %v", end.String(), tt.wantEnd)
			}

			// Verify end is exactly 24 hours after start
			diff := end.Sub(start)
			if diff != 24*time.Hour {
				t.Errorf("end - start = %v, want 24h", diff)
			}
		})
	}
}

func TestFormatDateUTC(t *testing.T) {
	tests := []struct {
		name  string
		input time.Time
		want  string
	}{
		{
			name:  "UTC midnight",
			input: time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
			want:  "2026-06-02",
		},
		{
			name:  "UTC afternoon",
			input: time.Date(2026, 6, 2, 15, 30, 0, 0, time.UTC),
			want:  "2026-06-02",
		},
		{
			name:  "non-UTC timezone",
			input: time.Date(2026, 6, 2, 23, 0, 0, 0, time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)),
			want:  "2026-06-02",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDateUTC(tt.input)
			if got != tt.want {
				t.Errorf("FormatDateUTC() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWeekRangeUTC(t *testing.T) {
	tests := []struct {
		name      string
		input     time.Time
		weekStart time.Weekday
		wantStart string
		wantEnd   string
	}{
		{
			name:      "Monday week start, input is Tuesday",
			input:     time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC), // Tuesday, June 2, 2026
			weekStart: time.Monday,
			wantStart: "2026-06-01 00:00:00 +0000 UTC", // Monday, June 1
			wantEnd:   "2026-06-08 00:00:00 +0000 UTC", // Monday, June 8
		},
		{
			name:      "Sunday week start, input is Tuesday",
			input:     time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC), // Tuesday, June 2, 2026
			weekStart: time.Sunday,
			wantStart: "2026-05-31 00:00:00 +0000 UTC", // Sunday, May 31
			wantEnd:   "2026-06-07 00:00:00 +0000 UTC", // Sunday, June 7
		},
		{
			name:      "Monday week start, input is Monday",
			input:     time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), // Monday, June 1, 2026
			weekStart: time.Monday,
			wantStart: "2026-06-01 00:00:00 +0000 UTC",
			wantEnd:   "2026-06-08 00:00:00 +0000 UTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := WeekRangeUTC(tt.input, tt.weekStart)

			if start.String() != tt.wantStart {
				t.Errorf("start = %v, want %v", start.String(), tt.wantStart)
			}
			if end.String() != tt.wantEnd {
				t.Errorf("end = %v, want %v", end.String(), tt.wantEnd)
			}

			// Verify end is exactly 7 days after start
			diff := end.Sub(start)
			if diff != 7*24*time.Hour {
				t.Errorf("end - start = %v, want 168h (7 days)", diff)
			}
		})
	}
}

func TestTodayUTC(t *testing.T) {
	// Test that TodayUTC returns midnight UTC
	today := TodayUTC()

	if today.Location().String() != "UTC" {
		t.Errorf("Location = %s, want UTC", today.Location().String())
	}

	if today.Hour() != 0 || today.Minute() != 0 || today.Second() != 0 || today.Nanosecond() != 0 {
		t.Errorf("TodayUTC() should return midnight, got %v", today)
	}

	// Verify it's within reasonable range of now
	now := NowUTC()
	if today.After(now) {
		t.Errorf("TodayUTC() is in the future: today=%v, now=%v", today, now)
	}

	yesterday := now.AddDate(0, 0, -1)
	if today.Before(StartOfDayUTC(yesterday)) {
		t.Errorf("TodayUTC() is too far in the past: today=%v, now=%v", today, now)
	}
}

func TestNowUTC(t *testing.T) {
	now := NowUTC()

	if now.Location().String() != "UTC" {
		t.Errorf("Location = %s, want UTC", now.Location().String())
	}

	// Verify it's close to time.Now()
	systemNow := time.Now()
	diff := systemNow.Sub(now)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Errorf("NowUTC() differs from system time by %v", diff)
	}
}

func TestNormalizeToUTC(t *testing.T) {
	tests := []struct {
		name  string
		input time.Time
		want  string
	}{
		{
			name:  "UTC time",
			input: time.Date(2026, 6, 2, 15, 30, 45, 0, time.UTC),
			want:  "2026-06-02 00:00:00 +0000 UTC",
		},
		{
			name:  "Asia/Ho_Chi_Minh timezone (UTC+7)",
			input: time.Date(2026, 6, 2, 14, 0, 0, 0, time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)),
			want:  "2026-06-02 00:00:00 +0000 UTC",
		},
		{
			name:  "US Eastern timezone (UTC-5)",
			input: time.Date(2026, 6, 2, 10, 0, 0, 0, time.FixedZone("EST", -5*3600)),
			want:  "2026-06-02 00:00:00 +0000 UTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeToUTC(tt.input)
			if got.String() != tt.want {
				t.Errorf("NormalizeToUTC() = %v, want %v", got.String(), tt.want)
			}
		})
	}
}

func TestDateRangeFromStringUTC(t *testing.T) {
	start, end, date, err := DateRangeFromStringUTC("2026-06-02")

	if err != nil {
		t.Fatalf("DateRangeFromStringUTC() unexpected error: %v", err)
	}

	if date != "2026-06-02" {
		t.Errorf("date = %v, want 2026-06-02", date)
	}

	if start.String() != "2026-06-02 00:00:00 +0000 UTC" {
		t.Errorf("start = %v, want 2026-06-02 00:00:00 +0000 UTC", start.String())
	}

	if end.String() != "2026-06-03 00:00:00 +0000 UTC" {
		t.Errorf("end = %v, want 2026-06-03 00:00:00 +0000 UTC", end.String())
	}

	// Test invalid input
	_, _, _, err = DateRangeFromStringUTC("invalid-date")
	if err == nil {
		t.Error("DateRangeFromStringUTC() expected error for invalid date")
	}
}

func TestTimezoneIndependence(t *testing.T) {
	// Create times in different timezones representing the same instant
	utcTime := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	hcmTime := utcTime.In(time.FixedZone("Asia/Ho_Chi_Minh", 7*3600))
	estTime := utcTime.In(time.FixedZone("EST", -5*3600))

	// All should produce the same start-of-day UTC
	utcDay := StartOfDayUTC(utcTime)
	hcmDay := StartOfDayUTC(hcmTime)
	estDay := StartOfDayUTC(estTime)

	if !utcDay.Equal(hcmDay) || !utcDay.Equal(estDay) {
		t.Errorf("StartOfDayUTC should be timezone-independent:\nUTC: %v\nHCM: %v\nEST: %v",
			utcDay, hcmDay, estDay)
	}

	// All should format to the same date string
	utcDateStr := FormatDateUTC(utcTime)
	hcmDateStr := FormatDateUTC(hcmTime)
	estDateStr := FormatDateUTC(estTime)

	if utcDateStr != hcmDateStr || utcDateStr != estDateStr {
		t.Errorf("FormatDateUTC should be timezone-independent:\nUTC: %v\nHCM: %v\nEST: %v",
			utcDateStr, hcmDateStr, estDateStr)
	}
}

func TestDateOnlyMarshalJSON(t *testing.T) {
	d := NewDateOnly(time.Date(2026, 6, 2, 10, 30, 0, 0, time.UTC))
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("MarshalJSON error: %v", err)
	}
	want := `"2026-06-02"`
	if string(b) != want {
		t.Errorf("MarshalJSON = %s, want %s", string(b), want)
	}
}

func TestDateOnlyUnmarshalJSON(t *testing.T) {
	input := `"2026-06-02"`
	var d DateOnly
	err := json.Unmarshal([]byte(input), &d)
	if err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}
	want := "2026-06-02"
	got := d.Time().Format("2006-01-02")
	if got != want {
		t.Errorf("UnmarshalJSON = %s, want %s", got, want)
	}
}

func TestDateOnlyRoundtrip(t *testing.T) {
	original := DateOnly(time.Date(2026, 12, 15, 0, 0, 0, 0, LocationVN))
	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var decoded DateOnly
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if original != decoded {
		t.Errorf("roundtrip failed: %v != %v", original.Time(), decoded.Time())
	}
}

func TestNewDateOnly(t *testing.T) {
	tests := []struct {
		name  string
		input time.Time
		wantY int
		wantM time.Month
		wantD int
	}{
		{
			name:  "UTC afternoon",
			input: time.Date(2026, 6, 2, 15, 30, 45, 0, time.UTC),
			wantY: 2026,
			wantM: time.June,
			wantD: 2,
		},
		{
			name:  "Asia/Ho_Chi_Minh timezone",
			input: time.Date(2026, 6, 2, 14, 0, 0, 0, time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)),
			wantY: 2026,
			wantM: time.June,
			wantD: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewDateOnly(tt.input)
			vn := got.Time().In(LocationVN)
			if vn.Year() != tt.wantY || vn.Month() != tt.wantM || vn.Day() != tt.wantD {
				t.Errorf("NewDateOnly(%v) = %v, want Y=%d M=%v D=%d", tt.input, got.Time(), tt.wantY, tt.wantM, tt.wantD)
			}
			if vn.Hour() != 0 || vn.Minute() != 0 || vn.Second() != 0 {
				t.Errorf("NewDateOnly(%v) should be midnight VN, got hour=%d min=%d", tt.input, vn.Hour(), vn.Minute())
			}
		})
	}
}

func TestParseISODateTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "RFC3339 with timezone offset",
			input: "2026-06-02T20:30:00+07:00",
			want:  "2026-06-02 13:30:00 +0000 UTC",
		},
		{
			name:  "RFC3339 UTC",
			input: "2026-06-02T20:30:00Z",
			want:  "2026-06-02 20:30:00 +0000 UTC",
		},
		{
			name:  "without timezone",
			input: "2026-06-02T20:30:00",
			want:  "2026-06-02 20:30:00 +0000 UTC",
		},
		{
			name:    "invalid format",
			input:   "not-a-datetime",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseISODateTime(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			gotUTC := got.UTC().String()
			if gotUTC != tt.want {
				t.Errorf("ParseISODateTime = %s, want %s", gotUTC, tt.want)
			}
		})
	}
}
