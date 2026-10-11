package dto

import (
	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
)

// ===== 登录日志 =====
type LoginLogSearchReq struct {
	LoginName     string `json:"userName" form:"userName"`
	Status        string `json:"status" form:"status"`
	Ipaddr        string `json:"ipaddr" form:"ipaddr"`
	LoginLocation string `json:"loginLocation" form:"loginLocation"`
	PageReq
}
type LoginLogSearchRes struct {
	ListRes
	List []*entity.LoginLog `json:"list"`
}
type LoginLogDelReq struct {
	Ids []string `json:"ids" form:"ids" binding:"required"`
}
type LoginLogClearReq struct{}

// ===== 操作日志 =====
type OperLogSearchReq struct {
	Title         string `json:"title" form:"title"`
	RequestMethod string `json:"requestMethod" form:"requestMethod"`
	OperName      string `json:"operName" form:"operName"`
	PageReq
}
type OperLogSearchRes struct {
	ListRes
	List []*model.OperLogListRes `json:"list"`
}
type OperLogGetReq struct {
	OperId string `json:"operId" form:"operId" binding:"required"`
}
type OperLogDeleteReq struct {
	OperIds []string `json:"operIds" form:"operIds" binding:"required"`
}
type OperLogClearReq struct{}

// ===== 在线用户 =====
type UserOnlineSearchReq struct {
	Username string `json:"userName" form:"userName"`
	Ip       string `json:"ipaddr" form:"ipaddr"`
	PageReq
}
type UserOnlineSearchRes struct {
	ListRes
	List []*entity.UserOnline `json:"list"`
}
type UserOnlineDeleteReq struct {
	Uuid string `json:"uuid" form:"uuid" binding:"required"`
}
