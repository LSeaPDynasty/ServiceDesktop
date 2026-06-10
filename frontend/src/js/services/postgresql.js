import { toast } from '../utils.js';

export function renderConfigPanel(container, svc) {
    container.innerHTML = `
        <div class="plugin-config">
            <h3>🐘 PostgreSQL 配置</h3>
            <div class="form-group"><label>安装路径</label><div style="color:var(--text-secondary)">${esc(svc.installPath||'未设置')}</div></div>
            <div class="form-group"><label>数据目录</label><input type="text" id="pgData" placeholder="{install_path}\\data"></div>
            <div class="form-group"><label>监听端口</label><input type="number" id="pgPort" value="${svc.port||'5432'}"></div>
            <p style="color:var(--text-tertiary);font-size:11px">通过 pg_ctl.exe 管理，启动参数与命令分离。<br>日志输出到 data\\log\\pg.log。</p>
            <button class="log-tiny-btn" style="padding:6px 16px;font-size:13px" id="pgSave">保存</button>
        </div>`;
    document.getElementById('pgSave').onclick = () => toast('PostgreSQL 配置已保存');
}
function esc(s) { return (s||'').replace(/"/g,'&quot;'); }
