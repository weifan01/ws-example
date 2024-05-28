package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"ws-example/common"

	"github.com/gorilla/websocket"
)

/*
Client 连接对象，用来收发消息
*/

type Client struct {
	connection *websocket.Conn
	// env 表明当前环境test/uat/prod，server定向发送消息
	env string
	// 发消息无缓冲通道，避免并发写
	egress chan []byte
}

// NewClient 初始化 Client 对象
func NewClient(conn *websocket.Conn) *Client {
	return &Client{
		connection: conn,
		env:        os.Getenv("environment"),
		egress:     make(chan []byte),
	}
}

func (c *Client) run() {
	go c.readMessages()
	go c.writeMessages()

	// 启动发送测试消息
	go c.produceMessage()
}

func (c *Client) writeMessages() {
	var interrupt = make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	// ticker 同方法发送ping，以避免对ws连接的并发写，比单独定义事件类型实现更简单
	ticker := time.NewTicker(pingInterval)
	defer func() {
		c.connection.Close()
		ticker.Stop()
		c.reconnect()
	}()
	for {
		select {
		case message, ok := <-c.egress:
			// 如果通道关闭，则ok为false
			if !ok {
				// 给服务端发送 CloseMessage
				if err := c.connection.WriteMessage(websocket.CloseMessage, nil); err != nil {
					log.Println("connection closed: ", err)
				}
				// 停止协程，重连服务端
				return
			}

			if err := c.connection.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Println(err)
			}
			log.Println("sent message")
		case <-ticker.C:
			if err := c.connection.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Println("send ping: ", err)
				return
			}
		case <-interrupt:
			// 捕获到中断信号则给服务端发送关闭连接消息然后退出进程
			err := c.connection.WriteMessage(
				websocket.CloseMessage, websocket.FormatCloseMessage(
					websocket.CloseNormalClosure, "",
				),
			)
			if err != nil {
				log.Println("write close:", err)
				log.Println("capture interrupt signal, exiting...")
				os.Exit(0)
			}
		}
	}
}

func (c *Client) readMessages() {
	defer func() {
		c.connection.Close()
		c.reconnect()
	}()

	// 处理pong消息，未收到pong消息表示连接状态已被破坏，读取消息会返回错误，此时客户端需要重连
	if err := c.connection.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		log.Println(err)
		return
	}
	c.connection.SetPongHandler(c.pongHandler)

	c.connection.SetReadLimit(readMaxSize)

	for {
		_, payload, err := c.connection.ReadMessage()
		if err != nil {
			// 如果从已关闭的连接中读取消息则err不为空
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error reading message: %v", err)
			}
			break
		}

		// 序列化消息内容为 Event 对象
		var request common.Event
		if err := json.Unmarshal(payload, &request); err != nil {
			log.Printf("error marshalling message: %v", err)
			break // TODO Breaking the connection here might be harsh xD
		}

		if err := router.routeEvent(request, c); err != nil {
			log.Println("Error handling Message: ", err)
		}
	}
}

var (
	// pongWait 响应pong包之间的超时时间
	pongWait = 10 * time.Second
	// 必须要小于pongWait时间，否则响应包未收到就已经超时了
	pingInterval = (pongWait * 9) / 10
	readMaxSize  = int64(102400)
)

// pongHandler 处理来自客户端的 PongMessages，重置deadline
func (c *Client) pongHandler(pongMsg string) error {
	log.Println("received pong")
	return c.connection.SetReadDeadline(time.Now().Add(pongWait))
}

func (c *Client) produceMessage() {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	// 用来结束生产者
	done := make(chan bool)
	go func() {
		time.Sleep(time.Second * 120)
		done <- true
	}()

	for {
		select {
		case <-done:
			fmt.Println("done")
			return
		case t := <-ticker.C:
			log.Println("produce a message, current time: ", t)
			testMessage := common.TestMessageEvent{
				TestMessage: common.TestMessage{
					Message: "This is a test message",
					From:    "dev",
				},
				Sent: t,
			}
			x, _ := json.Marshal(testMessage)
			event := common.Event{
				Type:    common.TEST,
				Payload: x,
			}
			d, _ := json.Marshal(event)
			c.egress <- d
		}
	}
}

// reconnect 重新连接服务端
func (c *Client) reconnect() {
	log.Println("reconnect to server")
	client := connect()
	c.connection = client
	c.run()
}

func connect() *websocket.Conn {
	// 加一个标识，server端用来做简单验证
	var header = make(http.Header)
	header.Add("identification", os.Getenv("identification"))
	conn, _, err := websocket.DefaultDialer.Dial(os.Getenv("serverAddress"), header)
	if err != nil {
		log.Fatalln("Error connect to server, ", err.Error())
	}
	return conn
}
