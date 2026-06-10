import './app.css';

import { GetServices, StartService, StopService, RestartService, StartAllServices, GetConfigFiles, ReadConfigFile, SaveConfigFile, GetLogContent, GetLogFiles, GetLogFileContent, GetLogGroupedFiles, GetConsoleLog, ClearConsoleLog, SetWatchLogFile, GetWatchLogFile, GetServiceLogSources, SetInstallPath, AddCustomService, OpenFolder, SetStartArgs, SaveStartProfile, DeleteStartProfile, GetStartProfiles, BrowseFolder, StreamLog, CheckPort, KillProcess, QuitApp, GetAppConfig, SetAppConfig, GetServiceDetail, EditCustomService, DeleteCustomService } from '../wailsjs/go/main/App.js';
import { EventsOn } from '../wailsjs/runtime/runtime.js';

let services = [];
let selectedId = null;
let _searchTimer = null;
let _autoScrollLog = true;

// ========== Toast ==========

function toast(msg, duration) {
    const el = document.getElementById('toast');
    if (!el) return;
    el.textContent = msg;
    el.style.display = 'block';
    clearTimeout(el._timer);
    el._timer = setTimeout(() => { el.style.display = 'none'; }, duration || 3000);
}

// ========== 初始化 ==========

window.addEventListener('DOMContentLoaded', async () => {
    await loadServices();
    EventsOn('log-update', function(data) {
        if (data && data.id === selectedId) {
            const el = document.getElementById('logContent');
            const preview = document.getElementById('logPreview');
            if (data.lines) {
                try {
                    const logLines = JSON.parse(data.lines);
                    // 实时日志模态框：只在 watch 模式更新
                    if (el && document.getElementById('logModal').classList.contains('show') && _logViewMode === 'watch') {
                        el.innerHTML = renderStructuredLines(logLines);
                        if (_autoScrollLog) el.scrollTop = el.scrollHeight;
                    }
                    // 详情面板预览：始终显示最新 8 行
                    if (preview && logLines.length > 0) {
                        const last8 = logLines.slice(-8);
                        preview.innerHTML = renderStructuredLines(last8);
                    }
                } catch(e) {}
            }
        }
    });
});

async function loadServices() {
    try {
        services = await GetServices();
        renderSidebar();
        renderStatusBar();
        if (selectedId) {
            const found = services.find(s => s.id === selectedId);
            if (found) renderDetail(found);
        }
    } catch (e) {
        console.error('Failed to load services:', e);
    }
}

window.loadServices = loadServices;
window.refreshAll = loadServices;

// ========== 日志格式化（分列渲染）==========

/** 渲染结构化 LogLine 数组（来自控制台输出） */
function renderStructuredLines(logLines) {
    if (!logLines || logLines.length === 0) return '<div class="log-line"><span class="log-col-msg" style="color:var(--text-tertiary)">暂无日志</span></div>';
    return logLines.map(ll => {
        let time = '';
        if (ll.time) {
            try {
                const d = new Date(ll.time);
                if (!isNaN(d.getTime())) {
                    time = String(d.getMonth()+1).padStart(2,'0') + '-' +
                           String(d.getDate()).padStart(2,'0') + ' ' +
                           String(d.getHours()).padStart(2,'0') + ':' +
                           String(d.getMinutes()).padStart(2,'0') + ':' +
                           String(d.getSeconds()).padStart(2,'0');
                }
            } catch(e) {}
        }
        const level = ll.level || '';
        const logger = ll.logger || '';
        const text = ll.text || '';

        if (level) {
            return `<div class="log-line">
                <span class="log-col-time">${escapeHtml(time)}</span>
                <span class="log-col-level lvl-${level}">${level}</span>
                <span class="log-col-source" title="${escapeHtml(text)}">${escapeHtml(logger)}</span>
                <span class="log-col-msg">${escapeHtml(text)}</span>
              </div>`;
        }
        // 普通行
        const trimmed = text.trim();
        if (!trimmed) return '<div class="log-line"><span class="log-col-msg" style="color:var(--text-tertiary)">&nbsp;</span></div>';
        let msg = escapeHtml(text);
        msg = msg.replace(/(ERROR|FATAL)/g, '<span style="color:var(--red)">$1</span>');
        msg = msg.replace(/\b(WARN|WARNING)\b/g, '<span style="color:var(--orange)">$1</span>');
        msg = msg.replace(/\bINFO\b/g, '<span style="color:var(--accent)">$1</span>');
        return `<div class="log-line"><span class="log-col-msg">${msg}</span></div>`;
    }).join('\n');
}

/** 解析一行原始日志文本为结构化对象 */
function parseLogLine(line) {
    // Tomcat 格式: DD-MMM-YYYY HH:mm:ss.SSS 级别 [线程] 类名 消息
    const tomcatRe = /^(\d{2}-\w{3}-\d{4} \d{2}:\d{2}:\d{2}\.\d{3})\s+(\S+)\s+\[([^\]]+)\]\s+(\S+)\s+(.*)$/;
    let m = line.match(tomcatRe);
    if (m) {
        let time = m[1];
        // 缩短: DD-MMM-YYYY → MM月DD日
        try {
            const d = new Date(time.replace(/-(\w{3})-/g, (_, mon) => {
                const months = {Jan:'01',Feb:'02',Mar:'03',Apr:'04',May:'05',Jun:'06',
                               Jul:'07',Aug:'08',Sep:'09',Oct:'10',Nov:'11',Dec:'12'};
                return '-' + (months[mon]||'01') + '-';
            }).replace(/\.\d{3}/, ''));
            if (!isNaN(d.getTime())) {
                time = String(d.getMonth()+1).padStart(2,'0') + '-' +
                       String(d.getDate()).padStart(2,'0') + ' ' +
                       time.slice(time.length-8); // HH:mm:ss
            }
        } catch(e) {}
        let level = m[2];
        // 中文转英文
        if (level === '信息') level = 'INFO';
        else if (level === '警告') level = 'WARN';
        else if (level === '严重' || level === '错误') level = 'ERROR';
        let source = m[4];
        // 缩短来源：只取最后两段
        const parts = source.split('.');
        if (parts.length > 2) source = parts[parts.length-2] + '.' + parts[parts.length-1];
        return { time, level, source, fullSource: m[4], msg: m[5] };
    }
    // Java 堆栈 / 普通行
    return { time: '', level: '', source: '', msg: line };
}

/** 将整个文本格式化为分列 HTML */
function formatLogContent(text) {
    if (!text) return '';
    const lines = text.split('\n');
    const html = lines.map(line => {
        const p = parseLogLine(line);
        if (p.level) {
            return `<div class="log-line">
                <span class="log-col-time">${escapeHtml(p.time)}</span>
                <span class="log-col-level lvl-${p.level}">${p.level}</span>
                <span class="log-col-source" title="${escapeHtml(p.fullSource||p.source)}">${escapeHtml(p.source)}</span>
                <span class="log-col-msg">${escapeHtml(p.msg)}</span>
              </div>`;
        }
        // 普通行（堆栈、空白等）
        const trimmed = line.trim();
        if (!trimmed) return '<div class="log-line"><span class="log-col-msg" style="color:var(--text-tertiary)">&nbsp;</span></div>';
        // ERROR/WARN 高亮
        let msg = escapeHtml(line);
        msg = msg.replace(/(ERROR|FATAL)/g, '<span style="color:var(--red)">$1</span>');
        msg = msg.replace(/\b(WARN|WARNING)\b/g, '<span style="color:var(--orange)">$1</span>');
        msg = msg.replace(/\bINFO\b/g, '<span style="color:var(--accent)">$1</span>');
        return `<div class="log-line"><span class="log-col-msg">${msg}</span></div>`;
    }).join('\n');
    return html;
}

// ========== 搜索防抖 ==========

window.filterServices = function() {
    clearTimeout(_searchTimer);
    _searchTimer = setTimeout(() => renderSidebar(), 150);
};

// ========== 侧栏渲染（可折叠分组）==========

let _collapsedGroups = {};

function renderSidebar() {
    const list = document.getElementById('sidebarList');
    const search = (document.getElementById('searchInput').value || '').toLowerCase();

    const categories = {};
    for (const svc of services) {
        if (search && !svc.name.toLowerCase().includes(search)) continue;
        const cat = svc.category || '其他';
        if (!categories[cat]) categories[cat] = [];
        categories[cat].push(svc);
    }

    const catOrder = ['Middleware', 'Database', 'Custom', '其他'];
    const catLabel = {Middleware:'中间件', Database:'数据库', Custom:'自定义'};

    let html = '';
    for (const cat of catOrder) {
        const items = categories[cat];
        if (!items || items.length === 0) continue;

        const isCollapsed = _collapsedGroups[cat];
        html += `<div class="sidebar-section" style="cursor:pointer;user-select:none" onclick="toggleGroup('${cat}')">
            ${isCollapsed ? '▶' : '▼'} ${catLabel[cat] || cat}
        </div>`;

        if (!isCollapsed) {
            for (const svc of items) {
                const active = svc.id === selectedId ? ' active' : '';
                html += `
                    <div class="svc-item${active}" onclick="selectService('${svc.id}')">
                        <div class="svc-item-icon" style="background:${iconBg(svc.status)}">${svgIcon(svc.name)}</div>
                        <div class="svc-item-info">
                            <div class="svc-item-name">${svc.name}</div>
                            <div class="svc-item-port">${svc.port ? ':' + svc.port : ''}</div>
                        </div>
                        <div class="status-dot ${statusDotClass(svc.status)}"></div>
                    </div>`;
            }
        }
    }
    if (!html) html = '<div style="padding:20px;text-align:center;color:var(--text-tertiary)">无匹配服务</div>';
    list.innerHTML = html;
}

window.toggleGroup = function(cat) {
    _collapsedGroups[cat] = !_collapsedGroups[cat];
    renderSidebar();
};

window.selectService = function(id) {
    selectedId = id;
    const svc = services.find(s => s.id === id);
    if (svc) { renderSidebar(); renderDetail(svc); }
};

// ========== 详情面板 ==========

function renderDetail(svc) {
    const panel = document.getElementById('detailPanel');
    const st = statusLabel(svc.status);

    document.getElementById('detailName').textContent = svc.name;
    document.getElementById('detailBadge').innerHTML = `<span class="badge-dot"></span>${st}`;
    document.getElementById('detailBadge').className = `status-badge ${statusBadgeClass(svc.status)}`;

    document.getElementById('btnStop').style.display = svc.status === 1 ? 'inline-flex' : 'none';
    document.getElementById('btnStart').style.display = svc.status !== 1 ? 'inline-flex' : 'none';
    document.getElementById('btnRestart').style.display = svc.status === 1 ? 'inline-flex' : 'none';

    const canEdit = !svc.id.startsWith('tomcat') && !svc.id.startsWith('redis') && !svc.id.startsWith('kafka') && !svc.id.startsWith('nacos') && !svc.id.startsWith('nginx') && !svc.id.startsWith('mysql') && !svc.id.startsWith('postgresql') && !svc.id.startsWith('mongodb');

    panel.innerHTML = `
        <div class="detail-header">
            <div class="detail-icon" style="background:${iconBg(svc.status)}">${svgIcon(svc.name, 26)}</div>
            <div>
                <div class="detail-meta-name">${svc.name}</div>
                <div class="detail-meta-sub">
                    <span id="installPathDisplay" style="cursor:pointer;border-bottom:1px dashed var(--text-tertiary)" title="点击修改安装路径" onclick="editInstallPath()">${svc.installPath || '点击设置安装路径'}</span>
                    <button class="log-tiny-btn" style="padding:1px 6px;font-size:10px;vertical-align:middle" onclick="openFolder()" title="打开文件夹">📂</button>
                    ${canEdit ? `<button class="log-tiny-btn" style="padding:1px 6px;font-size:10px" onclick="showEditService()">✏️</button>` : ''}
                    ${svc.pid ? `<span class="meta-sep">·</span><span>PID ${svc.pid}</span>` : ''}
                    <span class="meta-sep">·</span><span>${st}</span>
                </div>
            </div>
        </div>
        <div class="stats-grid">
            <div class="stat-card"><div class="stat-label">端口</div><div class="stat-value">${svc.port || '-'}</div></div>
            <div class="stat-card"><div class="stat-label">PID</div><div class="stat-value" style="font-size:15px">${svc.pid || '-'}</div></div>
            <div class="stat-card"><div class="stat-label">状态</div><div class="stat-value" style="font-size:15px">${st}</div></div>
            <div class="stat-card"><div class="stat-label">分类</div><div class="stat-value" style="font-size:15px">${svc.category}</div></div>
        </div>
        <div class="section-heading">最近日志 <a class="section-link" onclick="showLogs()">查看全部 →</a></div>
        <div class="log-panel">
            <div class="log-toolbar">
                <div class="log-toolbar-left"><div class="log-live-dot"></div>
                    <select id="logSourceSelect" class="log-source-select" onchange="onLogSourceChange()">
                        <option value="__process__">● 实时输出</option>
                    </select>
                </div>
                <div class="log-toolbar-right"><button class="log-tiny-btn" onclick="showLogs()">查看</button></div>
            </div>
            <div class="log-body" id="logPreview"><div style="color:var(--text-tertiary)">加载中...</div></div>
        </div>
        <div class="section-heading" style="margin-top:18px">配置文件 <a class="section-link" onclick="showConfig()">编辑 →</a></div>
        <div class="config-panel">
            <div class="config-toolbar">
                <div class="config-path"><svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 4a1 1 0 0 1 1-1h4l1.5 2H13a1 1 0 0 1 1 1v6a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1V4z"/></svg>配置文件</div>
                <div style="display:flex;gap:4px"><button class="log-tiny-btn" onclick="showConfig()">编辑</button></div>
            </div>
            <div class="config-body" id="configPreview"><div style="color:var(--text-tertiary)">暂无配置文件</div></div>
        </div>`;

    loadLogPreview(svc.id);
    loadConfigPreview(svc.id);
    loadLogSources(svc.id);
}

async function loadLogSources(id) {
    try {
        const sources = await GetServiceLogSources(id);
        const sel = document.getElementById('logSourceSelect');
        if (!sel) return;
        sel.innerHTML = '';
        for (const src of sources) {
            const opt = document.createElement('option');
            opt.value = src.id;
            opt.textContent = src.name;
            if (src.note) {
                opt.disabled = true;
                opt.title = src.note;
            }
            if (src.id === '__process__') {
                if (src.note) {
                    sel.dataset.processNote = src.note;
                } else {
                    opt.selected = true;
                }
            }
            sel.appendChild(opt);
        }
    } catch(e) {}
}

window.onLogSourceChange = async function() {
    const sel = document.getElementById('logSourceSelect');
    const val = sel.value;
    const preview = document.getElementById('logPreview');
    if (!preview) return;

    if (val === '__process__') {
        // 实时输出
        const note = sel.dataset.processNote;
        if (note) {
            preview.innerHTML = '<div style="color:var(--text-tertiary);font-size:11px">' + note + '</div>';
            return;
        }
        preview.innerHTML = '<div style="color:var(--text-tertiary)">加载中...</div>';
        try {
            const json = await GetConsoleLog(selectedId);
            let lines = [];
            try { lines = JSON.parse(json); } catch(e) {}
            preview.innerHTML = renderStructuredLines(lines.slice(-8)) || '<div style="color:var(--text-tertiary)">暂无日志</div>';
        } catch(e) {}
    } else if (val.startsWith('file:')) {
        // 文件：调用 SetWatchLogFile 然后显示
        preview.innerHTML = '<div style="color:var(--text-tertiary)">加载中...</div>';
        const fileName = val.substring(5);
        // 找出文件路径
        try {
            const sources = await GetServiceLogSources(selectedId);
            const src = sources.find(s => s.id === val);
            if (src && src.path) {
                const result = await SetWatchLogFile(selectedId, src.path);
                if (result !== 'ok') {
                    preview.innerHTML = '<div style="color:var(--orange)">切换失败: ' + result + '</div>';
                    return;
                }
            }
            const json = await GetConsoleLog(selectedId);
            let lines = [];
            try { lines = JSON.parse(json); } catch(e) {}
            // 在顶部插入系统提示
            const header = { level: 'INFO', text: '► 已切换到 ' + fileName, time: new Date().toISOString() };
            const allLines = [header, ...lines.slice(-7)];
            preview.innerHTML = renderStructuredLines(allLines);
        } catch(e) {
            preview.innerHTML = '<div style="color:var(--red)">切换失败: ' + e + '</div>';
        }
    }
};

async function loadLogPreview(id) {
    try {
        const sel = document.getElementById('logSourceSelect');
        // 如果有来源选择器，按当前选择加载
        if (sel && sel.value && sel.value !== '__process__') {
            // 文件来源：通过 onLogSourceChange 处理
            await onLogSourceChange();
            return;
        }
        // 默认：实时输出
        const json = await GetConsoleLog(id);
        const el = document.getElementById('logPreview');
        if (el) {
            let lines = [];
            try { lines = JSON.parse(json); } catch(e) {}
            const last8 = lines.slice(-8);
            el.innerHTML = renderStructuredLines(last8) || '<div style="color:var(--text-tertiary)">暂无日志</div>';
        }
    } catch(e) {}
}

async function loadConfigPreview(id) {
    try {
        const files = await GetConfigFiles(id);
        const el = document.getElementById('configPreview');
        if (!el) return;
        if (files.length === 0) { el.innerHTML = '<div style="color:var(--text-tertiary)">暂无配置文件</div>'; return; }
        const content = await ReadConfigFile(id, files[0]);
        el.innerHTML = `<pre style="margin:0;font-family:var(--font-mono);font-size:11.5px;line-height:1.75">${escapeHtml(content.slice(0, 500))}</pre>`;
    } catch(e) {}
}

// ========== 启停 ==========

window.startService = async function() {
    if (!selectedId) return;
    const result = await StartService(selectedId);
    if (!result.success) {
        toast('❌ ' + result.error, 5000);
    }
    await loadServices();
};

window.restartService = async function() {
    if (!selectedId) return;
    toast('正在重启...');
    const result = await RestartService(selectedId);
    if (!result.success) {
        toast('❌ 重启失败: ' + result.error, 5000);
    } else {
        toast('✅ 已重启');
    }
    await loadServices();
};

window.stopService = async function() {
    if (!selectedId) return;
    await StopService(selectedId);
    await loadServices();
    toast('已停止');
};

window.startAllServices = async function() {
    toast('正在启动全部服务...');
    const results = await StartAllServices();
    const failed = results.filter(r => !r.success);
    if (failed.length > 0) {
        toast('❌ ' + failed.length + ' 个启动失败', 5000);
    } else {
        toast('✅ 全部已启动');
    }
    await loadServices();
};

window.stopAllServices = async function() {
    for (const svc of services) {
        if (svc.status === 1) await StopService(svc.id);
    }
    await loadServices();
    toast('已全部停止');
};

// ========== 路径编辑 ==========

window.editInstallPath = function() {
    if (!selectedId) return;
    const svc = services.find(s => s.id === selectedId);
    if (!svc) return;
    document.getElementById('pathModalSvcName').textContent = svc.name;
    document.getElementById('pathInput').value = svc.installPath || '';
    openModal('pathModal');
    setTimeout(() => document.getElementById('pathInput').focus(), 200);
};

window.confirmPathEdit = async function() {
    const path = document.getElementById('pathInput').value.trim();
    if (!path) { toast('路径不能为空'); return; }
    const result = await SetInstallPath(selectedId, path);
    if (result === 'ok') {
        const svc = services.find(s => s.id === selectedId);
        if (svc) svc.installPath = path;
        document.getElementById('installPathDisplay').textContent = path;
        closeModal('pathModal');
        toast('路径已保存');
    } else {
        toast('保存失败: ' + result);
    }
};

window.browseFolder = async function() {
    try {
        const path = await BrowseFolder();
        if (path) document.getElementById('pathInput').value = path;
    } catch(e) { toast('选择失败: ' + e); }
};

window.openFolder = async function() {
    if (!selectedId) return;
    const result = await OpenFolder(selectedId);
    if (result !== 'ok') toast(result);
};

// ========== 添加服务 ==========

window.showAddService = function() {
    document.getElementById('asName').value = '';
    document.getElementById('asDisplayName').value = '';
    document.getElementById('asCategory').value = 'Custom';
    document.getElementById('asPort').value = '';
    document.getElementById('asPath').value = '';
    document.getElementById('asStartCmd').value = '';
    document.getElementById('asStopCmd').value = '';
    document.getElementById('asLogFile').value = '';
    openModal('addServiceModal');
};

window.confirmAddService = async function() {
    const name = document.getElementById('asName').value.trim();
    if (!name) { toast('请输入服务名称'); return; }
    const displayName = document.getElementById('asDisplayName').value.trim() || name;
    const category = document.getElementById('asCategory').value;
    const port = parseInt(document.getElementById('asPort').value) || 0;
    const path = document.getElementById('asPath').value.trim();
    const startCmd = document.getElementById('asStartCmd').value.trim();
    const stopCmd = document.getElementById('asStopCmd').value.trim();
    const logFile = document.getElementById('asLogFile').value.trim();
    try {
        await AddCustomService(name, displayName, category, path, startCmd, stopCmd, logFile, '', '', port);
        closeModal('addServiceModal');
        toast('服务已添加');
        await loadServices();
    } catch(e) { toast('添加失败: ' + e); }
};

// ========== 编辑服务 ==========

window.showEditService = async function() {
    if (!selectedId) return;
    const svc = services.find(s => s.id === selectedId);
    if (!svc) return;
    try {
        const detail = await GetServiceDetail(selectedId);
        if (!detail) { toast('获取服务详情失败'); return; }
        document.getElementById('editModalTitle').textContent = '编辑 - ' + detail.displayName;
        document.getElementById('esName').value = detail.name;
        document.getElementById('esDisplayName').value = detail.displayName;
        document.getElementById('esCategory').value = detail.category;
        document.getElementById('esPort').value = detail.port || '';
        document.getElementById('esPath').value = detail.installPath || '';
        document.getElementById('esStartCmd').value = detail.startCmd || '';
        document.getElementById('esStopCmd').value = detail.stopCmd || '';
        document.getElementById('esLogFile').value = detail.logFile || '';
        // 只有自定义服务可删
        document.getElementById('esDeleteBtn').style.display = detail.isTemplate ? 'none' : 'inline-block';
        openModal('editServiceModal');
    } catch(e) { toast('加载失败: ' + e); }
};

window.confirmEditService = async function() {
    const name = document.getElementById('esName').value.trim();
    if (!name) { toast('名称不能为空'); return; }
    const displayName = document.getElementById('esDisplayName').value.trim() || name;
    const category = document.getElementById('esCategory').value;
    const port = parseInt(document.getElementById('esPort').value) || 0;
    const path = document.getElementById('esPath').value.trim();
    const startCmd = document.getElementById('esStartCmd').value.trim();
    const stopCmd = document.getElementById('esStopCmd').value.trim();
    const logFile = document.getElementById('esLogFile').value.trim();
    const result = await EditCustomService(selectedId, name, displayName, category, path, startCmd, stopCmd, logFile, '', '', port);
    if (result === 'ok') {
        closeModal('editServiceModal');
        toast('已保存');
        await loadServices();
    } else { toast('保存失败: ' + result); }
};

window.deleteCurrentService = async function() {
    if (!confirm('确定要删除此服务吗？')) return;
    const result = await DeleteCustomService(selectedId);
    if (result === 'ok') {
        closeModal('editServiceModal');
        selectedId = null;
        toast('已删除');
        await loadServices();
        document.getElementById('detailPanel').innerHTML = '<div style="padding:40px;text-align:center;color:var(--text-tertiary)">← 请从左侧选择一个服务</div>';
    } else { toast('删除失败: ' + result); }
};

// ========== 设置 ==========

window.showSettings = async function() {
    try {
        const cfg = await GetAppConfig();
        document.getElementById('settingsLang').value = cfg.language || 'zh-Hans';
        document.getElementById('settingsAutoStart').checked = cfg.autoStart || false;
        openModal('settingsModal');
    } catch(e) { toast('加载设置失败: ' + e); }
};

window.saveSettings = async function() {
    const lang = document.getElementById('settingsLang').value;
    const autoStart = document.getElementById('settingsAutoStart').checked;
    const result = await SetAppConfig(lang, autoStart);
    if (result === 'ok') {
        closeModal('settingsModal');
        toast('设置已保存' + (lang !== 'zh-Hans' && lang !== 'en' ? '' : ''));
        await loadServices();
    } else { toast('保存失败: ' + result); }
};

// ========== 配置模态框 ==========

window.showConfig = async function() {
    if (!selectedId) return;
    const svc = services.find(s => s.id === selectedId);
    document.getElementById('configModalTitle').textContent = `配置文件 - ${svc.name}`;
    const select = document.getElementById('configFileSelect');
    select.innerHTML = '<option>加载中...</option>';
    document.getElementById('configEditor').value = '';
    try {
        const files = await GetConfigFiles(selectedId);
        select.innerHTML = files.map(f => `<option value="${f}">${f}</option>`).join('') || '<option>无配置文件</option>';
        if (files.length > 0) await loadConfigFileContent(files[0]);
    } catch(e) {}
    openModal('configModal');
};

window.loadConfigFile = async function() {
    const file = document.getElementById('configFileSelect').value;
    if (file) await loadConfigFileContent(file);
};

async function loadConfigFileContent(file) {
    document.getElementById('configFilePath').textContent = file;
    try {
        document.getElementById('configEditor').value = await ReadConfigFile(selectedId, file);
    } catch(e) { document.getElementById('configEditor').value = '// Error: ' + e; }
}

window.saveConfigFile = async function() {
    const file = document.getElementById('configFileSelect').value;
    if (!file) return;
    const content = document.getElementById('configEditor').value;
    const result = await SaveConfigFile(selectedId, file, content);
    if (result === 'ok') {
        closeModal('configModal');
        toast(services.find(s=>s.id===selectedId)?.status===1 ? '配置已保存，需重启生效' : '配置已保存');
    } else { toast('保存失败: ' + result); }
};

// ========== 日志模态框 ==========

let _logFileGroups = null;
let _logSearchActive = false;
let _logViewMode = 'watch'; // 'watch' 或 'file'

window.showLogs = async function() {
    if (!selectedId) return;
    const svc = services.find(s => s.id === selectedId);
    document.getElementById('logModalTitle').textContent = `日志 - ${svc.name}`;
    _autoScrollLog = true;
    _logSearchActive = false;
    _logViewMode = 'watch';
    openModal('logModal');

    try {
        // 加载已保存的日志路径
        const savedPath = await GetWatchLogFile(selectedId);
        document.getElementById('logWatchPath').value = savedPath || '';

        // 加载 collector 缓冲区
        const consoleJson = await GetConsoleLog(selectedId);
        const logView = document.getElementById('logContent');
        let logLines = [];
        try { logLines = JSON.parse(consoleJson); } catch(e) {}
        logView.innerHTML = renderStructuredLines(logLines) || '<div class="log-line"><span class="log-col-msg" style="color:var(--text-tertiary)">尚无实时日志，请在上方输入日志文件路径后按回车</span></div>';

        // 加载分组日志文件列表
        _logFileGroups = await GetLogGroupedFiles(selectedId);
        const typeSelect = document.getElementById('logTypeSelect');
        const dateSelect = document.getElementById('logDateSelect');
        typeSelect.innerHTML = '<option value="">选择日志...</option>';
        dateSelect.innerHTML = '<option value="">选择日期...</option>';
        dateSelect.disabled = true;
        if (_logFileGroups) {
            const types = Object.keys(_logFileGroups);
            for (const t of types) {
                const opt = document.createElement('option');
                opt.value = t;
                opt.textContent = t;
                typeSelect.appendChild(opt);
            }
        }

        // watch indicator 亮绿表示实时模式
        document.getElementById('logWatchIndicator').style.color = '#22c55e';
        await StreamLog(selectedId);
    } catch(e) {
        document.getElementById('logContent').innerHTML = '<pre class="log-pre">// Error: ' + e + '</pre>';
    }
};

window.switchToWatch = async function() {
    _logViewMode = 'watch';
    document.getElementById('logWatchIndicator').style.color = '#22c55e';
    document.getElementById('logTypeSelect').value = '';
    document.getElementById('logDateSelect').value = '';
    document.getElementById('logDateSelect').disabled = true;
    const logView = document.getElementById('logContent');
    logView.innerHTML = '<div class="log-line"><span class="log-col-msg" style="color:var(--text-tertiary)">加载中...</span></div>';
    try {
        const consoleJson = await GetConsoleLog(selectedId);
        let logLines = [];
        try { logLines = JSON.parse(consoleJson); } catch(e) {}
        logView.innerHTML = renderStructuredLines(logLines) || '<div class="log-line"><span class="log-col-msg" style="color:var(--text-tertiary)">尚无实时日志</span></div>';
    } catch(e) {
        logView.innerHTML = '<pre class="log-pre">// Error: ' + e + '</pre>';
    }
};

window.setWatchPath = async function() {
    const path = document.getElementById('logWatchPath').value.trim();
    if (!path) return;
    const result = await SetWatchLogFile(selectedId, path);
    if (result !== 'ok') {
        toast(result);
        return;
    }
    toast('已开始实时监听: ' + path);
    _logViewMode = 'watch';
    document.getElementById('logWatchIndicator').style.color = '#22c55e';
    document.getElementById('logTypeSelect').value = '';
    document.getElementById('logDateSelect').value = '';
    document.getElementById('logDateSelect').disabled = true;
    // 清空并重新加载
    const consoleJson = await GetConsoleLog(selectedId);
    const logView = document.getElementById('logContent');
    let logLines = [];
    try { logLines = JSON.parse(consoleJson); } catch(e) {}
    logView.innerHTML = renderStructuredLines(logLines) || '<div class="log-line"><span class="log-col-msg" style="color:var(--text-tertiary)">暂无内容</span></div>';
};

window.browseWatchPath = async function() {
    try {
        const folder = await BrowseFolder();
        if (folder) {
            // BrowseFolder 返回文件夹路径，追加常见的日志文件名
            document.getElementById('logWatchPath').value = folder;
        }
    } catch(e) {
        toast('选择失败: ' + e);
    }
};

window.onLogTypeChange = function() {
    _logViewMode = 'file';
    document.getElementById('logWatchIndicator').style.color = '';
    const type = document.getElementById('logTypeSelect').value;
    const dateSelect = document.getElementById('logDateSelect');
    if (!type || !_logFileGroups || !_logFileGroups[type]) {
        dateSelect.innerHTML = '<option value="">选择日期...</option>';
        dateSelect.disabled = true;
        return;
    }
    const entries = _logFileGroups[type];
    const dates = [...new Set(entries.map(e => e.date))].filter(Boolean);

    if (dates.length === 0) {
        dateSelect.innerHTML = '<option value="">---</option>';
        dateSelect.disabled = true;
        loadLogFileContent(entries[0].name);
        return;
    }

    dateSelect.innerHTML = '<option value="">选择日期...</option>';
    for (const d of dates) {
        const opt = document.createElement('option');
        opt.value = d;
        opt.textContent = d;
        dateSelect.appendChild(opt);
    }
    dateSelect.disabled = false;
    dateSelect.value = dates[0];
    onLogDateChange();
};

async function loadLogFileContent(fileName) {
    const logView = document.getElementById('logContent');
    logView.innerHTML = '<div class="log-line"><span class="log-col-msg" style="color:var(--text-tertiary)">加载中...</span></div>';
    try {
        const content = await GetLogFileContent(selectedId, fileName);
        logView.innerHTML = formatLogContent(content) || '<div class="log-line"><span class="log-col-msg" style="color:var(--text-tertiary)">暂无日志</span></div>';
    } catch(e) {
        logView.innerHTML = '<pre class="log-pre">// Error: ' + e + '</pre>';
    }
}

window.onLogDateChange = async function() {
    const type = document.getElementById('logTypeSelect').value;
    const date = document.getElementById('logDateSelect').value;
    if (!type || !date) return;

    _logViewMode = 'file';
    document.getElementById('logWatchIndicator').style.color = '';

    let fileName = '';
    if (_logFileGroups && _logFileGroups[type]) {
        const entry = _logFileGroups[type].find(e => e.date === date);
        if (entry) fileName = entry.name;
    }
    if (!fileName) return;

    await loadLogFileContent(fileName);
};

window.toggleLogScroll = function() {
    _autoScrollLog = !_autoScrollLog;
    const btn = document.getElementById('logScrollBtn');
    btn.classList.toggle('active', _autoScrollLog);
};

window.clearLogContent = function() {
    document.getElementById('logContent').innerHTML = '<div class="log-line"><span class="log-col-msg" style="color:var(--text-tertiary)">已清空</span></div>';
    ClearConsoleLog(selectedId);
};

window.toggleSearch = function() {
    _logSearchActive = !_logSearchActive;
    document.getElementById('logSearchBar').style.display = _logSearchActive ? 'block' : 'none';
    if (_logSearchActive) {
        document.getElementById('logSearchInput').focus();
    } else {
        document.getElementById('logSearchInput').value = '';
        onLogSearch();
    }
};

let _searchTimer2 = null;
window.onLogSearch = function() {
    clearTimeout(_searchTimer2);
    _searchTimer2 = setTimeout(() => {
        const kw = document.getElementById('logSearchInput').value.toLowerCase();
        const logView = document.getElementById('logContent');
        const lines = logView.querySelectorAll('.log-line');
        lines.forEach(line => {
            if (!kw) {
                line.style.display = '';
                return;
            }
            line.style.display = line.textContent.toLowerCase().includes(kw) ? '' : 'none';
        });
    }, 200);
};

// ========== Profile ==========

window.showProfiles = async function() {
    if (!selectedId) return;
    const svc = services.find(s => s.id === selectedId);
    document.getElementById('profileSvcName').textContent = svc.name;
    document.getElementById('profileArgsInput').value = svc.args || '';
    await refreshProfileList();
    openModal('profileModal');
};

async function refreshProfileList() {
    const list = document.getElementById('profileList');
    list.innerHTML = '<div style="color:var(--text-tertiary);font-size:12px">加载中...</div>';
    try {
        const profiles = await GetStartProfiles(selectedId);
        const names = Object.keys(profiles);
        if (names.length === 0) { list.innerHTML = '<div style="color:var(--text-tertiary);font-size:12px">暂无保存的配置</div>'; return; }
        list.innerHTML = names.map(name => `
            <div style="display:flex;align-items:center;justify-content:space-between;padding:6px 8px;background:var(--bg-page);border-radius:var(--radius-md)">
                <div style="flex:1;min-width:0">
                    <div style="font-size:12px;font-weight:500;color:var(--text-primary)">${name}</div>
                    <div style="font-size:10.5px;color:var(--text-tertiary);font-family:var(--font-mono);white-space:nowrap;overflow:hidden;text-overflow:ellipsis">${profiles[name] || '(空)'}</div>
                </div>
                <div style="display:flex;gap:4px;flex-shrink:0;margin-left:8px">
                    <button class="log-tiny-btn" onclick='loadProfile("${name}", ${JSON.stringify(profiles[name])})'>加载</button>
                    <button class="log-tiny-btn" onclick='deleteProfile("${name}")' style="color:var(--red)">删除</button>
                </div>
            </div>`).join('');
    } catch(e) { list.innerHTML = '<div style="color:var(--red);font-size:12px">加载失败</div>'; }
}

window.loadProfile = function(name, args) {
    document.getElementById('profileArgsInput').value = args || '';
    toast('已加载: ' + name);
};

window.saveCurrentProfile = async function() {
    const name = prompt('请输入配置名称:', '');
    if (!name) return;
    await SaveStartProfile(selectedId, name, document.getElementById('profileArgsInput').value);
    toast('已保存: ' + name);
    await refreshProfileList();
};

window.deleteProfile = async function(name) {
    await DeleteStartProfile(selectedId, name);
    toast('已删除: ' + name);
    await refreshProfileList();
};

window.applyProfileArgs = async function() {
    const args = document.getElementById('profileArgsInput').value;
    await SetStartArgs(selectedId, args);
    const svc = services.find(s => s.id === selectedId);
    if (svc) svc.args = args;
    closeModal('profileModal');
    toast('参数已应用');
};

// ========== 状态栏 ==========

function renderStatusBar() {
    const r = services.filter(s => s.status === 1).length;
    const st = services.filter(s => s.status === 0).length;
    const e = services.filter(s => s.status === 2).length;
    document.getElementById('statRunning').textContent = `${r} 运行中`;
    document.getElementById('statStopped').textContent = `${st} 已停止`;
    document.getElementById('statError').textContent = `${e} 异常`;
    document.getElementById('statTime').textContent = new Date().toLocaleTimeString();
}

// ========== 模态框控制 ==========

function openModal(id) { document.getElementById(id).classList.add('show'); }
window.closeModal = function(id) { document.getElementById(id).classList.remove('show'); };

document.addEventListener('click', function(e) {
    document.querySelectorAll('.modal-overlay.show').forEach(m => {
        if (e.target === m) m.classList.remove('show');
    });
});

document.addEventListener('keydown', function(e) {
    if (e.key === 'Enter') {
        if (document.getElementById('pathModal').classList.contains('show')) confirmPathEdit();
        else if (document.getElementById('profileModal').classList.contains('show')) applyProfileArgs();
    }
});

// ========== 辅助 ==========

function statusDotClass(s) {
    return s === 1 ? 'dot-green' : s === 2 ? 'dot-red' : (s === 3 || s === 4) ? 'dot-orange' : 'dot-gray';
}
function statusBadgeClass(s) {
    return s === 1 ? 'badge-running' : s === 2 ? 'badge-error' : (s === 3 || s === 4) ? 'badge-starting' : 'badge-stopped';
}
function statusLabel(s) {
    return s === 1 ? '运行中' : s === 2 ? '异常' : s === 3 ? '启动中...' : s === 4 ? '停止中...' : '已停止';
}
function iconBg(s) {
    return s === 1 ? '#EEF7EE' : s === 2 ? '#FEF2F2' : '#F3F0FF';
}
function svgIcon(name, size) {
    size = size || 14;
    return `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M21 12c0 1.66-4.03 3-9 3S3 13.66 3 12"/><path d="M3 5v14c0 1.66 4.03 3 9 3s9-1.34 9-3V5"/></svg>`;
}
function escapeHtml(str) {
    if (!str) return '';
    return str.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}
