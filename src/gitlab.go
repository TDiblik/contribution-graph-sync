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
	const endpoint = "https://gitlab.com/api/v4/user"

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
		return nil, fmt.Errorf("Unable to parse bodyBytes for UserInfo from response body: %v", err)
	}

	var user UserInfo
	err = json.Unmarshal(bodyBytes, &user)
	if err != nil {
		return nil, fmt.Errorf("Error decoding UserInfo JSON: %v", err)
	}

	return &user, nil
}

type ProjectEvent struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	ProjectID   int       `json:"project_id"`
	ActionName  string    `json:"action_name"`
	TargetID    int       `json:"target_id"`
	TargetIID   int       `json:"target_iid"`
	TargetType  string    `json:"target_type"`
	AuthorID    int       `json:"author_id"`
	TargetTitle string    `json:"target_title"`
	CreatedAt   time.Time `json:"created_at"`
	PushData    struct {
		CommitCount int    `json:"commit_count"`
		CommitFrom  string `json:"commit_from"`
		CommitTo    string `json:"commit_to"`
	} `json:"push_data"`
}

func GetEvents(userId uint32, fromDate time.Time) (*[]ProjectEvent, error) {
	endpoint := "https://gitlab.com/api/v4/users/" + fmt.Sprint(userId) + "/events?sort=asc&per_page=100&after=" + fromDate.Format(time.RFC3339)

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

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse bodyBytes for UserInfo from response body: %v", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Error while calling the %s endpoint, status code: %v", endpoint, resp.StatusCode)
	}

	var events []ProjectEvent
	err = json.Unmarshal(bodyBytes, &events)
	if err != nil {
		return nil, fmt.Errorf("Error decoding []Event JSON: %v", err)
	}

	return &events, nil
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
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Error while calling the %s endpoint, status code: %v", endpoint, resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse bodyBytes for CommitInfo from response body: %v", err)
	}

	var commitsBetween []CommitsBetween
	err = json.Unmarshal(bodyBytes, &commitsBetween)
	if err != nil {
		return nil, fmt.Errorf("Error decoding CommitInfo JSON: %v", err)
	}

	var result []CommitsBetween
	resultSize := min(event.PushData.CommitCount, len(commitsBetween))
	for i := resultSize - 1; i >= 0; i-- {
		result = append(result, commitsBetween[i])
	}

	return &result, nil
}
