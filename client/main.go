package main

import (
	"os"
	"os/signal"
)

var env string
var router Handlers

func main() {
	var interrupt = make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// 初始化事件处理器
	router = NewHandlers()
	router.setupEventHandlers()

	// 建立连接并开始读写ws
	conn := connect()
	client := NewClient(conn)
	client.run()

	// 阻塞主协程
	for {
		select {
		case <-interrupt:
			return
		}
	}
}
