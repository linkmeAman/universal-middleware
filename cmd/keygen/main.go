package main

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/linkmeAman/universal-middleware/internal/auth"
)

func main() {
	key, err := auth.GenerateRandomKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate key: %v\n", err)
		os.Exit(1)
	}

	encoded := base64.StdEncoding.EncodeToString(key)
	fmt.Printf("Generated session key: %s\n", encoded)
}
