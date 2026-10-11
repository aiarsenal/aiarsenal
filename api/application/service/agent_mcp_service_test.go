package service

import "testing"

// mcpServerEntries 的 server key 遇 hermes 保留字（all/*）加 aa- 前缀：
// hermes 把 server key 注册为 toolset 别名，"all"/"*" 是它内置的「全部工具」特殊别名，
// 撞名后 resolve_toolset 无限递归，实例一收消息就崩（2026-09-12 微信绑定后踩过）。
// url 不动——/mcp/{组名} 是网关聚合 scope，与 agent 侧 server key 无关。
func TestMcpServerEntriesReservedKeyPrefix(t *testing.T) {
	entries := mcpServerEntries("http://gw:43213", []string{"all", "*", "demo1"}, "sk", false)
	if _, ok := entries["all"]; ok {
		t.Fatalf("保留字 all 未加前缀: %v", entries)
	}
	if _, ok := entries["*"]; ok {
		t.Fatalf("保留字 * 未加前缀: %v", entries)
	}
	all := entries["aa-all"].(map[string]interface{})
	if all["url"] != "http://gw:43213/mcp/all" {
		t.Fatalf("all 前缀化了 key 但 url 也被改了: %v", all)
	}
	star := entries["aa-*"].(map[string]interface{})
	if star["url"] != "http://gw:43213/mcp/*" {
		t.Fatalf("* 前缀化了 key 但 url 也被改了: %v", star)
	}
	// 普通组名原样，不受影响。
	demo := entries["demo1"].(map[string]interface{})
	if demo["url"] != "http://gw:43213/mcp/demo1" {
		t.Fatalf("普通组名被误改: %v", demo)
	}
	// openclaw 链路（withTransport=true）同防御。
	oc := mcpServerEntries("http://gw:43213", []string{"all"}, "sk", true)
	ocAll := oc["aa-all"].(map[string]interface{})
	if ocAll["transport"] != "streamable-http" {
		t.Fatalf("openclaw 条目缺 transport: %v", ocAll)
	}
}
