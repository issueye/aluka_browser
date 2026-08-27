#!/usr/bin/env bash
#
# gio-browser 一键构建脚本（bash 版）
# 目标平台固定为 Windows（WebView2 专属应用）。
#
# 用法示例：
#   ./build.sh                 # 发布构建（默认）：隐藏控制台窗口
#   ./build.sh --dev           # 开发构建：保留控制台窗口以查看日志输出
#   ./build.sh --skip-test     # 跳过单元测试
#   ./build.sh --test-only     # 只运行测试，不产出可执行文件
#   ./build.sh --clean         # 清理构建产物后退出
#
# 环境变量：
#   OUT_DIR                    输出目录（默认 build）

set -euo pipefail
cd "$(dirname "$0")"

BIN_NAME="gio-browser.exe" # 程序为 Windows 专属，产物固定 .exe
OUT_DIR="${OUT_DIR:-build}"
BUILD_MODE="release"
RUN_TEST=1
TEST_ONLY=0
DO_CLEAN=0

log()  { printf '\033[1;34m[build]\033[0m %s\n' "$*"; }
ok()   { printf '\033[1;32m[OK]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[WARN]\033[0m %s\n' "$*"; }
err()  { printf '\033[1;31m[错误]\033[0m %s\n' "$*" >&2; }

usage() {
	sed -n '3,14p' "$0" | sed 's/^# \{0,1\}//'
}

# ---- 参数解析 ----
while [ $# -gt 0 ]; do
	case "$1" in
	--dev) BUILD_MODE="dev" ;;
	--skip-test) RUN_TEST=0 ;;
	--test-only) TEST_ONLY=1 ;;
	--clean) DO_CLEAN=1 ;;
	-h | --help) usage; exit 0 ;;
	*) err "未知参数: $1"; usage >&2; exit 1 ;;
	esac
	shift
done

if [ "$DO_CLEAN" = 1 ]; then
	if [ -d "$OUT_DIR" ]; then
		rm -rf "$OUT_DIR"
		ok "已清理构建目录: $OUT_DIR"
	else
		log "构建目录不存在，无需清理: $OUT_DIR"
	fi
	exit 0
fi

# ---- 环境检查 ----
command -v go >/dev/null 2>&1 || { err "未找到 go 命令，请先安装 Go 1.25+：https://go.dev/dl/"; exit 1; }
log "$(go version)"

GO_REQUIRED="$(awk '/^go /{print $2}' go.mod)"
log "go.mod 要求 Go >= ${GO_REQUIRED%.*}"

# 主机平台判定：非 Windows 主机走交叉编译路径
CROSS=0
HOST_OS="$(go env GOOS)"
if [ "$HOST_OS" != "windows" ]; then
	CROSS=1
	warn "当前主机系统为 $HOST_OS，将以 GOOS=windows 交叉编译（CGO_ENABLED=0）。"
	warn "交叉编译模式下自动跳过 go test / go vet（产物无法在本机运行）。"
fi

# 未格式化文件提醒（不阻断构建）
UNFMT="$(gofmt -l . || true)"
if [ -n "$UNFMT" ]; then
	warn "以下文件未通过 gofmt 格式化（不影响本次构建）："
	printf '%s\n' "$UNFMT" | sed 's/^/    /'
fi

# ---- 单元测试 ----
run_tests() {
	log "运行单元测试: go test ./..."
	if ! go test ./...; then
		err "测试失败，已中止构建。修复后再试，或使用 --skip-test 强行跳过。"
		exit 1
	fi
	ok "全部测试通过"
}

if [ "$TEST_ONLY" = 1 ]; then
	run_tests
	ok "--test-only 完成。"
	exit 0
fi

if [ "$RUN_TEST" = 1 ]; then
	run_tests
else
	warn "已按要求跳过单元测试。"
fi

# ---- 构建 ----
mkdir -p "$OUT_DIR"
if [ "$CROSS" = 1 ]; then
	export GOOS=windows CGO_ENABLED=0
fi

LDFLAGS="-s -w"
if [ "$BUILD_MODE" = "release" ]; then
	# -H windowsgui 隐藏控制台黑窗（代价：看不到 log 输出）
	LDFLAGS="$LDFLAGS -H windowsgui"
fi

log "开始构建（模式=$BUILD_MODE, 输出=$OUT_DIR/$BIN_NAME）..."
go build -trimpath -ldflags "$LDFLAGS" -o "$OUT_DIR/$BIN_NAME" .

SIZE="$(du -h "$OUT_DIR/$BIN_NAME" | cut -f1)"
ok "构建完成: $OUT_DIR/$BIN_NAME ($SIZE)"

if [ "$BUILD_MODE" = "release" ]; then
	log "提示：发布版已隐藏控制台窗口；如需查看运行日志请用 --dev 重新构建。"
fi
log "运行需要 Microsoft Edge WebView2 Runtime，用户数据位于 %APPDATA%\\gio-browser\\。"
