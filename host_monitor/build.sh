#!/bin/bash

set -e

echo "build node-exporter"

go mod tidy
CGO_ENABLED=0 go build -o ./host_monitor/rpm/bin/node-exporter node-exporter.go

PACKAGE_NAME="host-monitor"
VERSION="1.0.0"
BUILD_DIR="$(pwd)/rpmbuild"

RPMARCH=$([ "$GOARCH" = "arm64" ] && echo "aarch64" || echo "x86_64")
echo "build HMON $RPMARCH RPM..."

rm -rf rpmbuild
mkdir -p "$BUILD_DIR"/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
mkdir -p "$BUILD_DIR/SOURCES/$PACKAGE_NAME-$VERSION"

# build Source0
rm -rf "$PACKAGE_NAME-$VERSION.tar.gz"
cp -r host_monitor/rpm/* "$BUILD_DIR/SOURCES/$PACKAGE_NAME-$VERSION/"
cd "$BUILD_DIR/SOURCES"
tar -czf "$PACKAGE_NAME-$VERSION.tar.gz" "$PACKAGE_NAME-$VERSION/"
rm -rf "$PACKAGE_NAME-$VERSION"

cp "$OLDPWD/rpm/rpm.spec" "$BUILD_DIR/SPECS/host-monitor.spec"

rpmbuild -ba \
  --define "_topdir $BUILD_DIR" \
  --define "_sourcedir $BUILD_DIR/SOURCES" \
  --define "_builddir $BUILD_DIR/BUILD" \
  --define "_rpmdir $BUILD_DIR/RPMS" \
  --define "_srcrpmdir $BUILD_DIR/SRPMS" \
  --define "_specdir $BUILD_DIR/SPECS" \
  --define "_version $VERSION" \
  --target=$RPMARCH \
  "$BUILD_DIR/SPECS/host-monitor.spec"

# 显示结果
echo "Success"