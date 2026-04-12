#!/usr/bin/env bash
set -euo pipefail

# build-centos7.sh
# 编译适用于 CentOS 7 的静态二进制文件并打包分发
#
# 用法:
#   ./scripts/build-centos7.sh              # 本地交叉编译
#   ./scripts/build-centos7.sh --docker     # 在 CentOS 7 Docker 容器中编译

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
DIST_DIR="${PROJECT_DIR}/dist"
BUILD_NAME="dict_new_centos7"
PACKAGE_DIR="${DIST_DIR}/${BUILD_NAME}"
BINARY_NAME="dict_new"

cd "${PROJECT_DIR}"

# 检查 Go 是否安装
command -v go >/dev/null 2>&1 || {
    echo "错误: 未找到 Go，请先安装 Go (建议 >= 1.21)"
    exit 1
}

GO_VERSION=$(go version | awk '{print $3}')
echo "Go 版本: ${GO_VERSION}"

echo ""
echo "==> 编译 CentOS 7 兼容的静态二进制文件..."

# CentOS 7 使用较老的 glibc (2.17) 和内核 (3.10.x)
# 关闭 CGO 生成完全静态链接的二进制文件，避免 glibc 版本依赖
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

# -ldflags 优化: 去除调试信息、压缩二进制
# -trimpath 去除构建路径信息
# -tags netgo,osusergo 确保纯 Go 的网络和用户信息解析
go build -trimpath \
    -ldflags '-s -w -extldflags "-static"' \
    -tags "netgo osusergo" \
    -o "${BINARY_NAME}_centos7" \
    .

if [[ ! -f "${BINARY_NAME}_centos7" ]]; then
    echo "错误: 编译失败，二进制文件 ${BINARY_NAME}_centos7 未生成"
    exit 1
fi

echo "==> 编译完成: ${BINARY_NAME}_centos7"
file "${BINARY_NAME}_centos7"
ldd "${BINARY_NAME}_centos7" 2>/dev/null || echo "    静态二进制文件 (not a dynamic executable)"

# 清理并创建分发目录
echo ""
echo "==> 准备分发目录..."
rm -rf "${PACKAGE_DIR}"
mkdir -p "${PACKAGE_DIR}"

# 复制二进制文件
cp "${BINARY_NAME}_centos7" "${PACKAGE_DIR}/${BINARY_NAME}_centos7"
chmod +x "${PACKAGE_DIR}/${BINARY_NAME}_centos7"

# 复制运行时所需的静态文件和配置
cp -r static "${PACKAGE_DIR}/"
cp -r config "${PACKAGE_DIR}/"

# 创建运行时目录
mkdir -p "${PACKAGE_DIR}/cache"
mkdir -p "${PACKAGE_DIR}/logs"

# 生成 systemd 服务文件 (CentOS 7 使用 systemd)
cat > "${PACKAGE_DIR}/dict_new.service" <<'EOF'
[Unit]
Description=Dict New Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/dict_new
ExecStart=/opt/dict_new/dict_new_centos7
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 生成安装说明
cat > "${PACKAGE_DIR}/README-CENTOS7.md" <<'EOF'
# CentOS 7 部署说明

## 快速启动

```bash
# 1. 复制分发目录到 /opt
cp -r dist/dict_new_centos7 /opt/dict_new
cd /opt/dict_new

# 2. 编辑配置文件
vim config/app.yaml

# 3. 启动服务 (前台运行测试)
./dict_new_centos7

# 4. 或注册为 systemd 服务
sudo cp dict_new.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable dict_new
sudo systemctl start dict_new

# 查看状态
sudo systemctl status dict_new
```

## 防火墙放行

```bash
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --reload
```

## 说明

本二进制文件使用 `CGO_ENABLED=0` 静态编译，不依赖特定版本的 glibc，可直接在 CentOS 7 上运行。
EOF

echo ""
echo "========================================"
echo "  构建成功"
echo "========================================"
echo "  编译产物: ${BINARY_NAME}_centos7"
echo "  分发目录: ${PACKAGE_DIR}"
echo "    └── ${BINARY_NAME}_centos7"
echo "========================================"
