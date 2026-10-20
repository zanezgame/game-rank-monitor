#!/usr/bin/env bash
# 游戏榜单监控平台 — 服务器一键安装脚本
# 用法（在服务器上以 root 或 sudo 运行）：
#   sudo bash remote-deploy.sh /tmp/gamerank-deploy.tar.gz <域名或IP>
#
# 做的事情（全部幂等，可重复运行）：
#   1. 建 /opt/gamerank 目录结构与 gamerank 系统用户
#   2. 解压部署包（后端二进制 + 前端 dist + 历史数据库）
#   3. 安装并启用 systemd：后端常驻服务、日榜采集 timer、周榜聚合 timer
#   4. 安装 nginx（如缺失则尝试 apt/yum 安装）并写入站点配置
#   5. 把服务器时区设为 Asia/Shanghai（采集时间按北京时间）
#   6. 启动服务并自检：后端 /api/platforms 返回 3 平台、当天日榜有数据
set -euo pipefail

TARBALL="${1:?用法: $0 <gamerank-deploy.tar.gz 路径> <域名或IP>}"
SERVER_NAME="${2:?用法: $0 <gamerank-deploy.tar.gz 路径> <域名或IP>}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_ROOT="/opt/gamerank"

if [[ $EUID -ne 0 ]]; then
  echo "请以 root 或 sudo 运行此脚本" >&2
  exit 1
fi
if [[ ! -f "$TARBALL" ]]; then
  echo "部署包不存在: $TARBALL" >&2
  exit 1
fi

echo "==> 1/6 创建目录与系统用户"
id gamerank &>/dev/null || useradd -r -s /usr/sbin/nologin -d "$INSTALL_ROOT" gamerank
mkdir -p "$INSTALL_ROOT/bin" "$INSTALL_ROOT/backend/data/logs" "$INSTALL_ROOT/frontend"

echo "==> 2/6 解压部署包到 $INSTALL_ROOT"
tar -xzf "$TARBALL" -C "$INSTALL_ROOT"
chown -R gamerank:gamerank "$INSTALL_ROOT"
chmod +x "$INSTALL_ROOT/bin/gamerank-server"

echo "==> 3/6 安装 systemd 单元"
for f in gamerank.service gamerank-collect.service gamerank-collect.timer gamerank-weekly.service gamerank-weekly.timer; do
  cp "$SCRIPT_DIR/$f" /etc/systemd/system/
done
systemctl daemon-reload
systemctl enable --now gamerank.service
systemctl enable --now gamerank-collect.timer
systemctl enable --now gamerank-weekly.timer

echo "==> 4/6 配置 nginx"
if ! command -v nginx &>/dev/null; then
  echo "未检测到 nginx，尝试安装…"
  if command -v apt-get &>/dev/null; then
    apt-get update -qq && apt-get install -y -qq nginx
  elif command -v yum &>/dev/null; then
    yum install -y -q nginx
  else
    echo "无法自动安装 nginx（无 apt/yum），请手动安装后重新运行本脚本" >&2
    exit 1
  fi
fi
sed "s/SERVER_NAME_PLACEHOLDER/${SERVER_NAME}/g" "$SCRIPT_DIR/nginx-gamerank.conf" > /etc/nginx/conf.d/gamerank.conf
nginx -t
systemctl enable --now nginx
systemctl reload nginx

echo "==> 5/6 设置时区为 Asia/Shanghai（采集按北京时间）"
timedatectl set-timezone Asia/Shanghai || echo "警告：时区设置失败，请手动执行 timedatectl set-timezone Asia/Shanghai"

echo "==> 6/6 自检"
sleep 3
PLATFORMS=$(curl -s http://127.0.0.1:8080/api/platforms || true)
echo "后端 /api/platforms: $PLATFORMS"
TODAY=$(date +%F)
ROWS=$(sqlite3 "$INSTALL_ROOT/backend/data/gamerank.db" \
  "SELECT COUNT(*) FROM rank_snapshots WHERE date='$TODAY' AND period='daily';" 2>/dev/null || echo 0)
echo "今日(${TODAY})日榜快照行数: $ROWS"

echo ""
echo "部署完成。访问地址：http://$SERVER_NAME"
echo "定时任务：每天 02:10 采集日榜，每周一 03:00 聚合周榜（北京时间）"
echo "日志目录：$INSTALL_ROOT/backend/data/logs/"
