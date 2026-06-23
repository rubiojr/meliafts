package action

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Handle reads the current [Event] and passes it to fn. If reading the event or
// fn fails, the error is written to stderr and the process exits with status 1.
// It is the one-call entry point for an action written in Go.
func Handle(fn func(Event) error) {
	ev, err := Read()
	if err == nil {
		err = fn(ev)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Read builds an [Event] from the process environment and standard input: the
// MELIAFTS_* variables and the JSON object meliafts pipes in. When stdin is
// empty it reconstructs the message from the environment instead, so an action
// can also be run by hand for testing.
func Read() (Event, error) {
	return decode(os.Getenv, os.Stdin)
}

// decode is the testable core of Read.
func decode(getenv func(string) string, stdin io.Reader) (Event, error) {
	ev := Event{
		Name:   getenv(EnvEvent),
		Query:  getenv(EnvQuery),
		DBPath: getenv(EnvDB),
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		return ev, fmt.Errorf("action: read stdin: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		ev.Message = messageFromEnv(getenv)
		return ev, nil
	}
	if err := json.Unmarshal(data, &ev.Message); err != nil {
		return ev, fmt.Errorf("action: decode message: %w", err)
	}
	return ev, nil
}

// messageFromEnv reconstructs a Message from the MELIAFTS_* environment, used
// when no JSON is supplied on stdin.
func messageFromEnv(getenv func(string) string) Message {
	return Message{
		ID:             getenv(EnvID),
		Date:           getenv(EnvDate),
		IsRead:         getenv(EnvUnread) != "1",
		IsFlagged:      getenv(EnvFlagged) == "1",
		HasAttachments: getenv(EnvHasAttachments) == "1",
		FromName:       getenv(EnvFromName),
		FromAddress:    getenv(EnvFromAddress),
		Subject:        getenv(EnvSubject),
		Snippet:        getenv(EnvSnippet),
	}
}
