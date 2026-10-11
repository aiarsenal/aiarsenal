package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/middleware"
)

// agent_volume.go — 持久卷管理（aa_agent_persistent_volume 接线，localdocker）。
// 请求/响应结构体内联在本文件（dto 目录不归本任务改）。

// AgentVolumeController 持久卷控制器。
type AgentVolumeController struct{ svc *service.AgentVolumeService }

// NewAgentVolumeController 构造。
func NewAgentVolumeController(svc *service.AgentVolumeService) *AgentVolumeController {
	return &AgentVolumeController{svc: svc}
}

// List 卷列表（GET /agent/volume/list，带 docker 实时元数据）。
func (h *AgentVolumeController) List(c *gin.Context) {
	list, err := h.svc.List(c)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"list": list, "total": len(list)})
}

// Get 卷详情（GET /agent/volume/:id）。
func (h *AgentVolumeController) Get(c *gin.Context) {
	v, err := h.svc.Get(c, c.Param("id"))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, v)
}

// VolumeCreateReq 建卷请求体。
// 表无自由展示名列：卷名即 volume_name（aiarsenal-vol-<bizId>，落库由服务生成）。
type VolumeCreateReq struct {
	StorageSizeGb int    `json:"storageSizeGb"` // 仅落库展示/配额参考（local driver 无容量上限）
	StorageClass  string `json:"storageClass"`
	MountPath     string `json:"mountPath"` // 缺省 /data/<volumeId>
}

// Create 建卷（POST /agent/volume/create）：真建 docker 卷 + 落库。
// 卷是管理员态资源（挂 agInst 管理路由），表无 created_by 列，操作留痕靠 Casbin 路由口径。
func (h *AgentVolumeController) Create(c *gin.Context) {
	var req VolumeCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	id, err := h.svc.Create(c, &entity.AgentPersistentVolume{
		StorageSizeGb: req.StorageSizeGb,
		StorageClass:  req.StorageClass,
		MountPath:     req.MountPath,
	})
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"persistentVolumeId": id})
}

// Delete 删卷（POST /agent/volume/delete {ids:[...]}）：绑定中的卷拒绝。
func (h *AgentVolumeController) Delete(c *gin.Context) {
	var req struct {
		Ids []string `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Delete(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{})
}

// Unbind 解绑卷（POST /agent/volume/:id/unbind）：恢复 available，实例侧数据不动。
func (h *AgentVolumeController) Unbind(c *gin.Context) {
	if err := h.svc.Unbind(c, c.Param("id")); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{})
}
