// 一次性导出器：读取当前线上库（已为 id+<entity>_id 新 schema），为每行生成
// 随机 32 位业务 id（idgen.New），并维护全局旧 biz→新 biz 映射以转换所有 FK 引用。
// 用法：go run ./cmd/dump > resource/data/aiarsenal.sql
package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"gorm.io/gorm"

	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

type rowType struct {
	cols []string
}

var m = map[string]string{} // 旧 biz_id → 新随机 biz_id

// gen 为旧 biz 生成（或复用）新随机 biz；"0"/空 保持 "0"
func gen(old string) string {
	if old == "" || old == "0" {
		return "0"
	}
	if nv, ok := m[old]; ok {
		return nv
	}
	nv := idgen.New()
	m[old] = nv
	return nv
}

// mapFK 把 FK 列值映射到新 biz；未在 map 中的非 "0" 值原样返回（容错）
func mapFK(v string) string {
	if v == "" || v == "0" {
		return "0"
	}
	if nv, ok := m[v]; ok {
		return nv
	}
	return v
}

// mapAncestors：逗号分隔 biz 列表，逐个映射
func mapAncestors(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		out = append(out, mapFK(p))
	}
	return strings.Join(out, ",")
}

// q：SQL 字符串引号；"NULL" → NULL
func q(s string) string {
	if s == "NULL" {
		return "NULL"
	}
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func main() {
	if _, err := db.InitDB(); err != nil {
		log.Fatalf("init db: %v", err)
	}
	gdb := db.DB()

	menuRows := readRows(gdb, "SELECT id,menu_id,pid,name,title,icon,condition,remark,menu_type,weigh,is_hide,path,component,is_link,module_type,model_id,is_iframe,is_cached,redirect,is_affix,link_url,created_at,updated_at FROM aa_menu ORDER BY id")
	deptRows := readRows(gdb, "SELECT id,dept_id,parent_id,ancestors,dept_name,order_num,leader,phone,email,status,created_by,updated_by,created_at,updated_at,deleted_at FROM aa_dept ORDER BY id")
	roleRows := readRows(gdb, "SELECT id,role_id,status,list_order,name,remark,created_at,updated_at FROM aa_role ORDER BY id")
	postRows := readRows(gdb, "SELECT id,post_id,post_code,post_name,post_sort,status,remark,created_by,updated_by,created_at,updated_at FROM aa_post ORDER BY id")
	userRows := readRows(gdb, "SELECT id,user_id,user_name,mobile,user_nickname,birthday,user_password,user_salt,user_status,user_email,sex,avatar,dept_id,remark,is_admin,address,describe,last_login_ip,last_login_time,created_at,updated_at,deleted_at FROM aa_user ORDER BY id")
	loginRows := readRows(gdb, "SELECT id,info_id,login_name,ipaddr,login_location,browser,os,status,msg,login_time,module FROM aa_login_log ORDER BY id")
	operRows := readRows(gdb, "SELECT id,oper_id,title,business_type,method,request_method,operator_type,oper_name,dept_name,oper_url,oper_ip,oper_location,oper_param,error_msg,oper_time FROM aa_oper_log ORDER BY id")
	onlineRows := readRows(gdb, "SELECT id,uuid,token,create_time,user_name,ip,explorer,os FROM aa_user_online ORDER BY id")
	upRows := readRows(gdb, "SELECT id,user_id,post_id FROM aa_user_post ORDER BY id")
	ruleRows := readRows(gdb, "SELECT ptype,v0,v1,v2,v3,v4,v5 FROM aa_rule")

	// Phase 1：为每行生成随机 biz，填充 map（user_online.uuid 不参与，保留原值）
	for _, r := range menuRows {
		gen(r.cols[1])
	}
	for _, r := range deptRows {
		gen(r.cols[1])
	}
	for _, r := range roleRows {
		gen(r.cols[1])
	}
	for _, r := range postRows {
		gen(r.cols[1])
	}
	for _, r := range userRows {
		gen(r.cols[1])
	}
	for _, r := range loginRows {
		gen(r.cols[1])
	}
	for _, r := range operRows {
		gen(r.cols[1])
	}

	var b strings.Builder
	b.WriteString("-- aiarsenal PostgreSQL schema (id bigserial PK + <entity>_id varchar(32) 业务 id 随机)\n")
	b.WriteString("-- 业务 id 由 idgen 生成（32 位小写 [a-z0-9]，首尾字母）；自增 id 为内部主键\n\n")

	// ===== aa_rule（casbin，保持框架形状，无 id）=====
	b.WriteString("DROP TABLE IF EXISTS aa_rule CASCADE;\n")
	b.WriteString("CREATE TABLE aa_rule (\n  ptype varchar(10) DEFAULT NULL,\n  v0 varchar(256) DEFAULT NULL,\n  v1 varchar(256) DEFAULT NULL,\n  v2 varchar(256) DEFAULT NULL,\n  v3 varchar(256) DEFAULT NULL,\n  v4 varchar(256) DEFAULT NULL,\n  v5 varchar(256) DEFAULT NULL\n);\n")
	for _, r := range ruleRows {
		pt, v0, v1, v2, v3, v4, v5 := r.cols[0], r.cols[1], r.cols[2], r.cols[3], r.cols[4], r.cols[5], r.cols[6]
		if pt == "p" {
			v0 = mapFK(v0) // role biz
			v1 = mapFK(v1) // menu biz
		} else if pt == "g" {
			if strings.HasPrefix(v0, "u_") {
				v0 = "u_" + mapFK(strings.TrimPrefix(v0, "u_"))
			}
			v1 = mapFK(v1) // role biz
		}
		b.WriteString(fmt.Sprintf("INSERT INTO aa_rule VALUES (%s,%s,%s,%s,%s,%s,%s);\n", q(pt), q(v0), q(v1), q(v2), q(v3), q(v4), q(v5)))
	}
	b.WriteString("\n")

	// ===== aa_menu =====
	b.WriteString("DROP TABLE IF EXISTS aa_menu CASCADE;\n")
	b.WriteString("CREATE TABLE aa_menu (\n  id bigserial,\n  menu_id varchar(32) NOT NULL DEFAULT '',\n  pid varchar(32) NOT NULL DEFAULT '0',\n  name varchar(100) NOT NULL DEFAULT '',\n  title varchar(50) NOT NULL DEFAULT '',\n  icon varchar(300) NOT NULL DEFAULT '',\n  condition varchar(255) NOT NULL DEFAULT '',\n  remark varchar(255) NOT NULL DEFAULT '',\n  menu_type smallint NOT NULL DEFAULT 0,\n  weigh integer NOT NULL DEFAULT 0,\n  is_hide smallint NOT NULL DEFAULT 0,\n  path varchar(100) NOT NULL DEFAULT '',\n  component varchar(100) NOT NULL DEFAULT '',\n  is_link smallint NOT NULL DEFAULT 0,\n  module_type varchar(30) NOT NULL DEFAULT '',\n  model_id integer NOT NULL DEFAULT 0,\n  is_iframe smallint NOT NULL DEFAULT 0,\n  is_cached smallint NOT NULL DEFAULT 0,\n  redirect varchar(255) NOT NULL DEFAULT '',\n  is_affix smallint NOT NULL DEFAULT 0,\n  link_url varchar(500) NOT NULL DEFAULT '',\n  created_at timestamp DEFAULT NULL,\n  updated_at timestamp DEFAULT NULL,\n  PRIMARY KEY (id)\n);\n")
	b.WriteString("CREATE UNIQUE INDEX IF NOT EXISTS idx_aa_menu_menu_id ON aa_menu (menu_id);\n")
	b.WriteString("CREATE UNIQUE INDEX IF NOT EXISTS idx_aa_menu_name ON aa_menu (name);\n")
	b.WriteString("CREATE INDEX IF NOT EXISTS idx_aa_menu_pid ON aa_menu (pid);\n")
	b.WriteString("CREATE INDEX IF NOT EXISTS idx_aa_menu_weigh ON aa_menu (weigh);\n")
	for _, r := range menuRows {
		newBiz := gen(r.cols[1])
		pid := mapFK(r.cols[2])
		c := r.cols[3:]
		b.WriteString(fmt.Sprintf("INSERT INTO aa_menu (id,menu_id,pid,name,title,icon,condition,remark,menu_type,weigh,is_hide,path,component,is_link,module_type,model_id,is_iframe,is_cached,redirect,is_affix,link_url,created_at,updated_at) VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s);\n",
			r.cols[0], q(newBiz), q(pid), q(c[0]), q(c[1]), q(c[2]), q(c[3]), q(c[4]), c[5], c[6], c[7], q(c[8]), q(c[9]), c[10], q(c[11]), c[12], c[13], c[14], q(c[15]), c[16], q(c[17]), q(c[18]), q(c[19])))
	}
	b.WriteString("\n")

	// ===== aa_dept =====
	b.WriteString("DROP TABLE IF EXISTS aa_dept CASCADE;\n")
	b.WriteString("CREATE TABLE aa_dept (\n  id bigserial,\n  dept_id varchar(32) NOT NULL DEFAULT '',\n  parent_id varchar(32) NOT NULL DEFAULT '0',\n  ancestors varchar(500) DEFAULT '',\n  dept_name varchar(30) DEFAULT '',\n  order_num integer DEFAULT 0,\n  leader varchar(20) DEFAULT NULL,\n  phone varchar(11) DEFAULT NULL,\n  email varchar(50) DEFAULT NULL,\n  status smallint DEFAULT 0,\n  created_by varchar(32) DEFAULT '0',\n  updated_by varchar(32) DEFAULT '0',\n  created_at timestamp DEFAULT NULL,\n  updated_at timestamp DEFAULT NULL,\n  deleted_at timestamp DEFAULT NULL,\n  PRIMARY KEY (id)\n);\n")
	b.WriteString("CREATE UNIQUE INDEX IF NOT EXISTS idx_aa_dept_dept_id ON aa_dept (dept_id);\n")
	for _, r := range deptRows {
		newBiz := gen(r.cols[1])
		parentId := mapFK(r.cols[2])
		ancestors := mapAncestors(r.cols[3])
		createdBy := mapFK(r.cols[10])
		updatedBy := mapFK(r.cols[11])
		b.WriteString(fmt.Sprintf("INSERT INTO aa_dept (id,dept_id,parent_id,ancestors,dept_name,order_num,leader,phone,email,status,created_by,updated_by,created_at,updated_at,deleted_at) VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s);\n",
			r.cols[0], q(newBiz), q(parentId), q(ancestors), q(r.cols[4]), r.cols[5], q(r.cols[6]), q(r.cols[7]), q(r.cols[8]), r.cols[9], q(createdBy), q(updatedBy), q(r.cols[12]), q(r.cols[13]), q(r.cols[14])))
	}
	b.WriteString("\n")

	// ===== aa_role =====
	b.WriteString("DROP TABLE IF EXISTS aa_role CASCADE;\n")
	b.WriteString("CREATE TABLE aa_role (\n  id bigserial,\n  role_id varchar(32) NOT NULL DEFAULT '',\n  status smallint DEFAULT 0,\n  list_order integer DEFAULT 0,\n  name varchar(30) DEFAULT '',\n  remark varchar(100) DEFAULT NULL,\n  created_at timestamp DEFAULT NULL,\n  updated_at timestamp DEFAULT NULL,\n  PRIMARY KEY (id)\n);\n")
	b.WriteString("CREATE UNIQUE INDEX IF NOT EXISTS idx_aa_role_role_id ON aa_role (role_id);\n")
	for _, r := range roleRows {
		newBiz := gen(r.cols[1])
		b.WriteString(fmt.Sprintf("INSERT INTO aa_role (id,role_id,status,list_order,name,remark,created_at,updated_at) VALUES (%s,%s,%s,%s,%s,%s,%s,%s);\n",
			r.cols[0], q(newBiz), r.cols[2], r.cols[3], q(r.cols[4]), q(r.cols[5]), q(r.cols[6]), q(r.cols[7])))
	}
	b.WriteString("\n")

	// ===== aa_post =====
	b.WriteString("DROP TABLE IF EXISTS aa_post CASCADE;\n")
	b.WriteString("CREATE TABLE aa_post (\n  id bigserial,\n  post_id varchar(32) NOT NULL DEFAULT '',\n  post_code varchar(64) DEFAULT '',\n  post_name varchar(50) DEFAULT '',\n  post_sort integer DEFAULT 0,\n  status smallint DEFAULT 0,\n  remark varchar(500) DEFAULT NULL,\n  created_by varchar(32) DEFAULT '0',\n  updated_by varchar(32) DEFAULT '0',\n  created_at timestamp DEFAULT NULL,\n  updated_at timestamp DEFAULT NULL,\n  PRIMARY KEY (id)\n);\n")
	b.WriteString("CREATE UNIQUE INDEX IF NOT EXISTS idx_aa_post_post_id ON aa_post (post_id);\n")
	for _, r := range postRows {
		newBiz := gen(r.cols[1])
		createdBy := mapFK(r.cols[7])
		updatedBy := mapFK(r.cols[8])
		b.WriteString(fmt.Sprintf("INSERT INTO aa_post (id,post_id,post_code,post_name,post_sort,status,remark,created_by,updated_by,created_at,updated_at) VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s);\n",
			r.cols[0], q(newBiz), q(r.cols[2]), q(r.cols[3]), r.cols[4], r.cols[5], q(r.cols[6]), q(createdBy), q(updatedBy), q(r.cols[9]), q(r.cols[10])))
	}
	b.WriteString("\n")

	// ===== aa_user =====
	b.WriteString("DROP TABLE IF EXISTS aa_user CASCADE;\n")
	b.WriteString("CREATE TABLE aa_user (\n  id bigserial,\n  user_id varchar(32) NOT NULL DEFAULT '',\n  user_name varchar(60) NOT NULL DEFAULT '',\n  mobile varchar(20) NOT NULL DEFAULT '',\n  user_nickname varchar(50) NOT NULL DEFAULT '',\n  birthday integer NOT NULL DEFAULT 0,\n  user_password varchar(255) NOT NULL DEFAULT '',\n  user_salt char(10) NOT NULL,\n  user_status smallint NOT NULL DEFAULT 1,\n  user_email varchar(100) NOT NULL DEFAULT '',\n  sex smallint NOT NULL DEFAULT 0,\n  avatar varchar(255) NOT NULL DEFAULT '',\n  dept_id varchar(32) NOT NULL DEFAULT '0',\n  remark varchar(255) NOT NULL DEFAULT '',\n  is_admin smallint NOT NULL DEFAULT 0,\n  address varchar(255) NOT NULL DEFAULT '',\n  describe varchar(255) NOT NULL DEFAULT '',\n  last_login_ip varchar(50) NOT NULL DEFAULT '',\n  last_login_time timestamp DEFAULT NULL,\n  created_at timestamp DEFAULT NULL,\n  updated_at timestamp DEFAULT NULL,\n  deleted_at timestamp DEFAULT NULL,\n  PRIMARY KEY (id)\n);\n")
	b.WriteString("CREATE UNIQUE INDEX IF NOT EXISTS idx_aa_user_user_id ON aa_user (user_id);\n")
	b.WriteString("CREATE UNIQUE INDEX IF NOT EXISTS idx_aa_user_user_login ON aa_user (user_name, deleted_at);\n")
	for _, r := range userRows {
		newBiz := gen(r.cols[1])
		deptId := mapFK(r.cols[12])
		b.WriteString(fmt.Sprintf("INSERT INTO aa_user (id,user_id,user_name,mobile,user_nickname,birthday,user_password,user_salt,user_status,user_email,sex,avatar,dept_id,remark,is_admin,address,describe,last_login_ip,last_login_time,created_at,updated_at,deleted_at) VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s);\n",
			r.cols[0], q(newBiz), q(r.cols[2]), q(r.cols[3]), q(r.cols[4]), r.cols[5], q(r.cols[6]), q(r.cols[7]), r.cols[8], q(r.cols[9]), r.cols[10], q(r.cols[11]), q(deptId), q(r.cols[13]), r.cols[14], q(r.cols[15]), q(r.cols[16]), q(r.cols[17]), q(r.cols[18]), q(r.cols[19]), q(r.cols[20]), q(r.cols[21])))
	}
	b.WriteString("\n")

	// ===== aa_login_log =====
	b.WriteString("DROP TABLE IF EXISTS aa_login_log CASCADE;\n")
	b.WriteString("CREATE TABLE aa_login_log (\n  id bigserial,\n  info_id varchar(32) NOT NULL DEFAULT '',\n  login_name varchar(50) DEFAULT '',\n  ipaddr varchar(50) DEFAULT '',\n  login_location varchar(255) DEFAULT '',\n  browser varchar(50) DEFAULT '',\n  os varchar(50) DEFAULT '',\n  status smallint DEFAULT 0,\n  msg varchar(255) DEFAULT '',\n  login_time timestamp DEFAULT NULL,\n  module varchar(30) DEFAULT '',\n  PRIMARY KEY (id)\n);\n")
	b.WriteString("CREATE UNIQUE INDEX IF NOT EXISTS idx_aa_login_log_info_id ON aa_login_log (info_id);\n")
	for _, r := range loginRows {
		newBiz := gen(r.cols[1])
		b.WriteString(fmt.Sprintf("INSERT INTO aa_login_log (id,info_id,login_name,ipaddr,login_location,browser,os,status,msg,login_time,module) VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s);\n",
			r.cols[0], q(newBiz), q(r.cols[2]), q(r.cols[3]), q(r.cols[4]), q(r.cols[5]), q(r.cols[6]), r.cols[7], q(r.cols[8]), q(r.cols[9]), q(r.cols[10])))
	}
	b.WriteString("\n")

	// ===== aa_oper_log =====
	b.WriteString("DROP TABLE IF EXISTS aa_oper_log CASCADE;\n")
	b.WriteString("CREATE TABLE aa_oper_log (\n  id bigserial,\n  oper_id varchar(32) NOT NULL DEFAULT '',\n  title varchar(50) DEFAULT '',\n  business_type integer DEFAULT 0,\n  method varchar(100) DEFAULT '',\n  request_method varchar(10) DEFAULT '',\n  operator_type integer DEFAULT 0,\n  oper_name varchar(50) DEFAULT '',\n  dept_name varchar(50) DEFAULT '',\n  oper_url varchar(500) DEFAULT '',\n  oper_ip varchar(50) DEFAULT '',\n  oper_location varchar(255) DEFAULT '',\n  oper_param text,\n  error_msg varchar(2000) DEFAULT '',\n  oper_time timestamp DEFAULT NULL,\n  PRIMARY KEY (id)\n);\n")
	b.WriteString("CREATE UNIQUE INDEX IF NOT EXISTS idx_aa_oper_log_oper_id ON aa_oper_log (oper_id);\n")
	for _, r := range operRows {
		newBiz := gen(r.cols[1])
		b.WriteString(fmt.Sprintf("INSERT INTO aa_oper_log (id,oper_id,title,business_type,method,request_method,operator_type,oper_name,dept_name,oper_url,oper_ip,oper_location,oper_param,error_msg,oper_time) VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s);\n",
			r.cols[0], q(newBiz), q(r.cols[2]), r.cols[3], q(r.cols[4]), q(r.cols[5]), r.cols[6], q(r.cols[7]), q(r.cols[8]), q(r.cols[9]), q(r.cols[10]), q(r.cols[11]), q(r.cols[12]), q(r.cols[13]), q(r.cols[14])))
	}
	b.WriteString("\n")

	// ===== aa_user_online =====
	b.WriteString("DROP TABLE IF EXISTS aa_user_online CASCADE;\n")
	b.WriteString("CREATE TABLE aa_user_online (\n  id bigserial,\n  uuid char(32) NOT NULL DEFAULT '',\n  token varchar(255) NOT NULL DEFAULT '',\n  create_time timestamp DEFAULT NULL,\n  user_name varchar(255) NOT NULL,\n  ip varchar(120) NOT NULL DEFAULT '',\n  explorer varchar(30) NOT NULL DEFAULT '',\n  os varchar(30) NOT NULL DEFAULT '',\n  PRIMARY KEY (id)\n);\n")
	b.WriteString("CREATE UNIQUE INDEX IF NOT EXISTS idx_aa_user_online_uni_token ON aa_user_online (token);\n")
	b.WriteString("CREATE UNIQUE INDEX IF NOT EXISTS idx_aa_user_online_uni_uuid ON aa_user_online (uuid);\n")
	for _, r := range onlineRows {
		b.WriteString(fmt.Sprintf("INSERT INTO aa_user_online (id,uuid,token,create_time,user_name,ip,explorer,os) VALUES (%s,%s,%s,%s,%s,%s,%s,%s);\n",
			r.cols[0], q(r.cols[1]), q(r.cols[2]), q(r.cols[3]), q(r.cols[4]), q(r.cols[5]), q(r.cols[6]), q(r.cols[7])))
	}
	b.WriteString("\n")

	// ===== aa_user_post =====
	b.WriteString("DROP TABLE IF EXISTS aa_user_post CASCADE;\n")
	b.WriteString("CREATE TABLE aa_user_post (\n  id bigserial,\n  user_id varchar(32) NOT NULL,\n  post_id varchar(32) NOT NULL,\n  PRIMARY KEY (id)\n);\n")
	for _, r := range upRows {
		uid := mapFK(r.cols[1])
		pid := mapFK(r.cols[2])
		b.WriteString(fmt.Sprintf("INSERT INTO aa_user_post (user_id,post_id) VALUES (%s,%s);\n", q(uid), q(pid)))
	}
	b.WriteString("\n")

	b.WriteString("DROP TABLE IF EXISTS aa_job CASCADE;\n")
	b.WriteString("DROP TABLE IF EXISTS aa_job_log CASCADE;\n\n")

	for _, t := range []string{"aa_user", "aa_role", "aa_dept", "aa_post", "aa_menu", "aa_oper_log", "aa_login_log", "aa_user_online", "aa_user_post"} {
		b.WriteString(fmt.Sprintf("SELECT setval(pg_get_serial_sequence('%s','id'), (SELECT max(id) FROM %s));\n", t, t))
	}

	fmt.Print(b.String())
}

func readRows(gdb *gorm.DB, query string) []rowType {
	rows, err := gdb.Raw(query).Rows()
	if err != nil {
		log.Fatalf("query %q: %v", query, err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		log.Fatalf("columns: %v", err)
	}
	var out []rowType
	for rows.Next() {
		vals := make([]sql.NullString, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			log.Fatalf("scan: %v", err)
		}
		r := rowType{cols: make([]string, len(vals))}
		for i, v := range vals {
			if v.Valid {
				r.cols[i] = v.String
			} else {
				r.cols[i] = "NULL"
			}
		}
		out = append(out, r)
	}
	return out
}
