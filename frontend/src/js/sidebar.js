// sidebar.js — 侧栏渲染：搜索、分组折叠、服务选择

import { iconBg, svgIcon, statusDotClass } from './utils.js';

let _collapsedGroups = {};

export function toggleGroup(cat) {
    _collapsedGroups[cat] = !_collapsedGroups[cat];
    const ev = new CustomEvent('sidebar-refresh');
    window.dispatchEvent(ev);
}

export function renderSidebar(services, selectedId) {
    const list = document.getElementById('sidebarList');
    const search = (document.getElementById('searchInput').value || '').toLowerCase();

    const categories = {};
    for (const svc of services) {
        if (search && !svc.name.toLowerCase().includes(search)) continue;
        const cat = svc.category || '其他';
        if (!categories[cat]) categories[cat] = [];
        categories[cat].push(svc);
    }

    const catOrder = ['Middleware', 'Database', 'Custom', '其他'];
    const catLabel = { Middleware: '中间件', Database: '数据库', Custom: '自定义' };

    let html = '';
    for (const cat of catOrder) {
        const items = categories[cat];
        if (!items || items.length === 0) continue;

        const isCollapsed = _collapsedGroups[cat];
        html += `<div class="sidebar-section" style="cursor:pointer;user-select:none" data-cat="${cat}">
            ${isCollapsed ? '▶' : '▼'} ${catLabel[cat] || cat}
        </div>`;

        if (!isCollapsed) {
            for (const svc of items) {
                const active = svc.id === selectedId ? ' active' : '';
                html += `
                    <div class="svc-item${active}" data-svc-id="${svc.id}">
                        <div class="svc-item-icon" style="background:${iconBg(svc.status)}">${svgIcon(svc.name)}</div>
                        <div class="svc-item-info">
                            <div class="svc-item-name">${svc.name}</div>
                            <div class="svc-item-port">${svc.port ? ':' + svc.port : ''}</div>
                        </div>
                        <div class="status-dot ${statusDotClass(svc.status)}"></div>
                    </div>`;
            }
        }
    }
    if (!html) html = '<div style="padding:20px;text-align:center;color:var(--text-tertiary)">无匹配服务</div>';
    list.innerHTML = html;

    // 绑定事件
    list.querySelectorAll('.sidebar-section').forEach(el => {
        el.onclick = () => { toggleGroup(el.dataset.cat); };
    });
    list.querySelectorAll('.svc-item').forEach(el => {
        el.onclick = () => {
            const id = el.dataset.svcId;
            const ev = new CustomEvent('service-select', { detail: id });
            window.dispatchEvent(ev);
        };
    });
}

let _searchTimer = null;
export function filterServices() {
    clearTimeout(_searchTimer);
    _searchTimer = setTimeout(() => {
        const ev = new CustomEvent('sidebar-refresh');
        window.dispatchEvent(ev);
    }, 150);
}
