package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
)

// KbService 知识库管理：上传（抽取→清洗→分块→嵌入→入库）/ 列表 / 详情 / 检索 /
// 编辑（标题/摘要变动 → 重嵌）/ 更新原件（内容变动 → 全量重嵌）/ 删除。
// 嵌入用离线 LocalEmbedding（同技能库），同步完成——上传即检得。
// 身份参数 t *model.Identity（组合授权判定，见 doc_access.go）：nil = 系统/维护路径全放行。
type KbService struct {
	repo       repository.IKbRepository
	dirRepo    repository.IKbDirRepository
	accessRepo repository.IKbAccessLogRepository
	grantRepo  repository.IKbGrantRepository
	userRepo   repository.IUserRepository
	embed      EmbeddingProvider
	dir        string // 原件落盘根目录（api/data/kb/）

	externalSearch ExternalKBSearcher // 外部知识库搜索透传（KbDatasourceService 回指，构造后 Set）
}

func NewKbService(r repository.IKbRepository, dr repository.IKbDirRepository, ar repository.IKbAccessLogRepository,
	gr repository.IKbGrantRepository, ur repository.IUserRepository, e EmbeddingProvider, dir string) *KbService {
	if dir == "" {
		dir = "data/kb"
	}
	return &KbService{repo: r, dirRepo: dr, accessRepo: ar, grantRepo: gr, userRepo: ur, embed: e, dir: dir}
}

// chunk 尺寸：块 700-900 字（中文制度文档一个条目大致量级），标题/摘要单独一小块
const (
	kbChunkMin = 700
	kbChunkMax = 900
)

// kbNoTextNote 抽取为空时的入库说明（扫描件/图片型 PDF/暂不支持的老格式）：
// 嵌入不算失败（标题摘要仍可检索），但要把原因带给使用者。
const kbNoTextNote = "未抽取到正文（可能为扫描件/图片型 PDF 或暂不支持的格式），仅标题与摘要参与检索"

// IngestData 通用入库：title 可空（回落文件名去扩展名），同 hash 已存在则跳过。
// dirId 目标目录（”=根）。返回 (docId, created, error)。UI 上传与后续导入入口都走这里。
func (s *KbService) IngestData(ctx context.Context, fileName, title, summary string, data []byte, createdBy, source, dirId string) (string, bool, error) {
	fileType := kbDetectFileType(fileName)
	fileHash := kbHashBytes(data)

	// 同内容已存在：跳过（幂等）
	if old, err := s.repo.GetByHash(ctx, fileHash); err == nil && old != nil {
		return old.DocId, false, nil
	}

	content, err := KbExtractText(fileType, data)
	if err != nil {
		return "", false, err
	}
	if title == "" {
		title = strings.TrimSuffix(fileName, filepath.Ext(fileName))
	}
	title = kbSanitizeName(title, 256)
	if title == "" { // 文件名只剩扩展名/全非法字符（如 ".zip"）
		title = "未命名文档"
	}
	fileName = kbSanitizeName(filepath.Base(fileName), 256)

	// 原件落盘（文件名不可信，用 doc_id 命名）。Visibility 默认全员（Go 零值 0=私有，
	// 不显式给会把全部新上传翻成私有）；上传批次的授权由 UploadFiles 后置 applyUploadVis 覆写。
	doc := &entity.KbDocument{
		Title:      title,
		Summary:    summary,
		FileName:   fileName,
		FileType:   fileType,
		FileSize:   int64(len(data)),
		FileHash:   fileHash,
		Source:     source,
		CreatedBy:  createdBy,
		DirId:      dirId,
		Visibility: 2,
	}
	chunks, embedErr := s.buildChunks(title, summary, content)
	if embedErr != nil {
		doc.EmbedStatus, doc.EmbedError = 2, embedErr.Error()
	} else {
		doc.EmbedStatus = 1
		if strings.TrimSpace(content) == "" {
			doc.EmbedError = kbNoTextNote
		}
	}
	doc.ContentText = content
	doc.CharCount = utf8.RuneCountInString(content)
	doc.ChunkCount = len(chunks)

	if err := s.repo.Add(ctx, doc, chunks); err != nil {
		return "", false, err
	}
	if err := s.saveFile(doc.DocId, fileType, data); err != nil {
		// 落盘失败不回滚库（文本已在库，原件仅留档）；记录继续
		_ = err
	}
	return doc.DocId, true, nil
}

// ReplaceFile 更新文档原件：内容变了 → 重抽取重分块重嵌入；没变 → 只刷新标题/摘要。
func (s *KbService) ReplaceFile(ctx context.Context, docId, title, summary, fileName string, data []byte, t *model.Identity) error {
	old, err := s.assertDocWrite(ctx, docId, t)
	if err != nil {
		return err
	}
	fileType := kbDetectFileType(fileName)
	fileHash := kbHashBytes(data)

	if old.FileHash == fileHash {
		// 内容未变：只改标题/摘要（编辑语义）
		old.Title = kbPickTitle(title, old, fileName)
		if summary != "" {
			old.Summary = summary
		}
		if err := s.repo.Edit(ctx, old); err != nil {
			return err
		}
		return s.reembed(ctx, old)
	}

	content, err := KbExtractText(fileType, data)
	if err != nil {
		return err
	}
	old.Title = kbPickTitle(title, old, fileName)
	if summary != "" {
		old.Summary = summary
	}
	old.FileName, old.FileType, old.FileSize, old.FileHash = filepath.Base(fileName), fileType, int64(len(data)), fileHash
	chunks, embedErr := s.buildChunks(old.Title, old.Summary, content)
	if embedErr != nil {
		old.EmbedStatus, old.EmbedError = 2, embedErr.Error()
	} else {
		old.EmbedStatus, old.EmbedError = 1, ""
		if strings.TrimSpace(content) == "" {
			old.EmbedError = kbNoTextNote
		}
	}
	old.ContentText = content
	old.CharCount = utf8.RuneCountInString(content)
	old.ChunkCount = len(chunks)
	if err := s.repo.ReplaceChunks(ctx, docId, old, chunks); err != nil {
		return err
	}
	return s.saveFile(docId, fileType, data)
}

// IngestText 纯文本文档入库（MCP 写入口）：轻清洗 → 同 hash 去重 → 分块嵌入。
// 与 IngestData（文件上传）同库同检索；原件 = 提交原文（未清洗）落盘 raw/<doc_id>，
// 预览/下载与文件型文档同径；file_hash 指纹 = sha256(title+NUL+content)。
func (s *KbService) IngestText(ctx context.Context, title, summary, text, createdBy, source string) (*entity.KbDocument, bool, error) {
	if title == "" || strings.TrimSpace(text) == "" {
		return nil, false, errors.New("title 与 text 不能为空")
	}
	content := KbTidyPlainText(text)
	fileHash := kbHashString(title + "\x00" + content)
	if old, err := s.repo.GetByHash(ctx, fileHash); err == nil && old != nil {
		return old, false, nil
	}
	doc := &entity.KbDocument{
		Title:      title,
		Summary:    summary,
		FileName:   kbSanitizeName(title, 252) + ".txt", // 252+扩展名 = varchar(256) 上限内
		FileType:   "txt",
		FileSize:   int64(len(text)),
		FileHash:   fileHash,
		Source:     source,
		CreatedBy:  createdBy,
		Visibility: 2, // 同 IngestData：默认全员，避免零值翻私有
	}
	chunks, embedErr := s.buildChunks(title, summary, content)
	if embedErr != nil {
		doc.EmbedStatus, doc.EmbedError = 2, embedErr.Error()
	} else {
		doc.EmbedStatus = 1
		if strings.TrimSpace(content) == "" {
			doc.EmbedError = kbNoTextNote
		}
	}
	doc.ContentText = content
	doc.CharCount = utf8.RuneCountInString(content)
	doc.ChunkCount = len(chunks)
	if err := s.repo.Add(ctx, doc, chunks); err != nil {
		return nil, false, err
	}
	// 原件落盘（提交原文）：失败不回滚库（GetRaw 对缺失原件有库内文本兜底自愈）
	_ = s.saveFile(doc.DocId, "txt", []byte(text))
	return doc, true, nil
}

// UpdateText 更新纯文本文档（MCP 写入口）：text 给了 → 换内容重分块重嵌；不给 → 只改标题/摘要也重嵌。
func (s *KbService) UpdateText(ctx context.Context, docId, title, summary, text string, t *model.Identity) (*entity.KbDocument, error) {
	d, err := s.assertDocWrite(ctx, docId, t)
	if err != nil {
		return nil, err
	}
	if title != "" {
		d.Title = title
	}
	if summary != "" {
		d.Summary = summary
	}
	if text != "" {
		d.ContentText = KbTidyPlainText(text)
		d.CharCount = utf8.RuneCountInString(d.ContentText)
		d.FileHash = kbHashString(d.Title + "\x00" + d.ContentText)
		d.FileSize = int64(len(text))
	}
	if err := s.reembed(ctx, d); err != nil {
		return nil, err
	}
	// MCP 文本文档：正文变更同步重写原件（文件型文档原件不受文本更新影响）
	if text != "" && d.Source == "mcp" {
		_ = s.saveFile(d.DocId, "txt", []byte(text))
	}
	return d, nil
}

// SourceIngestAction 单条数据源条目的入库结果。
type SourceIngestAction int

const (
	SourceUnchanged SourceIngestAction = iota // 哈希未变，跳过
	SourceCreated                             // 新建文档
	SourceUpdated                             // 内容有变，重嵌覆盖
)

// SourceIngestResult 数据源单次同步的入库统计（ingest 回调侧口径）。
type SourceIngestResult struct {
	Created   int64 `json:"created"`
	Updated   int64 `json:"updated"`
	Unchanged int64 `json:"unchanged"`
}

// IngestSourceText 数据源同步的单条入库（upsert 语义，kb_datasource_service 的
// ingest 回调装配体）：外部键 → 稳定 file_name 定位既有文档——
// 哈希未变（标题+NUL+清洗后正文）跳过；变了 UpdateText 全量重嵌并重写原件；
// 没有则按 Source=datasource 新建（可见性 2 全员，随目录授权收口）。
// nil 身份 = 系统路径（同步引擎触发，不经用户权限）。
func (s *KbService) IngestSourceText(ctx context.Context, dirId, extKey, title, summary, text string) (SourceIngestAction, error) {
	if dirId == "" || extKey == "" || strings.TrimSpace(text) == "" {
		return SourceUnchanged, errors.New("dirId/extKey/text 不能为空")
	}
	if strings.TrimSpace(title) == "" {
		title = extKey
	}
	content := KbTidyPlainText(text)
	fileHash := kbHashString(title + "\x00" + content)
	fileName := kbSourceFileName(extKey)
	old, err := s.repo.GetByDirAndName(ctx, dirId, fileName)
	if err != nil {
		return SourceUnchanged, err
	}
	if old != nil {
		if old.FileHash == fileHash {
			return SourceUnchanged, nil // 未变化
		}
		if _, err := s.UpdateText(ctx, old.DocId, title, summary, text, nil); err != nil {
			return SourceUnchanged, err
		}
		// datasource 文档原件同样重写（UpdateText 只对 mcp 来源重写）
		_ = s.saveFile(old.DocId, "txt", []byte(text))
		return SourceUpdated, nil
	}
	doc := &entity.KbDocument{
		Title:      title,
		Summary:    summary,
		FileName:   fileName,
		FileType:   "txt",
		FileSize:   int64(len(text)),
		FileHash:   fileHash,
		Source:     "datasource",
		DirId:      dirId,
		CreatedBy:  "",
		Visibility: 2,
	}
	chunks, embedErr := s.buildChunks(title, summary, content)
	if embedErr != nil {
		doc.EmbedStatus, doc.EmbedError = 2, embedErr.Error()
	} else {
		doc.EmbedStatus = 1
		if strings.TrimSpace(content) == "" {
			doc.EmbedError = kbNoTextNote
		}
	}
	doc.ContentText = content
	doc.CharCount = utf8.RuneCountInString(content)
	doc.ChunkCount = len(chunks)
	if err := s.repo.Add(ctx, doc, chunks); err != nil {
		return SourceUnchanged, err
	}
	_ = s.saveFile(doc.DocId, "txt", []byte(text))
	return SourceCreated, nil
}

// kbSourceFileName 外部键 → 稳定文件名：清洗截断 + 外部键哈希前 12 位防碰撞，
// 同一键跨次同步可定位同一文档（upsert 依据），不同键不互相覆盖。
func kbSourceFileName(extKey string) string {
	return kbSanitizeName(extKey, 200) + "-" + kbHashString(extKey)[:12] + ".txt"
}

// ReadDoc 阅读入口：doc_id 优先，回落标题精确匹配（agent 常拿到的只有标题）
func (s *KbService) ReadDoc(ctx context.Context, docId, title string, t *model.Identity) (*entity.KbDocument, error) {
	if docId != "" {
		return s.Get(ctx, docId, t)
	}
	d, err := s.repo.GetByTitle(ctx, title)
	if err != nil {
		return nil, err
	}
	if t != nil { // 标题回落路径同样过读校验
		v, err := s.loadPerm(ctx, *t)
		if err != nil {
			return nil, err
		}
		if !v.canReadDoc(*t, d) {
			return nil, errors.New("没有访问权限")
		}
	}
	s.enrichCreators(ctx, []*entity.KbDocument{d})
	return d, nil
}

// Edit 编辑标题/摘要 → 重嵌（标题/摘要是独立小块，检索入口）
func (s *KbService) Edit(ctx context.Context, docId, title, summary string, t *model.Identity) error {
	d, err := s.assertDocWrite(ctx, docId, t)
	if err != nil {
		return err
	}
	if title != "" {
		d.Title = title
	}
	if summary != "" {
		d.Summary = summary
	}
	if err := s.repo.Edit(ctx, d); err != nil {
		return err
	}
	return s.reembed(ctx, d)
}

// reembed 按库内全文重建标题块 + 内容块（用于标题/摘要变更后刷新向量）
func (s *KbService) reembed(ctx context.Context, d *entity.KbDocument) error {
	chunks, err := s.buildChunks(d.Title, d.Summary, d.ContentText)
	if err != nil {
		return err
	}
	d.ChunkCount = len(chunks)
	d.EmbedStatus, d.EmbedError = 1, ""
	if strings.TrimSpace(d.ContentText) == "" { // 空抽取说明在重嵌入后保留
		d.EmbedError = kbNoTextNote
	}
	return s.repo.ReplaceChunks(ctx, d.DocId, d, chunks)
}

// buildChunks 标题/摘要一小块（seq 0）+ 正文分块。逐块嵌入（离线哈希嵌入，同步快）。
func (s *KbService) buildChunks(title, summary, content string) ([]*entity.KbChunk, error) {
	var chunks []*entity.KbChunk
	head := strings.TrimSpace(title + "\n" + summary)
	if head != "" {
		c, err := s.embedChunk(0, head)
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	for i, piece := range KbChunkText(content, kbChunkMin, kbChunkMax) {
		c, err := s.embedChunk(i+1, piece)
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, nil
}

func (s *KbService) embedChunk(seq int, text string) (*entity.KbChunk, error) {
	vec, err := s.embed.Embed(text)
	if err != nil {
		return nil, fmt.Errorf("嵌入失败：%w", err)
	}
	return &entity.KbChunk{Seq: seq, Content: text, SemanticVector: VectorToPGText(vec)}, nil
}

func (s *KbService) List(ctx context.Context, keyword, fileType, dirId string, pageNum, pageSize int, t *model.Identity) ([]*entity.KbDocument, int64, error) {
	var dirIds []string
	if dirId != "" { // 展开子树：选中目录 = 其下全部层级
		var err error
		dirIds, err = s.subtreeIds(ctx, dirId)
		if err != nil {
			return nil, 0, err
		}
	}
	// 权限视图只装一次：目录门（readableDirs 下推 SQL）与行内 canWrite 共用
	var readableDirs []string
	var v *kbPermView
	if t != nil {
		pv, err := s.loadPerm(ctx, *t)
		if err != nil {
			return nil, 0, err
		}
		v = pv
		for id, ok := range v.readableDirs {
			if ok {
				readableDirs = append(readableDirs, id)
			}
		}
	}
	list, total, err := s.repo.ListVisible(ctx, keyword, fileType, dirIds, pageNum, pageSize, t, readableDirs)
	if err != nil {
		return nil, 0, err
	}
	s.enrichCreators(ctx, list)
	// 回填 canWrite（行按钮显隐；nil 身份全 true）
	for _, d := range list {
		if t == nil {
			d.CanWrite = true
		} else {
			d.CanWrite = v.canWriteDoc(*t, d)
		}
	}
	return list, total, nil
}

// subtreeIds 目录自身 + 全部后代目录 id（目录量小，内存展开）
func (s *KbService) subtreeIds(ctx context.Context, dirId string) ([]string, error) {
	dirs, err := s.dirRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	children := make(map[string][]string, len(dirs))
	for _, d := range dirs {
		children[d.ParentId] = append(children[d.ParentId], d.DirId)
	}
	out := []string{dirId}
	for i := 0; i < len(out); i++ { // BFS
		out = append(out, children[out[i]]...)
	}
	return out, nil
}

// enrichCreators 回填上传人姓名（跨库读 aiarsenal.aa_user，不 join）
func (s *KbService) enrichCreators(ctx context.Context, list []*entity.KbDocument) {
	if s.userRepo == nil {
		return
	}
	ids := make([]string, 0, len(list))
	seen := map[string]struct{}{}
	for _, d := range list {
		if d.CreatedBy == "" {
			continue
		}
		if _, ok := seen[d.CreatedBy]; !ok {
			seen[d.CreatedBy] = struct{}{}
			ids = append(ids, d.CreatedBy)
		}
	}
	if len(ids) == 0 {
		return
	}
	users, err := s.userRepo.GetUsersByIds(ctx, ids)
	if err != nil {
		return
	}
	m := make(map[string]string, len(users))
	for _, u := range users {
		m[u.Id] = u.Name
	}
	for _, d := range list {
		if n, ok := m[d.CreatedBy]; ok {
			d.CreatorName = n
		}
	}
}

func (s *KbService) Get(ctx context.Context, docId string, t *model.Identity) (*entity.KbDocument, error) {
	d, err := s.assertDocRead(ctx, docId, t)
	if err != nil {
		return nil, err
	}
	s.enrichCreators(ctx, []*entity.KbDocument{d})
	return d, nil
}

// GetContent 详情阅读：库内全文优先，缺失回落拼块。带 userId/source 落一条阅读记录。
func (s *KbService) GetContent(ctx context.Context, docId, userId, source string, t *model.Identity) (string, error) {
	d, err := s.assertDocRead(ctx, docId, t)
	if err != nil {
		return "", err
	}
	s.logRead(ctx, docId, userId, source)
	if d.ContentText != "" {
		return d.ContentText, nil
	}
	return s.repo.ListChunkContents(ctx, docId)
}

// GetRaw 原件字节（预览/下载）。落一条阅读记录（与 Content 同口径）。
func (s *KbService) GetRaw(ctx context.Context, docId, userId, source string, t *model.Identity) (*entity.KbDocument, []byte, error) {
	d, err := s.assertDocRead(ctx, docId, t)
	if err != nil {
		return nil, nil, err
	}
	data, err := os.ReadFile(filepath.Join(s.dir, "raw", docId))
	if err != nil {
		// 兜底自愈：早期 MCP 文本文档入库未落原件，用库内文本补写一次后正常预览
		if d.Source == "mcp" && d.FileType == "txt" && d.ContentText != "" {
			if he := s.saveFile(d.DocId, "txt", []byte(d.ContentText)); he == nil {
				data = []byte(d.ContentText)
			}
		}
		if data == nil {
			return nil, nil, errors.New("原件未落盘，无法预览")
		}
	}
	s.logRead(ctx, docId, userId, source)
	return d, data, nil
}

// Search 语义检索：query → 向量 → 块 top-k（带文档标题聚合）。落一条检索记录（词+命中数）。
// SQL 侧已带授权读谓词 + 目录门；命中块再逐文档 Go 复核（belt-and-suspenders，防谓词漂移）。
func (s *KbService) Search(ctx context.Context, query string, limit int, userId, source string, t *model.Identity) ([]map[string]interface{}, error) {
	if query == "" {
		return nil, nil
	}
	vec, err := s.embed.Embed(query)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	readableDirs, err := s.readableDirIds(ctx, t)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.SearchChunks(ctx, VectorToPGText(vec), limit, t, readableDirs)
	if err != nil {
		return nil, err
	}
	rows = s.filterSearchRows(ctx, rows, t)
	rows = kbMergeSearchRows(ctx, rows, s.externalSearch, query, limit, t)
	s.logSearch(ctx, query, len(rows), userId, source)
	return rows, nil
}

// ExternalKBSearcher 外部知识库搜索透传口（由 KbDatasourceService 实现；
// setter 回指装配，避免 service 构造环）。返回的行已带 source=external 与数据源名。
type ExternalKBSearcher interface {
	SearchExternal(ctx context.Context, query string, k int, t *model.Identity) []map[string]interface{}
}

// SetExternalSearch 注入外部知识库搜索实现（main.go 装配点）。
func (s *KbService) SetExternalSearch(e ExternalKBSearcher) { s.externalSearch = e }

// kbMergeSearchRows 内部检索行与外部知识库透传行合并：去重（标题+正文归一比对，
// 内部优先）、总量封顶 2*limit。nil 透传实现或空查询直接返回内部行。
func kbMergeSearchRows(ctx context.Context, rows []map[string]interface{}, ext ExternalKBSearcher, query string, limit int, t *model.Identity) []map[string]interface{} {
	if ext == nil || strings.TrimSpace(query) == "" {
		for _, r := range rows { // 内部行统一打标
			r["source"] = "internal"
		}
		return rows
	}
	extRows := ext.SearchExternal(ctx, query, limit, t)
	if len(extRows) == 0 {
		for _, r := range rows {
			r["source"] = "internal"
		}
		return rows
	}
	seen := map[string]bool{}
	for _, r := range rows {
		r["source"] = "internal"
		title, _ := r["title"].(string)
		content, _ := r["content"].(string)
		seen[kbSearchDedupKey(title, content)] = true
	}
	for _, er := range extRows {
		title, _ := er["title"].(string)
		content, _ := er["content"].(string)
		k := kbSearchDedupKey(title, content)
		if seen[k] {
			continue
		}
		seen[k] = true
		rows = append(rows, er)
	}
	if max := limit * 2; len(rows) > max { // 外部命中再多也不挤占整体版面
		rows = rows[:max]
	}
	return rows
}

// kbSearchDedupKey 检索行去重键：标题+正文归一（小写、压空白）截前 200 字。
func kbSearchDedupKey(title, content string) string {
	norm := func(s string) string {
		return strings.Join(strings.Fields(strings.ToLower(s)), " ")
	}
	n := norm(content)
	if len(n) > 200 { // 按字节截断即可（去重键允许断在多字节字符中段）
		n = n[:200]
	}
	return norm(title) + "\x00" + n
}

// filterSearchRows 检索命中行按 Go 权威判定复核（nil 身份原样返回）
func (s *KbService) filterSearchRows(ctx context.Context, rows []map[string]interface{}, t *model.Identity) []map[string]interface{} {
	if t == nil || len(rows) == 0 {
		return rows
	}
	v, err := s.loadPerm(ctx, *t)
	if err != nil {
		return rows
	}
	out := rows[:0]
	for _, r := range rows {
		docId, _ := r["doc_id"].(string)
		if docId == "" {
			continue
		}
		d, err := s.repo.Get(ctx, docId)
		if err != nil || !v.canReadDoc(*t, d) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// logSearch / logRead 访问记录（尽力而为：写失败不影响业务）
func (s *KbService) logSearch(ctx context.Context, keyword string, hits int, userId, source string) {
	if s.accessRepo == nil || strings.TrimSpace(keyword) == "" {
		return
	}
	if len(keyword) > 256 {
		keyword = keyword[:256]
	}
	_ = s.accessRepo.Add(ctx, &entity.KbAccessLog{
		UserId: userId, Action: "search", Keyword: keyword, Hits: hits, Source: source,
	})
}

func (s *KbService) logRead(ctx context.Context, docId, userId, source string) {
	if s.accessRepo == nil || docId == "" {
		return
	}
	_ = s.accessRepo.Add(ctx, &entity.KbAccessLog{
		DocId: docId, UserId: userId, Action: "read", Source: source,
	})
}

// ListAccess 访问记录分页（回填 userName：”=MCP 侧）
func (s *KbService) ListAccess(ctx context.Context, action, docId string, pageNum, pageSize int) ([]*entity.KbAccessLog, int64, error) {
	list, total, err := s.accessRepo.List(ctx, action, docId, pageNum, pageSize)
	if err != nil {
		return nil, 0, err
	}
	if s.userRepo != nil {
		ids := make([]string, 0, len(list))
		seen := map[string]struct{}{}
		for _, l := range list {
			if l.UserId == "" {
				continue
			}
			if _, ok := seen[l.UserId]; !ok {
				seen[l.UserId] = struct{}{}
				ids = append(ids, l.UserId)
			}
		}
		if len(ids) > 0 {
			if users, err := s.userRepo.GetUsersByIds(ctx, ids); err == nil {
				m := make(map[string]string, len(users))
				for _, u := range users {
					m[u.Id] = u.Name
				}
				for _, l := range list {
					if n, ok := m[l.UserId]; ok {
						l.UserName = n
					}
				}
			}
		}
	}
	for _, l := range list {
		if l.UserId == "" {
			l.UserName = "MCP"
		}
	}
	return list, total, nil
}

func (s *KbService) Delete(ctx context.Context, docIds []string, t *model.Identity) error {
	if len(docIds) == 0 {
		return nil
	}
	for _, id := range docIds { // 逐文档写校验（任一无权即整体拒绝）
		if _, err := s.assertDocWrite(ctx, id, t); err != nil {
			return err
		}
	}
	if err := s.repo.Delete(ctx, docIds); err != nil {
		return err
	}
	_ = s.grantRepo.DeleteByTargets(ctx, "doc", docIds) // 授权行随之清理（尽力而为）
	for _, id := range docIds {                         // 原件随之清理（失败忽略：库已删，孤儿文件无害）
		_ = os.Remove(filepath.Join(s.dir, "raw", id))
	}
	return nil
}

// saveFile 原件落盘 api/data/kb/raw/<doc_id>（无扩展名：文件名不可信，类型在库里）
func (s *KbService) saveFile(docId, fileType string, data []byte) error {
	if err := os.MkdirAll(filepath.Join(s.dir, "raw"), 0o750); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "raw", docId), data, 0o640)
}

func kbHashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// kbHashString 文本指纹（MCP 纯文本文档用，与文件上传的 file_hash 同列同长度）
func kbHashString(s string) string {
	return kbHashBytes([]byte(s))
}

// kbSanitizeName 净化入库名：去控制字符、trim、按字符截断 max（PG varchar(n) 计字符）。
// zip 来源的名字即便解码失败也不至于把非法字节带进库。
func kbSanitizeName(s string, max int) string {
	clean := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	clean = strings.TrimSpace(clean)
	if r := []rune(clean); len(r) > max {
		return string(r[:max])
	}
	return clean
}

// ---- 目录能力：批量移动 / 按范围重嵌 / 多入口上传 ----

// MoveDocs 批量移动文档（dirId ”=根）。移动 = 对文档的写操作。
func (s *KbService) MoveDocs(ctx context.Context, docIds []string, dirId string, t *model.Identity) error {
	if len(docIds) == 0 {
		return errors.New("请选择文档")
	}
	for _, id := range docIds {
		if _, err := s.assertDocWrite(ctx, id, t); err != nil {
			return err
		}
	}
	if dirId != "" {
		if _, err := s.dirRepo.Get(ctx, dirId); err != nil {
			return errors.New("目标目录不存在")
		}
	}
	return s.repo.MoveDocs(ctx, docIds, dirId)
}

// ReembedScope 重新嵌入：docIds 指定文档，或 dirId 目录（含子树）下全部。
// 返回 (成功数, 跳过数, 首个错误说明)。重嵌 = 写操作（非-nil 身份逐文档校验）。
func (s *KbService) ReembedScope(ctx context.Context, docIds []string, dirId string, t *model.Identity) (int, int, string) {
	var docs []*entity.KbDocument
	switch {
	case len(docIds) > 0:
		for _, id := range docIds {
			d, err := s.repo.Get(ctx, id)
			if err != nil {
				continue
			}
			docs = append(docs, d)
		}
	case dirId != "":
		ids, err := s.subtreeIds(ctx, dirId)
		if err != nil {
			return 0, 0, err.Error()
		}
		if docs, err = s.repo.ListByDirIds(ctx, ids); err != nil {
			return 0, 0, err.Error()
		}
	default:
		return 0, 0, errors.New("请选择文档或目录").Error()
	}
	done, skipped, firstErr := 0, 0, ""
	for _, d := range docs {
		if t != nil {
			if _, err := s.assertDocWrite(ctx, d.DocId, t); err != nil {
				skipped++
				if firstErr == "" {
					firstErr = err.Error()
				}
				continue
			}
		}
		s.reextract(d) // 原件在盘则重抽全文（抽取器升级后修复存量乱码），否则沿用库内全文
		if strings.TrimSpace(d.ContentText) == "" {
			skipped++ // 无全文（历史数据/抽取失败），无从重嵌
			continue
		}
		if err := s.reembed(ctx, d); err != nil {
			skipped++
			if firstErr == "" {
				firstErr = err.Error()
			}
			continue
		}
		done++
	}
	return done, skipped, firstErr
}

// reextract 从落盘原件重抽全文（乱码修复后刷新存量；无原件/重抽失败静默沿用库内全文）
func (s *KbService) reextract(d *entity.KbDocument) {
	data, err := os.ReadFile(filepath.Join(s.dir, "raw", d.DocId))
	if err != nil {
		return
	}
	txt, err := KbExtractText(d.FileType, data)
	if err != nil {
		return
	}
	d.ContentText = txt
	d.CharCount = len([]rune(txt))
}

// KbUploadEntry 一个待入库文件：Path 为相对上传根的路径（目录上传/zip 解包带层级）
type KbUploadEntry struct {
	Name string
	Path string // 'a/b/c.docx'；普通文件上传为 ''
	Data []byte
}

// KbUploadResult 批量上传结果
type KbUploadResult struct {
	Created int            `json:"created"`
	Skipped int            `json:"skipped"` // 同内容已存在
	Failed  []KbUploadFail `json:"failed"`
}

type KbUploadFail struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

// UploadFiles 批量入库（UI 多文件 / 目录上传 / 压缩包统一入口）。
// 压缩包自动解包（zip/rar/7z/tar/gz…含嵌套，保留层级建目录）；entry.Path 相对 dirId 建目录链；
// 单文件且非压缩包时 title/summary 生效，否则按文件名自动。
// vis 非-nil = 上传批次对每个新建文档落可见性 + 读/写条件行（见 applyUploadVis）。
func (s *KbService) UploadFiles(ctx context.Context, entries []KbUploadEntry, title, summary, dirId, createdBy string, vis *KbUploadVis) (*KbUploadResult, error) {
	if dirId != "" {
		if _, err := s.dirRepo.Get(ctx, dirId); err != nil {
			return nil, errors.New("目标目录不存在")
		}
	}
	res := &KbUploadResult{}
	single := len(entries) == 1 && !KbIsArchiveName(entries[0].Name)
	for _, e := range entries {
		if KbIsArchiveName(e.Name) {
			inner, err := KbExpandArchive(e.Data, e.Name, 0, &KbExtractBudget{})
			if err != nil {
				res.Failed = append(res.Failed, KbUploadFail{Name: e.Name, Error: err.Error()})
				continue
			}
			if len(inner) == 0 {
				res.Failed = append(res.Failed, KbUploadFail{Name: e.Name, Error: "压缩包内没有可入库文件"})
				continue
			}
			for _, ie := range inner {
				s.ingestOne(ctx, ie.Name, ie.Path, "", "", ie.Data, dirId, createdBy, vis, res)
			}
			continue
		}
		t, sum := "", ""
		if single {
			t, sum = title, summary
		}
		s.ingestOne(ctx, e.Name, e.Path, t, sum, e.Data, dirId, createdBy, vis, res)
	}
	return res, nil
}

// ingestOne 单文件入库：Path 有层级 → 沿 dirId 建目录链
func (s *KbService) ingestOne(ctx context.Context, name, path, title, summary string, data []byte, dirId, createdBy string, vis *KbUploadVis, res *KbUploadResult) {
	target := dirId
	if path != "" {
		var err error
		if target, err = s.ensureDirPath(ctx, dirId, path); err != nil {
			res.Failed = append(res.Failed, KbUploadFail{Name: name, Error: err.Error()})
			return
		}
	}
	docId, created, err := s.IngestData(ctx, name, title, summary, data, createdBy, "ui", target)
	if err != nil {
		res.Failed = append(res.Failed, KbUploadFail{Name: name, Error: err.Error()})
		return
	}
	if created {
		res.Created++
		s.applyUploadVis(ctx, docId, vis) // 新建文档才落批次授权（已存在的不动他人配置）
	} else {
		res.Skipped++
	}
}

// ensureDirPath 沿 base 逐级取/建目录，返回路径末端目录 id
func (s *KbService) ensureDirPath(ctx context.Context, base string, relPath string) (string, error) {
	segments := strings.Split(filepath.ToSlash(relPath), "/")
	// 末段是文件名，不属于目录
	if len(segments) > 0 {
		segments = segments[:len(segments)-1]
	}
	cur := base
	for _, seg := range segments {
		seg = kbSanitizeName(seg, 128) // 目录名列宽 varchar(128)
		if seg == "" || seg == "." || seg == ".." {
			continue
		}
		d, err := s.dirRepo.GetByName(ctx, cur, seg)
		if err != nil {
			d = &entity.KbDir{Name: seg, ParentId: cur, CreatedBy: ""}
			if err := s.dirRepo.Add(ctx, d); err != nil {
				return "", err
			}
		}
		cur = d.DirId
	}
	return cur, nil
}

// kbDetectFileType 扩展名 → 归一小写类型
func kbDetectFileType(name string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
}

func kbPickTitle(title string, old *entity.KbDocument, fileName string) string {
	if title != "" {
		return title
	}
	if old.Title != "" && old.Title != strings.TrimSuffix(old.FileName, filepath.Ext(old.FileName)) {
		return old.Title // 用户改过标题，保留
	}
	return strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
}
