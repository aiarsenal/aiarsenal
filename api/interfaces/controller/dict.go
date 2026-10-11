package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

type DictController struct{ svc *service.DictService }

func NewDictController(s *service.DictService) *DictController { return &DictController{svc: s} }

// GetDictData 按字典类型取字典  GET /api/v1/system/dict/data/getDictData
func (h *DictController) GetDictData(c *gin.Context) {
	var req dto.GetDictReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	info, values, err := h.svc.GetDictData(c, req.DictType)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, dto.GetDictRes{Info: info, Values: values})
}
