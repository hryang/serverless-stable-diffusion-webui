## 准备

1. 安装 golang
2. 安装 sqlite3 命令行工具

## 运行

执行 `build.sh` 脚本即可。该脚本会在本地环境启动 1个sdproxy，2 个 sdagent 进程。sdproxy 接受浏览器或者 API 请求，并将请求以某种策略（当前是 round-robin）发送给 sdagent 进程。每个 sdagent 进程对应一个的 stable-diffusion-webui 服务。具体可通过修改 `build.sh` 来配置不同的后端服务。

在本地环境，所有的 meta 数据，包括任务进度信息等状态存储在本地的 SQLite 数据库中（当前目录下的 `test.db` 文件），最后生成的结果图片数据存储在指定的 OSS bucket 中。