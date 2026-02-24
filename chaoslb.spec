Name:           chaoslb
Version:        %{version}
Release:        1%{?dist}
Summary:        Chaos Load Balancer - IPVS Manager with Prometheus Metrics

License:        MIT
URL:            https://github.com/cylonchau/chaosLB
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  systemd-rpm-macros
Requires:       ipvsadm
Requires:       iptables
Requires:       systemd

%description
Chaos Load Balancer is a high-performance load balancer management tool that provides:
- IPVS virtual service configuration
- Real server management
- SNAT rules handling
- Prometheus metrics export
- Health checking

%prep
%autosetup

%build
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64
go mod download
mkdir -p target
go build -ldflags="-w -s -X main.version=%{version}" -o target/chaoslb ./cmd/chaoslb
upx --best target/chaoslb

%install
# Create directories
install -d %{buildroot}%{_bindir}
install -d %{buildroot}%{_sysconfdir}/chaoslb
install -d %{buildroot}%{_unitdir}
install -d %{buildroot}%{_var}/log/chaoslb
install -d %{buildroot}%{_docdir}/%{name}

# Install binary without stripping
install -m 755 target/chaoslb %{buildroot}%{_bindir}/chaoslb

# Install configuration
install -m 644 config.json %{buildroot}%{_sysconfdir}/chaoslb/config.json

# Install systemd service
install -m 644 chaoslb.service %{buildroot}%{_unitdir}/chaoslb.service

# Install documentation
install -m 644 README.md %{buildroot}%{_docdir}/%{name}/README.md
install -m 644 LICENSE %{buildroot}%{_docdir}/%{name}/LICENSE

%files
%{_bindir}/chaoslb
%config(noreplace) %{_sysconfdir}/chaoslb/config.json
%{_unitdir}/chaoslb.service
%dir %{_var}/log/chaoslb
%doc %{_docdir}/%{name}/README.md
%license %{_docdir}/%{name}/LICENSE

%pre
getent group chaoslb >/dev/null || groupadd -r chaoslb
getent passwd chaoslb >/dev/null || \
    useradd -r -g chaoslb -d /var/lib/chaoslb -s /sbin/nologin \
    -c "Chaos Load Balancer service account" chaoslb

%post
%systemd_post chaoslb.service

%preun
%systemd_preun chaoslb.service

%postun
%systemd_postun_with_restart chaoslb.service

%changelog
* Mon Sep 15 2025 Your Name <cylonchau@outlook.com> - 0.0.1-1
- Initial package
- IPVS management with Prometheus metrics
- Support for multiple backends
- Health checking capabilities
- SNAT rules management
