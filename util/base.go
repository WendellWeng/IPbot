package base
import (
	"regexp"
	"strings"
	"encoding/json"
	"github.com/tidwall/gjson"
)
// 用于过滤 at 结构的正则
var atRE = regexp.MustCompile(`<@!\d+>`)

// 用于过滤用户发送消息中的空格符号，\u00A0 是 &nbsp; 的 unicode 编码，某些 mac/pc 版本，连续多个空格的时候会转换成这个符号发送到后台
const spaceCharSet = " \u00A0"

// CMD 一个简单的指令结构
type CMD struct {
	Cmd     string
	Content string
}

type WSATMessageData Message
type MessageEventHandler func (*WSPayload, *WSATMessageData) error

var DefaultHandlers struct {
	MessageHandler MessageEventHandler
}

// WSHelloData hello 返回
type WSHelloData struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

// WSPayloadBase 基础消息结构
type WSPayloadBase struct {
	OPCode int    `json:"op"`
	Seq    uint32    `json:"s,omitempty"`
	Type   string `json:"t,omitempty"`
}

// WSPayload websocket 消息结构
type WSPayload struct {
	WSPayloadBase
	Data       interface{} `json:"d,omitempty"`
	RawMessage []byte      `json:"-"` // 原始的 message 数据
}

// WSReadyData ready，鉴权后返回
type WSReadyData struct {
	Version   int    `json:"version"`
	SessionID string `json:"session_id"`
	User      struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Bot      bool   `json:"bot"`
	} `json:"user"`
	Shard []uint32 `json:"shard"`
}

// MessageToCreate 发送消息结构体定义
type MessageToCreate struct {
	Content string `json:"content,omitempty"`
	Embed   *Embed `json:"embed,omitempty"`
	// Ark     *Ark   `json:"ark,omitempty"`
	Image   string `json:"image,omitempty"`
	// 要回复的消息id，为空是主动消息，公域机器人会异步审核，不为空是被动消息，公域机器人会校验语料
	MsgID            string                    `json:"msg_id,omitempty"`
	MessageReference *MessageReference         `json:"message_reference,omitempty"`
	Markdown         *Markdown                 `json:"markdown,omitempty"`
	// Keyboard         *keyboard.MessageKeyboard `json:"keyboard,omitempty"` // 消息按钮组件
	EventID          string                    `json:"event_id,omitempty"` // 要回复的事件id, 逻辑同MsgID
}

// MessageReference 引用消息
type MessageReference struct {
	MessageID             string `json:"message_id"`               // 消息 id
	IgnoreGetMessageError bool   `json:"ignore_get_message_error"` // 是否忽律获取消息失败错误
}

// Markdown markdown 消息
type Markdown struct {
	TemplateID int               `json:"template_id"` // 模版 id
	Params     []*MarkdownParams `json:"params"`      // 模版参数
	Content    string            `json:"content"`     // 原生 markdown
}

// MarkdownParams markdown 模版参数 键值对
type MarkdownParams struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

// Message 消息结构体定义
type Message struct {
	// 消息ID
	ID string `json:"id"`
	// 子频道ID
	ChannelID string `json:"channel_id"`
	// 频道ID
	GuildID string `json:"guild_id"`
	// 内容
	Content string `json:"content"`
	// 发送时间
	Timestamp string `json:"timestamp"`
	// 消息编辑时间
	EditedTimestamp string `json:"edited_timestamp"`
	// 是否@all
	MentionEveryone bool `json:"mention_everyone"`
	// 消息发送方
	Author *User `json:"author"`
	// 消息发送方Author的member属性，只是部分属性
	Member *Member `json:"member"`
	// 结构化消息-embeds
	Embeds []*Embed `json:"embeds"`
	// 消息中的提醒信息(@)列表
	Mentions []*User `json:"mentions"`
	// 私信消息
	DirectMessage bool `json:"direct_message"`
	// 子频道 seq，用于消息间的排序，seq 在同一子频道中按从先到后的顺序递增，不同的子频道之前消息无法排序
	SeqInChannel string `json:"seq_in_channel"`
	// 引用的消息
	MessageReference *MessageReference `json:"message_reference,omitempty"`
	// 私信场景下，该字段用来标识从哪个频道发起的私信
	SrcGuildID string `json:"src_guild_id"`
}

// Embed 结构
type Embed struct {
	Title       string                `json:"title,omitempty"`
	Description string                `json:"description,omitempty"`
	Prompt      string                `json:"prompt"` // 消息弹窗内容，消息列表摘要
	Thumbnail   MessageEmbedThumbnail `json:"thumbnail,omitempty"`
	Fields      []*EmbedField         `json:"fields,omitempty"`
}

// MessageEmbedThumbnail embed 消息的缩略图对象
type MessageEmbedThumbnail struct {
	URL string `json:"url"`
}
// EmbedField Embed字段描述
type EmbedField struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

// Member 群成员
type Member struct {
	GuildID  string    `json:"guild_id"`
	JoinedAt string `json:"joined_at"`
	Nick     string    `json:"nick"`
	User     *User     `json:"user"`
	Roles    []string  `json:"roles"`
	OpUserID string    `json:"op_user_id,omitempty"`
}

// 定义了返回IP数据结构
type IPResp struct {
	Success uint64 `json:"code,omitempty"` // 请求成功/失败
	Data IPResult `json:"data,omitempty"`    // 获取数据
	Msg string `json:"msg"`      // 请求失败，失败原语
}

type IPResult struct {
	Ip string `json:"ip"` // Ip地址
	Province string `json:"province"` // 省份
	ProvinceId uint64 `json:"provinceId"` // 省份的邮政编码
	City string `json:"city"` // 城市
	CityId uint64 `json:"cityId"` // 城市的邮政编码
	Isp string `json:"isp"` // 运营商
	Desc string `json:"desc"` // 详细描述
}

// User 用户
type User struct {
	ID               string `json:"id"`
	Username         string `json:"username"`
	Avatar           string `json:"avatar"`
	Bot              bool   `json:"bot"`
	UnionOpenID      string `json:"union_openid"`       // 特殊关联应用的 openid
	UnionUserAccount string `json:"union_user_account"` // 机器人关联的用户信息，与union_openid关联的应用是同一个
}

// 解析 Json 数据
func ParseData(message []byte, target interface{}) error {
	data := gjson.Get(string(message), "d")
	return json.Unmarshal([]byte(data.String()), target)
}

// 回调函数，执行用户定义的函数
func EventHandler(payload *WSPayload, message []byte) error {
	data := &WSATMessageData{}
	if err := ParseData(message, data); err != nil {
		return err
	}
	if DefaultHandlers.MessageHandler != nil {
		return DefaultHandlers.MessageHandler(payload, data)
	}
	return nil
}

// 解析用户输入
// ParseCommand 解析命令，支持 `{cmd} {content}` 的命令格式
func ParseCommand(input string) *CMD {
	input = ETLInput(input)
	s := strings.Split(input, " ")
	if len(s) < 2 {
		return &CMD{
			Cmd:     strings.Trim(input, spaceCharSet),
			Content: "",
		}
	}
	return &CMD{
		Cmd:     strings.Trim(s[0], spaceCharSet),
		Content: strings.Join(s[1:], " "),
	}
}

// ETLInput 清理输出
//  - 去掉@结构
//  - trim
func ETLInput(input string) string {
	etlData := string(atRE.ReplaceAll([]byte(input), []byte("")))
	etlData = strings.Trim(etlData, spaceCharSet)
	return etlData
}