package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// 响应结构与旧 libResponse.Response 一致：{code, data, message}
// code: 0=成功, -1=失败, 401=未登录

type Body struct {
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

// Success 成功返回 {code:0, data, message:"成功"}
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Body{Code: 0, Data: data, Message: "成功"})
}

// SuccessMsg 成功返回，自定义 message
func SuccessMsg(c *gin.Context, msg string, data interface{}) {
	c.JSON(http.StatusOK, Body{Code: 0, Data: data, Message: msg})
}

// Fail 失败返回 {code:-1, message}
func Fail(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, Body{Code: -1, Data: nil, Message: msg})
}

// FailWith 失败返回，自定义 code
func FailWith(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, Body{Code: code, Data: nil, Message: msg})
}

// Unauthorized 未登录/ token 失效，返回 401
func Unauthorized(c *gin.Context, msg string) {
	c.JSON(http.StatusUnauthorized, Body{Code: 401, Data: nil, Message: msg})
	c.Abort()
}
