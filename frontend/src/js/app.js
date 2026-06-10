// app.js — 应用入口，整合所有模块

import { toast, statusLabel, renderStructuredLines, formatLogContent, escapeHtml } from './utils.js';
import { renderSidebar, filterServices, toggleGroup } from './sidebar.js';
import { renderDetail, getSelectedId, setSelectedId, getServices, setServices, onLogSourceChange, doStartService, doStopService, doRestartService, doStartAllServices, doStopAllServices } from './detail.js';
import { showLogs, switchToWatch, setWatchPath, onLogTypeChange, loadLogFileDate, getLogViewMode, getAutoScrollLog, setAutoScrollLog } from './log.js';
import { showConfig, saveConfigFile } from './config.js';
import { editInstallPath, confirmPathEdit, browseFolder, openFolder, showAddService, confirmAddService, showEditService, confirmEditService, deleteCurrentService, showSettings, saveSettings, showServiceConfig } from './modals.js';
import { GetServices } from '../wailsjs/go/main/App.js';
import { EventsOn } from '../wailsjs/runtime/runtime.js';

let services = [];
let selectedId = null;

async function loadServices() {
    try {
        services = await GetServices();
        setServices(services);
        renderSidebar(services, selectedId);
        // renderStatusBar 暂不拆分，内联
        updateStatusBar();
        if (selectedId) {
            const found = services.find(s => s.id === selectedId);
            if (found) renderDetail(found);
        }
    } catch(e) { console.error('Failed to load services:', e); }
}

function updateStatusBar() {
    const running = services.filter(s => s.status === 1).length;
    const total = services.length;
    const el = document.getElementById('statusBar');
    if (el) el.textContent = `${running}/${total} 运行中`;
}

// 事件监听
window.addEventListener('DOMContentLoaded', async () => {
    await loadServices();

    EventsOn('services-update', () => loadServices());
    EventsOn('log-update', function(data) {
        if (data && data.id === selectedId) {
            const el = document.getElementById('logContent');
            const preview = document.getElementById('logPreview');
            if (data.lines) {
                try {
                    const logLines = JSON.parse(data.lines);
                    if (el && document.getElementById('logModal').classList.contains('show') && getLogViewMode() === 'watch') {
                        el.innerHTML = renderStructuredLines(logLines);
                        if (getAutoScrollLog()) el.scrollTop = el.scrollHeight;
                    }
                    if (preview && logLines.length > 0) {
                        const last8 = logLines.slice(-8);
                        preview.innerHTML = renderStructuredLines(last8);
                    }
                } catch(e) {}
            }
        }
    });

    // sidebar 事件
    window.addEventListener('sidebar-refresh', () => renderSidebar(services, selectedId));
    window.addEventListener('service-select', e => {
        selectedId = e.detail;
        setSelectedId(selectedId);
        const svc = services.find(s => s.id === selectedId);
        if (svc) { renderSidebar(services, selectedId); renderDetail(svc); }
    });
});

// 导出到 window 供 HTML onclick 使用
window.loadServices = loadServices;
window.refreshAll = loadServices;
window.filterServices = filterServices;
window.toggleGroup = toggleGroup;
window.selectService = function(id) {
    selectedId = id;
    setSelectedId(selectedId);
    const svc = services.find(s => s.id === id);
    if (svc) { renderSidebar(services, selectedId); renderDetail(svc); }
};

window.startService = doStartService;
window.stopService = doStopService;
window.restartService = doRestartService;
window.startAllServices = doStartAllServices;
window.stopAllServices = doStopAllServices;

window.editInstallPath = editInstallPath;
window.confirmPathEdit = confirmPathEdit;
window.browseFolder = browseFolder;
window.openFolder = openFolder;

window.showAddService = showAddService;
window.confirmAddService = confirmAddService;
window.showEditService = showEditService;
window.confirmEditService = confirmEditService;
window.deleteCurrentService = deleteCurrentService;

window.showSettings = showSettings;
window.saveSettings = saveSettings;

window.showLogs = () => { const svc = services.find(s=>s.id===selectedId); if(svc) showLogs(selectedId, svc.name); };
window.showConfig = () => { const svc = services.find(s=>s.id===selectedId); if(svc) showConfig(selectedId, svc.name); };
window.switchToWatch = () => switchToWatch(selectedId);
window.setWatchPath = () => setWatchPath(selectedId);
window.onLogTypeChange = onLogTypeChange;
window.loadLogFileDate = () => loadLogFileDate(selectedId);
window.onLogSourceChange = onLogSourceChange;
window.saveConfigFile = () => saveConfigFile(selectedId);
window.showServiceConfig = () => { if(selectedId) showServiceConfig(selectedId); };

window.QuitApp = async () => {
    const { QuitApp } = await import('../wailsjs/go/main/App.js');
    QuitApp();
};

// 模态框辅助
window.openModal = function(id) { document.getElementById(id).classList.add('show'); };
window.closeModal = function(id) { document.getElementById(id).classList.remove('show'); };
