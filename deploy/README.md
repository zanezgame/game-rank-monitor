# 游戏榜单监控平台 — 远程部署包（2026-10-20 准备）

用户已要求部署到远程服务器，目前卡在等用户提供：服务器地址、SSH 用户名、登录方式（密钥/密码）。
本目录是预先做好的部署物料，拿到服务器信息后即可一条龙执行，无需再临时准备。

## 物料清单

| 文件 | 说明 |
|---|---|
| `gamerank-deploy.tar.gz` | 部署包：后端二进制（linux x86-64，含 10-20 的 `-db` 路径修复与 Steam 名称回填修复：国区接口失败时 fallback 英文 + 整榜补名）、前端 `dist/`（含 10-20 的模板渲染修复）、2.1M 历史数据库（含 10-18 起的真实日榜/周榜数据） |
| `gamerank.service` | systemd 服务：后端常驻 `:8080`，开机自启，崩溃自动重启 |
| `gamerank-collect.service` / `gamerank-collect.timer` | 每天 02:10（北京时间）采集日榜，一次性运行后退出 |
| `gamerank-weekly.service` / `gamerank-weekly.timer` | 每周一 03:00（北京时间）聚合周榜 |
| `nginx-gamerank.conf` | nginx 站点配置：静态服务前端 `dist/`（含 SPA fallback），`/api` 反代到 `127.0.0.1:8080` |
| `remote-deploy.sh` | 在服务器上以 root（或 sudo）运行的一键安装脚本：建目录、解压、装 systemd、配 nginx、设时区、启动并自检 |

## 部署时执行（拿到服务器信息后）

```bash
# 1. 传包和脚本到服务器
scp deploy/gamerank-deploy.tar.gz deploy/remote-deploy.sh user@server:/tmp/

# 2. 在服务器上执行（root 或 sudo）
ssh user@server
sudo bash /tmp/remote-deploy.sh /tmp/gamerank-deploy.tar.gz your-domain-or-ip
```

脚本会输出访问地址和自检结果（后端健康检查 + 当天采集行数）。

## 前提与假设

- 服务器为任意主流 Linux（x86-64，glibc），有 systemd 和 nginx（脚本会自动 `apt/yum install nginx`，装不上则报错退出，不强行继续）
- 脚本把服务器时区设为 `Asia/Shanghai`（采集时间按北京时间；可逆：`timedatectl set-timezone` 改回即可）
- 不需要任何环境变量（微信相关变量已随功能移除；`GOOGLEPLAY_CHART_URL_*` 为可选覆盖，默认内置）
- 历史数据库随包带上：服务器上线第一天就有完整榜单可看，而不是从零开始攒数据
