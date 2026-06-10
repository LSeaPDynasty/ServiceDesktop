// Redis 配置面板
import { toast } from '../utils.js';
import { SetInstallPath, SetStartArgs, SaveStartProfile } from '../../../wailsjs/go/main/App.js';

export function renderConfigPanel(container, svc) {
    container.innerHTML = `
        <div class="plugin-config">
            <h3>⚙️ Redis 配置</h3>
            <div class="form-group">
                <label>安装路径</label>
                <div style="display:flex;gap:4px">
                    <input type="text" id="redisPath" value="${escAttr(svc.installPath||'')}" placeholder="C:\\tools\\redis">
                    <button class="log-tiny-btn" id="redisBrowse">浏览</button>
                </div>
            </div>
            <div class="form-group">
                <label>requirepass（密码）</label>
                <input type="text" id="redisPassword" placeholder="留空则无密码">
            </div>
            <div class="form-group">
                <label>最大内存</label>
                <input type="text" id="redisMaxmem" placeholder="如 256mb">
            </div>
            <div class="form-group">
                <label>持久化模式</label>
                <select id="redisPersist">
                    <option value="RDB">RDB</option>
                    <option value="AOF">AOF</option>
                    <option value="RDB+AOF">RDB+AOF</option>
                </select>
            </div>
            <button class="log-tiny-btn" style="padding:6px 16px;font-size:13px" id="redisSave">保存配置</button>
        </div>`;
    document.getElementById('redisBrowse').onclick = async () => {
        const { BrowseFolder } = await import('../../../wailsjs/go/main/App.js');
        const p = await BrowseFolder();
        if (p) document.getElementById('redisPath').value = p;
    };
    document.getElementById('redisSave').onclick = async () => {
        const path = document.getElementById('redisPath').value.trim();
        if (path) await SetInstallPath(svc.id, path);
        toast('Redis 配置已保存');
    };
}

function escAttr(s) { return (s||'').replace(/"/g, '&quot;'); }
