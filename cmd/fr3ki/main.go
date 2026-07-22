package main

import (
	"flag"
	"fmt"
	"os"
)

const version = "0.1.0-go"

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Println("fr3ki", version)
		os.Exit(0)
	}

	fmt.Fprintln(os.Stderr, "fr3ki: no command given — run with --help")
	os.Exit(1)
}
