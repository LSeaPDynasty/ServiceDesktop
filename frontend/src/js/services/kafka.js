import { toast } from '../utils.js';
import { SetInstallPath } from '../../../wailsjs/go/main/App.js';

export function renderConfigPanel(container, svc) {
    container.innerHTML = `
        <div class="plugin-config">
            <h3>📨 Kafka 配置</h3>
            <div class="form-group"><label>安装路径</label><div id="kafkaPath" style="color:var(--text-secondary)">${esc(svc.installPath||'未设置')}</div></div>
            <div class="form-group"><label>堆内存 (KAFKA_HEAP_OPTS)</label><input type="text" id="kafkaHeap" value="-Xmx1G -Xms1G"></div>
            <div class="form-group"><label>ZooKeeper 地址</label><input type="text" id="kafkaZk" value="localhost:2181"></div>
            <p style="color:var(--orange);font-size:11px">⚠ 需先启动 ZooKeeper（端口 2181）。Windows 版 Kafka 无停止脚本，停止将强制终止进程。</p>
            <button class="log-tiny-btn" style="padding:6px 16px;font-size:13px" id="kafkaSave">保存配置</button>
        </div>`;
    document.getElementById('kafkaSave').onclick = async () => {
        const heap = document.getElementById('kafkaHeap').value.trim();
        if (heap) await SetInstallPath(svc.id, svc.installPath);
        toast('Kafka 配置已保存（堆内存需在 config/server.properties 中修改）');
    };
}
function esc(s) { return (s||'').replace(/"/g,'&quot;'); }
