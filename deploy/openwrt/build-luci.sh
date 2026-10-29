#!/bin/sh
# 打包 LuCI 插件 luci-app-gately：在 LuCI「服务 → Gately」中查看运行状态、启停服务、设置开机自启与管理界面端口，
# 并一键打开 Gately 管理界面。插件只含脚本与静态文件，与架构无关，每个版本只出一份 .ipk 与 .apk。
#
# 用法：deploy/openwrt/build-luci.sh <版本> [输出目录]
#   输出 luci-app-gately_<版本>_openwrt_all.ipk 与 .apk
#
# 与 build.sh 相同，需要以 root 运行并提供 apk-tools v3 的 apk 命令：
#   docker run --rm -v "$PWD:/w" -w /w alpine:3.24 deploy/openwrt/build-luci.sh v0.6.1 dist
#
# 翻译源文件在 luci/po，修改后执行 go run ./deploy/openwrt/luci/po2lmo 重新生成 luci/root 下的 .lmo。
set -eu

if [ $# -lt 1 ]; then
	echo "用法: $0 <版本> [输出目录]" >&2
	exit 1
fi
TAG=$1
OUT=${2:-dist}
HERE=$(cd "$(dirname "$0")" && pwd)

if [ "$(id -u)" != 0 ]; then
	echo "需要以 root 运行，包内文件的属主才是 root" >&2
	exit 1
fi
if ! apk --version 2>/dev/null | grep -q 'apk-tools 3'; then
	echo "需要 apk-tools v3（OpenWrt 25.12 的 apk 包格式），可在 alpine:3.23 及以上的容器中运行" >&2
	exit 1
fi

. "$HERE/pkgver.sh"
pkg_versions "$TAG"
NAME="luci-app-gately_${TAG}_openwrt_all"

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$OUT"
OUT=$(cd "$OUT" && pwd)

# ---- 安装内容 ----
ROOT="$WORK/root"
mkdir -p "$ROOT"
cp -R "$HERE/luci/root/." "$ROOT/"
find "$ROOT" -type d -exec chmod 0755 {} +
find "$ROOT" -type f -exec chmod 0644 {} +
chmod 0755 "$ROOT/usr/libexec/rpcd/luci.gately"
chown -R 0:0 "$ROOT"
SIZE=$(du -sk "$ROOT" | cut -f1)

# ---- 维护脚本 ----
# 与 LuCI 官方插件相同：安装后清除菜单与模块缓存，并让 rpcd 重新加载（SIGHUP 会保留现有登录会话），新菜单与权限立即生效
SCRIPTS="$WORK/scripts"
mkdir -p "$SCRIPTS"
cat >"$SCRIPTS/postinst" <<'EOF'
#!/bin/sh
# 旧版 gately 安装包不会创建 UCI 配置，LuCI 页面需要它，缺失时在此补建
if [ ! -f "${IPKG_INSTROOT}/etc/config/gately" ]; then
	mkdir -p "${IPKG_INSTROOT}/etc/config"
	printf "\nconfig gately 'main'\n" >"${IPKG_INSTROOT}/etc/config/gately"
fi
[ -n "${IPKG_INSTROOT}" ] && exit 0
rm -f /tmp/luci-indexcache /tmp/luci-indexcache.*
rm -rf /tmp/luci-modulecache/
killall -HUP rpcd 2>/dev/null
exit 0
EOF
cat >"$SCRIPTS/postrm" <<'EOF'
#!/bin/sh
[ -n "${IPKG_INSTROOT}" ] && exit 0
rm -f /tmp/luci-indexcache /tmp/luci-indexcache.* /tmp/luci-gately-version
rm -rf /tmp/luci-modulecache/
killall -HUP rpcd 2>/dev/null
exit 0
EOF
chmod 0755 "$SCRIPTS"/*

DESC='LuCI support for Gately: service status, start / stop, start on boot, admin port and a shortcut to the Gately admin interface.'
URL='https://github.com/wangwanjie/gately'
MAINT='Gately <https://github.com/wangwanjie/gately>'

# ---- ipk ----
IPK="$WORK/ipk"
mkdir -p "$IPK/control"
cat >"$IPK/control/control" <<EOF
Package: luci-app-gately
Version: $IPK_VER
Depends: gately (>= $IPK_VER), luci-base
License: Apache-2.0
Section: luci
URL: $URL
Maintainer: $MAINT
Architecture: all
Installed-Size: $((SIZE * 1024))
Description: $DESC
EOF
cp "$SCRIPTS/postinst" "$SCRIPTS/postrm" "$IPK/control/"
chown -R 0:0 "$IPK/control"
echo "2.0" >"$IPK/debian-binary"
(cd "$ROOT" && tar -czf "$IPK/data.tar.gz" .)
(cd "$IPK/control" && tar -czf "$IPK/control.tar.gz" .)
(cd "$IPK" && tar -czf "$OUT/$NAME.ipk" ./debian-binary ./data.tar.gz ./control.tar.gz)

# ---- apk ----
apk mkpkg \
	--info "name:luci-app-gately" \
	--info "version:$APK_VER" \
	--info "description:$DESC" \
	--info "arch:noarch" \
	--info "license:Apache-2.0" \
	--info "origin:gately" \
	--info "url:$URL" \
	--info "maintainer:$MAINT" \
	--info "depends:gately>=$APK_VER luci-base" \
	--script "post-install:$SCRIPTS/postinst" \
	--script "post-upgrade:$SCRIPTS/postinst" \
	--script "post-deinstall:$SCRIPTS/postrm" \
	--files "$ROOT" \
	--output "$OUT/$NAME.apk"

echo "已生成 $OUT/$NAME.ipk 与 $OUT/$NAME.apk"
