package common

import (
	"encoding/json"
	"time"
)

const (
	TEST = "test"
)

const (
	JenkinsJobMap = "JenkinsJobMap" // client发送自己管理的所有jenkins job给server
	BuildResult   = "BuildResult"   // 构建结果发送到server
	BuildStart    = "BuildStart"    // 开始构建的内容发送到server
	BuildEvent    = "BuildEvent"    // 审批事件由server发到client
)

// Event 最终所有要发送的事件对象都封装为 Event
type Event struct {
	// Type 消息类型
	Type string `json:"type"`
	// Payload 消息内容
	Payload json.RawMessage `json:"payload"`
}

// TestMessageEvent 测试消息 payload
type TestMessageEvent struct {
	Message string `json:"message"`
	From    string `json:"from"`
}
type NewTestMessageEvent struct {
	TestMessageEvent
	Sent time.Time `json:"sent"`
}
