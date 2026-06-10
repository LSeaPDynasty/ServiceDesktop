// detail.js — 服务详情面板 + 启停操作 + 日志/配置预览

import { statusLabel, statusBadgeClass, iconBg, svgIcon, toast, renderStructuredLines, escapeHtml } from './utils.js';
import { GetConsoleLog, GetConfigFiles, ReadConfigFile, GetServiceLogSources, SetWatchLogFile } from '../../wailsjs/go/main/App.js';

let _selectedId = null;
let _services = [];

export function setSelectedId(id) { _selectedId = id; }
export function getSelectedId() { return _selectedId; }
export function setServices(s) { _services = s; }
export function getServices() { return _services; }

export function renderDetail(svc) {
    _selectedId = svc.id; // PATCH: ensure selectedId is set on initial load
    const st = statusLabel(svc.status);

    document.getElementById('detailName').textContent = svc.name;
    document.getElementById('detailBadge').innerHTML = `<span class="badge-dot"></span>${st}`;
    document.getElementById('detailBadge').className = `status-badge ${statusBadgeClass(svc.status)}`;

    document.getElementById('btnStop').style.display = svc.status === 1 ? 'inline-flex' : 'none';
    document.getElementById('btnStart').style.display = svc.status !== 1 ? 'inline-flex' : 'none';
    document.getElementById('btnRestart').style.display = svc.status === 1 ? 'inline-flex' : 'none';

    // 所有服务都可编辑参数；自定义/发现服务可编辑完整配置
    const builtinIds = ['tomcat','redis','kafka','nacos','nginx','mysql','postgresql','mongodb'];
    const isTemplate = builtinIds.some(id => svc.id === id || svc.id.startsWith(id + '-'));

    const panel = document.getElementById('detailPanel');
    panel.innerHTML = `
        <div class="detail-header">
            <div class="detail-icon" style="background:${iconBg(svc.status)}">${svgIcon(svc.name, 26)}</div>
            <div>
                <div class="detail-meta-name">${svc.name}</div>
                <div class="detail-meta-sub">
                    <span id="installPathDisplay" style="cursor:pointer;border-bottom:1px dashed var(--text-tertiary)" title="点击修改安装路径">${svc.installPath || '点击设置安装路径'}</span>
                    <button class="log-tiny-btn" style="padding:1px 6px;font-size:10px;vertical-align:middle" id="btnOpenFolder" title="打开文件夹">📂</button>
                    ${!isTemplate ? `<button class="log-tiny-btn" style="padding:1px 6px;font-size:10px" id="btnEditService">✏️</button>` : ''}
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
        <div class="section-heading">启动参数 <button class="log-tiny-btn" style="margin-left:8px;font-size:11px" id="btnSaveArgs">保存</button></div>
        <div style="padding:0 18px 6px">
            <input type="text" id="argsInput" style="width:100%;padding:6px 10px;font-family:var(--font-mono);font-size:12px;border:1px solid var(--border);border-radius:4px;background:var(--bg-card);color:var(--text)" value="${escapeHtml(svc.args || '')}" placeholder="例如: -m standalone --port=8080">
        </div>
        <div class="section-heading">最近日志 <a class="section-link" id="linkShowLogs">查看全部 →</a></div>
        <div class="log-panel">
            <div class="log-toolbar">
                <div class="log-toolbar-left"><div class="log-live-dot"></div>
                    <select id="logSourceSelect" class="log-source-select"></select>
                </div>
                <div class="log-toolbar-right"><button class="log-tiny-btn" id="btnShowLogs">查看</button></div>
            </div>
            <div class="log-body" id="logPreview"><div style="color:var(--text-tertiary)">加载中...</div></div>
        </div>
        <div class="section-heading" style="margin-top:18px">配置文件 <a class="section-link" id="linkShowConfig">编辑 →</a></div>
        <div class="config-panel">
            <div class="config-toolbar">
                <div class="config-path"><svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 4a1 1 0 0 1 1-1h4l1.5 2H13a1 1 0 0 1 1 1v6a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1V4z"/></svg>配置文件</div>
                <div style="display:flex;gap:4px"><button class="log-tiny-btn" id="btnShowConfig">编辑</button></div>
            </div>
            <div class="config-body" id="configPreview"><div style="color:var(--text-tertiary)">暂无配置文件</div></div>
        </div>`;

    // 绑定事件（用闭包捕获当前服务 ID）
    const svcId = svc.id;
    document.getElementById('installPathDisplay').onclick = () => window.editInstallPath(svc);
    document.getElementById('btnOpenFolder').onclick = () => window.openFolder(svcId);
    const btnEdit = document.getElementById('btnEditService');
    if (btnEdit) btnEdit.onclick = () => window.showEditService(svcId);
    document.getElementById('btnSaveArgs').onclick = async () => {
        const args = document.getElementById('argsInput').value;
        const { SetStartArgs } = await import('../../wailsjs/go/main/App.js');
        const result = await SetStartArgs(svcId, args);
        if (result === 'ok') toast('启动参数已保存');
        else toast('保存失败: ' + result);
    };
    document.getElementById('linkShowLogs').onclick = () => window.showLogs();
    document.getElementById('btnShowLogs').onclick = () => window.showLogs();
    document.getElementById('linkShowConfig').onclick = () => window.showConfig();
    document.getElementById('btnShowConfig').onclick = () => window.showConfig();

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
            if (src.note) { opt.disabled = true; opt.title = src.note; }
            if (src.id === '__process__') {
                if (src.note) { sel.dataset.processNote = src.note; }
                else { opt.selected = true; }
            }
            sel.appendChild(opt);
        }
        sel.onchange = () => onLogSourceChange();
    } catch(e) {}
}

export async function onLogSourceChange() {
    const sel = document.getElementById('logSourceSelect');
    if (!sel) return;
    const val = sel.value;
    const preview = document.getElementById('logPreview');
    if (!preview) return;

    if (val === '__process__') {
        const note = sel.dataset.processNote;
        if (note) { preview.innerHTML = '<div style="color:var(--text-tertiary);font-size:11px">' + note + '</div>'; return; }
        preview.innerHTML = '<div style="color:var(--text-tertiary)">加载中...</div>';
        try {
            const json = await GetConsoleLog(_selectedId);
            let lines = [];
            try { lines = JSON.parse(json); } catch(e) {}
            preview.innerHTML = renderStructuredLines(lines.slice(-8)) || '<div style="color:var(--text-tertiary)">暂无日志</div>';
        } catch(e) {}
    } else if (val && val.startsWith('file:')) {
        preview.innerHTML = '<div style="color:var(--text-tertiary)">加载中...</div>';
        try {
            const sources = await GetServiceLogSources(_selectedId);
            const src = sources.find(s => s.id === val);
            if (src && src.path) {
                const result = await SetWatchLogFile(_selectedId, src.path);
                if (result !== 'ok') { preview.innerHTML = '<div style="color:var(--orange)">切换失败: ' + result + '</div>'; return; }
            }
            const json = await GetConsoleLog(_selectedId);
            let lines = [];
            try { lines = JSON.parse(json); } catch(e) {}
            const header = { level: 'INFO', text: '► 已切换到文件', time: new Date().toISOString() };
            preview.innerHTML = renderStructuredLines([header, ...lines.slice(-7)]);
        } catch(e) { preview.innerHTML = '<div style="color:var(--red)">切换失败: ' + e + '</div>'; }
    }
}

async function loadLogPreview(id) {
    try {
        const sel = document.getElementById('logSourceSelect');
        if (sel && sel.value && sel.value !== '__process__') { await onLogSourceChange(); return; }
        const json = await GetConsoleLog(id);
        const el = document.getElementById('logPreview');
        if (el) {
            let lines = [];
            try { lines = JSON.parse(json); } catch(e) {}
            el.innerHTML = renderStructuredLines(lines.slice(-8)) || '<div style="color:var(--text-tertiary)">暂无日志</div>';
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

// 启停操作
export async function doStartService() {
    if (!_selectedId) return;
    const btn = document.getElementById('btnStart');
    if (btn) btn.disabled = true;
    try {
        const { StartService } = await import('../../wailsjs/go/main/App.js');
        const result = await StartService(_selectedId);
        if (!result.success) toast('❌ ' + result.error, 5000);
    } finally {
        if (btn) btn.disabled = false;
    }
    window.loadServices();
}

export async function doStopService() {
    if (!_selectedId) return;
    const btn = document.getElementById('btnStop');
    if (btn) btn.disabled = true;
    try {
        const { StopService } = await import('../../wailsjs/go/main/App.js');
        await StopService(_selectedId);
    } finally {
        if (btn) btn.disabled = false;
    }
    window.loadServices();
    toast('已停止');
}

export async function doRestartService() {
    if (!_selectedId) return;
    const btn = document.getElementById('btnRestart');
    if (btn) btn.disabled = true;
    try {
        toast('正在重启...');
        const { RestartService } = await import('../../wailsjs/go/main/App.js');
        const result = await RestartService(_selectedId);
        if (!result.success) toast('❌ 重启失败: ' + result.error, 5000);
        else toast('✅ 已重启');
    } finally {
        if (btn) btn.disabled = false;
    }
    window.loadServices();
}

export async function doStartAllServices() {
    toast('正在启动全部服务...');
    const { StartAllServices } = await import('../../wailsjs/go/main/App.js');
    const results = await StartAllServices();
    const failed = results.filter(r => !r.success);
    toast(failed.length > 0 ? '❌ ' + failed.length + ' 个启动失败' : '✅ 全部已启动', 5000);
    window.loadServices();
}

export async function doStopAllServices() {
    const { StopService } = await import('../../wailsjs/go/main/App.js');
    for (const svc of _services) {
        if (svc.status === 1) await StopService(svc.id);
    }
    window.loadServices();
    toast('已全部停止');
}
