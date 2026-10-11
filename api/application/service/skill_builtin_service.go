package service

// SkillBuiltinService 平台内置配对技能导入（api 进程启动时跑一次，幂等）。
// 资源源：resource/data/skills-builtin/<slug>/（embed 进二进制，随版本发布）。
// 落点：技能库 platform 命名空间，ImportZip 建 skill/版本 + PublishVersion 发布
// （install-latest 语义必需）。版本幂等：库中已有同 version 的版本则跳过，
// 重启不追加重复版本；升版本 = 改 SKILL.md frontmatter.version 再发版。
// 单包失败只记 warn 不影响启动；只在 api 主进程跑（cmd/ops 不 bootstrap，防双写）。

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"sort"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/resource"
)

const (
	builtinNamespaceSlug = "platform"
	builtinSource        = "builtin"
	builtinUserId        = "system"
	builtinRoot          = "data/skills-builtin"
)

type SkillBuiltinService struct {
	nsRepo    repository.ISkillNamespaceRepository
	skillRepo repository.ISkillRepository
	pkg       *SkillPackageService
}

func NewSkillBuiltinService(ns repository.ISkillNamespaceRepository, sr repository.ISkillRepository, pkg *SkillPackageService) *SkillBuiltinService {
	return &SkillBuiltinService{nsRepo: ns, skillRepo: sr, pkg: pkg}
}

// Bootstrap 幂等导入全部内置技能；任何单点失败降级为 warn（best-effort）。
func (s *SkillBuiltinService) Bootstrap(ctx context.Context) {
	nsId, err := s.ensureNamespace(ctx)
	if err != nil {
		slog.Warn("内置技能命名空间就绪失败，跳过导入", "err", err)
		return
	}
	dirs, err := fs.ReadDir(resource.SkillsBuiltin, builtinRoot)
	if err != nil {
		slog.Warn("内置技能目录读取失败", "err", err)
		return
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		if err := s.bootstrapOne(ctx, nsId, d.Name()); err != nil {
			slog.Warn("内置技能导入失败", "slug", d.Name(), "err", err)
		}
	}
}

// ensureNamespace 取（缺则建）platform 命名空间，返回 namespaceId
func (s *SkillBuiltinService) ensureNamespace(ctx context.Context) (string, error) {
	if ns, err := s.nsRepo.GetBySlug(ctx, builtinNamespaceSlug); err == nil && ns != nil {
		return ns.NamespaceId, nil
	}
	id, err := s.nsRepo.Add(ctx, &entity.SkillNamespace{
		Slug: builtinNamespaceSlug, DisplayName: "平台内置", Type: "TEAM",
		Description: "平台内置配对技能（随版本发布，自动导入）", Status: 1, CreatedBy: builtinUserId,
	})
	if err != nil {
		return "", fmt.Errorf("建 %s 命名空间: %w", builtinNamespaceSlug, err)
	}
	slog.Info("内置技能命名空间已创建", "namespaceId", id, "slug", builtinNamespaceSlug)
	return id, nil
}

// bootstrapOne 导入单个内置技能目录：目录即 slug（frontmatter.name 归一后应与之一致）。
func (s *SkillBuiltinService) bootstrapOne(ctx context.Context, nsId, slugDir string) error {
	files, err := builtinFiles(slugDir)
	if err != nil {
		return err
	}
	zipBytes, fmName, fmVersion, err := buildBuiltinZip(slugDir, files)
	if err != nil {
		return err
	}
	if fmName == "" {
		return fmt.Errorf("SKILL.md 缺少 name 字段")
	}
	if fmVersion == "" {
		fmVersion = "1.0.0"
	}

	// 版本幂等：库中已有同 version 版本则跳过（不追加重复版本）
	existing, ferr := s.pkg.findSkillBySlug(ctx, nsId, normalizeSlug(fmName))
	if ferr != nil {
		return ferr
	}
	if existing != nil {
		if versions, verr := s.skillRepo.ListVersions(ctx, existing.SkillId); verr == nil {
			for _, v := range versions {
				if v.Version == fmVersion {
					return nil
				}
			}
		}
	}

	_, versionId, err := s.pkg.ImportZip(ctx, zipBytes, &ImportZipReq{
		NamespaceId: nsId,
		Source:      builtinSource,
		Version:     fmVersion,
		Changelog:   "平台内置技能导入（随版本发布）",
		Status:      1,
	}, builtinUserId)
	if err != nil {
		return err
	}
	if err := s.skillRepo.PublishVersion(ctx, versionId); err != nil {
		return fmt.Errorf("发布版本 %s: %w", versionId, err)
	}
	slog.Info("内置技能已导入并发布", "slug", normalizeSlug(fmName), "version", fmVersion, "versionId", versionId)
	return nil
}

// builtinFiles 列目录下全部文件（相对路径，排序稳定保证 zip 可复现）
func builtinFiles(slugDir string) ([]string, error) {
	var files []string
	root := path.Join(builtinRoot, slugDir)
	err := fs.WalkDir(resource.SkillsBuiltin, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, p[len(root)+1:])
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("空目录")
	}
	return files, nil
}

// buildBuiltinZip 目录内容打包成内存 zip；同时解析 SKILL.md frontmatter 的 name/version
func buildBuiltinZip(slugDir string, files []string) (zipBytes []byte, name, version string, err error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		w, werr := zw.Create(f)
		if werr != nil {
			return nil, "", "", werr
		}
		data, rerr := resource.SkillsBuiltin.ReadFile(path.Join(builtinRoot, slugDir, f))
		if rerr != nil {
			return nil, "", "", rerr
		}
		if _, werr = w.Write(data); werr != nil {
			return nil, "", "", werr
		}
		if f == "SKILL.md" {
			_, fm := parseSkillMD(data)
			name, _ = fm["name"].(string)
			version, _ = fm["version"].(string)
		}
	}
	if err = zw.Close(); err != nil {
		return nil, "", "", err
	}
	return buf.Bytes(), name, version, nil
}
