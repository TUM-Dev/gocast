package main

import "github.com/joschahenningsen/TUM-Live/vod-service/internal"

func main() {
	app := internal.NewApp()
	app.Run()
}
