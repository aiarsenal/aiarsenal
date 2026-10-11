//! 设备已装技能扫描：按调用方给的 skills 根目录扫描（openclaw / hermes 各自布局不同）。
//!
//! 心跳每跳扫一次（纯本地 FS 读，无子进程）。两种布局：
//! - flat（openclaw，~/.openclaw/skills）：每个一级子目录是一个技能，slug=一级目录名
//!   （`skills install --as` 指定），SKILL.md 可按分组布局嵌套在内 ≤6 层。
//! - grouped（hermes，~/.hermes/skills）：全树 walk，每个直接含 SKILL.md 的目录是一个
//!   技能（slug=目录名）。技能可平铺（<slug>/SKILL.md，平台落盘形态）或带 category
//!   分组（<category>/…/<slug>/SKILL.md，`skills install --category` 可产生多级嵌套）。
//! 手装技能同样被扫到。根目录不存在 → 空数组（后端据此清空存量）。

use crate::types::SkillStateReport;
use std::path::Path;

/// SKILL.md 距技能根目录的最大嵌套深度（对齐 openclaw 分组布局规则）。
const MAX_SKILL_MD_DEPTH: usize = 6;

/// 扫描 skills 根目录（openclaw 平铺布局）：slug=一级目录名。按 name 排序（输出稳定，便于后端 diff）。
pub fn scan_skills_flat(root: &Path) -> Vec<SkillStateReport> {
    let Ok(entries) = std::fs::read_dir(root) else {
        return Vec::new(); // 未装任何技能（目录不存在）→ 空数组
    };
    let mut list = Vec::new();
    let mut dirs: Vec<_> = entries.flatten().filter(|e| e.path().is_dir()).collect();
    dirs.sort_by_key(|e| e.file_name());
    for e in dirs {
        let slug = e.file_name().to_string_lossy().to_string();
        let Some(skill_root) = locate_skill_root(&e.path()) else {
            continue; // 无 SKILL.md 的目录不是技能，跳过
        };
        let (name, description, version) = parse_skill_md(&skill_root.join("SKILL.md"), &skill_root);
        list.push(SkillStateReport { name, slug, description, version });
    }
    list
}

/// 扫描排除目录（镜像 hermes agent/skill_utils.py EXCLUDED_SKILL_DIRS）：
/// 元数据/VCS/虚拟环境/依赖/缓存目录不是技能，不进扫描。
const EXCLUDED_SKILL_DIRS: &[&str] = &[
    ".git", ".github", ".hub", ".archive", ".venv", "venv", "node_modules", "site-packages",
    "__pycache__", ".tox", ".nox", ".pytest_cache", ".mypy_cache", ".ruff_cache",
];

/// 技能包内 support 目录（镜像 hermes SKILL_SUPPORT_DIRS）：技能根下这些目录不递归——
/// 其中归档的完整旧技能包（含 SKILL.md）是 progressive-disclosure 数据，不是独立技能。
const SKILL_SUPPORT_DIRS: &[&str] = &["references", "templates", "assets", "scripts"];

/// 扫描 skills 根目录（hermes 布局）：全树 walk，**每个直接含 SKILL.md 的目录是一个技能**
/// （slug=目录名），支持任意深度 category 嵌套（hub 安装 official/mlops/training/<name> 形态）。
/// 规则镜像 hermes `iter_skill_index_files`：排除 EXCLUDED 目录；技能根下剪枝 support 目录；
/// frontmatter name 去重（首见优先，排序 walk 保证确定性）。按 name 排序输出（稳定，便于后端 diff）。
pub fn scan_skills_grouped(root: &Path) -> Vec<SkillStateReport> {
    let mut list = Vec::new();
    let mut seen_names: Vec<String> = Vec::new();
    walk_skill_dirs(root, 0, &mut list, &mut seen_names);
    list.sort_by(|a, b| a.name.cmp(&b.name));
    list
}

/// 递归 walk：当前目录直接含 SKILL.md 即技能（收编、不因 support 剪枝丢自身）；
/// 子目录排序遍历（确定性），排除 EXCLUDED，技能根下剪枝 support。
fn walk_skill_dirs(dir: &Path, depth: usize, list: &mut Vec<SkillStateReport>, seen_names: &mut Vec<String>) {
    let has_skill_md = dir.join("SKILL.md").is_file();
    if has_skill_md {
        let slug = dir
            .file_name()
            .map(|n| n.to_string_lossy().to_string())
            .unwrap_or_default();
        let (name, description, version) = parse_skill_md(&dir.join("SKILL.md"), dir);
        if !name.is_empty() && !seen_names.contains(&name) {
            seen_names.push(name.clone());
            list.push(SkillStateReport { name, slug, description, version });
        }
    }
    if depth >= MAX_SKILL_MD_DEPTH {
        return;
    }
    for s in skill_subdirs(dir, has_skill_md) {
        walk_skill_dirs(&s, depth + 1, list, seen_names);
    }
}

/// 列出 dir 下可继续 walk 的子目录（排序保证确定性）：排除 EXCLUDED；
/// dir 自身是技能根（has_skill_md）时再剪枝 support 目录。walk 规则的唯一出口，
/// scan 与 find 共用，改动须两边同步。
fn skill_subdirs(dir: &Path, has_skill_md: bool) -> Vec<std::path::PathBuf> {
    let Ok(entries) = std::fs::read_dir(dir) else {
        return Vec::new();
    };
    let mut subdirs: Vec<_> = entries
        .flatten()
        .filter(|e| e.path().is_dir())
        .filter(|e| {
            let name = e.file_name().to_string_lossy().to_string();
            if EXCLUDED_SKILL_DIRS.contains(&name.as_str()) {
                return false;
            }
            // 技能根下的 support 目录不递归（其中归档的 SKILL.md 不是独立技能）
            !(has_skill_md && SKILL_SUPPORT_DIRS.contains(&name.as_str()))
        })
        .map(|e| e.path())
        .collect();
    subdirs.sort_by_key(|p| p.file_name().map(|n| n.to_os_string()));
    subdirs
}

/// 在 grouped 布局的 skills 根下按目录名（slug）定位技能目录：与 scan_skills_grouped
/// 同一 walk 规则，返回首个**直接含 SKILL.md** 的同名目录（支持 category 嵌套形态）。
/// 找不到 → None（调用方按幂等「已卸载」处理）。skill_uninstall 用。
pub fn find_skill_dir_by_slug(root: &Path, slug: &str) -> Option<std::path::PathBuf> {
    fn find(dir: &Path, slug: &str, depth: usize) -> Option<std::path::PathBuf> {
        let has_skill_md = dir.join("SKILL.md").is_file();
        if has_skill_md
            && dir.file_name().map(|n| n == slug).unwrap_or(false)
        {
            return Some(dir.to_path_buf());
        }
        if depth >= MAX_SKILL_MD_DEPTH {
            return None;
        }
        for s in skill_subdirs(dir, has_skill_md) {
            if let Some(hit) = find(&s, slug, depth + 1) {
                return Some(hit);
            }
        }
        None
    }
    find(root, slug, 0)
}

/// 移动目录：先 rename（同卷零拷贝），跨卷（EXDEV）等失败回退递归复制+删源。
/// 技能落盘用（tmp 解压目录 → skills 托管目录）。
pub fn move_dir(src: &Path, dst: &Path) -> std::io::Result<()> {
    if std::fs::rename(src, dst).is_err() {
        copy_dir_recursive(src, dst)?;
        std::fs::remove_dir_all(src)?;
    }
    Ok(())
}

fn copy_dir_recursive(src: &Path, dst: &Path) -> std::io::Result<()> {
    std::fs::create_dir_all(dst)?;
    for entry in std::fs::read_dir(src)? {
        let entry = entry?;
        if entry.file_type()?.is_dir() {
            copy_dir_recursive(&entry.path(), &dst.join(entry.file_name()))?;
        } else {
            std::fs::copy(entry.path(), dst.join(entry.file_name()))?;
        }
    }
    Ok(())
}

/// 在 dir 下（含自身）逐层找含 SKILL.md 的目录（分组布局：SKILL.md 可嵌套 ≤6 层）。
/// 返回技能根目录（含 SKILL.md 的那层），按名排序保证确定性。
/// skill_install 解压后用它定位待装根（平台层复用）。
pub fn locate_skill_root(dir: &std::path::Path) -> Option<std::path::PathBuf> {
    locate_skill_root_inner(dir, 0)
}

fn locate_skill_root_inner(dir: &std::path::Path, depth: usize) -> Option<std::path::PathBuf> {
    if dir.join("SKILL.md").is_file() {
        return Some(dir.to_path_buf());
    }
    if depth >= MAX_SKILL_MD_DEPTH {
        return None;
    }
    let Ok(entries) = std::fs::read_dir(dir) else {
        return None;
    };
    let mut subs: Vec<_> = entries.flatten().filter(|e| e.path().is_dir()).collect();
    subs.sort_by_key(|e| e.file_name());
    for s in subs {
        if let Some(hit) = locate_skill_root_inner(&s.path(), depth + 1) {
            return Some(hit);
        }
    }
    None
}

/// 解析 `openclaw skills list --json` 输出取 managedSkillsDir（全局托管目录）。
/// 输出可能混有非 JSON 行 → 截首 `{` 到末 `}` 再解；失败返回 None（调用方兜底默认目录）。
/// skill_uninstall 用（openclaw 无 uninstall 子命令，删 <managedSkillsDir>/<slug> 即卸载）。
pub fn parse_managed_skills_dir(output: &str) -> Option<std::path::PathBuf> {
    let start = output.find('{')?;
    let end = output.rfind('}')? + 1;
    if end <= start {
        return None;
    }
    let v: serde_json::Value = serde_json::from_str(&output[start..end]).ok()?;
    v.get("managedSkillsDir")?.as_str().map(std::path::PathBuf::from)
}

/// 解析 SKILL.md frontmatter（--- 包围的 YAML 头）的 name/description/version。
/// 缺 name 时回退技能根目录名（对齐 openclaw 规则）；解析失败字段留空。
fn parse_skill_md(md: &std::path::Path, skill_root: &std::path::Path) -> (String, String, String) {
    let fallback = skill_root
        .file_name()
        .map(|n| n.to_string_lossy().to_string())
        .unwrap_or_default();
    let Ok(content) = std::fs::read_to_string(md) else {
        return (fallback, String::new(), String::new());
    };
    let mut name = String::new();
    let mut description = String::new();
    let mut version = String::new();
    // frontmatter：文件以 --- 行开头，到下一个 --- 行结束；之外不解析。
    let mut lines = content.lines();
    if lines.next().map(str::trim) != Some("---") {
        return (fallback, String::new(), String::new());
    }
    for line in lines {
        let t = line.trim();
        if t == "---" || t == "..." {
            break;
        }
        let Some((k, v)) = t.split_once(':') else { continue };
        let v = v.trim().trim_matches('"').trim_matches('\'').to_string();
        match k.trim() {
            "name" if !v.is_empty() => name = v,
            "description" if !v.is_empty() => description = v,
            "version" if !v.is_empty() => version = v,
            _ => {}
        }
    }
    if name.is_empty() {
        name = fallback;
    }
    (name, description, version)
}

#[cfg(test)]
mod tests {
    use super::*;

    /// 建一次性临时 skills 根目录（含 pid+纳秒防并发冲突），返回路径。
    fn temp_root(tag: &str) -> std::path::PathBuf {
        let p = std::env::temp_dir().join(format!(
            "aa-skill-test-{}-{}-{}",
            tag,
            std::process::id(),
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .map(|d| d.as_nanos())
                .unwrap_or(0)
        ));
        std::fs::create_dir_all(&p).unwrap();
        p
    }

    /// 写一个技能：dir/SKILL.md（含 frontmatter name/version）。
    fn write_skill(dir: &Path, name: &str, version: &str) {
        std::fs::create_dir_all(dir).unwrap();
        std::fs::write(
            dir.join("SKILL.md"),
            format!("---\nname: {name}\nversion: {version}\ndescription: test\n---\nbody\n"),
        )
        .unwrap();
    }

    #[test]
    fn scan_missing_dir_is_empty() {
        // 不抹真实环境：临时 HOME 指到空目录再扫。
        let tmp = std::env::temp_dir().join(format!("aa-skill-test-{}", std::process::id()));
        std::fs::create_dir_all(&tmp).unwrap();
        // scan 读真实 home；此处仅验证目录不存在路径不 panic——用不存在前缀断言。
        assert!(locate_skill_root(&tmp.join("nope")).is_none());
        let _ = std::fs::remove_dir_all(&tmp);
    }

    #[test]
    fn scan_flat_reads_slug_dirs() {
        let root = temp_root("flat");
        write_skill(&root.join("pdf-tools"), "PDF Tools", "1.2.0");
        write_skill(&root.join("web-scraper"), "Web Scraper", "0.3.1");
        std::fs::create_dir_all(root.join("not-a-skill")).unwrap(); // 无 SKILL.md → 跳过
        let list = scan_skills_flat(&root);
        assert_eq!(list.len(), 2);
        assert_eq!(list[0].slug, "pdf-tools"); // 按 name 排序：PDF Tools < Web Scraper
        assert_eq!(list[0].name, "PDF Tools");
        assert_eq!(list[0].version, "1.2.0");
        assert_eq!(list[1].slug, "web-scraper");
        let _ = std::fs::remove_dir_all(&root);
    }

    #[test]
    fn scan_flat_missing_root_is_empty() {
        let root = temp_root("flat-missing");
        let _ = std::fs::remove_dir_all(&root);
        assert!(scan_skills_flat(&root).is_empty());
    }

    #[test]
    fn scan_grouped_flat_and_categorized() {
        let root = temp_root("grouped");
        // 平铺技能（平台落盘形态）
        write_skill(&root.join("pdf-tools"), "PDF Tools", "1.0.0");
        // category 分组技能，含多级嵌套（official/mlops/training/<name> 是真实 hub 形态）
        write_skill(&root.join("official").join("mlops").join("training").join("trl-tune"), "TRL Tune", "2.0.0");
        write_skill(&root.join("official").join("web-scraper"), "Web Scraper", "0.3.1");
        let list = scan_skills_grouped(&root);
        assert_eq!(list.len(), 3);
        let slugs: Vec<&str> = list.iter().map(|s| s.slug.as_str()).collect();
        assert!(slugs.contains(&"pdf-tools"), "slugs={slugs:?}");
        assert!(slugs.contains(&"trl-tune"), "多级嵌套技能要能扫到：slugs={slugs:?}");
        assert!(slugs.contains(&"web-scraper"));
        let _ = std::fs::remove_dir_all(&root);
    }

    #[test]
    fn scan_grouped_dedup_by_name_first_wins() {
        let root = temp_root("grouped-dedup");
        // 同 frontmatter name 的两份（排序 walk 下 extra/ 在前）→ 首见优先，只报一条
        write_skill(&root.join("extra").join("pdf-tools"), "PDF Tools", "9.9.9");
        write_skill(&root.join("pdf-tools"), "PDF Tools", "1.0.0");
        // 技能根下的 support 目录不递归：references/ 里归档的旧 SKILL.md 不是独立技能
        write_skill(&root.join("pdf-tools").join("references").join("old-pack"), "Old Pack", "0.0.1");
        let list = scan_skills_grouped(&root);
        assert_eq!(list.len(), 1, "name 去重 + support 剪枝后只应有一条：{list:?}");
        assert_eq!(list[0].version, "9.9.9", "排序 walk 首见优先（extra/ 在 pdf-tools/ 前）");
        let _ = std::fs::remove_dir_all(&root);
    }

    #[test]
    fn scan_grouped_excludes_venv_metadata() {
        let root = temp_root("grouped-excl");
        write_skill(&root.join("venv").join("lib-skill"), "Lib Skill", "1.0.0");
        write_skill(&root.join(".git").join("hook-skill"), "Hook Skill", "1.0.0");
        write_skill(&root.join("real-skill"), "Real Skill", "1.0.0");
        let list = scan_skills_grouped(&root);
        assert_eq!(list.len(), 1, "venv/.git 下的 SKILL.md 不算技能：{list:?}");
        assert_eq!(list[0].slug, "real-skill");
        let _ = std::fs::remove_dir_all(&root);
    }

    #[test]
    fn scan_grouped_missing_root_is_empty() {
        let root = temp_root("grouped-missing");
        let _ = std::fs::remove_dir_all(&root);
        assert!(scan_skills_grouped(&root).is_empty());
    }

    #[test]
    fn find_by_slug_supports_nesting_and_misses() {
        let root = temp_root("find");
        write_skill(&root.join("pdf-tools"), "PDF Tools", "1.0.0");
        write_skill(&root.join("official").join("mlops").join("trl-tune"), "TRL Tune", "2.0.0");
        // 同名目录但无 SKILL.md：不是技能，不该命中
        std::fs::create_dir_all(root.join("official").join("web-scraper")).unwrap();
        assert!(find_skill_dir_by_slug(&root, "pdf-tools").is_some());
        assert!(find_skill_dir_by_slug(&root, "trl-tune").is_some());
        let hit = find_skill_dir_by_slug(&root, "trl-tune").unwrap();
        assert!(hit.join("SKILL.md").is_file());
        assert!(find_skill_dir_by_slug(&root, "web-scraper").is_none(), "无 SKILL.md 的同名目录不命中");
        assert!(find_skill_dir_by_slug(&root, "not-exist").is_none());
        let _ = std::fs::remove_dir_all(&root);
    }

    #[test]
    fn move_dir_moves_content() {
        let base = temp_root("move");
        let src = base.join("skill-src");
        write_skill(&src, "Mover", "1.0.0");
        std::fs::write(src.join("extra.txt"), "payload").unwrap();
        let dst = base.join("skill-dst");
        move_dir(&src, &dst).unwrap();
        assert!(dst.join("SKILL.md").is_file());
        assert_eq!(std::fs::read_to_string(dst.join("extra.txt")).unwrap(), "payload");
        assert!(!src.exists(), "源目录应已移走");
        let _ = std::fs::remove_dir_all(&base);
    }
}
