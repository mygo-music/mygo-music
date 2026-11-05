package main

import (
	"log"

	"github.com/mygo-music/mygo-music/internal/app"
	"github.com/mygo-music/mygo-music/internal/source"
	neteasesource "github.com/mygo-music/mygo-music/internal/source/netease"
)

func main() {
	// NetEase is the one source there is, built in, until plugins bring others.
	if err := app.Run(source.NewRegistry(neteasesource.New())); err != nil {
		log.Fatal(err)
	}
}
