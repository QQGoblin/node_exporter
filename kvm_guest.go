package main

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/prometheus/node_exporter/collector/utils/cgroup1"
	"github.com/prometheus/procfs"
)

const (
	defaultKVMGuestExecutable = "/usr/bin/qemu-system-x86_64"
)

var (
	guestUUIDRe = regexp.MustCompile(`-name\x00guest=([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})`)
)

type kvmGuestProcess struct {
	pid          int
	uuid         string
	rss          uint64 // 内存驻留的内存
	swap         uint64 // 以已经换出，且解除映射的内存
	swapCached   uint64 // 以已经换出，但未解除映射的内存
	vmPgMajFault uint64 // 缺页统计
}

type kvmGuestScanner struct {
	fs             procfs.FS
	logger         *slog.Logger
	executable     string
	executablePath func(procfs.Proc) (string, error)
	cmdLine        func(procfs.Proc) ([]string, error)
	status         func(procfs.Proc) (procfs.ProcStatus, error)
	sMaps          func(procfs.Proc) (procfs.ProcSMapsRollup, error)
}

func formatBytes(v uint64) string {
	const unit = 1024

	if v < unit {
		return fmt.Sprintf("%dB", v)
	}

	div, exp := uint64(unit), 0
	for n := v / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	return fmt.Sprintf("%.2f%s", float64(v)/float64(div), units[exp])
}

func newKVMGuestScanner() *kvmGuestScanner {

	fs, _ := procfs.NewDefaultFS()
	return &kvmGuestScanner{
		fs:             fs,
		executable:     defaultKVMGuestExecutable,
		executablePath: func(p procfs.Proc) (string, error) { return p.Executable() },
		cmdLine:        func(p procfs.Proc) ([]string, error) { return p.CmdLine() },
		status:         func(p procfs.Proc) (procfs.ProcStatus, error) { return p.NewStatus() },
		sMaps:          func(p procfs.Proc) (procfs.ProcSMapsRollup, error) { return p.ProcSMapsRollup() },
	}
}

func (c *kvmGuestScanner) Update() error {
	guests, err := c.discover()
	if err != nil {
		return err
	}
	if len(guests) == 0 {
		return nil
	}

	sort.Slice(guests, func(i, j int) bool {
		return guests[i].pid < guests[j].pid
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	fmt.Fprintln(w, "PID\tUUID\tRSS\tSwap\tSwapCached\tPgMajFault")
	for _, guest := range guests {
		fmt.Fprintf(
			w,
			"%d\t%s\t%s\t%s\t%s\t%d\n",
			guest.pid,
			guest.uuid,
			formatBytes(guest.rss),
			formatBytes(guest.swap),
			formatBytes(guest.swapCached),
			guest.vmPgMajFault,
		)
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush output: %w", err)
	}

	return nil
}

func (s *kvmGuestScanner) discover() ([]kvmGuestProcess, error) {
	procs, err := s.fs.AllProcs()
	if err != nil {
		return nil, fmt.Errorf("failed to list all processes: %w", err)
	}

	guests := make([]kvmGuestProcess, 0)
	for _, proc := range procs {
		executable, err := s.executablePath(proc)
		if err != nil {
			s.logger.Debug("failed to read executable path for process", "pid", proc.PID, "err", err)
			continue
		}
		if executable != s.executable {
			continue
		}

		cmdline, err := s.cmdLine(proc)
		if err != nil {
			s.logger.Debug("failed to read cmdline for process", "pid", proc.PID, "err", err)
			continue
		}

		matches := guestUUIDRe.FindStringSubmatch(strings.Join(cmdline, "\x00"))

		if matches == nil {
			s.logger.Debug("no guest UUID found in qemu cmdline", "pid", proc.PID)
			continue
		}

		stats, err := cgroup1.MemoryStatByPid(proc.PID)
		if err != nil {
			s.logger.Debug("failed to read memory status for process", "pid", proc.PID, "err", err)
			continue
		}

		guests = append(guests, kvmGuestProcess{
			pid:          proc.PID,
			uuid:         matches[1],
			rss:          stats.TotalRSS + stats.TotalCache,
			swap:         stats.TotalSwap,
			swapCached:   stats.TotalInactiveFile + stats.TotalActiveFile + stats.TotalInactiveAnon + stats.TotalActiveAnon + stats.TotalUnevictable,
			vmPgMajFault: stats.TotalPgMajFault,
		})
	}

	return guests, nil
}

func main() {

	scanner := newKVMGuestScanner()
	if err := scanner.Update(); err != nil {
		fmt.Printf("Scanner KVM Guest Error: %v\n", err)
	}
}
