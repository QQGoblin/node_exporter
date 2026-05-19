Name:           host-monitor
Version:        %{_version}
Release:        1.%{lua:print(os.date("%Y%m%d.%H%M"))}%{?dist}
Summary:        host monitoring package including node_exporter

Group:          RCOS/Tools
License:        Apache License 2.0
URL:            http://gitlab.rcd.ruijie.net/linqing/hmon
Source0:        %{name}-%{version}.tar.gz
BuildRoot:      %{_tmppath}/%{name}-%{version}-%{release}-root-%(%{__id_u} -n)

BuildArch:      %{_target_cpu}
Requires:       systemd

%global debug_package %{nil}
%global __strip /bin/true

%description
HMON is a monitoring system package that includes node_exporter for collecting system metrics and process_exporter for monitoring specific processes.

%prep
%setup -q

%build

%install

rm -rf $RPM_BUILD_ROOT
mkdir -p $RPM_BUILD_ROOT/etc/sysconfig
mkdir -p $RPM_BUILD_ROOT/usr/lib/systemd/system
mkdir -p $RPM_BUILD_ROOT/usr/local/bin
mkdir -p $RPM_BUILD_ROOT/etc/rcos_global/port_whitelist

install -m 755 bin/node_exporter                    $RPM_BUILD_ROOT/usr/local/bin/node_exporter
install -m 644 host_monitor.service                 $RPM_BUILD_ROOT/usr/lib/systemd/system/host_monitor.service
install -m 644 host_monitor.socket                  $RPM_BUILD_ROOT/usr/lib/systemd/system/host_monitor.socket
install -m 644 config/host_monitor.port             $RPM_BUILD_ROOT/etc/rcos_global/port_whitelist/host_monitor.port

%clean
rm -rf $RPM_BUILD_ROOT

%files
%defattr(-,root,root,-)
/usr/local/bin/node_exporter
/usr/lib/systemd/system/host_monitor.service
/usr/lib/systemd/system/host_monitor.socket
/etc/rcos_global/port_whitelist/host_monitor.port
%dir /usr/local/bin/
%dir /etc/rcos_global/port_whitelist/

%post
systemctl daemon-reload

systemctl enable host_monitor.socket
systemctl enable host_monitor.service

%preun
if [ $1 -eq 0 ] ; then
    systemctl stop host_monitor.socket >/dev/null 2>&1 || true
    systemctl stop host_monitor.service >/dev/null 2>&1  || true
    
    systemctl disable host_monitor.socket >/dev/null 2>&1 || true
    systemctl disable host_monitor.service >/dev/null 2>&1  || true
fi

%postun
systemctl daemon-reload
