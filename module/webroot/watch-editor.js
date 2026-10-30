export const fileTypes = [
  ['安装包', ['apk', 'apk.1', 'apks', 'xapk', 'apkm']],
  ['压缩包', ['zip', '7z', 'rar', 'tar', 'gz']],
  ['图片', ['jpg', 'jpeg', 'png', 'webp', 'gif', 'heic']],
  ['视频', ['mp4', 'mkv', 'mov', 'avi', 'webm']],
  ['音频', ['mp3', 'm4a', 'aac', 'flac', 'wav', 'ogg']],
  ['文档', ['pdf', 'doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx', 'txt', 'epub']],
];

export function ruleRoots(rule) {
  return [...new Set((rule.roots?.length ? rule.roots : rule.root ? [rule.root] : []).filter(Boolean))];
}

const el = (tag, cls, text) => { const e = document.createElement(tag); if (cls) e.className = cls; if (text !== undefined) e.textContent = text; return e; };
const button = (cls, text) => { const b = el('button', cls, text); b.type = 'button'; return b; };
// Lucide geometry for the few icons used here; kept local so this file stays standalone for the helper app.
const glyphs = {
  folder: [['path', {d: 'M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z'}]],
  target: [['path', {d: 'm6 14 1.5-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.54 6a2 2 0 0 1-1.95 1.5H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H18a2 2 0 0 1 2 2v2'}]],
  plus: [['path', {d: 'M5 12h14'}], ['path', {d: 'M12 5v14'}]],
  x: [['path', {d: 'M18 6 6 18'}], ['path', {d: 'm6 6 12 12'}]],
  chevron: [['path', {d: 'm9 18 6-6-6-6'}]],
  up: [['path', {d: 'M14 9 9 4 4 9'}], ['path', {d: 'M20 20h-7a4 4 0 0 1-4-4V4'}]],
  trash: [['path', {d: 'M10 11v6'}], ['path', {d: 'M14 11v6'}], ['path', {d: 'M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6'}], ['path', {d: 'M3 6h18'}], ['path', {d: 'M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2'}]],
  layers: [['path', {d: 'M12.83 2.18a2 2 0 0 0-1.66 0L2.6 6.08a1 1 0 0 0 0 1.83l8.58 3.91a2 2 0 0 0 1.66 0l8.58-3.9a1 1 0 0 0 0-1.83z'}], ['path', {d: 'M2 12a1 1 0 0 0 .58.91l8.6 3.91a2 2 0 0 0 1.65 0l8.58-3.9A1 1 0 0 0 22 12'}], ['path', {d: 'M2 17a1 1 0 0 0 .58.91l8.6 3.91a2 2 0 0 0 1.65 0l8.58-3.9A1 1 0 0 0 22 17'}]],
  storage: [['line', {x1: '22', x2: '2', y1: '12', y2: '12'}], ['path', {d: 'M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z'}], ['line', {x1: '6', x2: '6.01', y1: '16', y2: '16'}], ['line', {x1: '10', x2: '10.01', y1: '16', y2: '16'}]],
};
function glyph(name, size = 20) {
  const ns = 'http://www.w3.org/2000/svg', svg = document.createElementNS(ns, 'svg');
  for (const [k, v] of Object.entries({viewBox: '0 0 24 24', width: size, height: size, fill: 'none', stroke: 'currentColor', 'stroke-width': '1.8', 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'aria-hidden': 'true', class: 'mx-icon'})) svg.setAttribute(k, String(v));
  for (const [tag, attrs] of glyphs[name] || []) { const s = document.createElementNS(ns, tag); for (const [k, v] of Object.entries(attrs)) s.setAttribute(k, v); svg.append(s); }
  return svg;
}
const short = path => path.split('/').filter(Boolean).slice(-2).join('/') || '内部存储';

export function renderRules(box, rules, {chooseSources, chooseTarget}) {
  box.replaceChildren();
  const render = () => renderRules(box, rules, {chooseSources, chooseTarget});
  if (!rules.length) {
    const card = el('div', 'mx-card'), empty = el('div', 'mx-empty');
    empty.append(glyph('folder', 40), el('strong', '', '还没有监控规则'), el('span', '', '添加规则后选择来源目录、文件类型和目标目录'));
    card.append(empty); box.append(card);
  }
  rules.forEach((rule, index) => {
    rule.roots = ruleRoots(rule);
    const card = el('section', 'watch-rule mx-card'), head = el('div', 'watch-rule-heading mx-row');
    const summary = () => `${rule.roots.length} 个目录 · ${rule.extensions.length} 种类型`;
    const headText = el('span', 'mx-row-text');
    headText.append(el('h3', 'mx-row-title', `规则 ${String(index + 1).padStart(2, '0')}`), el('span', 'muted mx-row-summary', summary()));
    const remove = button('watch-delete-rule mx-icon-button');
    remove.setAttribute('aria-label', `删除规则 ${index + 1}`); remove.title = '删除规则'; remove.append(glyph('trash', 22));
    remove.addEventListener('click', () => { rules.splice(index, 1); render(); });
    head.append(headText, remove); card.append(head);

    const sources = el('div', 'mx-subpanel watch-section');
    sources.append(el('label', 'watch-field-label', '监控目录'));
    const paths = el('div', 'watch-source-list');
    rule.roots.forEach(path => {
      const row = el('div', 'watch-source-row'), text = el('span', '', path), drop = button('watch-remove mx-icon-button');
      drop.setAttribute('aria-label', '移除监控目录 ' + path); drop.title = '移除'; drop.append(glyph('x', 18));
      drop.addEventListener('click', () => { rule.roots = rule.roots.filter(p => p !== path); rule.root = rule.roots[0] || ''; render(); });
      row.append(glyph('folder', 18), text, drop); paths.append(row);
    });
    if (!rule.roots.length) paths.append(el('p', 'hint', '尚未选择监控目录'));
    const add = button('watch-add-sources mx-button is-small is-tonal', '选择监控目录');
    add.prepend(glyph('plus', 16));
    add.addEventListener('click', async () => { const selected = await chooseSources(rule.roots); if (selected !== null) { rule.roots = selected; rule.root = selected[0] || ''; render(); } });
    sources.append(paths, add); card.append(sources);

    const targetBox = el('div', 'mx-subpanel watch-section');
    targetBox.append(el('label', 'watch-field-label', '统一目标目录'));
    const target = button('path-field mx-field');
    target.setAttribute('aria-label', `规则 ${index + 1} 目标目录`);
    const targetText = el('span', '', rule.target || '点击选择目标目录');
    if (!rule.target) targetText.className = 'placeholder';
    target.append(glyph('target'), targetText, glyph('chevron', 18));
    target.addEventListener('click', async () => { const selected = await chooseTarget(rule.target); if (selected !== null) { rule.target = selected; render(); } });
    targetBox.append(target); card.append(targetBox);

    const typesBox = el('div', 'mx-subpanel watch-section');
    typesBox.append(el('label', 'watch-field-label', '文件类型'));
    const known = new Set(fileTypes.flatMap(([, types]) => types.map(t => '.' + t))), extras = rule.extensions.filter(t => !known.has(t));
    const groups = [...fileTypes.map(([name, types]) => [name, types.map(t => '.' + t)]), ...(extras.length ? [['已有的其他类型', extras]] : [])];
    for (const [name, types] of groups) {
      const section = el('section', 'type-group'), title = el('div', 'type-group-title');
      const toggleAll = button('type-select-all mx-button is-text is-small');
      const allSelected = () => types.every(t => rule.extensions.includes(t));
      const updateToggle = () => { const all = allSelected(); toggleAll.textContent = all ? '取消全选' : '全选'; toggleAll.setAttribute('aria-label', (all ? '取消全选' : '全选') + name); toggleAll.setAttribute('aria-pressed', String(all)); };
      updateToggle();
      title.append(el('span', '', name), toggleAll);
      const chips = el('div', 'type-chips mx-chips');
      toggleAll.addEventListener('click', () => {
        rule.extensions = allSelected() ? rule.extensions.filter(t => !types.includes(t)) : [...new Set([...rule.extensions, ...types])];
        for (const chip of chips.children) chip.setAttribute('aria-pressed', String(rule.extensions.includes(chip.dataset.extension)));
        updateToggle(); headText.lastChild.textContent = summary();
      });
      for (const ext of types) {
        const b = button('type-chip mx-chip', ext.slice(1).toUpperCase());
        b.dataset.extension = ext; b.setAttribute('aria-pressed', String(rule.extensions.includes(ext)));
        b.addEventListener('click', () => {
          rule.extensions = rule.extensions.includes(ext) ? rule.extensions.filter(t => t !== ext) : [...rule.extensions, ext];
          b.setAttribute('aria-pressed', String(rule.extensions.includes(ext)));
          updateToggle(); headText.lastChild.textContent = summary();
        });
        chips.append(b);
      }
      section.append(title, chips); typesBox.append(section);
    }
    card.append(typesBox);

    const recursive = el('label', 'switch-row mx-row watch-recursive'), check = el('input', 'mx-switch'), recText = el('span', 'mx-row-text');
    check.type = 'checkbox'; check.setAttribute('role', 'switch'); check.checked = rule.recursive;
    check.addEventListener('change', () => rule.recursive = check.checked);
    recText.append(el('span', 'mx-row-title', '包含子文件夹'));
    const recIcon = el('span', 'mx-row-icon'); recIcon.append(glyph('layers'));
    recursive.append(recIcon, recText, check); card.append(recursive);
    box.append(card);
  });
}

export function createDirectoryChooser(api) {
  const dialog = el('dialog', 'mx-dialog watch-directory-dialog');
  dialog.setAttribute('aria-label', '选择文件夹');
  const heading = el('div', 'mx-dialog-header'), title = el('h2', 'mx-dialog-title', '选择文件夹'), cancel = button('mx-icon-button');
  cancel.setAttribute('aria-label', '取消'); cancel.title = '取消'; cancel.append(glyph('x', 22));
  heading.append(cancel, title, el('span', 'header-spacer'));
  const crumbs = el('div', 'breadcrumbs'), pathLabel = el('p', 'picker-path');
  const current = el('label', 'watch-current mx-row'), currentCheck = el('input', 'mx-check'), currentText = el('span', 'mx-row-title', '选择当前目录');
  currentCheck.type = 'checkbox'; current.append(currentCheck, currentText);
  const status = el('p', 'status-line'), list = el('div', 'watch-directory-list picker-list');
  const paging = el('div', 'pagination'), prev = button('mx-icon-button'), pageLabel = el('span', '', '1'), next = button('mx-icon-button');
  prev.setAttribute('aria-label', '上一页'); prev.title = '上一页'; prev.append(glyph('chevron', 22)); prev.firstChild.style.transform = 'scaleX(-1)';
  next.setAttribute('aria-label', '下一页'); next.title = '下一页'; next.append(glyph('chevron', 22));
  paging.append(prev, pageLabel, next);
  const actions = el('div', 'mx-dialog-actions dialog-actions'), done = button('mx-button is-primary primary', '完成选择');
  actions.append(done);
  dialog.append(heading, crumbs, pathLabel, current, status, list, paging, actions);
  document.body.append(dialog);
  let resolve = null, selected = new Set(), multiple = false, path = '', page = 0, storage = '', generation = 0;
  const refreshCount = () => { done.textContent = multiple ? `完成选择（${selected.size}）` : '选择当前目录'; currentCheck.checked = selected.has(path); };
  async function load(root = '', n = 0) {
    const id = ++generation; done.disabled = true; status.textContent = '正在读取目录…';
    try {
      const d = await api('directories', {root, page: n});
      if (id !== generation) return;
      path = d.path; page = n; storage = d.storage; pathLabel.textContent = path;
      crumbs.replaceChildren();
      const home = button('mx-chip', '内部存储'); home.prepend(glyph('storage', 16)); home.addEventListener('click', () => load(storage)); crumbs.append(home);
      let prefix = storage;
      for (const part of path.slice(storage.length).split('/').filter(Boolean)) {
        prefix += '/' + part; const dest = prefix, b = button('mx-chip', part); b.addEventListener('click', () => load(dest)); crumbs.append(b);
      }
      list.replaceChildren();
      if (d.parent) { const up = button('directory-row mx-row'); up.append(glyph('up'), el('span', 'mx-row-title', '返回上级')); up.addEventListener('click', () => load(d.parent)); list.append(up); }
      for (const item of d.items) {
        const row = el('div', 'watch-directory-row');
        if (multiple) {
          const label = el('label', 'watch-folder-check'), check = el('input', 'mx-check');
          check.type = 'checkbox'; check.checked = selected.has(item.path); check.setAttribute('aria-label', '选择 ' + item.name);
          check.addEventListener('change', () => { check.checked ? selected.add(item.path) : selected.delete(item.path); refreshCount(); });
          label.append(check); row.append(label);
        }
        const open = button('directory-row mx-row');
        open.setAttribute('aria-label', '打开 ' + item.name);
        open.append(glyph('folder'), el('span', 'mx-row-title', item.name), glyph('chevron', 18));
        open.addEventListener('click', () => load(item.path));
        row.append(open); list.append(row);
      }
      status.textContent = d.total ? `共 ${d.total} 个文件夹` : '此目录没有子文件夹';
      current.hidden = !multiple; refreshCount(); done.disabled = false;
      prev.disabled = n === 0; next.disabled = (n + 1) * 50 >= d.total;
      paging.hidden = d.total <= 50;
      pageLabel.textContent = `${n + 1} / ${Math.max(1, Math.ceil(d.total / 50))}`;
    } catch (e) {
      if (id !== generation) return;
      status.textContent = e.message; list.replaceChildren();
      const retry = button('mx-button is-tonal', '返回内部存储'); retry.addEventListener('click', () => load('')); list.append(retry);
    }
  }
  currentCheck.addEventListener('change', () => { currentCheck.checked ? selected.add(path) : selected.delete(path); refreshCount(); });
  prev.addEventListener('click', () => load(path, page - 1));
  next.addEventListener('click', () => load(path, page + 1));
  function finish(value) { generation++; const fn = resolve; resolve = null; dialog.close(); if (fn) fn(value); }
  cancel.addEventListener('click', () => finish(null));
  dialog.addEventListener('cancel', e => { e.preventDefault(); finish(null); });
  done.addEventListener('click', () => finish(multiple ? [...selected].sort() : path));
  return {
    choose(roots = [], multi = false) {
      if (resolve) return Promise.resolve(null);
      multiple = multi; selected = new Set(multi ? roots : []);
      title.textContent = multi ? '选择监控目录' : '选择目标目录';
      dialog.showModal(); load(multi ? '' : roots[0] || '');
      return new Promise(r => resolve = r);
    },
    dialog,
  };
}
