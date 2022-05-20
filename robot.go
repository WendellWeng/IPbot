package main
import (
	"log"
	"os"
	"time"
	"context"
	"strings"
	"io/ioutil"
	"encoding/json"
	yaml "gopkg.in/yaml.v2" // 包重命名
	"github.com/go-resty/resty/v2"
	"github.com/go-redis/redis/v8"
	// 自己实现的库
	"ipbot/token"
    "ipbot/ws"
	"ipbot/util"
)
// URL
const (
	gatewayBotURI = "https://api.sgroup.qq.com/gateway/bot"
	messagesURI = "https://api.sgroup.qq.com/channels/{channel_id}/messages"
	// https://www.mxnzp.com/api/ip/aim_ip?ip=8877&app_id=jsnbfgooehkrhqgi&app_secret=Ny9XV2gvWmhjc2dsY0tESzh2NmJrQT09
	IPsearchURL = "http://www.mxnzp.com/api/ip/aim_ip?ip={ipstr}&app_id={app_id}&app_secret={app_secret}"
)

// 定义常量
const (
	GroupMsg = "/IP查询"
	DirectChatMsg = "/私信IP查询"
)

type Config struct {
	// 机器人appid
	AppID uint64 `yaml:"appid"`
	// 机器人token
	Token string `yaml:"token"`
	// IP查询的appid
	Aid string `yaml:"aid"`
	// IP查询的token
	Asecret string `yaml:"asecret"`
}

// 配置文件结构
var config Config
// 复用连接
var client *resty.Client
var ipclient *resty.Client
var rdb *redis.Client

// golang程序初始化先于main函数执行，由runtime进行初始化
func init() {
	content, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		log.Println("读取配置文件失败, err = ", err)
		os.Exit(1)
	}
	err = yaml.Unmarshal(content, &config)
	if err != nil {
		log.Println("解析配置文件失败, err = ", err)
		os.Exit(1)
	}
	// 声明Redis实例
	rdb = redis.NewClient(&redis.Options{
		Addr:	  "localhost:6379",
		Password: "", // no password set
		DB:		  0,  // use default DB
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	defer cancel()
	_, err = rdb.Ping(ctx).Result()
	if err != nil {
		log.Println("Redis启动失败, err = ", err)
		os.Exit(1)
	}
}

func main() {
	// 读取config文件, 并获取token
	token := token.BotToken(config.AppID, config.Token)

	// create a new go-resty client
	// 之后需要切换websocket协议
	client = resty.New().
			SetAuthToken(token.GetString()).
			SetAuthScheme(string(token.Type))
	// 发起HTTPS的GET请求， 并将结果注入WebsocketAP
	resp, err := client.R().SetResult(ws.WebsocketAP{}).Get(gatewayBotURI)
	
	if err != nil {
		log.Printf("HTTPS Get failed! %s\n", err)
	}

	wsap :=  resp.Result().(*ws.WebsocketAP)
	
	// 注册回调函数
	base.DefaultHandlers.MessageHandler = atMassageEventHandler

	// 临时使用这个intent, 不然验证不通过
	intents := 1073741824

	// 创建一个线程来处理查询
	ipclient = resty.New()

	// 启动Socket
	ws.Start(wsap, token, &intents)
}

// 调用client发送消息到QQ频道
func PostMessage(channelID string, msg *base.MessageToCreate) (*base.Message, error) {
	resp, err := client.R().SetResult(base.Message{}).
		SetPathParam("channel_id", channelID).
		SetBody(msg).
		Post(messagesURI)
	if err != nil {
		return nil, err
	}
	log.Printf("wiki~, 数据发送成功\n")
	return resp.Result().(*base.Message), nil
}

func atMassageEventHandler(event *base.WSPayload, data *base.WSATMessageData) error {
	// log.Printf("Data = %v\n", data.Content)
	res := base.ParseCommand(data.Content)
	status := res.Cmd
	contents := strings.Split(res.Content, " ")
	// 只识别输入的第一个指令
	content := contents[0]
	ctx := context.Background()

	// 首先在Redis数据库里面查询Key
	val, err := rdb.Get(ctx, content).Result()
	if err == redis.Nil {
		log.Println("key does not exist")
	} else if err != nil {
		panic(err)
	} else { // 存在IP
		// 解析结果
		// json反序列化
		ipres := &base.IPResult{}
		json.Unmarshal([]byte(val), ipres)
		// 直接发送
		PostMessage(data.ChannelID, &base.MessageToCreate{MsgID: data.ID, Embed: createEmbed(ipres)})
		return nil
	}

	// 如果未查询到IP，就通过HTTP进行查询，并缓存至数据库中，同时设置过期时间为一天
	switch status {
		case GroupMsg:
			if content == "" {
				PostMessage(data.ChannelID, &base.MessageToCreate{MsgID: data.ID, Content: "待查询的IP地址不能为空！"})
			} else { // 获取指定IP信息
				var IPRes *base.IPResp
				IPRes, _ = getInfoByIp(content)

				if IPRes.Success == 1 {
					PostMessage(data.ChannelID, &base.MessageToCreate{MsgID: data.ID, Embed: createEmbed(&IPRes.Data)})
					// 目前支持只存储正确IP的结果，对非法输入不进行存储，为了减少内存的压力
					// json序列化
    				jdata, _ := json.Marshal(IPRes.Data)
					// 采用字符串存储，将json转换成字符串进行存储
    				if err := rdb.Set(ctx, content, jdata, 60 * time.Second).Err(); err != nil {
       					panic(err)
   					}
				} else {
					PostMessage(data.ChannelID, &base.MessageToCreate{MsgID: data.ID, Content: "待查询的IP地址不合法！"})
				} 
			}
		case DirectChatMsg:
			PostMessage(data.ChannelID, &base.MessageToCreate{MsgID: data.ID, Content: "功能还在开发中，请耐心等待～"})
		default:
			PostMessage(data.ChannelID, &base.MessageToCreate{MsgID: data.ID, Content: "好气哦，小测没能识别出小主的指令～"})
	}
	return nil
}

// 获取IP数据
func getInfoByIp(ipstr string) (*base.IPResp, error) {
	// urlstr := "http://www.mxnzp.com/api/ip/aim_ip?ip=" + ipstr +"&app_id=jsnbfgooehkrhqgi&app_secret=Ny9XV2gvWmhjc2dsY0tESzh2NmJrQT09"
	// GET请求查询
	resp, err := ipclient.R().SetResult(base.IPResp{}).
				SetPathParams(map[string]string{"ipstr": ipstr, "app_id": config.Aid, "app_secret": config.Asecret}).
				Get(IPsearchURL)
	if err != nil {
		log.Println("HTTP请求失败, err = ", err)
		return nil, err
	}
	// 结构体格式
	return resp.Result().(*base.IPResp), nil
}

// 获取 Embed
func createEmbed(ipData *base.IPResult) *base.Embed {
    return &base.Embed{
        Title: "恭喜你，查询成功啦",
        Thumbnail: base.MessageEmbedThumbnail{
            URL: "https://tva1.sinaimg.cn/large/e6c9d24egy1h2bjbcbuokj20bw0bwaa5.jpg",
        },
        Fields: []*base.EmbedField{
            {
                Name: "IP地址: " + ipData.Ip,
            },
            {
                Name: "运营商: " + ipData.Isp,
            },
			{
				Name: "详细信息: " + ipData.Desc,
			},
        },
    }
}

