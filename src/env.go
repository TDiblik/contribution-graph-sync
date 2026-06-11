package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type IEnvData struct {
	TARGET_SYNC_REPO  string
	GL_API_TOKEN      string
	AZDO_ORGANIZATION string
	AZDO_AUTHOR       string
}

var EnvData IEnvData

var PragueDateLoc *time.Location

const DateDateFormatLayout = "2006-01-02 15:04:05 CET"

func SetupENV(env_files ...string) error {
	err := godotenv.Load(env_files...)
	if err != nil {
		return fmt.Errorf("Unable to load .env file: %v", err)
	}

	EnvData.GL_API_TOKEN = getOptionalEnvKey("GL_API_TOKEN")
	EnvData.AZDO_ORGANIZATION = getOptionalEnvKey("AZDO_ORGANIZATION")
	EnvData.AZDO_AUTHOR = getOptionalEnvKey("AZDO_AUTHOR")
	EnvData.TARGET_SYNC_REPO = getOptionalEnvKey("TARGET_SYNC_REPO")
	if EnvData.TARGET_SYNC_REPO == "" {
		return fmt.Errorf("TARGET_SYNC_REPO is required")
	}
	if stat, err := os.Stat(EnvData.TARGET_SYNC_REPO); err != nil || !stat.IsDir() {
		return fmt.Errorf("TARGET_SYNC_REPO does not exist: %v", err)
	}
	if EnvData.GL_API_TOKEN == "" && EnvData.AZDO_ORGANIZATION == "" {
		return fmt.Errorf("no sync providers configured (set GL_API_TOKEN or AZDO_ORGANIZATION)")
	}

	// just to make sure we can convert from UTC to CET
	PragueDateLoc, err = time.LoadLocation("Europe/Prague")
	if err != nil {
		return fmt.Errorf("unable to load Europe/Prague time location: %v", err)
	}

	return nil
}

func getOptionalEnvKey(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func GetLastRecordedDate(sourceName string) (*time.Time, error) {
	filePath := lastRecordedDateFileName(sourceName)
	if _, err := os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		final := time.Now().AddDate(-18, -6, 0) // even tho Gitlab officially says it only keeps records 3 years old.
		return &final, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	date, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	return &date, err
}

func SetLastRecordedDate(sourceName string, newDate time.Time) error {
	dateStr := newDate.Format(time.RFC3339)
	if err := os.WriteFile(lastRecordedDateFileName(sourceName), []byte(dateStr), 0644); err != nil {
		return err
	}
	return nil
}

func lastRecordedDateFileName(sourceName string) string {
	fileName := fmt.Sprintf("last-recorded-date-%s.txt", strings.ToLower(strings.ReplaceAll(sourceName, " ", "-")))
	return path.Join(EnvData.TARGET_SYNC_REPO, fileName)
}

func UtcToCet(date time.Time) time.Time {
	return date.In(PragueDateLoc)
}
