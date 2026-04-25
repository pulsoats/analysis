package health

import (
	"context"
	"time"

	"github.com/pulsoats/core/health"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

type pinger interface {
	Ping(ctx context.Context) error
}

type Application struct {
	info      health.ServiceInfo
	db        pinger
	startedAt time.Time
}

func NewApplication(info health.ServiceInfo, db pinger) *Application {
	return &Application{
		info:      info,
		db:        db,
		startedAt: time.Now(),
	}
}

func (a *Application) Info() health.ServiceInfo {
	return a.info
}

func (a *Application) Metrics(ctx context.Context) (health.ServiceMetrics, error) {
	now := time.Now()

	status := health.ServiceStatus(health.ServiceStatusHealthy)
	if err := a.db.Ping(ctx); err != nil {
		status = health.ServiceStatus(health.ServiceStatusDegraded)
	}

	var memPercent float64
	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		memPercent = vm.UsedPercent
	}

	var cpuPercent float64
	if percents, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(percents) > 0 {
		cpuPercent = percents[0]
	}

	return health.ServiceMetrics{
		ServiceID:     a.info.ID,
		Status:        status,
		CpuPercent:    cpuPercent,
		MemoryPercent: memPercent,
		UptimeSeconds: int64(now.Sub(a.startedAt).Seconds()),
		ReportedAt:    now,
	}, nil
}
