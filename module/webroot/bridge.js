const actions = new Set(['info','start','status','groups','trash','cancel','clean','restore','purge','repairs','repair-apply','directories','select','select-items','job-result']);

function encoded(data) {
  const bytes = new TextEncoder().encode(JSON.stringify(data));
  let text=''; for (const value of bytes) text+=String.fromCharCode(value);
  return btoa(text);
}

export function createBackend(exec, expectedVersion) {
  async function execute(action,data) {
    const raw = await exec(`/data/adb/modules/dupeguard/bin/dupeguard ${action} '${encoded(data)}'`);
    let response;
    try { response=JSON.parse(raw); } catch { throw new Error('后端响应无效，请关闭旧网页并重新打开模块。'); }
    if(!response.ok) throw new Error(response.error||'操作失败');
    return response.data;
  }
  return async function call(action,data={}) {
    if(!actions.has(action)) throw new Error('未知操作');
    if(action==='info') return execute(action,data);
    const info=await execute('info',{});
    if(info?.version!==expectedVersion) throw new Error(`页面与后端版本不一致（需要 ${expectedVersion}，当前 ${info?.version||'未知'}）。请退出旧网页、刷新模块列表后重新打开。`);
    return execute(action,data);
  };
}
