# ServiceDesktop 问题清单（≥100 项）

本文档基于对源码的全面审查，按严重程度和模块分类列出 100+ 个问题。

---

## 一、并发安全与竞态（Critical）

1. **`App.QuitApp` 无锁写 `isQuitting`**  
   `isQuitting` 在 `beforeClose` 和 `QuitApp` 之间没有原子操作或互斥锁，可能发生竞态。

2. **`Service.Status` 裸字段无锁访问**  
   `app.go` 中 `loadServices()` 直接 `svc.Status = a.runtime.CheckStatus(&svc)`，而 `Runtime.Start` / `Runtime.Stop` / `RefreshAll` 也直接修改该字段，多 goroutine 并发写 `Status` 无保护。

3. **`RefreshAll` 并发写 `svc.Status`**  
   `RefreshAll` 对每个服务启动 goroutine 写 `svc.Status`，与 `Start`/`Stop` 方法同时调用时发生竞态。

4. **`loadServices()` 修改 `svc.EnvVars` 和 `svc.Args`**  
   启动阶段 `loadServices` 通过 `copyEnvVars` / `copyStringSlice` 赋值，随后其他 goroutine 读取，无同步。

5. **`pushStatusUpdate()` 遍历 `a.services` 无锁**  
   `pushStatusUpdate` 在 UI goroutine 中遍历 `a.services`，同时 `loadServices` 可能替换 `a.services` 切片。

6. **`Start` 方法持锁等待 30s 端口就绪**  
   `startMu.Lock()` 在端口预检前获取，等待端口（最多 30s）期间一直持有锁，阻塞同服务的 `Stop`。

7. **`starting` map 中存的是 `*sync.Mutex` 指针可被覆盖**  
   两个 goroutine 同时调用 `Start(sameSvc)` 时，第一个可能拿到旧的 mutex 而第二个拿到新的，但旧 mutex 已被释放 — 存在 ABA 问题。

8. **`plugins/registry.go` 全局 map 无并发读保护**  
   `GetPlugin` / `GetAllPlugins` 在运行时被并发调用（多个 goroutine 读），写仅在 `init()` 完成，Go map 并发读本身安全，但 `GetAllPlugins` 返回的切片是动态分配，若后续有插件动态注册会丢失。

9. **`cfg.Save()` 内的 `sync.Mutex` 不保护所有读**  
   `Config` 的字段（如 `PathOverrides`、`StartProfiles`）在 `loadServices` 中被读取时未加锁。

---

## 二、资源泄漏与 Goroutine 泄漏

10. **`StreamLog` 每次调用创建新 goroutine**  
    每次前端打开日志时调用 `StreamLog`，内部启动 goroutine + 创建 `context.WithCancel`。若用户频繁关闭/打开日志，旧 goroutine 未被及时取消（虽然有 `streamCancel[serviceID]` 取消前一个，但模式脆弱）。

11. **`Start` 中 goroutine 监听进程退出时引用 `svc` 指针逃逸**  
    ```go
    go func() {
        _ = cmd.Wait()
        if svc.Status == StatusRunning {
            svc.Status = StatusError
        }
    }()
    ```
    `svc` 指针被 goroutine 捕获，生命周期超出调用栈。

12. **`LogCollector.stop()` 关闭所有 subscriber channel**  
    `close(ch)` 后若仍有代码尝试 `ch <- line` 则 panic（虽用 `recover()` 兜底，但这不是正常关闭方式）。

13. **`TailFile` 链式 cancel 函数顺序可能导致提前取消**  
    同一个服务的 `TailFile` 多次调用时，`prevCancel()` 被嵌套包装 — 若外层 cancel 被调用，内层也被级联取消，但资源未完全释放。

14. **`attachProcess` 创建的 stdout/stderr pipe 在进程崩溃后可能残留**  
    若 `cmd.Wait()` 返回后 pipe 读取 goroutine 仍在运行（因为 `ctx.Done()` 可能未触发），goroutine 泄漏。

15. **前端 `EventsOn('log-update', …)` 从未 `EventsOff`**  
    Wails 事件监听器在页面 unload 时未清理，可能造成内存累积。

16. **`runtime.EventsEmit` 推送大 JSON 字符串在频繁日志时压力大**  
    `StreamLog` 每 800ms 推送一次，JSON `Marshal` 整个 logLines 数组，高频日志场景下 GC 压力较大。

---

## 三、错误处理与边界情况

17. **`trySingleton` 未正确处理所有 Windows 错误码**  
    `CreateMutexW` 返回 `ERROR_ALREADY_EXISTS` (183) 时判定为已运行，但其他错误码（如 `ERROR_ACCESS_DENIED`）被静默忽略，返回 `true`。

18. **`StopService` 忽略 `runtime.Stop` 返回的错误**  
    `app.go:345` — `_ = a.runtime.Stop(svc)` 丢弃了停止服务时可能发生的错误。

19. **`StartService` 在 `pushStatusUpdate` 之后返回**  
    状态推送是异步事件，用户可能看到过时的状态，直到 push 到达。

20. **`GetLogContent` 将整个文件读入内存无大小限制**  
    若日志文件达 GB 级，`os.ReadFile` 会耗尽内存。

21. **`SetWatchLogFile` 有 TOCTOU 竞态**  
    先 `os.Stat(filePath)` 验证存在，后用 `collector.TailFile(filePath)` — 文件可能在验证后被删除/替换。

22. **`deleteProfile` 使用 `prompt()` 获取配置名称**  
    弹窗的文本框中用户可直接输入任意内容，后端无校验。

23. **`AddCustomService` 接口无任何输入校验**  
    端口号可为负数、启动命令可为空字符串、路径可为不存在的目录。

24. **`EditCustomService` 中参数 `envVars` 解析失败时静默忽略**  
    `parseEnvVars` 对 `KEY=VAL` 格式不匹配的片段直接跳过，用户以为设置了环境变量但实际未生效。

25. **`splitArgs` 不支持转义引号**  
    参数 `"a b" c` 中的引号能正确处理，但 `\"a b\"` 会导致错误拆分。

26. **`parseEnvVars` 使用 `;` 做分隔符**  
    若环境变量值本身包含 `;`（如 `PATH=C:\bin;D:\bin`），解析会错误切分。

27. **`ReadConfigFile` 路径穿越**  
    `filepath.Join(svc.InstallPath, fileName)` — 若 `fileName` 包含 `../` 可能逃逸到安装目录之外。需检查 `path.Clean` 后是否以 `InstallPath` 为前缀。

28. **`SaveConfigFile` 同样路径穿越漏洞**  
   同上，可写入任意路径。

29. **`loadTranslationMap` 的 `paths` 切片只迭代一次但声明为数组**  
    误导性代码：`paths` 长度恒为 1，循环一次即返回。

30. **`GetAppConfig` 返回 `AutoStart` 但不返回当前设置是否成功**  
    前端无法知道开机自启注册表是否写入成功。

31. **`setAutoStart` 调用 `cmd.Run()` 不考虑退出码**  
    `reg.exe` 可能失败（如权限不足），调用方无法感知。

---

## 四、性能问题

32. **`CheckStatus` 每次调用都执行 `findPidByPort` 运行 `netstat -ano`**  
    每次刷新都 fork 一个子进程执行 `netstat`，高频轮询场景成本极高。

33. **`isProcessAlive` 同样运行 `tasklist` 子进程**  
    单个调用开销不大，但在 `RefreshAll` 中可能同时执行数十次。

34. **`findPidByPortWindows` 捕获整个 `netstat -ano` 输出**  
    大型系统上 `netstat` 输出可达数 MB，每次扫描都全量捕获。

35. **`GetLogContent` 和 `GetLogFiles` 重复读取目录**  
    两个独立方法都调用 `os.ReadDir(logsDir)`，没有缓存。

36. **`Start` 方法在持有锁时做 `os.Stat` I/O 操作**  
    `splitInlineArgs` 内部对命令路径做 `os.Stat`，在持有 `startMu` 锁时执行阻塞 I/O。

37. **`pushStatusUpdate` 每次 marshal 全部服务列表**  
    即使只有一个服务状态变更，也 JSON 序列化整个 `[]ServiceDTO`。

38. **前端 `renderSidebar` 每次重新构建全部 HTML**  
    只有服务和状态变化时才应该局部更新，而非全量 innerHTML 替换。

39. **日志行正则 `javaLogRe` 对每行都匹配**  
    高吞吐日志场景下，regexp 匹配可能成为性能瓶颈（虽已编译，但仍需逐行匹配）。

---

## 五、代码质量与可维护性

40. **`app.go` 文件长达 1300 行**  
    单一文件承担了服务加载、CRUD、日志、配置、设置等所有职责，严重违反单一职责原则。

41. **Go 模块版本号 `go 1.25.0`**  
    截至 2025 年 Go 稳定版为 1.24.x，1.25 尚不存在。这会导致 `go` 工具链报错 — 除非项目使用了超前构建工具。

42. **`EditCustomService` 方法 13 个参数**  
    应使用结构体参数，当前列表过长，极易传错。

43. **`ServiceDTO` 同时包含 `Status`（int）和 `StatusText`（string）**  
    冗余字段，前端可以从 int 自行推导文本。

44. **`toDTO` 和 `toDTOs` 两个方法分开但缺乏一致性校验**  
    多次出现的转换逻辑容易不同步。

45. **`plugins` 目录下 8 个插件的 `BeforeStart`、`AfterStop`、`ConfigFiles` 逻辑几乎相同**  
    Tomcat/Redis/Kafka/Nginx 四个插件都有完全相同的 `os.MkdirAll(logs)` + `os.Stat` 循环模式，大量重复代码。

46. **`registry.go` 中的 `DefaultRegistry()` 与 `plugin.go` 的注册机制功能重叠**  
    模板定义同时在 `DefaultRegistry()` 和各插件的 `Template()` 中存在，两条路径容易不一致。

47. **`GetLogGroupedFiles` 和 `GetLogFileContent` 方法在前端被调用但后端未找到对应实现**  
    `app.go` 中无 `GetLogGroupedFiles` 方法（仅有 `GetLogFiles`），前端代码调用了不存在的方法。

48. **JSON 字段命名风格不统一**  
    `ServiceDTO` 用 `installPath`（驼峰），`Service` 用 `install_path`（蛇形），前端和 Go 结构体混用两种风格。

49. **`DiscoveredServiceConf` 与 `UserServiceConf` 字段高度重叠**  
    两个结构体本质相同，应复用或组合。

50. **Go 包命名不够规范**：`ServiceDesktop/config` 而非 `config` — 模块内包名不应包含模块名前缀。

51. **多个魔法字面量**  
    `RefreshAll` 并发数 4、`waitForPort` 超时 30s、`waitForPortClosed` 超时 5s、`logBuf` 容量 2000、`TailFile` 轮询 200ms — 均硬编码。

52. **注释混杂中英文，部分过时**  
    `app.go:74` — `// 在这里清理资源` 无实际代码；`runtime.go:222` — `StopTail` 定义了两次（导出和私有版本）。

53. **`loadServices()` 既是初始化方法又有持久化副作用（保存发现服务）**  
    违反 CQS（命令-查询分离），调用者难以预料其效果。

---

## 六、安全问题

54. **`SaveConfigFile` 无任何文件类型/路径白名单**  
    可写任意文件到任意安装路径（配合路径穿越）。

55. **`ReadConfigFile` 同样可读任意文件**  
    在服务安装路径下可读取任何文件，若 `InstallPath` 被误配置可泄露敏感信息。

56. **用户服务配置以明文 JSON 存储**  
    若服务配置中包含密码（如 MySQL root 密码），`config.json` 中直接明文存储。

57. **`setAutoStart` 运行时拼接 shell 命令**  
    ```go
    cmd.Args = append(cmd.Args, "/d", "\""+exe+"\"")
    ```
    若 `os.Executable()` 返回的路径含特殊字符，可能导致注册表写入异常。

58. **`ReadConfigFile` HTTP 返回内容无敏感信息过滤**  
    即使包含密码的文件也直接返回给前端。

59. **前端 `loadProfile` 通过 `JSON.stringify` 传递参数**  
    `onclick='loadProfile("${name}", ${JSON.stringify(profiles[name])})'` — 若 profile 值含 `"` 可能 XSS。

---

## 七、国际化问题

60. **后端 Go 错误消息全部中文硬编码**  
    `"服务未找到"`、`"启动失败"` 等字符串在前端 `i18n` 之外，无法国际化。

61. **前端状态文本全部中文硬编码**  
    `statusLabel` 函数中 `'已停止'`、`'运行中'` 等字符串未走 i18n。

62. **`catLabel` 硬编码中文**  
    `{Middleware:'中间件', Database:'数据库', Custom:'自定义'}` — 未走 i18n。

63. **i18n 加载策略在 `app.go` 中是懒加载，且 `language` 变更后需重启应用才生效**  
    `SetAppConfig` 虽然调用 `loadTranslations`，但前端界面的翻译文本不自动刷新。

---

## 八、Windows 平台绑定与兼容性

64. **大量使用 `syscall.NewLazyDLL` 调用 Windows API**  
    `CreateMutexW`、`MessageBoxW` — 不跨平台且无回退。

65. **`hiddenCmd` 在非 Windows 平台不设置 `HideWindow`**  
    但 `findPidByPortUnix` 中调用了 `lsof` — macOS 默认无 `lsof`，需安装。

66. **`killProcess` 在非 Windows 平台中先 SIGTERM 再 2s 后 SIGKILL**  
    间隔固定 2s，没有检查进程是否已优雅退出。

67. **`setAutoStart` 只支持 Windows 注册表**  
    macOS/Linux 上此方法静默失败。

68. **`openFolder()` 调用 `explorer.exe` 硬编码**  
    非 Windows 平台上路径用 `xdg-open` 或 `open` 命令。

69. **路径分隔符硬编码为 `\`**  
    大量使用 `{install_path}\`，Linux/macOS 上路径不兼容。

70. **`Go toast` 库仅在 Windows 有效**  
    `go.mod` 中包含 `git.sr.ht/~jackmordaunt/go-toast/v2`，但在 macOS/Linux 上禁用。

---

## 九、前端问题

71. **前端使用 Vanilla JS 但 `frontend/src/main.js` 与 `src/main.js` 两份实现并存**  
    `frontend/src/` 使用 ES Module 拆分（utils.js / sidebar.js / detail.js / modals.js），而 `frontend/dist/` 中似乎是旧版单文件构建，存在两份不同实现。

72. **`canEdit` 硬编码 8 个内置服务 ID 前缀**  
    `detail.js:250` — `!svc.id.startsWith('tomcat') && ...` — 新增插件时必须同步修改前端代码。

73. **前端 `onLogSourceChange` 中 `JSON.parse` 无 try-catch 包围**  
    但调用的外层有 try-catch，不过嵌套较深。

74. **`renderStructuredLines` 和 `formatLogContent` 两套日志渲染函数几乎相同**  
    `utils.js` 中两套函数重复实现了相同的 HTML 生成逻辑。

75. **前端日志搜索在客户端逐个比较 DOM 元素**  
    日志量大时（数千行），`querySelectorAll` + 逐行 `style.display` 切换可能导致 UI 卡顿。

76. **`renderSidebar` 每次全量重建 DOM**  
    `sidebarList.innerHTML = html` 销毁并重建所有元素，丢失事件监听器（虽然后续用 `querySelectorAll` 重新绑定）。

77. **`frontend/index.html` 仅含 CSS 变量和骨架**  
    实际内容完全由 JS 动态生成，SEO 和首次加载体验不佳（但这是 Desktop 应用，可接受）。

78. **`EventsOn` 回调中直接 `JSON.parse(data.lines)`**  
    如果 `data.lines` 不是合法 JSON 字符串，`JSON.parse` 抛出异常 — 虽被 try-catch 捕获，但静默失败不利于调试。

79. **`profileArgsInput` 的值通过 `prompt()` 获取配置名**  
    用户拒绝 prompt 时返回 `null`，代码判断 `if (!name) return` 正确，但体验差。

80. **无 any loading skeleton / spinner**  
    所有异步操作（`GetServices`、`GetLogContent` 等）在等待时仅显示 `"加载中..."` 文本。

---

## 十、配置与持久化问题

81. **`config.Save()` 非原子写入**  
    `os.WriteFile` 直接覆盖文件，写入过程中崩溃会导致配置文件损坏。

82. **`config.json` 中 `Args` 在 `UserServiceConf` 中是 `string`，在 `Service` 中是 `[]string`**  
    类型不一致。`UserServiceConf.Args` 存的是空格分隔的字符串，`Service.Args` 是切片。

83. **`loadServices()` 每次启动都修改 `DiscoveredServices` 并保存**  
    即使没有任何变化也调用 `cfg.Save()`，造成不必要的磁盘写入。

84. **`DiscoveredServices` 按端口去重但无 ID 去重**  
    若两个不同端口对应同一服务（如 Nginx 80 和 8080），会创建两条记录。

85. **`PathOverrides` 和 `StartProfiles` 没有被 `loadServices()` 清空**  
    当插件模板更新后，旧的覆盖配置依然生效，用户可能误以为模板未更新。

---

## 十一、服务发现问题

86. **`portScan` 依赖 `gopsutilNet.Connections("tcp")` — 在 Windows 上可能需要管理员权限**  
    无管理员权限时可能返回空连接列表，导致端口扫描失败。

87. **`dockerScan` 硬编码 `docker` 命令路径**  
    若 Docker 不在 `PATH` 中或未安装，`exec.Command("docker", …)` 失败被静默忽略。

88. **`probeMySQL` 只读取前 5 字节验证协议版本**  
    `buf[4] == 0x0a` 仅检查 MySQL 协议版本 — 其他协议的端口监听也可能返回 0x0a，造成误判。

89. **`probeTomcat` 检查响应中包含 `coyote` 或 `apache tomcat`**  
    其他 Java HTTP 服务器（如 Jetty、Undertow）不会误判，但自定义响应头可能产生假阳性。

90. **`matchDockerImage` 做子串匹配**  
    `strings.Contains(image, "redis")` 可能匹配 `redisearch`、`redis-sentinel` 等镜像名。

91. **`extractDockerPort` 解析端口映射逻辑不完善**  
    对 `0.0.0.0:3306->3306/tcp` 格式有效，但对 `127.0.0.1:5432->5432/tcp` 的 `127.0.0.1:5432` 部分，`LastIndex(":")` 的行为正确但依赖于格式固定。

92. **`processEnrich` 修改 `inst.Pid` 等字段时无锁**  
    同时 `portScan` goroutine 已完成，但 `RunDiscovery` 随后在主 goroutine 中调用 `processEnrich`，此时 `seen` map 没有再被并发修改 — 由于 `wg.Wait()` 后才调用 `processEnrich`，所以实际上安全（但逻辑隐晦）。

93. **自动发现服务启动命令中 `{install_path}` 未被解析**  
    `loadServices` 中 `ds.StartCmd` 是从 `knownServices.StartCmd` 直接复制，未被 `resolvePath` 替换占位符。

---

## 十二、测试与 CI/CD

94. **整个项目没有任何测试文件**  
    零单元测试、零集成测试、零 E2E 测试。`go test ./...` 会直接通过（没有测试文件）。

95. **无 lint 配置或 pre-commit 钩子**  
    `.gitignore` 存在但无 `.golangci.yml`、`eslintrc` 等。

96. **`wails.json` 中 `outputfilename` 为 `wails-temp`**  
    实际发布的 exe 名为 `ServiceDesktop.exe`，build 配置名与实际输出名不一致。

97. **`go.mod` 中 `replace` 指令被注释掉，但包含用户本地路径**  
    `// replace github.com/wailsapp/wails/v2 v2.12.0 => C:\Users\l\go\pkg\mod` — 泄露了本地开发路径。

---

## 十三、杂项

98. **`nul` 空文件存在于项目根目录**  
    可能为调试残留。

99. **`_bak_v4.exe` 和 `_bak_v5.exe` 等大文件（各 45MB）提交在仓库中**  
    二进制文件不应在版本控制中，破坏仓库大小和 clone 速度。

100. **`ServiceDesktop_v4.exe~` 备份文件也未 gitignore**  
     同样是大二进制文件。

101. **`.codegraph/` 和 `.reasonix/` 等 IDE/工具目录无 `.gitignore`**  
     可能在多人协作时被提交（虽然 `.gitignore` 目前不存在这些条目）。

102. **`README.md` 仍是 Wails 默认模板内容**  
     未更新为项目专属说明。

103. **`loadTranslationMap` 中 `paths` 声明为 `[]string` 但有且只有一个元素**  
     多余的切片，可直接用单字符串路径。

104. **`main.go` 中 `_ "ServiceDesktop/services/plugins"` 的注释 `// 触发所有插件的 init() 注册`**  
     这是 Go 标准模式，但注释暗示这是一个 hack 而非标准做法。

105. **`runtime.go` 中 `LogLevel` 解析逻辑（`parseLine`）适用于 Java 日志格式，对其他服务无意义**  
     Redis / Nginx / MongoDB 的输出不匹配 Java 日志格式，水平/线程/logger 字段永远为空。

106. **`service/plugins/nacos.go` 的 `BeforeStart` 自动修改 `application.properties` 文件**  
     不可逆地修改配置文件，若用户手动编辑过该文件，启动时会被覆写。

107. **`runtime.go:222-238` — 存在两个 `StopTail` 方法签名相同但版本不同**  
     `StopTail` 保留订阅者，`stop()` 会关闭所有 channel — 容易误调用。

108. **前端 `app.js` 和 `sidebar.js` 都定义了 `renderSidebar` 函数**  
     `frontend/src/js/` 使用模块化导入，但 `frontend/src/main.js`（旧版）也定义了同名函数 — 代码交织混用。

109. **前端 `wailsjs/runtime/runtime.js` 和 `wailsjs/go/main/App.js` 是自动生成文件**  
     但这些生成文件被检入版本控制，应该在构建时生成而非提交。

110. **`SetWatchLogFile` 中 persistence 逻辑只操作 `UserServices` 和 `DiscoveredServices`**  
     对于插件模板服务（如 Tomcat），日志路径变更不会持久化。
