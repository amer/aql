package main

import (
	"fmt"
	"os"

	"github.com/amer/aql/internal/aql"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	result := aql.Execute("SELECT * FROM users")
	fmt.Println(result)
	return nil
}
