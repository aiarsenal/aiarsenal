package dto

import "aiarsenal/domain/model"

// GetDictReq 按字典类型查询字典数据
type GetDictReq struct {
	DictType     string `json:"dictType" form:"dictType" binding:"required"`
	DefaultValue string `json:"defaultValue" form:"defaultValue"`
}

// GetDictRes 字典数据返回（前端 getDictData / useDict 消费）
type GetDictRes struct {
	Info   *model.DictTypeRes   `json:"info"`
	Values []*model.DictDataRes `json:"values"`
}
