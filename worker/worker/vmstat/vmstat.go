package vmstat

import (
	"context"
	"fmt"
	"github.com/TUM-Dev/gocast/tools/pathprovider"
	"github.com/icza/gox/fmtx"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"golang.org/x/sync/errgroup"
	"time"
)

type VmStat struct {
	diskPath string

	CpuPercent float64

	MemTotal     uint64
	MemAvailable uint64

	DiskTotal   uint64
	DiskPercent float64
	DiskUsed    uint64
}

var (
	ErrCPUInvalid = fmt.Errorf("len (cpu.Percent) != 1")
)

func New() *VmStat {
	return &VmStat{diskPath: pathprovider.Root()}
}

func NewWithPath(path string) *VmStat {
	return &VmStat{diskPath: path}
}

func (s *VmStat) Update() error {
	g, _ := errgroup.WithContext(context.Background())
	g.Go(s.getCpu)
	g.Go(s.getMem)
	g.Go(s.getDisk)
	return g.Wait()
}

func (s *VmStat) getMem() error {
	memory, err := mem.VirtualMemory()
	if err != nil {
		return err
	}
	s.MemAvailable = memory.Available
	s.MemTotal = memory.Total
	return nil
}

func (s *VmStat) getCpu() error {
	percent, err := cpu.Percent(time.Second*5, false)
	if err != nil {
		return err
	}
	if len(percent) != 1 {
		return ErrCPUInvalid
	}
	s.CpuPercent = percent[0]
	return nil
}

func (s *VmStat) getDisk() error {
	usage, err := disk.Usage(pathprovider.Root())
	if err != nil {
		return err
	}
	s.DiskUsed = usage.Used
	s.DiskTotal = usage.Total
	s.DiskPercent = usage.UsedPercent
	return nil
}

func (s *VmStat) GetCpuStr() string {
	return fmt.Sprintf("%.f%%", s.CpuPercent)
}

func (s *VmStat) GetMemStr() string {
	if s.MemAvailable == 0 || s.MemTotal == 0 {
		return "unknown"
	}
	return fmt.Sprintf("%sM/%sM (%.f%%)",
		fmtx.FormatInt(int64(s.MemAvailable/1e+6), 3, '.'),
		fmtx.FormatInt(int64(s.MemTotal/1e+6), 3, '.'),
		(1-(float64(s.MemAvailable)/float64(s.MemTotal)))*100)
}

func (s *VmStat) GetDiskStr() string {
	return fmt.Sprintf("%dG/%dG (%.f%%)", s.DiskUsed/1e+9, s.DiskTotal/1e+9, s.DiskPercent)
}

func (s *VmStat) String() string {
	return fmt.Sprintf("CPU: [%s], Mem: [%s], Disk: [%s]",
		s.GetCpuStr(), s.GetMemStr(), s.GetDiskStr())
}
