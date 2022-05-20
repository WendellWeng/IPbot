package token
import (
	"fmt"
)

const (
	TypeBot string = "Bot"
)

// Token 用于调用接口的 token 结构
type Token struct {
	AppID       uint64
	AccessToken string
	Type        string
}

// BotToken 机器人身份的 token
func BotToken(appID uint64, accessToken string) *Token {
	return &Token{
		AppID:       appID,
		AccessToken: accessToken,
		Type:        TypeBot,
	}
}
// GetString 获取授权头字符串
func (t *Token) GetString() string {
	return fmt.Sprintf("%v.%s", t.AppID, t.AccessToken)
}