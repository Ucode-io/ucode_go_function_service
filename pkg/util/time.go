package util

import "time"

func TruncateToStartOfDay(t time.Time) time.Time {
	var dateTime, err = time.Parse("2006-01-02", t.Format("2006-01-02"))
	if err != nil {
		return time.Time{}
	}
	return dateTime
}
