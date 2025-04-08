package util

import (
	"fmt"
	"time"

	"git.pepabo.com/yukyan/gh-pric/github/model"
)

// ParseDateRange は日付文字列を解析して日付範囲を返します
func ParseDateRange(startStr, endStr string) (model.DateRange, error) {
	startDate, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		return model.DateRange{}, fmt.Errorf("Failed to parse start date: %w", err)
	}

	endDate, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		return model.DateRange{}, fmt.Errorf("Failed to parse end date: %w", err)
	}

	// Set end date to 23:59:59
	endDate = endDate.Add(24*time.Hour - time.Second)

	if endDate.Before(startDate) {
		return model.DateRange{}, fmt.Errorf("End date must be after start date")
	}

	return model.DateRange{
		StartDate: startDate,
		EndDate:   endDate,
	}, nil
} 
