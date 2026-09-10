(() => {
const scenario = new URLSearchParams(location.search).get('scenario') || 'normal';
const state = window.__audit = { recovered: false, baseRoot: 'C:\\Fixture', scenario, processes: [{pid: 4242, exePath: 'C:\\Fixture\\Minecraft.Windows.exe', isLauncher: true, versionName: 'UI test instance'}] };
localStorage.setItem('i18nextLng', 'zh_CN');
localStorage.setItem('ll.clarity.enabled', 'false');
localStorage.setItem('ll.clarity.choiceMade', 'fixture');
localStorage.setItem('ll.termsAccepted', 'fixture');
localStorage.setItem('ll.currentVersionName', 'UI test instance');
localStorage.setItem('app.backgroundImage', scenario === 'wallpaper' ? 'C:\\Fixture\\wallpapers' : '');
if (scenario === 'root-error') localStorage.removeItem('ll.onboarded'); else localStorage.setItem('ll.onboarded','fixture');
state.call = async (name, ...args) => {
 await new Promise(resolve => setTimeout(resolve, 80));
 if (/^(Set|Reset|Kill|Open|Start)/.test(name)) {
   const output = document.getElementById('audit-calls');
   if(output) output.textContent = name + ': ' + JSON.stringify(args);
 }
 if (name === 'GetBaseRoot') {if(scenario === 'root-error' && !state.recovered)throw new Error('Fixture: folder unavailable');return state.baseRoot;}
 if (name === 'CanWriteToDir') return !String(args[0]).includes('invalid');
 if (name === 'SetBaseRoot') {state.baseRoot=args[0];return '';}
 if (name === 'ResetBaseRoot') {state.baseRoot='C:\\Fixture';return '';}
 if (name === 'GetInstallerDir') return 'C:\\Fixture\\installers';
 if (name === 'GetVersionsDir') return 'C:\\Fixture\\versions';
 if (name === 'GetLanguageNames') return [{code:'zh_CN',language:'简体中文'},{code:'en_US',language:'English'}];
 if (name === 'GetAppVersion') return '1.0.0-preview';
 if (name === 'GetContentRoots') return {base:state.baseRoot,usersRoot:'C:\\Fixture\\users',resourcePacks:'C:\\Fixture\\resource_packs',behaviorPacks:'C:\\Fixture\\behavior_packs',isIsolation:true,isPreview:false};
 if (name === 'ListDir') {
  const path=String(args[0]);
  if(path.endsWith('users')) {if(scenario==='players-error'&&!state.recovered)throw new Error('Fixture: player directory access denied');return [{name:'player-one',path:path+'\\player-one',isDir:true}];}
  if(path.endsWith('minecraftWorlds')) {if(scenario==='worlds-error'&&!state.recovered)throw new Error('Fixture: world directory access denied');return scenario==='empty'?[]:[{name:'healthy',path:path+'\\healthy',isDir:true},{name:'broken',path:path+'\\broken',isDir:true}];}
  return [];
 }
 if (name==='GetWorldLevelName') {if(scenario==='partial'&&!state.recovered&&String(args[0]).endsWith('broken'))throw new Error('Fixture: unreadable world');return String(args[0]).endsWith('broken')?'Recovered world':'Healthy world';}
 if (name==='GetPathSize') return 1048576;
 if (name==='GetPathModTime') return 1700000000;
 if (name==='GetUserGamertagMap') return {'player-one':'Preview player'};
 if (name==='GetLocalUserGamertag') return 'Preview player';
 if (name==='ListServers') {if(scenario==='servers-error'&&!state.recovered)throw new Error('Fixture: servers unavailable');return [];}
 if (name==='ListMinecraftProcesses') return state.processes;
 if (name==='KillProcess'||name==='KillAllMinecraftProcesses') {if(scenario==='process-error'&&!state.recovered)return 'Fixture: Access is denied';state.processes=[];return '';}
 if (name==='CheckUpdate') return {isUpdate:false};
 if (name==='GetLipStatus') return {installed:false,upToDate:false,currentVersion:'',latestVersion:''};
 if (name==='GetResourceRulesStatus') return {installed:false,upToDate:false};
 if (name==='ListVersionMetas') return [{name:'UI test instance',gameVersion:'1.21.0',type:'Release',enableIsolation:true}];
 if (name==='GetVersionMeta') return {name:'UI test instance',gameVersion:'1.21.0',type:'Release',enableIsolation:true};
 if (name==='GetLocalVersionNames') return ['UI test instance'];
 if (name==='GetSunTimes') return {};
 if (name==='StartMsixvcDownload') return 'C:\\Fixture\\installers\\retry.msixvc';
 if (name.startsWith('List')||name.includes('Versions')||name.includes('VersionDB')) return [];
 if (name.startsWith('Is')||name.startsWith('Can')||name.startsWith('GetEnable')||name.startsWith('GetDisable')) return false;
 return '';
};
state.downloads = scenario==='downloads' ? {
 'C:\\Fixture\\installers\\Release 1.21.0.msixvc': {dest:'C:\\Fixture\\installers\\Release 1.21.0.msixvc',fileName:'Release 1.21.0.msixvc',status:'done',error:'',speed:0,progress:{downloaded:2048,total:2048},installer:{version:'1.21.0',type:'Release',isLeviLaminaSupported:false}},
 'C:\\Fixture\\installers\\retry.msixvc': {dest:'C:\\Fixture\\installers\\retry.msixvc',fileName:'retry.msixvc',status:'error',error:'Fixture: network unavailable',speed:0,progress:null,url:'https://example.com/game.msixvc',md5sum:'fixture-checksum',installer:{version:'1.21.1',type:'Release',isLeviLaminaSupported:false}},
 'active.msixvc':{dest:'active.msixvc',fileName:'active.msixvc',status:'started',error:'',speed:1024,progress:{downloaded:512,total:4096}}
} : {};
document.addEventListener('DOMContentLoaded',()=>{const panel=document.createElement('aside');panel.style.cssText='position:fixed;bottom:0;right:0;z-index:20000;background:#fff;color:#111;border:1px solid #777;padding:4px;font:12px sans-serif;max-width:420px';panel.innerHTML='<b>UI 预览 · 模拟接口，不执行系统操作</b> <button id="audit-recover">恢复模拟接口</button><output id="audit-calls" style="display:block;overflow-wrap:anywhere"></output>';document.body.append(panel);panel.querySelector('button').onclick=()=>{state.recovered=true;document.getElementById('audit-calls').textContent='模拟接口已恢复';};});
})();