package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

const azureDevOpsAPIVersion = "7.1"
const azureDevOpsPageSize = 100

type AzureDevOpsSource struct {
	organization string
	author       string
	profileID    string
}

func NewAzureDevOpsSource() (*AzureDevOpsSource, error) {
	source := &AzureDevOpsSource{
		organization: EnvData.AZDO_ORGANIZATION,
	}

	profile, err := GetAzureDevOpsProfile()
	if err != nil {
		return nil, err
	}

	author := firstNonEmpty(EnvData.AZDO_AUTHOR, profile.DisplayName(), profile.Email())
	if author == "" {
		return nil, fmt.Errorf("unable to determine author from Azure DevOps profile (no email or display name)")
	}

	source.author = author
	source.profileID = profile.ID
	return source, nil
}

func (source *AzureDevOpsSource) Name() string {
	return "Azure DevOps"
}

func (source *AzureDevOpsSource) GetActivities(fromDate time.Time) ([]SyncActivity, error) {
	repositories, err := source.GetRepositories()
	if err != nil {
		return nil, err
	}

	type commitResult struct {
		repo    AzureDevOpsRepository
		commits []AzureDevOpsCommit
		err     error
	}
	commitChan := make(chan commitResult, len(repositories))
	for _, repository := range repositories {
		go func(repo AzureDevOpsRepository) {
			commits, err := source.GetCommits(repo, fromDate)
			commitChan <- commitResult{repo, commits, err}
		}(repository)
	}

	var activities []SyncActivity
	seenCommits := make(map[string]struct{})

	for i := 0; i < len(repositories); i++ {
		res := <-commitChan
		if res.err != nil {
			return nil, res.err
		}
		for _, commit := range res.commits {
			if !commit.Author.Date.After(fromDate) {
				continue
			}

			key := res.repo.ID + ":" + commit.CommitID
			if _, exists := seenCommits[key]; exists {
				continue
			}
			seenCommits[key] = struct{}{}

			activities = append(activities, newSyncActivity(
				fmt.Sprintf("created a commit in %s/%s", res.repo.Project.Name, res.repo.Name),
				commit.Author.Date,
				commit.Author.Date,
			))
		}
	}

	projectNames := source.projectNames(repositories)
	type prResult struct {
		project string
		prs     []AzureDevOpsPullRequest
		err     error
	}
	prChan := make(chan prResult, len(projectNames))
	for _, projectName := range projectNames {
		go func(proj string) {
			prs, err := source.GetPullRequests(proj, fromDate)
			prChan <- prResult{proj, prs, err}
		}(projectName)
	}

	for i := 0; i < len(projectNames); i++ {
		res := <-prChan
		if res.err != nil {
			return nil, res.err
		}
		for _, pullRequest := range res.prs {
			if !pullRequest.CreationDate.After(fromDate) {
				continue
			}

			activities = append(activities, newSyncActivity(
				fmt.Sprintf("opened pull request in %s/%s", pullRequest.Repository.Project.Name, pullRequest.Repository.Name),
				pullRequest.CreationDate,
				pullRequest.CreationDate,
			))
		}
	}

	sort.SliceStable(activities, func(i, j int) bool {
		if activities[i].CursorDate.Equal(activities[j].CursorDate) {
			return activities[i].Message < activities[j].Message
		}
		return activities[i].CursorDate.Before(activities[j].CursorDate)
	})

	return activities, nil
}

func (source *AzureDevOpsSource) GetRepositories() ([]AzureDevOpsRepository, error) {
	endpoint := fmt.Sprintf("https://dev.azure.com/%s/_apis/git/repositories?api-version=%s",
		url.PathEscape(source.organization),
		azureDevOpsAPIVersion,
	)

	response, err := azureDevOpsRequest[azureDevOpsListResponse[AzureDevOpsRepository]](endpoint)
	if err != nil {
		return nil, err
	}
	return response.Value, nil
}

func (source *AzureDevOpsSource) GetCommits(repository AzureDevOpsRepository, fromDate time.Time) ([]AzureDevOpsCommit, error) {
	var commits []AzureDevOpsCommit
	for skip := 0; ; {
		values := url.Values{}
		values.Set("api-version", azureDevOpsAPIVersion)
		values.Set("searchCriteria.author", source.author)
		values.Set("searchCriteria.fromDate", fromDate.Format(time.RFC3339))
		values.Set("searchCriteria.showOldestCommitsFirst", "true")
		values.Set("$top", strconv.Itoa(azureDevOpsPageSize))
		values.Set("$skip", strconv.Itoa(skip))

		endpoint := fmt.Sprintf("https://dev.azure.com/%s/%s/_apis/git/repositories/%s/commits?%s",
			url.PathEscape(source.organization),
			url.PathEscape(repository.Project.Name),
			url.PathEscape(repository.ID),
			values.Encode(),
		)

		response, err := azureDevOpsRequest[azureDevOpsListResponse[AzureDevOpsCommit]](endpoint)
		if err != nil {
			return nil, err
		}
		if len(response.Value) == 0 {
			break
		}

		commits = append(commits, response.Value...)
		skip += len(response.Value)
		if len(response.Value) < azureDevOpsPageSize {
			break
		}
	}
	return commits, nil
}

func (source *AzureDevOpsSource) GetPullRequests(projectName string, fromDate time.Time) ([]AzureDevOpsPullRequest, error) {
	if source.profileID == "" {
		return nil, fmt.Errorf("unable to fetch Azure DevOps pull requests because the profile id is empty")
	}

	var pullRequests []AzureDevOpsPullRequest
	for skip := 0; ; {
		values := url.Values{}
		values.Set("api-version", azureDevOpsAPIVersion)
		values.Set("searchCriteria.creatorId", source.profileID)
		values.Set("searchCriteria.minTime", fromDate.Format(time.RFC3339))
		values.Set("searchCriteria.queryTimeRangeType", "created")
		values.Set("searchCriteria.status", "all")
		values.Set("$top", strconv.Itoa(azureDevOpsPageSize))
		values.Set("$skip", strconv.Itoa(skip))

		endpoint := fmt.Sprintf("https://dev.azure.com/%s/%s/_apis/git/pullrequests?%s",
			url.PathEscape(source.organization),
			url.PathEscape(projectName),
			values.Encode(),
		)

		response, err := azureDevOpsRequest[azureDevOpsListResponse[AzureDevOpsPullRequest]](endpoint)
		if err != nil {
			return nil, err
		}
		if len(response.Value) == 0 {
			break
		}

		pullRequests = append(pullRequests, response.Value...)
		skip += len(response.Value)
		if len(response.Value) < azureDevOpsPageSize {
			break
		}
	}
	return pullRequests, nil
}

func (source *AzureDevOpsSource) projectNames(repositories []AzureDevOpsRepository) []string {
	seen := make(map[string]struct{})
	var names []string
	for _, repository := range repositories {
		if repository.Project.Name == "" {
			continue
		}
		if _, exists := seen[repository.Project.Name]; exists {
			continue
		}
		seen[repository.Project.Name] = struct{}{}
		names = append(names, repository.Project.Name)
	}
	sort.Strings(names)
	return names
}

type AzureDevOpsProfile struct {
	ID             string                                 `json:"id"`
	DisplayNameRaw string                                 `json:"displayName"`
	EmailRaw       string                                 `json:"emailAddress"`
	CoreAttributes map[string]AzureDevOpsProfileAttribute `json:"coreAttributes"`
}

type AzureDevOpsProfileAttribute struct {
	Value any `json:"value"`
}

func GetAzureDevOpsProfile() (*AzureDevOpsProfile, error) {
	endpoint := "https://app.vssps.visualstudio.com/_apis/profile/profiles/me?api-version=" + azureDevOpsAPIVersion
	return azureDevOpsRequest[AzureDevOpsProfile](endpoint)
}

func (profile AzureDevOpsProfile) DisplayName() string {
	return firstNonEmpty(
		profile.DisplayNameRaw,
		profile.coreAttribute("DisplayName"),
		profile.coreAttribute("displayName"),
	)
}

func (profile AzureDevOpsProfile) Email() string {
	return firstNonEmpty(
		profile.EmailRaw,
		profile.coreAttribute("Email"),
		profile.coreAttribute("email"),
		profile.coreAttribute("EmailAddress"),
		profile.coreAttribute("emailAddress"),
	)
}

func (profile AzureDevOpsProfile) coreAttribute(key string) string {
	if profile.CoreAttributes == nil {
		return ""
	}

	attr, exists := profile.CoreAttributes[key]
	if !exists || attr.Value == nil {
		return ""
	}
	value, ok := attr.Value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

type AzureDevOpsRepository struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	RemoteURL string `json:"remoteUrl"`
	Project   struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"project"`
}

type AzureDevOpsCommit struct {
	CommitID string `json:"commitId"`
	Author   struct {
		Name  string    `json:"name"`
		Email string    `json:"email"`
		Date  time.Time `json:"date"`
	} `json:"author"`
	Comment   string `json:"comment"`
	RemoteURL string `json:"remoteUrl"`
}

type AzureDevOpsPullRequest struct {
	PullRequestID int       `json:"pullRequestId"`
	CreationDate  time.Time `json:"creationDate"`
	Repository    struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Project struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"project"`
	} `json:"repository"`
}

type azureDevOpsListResponse[T any] struct {
	Count int `json:"count"`
	Value []T `json:"value"`
}

var azureDevOpsCred *azidentity.DefaultAzureCredential

func getAzureDevOpsCredential() (*azidentity.DefaultAzureCredential, error) {
	if azureDevOpsCred == nil {
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, err
		}
		azureDevOpsCred = cred
	}
	return azureDevOpsCred, nil
}

func azureDevOpsRequest[T any](endpoint string) (*T, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create GET %s request: %v", endpoint, err)
	}

	cred, err := getAzureDevOpsCredential()
	if err != nil {
		return nil, fmt.Errorf("failed to obtain Azure credential: %v", err)
	}

	token, err := cred.GetToken(context.Background(), policy.TokenRequestOptions{Scopes: []string{"499b84ac-1321-427f-aa17-267ca6975798/.default"}})
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure DevOps token: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error while calling the %s endpoint: %v", endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("Azure DevOps API (%s) returned non-OK status %d: %s: %s", endpoint, resp.StatusCode, http.StatusText(resp.StatusCode), strings.TrimSpace(string(bodyBytes)))
	}

	var bodyParsed T
	if err := json.NewDecoder(resp.Body).Decode(&bodyParsed); err != nil {
		return nil, fmt.Errorf("error decoding JSON: %v", err)
	}

	return &bodyParsed, nil
}
