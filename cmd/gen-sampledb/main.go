// Command gen-sampledb writes a sample melia database populated with random
// emails, for demos and manual testing. e2e tests build their own fixtures
// directly via the internal/sampledb package.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/rubiojr/meliafts/internal/sampledb"
	_ "modernc.org/sqlite"
)

func main() {
	out := flag.String("o", "melia.db", "output database path (replaced if it exists)")
	n := flag.Int("n", 140, "number of random messages to generate")
	seed := flag.Int64("seed", 1, "random seed for reproducibility")
	flag.Parse()

	if err := run(*out, *n, *seed); err != nil {
		fmt.Fprintln(os.Stderr, "gen-sampledb:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d random + curated messages, seed %d)\n", *out, *n, *seed)
}

func run(out string, n int, seed int64) error {
	if err := os.Remove(out); err != nil && !os.IsNotExist(err) {
		return err
	}
	return sampledb.Build(context.Background(), out, sampledb.Options{
		Seed:     seed,
		Messages: n,
		Now:      time.Now(),
	})
}
