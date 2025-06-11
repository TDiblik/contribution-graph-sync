package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type UserInfo struct {
	Id          uint32 `json:"id"`
	Username    string `json:"username"`
	CommitEmail string `json:"commit_email"`
}

func GetUserInfo() (*UserInfo, error) {
	return gitlabRequest[UserInfo]("https://gitlab.com/api/v4/user")
}

type ProjectEvent struct {
	ID         int       `json:"id"`
	ProjectID  int       `json:"project_id"`
	ActionName string    `json:"action_name"`
	TargetType string    `json:"target_type"`
	CreatedAt  time.Time `json:"created_at"`
	PushData   struct {
		CommitCount int    `json:"commit_count"`
		CommitFrom  string `json:"commit_from"`
		CommitTo    string `json:"commit_to"`
	} `json:"push_data"`
}

func GetEvents(userId uint32, fromDate time.Time) (*[]ProjectEvent, error) {
	endpoint := "https://gitlab.com/api/v4/users/" + fmt.Sprint(userId) + "/events?sort=asc&per_page=100&after=" + fromDate.Format(time.RFC3339)
	return gitlabRequest[[]ProjectEvent](endpoint)
}

type CommitsBetween struct {
	Id           string    `json:"id"`
	Title        string    `json:"title"`
	AuthoredDate time.Time `json:"authored_date"`
}

func GetCommitsBetween(event ProjectEvent, commiterEmail string) (*[]CommitsBetween, error) {
	revisionRange := event.PushData.CommitFrom + ".." + event.PushData.CommitTo
	if event.PushData.CommitFrom == "" {
		revisionRange = event.PushData.CommitTo + "~" + fmt.Sprint(event.PushData.CommitCount) + ".." + event.PushData.CommitTo
	}
	endpoint := "https://gitlab.com/api/v4/projects/" + fmt.Sprint(event.ProjectID) + "/repository/commits?ref_name=" + revisionRange + "&author=" + url.QueryEscape(commiterEmail)

	commitsBetween, err := gitlabRequest[[]CommitsBetween](endpoint)
	if err != nil {
		return nil, err
	}

	var ordered []CommitsBetween
	resultSize := min(event.PushData.CommitCount, len(*commitsBetween))
	for i := resultSize - 1; i >= 0; i-- {
		ordered = append(ordered, (*commitsBetween)[i])
	}

	return &ordered, nil
}

func gitlabRequest[T any](endpoint string) (*T, error) {
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("Unable to create GET %s request: %v", endpoint, err)
	}
	req.Header.Set("PRIVATE-TOKEN", EnvData.GL_API_TOKEN)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Error while calling the %s endpoint: %v", endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitLab API (%s) returned non-OK status %d: %s", endpoint, resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse bodyBytes from response body: %v", err)
	}

	var bodyParsed T
	err = json.Unmarshal(bodyBytes, &bodyParsed)
	if err != nil {
		return nil, fmt.Errorf("Error decoding JSON: %v", err)
	}

	return &bodyParsed, nil
}
