#!/bin/sh
# 打包 OpenWrt 安装包：同时生成 opkg 用的 .ipk（OpenWrt 24.10 及以前）与 apk-tools v3 用的 .apk（OpenWrt 25.12 起）。
#
# 用法：deploy/openwrt/build.sh <版本> <Go 架构> <gately 二进制> [输出目录]
#   版本     发布标签，如 v0.5.4、v0.6.0-rc1
#   Go 架构  amd64 / arm64 / arm / mipsle / mips / riscv64
#   输出     gately_<版本>_openwrt_<Go 架构>.ipk 与 .apk
#
# 需要以 root 运行（包内文件属主写为 root），并提供 apk-tools v3 的 apk 命令。发布流程在 Alpine 容器中执行：
#   docker run --rm -v "$PWD:/w" -w /w alpine:3.24 deploy/openwrt/build.sh v0.5.4 arm64 dist/gately dist
#
# Gately 是静态链接的 Go 程序，不依赖 OpenWrt 的 libc 版本，因此包的架构写为通用的 all / noarch，
# 每个 Go 架构单独出包，安装前由脚本按 /etc/openwrt_release 中的 DISTRIB_ARCH 拦截装错架构的情况。
set -eu

if [ $# -lt 3 ]; then
	echo "用法: $0 <版本> <Go 架构> <gately 二进制> [输出目录]" >&2
	exit 1
fi
TAG=$1
GOARCH_NAME=$2
BIN=$3
OUT=${4:-dist}
HERE=$(cd "$(dirname "$0")" && pwd)

if [ "$(id -u)" != 0 ]; then
	echo "需要以 root 运行，包内文件的属主才是 root" >&2
	exit 1
fi
if ! apk --version 2>/dev/null | grep -q 'apk-tools 3'; then
	echo "需要 apk-tools v3（OpenWrt 25.12 的 apk 包格式），可在 alpine:3.23 及以上的容器中运行" >&2
	exit 1
fi
if [ ! -f "$BIN" ]; then
	echo "找不到二进制: $BIN" >&2
	exit 1
fi

# DISTRIB_ARCH 的匹配规则与架构说明。arm 包按 GOARM=7 编译，需要 ARMv7 且带硬件浮点
# （OpenWrt 架构名形如 arm_cortex-a7_neon-vfpv4），arm_cortex-a9 这类无 FPU 的设备无法运行
case "$GOARCH_NAME" in
amd64)
	ARCH_GLOB='x86_64'
	ARCH_DESC='x86_64'
	;;
arm64)
	ARCH_GLOB='aarch64_*'
	ARCH_DESC='aarch64_*'
	;;
arm)
	ARCH_GLOB='arm_cortex-a*_*'
	ARCH_DESC='arm_cortex-a*（ARMv7 且带 VFP/NEON）'
	;;
mipsle)
	ARCH_GLOB='mipsel_*'
	ARCH_DESC='mipsel_*'
	;;
mips)
	ARCH_GLOB='mips_*'
	ARCH_DESC='mips_*'
	;;
riscv64)
	ARCH_GLOB='riscv64_*'
	ARCH_DESC='riscv64_*'
	;;
*)
	echo "不支持的架构: $GOARCH_NAME" >&2
	exit 1
	;;
esac

# 版本号转换（v0.5.4 → 0.5.4-r1，预发布版本两种包管理器写法不同），与 build-luci.sh 共用
. "$HERE/pkgver.sh"
pkg_versions "$TAG"
NAME="gately_${TAG}_openwrt_${GOARCH_NAME}"

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$OUT"
OUT=$(cd "$OUT" && pwd)

# ---- 安装内容 ----
ROOT="$WORK/root"
install -d -m 0755 "$ROOT/usr/bin" "$ROOT/etc/init.d" "$ROOT/lib/upgrade/keep.d"
install -m 0755 "$BIN" "$ROOT/usr/bin/gately"
install -m 0755 "$HERE/gately.init" "$ROOT/etc/init.d/gately"
# 刷写固件（sysupgrade）时保留配置目录
echo "/etc/gately/" >"$ROOT/lib/upgrade/keep.d/gately"
chmod 0644 "$ROOT/lib/upgrade/keep.d/gately"
chown -R 0:0 "$ROOT"
SIZE=$(du -sk "$ROOT" | cut -f1)

# ---- 维护脚本 ----
# 打进固件（ImageBuilder 设置 IPKG_INSTROOT）时只建立开机自启链接，不在构建机上启动服务
SCRIPTS="$WORK/scripts"
mkdir -p "$SCRIPTS"

# 架构校验：设备架构与安装包不一致时返回失败。没有 /etc/openwrt_release 时（非 OpenWrt）不拦截
cat >"$SCRIPTS/arch_check" <<EOF
arch_ok() {
	arch=\$( . "\${IPKG_INSTROOT}/etc/openwrt_release" 2>/dev/null; echo "\$DISTRIB_ARCH" )
	case "\$arch" in
	""|$ARCH_GLOB)
		return 0
		;;
	esac
	echo "gately: 此安装包适用于 $ARCH_DESC 架构，本机为 \$arch，请下载对应架构的安装包" >&2
	return 1
}
EOF

{
	echo '#!/bin/sh'
	cat "$SCRIPTS/arch_check"
	echo '# 安装前确认设备架构与安装包一致，避免装上无法运行的程序'
	echo 'arch_ok || exit 1'
} >"$SCRIPTS/preinst"

{
	echo '#!/bin/sh'
	cat "$SCRIPTS/arch_check"
	cat <<'EOF'
# OpenWrt 25.12 的 apk 在 pre-install 失败时仍会继续安装，这里再校验一次，架构不符时不启用、不启动
if ! arch_ok; then
	echo "gately: 已跳过启动，请执行 apk del gately 移除后安装正确架构的安装包" >&2
	exit 1
fi
# UCI 配置 /etc/config/gately 只在缺失时创建，不随包分发，升级时不会覆盖用户在 LuCI 中的设置
if [ ! -f "${IPKG_INSTROOT}/etc/config/gately" ]; then
	mkdir -p "${IPKG_INSTROOT}/etc/config"
	printf "\nconfig gately 'main'\n" >"${IPKG_INSTROOT}/etc/config/gately"
fi
if [ -n "${IPKG_INSTROOT}" ]; then
	mkdir -p "${IPKG_INSTROOT}/etc/rc.d"
	ln -sf ../init.d/gately "${IPKG_INSTROOT}/etc/rc.d/S99gately"
	ln -sf ../init.d/gately "${IPKG_INSTROOT}/etc/rc.d/K10gately"
	exit 0
fi
# 正在运行（升级或覆盖手动部署的程序）时重启以换上新程序，否则直接启动
start_or_restart() {
	if /etc/init.d/gately running; then
		/etc/init.d/gately restart
	else
		/etc/init.d/gately start
	fi
}
# 升级时（opkg 设置 PKG_UPGRADE=1）保留用户的开机自启选择：已停用且未运行的服务不重新启用、不启动
if [ "${PKG_UPGRADE}" = 1 ]; then
	if /etc/init.d/gately running || /etc/init.d/gately enabled; then
		start_or_restart
	fi
	exit 0
fi
/etc/init.d/gately enable
start_or_restart
echo "gately: 已启动，管理界面 http://<路由器地址>:16680，默认账号 admin，默认密码 gately.cn；安装 luci-app-gately 后可在 LuCI「服务 → Gately」中管理"
exit 0
EOF
} >"$SCRIPTS/postinst"

cat >"$SCRIPTS/prerm" <<'EOF'
#!/bin/sh
if [ -n "${IPKG_INSTROOT}" ]; then
	rm -f "${IPKG_INSTROOT}/etc/rc.d/S99gately" "${IPKG_INSTROOT}/etc/rc.d/K10gately"
	exit 0
fi
/etc/init.d/gately running && /etc/init.d/gately stop
# 升级时不取消开机自启；卸载时一并取消。配置目录 /etc/gately 保留，需要时手动删除
[ "${PKG_UPGRADE}" = 1 ] || /etc/init.d/gately disable
exit 0
EOF
chmod 0755 "$SCRIPTS"/*

DESC='Open-source network gateway: port forwarding, reverse proxy, DDNS, ACME certificates, WAF and more. Web UI on port 16680; config in /etc/gately.'
URL='https://github.com/wangwanjie/gately'
MAINT='Gately <https://github.com/wangwanjie/gately>'

# ---- ipk：外层 tar.gz 包含 debian-binary、data.tar.gz、control.tar.gz（与 OpenWrt 的 ipkg-build 一致） ----
IPK="$WORK/ipk"
mkdir -p "$IPK/control"
cat >"$IPK/control/control" <<EOF
Package: gately
Version: $IPK_VER
Depends: ca-bundle
License: Apache-2.0
Section: net
URL: $URL
Maintainer: $MAINT
Architecture: all
Installed-Size: $((SIZE * 1024))
Description: $DESC
EOF
cp "$SCRIPTS/preinst" "$SCRIPTS/postinst" "$SCRIPTS/prerm" "$IPK/control/"
chown -R 0:0 "$IPK/control"
echo "2.0" >"$IPK/debian-binary"
(cd "$ROOT" && tar -czf "$IPK/data.tar.gz" .)
(cd "$IPK/control" && tar -czf "$IPK/control.tar.gz" .)
(cd "$IPK" && tar -czf "$OUT/$NAME.ipk" ./debian-binary ./data.tar.gz ./control.tar.gz)

# ---- apk：apk-tools v3 的 ADB 格式 ----
# apk 不设置 PKG_UPGRADE，升级与安装分别调用 post-upgrade 与 post-install，升级脚本里显式标记为升级
APKS="$WORK/apk"
mkdir -p "$APKS"
cp "$SCRIPTS/preinst" "$APKS/pre-install"
cp "$SCRIPTS/preinst" "$APKS/pre-upgrade"
cp "$SCRIPTS/postinst" "$APKS/post-install"
{
	echo '#!/bin/sh'
	echo 'PKG_UPGRADE=1'
	tail -n +2 "$SCRIPTS/postinst"
} >"$APKS/post-upgrade"
cp "$SCRIPTS/prerm" "$APKS/pre-deinstall"
chmod 0755 "$APKS"/*
apk mkpkg \
	--info "name:gately" \
	--info "version:$APK_VER" \
	--info "description:$DESC" \
	--info "arch:noarch" \
	--info "license:Apache-2.0" \
	--info "origin:gately" \
	--info "url:$URL" \
	--info "maintainer:$MAINT" \
	--info "depends:ca-bundle" \
	--script "pre-install:$APKS/pre-install" \
	--script "pre-upgrade:$APKS/pre-upgrade" \
	--script "post-install:$APKS/post-install" \
	--script "post-upgrade:$APKS/post-upgrade" \
	--script "pre-deinstall:$APKS/pre-deinstall" \
	--files "$ROOT" \
	--output "$OUT/$NAME.apk"

echo "已生成 $OUT/$NAME.ipk 与 $OUT/$NAME.apk"
