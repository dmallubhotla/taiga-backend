package main

import (
	"encoding/json"
	"fmt"
	"log"
	flag "github.com/spf13/pflag"
	"os"

	"gitea.deepak.science/deepak/trygo/internal/config"
	"gitea.deepak.science/deepak/trygo/internal/workouts"
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

	log.Printf("obtained config: %+v", cfg)

	f, err := os.Open(filename)
	if err != nil {
		log.Printf("error opening filename %v: %v", filename, err)
		panic(err)
	}

	run, err := workouts.ReadFitFile(f)
	if err != nil {
		log.Printf("Failed with error %v\n", err)
		panic(err)
	}

	// log.Printf("%+v", activitySummary.Summary)
	printJson(run.Summary)

	// for idx, rec := range run.Records {
	// 	log.Printf("Record: [%d]. %v mi in %v minutes \n", idx, rec.Distance.AsMiles(), rec.DurationFromStart.Minutes())
	// 	// printJson(rec)
	// }
	results := workouts.FastestSegments(run, []float64{1000, 1609.344, 3000, 5000, 8046.72, 10000})
	// log.Printf("results: %+v", results[1])
	for _, result := range results {
		if result.End > 0 {
			log.Println(result)
		}
	}

}
