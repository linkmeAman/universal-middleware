package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("Service error: %v\n", err)
		os.Exit(1)
	}
}
