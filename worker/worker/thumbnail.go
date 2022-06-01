package worker

import (
	"fmt"
	"github.com/joschahenningsen/thumbgen"
)

func CreateThumbnailSprite(ctx *StreamContext) error {
	// Create thumbnails
	progress := make(chan int)
	g, err := thumbgen.New(ctx.getTranscodingFileName(), 160, 100, ctx.getThumbnailFileName(), thumbgen.WithJpegCompression(70), thumbgen.WithProgressChan(&progress))
	if err != nil {
		fmt.Println(err)
	}
	go func() {
		for {
			p := <-progress
			if p == 100 {
				break
			}
		}
	}()
	err = g.Generate()
	return err
}
