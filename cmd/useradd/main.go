// useradd creates an account directly in the database, for deployments with auth.registration closed.
//
// In the cluster: kubectl -n taiga exec -it deploy/taiga-backend -- useradd -email you@example.com
// The password is prompted for without echo, or read from the first line of stdin when it isn't a terminal.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"golang.org/x/term"

	"gitea.deepak.science/deepak/taiga/internal/config"
	"gitea.deepak.science/deepak/taiga/internal/models"
)

const minPasswordLength = 8

func main() {
	email := flag.String("email", "", "email address to sign in with (required)")
	displayName := flag.String("display-name", "", "display name (defaults to the email address)")
	flag.Parse()

	if *email == "" {
		flag.Usage()
		os.Exit(2)
	}
	if *displayName == "" {
		*displayName = *email
	}

	password, err := readPassword(os.Stdin)
	if err != nil {
		log.Fatalf("reading password: %v", err)
	}
	if len(password) < minPasswordLength {
		log.Fatalf("password must be at least %d characters", minPasswordLength)
	}

	cfg, err := config.Load("config")
	if err != nil {
		log.Fatalf("loading configuration: %v", err)
	}
	m, err := models.New(cfg)
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	defer func() { _ = m.Close() }()

	user, err := m.CreateUser(context.Background(), &models.CreateUserRequest{Email: *email, DisplayName: *displayName, Password: password})
	if err != nil {
		log.Fatalf("creating user: %v", err)
	}
	fmt.Printf("created user %d (%s)\n", user.Id, user.Email)
}

func readPassword(stdin *os.File) (string, error) {
	fd := int(stdin.Fd())
	if !term.IsTerminal(fd) {
		line, err := bufio.NewReader(stdin).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}

	fmt.Fprint(os.Stderr, "Password: ")
	first, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	fmt.Fprint(os.Stderr, "Again: ")
	second, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if string(first) != string(second) {
		return "", errors.New("passwords don't match")
	}
	return string(first), nil
}
