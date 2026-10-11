package controller

import (
	"fmt"
	"net/http"
	"path"
	"strconv"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/middleware"
)

type SkillPackageController struct {
	pkgSvc *service.SkillPackageService
	svc    *service.SkillService
	embed  service.EmbeddingProvider
}

func NewSkillPackageController(p *service.SkillPackageService, s *service.SkillService, e service.EmbeddingProvider) *SkillPackageController {
	return &SkillPackageController{pkgSvc: p, svc: s, embed: e}
}

// Upload 上传技能包 zip，创建 DRAFT 版本并触发扫描。
// POST /api/v1/skill/skill/version/upload  (multipart: file, skillId, version, changelog)
func (h *SkillPackageController) Upload(c *gin.Context) {
	skillId := c.PostForm("skillId")
	version := c.PostForm("version")
	changelog := c.PostForm("changelog")
	if skillId == "" || version == "" {
		middleware.Fail(c, "skillId 与 version 不能为空")
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		middleware.Fail(c, "请上传 zip 包文件")
		return
	}
	versionId, err := h.pkgSvc.UploadVersion(c, skillId, version, changelog, fh, middleware.GetUserId(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"versionId": versionId})
}

// Search 语义检索技能。
// GET /api/v1/skill/skill/search?query=xxx&limit=10
func (h *SkillPackageController) Search(c *gin.Context) {
	q := c.Query("query")
	limit := 10
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	rows, err := h.svc.Search(c, q, limit, h.embed)
	if err != nil {
		middleware.Fail(c, "检索失败")
		return
	}
	middleware.Success(c, gin.H{"list": rows})
}

// Parse 预解析技能包 zip：解析 SKILL.md 返回字段供前端回填，并暂存到 staging。
// POST /api/v1/skill/skill/package/parse  (multipart: file=zip)
func (h *SkillPackageController) Parse(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		middleware.Fail(c, "请上传 zip 包文件")
		return
	}
	res, err := h.pkgSvc.ParsePackage(c, fh, middleware.GetUserId(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, res)
}

// Create 凭 tempId 提交创建 skill + 版本 + 持久化包。
// POST /api/v1/skill/skill/package/create  (JSON: CreatePackageReq)
func (h *SkillPackageController) Create(c *gin.Context) {
	var req service.CreatePackageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	skillId, versionId, err := h.pkgSvc.CreatePackage(c, &req, middleware.GetUserId(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"skillId": skillId, "versionId": versionId})
}

// ListFiles 版本文件列表。
// GET /api/v1/skill/skill/version/file/list?versionId=
func (h *SkillPackageController) ListFiles(c *gin.Context) {
	versionId := c.Query("versionId")
	if versionId == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	list, err := h.pkgSvc.GetVersionFiles(c, versionId)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

// DownloadFile 下载版本内单个文件。
// GET /api/v1/skill/skill/file/download?versionId=&filePath=
func (h *SkillPackageController) DownloadFile(c *gin.Context) {
	versionId := c.Query("versionId")
	filePath := c.Query("filePath")
	if versionId == "" || filePath == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	f, rc, err := h.pkgSvc.OpenFile(c, versionId, filePath)
	if err != nil {
		middleware.Fail(c, "文件不存在")
		return
	}
	defer rc.Close()
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, path.Base(filePath)))
	c.DataFromReader(http.StatusOK, -1, f.ContentType, rc, nil)
}

// DownloadVersion 下载整包 zip。
// GET /api/v1/skill/skill/version/download?versionId=
func (h *SkillPackageController) DownloadVersion(c *gin.Context) {
	versionId := c.Query("versionId")
	if versionId == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	filename, data, err := h.pkgSvc.DownloadVersion(c, versionId)
	if err != nil {
		middleware.Fail(c, "打包失败")
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/zip", data)
}

// DownloadPackage 供容器内 manager（InstanceAuth，instance token）拉取整包 zip。
// 与 DownloadVersion 同源（复用 pkgSvc.DownloadVersion），仅鉴权方式不同：本端点挂在
// /agent/runtime（InstanceAuth）而非 /skill（JWTAuth+CasbinAuth），让 manager 用自身凭证下载、
// 就地解压安装（install_skill 命令携带的 https 链接即指向此处）。
// GET /api/v1/agent/runtime/skill/package?versionId=
func (h *SkillPackageController) DownloadPackage(c *gin.Context) {
	versionId := c.Query("versionId")
	if versionId == "" {
		c.Data(http.StatusBadRequest, "text/plain; charset=utf-8", []byte("缺少 versionId"))
		return
	}
	// 安全门禁：只有审计「安全」的版本可取包安装（不安全/扫描中/扫描失败一律拒）。
	if err := h.pkgSvc.EnsureVersionSafe(c, versionId); err != nil {
		c.Data(http.StatusForbidden, "text/plain; charset=utf-8", []byte(err.Error()))
		return
	}
	filename, data, err := h.pkgSvc.DownloadVersion(c, versionId)
	if err != nil {
		c.Data(http.StatusInternalServerError, "text/plain; charset=utf-8", []byte("打包失败"))
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/zip", data)
}

// DownloadDevicePackage 供桌面客户端 App（设备 token）拉取整包 zip：设备管理页「技能管理」
// 下发 skill_install 后，App 凭自身 device token 下载、就地解压安装（packagePath 即指向此处）。
// 与 DownloadPackage 同源，仅端点归属不同（挂 clientAuth：JWTAuth+ClientDeviceAuth）。
// 仅接受设备 token（web 用户 JWT 虽过 ClientDeviceAuth，但无 deviceId，直接拒）。
// GET /api/v1/system/client/skill/package?versionId=
func (h *SkillPackageController) DownloadDevicePackage(c *gin.Context) {
	if middleware.GetUser(c).DeviceId == "" {
		c.Data(http.StatusUnauthorized, "text/plain; charset=utf-8", []byte("仅限设备 token 访问"))
		return
	}
	versionId := c.Query("versionId")
	if versionId == "" {
		c.Data(http.StatusBadRequest, "text/plain; charset=utf-8", []byte("缺少 versionId"))
		return
	}
	// 安全门禁：同 DownloadPackage（机器取包必须审计「安全」）。
	if err := h.pkgSvc.EnsureVersionSafe(c, versionId); err != nil {
		c.Data(http.StatusForbidden, "text/plain; charset=utf-8", []byte(err.Error()))
		return
	}
	filename, data, err := h.pkgSvc.DownloadVersion(c, versionId)
	if err != nil {
		c.Data(http.StatusInternalServerError, "text/plain; charset=utf-8", []byte("打包失败"))
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/zip", data)
}
