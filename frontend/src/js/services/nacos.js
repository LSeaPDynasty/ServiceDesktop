import { toast } from '../utils.js';

export function renderConfigPanel(container, svc) {
    container.innerHTML = `
        <div class="plugin-config">
            <h3>🌐 Nacos 配置</h3>
            <div class="form-group"><label>安装路径</label><div style="color:var(--text-secondary)">${esc(svc.installPath||'未设置')}</div></div>
            <div class="form-group"><label>运行模式</label><select id="nacosMode"><option value="standalone">standalone（单机）</option><option value="cluster">cluster（集群）</option></select></div>
            <div class="form-group"><label>集群数据库地址（仅 cluster）</label><input type="text" id="nacosDb" placeholder="mysql://..."></div>
            <p style="color:var(--text-tertiary);font-size:11px">模式由环境变量 MODE 控制，无需传递启动参数。<br>默认端口 8848（HTTP）+ 9848（gRPC）。</p>
            <button class="log-tiny-btn" style="padding:6px 16px;font-size:13px" id="nacosSave">保存</button>
        </div>`;
    document.getElementById('nacosSave').onclick = () => toast('Nacos 配置已保存');
}
function esc(s) { return (s||'').replace(/"/g,'&quot;'); }
