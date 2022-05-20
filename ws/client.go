package ws

import (
	"time"
	"log"
	"encoding/json"
	wss "github.com/gorilla/websocket"
	// 自己实现的模块
	"ipbot/token"
	"ipbot/util"
)
// 队列长度
const QueueSize = 5000

type ShardConfig struct {
	ShardID    uint32
	ShardCount uint32
}

// Session 连接的 session 结构，我去掉了暂时不需要的字段
type Session struct {
	ID      string
	URL     string
	Token   token.Token
	Intent  int
	// LastSeq uint32
	Shards  ShardConfig
}

// WebsocketAP wss 接入点信息
type WebsocketAP struct {
	URL               string            `json:"url"`
	Shards            uint32            `json:"shards"`
	//SessionStartLimit SessionStartLimit `json:"session_start_limit"`
}

type closeErrorChan chan error
type messageChan chan *base.WSPayload

// 用户信息
type WSUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Bot      bool   `json:"bot"`
}

// Client websocket 连接客户端
type Client struct {
	user            *WSUser     // 机器人的一些信息
	messageQueue    messageChan // 用于消息传递
	conn            *wss.Conn   // 连接信息，用于接收消息
	closeChan       closeErrorChan
	session         *Session
	heartBeatTicker *time.Ticker // 用于维持定时心跳
}

// WSIdentityData 鉴权数据
// 本来想去掉，但是因为建立连接前必须鉴权，所以保留了
type WSIdentityData struct {
	Token      string   `json:"token"`
	Intents    int   `json:"intents"`
	Shard      []uint32 `json:"shard"` 
	Properties struct {
		Os      string `json:"$os,omitempty"`
		Browser string `json:"$browser,omitempty"`
		Device  string `json:"$device,omitempty"`
	} `json:"properties,omitempty"`
}

// 启动一个会话，目前只支持建立一个会话
func Start(aiInfo *WebsocketAP, token *token.Token, intents *int) {
	session := Session{
		URL:     aiInfo.URL,
		Token:   *token,
		Intent:  *intents,
		// LastSeq: 0,
		Shards: ShardConfig{
			ShardID:    0,
			ShardCount: aiInfo.Shards,
		},
	}
	// 开启websocket连接
	newConnect(session)
}

func newConnect(session Session) {
	// 创建一个客户端
	wsClient := New(session)
	// 建立连接
	if err := wsClient.Connect(); err != nil {
		log.Println(err)
		return
	}

	// 初次鉴权
    wsClient.Identify()
	// 监听连接
	if err := wsClient.Listening(); err != nil {
		log.Println("[ws/session] Listening err %+v", err)
		return
	}
}

// New 新建一个连接对象
func New(session Session) *Client {
	return &Client{
		messageQueue:    make(messageChan, QueueSize),
		session:         &session,
		closeChan:       make(closeErrorChan, 10),
		heartBeatTicker: time.NewTicker(60 * time.Second), // 先给一个默认 ticker，在收到 hello 包之后，会 reset
	}
}

func (c *Client) Connect() error {
	// if c.session.URL == "" {
	// 	return 
	// }
	var err error
	c.conn, _, err = wss.DefaultDialer.Dial(c.session.URL, nil)
	if err != nil {
		log.Printf("%+v, connect err: %v\n", c.session, err)
		return err
	}
	log.Printf("%+v, url %s, connected\n", c.session, c.session.URL)
	return nil
}


// Identify 对一个连接进行鉴权，并声明监听的 shard 信息
func (c *Client) Identify() error {
	payload := &base.WSPayload{
		Data: &WSIdentityData{
			Token:   c.session.Token.GetString(),
			Intents: c.session.Intent,
			Shard: []uint32{
			 	c.session.Shards.ShardID,
			 	c.session.Shards.ShardCount,
			},
		},
	}
	// 鉴权的操作码
	payload.OPCode = 2
	return c.Write(payload)
}

// Listening 开始监听，会阻塞进程，内部会从事件队列不断的读取事件，解析后投递到注册的 event handler，如果读取消息过程中发生错误，会循环
func (c *Client) Listening() error {
	defer c.Close()
	// 读取数据
	go c.readMessageToQueue()
	// 从消息队列里面取出数据，并进行处理
	go c.listenMessageAndHandle()

	// handler message
	for {
		select {
		case err := <-c.closeChan:
			log.Printf("%+v Listening stop. err is %v\n", c.session, err)
			return err
		case <-c.heartBeatTicker.C:
			// 处理心跳事件
			log.Printf("%s listened heartBeat\n", c.session)
			heartBeatEvent := &base.WSPayload{
				WSPayloadBase: base.WSPayloadBase{
					OPCode: 1,
				},
			}
			// 不处理错误，Write 内部会处理，如果发生发包异常，会通知主协程退出
			_ = c.Write(heartBeatEvent)
		}
	}
}

func (c *Client) readMessageToQueue() {
	// 死循环读取用户数据
	for {
		// 读取用户的数据
		_, message, err := c.conn.ReadMessage()
		log.Printf("Message=%v\n", string(message))
		if err != nil {
			log.Printf("%s read message failed, %v, message %s\n", c.session, err, string(message))
			close(c.messageQueue)
			c.closeChan <- err
			return
		}
		// 解析有效载荷数据
		payload := &base.WSPayload{}
		// Json解码
		if err := json.Unmarshal(message, payload); err != nil {
			log.Printf("%s json failed, %v\n", c.session, err)
			continue
		}
		// 有效载荷的二进制
		payload.RawMessage = message
		
		// 处理内置的一些事件，如果处理成功，则这个事件不再投递给业务
		if c.isHandleBuildIn(payload) {
			continue
		}
		c.messageQueue <- payload
	}
}

func (c *Client) listenMessageAndHandle() {
	// 循环读取队列里的数据, 没有数据时会阻塞
	for payload := range c.messageQueue {
		// ready 事件提取用户信息
		if payload.Type == "READY" {
			c.readyHandler(payload)
			continue
		}
		// 解析具体事件
		log.Printf("------------Start to send data!-------------\n")
		if err := base.EventHandler(payload, payload.RawMessage); err != nil {
			log.Printf("%s atMassageEventHandler failed, %v\n", c.session, err)
		}
	}
	log.Printf("%s message queue is closed\n", c.session)
}

// Write 往 ws 写入数据
func (c *Client) Write(message *base.WSPayload) error {
	// 解析Json文件
	m, _ := json.Marshal(message)
	// 发送数据
	if err := c.conn.WriteMessage(wss.TextMessage, m); err != nil {
		log.Printf("%+v write message failed, %v\n", c.session, err)
		c.closeChan <- err
		return err
	}
	return nil
}

// 默认借鉴官方的实现
// isHandleBuildIn 内置的事件处理，处理那些不需要业务方处理的事件
func (c *Client) isHandleBuildIn(payload *base.WSPayload) bool {
	switch payload.OPCode {
	case 10: // 接收到 hello 后需要开始发心跳
		c.startHeartBeatTicker(payload.RawMessage)
	default:
		return false
	}
	return true
}

// startHeartBeatTicker 启动定时心跳
func (c *Client) startHeartBeatTicker(message []byte) {
	helloData := &base.WSHelloData{}
	if err := base.ParseData(message, helloData); err != nil {
		log.Printf("%s hello data parse failed, %v, message %v\n", c.session, err, message)
	}
	// 根据 hello 的回包，重新设置心跳的定时器时间
	c.heartBeatTicker.Reset(time.Duration(helloData.HeartbeatInterval) * time.Millisecond)
}

// readyHandler 针对ready返回的处理，需要记录 sessionID 等相关信息
func (c *Client) readyHandler(payload *base.WSPayload) {
	// 解析数据
	readyData := &base.WSReadyData{}
	if err := base.ParseData(payload.RawMessage, readyData); err != nil {
		log.Printf("%+v parseReadyData failed, %v, message %v\n", c.session, err, payload.RawMessage)
	}
	// 基于 ready 事件，更新 session 信息
	c.session.ID = readyData.SessionID
	c.session.Shards.ShardID = readyData.Shard[0]
	c.session.Shards.ShardCount = readyData.Shard[1]
	c.user = &WSUser{
		ID:       readyData.User.ID,
		Username: readyData.User.Username,
		Bot:      readyData.User.Bot,
	}
}

// Close 关闭连接
func (c *Client) Close() {
	if err := c.conn.Close(); err != nil {
		log.Printf("%+v, close conn err: %v\n", c.session, err)
	}
	c.heartBeatTicker.Stop()
}
