package service

import (
	"context"
	"testing"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

type fakeOnlineRepo struct {
	repository.IUserOnlineRepository
	deleted []string
}

func (f *fakeOnlineRepo) List(ctx context.Context, username, ip string, pageNum, pageSize int) ([]*entity.UserOnline, int64, error) {
	return []*entity.UserOnline{{Uuid: "row-1"}}, 1, nil
}

func (f *fakeOnlineRepo) Delete(ctx context.Context, uuid string) error {
	f.deleted = append(f.deleted, uuid)
	return nil
}

// TestKickRejectsNonSuper 吊销会话是接管类管理动作：路由未登记进菜单表时中间件会
// 默认放行，服务层必须自证调用者是超管——不能把闸门寄托在菜单数据上。
func TestKickRejectsNonSuper(t *testing.T) {
	online := &fakeOnlineRepo{}
	svc := NewUserOnlineService(online, &fakeUserRepo{
		curUsers: map[string]*entity.User{"u-1": ordinaryUser("u-1")},
	})

	if err := svc.Kick(context.Background(), "u-1", "u-2"); err == nil {
		t.Fatal("非超管吊销他人会话应当被拒，实际通过")
	}
	if len(online.deleted) != 0 {
		t.Fatal("拒绝路径不应清理在线记录")
	}
}

// TestKickRejectsUnknownCaller 操作者查无记录时按拒绝处理，与用户维护闸门同口径。
func TestKickRejectsUnknownCaller(t *testing.T) {
	svc := NewUserOnlineService(&fakeOnlineRepo{}, &fakeUserRepo{
		curUsers: map[string]*entity.User{"u-2": ordinaryUser("u-2")},
	})

	if err := svc.Kick(context.Background(), "ghost", "u-2"); err == nil {
		t.Fatal("操作者记录缺失时应当被拒，实际通过")
	}
}

// TestKickAllowsSuper 超管吊销：会话注册表清空 + 在线展示行清理。
func TestKickAllowsSuper(t *testing.T) {
	online := &fakeOnlineRepo{}
	svc := NewUserOnlineService(online, &fakeUserRepo{
		curUsers: map[string]*entity.User{
			"u-9": superUser("u-9"),
			"u-2": ordinaryUser("u-2"),
		},
	})

	if err := svc.Kick(context.Background(), "u-9", "u-2"); err != nil {
		t.Fatalf("超管吊销会话被误拦: %v", err)
	}
	if len(online.deleted) == 0 {
		t.Fatal("在线展示行未被清理")
	}
}
