package telegram

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// TimePeriod represents a time period with start and end dates
type TimePeriod struct {
	Start time.Time
	End   time.Time
}

// GetTodayPeriod returns period for today
func GetTodayPeriod() TimePeriod {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, now.Location())
	return TimePeriod{Start: start, End: end}
}

// GetWeekPeriod returns period for last 7 days
func GetWeekPeriod() TimePeriod {
	now := time.Now()
	end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, now.Location())
	start := end.AddDate(0, 0, -6)
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	return TimePeriod{Start: start, End: end}
}

// GetMonthPeriod returns period for last 30 days
func GetMonthPeriod() TimePeriod {
	now := time.Now()
	end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, now.Location())
	start := end.AddDate(0, 0, -29)
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	return TimePeriod{Start: start, End: end}
}

// GetAllTimePeriod returns period from 2000 to now
func GetAllTimePeriod() TimePeriod {
	now := time.Now()
	start := time.Date(2000, 1, 1, 0, 0, 0, 0, now.Location())
	end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, now.Location())
	return TimePeriod{Start: start, End: end}
}

// ParseCustomPeriod parses custom period from user input
// Supported formats:
// - "03.04.25 07.04.25" or "03.04.25-07.04.25"
// - "03.04 07.04" or "03.04-07.04" (uses current year)
func ParseCustomPeriod(input string) (TimePeriod, error) {
	// Remove extra spaces
	input = strings.TrimSpace(input)

	// Try to split by various separators
	var parts []string
	if strings.Contains(input, "-") {
		parts = strings.Split(input, "-")
	} else {
		parts = strings.Split(input, " ")
	}

	if len(parts) != 2 {
		return TimePeriod{}, errors.New("неверный формат даты")
	}

	start, err := parseDate(strings.TrimSpace(parts[0]))
	if err != nil {
		return TimePeriod{}, fmt.Errorf("ошибка в начальной дате: %w", err)
	}

	end, err := parseDate(strings.TrimSpace(parts[1]))
	if err != nil {
		return TimePeriod{}, fmt.Errorf("ошибка в конечной дате: %w", err)
	}

	// Set time to start of day for start date and end of day for end date
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, end.Location())

	if start.After(end) {
		return TimePeriod{}, errors.New("начальная дата не может быть позже конечной")
	}

	return TimePeriod{Start: start, End: end}, nil
}

// parseYear parses year from string or returns current year if empty
func parseYear(yearStr string) int {
	if yearStr == "" {
		return time.Now().Year()
	}

	year, _ := strconv.Atoi(yearStr)
	// Convert 2-digit year to 4-digit
	if year < 100 {
		if year < 50 {
			return year + 2000
		}
		return year + 1900
	}
	return year
}

// parseDate parses date from string
// Formats: "03.04.25", "03.04"
func parseDate(s string) (time.Time, error) {
	// Match dd.mm.yy or dd.mm.yyyy or dd.mm
	re := regexp.MustCompile(`^(\d{1,2})\.(\d{1,2})(?:\.(\d{2,4}))?$`)
	matches := re.FindStringSubmatch(s)

	if matches == nil {
		return time.Time{}, errors.New("неверный формат даты (используйте ДД.ММ.ГГ или ДД.ММ)")
	}

	day, _ := strconv.Atoi(matches[1])
	month, _ := strconv.Atoi(matches[2])
	year := parseYear(matches[3])

	// Validate date
	if month < 1 || month > 12 {
		return time.Time{}, errors.New("месяц должен быть от 1 до 12")
	}
	if day < 1 || day > 31 {
		return time.Time{}, errors.New("день должен быть от 1 до 31")
	}

	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Now().Location())

	// Check if date is valid (e.g., not February 30)
	if date.Day() != day {
		return time.Time{}, errors.New("несуществующая дата")
	}

	return date, nil
}

// FormatDate formats date as DD.MM.YY
func FormatDate(t time.Time) string {
	return t.Format("02.01.06")
}

// FormatPeriod formats period as "DD.MM.YY - DD.MM.YY"
func FormatPeriod(period TimePeriod) string {
	return fmt.Sprintf("%s - %s", FormatDate(period.Start), FormatDate(period.End))
}

// DaysBetween returns number of days between start and end
func (p TimePeriod) DaysBetween() int {
	return int(p.End.Sub(p.Start).Hours() / 24)
}
