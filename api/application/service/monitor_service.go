package service

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

type MonitorService struct {
	startTime time.Time
}

func NewMonitorService() *MonitorService { return &MonitorService{startTime: time.Now()} }

// Server 返回服务监控信息（对应旧 sys_monitor.List）
func (s *MonitorService) Server() (map[string]interface{}, error) {
	res := map[string]interface{}{}

	cpuNum := runtime.NumCPU()
	res["cpuNum"] = cpuNum
	cpuUsed, avg5, avg15 := 0.0, 0.0, 0.0
	if info, err := cpu.Percent(time.Second, false); err == nil && len(info) > 0 {
		cpuUsed, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", info[0]), 64)
	}
	if l, err := load.Avg(); err == nil {
		avg5, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", l.Load5), 64)
		avg15, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", l.Load15), 64)
	}
	res["cpuUsed"] = cpuUsed
	res["cpuAvg5"] = avg5
	res["cpuAvg15"] = avg15

	var total, used, usedPercent float64
	if m, err := mem.VirtualMemory(); err == nil {
		total = float64(m.Total)
		used = float64(m.Used)
		usedPercent, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", m.UsedPercent), 64)
	}
	res["memTotal"] = total
	res["memUsed"] = used
	res["memPercent"] = usedPercent

	if h, err := host.Info(); err == nil {
		res["hostName"] = h.Hostname
		res["os"] = h.OS
		res["kernelArch"] = h.KernelArch
		res["kernelVersion"] = h.KernelVersion
		res["uptime"] = h.Uptime
		res["bootTime"] = h.BootTime
	}

	if d, err := disk.Usage("/"); err == nil {
		res["diskTotal"] = float64(d.Total)
		res["diskUsed"] = float64(d.Used)
		res["diskPercent"], _ = strconv.ParseFloat(fmt.Sprintf("%.2f", d.UsedPercent), 64)
	}

	res["goVersion"] = runtime.Version()
	res["goroutines"] = runtime.NumGoroutine()
	res["startTime"] = s.startTime.Format("2006-01-02 15:04:05")
	res["pid"] = os.Getpid()
	res["runTime"] = time.Since(s.startTime).Truncate(time.Second).String()
	return res, nil
}
