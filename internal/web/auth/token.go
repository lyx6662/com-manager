package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Claims Token声明
type Claims struct {
	Username string `json:"username"`
	Expire   int64  `json:"expire"`
}

// TokenManager Token管理器
type TokenManager struct {
	secret     string
	expireHour int
}

// NewTokenManager 创建Token管理器
func NewTokenManager(secret string, expireHour int) *TokenManager {
	if expireHour <= 0 {
		expireHour = 24
	}
	return &TokenManager{
		secret:     secret,
		expireHour: expireHour,
	}
}

// Generate 生成Token
func (m *TokenManager) Generate(username string) (string, error) {
	claims := Claims{
		Username: username,
		Expire:   time.Now().Add(time.Duration(m.expireHour) * time.Hour).Unix(),
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("序列化claims失败: %w", err)
	}

	encoded := base64.URLEncoding.EncodeToString(payload)
	signature := m.sign(encoded)

	return encoded + "." + signature, nil
}

// Validate 验证Token并返回Claims
func (m *TokenManager) Validate(tokenStr string) (*Claims, error) {
	parts := strings.SplitN(tokenStr, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("无效的token格式")
	}

	encoded, signature := parts[0], parts[1]

	// 验证签名
	expectedSig := m.sign(encoded)
	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return nil, fmt.Errorf("token签名验证失败")
	}

	// 解码payload
	payload, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("token解码失败: %w", err)
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("解析claims失败: %w", err)
	}

	// 检查过期时间
	if time.Now().Unix() > claims.Expire {
		return nil, fmt.Errorf("token已过期")
	}

	return &claims, nil
}

// sign 计算HMAC签名
func (m *TokenManager) sign(data string) string {
	mac := hmac.New(sha256.New, []byte(m.secret))
	mac.Write([]byte(data))
	return base64.URLEncoding.EncodeToString(mac.Sum(nil))
}
