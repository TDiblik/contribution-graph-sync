package main

import (
	"log"
	"time"
)

func main() {
	if err := SetupENV(); err != nil {
		log.Fatalln(err)
	}

	sources, err := GetActivitySources()
	if err != nil {
		log.Fatalln(err)
	}

	for {
		syncedAny := false
		for _, source := range sources {
			lastRecordedDate, err := GetLastRecordedDate(source.Name())
			if err != nil {
				log.Fatalln(err)
			}

			activities, err := source.GetActivities(*lastRecordedDate)
			if err != nil {
				log.Fatalln(err)
			}
			if len(activities) == 0 {
				continue
			}

			processed := 0
			for _, activity := range activities {
				if !activity.CursorDate.After(*lastRecordedDate) {
					continue
				}
				CreateGitCommit(activity.Message, activity.CommitDate)
				if err := SetLastRecordedDate(source.Name(), activity.CursorDate); err != nil {
					log.Fatalln(err)
				}
				processed++
			}
			if processed > 0 {
				log.Printf("Synced %d activities from %s\n", processed, source.Name())
				syncedAny = true
			}
		}

		if !syncedAny {
			log.Println("All sources are fully synced. Exiting.")
			break
		}

		time.Sleep(time.Millisecond * 500) // give the API some rest :D (prevents rate-limiting + random HTTP 500 codes)
	}
}
