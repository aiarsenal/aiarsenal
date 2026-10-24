package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"aiarsenal/config"
	"aiarsenal/infrastructure/idgen"
)

// Claims JWT 载荷：登录用户信息（替代旧 gtoken 的服务端缓存 token）
type Claims struct {
	UserId     string   `json:"userId"`
	UserName   string   `json:"userName"`
	IsAdmin    int      `json:"isAdmin"`
	DeptId     string   `json:"deptId,omitempty"`     // 权限五元组-部门（登录时新鲜解析）
	PostIds    []string `json:"postIds,omitempty"`    // 权限五元组-岗位
	EmployeeNo string   `json:"employeeNo,omitempty"` // 工号（员工集合判定；登录时新鲜解析）
	DeviceId   string   `json:"deviceId,omitempty"`   // 桌面 App 登录携带；web 端为空
	InstanceId string   `json:"instanceId,omitempty"` // 容器内 agent 控制面 token 携带
	jwt.RegisteredClaims
}

// Generate 签发 JWT（web 端登录用，无设备绑定）。secret/超时来自 config.jwt 配置段。
// deptId/postIds/employeeNo 为权限身份的部门/岗位/工号维（角色维不走 claims，请求时由 casbin 实时解析）。
func Generate(userId string, userName string, isAdmin int, deptId string, postIds []string, employeeNo string) (string, error) {
	return GenerateForDevice(userId, userName, isAdmin, "", deptId, postIds, employeeNo)
}

// GenerateForDevice 签发带 deviceId 的 JWT（桌面 App 登录用，deviceId 非空）。
// deviceId 为空时等同 web 登录 token（向后兼容 Generate）。
// 每个登录令牌带 jti 并登记进会话注册表：退出/吊销才有服务端依据（见 session.go）。
func GenerateForDevice(userId string, userName string, isAdmin int, deviceId string, deptId string, postIds []string, employeeNo string) (string, error) {
	c := config.Get()
	timeout := c.Jwt.TimeOut
	if timeout <= 0 {
		timeout = 60 * 60 * 24 * 30
	}
	claims := Claims{
		UserId:     userId,
		UserName:   userName,
		IsAdmin:    isAdmin,
		DeptId:     deptId,
		PostIds:    postIds,
		EmployeeNo: employeeNo,
		DeviceId:   deviceId,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        idgen.New(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(timeout) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(c.Jwt.EncryptKey))
	if err != nil {
		return "", err
	}
	RegisterSession(userId, claims.ID)
	return signed, nil
}

// GenerateForInstance 签发带 instanceId 的 JWT（容器内 agent 控制面鉴权用）。
// userId 取实例创建者；instanceId 非空时 InstanceAuth 中间件据此放行 /agent/runtime/*。
func GenerateForInstance(userId string, instanceId string) (string, error) {
	c := config.Get()
	timeout := c.Jwt.TimeOut
	if timeout <= 0 {
		timeout = 60 * 60 * 24 * 365 // 实例 token 长效（1 年）
	}
	claims := Claims{
		UserId:     userId,
		InstanceId: instanceId,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(timeout) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString([]byte(c.Jwt.EncryptKey))
}

// GenerateDesktopAccess 签发短期 JWT（5 分钟），用于「打开桌面」反向代理的路径内鉴权。
// iframe 是导航请求，不带 Authorization 头；把 token 放进代理 URL 路径，
// 让桌面 SPA 的相对子请求（资产 / WebSocket signaling）自动携带，无需 cookie/CORS。
// 校验复用 Parse；handler 再校 claims.InstanceId 与路径 id 一致 + 实例归属。
func GenerateDesktopAccess(userId string, instanceId string) (string, error) {
	c := config.Get()
	ttl := 5 * 60 // 5 分钟
	claims := Claims{
		UserId:     userId,
		InstanceId: instanceId,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(ttl) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString([]byte(c.Jwt.EncryptKey))
}

// Parse 解析并校验 JWT，返回 Claims。
func Parse(tokenStr string) (*Claims, error) {
	c := config.Get()
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(c.Jwt.EncryptKey), nil
	})
	if err != nil {
		return nil, err
	}
	if !tok.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
