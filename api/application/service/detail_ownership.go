package service

import "errors"

// errDetailNotFound 详情越权的拒绝形态：与「资源不存在」同文案（报告修复建议：
// 不返回「数据不存在」以外的信息——存在性本身也不泄露）。
var errDetailNotFound = errors.New("未找到")

// F-017 详情路由横向越权：属主型资源的读取谓词收口（纯函数表测 + service 接线）。
// B 轮实测：列表被拒、详情放行的函数级授权不一致——零权限账号可横向读他人
// 用户档案、超管角色标识、MCP 服务完整配置（地址/请求头/环境变量）、技能与命名空间。
// PR #66（默认拒绝）闭合竖直半边（未授权访问）后，本文件闭合横向半边：
// 被授权读取的角色也只能读到「自己的/可见的」资源。
// 拒绝形态 = 「未找到」（不泄露资源存在性，报告修复建议原文）。

// canReadNamespace 命名空间详情：创建者 / 命名空间成员 / 超管。
func canReadNamespace(createdBy, callerId string, callerIsAdmin, isMember bool) bool {
	if callerIsAdmin {
		return true
	}
	if createdBy != "" && createdBy == callerId {
		return true
	}
	return isMember
}

// canReadSkill 技能详情：PUBLIC（visibility=0）登录即可读（市场浏览/安装流依赖）；
// PRIVATE（visibility=1）→ 属主 / 命名空间成员 / 超管。
func canReadSkill(ownerId string, visibility uint, callerId string, callerIsAdmin, isNamespaceMember bool) bool {
	if callerIsAdmin {
		return true
	}
	if visibility == 0 { // PUBLIC
		return true
	}
	if ownerId != "" && ownerId == callerId {
		return true
	}
	return isNamespaceMember
}

// canReadMCPServer MCP 服务详情（完整配置含地址/请求头/环境变量，敏感面）：
// 创建者 / 超管——不给成员（服务配置是全局设施不是个人协作件）。
func canReadMCPServer(createdBy, callerId string, callerIsAdmin bool) bool {
	if callerIsAdmin {
		return true
	}
	return createdBy != "" && createdBy == callerId
}
