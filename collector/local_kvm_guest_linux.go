// Copyright 2026 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// LOCAL CUSTOMIZATION NOTICE:
// This collector is a project-local addition for this repository and is not part
// of upstream node_exporter. It ports the extmon KVM guest memory monitoring
// logic from /home/lqing/Develop/projects/RG/hmon/extmon.

//go:build !nokvm_guest && linux

package collector

import (
	"fmt"
	"github.com/prometheus/node_exporter/collector/utils/cgroup1"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/procfs"
)

const defaultKVMGuestExecutable = "/usr/bin/qemu-system-x86_64"

var (
	guestUUIDRe = regexp.MustCompile(`-name\x00guest=([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})`)
)

type kvmGuestProcess struct {
	pid          int
	uuid         string
	rss          uint64
	cache        uint64
	swap         uint64
	swapCached   uint64
	inactiveFile uint64
	activeFile   uint64
	inactiveAnon uint64
	activeAnon   uint64
	unevictable  uint64
	pgMajFault   uint64
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

type kvmGuestCollector struct {
	scanner      *kvmGuestScanner
	rss          *prometheus.Desc
	cache        *prometheus.Desc
	swap         *prometheus.Desc
	swapCached   *prometheus.Desc
	inactiveFile *prometheus.Desc
	activeFile   *prometheus.Desc
	inactiveAnon *prometheus.Desc
	activeAnon   *prometheus.Desc
	unevictable  *prometheus.Desc
	pgMajFault   *prometheus.Desc
}

func init() {
	registerCollector("kvm_guest", defaultDisabled, NewKVMGuestCollector)
}

// NewKVMGuestCollector returns a Collector exposing per-guest KVM process memory usage.
func NewKVMGuestCollector(logger *slog.Logger) (Collector, error) {
	fs, err := procfs.NewFS(*procPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open procfs: %w", err)
	}

	subsystem := "kvm_guest"
	return &kvmGuestCollector{
		scanner: newKVMGuestScanner(fs, logger),
		rss: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "guest_rss_bytes"),
			"Rss memory size in bytes for a KVM guest cgroup1 memory stats.",
			[]string{"pid", "uuid"}, nil,
		),
		cache: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "guest_cache_bytes"),
			"Cache memory size in bytes for a KVM guest cgroup1 memory stats.",
			[]string{"pid", "uuid"}, nil,
		),
		swap: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "guest_swap_bytes"),
			"Swapped memory size in bytes for a KVM guest cgroup1 memory stats.",
			[]string{"pid", "uuid"}, nil,
		),
		swapCached: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "guest_swapCached_bytes"),
			"SwapCached memory size in bytes for a KVM guest cgroup1 memory stats.",
			[]string{"pid", "uuid"}, nil,
		),
		inactiveFile: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "guest_inactiveFile_bytes"),
			"InactiveFile memory size in bytes for a KVM guest cgroup1 memory stats.",
			[]string{"pid", "uuid"}, nil,
		),
		activeFile: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "guest_activeFile_bytes"),
			"ActiveFile memory size in bytes for a KVM guest cgroup1 memory stats.",
			[]string{"pid", "uuid"}, nil,
		),
		inactiveAnon: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "guest_inactiveAnon_bytes"),
			"InactiveAnon memory size in bytes for a KVM guest cgroup1 memory stats.",
			[]string{"pid", "uuid"}, nil,
		),
		activeAnon: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "guest_activeAnon_bytes"),
			"ActiveAnon memory size in bytes for a KVM guest cgroup1 memory stats.",
			[]string{"pid", "uuid"}, nil,
		),
		unevictable: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "guest_unevictable_bytes"),
			"Unevictable memory size in bytes for a KVM guest cgroup1 memory stats.",
			[]string{"pid", "uuid"}, nil,
		),
		pgMajFault: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "guest_pgMajFault_total"),
			"PgMajFault memory size in bytes for a KVM guest cgroup1 memory stats.",
			[]string{"pid", "uuid"}, nil,
		),
	}, nil
}

func newKVMGuestScanner(fs procfs.FS, logger *slog.Logger) *kvmGuestScanner {
	return &kvmGuestScanner{
		fs:             fs,
		logger:         logger,
		executable:     defaultKVMGuestExecutable,
		executablePath: func(p procfs.Proc) (string, error) { return p.Executable() },
		cmdLine:        func(p procfs.Proc) ([]string, error) { return p.CmdLine() },
		status:         func(p procfs.Proc) (procfs.ProcStatus, error) { return p.NewStatus() },
		sMaps:          func(p procfs.Proc) (procfs.ProcSMapsRollup, error) { return p.ProcSMapsRollup() },
	}
}

func (c *kvmGuestCollector) Update(ch chan<- prometheus.Metric) error {
	guests, err := c.scanner.discover()
	if err != nil {
		return err
	}
	if len(guests) == 0 {
		return ErrNoData
	}

	for _, guest := range guests {
		pid := strconv.Itoa(guest.pid)
		ch <- prometheus.MustNewConstMetric(c.rss, prometheus.GaugeValue, float64(guest.rss), pid, guest.uuid)
		ch <- prometheus.MustNewConstMetric(c.cache, prometheus.GaugeValue, float64(guest.cache), pid, guest.uuid)
		ch <- prometheus.MustNewConstMetric(c.swap, prometheus.GaugeValue, float64(guest.swap), pid, guest.uuid)
		ch <- prometheus.MustNewConstMetric(c.swapCached, prometheus.GaugeValue, float64(guest.swapCached), pid, guest.uuid)
		ch <- prometheus.MustNewConstMetric(c.inactiveAnon, prometheus.GaugeValue, float64(guest.inactiveAnon), pid, guest.uuid)
		ch <- prometheus.MustNewConstMetric(c.activeAnon, prometheus.GaugeValue, float64(guest.activeAnon), pid, guest.uuid)
		ch <- prometheus.MustNewConstMetric(c.inactiveFile, prometheus.GaugeValue, float64(guest.inactiveFile), pid, guest.uuid)
		ch <- prometheus.MustNewConstMetric(c.activeFile, prometheus.GaugeValue, float64(guest.activeFile), pid, guest.uuid)
		ch <- prometheus.MustNewConstMetric(c.unevictable, prometheus.GaugeValue, float64(guest.unevictable), pid, guest.uuid)
		ch <- prometheus.MustNewConstMetric(c.pgMajFault, prometheus.GaugeValue, float64(guest.pgMajFault), pid, guest.uuid)
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

		status, err := cgroup1.MemoryStatByPid(proc.PID)
		if err != nil {
			s.logger.Debug("failed to read smaps for process", "pid", proc.PID, "err", err)
			continue
		}

		guests = append(guests, kvmGuestProcess{
			pid:          proc.PID,
			uuid:         matches[1],
			rss:          status.TotalRSS,
			cache:        status.TotalCache,
			swap:         status.TotalSwap,
			swapCached:   status.TotalSwapCached,
			inactiveFile: status.TotalInactiveFile,
			activeFile:   status.TotalActiveFile,
			inactiveAnon: status.TotalInactiveAnon,
			activeAnon:   status.TotalActiveAnon,
			unevictable:  status.TotalUnevictable,
			pgMajFault:   status.TotalPgMajFault,
		})
	}

	return guests, nil
}
