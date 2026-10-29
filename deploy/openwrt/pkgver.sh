# 由 build.sh 与 build-luci.sh 引用：把发布标签转换为安装包版本号，设置 APK_VER 与 IPK_VER。
#
# v0.5.4 → 0.5.4-r1。预发布版本需排在正式版之前，两种包管理器写法不同：
# apk 写作 0.6.0_rc1-r1，opkg 不认识 _rc 后缀（会排在正式版之后），改用 dpkg 风格的 0.6.0~rc1-r1
pkg_versions() {
	VER=${1#v}
	# apk 只认 alpha / beta / pre / rc 等预发布后缀，其他写法在打包前报错
	if ! echo "$VER" | grep -Eq '^[0-9]+(\.[0-9]+)*(-(alpha|beta|pre|rc)[0-9]*)?$'; then
		echo "版本号格式不支持: $1（应形如 v0.5.4 或 v0.6.0-rc1）" >&2
		exit 1
	fi
	APK_VER="$VER-r1"
	IPK_VER="$VER-r1"
	case "$VER" in
	*-*)
		APK_VER="${VER%%-*}_${VER#*-}-r1"
		IPK_VER="${VER%%-*}~${VER#*-}-r1"
		;;
	esac
}
