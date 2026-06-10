# ServiceDesktop

> 本地开发服务的一站式管理桌面工具（Windows）

一键启停、实时日志、自动发现、配置文件编辑 —— 在桌面集中管理你的 Tomcat、Redis、Kafka、Nacos、Nginx、MySQL、PostgreSQL、MongoDB 等开发服务。

![screenshot](https://img.shields.io/badge/platform-Windows-blue) ![language](https://img.shields.io/badge/language-Go%2BJavaScript-yellow) ![framework](https://img.shields.io/badge/framework-Wails_v2-252526)

---

## 功能

| 功能 | 说明 |
|---|---|
| 🚀 **一键启停** | 启动/停止/重启任意服务，支持并发启动（最多 4 个同时） |
| 🔍 **自动发现** | 四层扫描：端口嗅探 → Docker 检测 → 进程识别 → 协议验证 |
| 📋 **实时日志** | 进程 stdout/stderr 实时捕获 + 日志文件 tail，支持日志轮转 |
| ✏️ **配置编辑** | 内置代码编辑器，直接在 UI 中修改 server.xml、nginx.conf 等 |
| 📂 **日志浏览** | 按类型/日期分组的日志文件查看，实时搜索过滤 |
| 🔧 **自定义服务** | 添加任意命令行程序作为服务管理 |
| 📝 **启动参数** | 保存多组启动参数配置（Profile），一键切换 |
| 🌐 **国际化** | 中文/英文界面 |
| ⚙️ **开机自启** | 注册 Windows 开机自启动 |
| 🐳 **Docker 感知** | 自动识别 Docker 容器中运行的服务 |

## 截图

（界面以实际为准）

```
┌─────────────┬──────────────────────────────────────┐
│  搜索框     │  服务详情 / 启停控制 / 状态面板      │
│             │                                      │
│  ▼ 中间件   │  端口: 8080   PID: 12345  运行中     │
│    Tomcat   │                                      │
│    Nginx    │  ┌─ 最近日志 ────────────────────┐   │
│    Kafka    │  │  06-10 09:43:16 INFO ...      │   │
│             │  │  06-10 09:43:17 WARN ...      │   │
│  ▼ 数据库   │  └──────────────────────────────┘   │
│    Redis    │                                      │
│    MySQL    │  ┌─ 配置文件 ───────────────────┐   │
│             │  │  server.xml          [编辑]  │   │
│             │  │  catalina.properties [编辑]  │   │
│             │  └──────────────────────────────┘   │
│  ────────   │                                      │
│  3/8 运行中 │                            [全部启动]│
└─────────────┴──────────────────────────────────────┘
```

## 快速开始

### 前置要求

- Go 1.21+
- Node.js 16+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)

### 开发模式

```bash
wails dev
```

前端热重载 + Go 后端实时编译，浏览器访问 `http://localhost:34115` 可直接调试。

### 构建

```bash
wails build
```

产物为 `build/bin/ServiceDesktop.exe`（约 45MB，单文件分发包）。

## 架构概览

```
main.go                  ← 应用入口 + 单实例保护
app.go                   ← 核心枢纽（Go ↔ JS 绑定，~1300行）
services/
├── types.go             ← Service / ServiceTemplate 类型定义
├── plugin.go            ← ServicePlugin 接口 + 注册表
├── registry.go          ← 8 个内置服务模板定义
├── runtime.go           ← 服务启停 + 日志收集器（~940行）
├── scanner.go           ← 四层服务自动发现（~500行）
├── plugins/
│   ├── tomcat.go        ← Tomcat 插件
│   ├── redis.go         ← Redis 插件
│   ├── kafka.go         ← Kafka 插件
│   ├── nacos.go         ← Nacos 插件
│   ├── nginx.go         ← Nginx 插件
│   ├── mysql.go         ← MySQL 插件
│   ├── postgresql.go    ← PostgreSQL 插件
│   └── mongodb.go       ← MongoDB 插件
config/
└── config.go            ← JSON 配置持久化（%APPDATA%/ServiceDesktop/config.json）
i18n/
├── zh.json              ← 中文
└── en.json              ← English
frontend/
├── src/
│   ├── main.js          ← 入口 + 事件监听
│   ├── app.css          ← 全局样式
│   ├── app.js           ← 应用主模块
│   ├── js/
│   │   ├── utils.js     ← 工具函数
│   │   ├── sidebar.js   ← 侧栏渲染
│   │   ├── detail.js    ← 详情面板
│   │   ├── log.js       ← 日志查看
│   │   └── modals.js    ← 模态框
│   └── style.css        ← 旧版样式（Wails 默认）
└── index.html           ← 入口 HTML
```

### 技术栈

| 层 | 技术 |
|---|---|
| 桌面框架 | [Wails v2](https://wails.io/)（Go ↔ JS Bridge） |
| 后端 | Go 1.25（标准库 + gopsutil 系统信息） |
| 前端 | Vanilla JS + Vite 3 |
| 样式 | CSS Variables + 原生 CSS |

## 内置服务

| 服务 | 类型 | 默认端口 | 启停方式 |
|---|---|---|---|
| **Apache Tomcat** | 中间件 | 8080 | startup.bat / shutdown.bat |
| **Apache Kafka** | 中间件 | 9092 | kafka-server-start.bat / 强制终止 |
| **Nacos** | 中间件 | 8848 | startup.cmd / shutdown.cmd |
| **Nginx** | 中间件 | 8080 | nginx.exe / nginx -s stop |
| **Redis** | 数据库 | 6379 | redis-server.exe / 强制终止 |
| **MySQL** | 数据库 | 3306 | mysqld.exe / 强制终止 |
| **PostgreSQL** | 数据库 | 5432 | pg_ctl start / pg_ctl stop |
| **MongoDB** | 数据库 | 27017 | mongod.exe / 强制终止 |

> 服务自动扫描本地安装路径（`C:\tools\*`、`C:\Program Files\*` 等），也可手动设置路径。

## 快捷键

| 按键 | 操作 |
|---|---|
| `Enter`（路径编辑弹窗） | 确认路径修改 |
| `Enter`（启动参数弹窗） | 应用参数 |
| 搜索框输入 | 自动过滤服务列表（150ms 防抖） |

## 配置文件

存储在 `%APPDATA%\ServiceDesktop\config.json`，包含：

- 语言设置与开机自启开关
- 用户自定义服务定义
- 自动发现服务列表
- 路径覆盖与启动参数配置（Profile）

## 常见命令

```bash
# 开发模式（热重载）
wails dev

# 构建生产版本
wails build

# 仅构建前端（调试用）
cd frontend && npm run build
```

## 已知限制

- **仅 Windows 完全支持**：使用 Windows API（命名 Mutex、注册表自启、`taskkill`），macOS/Linux 部分功能受限
- **日志解析侧重 Java 格式**：Tomcat/Nacos/Kafka 日志能提取 level/logger/thread，其他服务为纯文本
- **服务状态检测依赖端口**：无端口的服务无法精确判断运行状态

## License

MIT
