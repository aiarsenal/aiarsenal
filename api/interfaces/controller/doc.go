package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/model"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

type DocController struct {
	svc *service.DocService
}

func NewDocController(s *service.DocService) *DocController { return &DocController{svc: s} }

// Upload 上传文档（multipart：files 多文件 + paths 各自相对路径（目录上传，可空）+
// dirId 目标目录 + title?/summary?（仅单文件时生效））。
// zip 自动解包并按包内层级建目录；同内容（sha256 一致）自动跳过。
// POST /api/v1/doc/document/upload
func (h *DocController) Upload(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		middleware.Fail(c, "表单解析失败")
		return
	}
	fhs := form.File["files"]
	if len(fhs) == 0 {
		fhs = form.File["file"] // 兼容单文件老字段
	}
	if len(fhs) == 0 {
		middleware.Fail(c, "请选择文件")
		return
	}
	if len(fhs) > 200 {
		middleware.Fail(c, "单次最多 200 个文件")
		return
	}
	paths := form.Value["paths"]
	var entries []service.UploadEntry
	var total int64
	for i, fh := range fhs {
		if fh.Size > 64<<20 {
			middleware.Fail(c, fh.Filename+"：文件过大（上限 64MB）")
			return
		}
		total += fh.Size
		if total > 256<<20 {
			middleware.Fail(c, "单次上传总量超过 256MB")
			return
		}
		f, err := fh.Open()
		if err != nil {
			middleware.Fail(c, "文件读取失败")
			return
		}
		data, err := io.ReadAll(io.LimitReader(f, 64<<20+1))
		f.Close()
		if err != nil {
			middleware.Fail(c, "文件读取失败")
			return
		}
		if int64(len(data)) > 64<<20 {
			middleware.Fail(c, fh.Filename+"：文件过大（上限 64MB）")
			return
		}
		p := ""
		if i < len(paths) {
			p = paths[i]
		}
		entries = append(entries, service.UploadEntry{Name: filepath.Base(fh.Filename), Path: p, Data: data})
	}
	vis, err := h.parseUploadVis(c)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	res, err := h.svc.UploadFiles(c, entries, c.PostForm("title"), c.PostForm("summary"),
		c.PostForm("dirId"), middleware.GetUserId(c), vis)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, res)
}

// List 文档列表（dirId ”=全部；选中目录=含子树）
// GET /api/v1/doc/document/list
func (h *DocController) List(c *gin.Context) {
	var req dto.DocSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.List(c, req.KeyWords, req.FileType, req.DirId, req.PageNum, req.PageSize, middleware.GetIdentityPtr(c))
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.DocListRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

// Get 文档详情（元数据；全文走 /content）
// GET /api/v1/doc/document/get?docId=
func (h *DocController) Get(c *gin.Context) {
	docId := c.Query("docId")
	if docId == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	d, err := h.svc.Get(c, docId, middleware.GetIdentityPtr(c))
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, d)
}

// Content 文档全文（抽取清洗后）
// GET /api/v1/doc/document/content?docId=
func (h *DocController) Content(c *gin.Context) {
	docId := c.Query("docId")
	if docId == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	txt, err := h.svc.GetContent(c, docId, middleware.GetUserId(c), "ui", middleware.GetIdentityPtr(c))
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, gin.H{"docId": docId, "content": txt})
}

// Raw 原件流（预览/下载）。download=1 走 attachment（浏览器另存）。
// GET /api/v1/doc/document/raw?docId=&download=1
func (h *DocController) Raw(c *gin.Context) {
	docId := c.Query("docId")
	if docId == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	d, data, err := h.svc.GetRaw(c, docId, middleware.GetUserId(c), "ui", middleware.GetIdentityPtr(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	kind := "inline"
	if c.Query("download") == "1" {
		kind = "attachment"
	}
	// html 当纯文本回（浏览器直开 api 域下的上传 html 会执行脚本，自伤面也不留）
	mime := docMIME(d.FileType)
	c.Header("Content-Disposition", contentDisposition(kind, d.FileName))
	c.Data(http.StatusOK, mime, data)
}

// docMIME 按类型回 Content-Type（预览语义：能内联的给真实类型，其余二进制）。
func docMIME(t string) string {
	switch t {
	case "pdf":
		return "application/pdf"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "doc":
		return "application/msword"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "xlsm":
		return "application/vnd.ms-excel.sheet.macroEnabled.12"
	case "xls":
		return "application/vnd.ms-excel"
	case "pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case "ppt":
		return "application/vnd.ms-powerpoint"
	case "ofd":
		return "application/ofd"
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	case "bmp":
		return "image/bmp"
	case "txt", "md", "markdown", "csv", "log", "json", "html", "htm":
		return "text/plain; charset=utf-8"
	case "zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}

// contentDisposition 中文名走 RFC 5987（filename*=UTF-8”…），各浏览器统一识别。
func contentDisposition(kind, name string) string {
	if name == "" {
		return kind
	}
	return fmt.Sprintf("%s; filename*=UTF-8''%s", kind, url.PathEscape(name))
}

// Search 语义检索（向量 top-k 块）
// GET /api/v1/doc/document/search?query=&limit=
func (h *DocController) Search(c *gin.Context) {
	q := c.Query("query")
	limit := 10
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	rows, err := h.svc.Search(c, q, limit, middleware.GetUserId(c), "ui", middleware.GetIdentityPtr(c))
	if err != nil {
		middleware.Fail(c, "检索失败")
		return
	}
	middleware.Success(c, gin.H{"list": rows})
}

// Edit 编辑标题/摘要（自动重嵌）
// PUT /api/v1/doc/document/edit
func (h *DocController) Edit(c *gin.Context) {
	var req dto.DocEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Edit(c, req.DocId, req.Title, req.Summary, middleware.GetIdentityPtr(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// Update 更新文档原件（multipart: file, docId, title?, summary?）——内容变了全量重嵌
// POST /api/v1/doc/document/update
func (h *DocController) Update(c *gin.Context) {
	docId := c.PostForm("docId")
	if docId == "" {
		middleware.Fail(c, "docId 不能为空")
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		middleware.Fail(c, "请选择文件")
		return
	}
	if fh.Size > 64<<20 {
		middleware.Fail(c, "文件过大（上限 64MB）")
		return
	}
	f, err := fh.Open()
	if err != nil {
		middleware.Fail(c, "文件读取失败")
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 64<<20))
	if err != nil {
		middleware.Fail(c, "文件读取失败")
		return
	}
	if err := h.svc.ReplaceFile(c, docId, c.PostForm("title"), c.PostForm("summary"), fh.Filename, data, middleware.GetIdentityPtr(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// Delete 删除文档（连同分块与原件）
// DELETE /api/v1/doc/document/delete
func (h *DocController) Delete(c *gin.Context) {
	var req dto.DocDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Delete(c, req.Ids, middleware.GetIdentityPtr(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// Move 批量移动文档到目录（拖拽/移动弹窗；dirId ”=移回根）
// POST /api/v1/doc/document/move
func (h *DocController) Move(c *gin.Context) {
	var req dto.DocMoveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.MoveDocs(c, req.DocIds, req.DirId, middleware.GetIdentityPtr(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// Embed 重新嵌入：选中文档 / 整个目录（含子树）
// POST /api/v1/doc/document/embed
func (h *DocController) Embed(c *gin.Context) {
	var req dto.DocEmbedReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	done, skipped, firstErr := h.svc.ReembedScope(c, req.DocIds, req.DirId, middleware.GetIdentityPtr(c))
	if done == 0 && skipped > 0 && firstErr != "" {
		middleware.Fail(c, firstErr)
		return
	}
	middleware.Success(c, gin.H{"done": done, "skipped": skipped, "error": firstErr})
}

// parseUploadVis 解析上传批次的授权参数（multipart 表单值；不给 = 默认全员）
func (h *DocController) parseUploadVis(c *gin.Context) (*service.UploadVis, error) {
	v := c.PostForm("visibility")
	if v == "" && c.PostForm("readRules") == "" && c.PostForm("writeRules") == "" {
		return nil, nil // 未配授权：走默认全员
	}
	vis := 2
	if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 2 {
		vis = n
	}
	readRows, err := model.ParseGrantRows(c.PostForm("readRules"))
	if err != nil {
		return nil, errors.New("读条件：" + err.Error())
	}
	writeRows, err := model.ParseGrantRows(c.PostForm("writeRules"))
	if err != nil {
		return nil, errors.New("写条件：" + err.Error())
	}
	return &service.UploadVis{
		Visibility: uint(vis), ReadRows: readRows, WriteRows: writeRows, Actor: middleware.GetUserId(c),
	}, nil
}

// GrantGet 查询文档/目录的授权配置（回显与保存同门槛）
// GET /api/v1/doc/grant/get?targetType=doc&targetId=
func (h *DocController) GrantGet(c *gin.Context) {
	var req dto.DocGrantGetReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	info, err := h.svc.GetGrants(c, req.TargetType, req.TargetId, middleware.GetIdentityPtr(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, info)
}

// GrantSave 保存文档/目录的可见性档位 + 读/写条件行
// POST /api/v1/doc/grant/save
func (h *DocController) GrantSave(c *gin.Context) {
	var req dto.DocGrantSaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.SaveGrants(c, req.TargetType, req.TargetId, req.Visibility,
		req.ReadRules, req.WriteRules, middleware.GetIdentityPtr(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// Access 访问记录（谁检索/阅读过）
// GET /api/v1/doc/document/access
func (h *DocController) Access(c *gin.Context) {
	var req dto.DocAccessReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListAccess(c, req.Action, req.DocId, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, gin.H{"list": list, "current": pn, "total": total})
}
