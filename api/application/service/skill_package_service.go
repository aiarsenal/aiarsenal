package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"path"
	"strings"

	"github.com/goccy/go-yaml"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/idgen"
	storagestg "github.com/aiarsenal/runtime-go/storage"
)

// 包大小/数量限制（单文件 10MB、总包 10MB、100 文件）。
const (
	maxSingleFileBytes = 10 * 1024 * 1024
	maxTotalSize       = 10 * 1024 * 1024
	maxFileCount       = 100
)

// SkillPackageService 技能包上传解析：解压 zip → 逐文件 sha256/存储 → 解析 SKILL.md 与 manifest
// → 创建 version + file 记录 → 更新搜索文档(含向量) → 触发安全扫描。
// storage 为 runtime-go storage.Storage 接口（local / s3 兼容 OSS/COS/MinIO/AWS）。
type SkillPackageService struct {
	repo    repository.ISkillRepository
	secRepo repository.ISkillSecurityRepository
	storage storagestg.Storage
	scanner *ScannerService
	embed   EmbeddingProvider
}

func NewSkillPackageService(r repository.ISkillRepository, sr repository.ISkillSecurityRepository, s storagestg.Storage, sc *ScannerService, e EmbeddingProvider) *SkillPackageService {
	return &SkillPackageService{repo: r, secRepo: sr, storage: s, scanner: sc, embed: e}
}

// ===== 内部结构 =====

// zipFileData 解析出的单个文件（持有内容供存储；包 ≤10MB，可接受）。
type zipFileData struct {
	FilePath    string
	FileSize    int64
	ContentType string
	Sha256      string
	Data        []byte
}

// zipInspectResult inspectZip 的产物（纯解析，不落盘、不建记录）。
type zipInspectResult struct {
	Files         []zipFileData
	TotalSize     int64
	SkillMDBody   string
	Frontmatter   map[string]interface{}
	ManifestBytes []byte
}

// ParsePackageResult 预解析返回（前端回填弹窗字段）。
type ParsePackageResult struct {
	TempId    string                 `json:"tempId"`
	Slug      string                 `json:"slug"`    // frontmatter.name 经 normalizeSlug
	Name      string                 `json:"name"`    // 原始 name，预填 displayName
	Summary   string                 `json:"summary"` // frontmatter.description
	Version   string                 `json:"version"` // frontmatter.version（可空）
	License   string                 `json:"license"` // frontmatter.license（可空）
	FileCount int                    `json:"fileCount"`
	TotalSize int64                  `json:"totalSize"`
	Metadata  map[string]interface{} `json:"metadata"` // 完整 frontmatter
}

// CreatePackageReq 提交创建 skill（凭预解析返回的 tempId）。
type CreatePackageReq struct {
	TempId      string `json:"tempId"`
	NamespaceId string `json:"namespaceId"`
	Slug        string `json:"slug"`
	DisplayName string `json:"displayName"`
	Summary     string `json:"summary"`
	Visibility  uint   `json:"visibility"`
	Status      uint   `json:"status"`
	Version     string `json:"version"`
	Changelog   string `json:"changelog"`
}

// ===== inspectZip: 纯解析 =====

func inspectZip(raw []byte) (*zipInspectResult, error) {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, errors.New("无法解析 zip 包")
	}
	if len(zr.File) > maxFileCount {
		return nil, errors.New("包内文件数量超过上限(100)")
	}
	ins := &zipInspectResult{}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > maxSingleFileBytes {
			return nil, errors.New("包内单文件超过上限(10MB): " + f.Name)
		}
		base := normalizeZipPath(f.Name)
		if base == "" {
			continue
		}
		// 路径穿越保护
		if strings.Contains(base, "..") {
			return nil, errors.New("包内含非法路径: " + f.Name)
		}
		if ins.TotalSize+int64(len(data)) > maxTotalSize {
			return nil, errors.New("包总大小超过上限(10MB)")
		}
		sum := sha256.Sum256(data)
		ins.Files = append(ins.Files, zipFileData{
			FilePath:    base,
			FileSize:    int64(len(data)),
			ContentType: detectContentType(base, data),
			Sha256:      hex.EncodeToString(sum[:]),
			Data:        data,
		})
		ins.TotalSize += int64(len(data))

		lower := strings.ToLower(path.Base(base))
		if lower == "skill.md" {
			ins.SkillMDBody, ins.Frontmatter = parseSkillMD(data)
		} else if lower == "manifest.json" {
			ins.ManifestBytes = data
		}
	}
	if len(ins.Files) == 0 {
		return nil, errors.New("包内无有效文件")
	}
	return ins, nil
}

// persistVersionFiles 存储所有文件并建 SkillFile 记录（需 versionId）。
func (s *SkillPackageService) persistVersionFiles(ctx context.Context, skillId, versionId, version string, ins *zipInspectResult) ([]*entity.SkillFile, error) {
	files := make([]*entity.SkillFile, 0, len(ins.Files))
	for _, fd := range ins.Files {
		storageKey := path.Join("skill", skillId, version, fd.FilePath)
		if _, err := s.storage.Save(ctx, storageKey, bytes.NewReader(fd.Data)); err != nil {
			slog.Error("skill 文件存储失败", "skillId", skillId, "versionId", versionId, "file", fd.FilePath, "err", err)
			return nil, err
		}
		f := &entity.SkillFile{
			VersionId:   versionId,
			FilePath:    fd.FilePath,
			FileSize:    fd.FileSize,
			ContentType: fd.ContentType,
			Sha256:      fd.Sha256,
			StorageKey:  storageKey,
		}
		if err := s.repo.AddFile(ctx, f); err != nil {
			slog.Error("skill file AddFile 失败", "skillId", skillId, "versionId", versionId, "file", fd.FilePath, "err", err)
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

// finalizeVersion 组装 SkillVersion(DRAFT) 入库 + 搜索文档 + 安全扫描。fileCount/totalSize 由解析期已知。
func (s *SkillPackageService) finalizeVersion(ctx context.Context, sk *entity.Skill, version, changelog string, ins *zipInspectResult, fileCount int, totalSize int64, userId string) (string, error) {
	// jsonb 列保持 nil→NULL 语义，勿赋空串（MEMORY jsonb-column-empty-string-gotcha）
	var metaJSON, manifestJSON json.RawMessage
	if ins.Frontmatter != nil {
		metaJSON, _ = json.Marshal(ins.Frontmatter)
	}
	if ins.ManifestBytes != nil {
		manifestJSON = ins.ManifestBytes
	}
	v := &entity.SkillVersion{
		SkillId:            sk.SkillId,
		Version:            version,
		Status:             0, // DRAFT
		Changelog:          changelog,
		ParsedMetadataJson: metaJSON,
		ManifestJson:       manifestJSON,
		FileCount:          fileCount,
		TotalSize:          totalSize,
		BundleReady:        1,
		DownloadReady:      0,
		CreatedBy:          userId,
	}
	versionId, err := s.repo.AddVersion(ctx, v)
	if err != nil {
		slog.Error("skill version AddVersion 失败", "skillId", sk.SkillId, "version", version, "err", err)
		return "", err
	}
	s.upsertSearchDoc(ctx, sk, ins.Frontmatter, ins.SkillMDBody, ins.ManifestBytes)
	if s.scanner != nil {
		auditId, _ := s.secRepo.Rescan(ctx, versionId)
		if auditId != "" {
			s.scanner.ScanVersionAsync(versionId, auditId)
		}
	}
	slog.Info("skill version 创建完成", "skillId", sk.SkillId, "versionId", versionId, "version", version, "fileCount", fileCount)
	return versionId, nil
}

// createVersionFromZip inspect→finalize→persist 编排（skill 须已存在）。三入口共用。
func (s *SkillPackageService) createVersionFromZip(ctx context.Context, sk *entity.Skill, version, changelog string, raw []byte, userId string) (string, error) {
	ins, err := inspectZip(raw)
	if err != nil {
		return "", err
	}
	versionId, err := s.finalizeVersion(ctx, sk, version, changelog, ins, len(ins.Files), ins.TotalSize, userId)
	if err != nil {
		return "", err
	}
	if _, err := s.persistVersionFiles(ctx, sk.SkillId, versionId, version, ins); err != nil {
		return versionId, err // version 已建，文件失败带 err 返回（上传失败可重试）
	}
	return versionId, nil
}

// UploadVersion 给已存在 skill 上传新版本 zip（前端版本管理弹窗用）。
func (s *SkillPackageService) UploadVersion(ctx context.Context, skillId, version, changelog string, fh *multipart.FileHeader, userId string) (string, error) {
	if skillId == "" || version == "" || fh == nil {
		return "", errors.New("skillId/version/包文件不能为空")
	}
	raw, err := readZipBytes(fh)
	if err != nil {
		return "", err
	}
	sk, err := repoGetSkill(s.repo, ctx, skillId)
	if err != nil {
		return "", err
	}
	return s.createVersionFromZip(ctx, sk, version, changelog, raw, userId)
}

// ParsePackage 预解析 zip：解析 SKILL.md → 回填字段 + 暂存到 staging（按用户隔离）。
func (s *SkillPackageService) ParsePackage(ctx context.Context, fh *multipart.FileHeader, userId string) (*ParsePackageResult, error) {
	if fh == nil {
		return nil, errors.New("请上传 zip 包文件")
	}
	raw, err := readZipBytes(fh)
	if err != nil {
		return nil, err
	}
	ins, err := inspectZip(raw)
	if err != nil {
		return nil, err
	}
	fm := ins.Frontmatter
	name, _ := fm["name"].(string)
	if name == "" {
		return nil, errors.New("SKILL.md 缺少必需的 name 字段")
	}
	summary, _ := fm["description"].(string)
	if summary == "" {
		summary = getString(fm, "summary")
	}
	tempId := idgen.New()
	stagingKey := path.Join("staging", userId, tempId+".zip")
	if _, err := s.storage.Save(ctx, stagingKey, bytes.NewReader(raw)); err != nil {
		slog.Error("skill 包 staging 暂存失败", "userId", userId, "err", err)
		return nil, errors.New("压缩包暂存失败")
	}
	slog.Info("skill 包预解析完成", "userId", userId, "tempId", tempId, "slug", normalizeSlug(name), "fileCount", len(ins.Files), "totalSize", ins.TotalSize)
	return &ParsePackageResult{
		TempId:    tempId,
		Slug:      normalizeSlug(name),
		Name:      orDefault(strings.TrimSpace(getString(fm, "title")), name), // 创建表单预填中文显示名
		Summary:   summary,
		Version:   getString(fm, "version"),
		License:   getString(fm, "license"),
		FileCount: len(ins.Files),
		TotalSize: ins.TotalSize,
		Metadata:  fm,
	}, nil
}

// CreatePackage 凭 tempId 提交创建 skill + 版本 + 持久化包。
func (s *SkillPackageService) CreatePackage(ctx context.Context, req *CreatePackageReq, userId string) (skillId, versionId string, err error) {
	if req.TempId == "" || req.NamespaceId == "" || req.Slug == "" || req.DisplayName == "" || req.Version == "" {
		return "", "", errors.New("tempId/namespaceId/slug/displayName/version 不能为空")
	}
	// 1. 取 staging zip
	stagingKey := path.Join("staging", userId, req.TempId+".zip")
	rc, err := s.storage.Open(ctx, stagingKey)
	if err != nil {
		if storagestg.IsNotFound(err) {
			return "", "", errors.New("压缩包已失效，请重新上传")
		}
		return "", "", err
	}
	raw, rerr := io.ReadAll(rc)
	rc.Close()
	if rerr != nil {
		return "", "", rerr
	}
	// 2. 创建 skill（ownerId=当前用户）
	status := req.Status
	if status == 0 {
		status = 1 // 默认 ACTIVE
	}
	sk := &entity.Skill{
		NamespaceId: req.NamespaceId,
		Slug:        normalizeSlug(req.Slug),
		DisplayName: req.DisplayName,
		Summary:     req.Summary,
		OwnerId:     userId,
		Visibility:  req.Visibility,
		Status:      status,
		CreatedBy:   userId,
	}
	skillId, err = s.repo.Add(ctx, sk)
	if err != nil {
		slog.Error("skill Add 失败", "namespaceId", req.NamespaceId, "slug", sk.Slug, "err", err)
		_ = s.storage.Delete(ctx, stagingKey) // 清理 staging
		return "", "", err
	}
	sk.SkillId = skillId
	// 3. 复用编排创建版本
	versionId, err = s.createVersionFromZip(ctx, sk, req.Version, req.Changelog, raw, userId)
	// 4. 删 staging（best-effort）
	if derr := s.storage.Delete(ctx, stagingKey); derr != nil {
		slog.Warn("staging 清理失败", "stagingKey", stagingKey, "err", derr)
	}
	if err != nil {
		slog.Error("skill 包版本创建失败", "skillId", skillId, "err", err)
		return skillId, "", err
	}
	slog.Info("skill 包创建完成", "skillId", skillId, "versionId", versionId, "userId", userId, "slug", sk.Slug)
	return skillId, versionId, nil
}

// GetVersionFiles 版本的文件列表。
func (s *SkillPackageService) GetVersionFiles(ctx context.Context, versionId string) ([]*entity.SkillFile, error) {
	return s.repo.ListFiles(ctx, versionId)
}

// OpenFile 打开某版本某文件，返回文件记录 + 读取流（调用方负责 Close）。
func (s *SkillPackageService) OpenFile(ctx context.Context, versionId, filePath string) (*entity.SkillFile, io.ReadCloser, error) {
	f, err := s.repo.GetFile(ctx, versionId, filePath)
	if err != nil {
		return nil, nil, err
	}
	rc, err := s.storage.Open(ctx, f.StorageKey)
	if err != nil {
		return nil, nil, err
	}
	return f, rc, nil
}

// EnsureVersionSafe 设备/实例取包前的安全门禁：最新审计须为「安全」。
// 仅拦机器取包端点（DownloadPackage/DownloadDevicePackage）；管理端 /version/download
// 不拦（供管理员检查不安全包内容用）。
func (s *SkillPackageService) EnsureVersionSafe(ctx context.Context, versionId string) error {
	return ensureSkillVersionSafe(ctx, s.secRepo, versionId)
}

// DownloadVersion 把版本所有文件打包成 zip 返回（filename + bytes）。
func (s *SkillPackageService) DownloadVersion(ctx context.Context, versionId string) (string, []byte, error) {
	files, err := s.repo.ListFiles(ctx, versionId)
	if err != nil {
		return "", nil, err
	}
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for _, f := range files {
		rc, e := s.storage.Open(ctx, f.StorageKey)
		if e != nil {
			continue
		}
		w, e := zw.Create(f.FilePath)
		if e != nil {
			rc.Close()
			continue
		}
		_, _ = io.Copy(w, rc)
		rc.Close()
	}
	if err := zw.Close(); err != nil {
		return "", nil, err
	}
	name := versionId
	if ver, e := s.repo.GetVersion(ctx, versionId); e == nil && ver != nil {
		name = ver.Version
	}
	return name + ".zip", buf.Bytes(), nil
}

// readZipBytes 从 multipart FileHeader 读出全部字节。
func readZipBytes(fh *multipart.FileHeader) ([]byte, error) {
	src, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()
	return io.ReadAll(src)
}

func getString(m map[string]interface{}, k string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func (s *SkillPackageService) upsertSearchDoc(ctx context.Context, sk *entity.Skill, fm map[string]interface{}, body string, manifest []byte) {
	title := sk.DisplayName
	summary := sk.Summary
	keywords := ""
	if fm != nil {
		if v, ok := fm["name"].(string); ok && v != "" {
			title = v
		}
		if v, ok := fm["summary"].(string); ok && v != "" {
			summary = v
		}
		if v, ok := fm["description"].(string); ok && v != "" {
			summary = v
		}
		if kw, ok := fm["keywords"].([]interface{}); ok {
			ks := make([]string, 0, len(kw))
			for _, k := range kw {
				if ks2, ok := k.(string); ok {
					ks = append(ks, ks2)
				}
			}
			keywords = strings.Join(ks, ",")
		}
	}
	if len(title) > 500 {
		title = title[:500] // aa_skill_search_document.title varchar(512) 防超长
	}
	searchText := body
	if len(manifest) > 0 {
		searchText += "\n" + string(manifest)
	}
	if len(searchText) > 8000 {
		searchText = searchText[:8000]
	}
	vecText := ""
	if s.embed != nil {
		if vec, err := s.embed.Embed(title + " " + summary + " " + keywords + " " + searchText); err == nil {
			vecText = VectorToPGText(vec)
		}
	}
	_ = s.repo.UpsertSearchDocument(ctx, sk.SkillId, title, summary, keywords, searchText,
		sk.NamespaceId, "", sk.OwnerId, sk.Visibility, sk.Status, vecText)
}

// repoGetSkill 取技能主表。
func repoGetSkill(r repository.ISkillRepository, ctx context.Context, skillId string) (*entity.Skill, error) {
	return r.Get(ctx, skillId)
}

// parseSkillMD 解析 SKILL.md：首部 --- YAML frontmatter，余下为正文。
func parseSkillMD(data []byte) (string, map[string]interface{}) {
	s := string(data)
	body := s
	fm := map[string]interface{}{}
	if strings.HasPrefix(s, "---") {
		end := strings.Index(s[3:], "\n---")
		if end >= 0 {
			front := s[3 : 3+end]
			body = strings.TrimSpace(s[3+end+4:])
			_ = yaml.Unmarshal([]byte(front), &fm)
		}
	}
	return body, fm
}

func normalizeZipPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	for strings.HasPrefix(p, "/") {
		p = p[1:]
	}
	return p
}

func detectContentType(name string, data []byte) string {
	lower := strings.ToLower(path.Ext(name))
	switch lower {
	case ".md":
		return "text/markdown"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "application/yaml"
	case ".txt":
		return "text/plain"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".html":
		return "text/html"
	case ".js":
		return "text/javascript"
	case ".css":
		return "text/css"
	}
	return "application/octet-stream"
}

// ===== 第三方市场同步导入 =====

// ImportZipReq 第三方市场一键同步的导入请求。
type ImportZipReq struct {
	NamespaceId    string   `json:"namespaceId"`    // 同步目标命名空间
	Source         string   `json:"source"`         // 来源标识（如 "xfyun"），写入 changelog/MarketRef/Source/日志
	SourceURL      string   `json:"sourceUrl"`      // 该 skill 在来源站的详情链接
	UpstreamAuthor string   `json:"upstreamAuthor"` // 原作者（外部市场作者 handle 如 @user_xxx；空则回落包内 SKILL.md author）
	Tags           []string `json:"tags"`           // 导入后给技能打的标签（自由文本，去重）
	Version        string   `json:"version"`        // 可空：空则取 frontmatter.version，再空则默认 1.0.0
	Changelog      string   `json:"changelog"`      // 可空：非空则覆盖默认「从第三方市场同步导入」变更说明
	Visibility     uint     `json:"visibility"`
	Status         uint     `json:"status"`
}

// ImportZip 从已下载的 zip 字节创建/更新本地 skill（第三方市场一键同步入口）。
//   - 复用 inspectZip 解析 SKILL.md frontmatter（name→slug, description→summary），不重写解析
//   - 同 namespace + slug 已存在 → 给该 skill 追加新版本（视为更新）
//   - 不存在 → repo.Add 建 skill 再 createVersionFromZip
//   - MarketRef 标记来源（{source}:{slug}），便于溯源
//   - 不走 staging（同步是后端单步完成，内存 zip 字节直接处理）
func (s *SkillPackageService) ImportZip(ctx context.Context, raw []byte, req *ImportZipReq, userId string) (skillId, versionId string, err error) {
	if len(raw) == 0 || req == nil || req.NamespaceId == "" {
		return "", "", errors.New("包内容/namespaceId 不能为空")
	}
	ins, err := inspectZip(raw)
	if err != nil {
		slog.Warn("第三方 skill 同步解析包失败", "source", req.Source, "err", err)
		return "", "", err
	}
	fm := ins.Frontmatter
	name, _ := fm["name"].(string)
	if name == "" {
		return "", "", errors.New("SKILL.md 缺少必需的 name 字段")
	}
	if len(name) > 256 {
		name = name[:256] // display_name varchar(256)
	}
	slug := normalizeSlug(name)
	// 显示名优先 frontmatter.title（可中文，供列表页「名字」列），无 title 回落英文标识符 name
	title := strings.TrimSpace(getString(fm, "title"))
	if len([]rune(title)) > 256 {
		title = string([]rune(title)[:256]) // display_name varchar(256)
	}
	summary, _ := fm["description"].(string)
	if summary == "" {
		summary = getString(fm, "summary")
	}
	// 原作者：外部市场作者 handle（如引用安装的 @user_xxx）优先，回落包内 SKILL.md author
	author := strings.TrimSpace(req.UpstreamAuthor)
	if author == "" {
		author = strings.TrimSpace(getString(fm, "author"))
	}
	if len([]rune(author)) > 200 {
		author = string([]rune(author)[:200]) // upstream_author varchar(200)
	}
	version := req.Version
	if version == "" {
		version = getString(fm, "version")
	}
	if version == "" {
		version = "1.0.0"
	}
	status := req.Status
	if status == 0 {
		status = 1 // 默认 ACTIVE
	}
	changelog := "从第三方市场同步导入"
	if req.Source != "" {
		changelog = "从第三方市场同步导入: " + req.Source + "/" + slug
	}
	if req.Changelog != "" {
		changelog = req.Changelog
	}

	// 同 namespace + slug 已存在（精确匹配活跃记录）则追加版本。
	existing, ferr := s.findSkillBySlug(ctx, req.NamespaceId, slug)
	if ferr != nil {
		slog.Error("第三方 skill 同步查重失败", "namespaceId", req.NamespaceId, "slug", slug, "err", ferr)
		return "", "", ferr
	}
	if existing != nil {
		// 同号版本已存在：幂等导入——不重复追加版本/重扫（上游没升版本号时重复导入
		// 会撞 UNIQUE(skill_id, version)，且重导同包=重扫+重复通知，均无意义），仅补溯源与标签。
		vid := s.findVersionIdByNumber(ctx, existing.SkillId, version)
		if vid == "" {
			var verr error
			vid, verr = s.createVersionFromZip(ctx, existing, version, changelog, raw, userId)
			if verr != nil {
				slog.Error("第三方 skill 同步(追加版本)失败", "skillId", existing.SkillId, "source", req.Source, "slug", slug, "err", verr)
				return existing.SkillId, "", verr
			}
		} else {
			slog.Info("第三方 skill 同步命中同号版本，跳过追加(幂等导入)", "skillId", existing.SkillId, "versionId", vid, "source", req.Source, "slug", slug, "version", version)
		}
		// 追加版本时补记原作者/原链接（老数据为空且本次拿得到时）
		if existing.UpstreamAuthor == "" && author != "" {
			existing.UpstreamAuthor = author
			if eerr := s.repo.Edit(ctx, existing); eerr != nil {
				slog.Warn("第三方 skill 同步补记原作者失败", "skillId", existing.SkillId, "err", eerr)
			}
		}
		if existing.SourceUrl == "" && req.SourceURL != "" {
			existing.SourceUrl = req.SourceURL
			if eerr := s.repo.Edit(ctx, existing); eerr != nil {
				slog.Warn("第三方 skill 同步补记原链接失败", "skillId", existing.SkillId, "err", eerr)
			}
		}
		// 老数据显示名仍是英文 slug 时补上中文 title（用户在 UI 改过显示名的不覆盖）
		if title != "" && (existing.DisplayName == "" || existing.DisplayName == existing.Slug) {
			existing.DisplayName = title
			if eerr := s.repo.Edit(ctx, existing); eerr != nil {
				slog.Warn("skill 同步补显示名失败", "skillId", existing.SkillId, "err", eerr)
			}
		}
		s.applyImportTags(ctx, existing.SkillId, req.Tags, userId)
		slog.Info("第三方 skill 同步完成(既有技能)", "skillId", existing.SkillId, "versionId", vid, "source", req.Source, "slug", slug, "version", version)
		return existing.SkillId, vid, nil
	}

	// 新建：slug 唯一冲突（含被软删除行占约束）则自动加 -2/-3 后缀重试。
	var sk *entity.Skill
	trySlug := slug
	for i := 0; i < 6; i++ {
		sk = &entity.Skill{
			NamespaceId:    req.NamespaceId,
			Slug:           trySlug,
			DisplayName:    orDefault(title, name),
			Summary:        summary,
			OwnerId:        userId,
			MarketRef:      marketRef(req.Source, trySlug),
			Source:         req.Source,
			SourceUrl:      req.SourceURL,
			UpstreamAuthor: author,
			Visibility:     req.Visibility,
			Status:         status,
			CreatedBy:      userId,
		}
		skillId, err = s.repo.Add(ctx, sk)
		if err == nil {
			break
		}
		if !isUniqueViolation(err) {
			slog.Error("第三方 skill 新建失败", "source", req.Source, "slug", trySlug, "err", err)
			return "", "", err
		}
		slog.Warn("第三方 skill slug 唯一冲突，改后缀重试", "orig", slug, "attempt", trySlug, "source", req.Source)
		if i == 5 {
			return "", "", fmt.Errorf("slug %s 唯一冲突，多次重试仍失败", slug)
		}
		trySlug = fmt.Sprintf("%s-%d", slug, i+2)
	}
	sk.SkillId = skillId
	versionId, err = s.createVersionFromZip(ctx, sk, version, changelog, raw, userId)
	if err != nil {
		slog.Error("第三方 skill 同步(新建)版本失败", "skillId", skillId, "source", req.Source, "slug", sk.Slug, "err", err)
		return skillId, "", err
	}
	s.applyImportTags(ctx, skillId, req.Tags, userId)
	slog.Info("第三方 skill 同步完成(新建)", "skillId", skillId, "versionId", versionId, "source", req.Source, "slug", sk.Slug, "version", version, "fileCount", len(ins.Files), "totalSize", ins.TotalSize)
	return skillId, versionId, nil
}

// applyImportTags 导入落标签（自由文本）：去空/去重/跳过已有同名，失败只记日志不阻断导入。
func (s *SkillPackageService) applyImportTags(ctx context.Context, skillId string, tags []string, userId string) {
	if len(tags) == 0 {
		return
	}
	existing, terr := s.repo.ListTags(ctx, skillId)
	if terr != nil {
		slog.Warn("第三方 skill 导入读已有标签失败（继续打新标签）", "skillId", skillId, "err", terr)
	}
	seen := map[string]bool{}
	for _, t := range existing {
		seen[t.TagName] = true
	}
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" || len([]rune(t)) > 64 || seen[t] {
			continue
		}
		seen[t] = true
		if err := s.repo.AddTag(ctx, &entity.SkillTag{SkillId: skillId, TagName: t, CreatedBy: userId}); err != nil {
			slog.Warn("第三方 skill 导入打标签失败", "skillId", skillId, "tag", t, "err", err)
		}
	}
}

// isUniqueViolation 判断是否 PG 唯一约束冲突（SQLSTATE 23505）。
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}

// findVersionIdByNumber 按 version 号精确匹配该技能的既有版本（命中返回 version_id，无则空串）。
func (s *SkillPackageService) findVersionIdByNumber(ctx context.Context, skillId, version string) string {
	if skillId == "" || version == "" {
		return ""
	}
	list, err := s.repo.ListVersions(ctx, skillId)
	if err != nil {
		return "" // 查失败按无既有版本处理，交由后续创建兜底（唯一约束最终把关）
	}
	for _, v := range list {
		if v.Version == version {
			return v.VersionId
		}
	}
	return ""
}

// findSkillBySlug 在 namespace 内按 slug 精确查找（repo.List 用 LIKE，需遍历精确比对）。
func (s *SkillPackageService) findSkillBySlug(ctx context.Context, namespaceId, slug string) (*entity.Skill, error) {
	if slug == "" {
		return nil, nil
	}
	list, _, err := s.repo.List(ctx, namespaceId, slug, "", "", "", 1, 50)
	if err != nil {
		return nil, err
	}
	for _, sk := range list {
		if sk.Slug == slug {
			return sk, nil
		}
	}
	return nil, nil
}

// marketRef 生成来源溯源标识（{source}:{slug}），写入 aa_skill.market_ref。
func marketRef(source, slug string) string {
	if source == "" {
		return ""
	}
	return source + ":" + slug
}
