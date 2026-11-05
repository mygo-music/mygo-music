package main

import (
	"log"

	"github.com/mygo-music/mygo-music/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
