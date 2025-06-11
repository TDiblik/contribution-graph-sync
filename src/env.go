package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type IEnvData struct {
	GL_API_TOKEN        string
	GL_TARGET_SYNC_REPO string
}

var EnvData IEnvData

var PragueDateLoc *time.Location

const DateDateFormatLayout = "2006-01-02 15:04:05 CET"

func SetupENV(env_files ...string) error {
	log.Println("Setting up env variables: start")

	err := godotenv.Load(env_files...)
	if err != nil {
		return fmt.Errorf("Unable to load .env file: %v", err)
	}

	EnvData.GL_API_TOKEN = getEnvKeyOrPanic("GL_API_TOKEN")
	EnvData.GL_TARGET_SYNC_REPO = getEnvKeyOrPanic("GL_TARGET_SYNC_REPO")
	if stat, err := os.Stat(EnvData.GL_TARGET_SYNC_REPO); err != nil || !stat.IsDir() {
		return fmt.Errorf("GL_TARGET_SYNC_REPO does not exist: %v", err)
	}

	// just to make sure we can convert from UTC to CET
	PragueDateLoc, err = time.LoadLocation("Europe/Prague")
	if err != nil {
		return fmt.Errorf("unable to load Europe/Prague time location: %v", err)
	}

	log.Println("Setting up env variables: end")
	return nil
}

func getEnvKeyOrPanic(key string) string {
	val := os.Getenv(key)
	if len(val) == 0 {
		log.Fatal("Error loading ", key)
	}
	return val
}

func GetLastRecordedDate() (*time.Time, error) {
	filePath := lastRecordedDateFileName()
	if _, err := os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		final := time.Now().AddDate(-3, -6, 0) // even tho Gitlab officially says it only keeps records 3 years old.
		return &final, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	date, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	return &date, err
}

func SetLastRecordedDate(newDate time.Time) error {
	dateStr := newDate.Format(time.RFC3339)
	if err := os.WriteFile(lastRecordedDateFileName(), []byte(dateStr), 0644); err != nil {
		return err
	}
	return nil
}

func lastRecordedDateFileName() string {
	return path.Join(EnvData.GL_TARGET_SYNC_REPO, "last-recorded-date.txt")
}

func UtcToCet(date time.Time) time.Time {
	return date.In(PragueDateLoc)
}
