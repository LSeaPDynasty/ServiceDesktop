import { toast } from '../utils.js';

export function renderConfigPanel(container, svc) {
    container.innerHTML = `
        <div class="plugin-config">
            <h3>🌍 Nginx 配置</h3>
            <div class="form-group"><label>安装路径</label><div style="color:var(--text-secondary)">${esc(svc.installPath||'未设置')}</div></div>
            <div class="form-group"><label>监听端口</label><input type="number" id="nginxPort" placeholder="8080" value="${svc.port||'8080'}"></div>
            <p style="color:var(--text-tertiary);font-size:11px">默认使用 8080 端口避免 Windows 非管理员的 80 端口权限问题。<br>停止使用 nginx.exe -s stop。</p>
            <button class="log-tiny-btn" style="padding:6px 16px;font-size:13px" id="nginxSave">保存</button>
        </div>`;
    document.getElementById('nginxSave').onclick = () => toast('Nginx 配置已保存（端口需在 conf/nginx.conf 中修改）');
}
function esc(s) { return (s||'').replace(/"/g,'&quot;'); }
