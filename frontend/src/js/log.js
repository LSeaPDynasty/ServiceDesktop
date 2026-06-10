// log.js — 日志查看模态框

import { renderStructuredLines, escapeHtml, toast } from './utils.js';
import { GetLogContent, GetLogFiles, GetLogFileContent, GetLogGroupedFiles, GetConsoleLog, ClearConsoleLog, SetWatchLogFile, GetWatchLogFile, GetServiceLogSources, StreamLog } from '../../wailsjs/go/main/App.js';

let _logFileGroups = null;
let _logViewMode = 'watch';
let _autoScrollLog = true;

export function initLogModal() { /* 保持变量引用，由外部设置 */ }

export async function showLogs(serviceId, serviceName) {
    _logViewMode = 'watch';
    _autoScrollLog = true;
    document.getElementById('logModalTitle').textContent = `日志 - ${serviceName}`;
    openModal('logModal');

    try {
        const savedPath = await GetWatchLogFile(serviceId);
        document.getElementById('logWatchPath').value = savedPath || '';

        const consoleJson = await GetConsoleLog(serviceId);
        const logView = document.getElementById('logContent');
        let logLines = [];
        try { logLines = JSON.parse(consoleJson); } catch(e) {}
        logView.innerHTML = renderStructuredLines(logLines) || '<div class="log-line"><span class="log-col-msg" style="color:var(--text-tertiary)">尚无实时日志，请在上方输入日志文件路径后按回车</span></div>';

        _logFileGroups = await GetLogGroupedFiles(serviceId);
        const typeSelect = document.getElementById('logTypeSelect');
        const dateSelect = document.getElementById('logDateSelect');
        typeSelect.innerHTML = '<option value="">选择日志...</option>';
        dateSelect.innerHTML = '<option value="">选择日期...</option>';
        dateSelect.disabled = true;
        if (_logFileGroups) {
            for (const t of Object.keys(_logFileGroups)) {
                const opt = document.createElement('option');
                opt.value = t; opt.textContent = t;
                typeSelect.appendChild(opt);
            }
        }
        document.getElementById('logWatchIndicator').style.color = '#22c55e';
        await StreamLog(serviceId);
    } catch(e) {
        document.getElementById('logContent').innerHTML = '<pre class="log-pre">// Error: ' + e + '</pre>';
    }
}

export async function switchToWatch(serviceId) {
    _logViewMode = 'watch';
    document.getElementById('logWatchIndicator').style.color = '#22c55e';
    document.getElementById('logTypeSelect').value = '';
    document.getElementById('logDateSelect').value = '';
    document.getElementById('logDateSelect').disabled = true;
    const logView = document.getElementById('logContent');
    logView.innerHTML = '<div class="log-line"><span class="log-col-msg" style="color:var(--text-tertiary)">加载中...</span></div>';
    try {
        const consoleJson = await GetConsoleLog(serviceId);
        let logLines = [];
        try { logLines = JSON.parse(consoleJson); } catch(e) {}
        logView.innerHTML = renderStructuredLines(logLines) || '<div class="log-line"><span class="log-col-msg" style="color:var(--text-tertiary)">尚无实时日志</span></div>';
    } catch(e) { logView.innerHTML = '<pre class="log-pre">// Error: ' + e + '</pre>'; }
}

export async function setWatchPath(serviceId) {
    const path = document.getElementById('logWatchPath').value.trim();
    if (!path) return;
    const result = await SetWatchLogFile(serviceId, path);
    if (result !== 'ok') { toast(result); return; }
    toast('已开始实时监听: ' + path);
    _logViewMode = 'watch';
    document.getElementById('logWatchIndicator').style.color = '#22c55e';
    document.getElementById('logTypeSelect').value = '';
    document.getElementById('logDateSelect').value = '';
    document.getElementById('logDateSelect').disabled = true;
    const consoleJson = await GetConsoleLog(serviceId);
    const logView = document.getElementById('logContent');
    let logLines = [];
    try { logLines = JSON.parse(consoleJson); } catch(e) {}
    logView.innerHTML = renderStructuredLines(logLines) || '<div class="log-line"><span class="log-col-msg" style="color:var(--text-tertiary)">暂无内容</span></div>';
}

export function onLogTypeChange() {
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
    dateSelect.innerHTML = '<option value="">选择日期...</option>';
    for (const e of entries) {
        const opt = document.createElement('option');
        opt.value = e.name; opt.textContent = e.date || e.name;
        dateSelect.appendChild(opt);
    }
    dateSelect.disabled = false;
}

export async function loadLogFileDate(serviceId) {
    const file = document.getElementById('logDateSelect').value;
    if (!file) return;
    try {
        const content = await GetLogFileContent(serviceId, file);
        const logView = document.getElementById('logContent');
        logView.innerHTML = '<pre class="log-pre">' + escapeHtml(content) + '</pre>';
        logView.scrollTop = logView.scrollHeight;
    } catch(e) {
        document.getElementById('logContent').innerHTML = '<pre class="log-pre">// Error: ' + e + '</pre>';
    }
}

export function getLogViewMode() { return _logViewMode; }
export function getAutoScrollLog() { return _autoScrollLog; }
export function setAutoScrollLog(v) { _autoScrollLog = v; }
export function setLogFileGroups(g) { _logFileGroups = g; }

function openModal(id) { document.getElementById(id).classList.add('show'); }
