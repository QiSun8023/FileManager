const $ = id => document.getElementById(id), bridge = window.FileManagerNative, preview = !bridge, callbacks = new Map();
let sequence = 0, config = {enabled: false, stableSeconds: 10, rules: []}, status = null, connected = false, loading = false, saving = false;
const Icons = window.FMIcons || {icon: () => document.createElement('span'), hydrateIcons: () => {}};
const titles = {overview: '文件管理', monitor: '监控规则', pending: '待处理', settings: '设置'};
const darkQuery = window.matchMedia ? matchMedia('(prefers-color-scheme: dark)') : null;
function syncTone() { document.documentElement.dataset.tone = darkQuery && darkQuery.matches ? 'black' : 'light'; }
syncTone();
if (darkQuery && darkQuery.addEventListener) darkQuery.addEventListener('change', syncTone);

window.fmReply = (id, result) => {
  const task = callbacks.get(id); if (!task) return;
  callbacks.delete(id); clearTimeout(task.timer);
  result.ok ? task.resolve(result.data) : task.reject(new Error(result.error || '操作失败'));
};
async function api(action, data = {}) {
  if (preview) return demo(action, data);
  return new Promise((resolve, reject) => {
    const id = 'h' + (++sequence), timer = setTimeout(() => { callbacks.delete(id); reject(new Error('连接超时，请检查助手的 Root 授权')); }, 600000);
    callbacks.set(id, {resolve, reject, timer});
    bridge.call(action, JSON.stringify(data), id);
  });
}
function node(tag, cls, text) { const e = document.createElement(tag); if (cls) e.className = cls; if (text !== undefined) e.textContent = text; return e; }
function showMessage(text) {
  const box = $('message'); box.replaceChildren();
  if (text) box.append(Icons.icon('info'), node('span', '', text));
  box.hidden = !text;
}
window.showMessage = showMessage;

const topbar = $('topbar'), largeTitle = document.querySelector('.mx-large-title');
function collapse() { topbar.classList.toggle('is-collapsed', largeTitle.getBoundingClientRect().bottom <= topbar.getBoundingClientRect().bottom + 4); }
window.addEventListener('scroll', collapse, {passive: true});
function changePage(page) {
  for (const id of ['overview', 'monitor', 'pending', 'settings']) $(id).hidden = id !== page;
  document.querySelectorAll('[data-nav]').forEach(b => { b.classList.toggle('active', b.dataset.nav === page); b.setAttribute('aria-current', b.dataset.nav === page ? 'page' : 'false'); });
  $('page-title').textContent = titles[page]; $('topbar-title').textContent = titles[page];
  window.scrollTo({top: 0, behavior: 'instant'}); collapse();
}
window.fmOpenPending = () => changePage('pending');
document.querySelectorAll('[data-nav],[data-page]').forEach(b => b.addEventListener('click', () => changePage(b.dataset.nav || b.dataset.page)));

function setTone(el, tone) { el.classList.remove('is-ok', 'is-warn', 'is-error', 'is-info'); if (tone) el.classList.add('is-' + tone); }
window.refreshPermissions = () => {
  let p = {};
  try { p = bridge ? JSON.parse(bridge.permissions()) : {notifications: true, overlay: true, reminders: true, connected: true}; } catch {}
  $('notification-status').textContent = p.notifications ? '已允许' : '未允许 · 点击设置';
  $('live-status').textContent = p.liveSupported ? (p.liveAllowed ? '系统已允许 · 有待处理文件时申请展示' : '未允许 · 点击设置') : '需 Android 16 或以上，使用普通通知';
  $('live-setting').disabled = !p.liveSupported;
  $('overlay-status').textContent = p.overlay ? '已允许 · 解锁时显示' : '未允许 · 点击设置';
  setTone($('notification-setting'), p.notifications ? 'ok' : 'warn');
  setTone($('overlay-setting'), p.overlay ? 'ok' : 'warn');
  $('reminder-state').textContent = p.reminders ? (p.serviceRunning ? '提醒服务运行中' : '已开启 · 等待服务启动') + (p.reminderError ? ' · ' + p.reminderError : '') : '已暂停';
  const ready = p.notifications && p.overlay;
  $('permission-title').textContent = ready ? '提醒权限已就绪' : '提醒权限尚未完整';
  $('permission-summary').textContent = `通知${p.notifications ? '已允许' : '未允许'} · 悬浮窗${p.overlay ? '已允许' : '未允许'} · 仍需允许后台运行`;
  setTone($('permission-row'), ready ? 'ok' : 'warn');
  return p;
};
for (const [id, name] of [['notification-setting', 'notifications'], ['live-setting', 'live'], ['overlay-setting', 'overlay'], ['battery-setting', 'battery'], ['start', 'start'], ['pause', 'pause']])
  $(id).addEventListener('click', () => { if (bridge) bridge.setting(name); else showMessage('这是界面预览，手机权限需在助手中设置'); });

const chooser = FMWatchEditor.createDirectoryChooser(api);
function render() { FMWatchEditor.renderRules($('rules'), config.rules, {chooseSources: roots => chooser.choose(roots, true), chooseTarget: target => chooser.choose(target ? [target] : [], false)}); }

function setStatus(s) {
  const pill = $('connection'), iconBox = $('status-icon');
  let tone, label, title, desc;
  if (!s) { tone = 'error'; label = '未连接模块'; title = '连接模块'; desc = '连接后查看文件监控与转移提醒'; }
  else if (!s.enabled) { tone = ''; label = '监控已关闭'; title = '从一条监控规则开始'; desc = '选择来源、文件类型和目标目录，保存后开始监控'; }
  else if (s.rules === 0) { tone = 'info'; label = '已开启 · 等待目录'; title = '添加监控目录'; desc = '总开关已保存；添加来源和目标目录后开始检测'; }
  else if (s.running) { tone = 'ok'; label = '后台监控中'; title = '新文件，有序归档'; desc = '只对新增文件询问，保留原处不会反复提醒'; }
  else { tone = 'warn'; label = '等待恢复监控'; title = '后台进程未运行'; desc = '可保存规则或恢复助手提醒'; }
  if (s && s.error) desc = s.error;
  $('monitor-state').textContent = !s ? '无法读取状态，请检查连接' : !s.enabled ? '监控已关闭' : s.rules === 0 ? '已开启，等待添加监控目录' : s.running ? '已开启，后台监控中' : '已开启，等待后台启动';
  pill.textContent = label; pill.className = 'mx-pill' + (tone ? ' is-' + tone : '');
  setTone(iconBox.closest('.mx-row'), tone);
  $('hero-title').textContent = title; $('hero-desc').textContent = desc;
}
async function refresh() {
  if (loading || saving) return;
  loading = true;
  try {
    const [s, c] = await Promise.all([api('watch-status'), api('watch-config')]);
    status = s;
    if (!connected) { config = c; $('enabled').checked = config.enabled; $('stable').value = String(config.stableSeconds); render(); }
    connected = true; $('enabled').disabled = false;
    $('connect').textContent = '重新连接'; $('connect').hidden = true; $('connect-panel').hidden = true;
    setStatus(s);
    $('dir-count').textContent = new Set(c.rules.flatMap(FMWatchEditor.ruleRoots)).size;
    $('type-count').textContent = new Set(c.rules.flatMap(r => r.extensions)).size;
    $('pending-count').textContent = s.items.length;
    for (const id of ['pending-badge', 'pending-nav-badge']) { $(id).textContent = s.items.length || ''; $(id).dataset.count = String(s.items.length); }
    $('pending-badge').hidden = !s.items.length;
    renderItems(s.items); refreshPermissions();
  } catch (e) {
    showMessage(e.message); setStatus(null); $('connect').hidden = false; $('connect-panel').hidden = false;
  } finally { loading = false; }
}
function ask(title, text) {
  $('confirmation-title').textContent = title; $('confirmation-text').textContent = text;
  const d = $('confirmation'); d.returnValue = 'cancel'; d.showModal();
  return new Promise(resolve => d.addEventListener('close', () => resolve(d.returnValue === 'ok'), {once: true}));
}
$('confirmation-ok').addEventListener('click', () => $('confirmation').close('ok'));
$('confirmation-cancel').addEventListener('click', () => $('confirmation').close('cancel'));
function renderItems(items) {
  const box = $('items'); box.replaceChildren();
  if (!items.length) { const c = node('div', 'mx-card'), e = node('div', 'mx-empty'); e.append(Icons.icon('ok'), node('span', '', '暂时没有待处理文件')); c.append(e); box.append(c); return; }
  for (const item of items) {
    const destination = item.target + (item.targetName ? '/' + item.targetName : '');
    const card = node('article', 'mx-card pending-item'), head = node('div', 'mx-row'), badge = node('span', 'mx-row-icon'), text = node('span', 'mx-row-text');
    badge.append(Icons.icon(item.verified ? 'shield' : 'clock'));
    text.append(node('h3', 'mx-row-title', item.path.split('/').pop()), node('span', 'verified mx-row-summary', item.verified ? '完整性检查通过' : '稳定等待完成，请核对下载状态'));
    head.append(badge, text); if (!item.verified) head.classList.add('is-warn');
    const route = node('div', 'watch-route'); route.append(node('span', '', item.path), node('span', '', destination));
    const actions = node('div', 'watch-toolbar mx-button-row');
    for (const [choice, label] of [['keep', '保留原处'], ['move', '转移']]) {
      const b = node('button', choice === 'move' ? 'mx-button is-primary' : 'mx-button', label); b.type = 'button';
      b.addEventListener('click', async () => {
        if (saving) return;
        if (choice === 'move' && !await ask('转移文件', `${item.path}

转移至 ${destination}
不会覆盖同名文件。`)) return;
        saving = true;
        try { await api('watch-decide', {id: item.id, decision: choice}); showMessage(choice === 'move' ? '文件已转移' : '已保留原处，本次新增不再提醒'); }
        catch (e) { showMessage(e.message); }
        finally { saving = false; await refresh(); }
      });
      actions.append(b);
    }
    card.append(head, route, actions); box.append(card);
  }
}
$('connect').addEventListener('click', () => refresh());
$('refresh').addEventListener('click', () => refresh());
$('refresh-pending').addEventListener('click', () => refresh());
$('add').addEventListener('click', () => {
  if (!connected) { showMessage('请先连接模块'); return; }
  if (config.rules.length >= 20) { showMessage('最多 20 条规则'); return; }
  config.rules.push({id: '', roots: [], target: '', extensions: ['.zip', '.apk'], recursive: false}); render();
});
$('enabled').addEventListener('change', async () => {
  if (saving || !connected) { $('enabled').checked = config.enabled; return; }
  const checked = $('enabled').checked;
  saving = true; $('enabled').disabled = true; $('save').disabled = true;
  $('monitor-state').textContent = '正在保存总开关…';
  try {
    const saved = await api('watch-enable', {checked});
    const readback = await api('watch-config');
    config.enabled = readback.enabled; $('enabled').checked = readback.enabled;
    if (saved.enabled !== checked || readback.enabled !== checked) throw new Error('总开关未保存成功，请重试');
    showMessage(checked ? '监控已开启；没有规则时等待添加目录' : '监控已关闭');
  } catch (e) { $('enabled').checked = config.enabled; showMessage(e.message); $('monitor-state').textContent = '总开关保存失败：' + e.message; }
  finally { saving = false; $('enabled').disabled = false; $('save').disabled = false; await refresh(); }
});
$('save').addEventListener('click', async () => {
  if (saving || !connected) return;
  const next = {...config, enabled: $('enabled').checked, stableSeconds: Number($('stable').value)};
  if (next.enabled && !await ask('保存监控规则', '新增目录会忽略已有文件。仅增减目录时，其他目录的检测记录保留。')) return;
  saving = true; $('save').disabled = true;
  try { config = await api('watch-save', {watch: next}); render(); showMessage('规则已保存'); $('save-result').textContent = '规则已保存'; $('save-result').hidden = false; }
  catch (e) { showMessage(e.message); $('save-result').textContent = '未保存：' + e.message; $('save-result').hidden = false; }
  finally { saving = false; $('save').disabled = false; refresh(); }
});

let demoConfig = {enabled: true, stableSeconds: 10, rules: [{id: 'preview', roots: ['/storage/emulated/0/Download', '/storage/emulated/0/Download/WeiXin'], target: '/storage/emulated/0/Documents/安装包', extensions: ['.apk', '.zip'], recursive: false}]};
const demoItems = [{id: 'p1', path: '/storage/emulated/0/Download/WeiXin/工具箱_3.2.apk.1', target: '/storage/emulated/0/Documents/安装包', targetName: '工具箱_3.2.apk', verified: true}, {id: 'p2', path: '/storage/emulated/0/Download/资料合集.zip', target: '/storage/emulated/0/Documents/安装包', verified: false}];
function demo(action, data) {
  if (action === 'watch-config') return structuredClone(demoConfig);
  if (action === 'watch-status') return {enabled: demoConfig.enabled, running: true, error: '', version: 'v1.11.0', items: structuredClone(demoItems)};
  if (action === 'watch-enable') { demoConfig.enabled = data.checked; return structuredClone(demoConfig); }
  if (action === 'watch-save') { demoConfig = structuredClone(data.watch); return demoConfig; }
  if (action === 'directories') { const root = '/storage/emulated/0', path = data.root || root, names = path === root ? ['Download', 'Documents', 'Pictures'] : path.endsWith('Download') ? ['WeiXin', '浏览器'] : []; return {path, storage: root, parent: path === root ? '' : path.slice(0, path.lastIndexOf('/')), total: names.length, items: names.map(name => ({name, path: path + '/' + name}))}; }
  throw new Error('界面预览不执行文件操作');
}

Icons.hydrateIcons();
if (preview) document.querySelector('.version').textContent = '界面预览';
refreshPermissions();
if (preview || refreshPermissions().connected) refresh();
setInterval(() => { if (connected && !document.hidden) refresh(); }, 10000);
collapse();
