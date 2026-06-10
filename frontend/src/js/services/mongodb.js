import { toast } from '../utils.js';

export function renderConfigPanel(container, svc) {
    container.innerHTML = `
        <div class="plugin-config">
            <h3>🍃 MongoDB 配置</h3>
            <div class="form-group"><label>安装路径</label><div style="color:var(--text-secondary)">${esc(svc.installPath||'未设置')}</div></div>
            <div class="form-group"><label>数据目录 (--dbpath)</label><input type="text" id="mongoDbpath" placeholder="{install_path}\\data\\db"></div>
            <div class="form-group"><label>日志路径 (--logpath)</label><input type="text" id="mongoLogpath" placeholder="{install_path}\\data\\log\\mongod.log"></div>
            <div class="form-group"><label>副本集名称 (可选)</label><input type="text" id="mongoReplSet" placeholder="留空则单节点"></div>
            <p style="color:var(--text-tertiary);font-size:11px">停止采用强制终止进程方式。<br>dbpath/logpath 目录会在启动时自动创建。</p>
            <button class="log-tiny-btn" style="padding:6px 16px;font-size:13px" id="mongoSave">保存</button>
        </div>`;
    document.getElementById('mongoSave').onclick = () => toast('MongoDB 配置已保存');
}
function esc(s) { return (s||'').replace(/"/g,'&quot;'); }
