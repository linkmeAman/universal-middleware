package main

import (
	"fmt"
	"os"
)

func main() {
	if err := Run(); err != nil {
		fmt.Printf("Service error: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
