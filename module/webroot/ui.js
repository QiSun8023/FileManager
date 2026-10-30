const paths={
 moon:['M20 14.5A8.5 8.5 0 0 1 9.5 4 8.5 8.5 0 1 0 20 14.5'],
 copies:['M8 8h12v12H8z','M16 8V4H4v12h4'],
 repair:['M14.5 6.5a5 5 0 0 0-6.4 6.4L3 18l3 3 5.1-5.1a5 5 0 0 0 6.4-6.4l-3 3-3-3z'],
 trash:['M4 7h16','M9 7V4h6v3','M6 7l1 14h10l1-14','M10 11v6','M14 11v6'],
 settings:['M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8','M9 3h6l.6 2.2 2 .9 2.1-.6 3 5.1-1.6 1.6v2.3l1.6 1.5-3 5.2-2.2-.6-2 .9L15 24H9l-.6-2.2-2-.9-2.1.6-3-5.2 1.6-1.5v-2.3l-1.6-1.6 3-5.1 2.1.6 2-.9z'],
 sun:['M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8','M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.4 1.4m11.2 11.2L19 19M5 19l1.4-1.4M17.6 6.4 19 5'],
 palette:['M12 3a9 9 0 1 0 0 18h1a2 2 0 0 0 1.5-3.3 1.5 1.5 0 0 1 1.2-2.4H18a3 3 0 0 0 3-3.1A9 9 0 0 0 12 3','M7 10h.01M10 7h.01M15 7h.01M18 10h.01'],
 layers:['m12 3 10 5-10 5L2 8z','m2 12 10 5 10-5','m2 16 10 5 10-5'],
 dock:['M3 5h18v14H3z','M7 14h10v3H7z'],
 glass:['m12 2 9 5v10l-9 5-9-5V7z','m3 7 9 5 9-5','M12 12v10'],
 folder:['M3 6a2 2 0 0 1 2-2h5l2 3h7a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z']
};
export function normalizeDisplay(value={}){if(!value||typeof value!=='object')value={};return {blur:value.blur!==false,floating:value.floating!==false,glass:value.glass===true,scale:[90,95,100,105,110,115,120].includes(value.scale)?value.scale:100};}
export function initInterface(){
 const root=document.documentElement,$=id=>document.getElementById(id);let settings;try{settings=normalizeDisplay(JSON.parse(localStorage.getItem('fm.display')||'{}'));}catch{settings=normalizeDisplay();}
 document.querySelectorAll('[data-icon]').forEach(host=>{const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');svg.setAttribute('viewBox',host.dataset.icon==='settings'?'0 0 25 27':'0 0 24 24');svg.setAttribute('fill','none');svg.setAttribute('stroke','currentColor');svg.setAttribute('stroke-width','1.7');svg.setAttribute('stroke-linecap','round');svg.setAttribute('stroke-linejoin','round');svg.setAttribute('aria-hidden','true');for(const d of paths[host.dataset.icon]||[]){const path=document.createElementNS(svg.namespaceURI,'path');path.setAttribute('d',d);svg.append(path);}host.prepend(svg);});
 function measureChrome(){const scale=settings.scale/100,dock=document.querySelector('.navigation-dock');root.style.setProperty('--nav-space',(dock.getBoundingClientRect().height/scale+10)+'px');const bar=[...document.querySelectorAll('.cleanbar,.trash-actions')].find(e=>e.getBoundingClientRect().height>0);root.style.setProperty('--action-space',bar?(bar.getBoundingClientRect().height/scale+20)+'px':'0px');}
 function repaint(){for(const key of ['blur','floating','glass']){root.dataset[key]=String(settings[key]);$('display-'+key).checked=settings[key];}$('display-glass').disabled=!settings.floating;root.style.setProperty('--ui-scale',String(settings.scale/100));$('display-scale').value=settings.scale;$('display-scale-value').textContent=settings.scale+'%';requestAnimationFrame(measureChrome);}
 function save(){try{localStorage.setItem('fm.display',JSON.stringify(settings));}catch{}repaint();}
 for(const key of ['blur','floating','glass'])$('display-'+key).addEventListener('change',()=>{settings[key]=$('display-'+key).checked;save();});$('display-scale').addEventListener('input',()=>{settings.scale=Number($('display-scale').value);save();});
 const labels={scan:'重复文件',repair:'文件修复',trash:'回收站',watch:'监控',settings:'设置'};
 document.addEventListener('fm:page',event=>{root.dataset.page=event.detail.page;$('page-title').textContent=labels[event.detail.page];requestAnimationFrame(measureChrome);window.scrollTo({top:0,behavior:'instant'});});
 $('appearance-open').addEventListener('click',()=>$('nav-settings').click());
 document.addEventListener('keydown',event=>{if(event.target.matches('[role=tab]')&&['ArrowLeft','ArrowRight','Home','End'].includes(event.key)){event.preventDefault();const tabs=[...document.querySelectorAll('[role=tab]')],i=tabs.indexOf(event.target);const index=event.key==='Home'?0:event.key==='End'?tabs.length-1:(i+(event.key==='ArrowRight'?1:-1)+tabs.length)%tabs.length;tabs[index].click();tabs[index].focus();}});
 root.dataset.page='scan';const observer=new ResizeObserver(measureChrome);observer.observe(document.querySelector('.navigation-dock'));document.querySelectorAll('.cleanbar,.trash-actions').forEach(e=>observer.observe(e));window.addEventListener('resize',measureChrome);repaint();
}
