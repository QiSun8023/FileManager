export const accents=[['blue','蓝'],['green','绿'],['teal','青'],['purple','紫'],['orange','橙'],['rose','玫红']];
export function normalizePreferences(value={}){if(!value||typeof value!=='object')value={};return {accent:accents.some(a=>a[0]===value.accent)?value.accent:'blue',scheme:['system','light','dark','black'].includes(value.scheme)?value.scheme:'system',monet:value.monet===true};}
export function paletteTone(value){let rgb;if(/^#[a-f0-9]{6}([a-f0-9]{2})?$/i.test(value||''))rgb=[1,3,5].map(i=>parseInt(value.slice(i,i+2),16));else{const match=(value||'').match(/^rgba?\(\s*([\d.]+)[, ]+([\d.]+)[, ]+([\d.]+)/i);if(match)rgb=match.slice(1,4).map(Number);}if(!rgb)return null;return .2126*rgb[0]+.7152*rgb[1]+.0722*rgb[2]<128?'dark':'light';}
export function resolveMonetPalette(values,valid=v=>/^#[a-f0-9]{6}([a-f0-9]{2})?$/i.test(v||'')){
 const color=name=>(values[name]||'').trim();for(const name of ['primary','onPrimary','surface','onSurface'])if(!valid(color(name)))return null;
 const pick=(name,fallback)=>valid(color(name))?color(name):color(fallback);
 return {'--accent':color('primary'),'--on-accent':color('onPrimary'),'--soft':pick('primaryContainer','surface'),'--on-soft':pick('onPrimaryContainer','onSurface'),'--page':pick('background','surface'),'--card':color('surface'),'--ink':color('onSurface'),'--muted':pick('onSurfaceVariant','onSurface'),'--line':pick('outlineVariant','onSurface')};
}
export function initAppearance(){
 const root=document.documentElement,$=id=>document.getElementById(id),media=matchMedia('(prefers-color-scheme: dark)');let settings;try{settings=normalizePreferences(JSON.parse(localStorage.getItem('fm.appearance')||'{}'));}catch{settings=normalizePreferences();}
 let palette=null,link=null,generation=0,retryTimer=null,message='';const keys=['--accent','--on-accent','--soft','--on-soft','--page','--card','--ink','--muted','--line'];
 const save=()=>{try{localStorage.setItem('fm.appearance',JSON.stringify(settings));}catch{}};
 function repaint(){root.dataset.accent=settings.accent;root.dataset.tone=settings.scheme==='system'?(media.matches?'dark':'light'):settings.scheme;if(settings.monet&&palette)root.dataset.tone=paletteTone(palette['--card'])||root.dataset.tone;root.style.colorScheme=root.dataset.tone==='black'?'dark':root.dataset.tone;for(const key of keys)root.style.removeProperty(key);if(settings.monet&&palette)for(const [key,value]of Object.entries(palette))root.style.setProperty(key,value);root.dataset.monet=String(!!(settings.monet&&palette));$('appearance-scheme').value=settings.scheme;$('appearance-scheme-value').textContent=settings.monet&&palette?'跟随宿主':({system:'跟随系统',light:'浅色',dark:'深色',black:'纯黑'})[settings.scheme];document.querySelectorAll('[data-scheme]').forEach(b=>b.setAttribute('aria-pressed',String(b.dataset.scheme===settings.scheme)));$('appearance-scheme').disabled=!!(settings.monet&&palette);$('appearance-monet').checked=settings.monet;$('appearance-status').textContent=message;$('appearance-retry').hidden=!settings.monet;document.querySelectorAll('[data-palette]').forEach(b=>{b.disabled=!!(settings.monet&&palette);b.setAttribute('aria-pressed',String(b.dataset.palette===settings.accent));});}
 const unavailable='未读取到宿主配色，已使用手动配色。请在 KernelSU 外观设置中启用 Monet／动态配色，再返回重新读取；部分宿主版本或主题模式不提供配色。';
 function readMonet(){const css=getComputedStyle(root),values={};for(const key of ['primary','onPrimary','surface','onSurface','primaryContainer','onPrimaryContainer','background','onSurfaceVariant','outlineVariant'])values[key]=css.getPropertyValue('--'+key);palette=resolveMonetPalette(values,v=>CSS.supports('color',v)&&!v.includes('var('));return !!palette;}
 function refresh(attempt=0){
  if(retryTimer!==null){clearTimeout(retryTimer);retryTimer=null;}const current=++generation;palette=null;
  if(link){link.remove();link=null;}
  if(!settings.monet){message='';repaint();return;}
  if(!window.ksu){message='当前打开方式未提供宿主配色，已使用手动配色。请从支持 Monet 的 KernelSU／WebUI 宿主打开。';repaint();return;}
  message='正在读取宿主配色…';repaint();
  const sheet=document.createElement('link');link=sheet;sheet.rel='stylesheet';sheet.href='/internal/colors.css?fm='+Date.now()+'-'+current;
  const finish=loaded=>{if(current!==generation)return;if(loaded&&readMonet()){message='已启用宿主配色，明暗随宿主主题；关闭 Monet 后可手动切换。';repaint();return;}if(attempt<3){retryTimer=setTimeout(()=>refresh(attempt+1),[300,900,1800][attempt]);return;}message=unavailable;repaint();};
  sheet.onload=()=>finish(true);sheet.onerror=()=>finish(false);document.head.append(sheet);
 }
 $('appearance-retry').addEventListener('click',()=>refresh());
 for(const [id,label]of accents){const b=document.createElement('button');b.type='button';b.dataset.palette=id;b.setAttribute('aria-label',label+'色');const dot=document.createElement('span');dot.className='swatch-dot';const name=document.createElement('span');name.className='swatch-label';name.textContent=label;b.title=label+'色';b.append(dot,name);b.addEventListener('click',()=>{settings.accent=id;save();refresh();});$('accent-palette').append(b);}
 $('appearance-scheme').addEventListener('click',()=>$('scheme-dialog').showModal());$('scheme-close').addEventListener('click',()=>$('scheme-dialog').close());document.querySelectorAll('[data-scheme]').forEach(b=>b.addEventListener('click',()=>{settings.scheme=b.dataset.scheme;save();repaint();$('scheme-dialog').close();}));$('appearance-monet').addEventListener('change',()=>{settings.monet=$('appearance-monet').checked;save();refresh();});if(media.addEventListener)media.addEventListener('change',repaint);else media.addListener(repaint);document.addEventListener('visibilitychange',()=>{if(!document.hidden&&settings.monet)refresh();});refresh();
}
