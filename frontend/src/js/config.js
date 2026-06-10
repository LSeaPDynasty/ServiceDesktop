// config.js — 配置文件编辑模态框

import { toast, escapeHtml } from './utils.js';
import { GetConfigFiles, ReadConfigFile, SaveConfigFile } from '../../wailsjs/go/main/App.js';

export async function showConfig(serviceId, serviceName) {
    document.getElementById('configModalTitle').textContent = `配置文件 - ${serviceName}`;
    const select = document.getElementById('configFileSelect');
    select.innerHTML = '<option>加载中...</option>';
    document.getElementById('configEditor').value = '';
    try {
        const files = await GetConfigFiles(serviceId);
        select.innerHTML = files.map(f => `<option value="${f}">${f}</option>`).join('') || '<option>无配置文件</option>';
        select.onchange = () => loadConfigFile(serviceId);
        if (files.length > 0) await loadConfigFileContent(serviceId, files[0]);
    } catch(e) {}
    document.getElementById('configModal').classList.add('show');
}

async function loadConfigFile(serviceId) {
    const file = document.getElementById('configFileSelect').value;
    if (file) await loadConfigFileContent(serviceId, file);
}

async function loadConfigFileContent(serviceId, file) {
    document.getElementById('configFilePath').textContent = file;
    try {
        document.getElementById('configEditor').value = await ReadConfigFile(serviceId, file);
    } catch(e) { document.getElementById('configEditor').value = '// Error: ' + e; }
}

export async function saveConfigFile(serviceId) {
    const file = document.getElementById('configFileSelect').value;
    if (!file) return;
    const content = document.getElementById('configEditor').value;
    const result = await SaveConfigFile(serviceId, file, content);
    if (result === 'ok') {
        document.getElementById('configModal').classList.remove('show');
        toast('配置已保存');
    } else { toast('保存失败: ' + result); }
}
