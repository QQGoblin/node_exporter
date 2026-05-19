# Host Monitor

基于 node-exporter 进行了以下修订：

- 添加 local_kvm_guest_linux.go 用于监控 kvm 虚拟机的进程使用情况
- 添加 local_pressure_stat_linux.go 用于监控 /proc/pressure/stat 中的指标
- TODO： 修订 diskstats 过滤所有 iscsi 设备