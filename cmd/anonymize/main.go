// Command anonymize replaces the real identifiers in recorded API responses with
// the placeholders used in gcp/sampledata.
//
//	go run ./cmd/anonymize [-o dir] file.json...
//
// Each file is written to the directory given by -o, with the same name. See
// rules.go for what is replaced.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	outDir := flag.String("o", "gcp/sampledata", "directory the anonymized files are written to")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: anonymize [-o dir] file.json...")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	failed := false
	for _, in := range flag.Args() {
		out := filepath.Join(*outDir, filepath.Base(in))
		if err := run(in, out); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", in, err)
			failed = true
			continue
		}
		fmt.Printf("wrote %s\n", out)
	}
	if failed {
		os.Exit(1)
	}
}

func run(in, out string) error {
	data, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	result, err := anonymize(data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(out, result, 0o644)
}
