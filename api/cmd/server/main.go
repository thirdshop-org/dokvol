package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("This binary has moved. Use: go run ./cmd/dokvol server")
	fmt.Println("Or: dokvol server")
	os.Exit(1)
}
