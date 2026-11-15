package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	storagestg "github.com/aiarsenal/runtime-go/storage"
)

// ===== fakes（嵌入 nil 接口=哨兵：路径里多调一个未实现方法即 panic，按设计触发） =====

type fakeSkillRepo struct {
	repository.ISkillRepository
	mu        sync.Mutex
	files     []*entity.SkillFile
	addDelay  time.Duration // AddFile 刻意放慢：竞态在真实时序下必然复现
	listCalls []int         // 每次 ListFiles 时已持久化的文件数（竞态观测点）
}

func (f *fakeSkillRepo) AddVersion(ctx context.Context, v *entity.SkillVersion) (string, error) {
	return "ver-fake-1", nil
}
func (f *fakeSkillRepo) AddFile(ctx context.Context, fi *entity.SkillFile) error {
	time.Sleep(f.addDelay)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files = append(f.files, fi)
	return nil
}
func (f *fakeSkillRepo) ListFiles(ctx context.Context, versionId string) ([]*entity.SkillFile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listCalls = append(f.listCalls, len(f.files))
	out := make([]*entity.SkillFile, len(f.files))
	copy(out, f.files)
	return out, nil
}
func (f *fakeSkillRepo) UpsertSearchDocument(ctx context.Context, skillId, title, summary, keywords, searchText, namespaceId, namespaceSlug, ownerId string, visibility, status uint, vectorText string) error {
	return nil
}
func (f *fakeSkillRepo) GetVersion(ctx context.Context, versionId string) (*entity.SkillVersion, error) {
	return nil, errors.New("not found") // DownloadVersion 的命名回退路径
}

type fakeSecRepo struct {
	repository.ISkillSecurityRepository
	mu       sync.Mutex
	audits   []entity.SkillSecurityAudit // 每次 UpdateAudit 的落库快照
	rescanID string
}

func (f *fakeSecRepo) Rescan(ctx context.Context, versionId string) (string, error) {
	return "audit-fake-1", nil
}
func (f *fakeSecRepo) UpdateAudit(ctx context.Context, auditId string, status, severity uint, findings json.RawMessage, scannerRaw, contentHash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.audits = append(f.audits, entity.SkillSecurityAudit{
		Status: status, Severity: severity, FindingsJson: findings,
		ScannerRaw: scannerRaw, ContentHash: contentHash,
	})
	return nil
}
func (f *fakeSecRepo) terminal() *entity.SkillSecurityAudit {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.audits) - 1; i >= 0; i-- {
		st := f.audits[i].Status
		if st == entity.AuditStatusPassed || st == entity.AuditStatusFailed || st == entity.AuditStatusError {
			return &f.audits[i]
		}
	}
	return nil
}

type fakeStorage struct {
	storagestg.Storage
	mu    sync.Mutex
	items map[string][]byte
}

func (f *fakeStorage) Save(ctx context.Context, key string, r io.Reader) (int64, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items[key] = b
	return int64(len(b)), nil
}
func (f *fakeStorage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.items[key]
	if !ok {
		return nil, storagestg.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
func (f *fakeStorage) Delete(ctx context.Context, key string) error { return nil }
func (f *fakeStorage) Stat(ctx context.Context, key string) (*storagestg.ObjectInfo, error) {
	return nil, storagestg.ErrNotFound
}
func (f *fakeStorage) Provider() string { return "fake" }

// ===== N1：编排竞态行为测试（不依赖源码文本守卫的真时序证明） =====

// 恶意包（tool.exe）走完整 createVersionFromZip：AddFile 刻意放慢，若派发早于
// 持久化（修复前形态/变异 M1），扫描 goroutine 的 ListFiles 只看到空集，
// 以 missing_skill_md 判 PASSED——本测试断言终态必须是 FAILED。
func TestCreateVersionDispatchAfterPersistBehavior(t *testing.T) {
	zipBytes := buildZipBytes(t, map[string][]byte{
		"SKILL.md": []byte("---\nname: race-probe\ndescription: x\n---\n\nbody\n"),
		"tool.exe": []byte("MZ" + strings.Repeat("A", 64)),
	})
	srepo := &fakeSkillRepo{addDelay: 30 * time.Millisecond}
	sec := &fakeSecRepo{rescanID: "audit-fake-1"}
	stg := &fakeStorage{items: map[string][]byte{}}
	scanner := NewScannerService(srepo, sec, stg)
	svc := NewSkillPackageService(srepo, sec, stg, scanner, nil)

	sk := &entity.Skill{SkillId: "sk-1"}
	vid, err := svc.createVersionFromZip(context.Background(), sk, "1.0.0", "", zipBytes, "u1")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if vid != "ver-fake-1" {
		t.Fatalf("versionId=%s", vid)
	}
	// 等扫描落终态（异步 goroutine；真实扫描毫秒级）
	deadline := time.Now().Add(3 * time.Second)
	var final *entity.SkillSecurityAudit
	for time.Now().Before(deadline) {
		if final = sec.terminal(); final != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if final == nil {
		t.Fatal("3s 内扫描未落终态")
	}
	// 竞态下此处为 PASSED（扫描只看到空文件集、仅 missing_skill_md 低危）
	if final.Status != entity.AuditStatusFailed {
		t.Fatalf("恶意包首扫终态=%d（期望 FAILED=2）——竞态复现：扫描未看到持久化后的文件集，findings=%s",
			final.Status, string(final.FindingsJson))
	}
	if !strings.Contains(string(final.FindingsJson), "banned_extension") {
		t.Fatalf("未识别 banned_extension: %s", string(final.FindingsJson))
	}
}

// ===== N2：扫描器→门禁的哈希接线（审计写下的哈希必须等于门禁重算值） =====

func TestScanVersionWritesGateConsistentHash(t *testing.T) {
	files := []*entity.SkillFile{
		{FilePath: "a.md", FileSize: 4, Sha256: mustSha(t, []byte("dddd")), StorageKey: "k1"},
		{FilePath: "b.txt", FileSize: 4, Sha256: mustSha(t, []byte("eeee")), StorageKey: "k2"},
	}
	srepo := &fakeSkillRepo{}
	srepo.files = files
	sec := &fakeSecRepo{}
	stg := &fakeStorage{items: map[string][]byte{"k1": []byte("dddd"), "k2": []byte("eeee")}}
	scanner := NewScannerService(srepo, sec, stg)

	if err := scanner.ScanVersion(context.Background(), "ver-1", "audit-1"); err != nil {
		t.Fatalf("ScanVersion: %v", err)
	}
	if len(sec.audits) == 0 {
		t.Fatal("未写审计")
	}
	last := sec.audits[len(sec.audits)-1]
	want := skillContentHash(files)
	if last.ContentHash == "" {
		t.Fatal("终态审计未写 content_hash——门禁「无内容绑定」会把全量版本砖死（接线断）")
	}
	if last.ContentHash != want {
		t.Fatalf("审计哈希 %s ≠ 门禁重算 %s——扫描侧与门禁侧口径漂移", last.ContentHash, want)
	}
	if last.Status != entity.AuditStatusPassed {
		t.Fatalf("干净文件应 PASSED, 得 %d findings=%s", last.Status, string(last.FindingsJson))
	}
}

// ===== B3：下载时逐文件 sha256 校验（存储字节对回行值） =====

func TestDownloadVersionVerifiesStorageBytes(t *testing.T) {
	files := []*entity.SkillFile{
		{FilePath: "ok.txt", Sha256: mustSha(t, []byte("good")), StorageKey: "k1"},
		{FilePath: "bad.txt", Sha256: mustSha(t, []byte("good")), StorageKey: "k2"}, // 行值相同
	}
	srepo := &fakeSkillRepo{}
	srepo.files = files
	svc := NewSkillPackageService(srepo, nil, nil, nil, nil)

	// 存储字节被带外篡改（bad.txt 实际内容与行值 sha256 不符）
	stg := &fakeStorage{items: map[string][]byte{"k1": []byte("good"), "k2": []byte("tampered")}}
	svc.storage = stg
	_, _, err := svc.DownloadVersion(context.Background(), "ver-1")
	if !errors.Is(err, ErrSkillContentMismatch) {
		t.Fatalf("篡改字节应返回 ErrSkillContentMismatch, 得 %v", err)
	}

	// 字节一致 → 打包成功
	stg2 := &fakeStorage{items: map[string][]byte{"k1": []byte("good"), "k2": []byte("good")}}
	svc.storage = stg2
	_, data, err := svc.DownloadVersion(context.Background(), "ver-1")
	if err != nil {
		t.Fatalf("一致字节不应失败: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("zip 为空")
	}
}

// ===== B2：isMaybeText 与黑名单同口径（内容扫描不再被文件名变体绕过） =====

func TestIsMaybeTextNormalized(t *testing.T) {
	if !isMaybeText("secrets.ｔｘｔ") {
		t.Fatal("全角 .ｔｘｔ 未进内容扫描——私钥规则整层逃逸（B2 回归）")
	}
	if !isMaybeText("env.txt ") {
		t.Fatal("尾空格 .txt 未进内容扫描（B2 回归）")
	}
	if !isMaybeText("notes.TXT.") {
		t.Fatal("尾点+大写 .TXT 未进内容扫描")
	}
	if isMaybeText("tool.exe") {
		t.Fatal("二进制不应进文本内容扫描")
	}
}

// ===== helpers =====

func mustSha(t *testing.T, b []byte) string {
	t.Helper()
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func buildZipBytes(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
