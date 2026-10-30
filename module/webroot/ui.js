export function normalizeDisplay(value = {}) {
  if (!value || typeof value !== 'object') value = {};
  return {
    blur: value.blur !== false,
    floating: value.floating !== false,
    glass: value.glass === true,
    scale: [90, 95, 100, 105, 110, 115, 120].includes(value.scale) ? value.scale : 100,
  };
}

const titles = {scan: '文件', repair: '文件', trash: '回收站', watch: '监控', settings: '设置'};

export function initInterface(hydrateIcons = () => {}) {
  const root = document.documentElement, $ = id => document.getElementById(id);
  let settings;
  try { settings = normalizeDisplay(JSON.parse(localStorage.getItem('fm.display') || '{}')); } catch { settings = normalizeDisplay(); }
  hydrateIcons();

  // Reserve space for the fixed navigation bar and any visible floating action bar.
  function measureChrome() {
    const scale = settings.scale / 100, dock = document.querySelector('.navigation-dock');
    const nav = dock.getBoundingClientRect(), navSpace = (window.innerHeight - nav.top) / scale;
    root.style.setProperty('--mx-nav-space', Math.max(0, navSpace) + 'px');
    const bar = [...document.querySelectorAll('.cleanbar,.trash-actions')].find(e => e.getBoundingClientRect().height > 0);
    root.style.setProperty('--mx-bottom-space', (navSpace + 24 + (bar ? bar.getBoundingClientRect().height / scale + 12 : 0)) + 'px');
  }
  const topbar = $('topbar'), largeTitle = document.querySelector('.mx-large-title');
  function collapse() { topbar.classList.toggle('is-collapsed', largeTitle.getBoundingClientRect().bottom <= topbar.getBoundingClientRect().bottom + 4); }

  function repaint() {
    for (const key of ['blur', 'floating', 'glass']) { root.dataset[key] = String(settings[key]); $('display-' + key).checked = settings[key]; }
    $('display-glass').disabled = !settings.floating;
    root.style.setProperty('--ui-scale', String(settings.scale / 100));
    $('display-scale').value = settings.scale;
    $('display-scale').style.setProperty('--fill', ((settings.scale - 90) / 30 * 100) + '%');
    $('display-scale-value').textContent = settings.scale + '%';
    requestAnimationFrame(measureChrome);
  }
  function save() { try { localStorage.setItem('fm.display', JSON.stringify(settings)); } catch {} repaint(); }
  for (const key of ['blur', 'floating', 'glass']) $('display-' + key).addEventListener('change', () => { settings[key] = $('display-' + key).checked; save(); });
  $('display-scale').addEventListener('input', () => { settings.scale = Number($('display-scale').value); save(); });

  document.addEventListener('fm:page', event => {
    const page = event.detail.page;
    root.dataset.page = page;
    $('page-title').textContent = titles[page];
    $('topbar-title').textContent = titles[page];
    $('appearance-open').hidden = page === 'settings';
    requestAnimationFrame(() => { measureChrome(); collapse(); });
    window.scrollTo({top: 0, behavior: 'instant'});
  });
  $('appearance-open').addEventListener('click', () => $('nav-settings').click());
  document.addEventListener('keydown', event => {
    if (!event.target.matches('[role=tab]') || !['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
    event.preventDefault();
    const tabs = [...document.querySelectorAll('[role=tab]')], i = tabs.indexOf(event.target);
    const index = event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 : (i + (event.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length;
    tabs[index].click(); tabs[index].focus();
  });
  // Keep the trash badge hidden when empty.
  new MutationObserver(() => { const b = $('trash-count'); b.dataset.count = b.textContent.trim() || '0'; }).observe($('trash-count'), {childList: true, characterData: true, subtree: true});

  root.dataset.page = 'scan';
  const observer = new ResizeObserver(measureChrome);
  observer.observe(document.querySelector('.navigation-dock'));
  document.querySelectorAll('.cleanbar,.trash-actions').forEach(e => observer.observe(e));
  window.addEventListener('resize', measureChrome);
  window.addEventListener('scroll', collapse, {passive: true});
  repaint(); collapse();
}
