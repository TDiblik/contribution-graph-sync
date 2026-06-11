package main

import "time"

type SyncActivity struct {
	Message    string
	CommitDate time.Time
	CursorDate time.Time
}

type ActivitySource interface {
	Name() string
	GetActivities(fromDate time.Time) ([]SyncActivity, error)
}

func newSyncActivity(message string, commitDate time.Time, cursorDate time.Time) SyncActivity {
	return SyncActivity{
		Message:    message,
		CommitDate: commitDate,
		CursorDate: cursorDate,
	}
}

func GetActivitySources() ([]ActivitySource, error) {
	var sources []ActivitySource
	if EnvData.GL_API_TOKEN != "" {
		gl, err := NewGitLabSource()
		if err != nil {
			return nil, err
		}
		sources = append(sources, gl)
	}
	if EnvData.AZDO_ORGANIZATION != "" {
		azdo, err := NewAzureDevOpsSource()
		if err != nil {
			return nil, err
		}
		sources = append(sources, azdo)
	}
	return sources, nil
}
