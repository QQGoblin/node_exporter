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
	pid    int
	uuid   string
	vmSize uint64
	vmRSS  uint64
	vmSwap uint64
}

type kvmGuestScanner struct {
	fs             procfs.FS
	logger         *slog.Logger
	executable     string
	executablePath func(procfs.Proc) (string, error)
	cmdLine        func(procfs.Proc) ([]string, error)
	status         func(procfs.Proc) (procfs.ProcStatus, error)
}

type kvmGuestCollector struct {
	scanner *kvmGuestScanner
	vmSize  *prometheus.Desc
	vmRSS   *prometheus.Desc
	vmSwap  *prometheus.Desc
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
		vmSize: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "vm_size_bytes"),
			"Virtual memory size in bytes for a KVM guest process.",
			[]string{"pid", "uuid"}, nil,
		),
		vmRSS: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "vm_rss_bytes"),
			"Resident memory size in bytes for a KVM guest process.",
			[]string{"pid", "uuid"}, nil,
		),
		vmSwap: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "vm_swap_bytes"),
			"Swapped memory size in bytes for a KVM guest process.",
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
		ch <- prometheus.MustNewConstMetric(c.vmSize, prometheus.GaugeValue, float64(guest.vmSize), pid, guest.uuid)
		ch <- prometheus.MustNewConstMetric(c.vmRSS, prometheus.GaugeValue, float64(guest.vmRSS), pid, guest.uuid)
		ch <- prometheus.MustNewConstMetric(c.vmSwap, prometheus.GaugeValue, float64(guest.vmSwap), pid, guest.uuid)
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

		status, err := s.status(proc)
		if err != nil {
			s.logger.Debug("failed to read status for process", "pid", proc.PID, "err", err)
			continue
		}

		guests = append(guests, kvmGuestProcess{
			pid:    proc.PID,
			uuid:   matches[1],
			vmSize: status.VmSize,
			vmRSS:  status.VmRSS,
			vmSwap: status.VmSwap,
		})
	}

	return guests, nil
}
