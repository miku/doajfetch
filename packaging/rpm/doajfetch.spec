Summary:    Fetch DOAJ API data.
Name:       doajfetch
Version:    0.4.0
Release:    0
License:    GPL
BuildArch:  x86_64
BuildRoot:  %{_tmppath}/%{name}-build
Group:      System/Base
Vendor:     Leipzig University Library, https://www.ub.uni-leipzig.de
URL:        https://github.com/miku/doajfetch

%description

A simple key value store for JSON data.

%prep

%build

%pre

%install

mkdir -p $RPM_BUILD_ROOT/usr/local/sbin
install -m 755 doajfetch $RPM_BUILD_ROOT/usr/local/sbin

# mkdir -p $RPM_BUILD_ROOT/usr/local/share/man/man1
# install -m 644 microblob.1.gz $RPM_BUILD_ROOT/usr/local/share/man/man1/microblob.1.gz

%post

%clean
rm -rf $RPM_BUILD_ROOT
rm -rf %{_tmppath}/%{name}
rm -rf %{_topdir}/BUILD/%{name}

%files
%defattr(-,root,root)

/usr/local/sbin/doajfetch
# /usr/local/share/man/man1/microblob.1.gz

%changelog

* Tue Jul 03 2018 Martin Czygan
- 0.1.0 initial release
