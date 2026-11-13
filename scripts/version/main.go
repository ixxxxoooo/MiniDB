// Command version prints the app version from mygo.json, for CI.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	data, err := os.ReadFile("mygo.json")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var cfg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil || cfg.Version == "" {
		fmt.Fprintln(os.Stderr, "mygo.json has no version")
		os.Exit(1)
	}
	fmt.Println(cfg.Version)
}
