// utils.js — 工具函数：日志渲染、状态标签、图标、HTML 转义

export function escapeHtml(s) {
    if (!s) return '';
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

export function toast(msg, duration) {
    const el = document.getElementById('toast');
    if (!el) return;
    el.textContent = msg;
    el.style.display = 'block';
    clearTimeout(el._timer);
    el._timer = setTimeout(() => { el.style.display = 'none'; }, duration || 3000);
}

export function statusLabel(s) {
    const m = { 0: '已停止', 1: '运行中', 2: '异常', 3: '启动中', 4: '停止中' };
    return m[s] || '未知';
}

export function statusBadgeClass(s) {
    const m = { 0: '', 1: 'running', 2: 'error', 3: 'starting', 4: 'stopping' };
    return m[s] || '';
}

export function statusDotClass(s) {
    const m = { 0: 'dot-stopped', 1: 'dot-running', 2: 'dot-error', 3: 'dot-starting', 4: 'dot-stopping' };
    return m[s] || '';
}

export function iconBg(status) {
    return status === 1 ? '#e8f5e9' : status === 2 ? '#fdecea' : '#f3f4f6';
}

const svgMap = {
    Tomcat: '<circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="4" fill="#fff"/>',
    Redis: '<path d="M4 12l8-6 8 6-8 6z"/><path d="M4 12l8 6 8-6" stroke="#fff" fill="none"/>',
    Kafka: '<rect x="3" y="6" width="18" height="12" rx="2"/><line x1="8" y1="10" x2="16" y2="10"/><line x1="8" y1="14" x2="13" y2="14"/>',
    Nacos: '<circle cx="12" cy="12" r="10"/><path d="M12 6v12M6 12h12" stroke="#fff"/>',
    Nginx: '<circle cx="12" cy="12" r="10"/><polygon points="8,8 16,8 16,16 12,12 8,16" fill="#fff"/>',
    MySQL: '<rect x="3" y="4" width="18" height="16" rx="2"/><line x1="8" y1="9" x2="16" y2="9"/><line x1="8" y1="13" x2="16" y2="13"/><line x1="8" y1="17" x2="12" y2="17"/>',
    PostgreSQL: '<rect x="3" y="4" width="18" height="16" rx="2"/><line x1="7" y1="9" x2="17" y2="9"/><line x1="7" y1="13" x2="17" y2="13"/><line x1="7" y1="17" x2="12" y2="17"/>',
    MongoDB: '<rect x="3" y="4" width="18" height="16" rx="2"/><circle cx="9" cy="12" r="2"/><circle cx="15" cy="12" r="2"/>',
};

export function svgIcon(name, size) {
    size = size || 18;
    let key = '';
    for (const k of Object.keys(svgMap)) {
        if (name.toLowerCase().includes(k.toLowerCase())) { key = k; break; }
    }
    return `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="1.5">${svgMap[key] || '<circle cx="12" cy="12" r="10" stroke-dasharray="4 2"/>'}</svg>`;
}

// ---------- 日志渲染 ----------

export function renderStructuredLines(logLines) {
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
        const trimmed = text.trim();
        if (!trimmed) return '<div class="log-line"><span class="log-col-msg" style="color:var(--text-tertiary)">&nbsp;</span></div>';
        let msg = escapeHtml(text);
        msg = msg.replace(/(ERROR|FATAL)/g, '<span style="color:var(--red)">$1</span>');
        msg = msg.replace(/\b(WARN|WARNING)\b/g, '<span style="color:var(--orange)">$1</span>');
        msg = msg.replace(/\bINFO\b/g, '<span style="color:var(--accent)">$1</span>');
        return `<div class="log-line"><span class="log-col-msg">${msg}</span></div>`;
    }).join('\n');
}

export function formatLogContent(text) {
    if (!text) return '';
    return text.split('\n').map(line => {
        const p = parseLogLine(line);
        if (p.level) {
            return `<div class="log-line">
                <span class="log-col-time">${escapeHtml(p.time)}</span>
                <span class="log-col-level lvl-${p.level}">${p.level}</span>
                <span class="log-col-source" title="${escapeHtml(p.fullSource||p.source)}">${escapeHtml(p.source)}</span>
                <span class="log-col-msg">${escapeHtml(p.msg)}</span>
              </div>`;
        }
        const trimmed = line.trim();
        if (!trimmed) return '<div class="log-line"><span class="log-col-msg" style="color:var(--text-tertiary)">&nbsp;</span></div>';
        let msg = escapeHtml(line);
        msg = msg.replace(/(ERROR|FATAL)/g, '<span style="color:var(--red)">$1</span>');
        msg = msg.replace(/\b(WARN|WARNING)\b/g, '<span style="color:var(--orange)">$1</span>');
        msg = msg.replace(/\bINFO\b/g, '<span style="color:var(--accent)">$1</span>');
        return `<div class="log-line"><span class="log-col-msg">${msg}</span></div>`;
    }).join('\n');
}

function parseLogLine(line) {
    const tomcatRe = /^(\d{2}-\w{3}-\d{4} \d{2}:\d{2}:\d{2}\.\d{3})\s+(\S+)\s+\[([^\]]+)\]\s+(\S+)\s+(.*)$/;
    let m = line.match(tomcatRe);
    if (m) {
        let time = m[1];
        try {
            const d = new Date(time.replace(/-(\w{3})-/g, (_, mon) => {
                const months = {Jan:'01',Feb:'02',Mar:'03',Apr:'04',May:'05',Jun:'06',
                               Jul:'07',Aug:'08',Sep:'09',Oct:'10',Nov:'11',Dec:'12'};
                return '-' + (months[mon]||'01') + '-';
            }).replace(/\.\d{3}/, ''));
            if (!isNaN(d.getTime())) {
                time = String(d.getMonth()+1).padStart(2,'0') + '-' +
                       String(d.getDate()).padStart(2,'0') + ' ' +
                       time.slice(time.length-8);
            }
        } catch(e) {}
        let level = m[2];
        if (level === '信息') level = 'INFO';
        else if (level === '警告') level = 'WARN';
        else if (level === '严重' || level === '错误') level = 'ERROR';
        let source = m[4];
        const parts = source.split('.');
        if (parts.length > 2) source = parts[parts.length-2] + '.' + parts[parts.length-1];
        return { time, level, source, fullSource: m[4], msg: m[5] };
    }
    return { time: '', level: '', source: '', msg: line };
}
