package main

import (
	"embed"
	"fmt"
)

//go:embed assets
var content embed.FS

func main() {
	page, err := content.ReadFile("assets/index.html")
	if err != nil {
		panic(err)
	}
	fmt.Print(string(page))
}
