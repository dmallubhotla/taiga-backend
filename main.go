package main

import (
	"log"
	"os"
)

func main() {
	log.Printf("This is a template project. Run the server with: go run cmd/server/main.go")
	log.Printf("Or build and run: go build -o server cmd/server/main.go && ./server")
	os.Exit(0)
}
