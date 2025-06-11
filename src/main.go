package main

import (
	"log"
	"time"
)

func main() {
	if err := SetupENV(); err != nil {
		log.Fatalln(err)
	}

	user, err := GetUserInfo()
	if err != nil {
		log.Fatalln(err)
	}

	for {
		lastRecordedDate, err := GetLastRecordedDate()
		if err != nil {
			log.Fatalln(err)
		}

		events, err := GetEvents(user.Id, *lastRecordedDate)
		if err != nil {
			log.Fatalln(err)
		}
		if len(*events) == 0 {
			return
		}

		for _, event := range *events {
			switch {
			// todo: ADD an option to log PRs the user has merged.
			case event.ActionName == "pushed new" && event.PushData.CommitCount == 1:
				CreateGitCommit("pushed new branch", event.CreatedAt)
			case event.ActionName == "pushed to" && event.PushData.CommitCount == 1:
				CreateGitCommit("created a commit", event.CreatedAt)
			case event.ActionName == "pushed new":
				HandleMultipleCommits(event, user.CommitEmail)
				CreateGitCommit("pushed new branch", event.CreatedAt)
			case event.ActionName == "pushed to":
				HandleMultipleCommits(event, user.CommitEmail)
			case event.ActionName == "opened" && event.TargetType == "MergeRequest":
				CreateGitCommit("opened merge request", event.CreatedAt)
			default:
				log.Println("not handled: ", event.ActionName)
			}
		}
		time.Sleep(time.Millisecond * 500) // give the API some rest :D (prevents rate-limiting + random HTTP 500 codes)
	}
}

func HandleMultipleCommits(event ProjectEvent, commiterEmail string) {
	commitsBetween, err := GetCommitsBetween(event, commiterEmail)
	if err != nil {
		log.Fatalln(err)
	}
	for _, commit := range *commitsBetween {
		CreateGitCommit("created a commit", commit.AuthoredDate)
	}
	SetLastRecordedDate(event.CreatedAt)
	time.Sleep(time.Millisecond * 500) // give the API some rest :D (prevents rate-limiting + random HTTP 500 codes)
}
