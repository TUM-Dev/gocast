package main

import "github.com/TUM-Dev/gocast/vod-service/internal"

func main() {
	app := internal.NewApp()
	app.Run()
}
