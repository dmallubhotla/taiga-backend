package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	flag "github.com/spf13/pflag"

	"gitea.deepak.science/deepak/taiga/internal/config"
	"gitea.deepak.science/deepak/taiga/internal/filerepo"
	"gitea.deepak.science/deepak/taiga/internal/workouts"
)

func printJson(truc any) error {
	printable, err := json.MarshalIndent(truc, "", "\t")
	if err != nil {
		return fmt.Errorf("got a prob")
	}
	log.Println(string(printable))
	return nil
}

func main() {
	// Load configuration
	cfg, err := config.Load("config")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	var filename string
	flag.StringVar(&filename, "filename", "", "filename of fit file")

	flag.Parse()

	f, err := os.Open(filename)
	if err != nil {
		log.Printf("error opening filename %v: %v", filename, err)
		panic(err)
	}
	defer func() { _ = f.Close() }()

	repo, err := filerepo.NewFileRepo(*cfg)
	if err != nil {
		log.Printf("error creating filerepo: %v", err)
		panic(err)
	}
	hash, err := repo.Store(context.Background(), f)
	if err != nil {
		panic(err)
	}
	log.Printf("Stored with hash %v", hash)

	retrievedFile, err := repo.Fetch(context.Background(), hash)
	if err != nil {
		panic(err)
	}
	run, err := workouts.ReadActivityFile(retrievedFile)
	if err != nil {
		log.Printf("Failed with error %v\n", err)
		panic(err)
	}

	// log.Printf("%+v", activitySummary.Summary)
	if err := printJson(run.Summary); err != nil {
		log.Printf("error printing summary: %v", err)
	}

	// for idx, rec := range run.Records {
	// 	log.Printf("Record: [%d]. %v mi in %v minutes \n", idx, rec.Distance.AsMiles(), rec.DurationFromStart.Minutes())
	// 	// printJson(rec)
	// }
	results := append(workouts.FastestSegments(run, workouts.StandardDistances), workouts.FarthestEfforts(run, workouts.StandardDurations)...)
	for _, result := range results {
		log.Println(result)
	}

}
