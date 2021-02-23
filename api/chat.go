package api

import (
	"github.com/gin-gonic/gin"
	"io"
	"time"
)

func configGinChatRouter(router gin.IRoutes) {
	router.GET("/chat", ChatStream)
}

func ChatStream(context *gin.Context) {
	chanStream := make(chan int, 10)
	go func() {
		defer close(chanStream)
		for i := 0; i < 5; i++ {
			chanStream <- i
			time.Sleep(time.Second * 1)
		}
	}()
	context.Stream(func(w io.Writer) bool {
		if msg, ok := <-chanStream; ok {
			context.SSEvent("message", msg)
			return true
		}
		return false
	})
}
