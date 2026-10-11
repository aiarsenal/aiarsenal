// Package resource 内嵌资源入口。
//
// SkillsBuiltin 只嵌 data/skills-builtin/（平台内置配对技能，随二进制分发、首启幂等导入）。
// ⛔ 绝不嵌 data/skill/ ——那是技能包运行时存储（用户上传/市场导入落盘的地方，容量不可控）。
package resource

import "embed"

//go:embed data/skills-builtin
var SkillsBuiltin embed.FS
