package handler

import (
	"testing"
	"time"
)

func TestParseDateUsesUTC(t *testing.T) {
	date, err := parseDate(" 2026-09-19 ")
	if err != nil {
		t.Fatalf("parseDate returned an error: %v", err)
	}

	want := time.Date(2026, time.September, 19, 0, 0, 0, 0, time.UTC)
	if !date.Equal(want) {
		t.Fatalf("parseDate() = %v, want %v", date, want)
	}
	if date.Location() != time.UTC {
		t.Fatalf("parseDate() location = %v, want UTC", date.Location())
	}
}

func TestParseDateRejectsNonGregorianAPIValue(t *testing.T) {
	if _, err := parseDate("1405/06/28"); err == nil {
		t.Fatal("parseDate accepted a Jalali display value; the API must receive YYYY-MM-DD")
	}
}
