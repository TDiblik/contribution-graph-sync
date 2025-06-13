package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"time"
)

func runGit(args []string, envVars []string, dir string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), envVars...)
	if err := cmd.Run(); err != nil {
		log.Fatalf("git %v failed: %v", args, err)
	}
}

func CreateGitCommit(message string, dateRaw time.Time) {
	date := UtcToCet(dateRaw)

	var fileHandle *os.File
	filePath := path.Join(EnvData.GL_TARGET_SYNC_REPO, date.Format(time.DateOnly)+".txt")
	if _, err := os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		fileHandle, err = os.Create(filePath)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		fileHandle, err = os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Fatal(err)
		}
	}
	defer fileHandle.Close()

	if _, err := fileHandle.Write(fmt.Appendln(nil, date.Format(DateDateFormatLayout)+":", message)); err != nil {
		log.Fatal(err)
	}
	if err := SetLastRecordedDate(date); err != nil {
		log.Fatal(err)
	}

	dateStr := date.Format(time.RFC3339)
	runGit([]string{"add", "."}, nil, EnvData.GL_TARGET_SYNC_REPO)
	runGit([]string{"commit", "-m", fmt.Sprintf("[sync] %s", date.Format(DateDateFormatLayout))},
		[]string{
			"GIT_AUTHOR_DATE=" + dateStr,
			"GIT_COMMITTER_DATE=" + dateStr,
		}, EnvData.GL_TARGET_SYNC_REPO)
}
