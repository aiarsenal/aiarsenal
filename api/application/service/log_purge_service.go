package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/alert"
)

// LogPurgeService 审计日志留存清理（PR③ 等保 8.1.4.3c + 网安法 21 条的双面：
// 留存 ≥180 天是硬下限，之上允许 sanctioned 清理防磁盘膨胀）。删除/清空的人工端点
// 已被 audit.protection 闸门管住；本服务是**唯一的自动清理通道**——只清两表、
// 动作本身写 aa_oper_log + 触发 log_cleanup 告警，绝不静默。
//
// 调度：main（仅 api 进程）经 scheduler 每日注册一次；retentionDays 由 main 从
// config 取归一后的值注入（构造期固定，测试友好）。清理错误只记 ERROR 日志，
// 不阻断调度循环（fail-open：清理失败=数据多留，合规无虞）。
type LogPurgeService struct {
	loginLogRepo  repository.ILoginLogRepository
	operLogRepo   repository.IOperLogRepository
	retentionDays int // 已在 config 归一（≥180）
	now           func() time.Time
}

func NewLogPurgeService(l repository.ILoginLogRepository, o repository.IOperLogRepository, retentionDays int) *LogPurgeService {
	return &LogPurgeService{loginLogRepo: l, operLogRepo: o, retentionDays: retentionDays, now: time.Now}
}

// PurgeOnce 执行一轮清理：两表超过留存下限的行物理删除，动作写 oper_log + 告警。
// 返回两表删除行数；错误已记日志并如实上返（调度层只记不重试，下轮自然再清）。
func (s *LogPurgeService) PurgeOnce(ctx context.Context) (loginRows, operRows int64, err error) {
	cutoff := s.now().AddDate(0, 0, -s.retentionDays)

	loginRows, err = s.loginLogRepo.PurgeBefore(ctx, cutoff)
	if err != nil {
		slog.Error("登录日志留存清理失败", "err", err, "cutoff", cutoff)
		return 0, 0, err
	}
	operRows, err = s.operLogRepo.PurgeBefore(ctx, cutoff)
	if err != nil {
		slog.Error("操作日志留存清理失败", "err", err, "cutoff", cutoff)
		return loginRows, 0, err
	}

	// 清理动作自身留痕进 aa_oper_log（等保：审计进程的每一步可审计）。
	now := s.now()
	traceErr := s.operLogRepo.Insert(ctx, &entity.OperLog{
		Title:         "日志留存清理",
		Method:        "LogPurgeService.PurgeOnce",
		RequestMethod: "SCHEDULE",
		OperatorType:  1,
		OperName:      "system",
		OperUrl:       "schedule:audit-log-purge",
		OperParam:     fmt.Sprintf(`{"retentionDays":%d,"cutoff":%q}`, s.retentionDays, cutoff.Format(time.RFC3339)),
		OperTime:      (*types.Time)(&now),
	})
	if traceErr != nil {
		// 留痕失败不静默（审计 P2）：写进 log_cleanup 告警正文，让「删了 N 行但
		// oper_log 无痕」对外可感知；PurgeOnce 本身仍算成功（数据已删，回滚无意义）。
		slog.Error("留存清理动作写操作日志失败", "err", traceErr)
	}

	// 告警：清理是删除类动作，PR② 的事件面必须可见（bizKey 留空不节流——每日一次量极小）。
	traceNote := ""
	if traceErr != nil {
		traceNote = " 注意：本轮操作留痕写入失败（" + traceErr.Error() + "），aa_oper_log 无本轮记录。"
	}
	alert.Dispatch("log_cleanup", "info", "审计日志留存清理",
		fmt.Sprintf("清理超过 %d 天的审计日志：登录日志 %d 行、操作日志 %d 行（cutoff %s）。%s",
			s.retentionDays, loginRows, operRows, cutoff.Format("2006-01-02 15:04:05"), traceNote),
		"")

	slog.Info("审计日志留存清理完成", "loginRows", loginRows, "operRows", operRows, "cutoff", cutoff)
	return loginRows, operRows, nil
}
