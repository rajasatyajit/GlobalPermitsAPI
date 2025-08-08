package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rajasatyajit/GlobalPermitsAPI/internal/generator"
)

func main() {
	in := flag.String("in", "input/globalpermits_doc.md", "Path to the GlobalPermits product description file (markdown or text)")
	out := flag.String("out", "output/GlobalPermits_Architecture.md", "Output path for the generated architecture blueprint")
	flag.Parse()

	if _, err := os.Stat(*in); err != nil {
		fmt.Fprintf(os.Stderr, "input file not found: %s (err=%v)\n", *in, err)
		os.Exit(1)
	}

	content, err := os.ReadFile(*in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read input: %v\n", err)
		os.Exit(1)
	}

	tpl := generator.DefaultTemplate()
	arch, err := generator.Generate(string(content), tpl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generation failed: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll("output", 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create output dir: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*out, []byte(arch), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write output: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Architecture blueprint generated at %s\n", *out)
}

