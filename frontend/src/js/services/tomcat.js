export function renderConfigPanel(container, svc) {
    container.innerHTML = `
        <div class="plugin-config">
            <h3>🐱 Tomcat 配置</h3>
            <div class="form-group"><label>安装路径</label><div id="tomcatPath" style="color:var(--text-secondary)">${escAttr(svc.installPath||'未设置')}</div></div>
            <p style="color:var(--text-tertiary);font-size:11px">Tomcat 使用标准 startup.bat / shutdown.bat 启停。<br>JAVA_HOME 从系统环境变量继承，CATALINA_HOME 自动设置为安装路径。</p>
        </div>`; }
function escAttr(s) { return (s||'').replace(/"/g,'&quot;'); }
