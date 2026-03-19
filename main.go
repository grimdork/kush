package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/grimdork/kush/internal/shell"
)

// version is populated at build time via -ldflags "-X main.version=..."
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	shortV := flag.Bool("v", false, "print version and exit (shorthand)")
	flag.Parse()

	if *showVersion || *shortV {
		fmt.Println(version)
		return
	}

	if err := shell.Run(); err != nil {
		log.Fatal(err)
	}
}
