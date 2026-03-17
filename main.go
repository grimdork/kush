package main

import (
	"log"

	"github.com/grimdork/kush/internal/shell"
)

func main() {
	if err := shell.Run(); err != nil {
		log.Fatal(err)
	}
}
