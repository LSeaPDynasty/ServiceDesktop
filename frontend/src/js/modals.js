// modals.js — 模态框：路径编辑、添加/编辑/删除服务、设置

import { toast } from './utils.js';
import { SetInstallPath, AddCustomService, BrowseFolder, OpenFolder, GetAppConfig, SetAppConfig, GetServiceDetail, EditCustomService, DeleteCustomService } from '../../wailsjs/go/main/App.js';

// ---------- 路径编辑 ----------

export function editInstallPath(svc) {
    if (!svc) return;
    document.getElementById('pathModalSvcName').textContent = svc.name;
    document.getElementById('pathInput').value = svc.installPath || '';
    openModal('pathModal');
    setTimeout(() => document.getElementById('pathInput').focus(), 200);
}

export async function confirmPathEdit(svcId) {
    const path = document.getElementById('pathInput').value.trim();
    if (!path) { toast('路径不能为空'); return; }
    const result = await SetInstallPath(svcId, path);
    if (result === 'ok') {
        closeModal('pathModal');
        toast('路径已保存');
        window.loadServices();
    } else { toast('保存失败: ' + result); }
}

export async function browseFolder() {
    try {
        const path = await BrowseFolder();
        if (path) document.getElementById('pathInput').value = path;
    } catch(e) { toast('选择失败: ' + e); }
}

export function openFolderCmd(svcId) {
    if (!svcId) return;
    OpenFolder(svcId).then(r => { if (r !== 'ok') toast(r); });
}

// ---------- 添加服务 ----------

export function showAddService() {
    document.getElementById('asName').value = '';
    document.getElementById('asDisplayName').value = '';
    document.getElementById('asCategory').value = 'Custom';
    document.getElementById('asPort').value = '';
    document.getElementById('asPath').value = '';
    document.getElementById('asStartCmd').value = '';
    document.getElementById('asStopCmd').value = '';
    document.getElementById('asLogFile').value = '';
    openModal('addServiceModal');
}

export async function confirmAddService() {
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
        window.loadServices();
    } catch(e) { toast('添加失败: ' + e); }
}

// ---------- 编辑服务 ----------

export async function showEditService(svcId) {
    if (!svcId) return;
    try {
        const detail = await GetServiceDetail(svcId);
        if (!detail) { toast('无法获取服务详情'); return; }
        document.getElementById('esName').value = detail.name || '';
        document.getElementById('esDisplayName').value = detail.displayName || '';
        document.getElementById('esCategory').value = detail.category || 'Custom';
        document.getElementById('esPort').value = detail.port || '';
        document.getElementById('esPath').value = detail.installPath || '';
        document.getElementById('esStartCmd').value = detail.startCmd || '';
        document.getElementById('esStopCmd').value = detail.stopCmd || '';
        document.getElementById('esLogFile').value = detail.logFile || '';
        openModal('editServiceModal');
    } catch(e) { toast('加载服务详情失败: ' + e); }
}

export async function confirmEditService(svcId) {
    if (!svcId) return;
    const name = document.getElementById('esName').value.trim();
    const displayName = document.getElementById('esDisplayName').value.trim() || name;
    const category = document.getElementById('esCategory').value;
    const port = parseInt(document.getElementById('esPort').value) || 0;
    const path = document.getElementById('esPath').value.trim();
    const startCmd = document.getElementById('esStartCmd').value.trim();
    const stopCmd = document.getElementById('esStopCmd').value.trim();
    const logFile = document.getElementById('esLogFile').value.trim();
    const result = await EditCustomService(svcId, name, displayName, category, path, startCmd, stopCmd, logFile, '', '', port);
    if (result === 'ok') { closeModal('editServiceModal'); toast('已保存'); window.loadServices(); }
    else { toast('保存失败: ' + result); }
}

export async function deleteCurrentService(svcId) {
    if (!svcId || !confirm('确定要删除此服务吗？')) return;
    const result = await DeleteCustomService(svcId);
    if (result === 'ok') {
        closeModal('editServiceModal');
        toast('已删除');
        window.loadServices();
        document.getElementById('detailPanel').innerHTML = '<div style="padding:40px;text-align:center;color:var(--text-tertiary)">← 请从左侧选择一个服务</div>';
    } else { toast('删除失败: ' + result); }
}

// ---------- 设置 ----------

export async function showSettings() {
    try {
        const cfg = await GetAppConfig();
        document.getElementById('settingsLang').value = cfg.language || 'zh-Hans';
        document.getElementById('settingsAutoStart').checked = cfg.autoStart || false;
        openModal('settingsModal');
    } catch(e) { toast('加载设置失败: ' + e); }
}

export async function saveSettings() {
    const lang = document.getElementById('settingsLang').value;
    const autoStart = document.getElementById('settingsAutoStart').checked;
    const result = await SetAppConfig(lang, autoStart);
    if (result === 'ok') { closeModal('settingsModal'); toast('设置已保存'); window.loadServices(); }
    else { toast('保存失败: ' + result); }
}

// ---------- 服务专属配置面板路由 ----------

export async function showServiceConfig(svcId, svcName) {
    if (!svcId) return;
    const pluginId = svcId.split('-')[0]; // tomcat-xxx → tomcat
    try {
        const mod = await import(`./services/${pluginId}.js`);
        if (mod && mod.renderConfigPanel) {
            const container = document.getElementById('detailPanel');
            if (container) mod.renderConfigPanel(container, { id: svcId, name: svcName, installPath: '' });
            return;
        }
    } catch(e) { /* 无专属面板 */ }
    toast('该服务无专属配置面板');
}

function openModal(id) { document.getElementById(id).classList.add('show'); }
function closeModal(id) { document.getElementById(id).classList.remove('show'); }
