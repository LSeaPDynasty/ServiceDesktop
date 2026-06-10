import { toast } from '../utils.js';

export function renderConfigPanel(container, svc) {
    container.innerHTML = `
        <div class="plugin-config">
            <h3>🐬 MySQL 配置</h3>
            <div class="form-group"><label>安装路径</label><div style="color:var(--text-secondary)">${esc(svc.installPath||'未设置')}</div></div>
            <div class="form-group"><label>root 密码</label><input type="password" id="mysqlPwd" placeholder="留空则无密码"></div>
            <div class="form-group"><label>数据目录</label><input type="text" id="mysqlData" placeholder="{install_path}\\data"></div>
            <div class="form-group"><label>字符集</label><select id="mysqlCharset"><option>utf8mb4</option><option>utf8</option><option>gbk</option><option>latin1</option></select></div>
            <p style="color:var(--orange);font-size:11px">⚠ 首次使用需先初始化 data 目录：<br><code>mysqld --initialize --console</code><br>停止采用强制终止进程方式。</p>
            <button class="log-tiny-btn" style="padding:6px 16px;font-size:13px" id="mysqlSave">保存</button>
        </div>`;
    document.getElementById('mysqlSave').onclick = () => toast('MySQL 配置已保存');
}
function esc(s) { return (s||'').replace(/"/g,'&quot;'); }
