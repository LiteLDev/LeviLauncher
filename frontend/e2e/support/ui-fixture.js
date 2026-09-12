(() => {
const scenario = new URLSearchParams(location.search).get('scenario') || 'normal';
const isUWP = scenario.startsWith('uwp');
const state = window.__audit = { recovered: false, baseRoot: 'C:\\Fixture', scenario, processes: [{pid: 4242, exePath: 'C:\\Fixture\\Minecraft.Windows.exe', isLauncher: true, versionName: 'UI test instance'}] };
localStorage.setItem('i18nextLng', 'zh_CN');
localStorage.setItem('ll.clarity.enabled', 'false');
localStorage.setItem('ll.clarity.choiceMade', 'fixture');
localStorage.setItem('ll.termsAccepted', 'fixture');
localStorage.setItem('ll.currentVersionName', isUWP ? 'UWP release instance' : 'UI test instance');
if(isUWP) localStorage.setItem('download.filters', JSON.stringify({packageType:'uwp',type:'all',status:'all',loader:'all'}));
localStorage.setItem('app.backgroundImage', scenario === 'wallpaper' ? 'C:\\Fixture\\wallpapers' : '');
if (scenario === 'root-error') localStorage.removeItem('ll.onboarded'); else localStorage.setItem('ll.onboarded','fixture');
state.catalogPackages = Array.from({ length: 45 }, (_, index) => ({
  identifier: `fixture/package-${index + 1}`,
  name: `Preview package ${String(index + 1).padStart(2, '0')}`,
  description: 'A sample package used to check search, scrolling and pagination in a small launcher window.',
  author: 'Preview author', avatarUrl: '', projectUrl: '', hotness: 100 - index,
  updated: '2026-09-08', tags: ['platform:levilamina', 'type:mod'],
  versions: ['1.0.0'], llDependencyRanges: ['>=0.1.0'], variants: [], preferredVariantKey: '',
}));
state.uwpVersions = Array.from({length: 24}, (_, index) => ({version: `1.21.${100-index}.0`, uuid:`00000000-0000-4000-8000-${String(index).padStart(12,'0')}`,type: ['release','beta','preview'][index%3], packageType:'uwp'}));
state.uwpMetas = ['release','beta','preview'].map((type,index)=>({name:`UWP ${type} instance`,gameVersion:state.uwpVersions[index].version,type,packageType:'uwp',enableIsolation:false,registered:index===0}));
if (['uwp-unregistered','uwp-register-fail','uwp-register-unconfirmed','uwp-stale-register'].includes(scenario)) state.uwpMetas.forEach(meta=>{meta.registered=false;});
state.uwpRoots = {base:'C:\\Fixture\\Packages\\Microsoft.MinecraftUWP_8wekyb3d8bbwe\\LocalState\\games\\com.mojang', usersRoot:'',isIsolation:false,isPreview:false,packageType:'uwp'};
Object.assign(state.uwpRoots, {comMojangRoot:state.uwpRoots.base,worlds:state.uwpRoots.base+'\\minecraftWorlds',resourcePacks:state.uwpRoots.base+'\\resource_packs',behaviorPacks:state.uwpRoots.base+'\\behavior_packs',skinPacks:state.uwpRoots.base+'\\skin_packs',screenshots:state.uwpRoots.base+'\\Screenshots'});
state.call = async (name, ...args) => {
  state.calls = [...(state.calls || []), {name,args}];
  if (/^(Set|Reset|Kill|Open|Start|Install|Register)/.test(name)) {
    const output = document.getElementById('audit-calls');
    if(output) output.textContent = name + ': ' + JSON.stringify(args);
  }
 await new Promise(resolve => setTimeout(resolve, 80));
  if (isUWP) {
    if(name==='RegisterVersionWithWdapp') {
      await new Promise(resolve=>setTimeout(resolve,900));
      if(scenario==='uwp-register-fail'&&!state.recovered)return 'ERR_UWP_QUERY: Fixture registration query failed';
      if(scenario!=='uwp-register-unconfirmed') {
        const target=state.uwpMetas.find(meta=>meta.name===args[0]);
        state.uwpMetas.forEach(meta=>{if((meta.type==='preview')===(target?.type==='preview'))meta.registered=meta.name===args[0];});
      }
      state.registrationChanged=true;
      return 'success';
    }
    if(name==='UnregisterVersionByName') {state.uwpMetas.forEach(meta=>{if(meta.name===args[0])meta.registered=false;});return '';}
    if(name==='GetVersionMenuDetails') {
      const details=state.uwpMetas.map(meta=>({name:meta.name,registered:meta.registered,leviLaminaInstalled:false,logoDataUrl:''}));
      if(scenario==='uwp-stale-register')await new Promise(resolve=>setTimeout(resolve,5000));
      return details;
    }
    if(name==='LaunchVersionByName') {
      await new Promise(resolve=>setTimeout(resolve,700));
      const registered=state.uwpMetas.find(meta=>meta.name===args[0])?.registered;
      return scenario==='uwp-registration-lost'||!registered ? 'ERR_UWP_NOT_REGISTERED: register the selected UWP instance first' : '';
    }
    if(name==='FetchUWPVersions') {if(scenario==='uwp-error'&&!state.recovered)throw new Error('Fixture: UWP version database unavailable');return state.uwpVersions;}
    if(name==='FetchHistoricalVersions') return {releaseVersions:[{version:'Release 1.26.0',urls:['https://example.com/game.msixvc']}],previewVersions:[]};
    if(name==='GetContentRoots') return state.uwpRoots;
    if(name==='ListVersionMetas'||name==='ListVersionMetasWithRegistered') {
      if(name==='ListVersionMetasWithRegistered'&&state.registrationChanged)await new Promise(resolve=>setTimeout(resolve,600));
      return state.uwpMetas.map(meta=>({...meta}));
    }
    if(name==='GetVersionMeta') {
      const meta={...(state.uwpMetas.find(meta=>meta.name===args[0]) || state.uwpMetas[0])};
      if(scenario==='uwp-stale-register')await new Promise(resolve=>setTimeout(resolve,5000));
      return meta;
    }
    if(name==='GetLocalVersionNames') return state.uwpMetas.map(meta=>meta.name);
    if(name==='GetAllVersionsStatus') return args[0].map(item=>({version:item.short,type:item.type,packageType:item.packageType,isDownloaded:item.short===state.uwpVersions[0].version,isInstalled:false}));
    if(name==='GetVersionStatusForPackage') return {version:args[0],type:args[1],packageType:args[2],isDownloaded:false,isInstalled:false};
    if(name==='ResolveDownloadedUWP') return args[0]===state.uwpVersions[0].version ? 'C:\\Fixture\\installers\\Minecraft-UWP-Release-'+args[0]+'.appx' : '';
    if(name==='StartUWPDownload') {if(scenario==='uwp-download-error'&&!state.recovered)throw new Error('ERR_UWP_DOWNLOAD_URL: Fixture: Windows Update link unavailable');const channel=String(args[2]);return 'C:\\Fixture\\installers\\Minecraft-UWP-'+channel[0].toUpperCase()+channel.slice(1)+'-'+args[0]+'.appx';}
    if(name==='InstallExtractAppx') return scenario==='uwp-install-error'&&!state.recovered ? 'ERR_UWP_REGISTER: Fixture registration failure' : '';
    if(name==='ValidateVersionFolderName') return '';
  }
 if(scenario==='gdk-launch') {
   if(name==='IsGameInputInstalled'||name==='IsGamingServicesInstalled'||name==='IsVcRuntimeInstalled') return true;
   if(name==='GetVersionMenuDetails') return [{name:'UI test instance',registered:false,leviLaminaInstalled:false,logoDataUrl:''}];
   if(name==='LaunchVersionByName') {await new Promise(resolve=>setTimeout(resolve,700));return '';}
 }
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
 if (name==='GetLipStatus') return {installed:scenario.startsWith('catalog'),upToDate:true,currentVersion:'1.0.0',latestVersion:'1.0.0'};
 if (scenario.startsWith('catalog') && name==='FetchLeviLaminaVersionDB') return {'1.21.0':['0.1.0','0.2.0']};
 if (scenario.startsWith('catalog') && name==='GetCurseForgeGameVersions') return [{id:1,name:'1.21.0'},{id:2,name:'1.20.0'}];
 if (scenario.startsWith('catalog') && name==='GetCurseForgeCategories') return [];
 if (scenario.startsWith('catalog') && name==='SearchCurseForgeMods') {
  const matches = state.catalogPackages.filter(pkg => pkg.name.toLowerCase().includes(String(args[4] || '').toLowerCase()));
  const index = Number(args[8] || 0), pageSize = Number(args[7] || 20);
  return {data:matches.slice(index,index+pageSize).map(pkg=>({id:Number(pkg.identifier.split('-').pop()),name:pkg.name,summary:pkg.description,authors:[{name:pkg.author}],logo:{thumbnailUrl:'/src/assets/images/LeviLamina.png',url:'/src/assets/images/LeviLamina.png'},downloadCount:pkg.hotness,dateModified:pkg.updated,dateCreated:pkg.updated,categories:[],latestFiles:[],latestFilesIndexes:[]})),pagination:{index,pageSize,totalCount:matches.length}};
 }
 if (name==='ListVersionMetas') return [{name:'UI test instance',gameVersion:'1.21.0',type:'Release',enableIsolation:true}];
 if (name==='ListPacksForVersion' && scenario==='resource-packs') return Array.from({length:24},(_,index)=>({name:'Resource pack '+String(index+1).padStart(2,'0'),path:'C:/Fixture/resource_packs/pack-'+(index+1),manifest:{pack_type:6,name:'Resource pack '+String(index+1).padStart(2,'0'),description:'A sample resource pack.',identity:{version:{major:1,minor:0,patch:0}}}}));
 if (name==='GetPackInfo' && scenario==='resource-packs') return {name:'Resource pack '+String(args[0]).split('-').pop().padStart(2,'0'),description:'A sample resource pack for checking search, sorting, and pagination.',version:'1.0.0',iconDataUrl:''};
 if (name==='GetVersionMeta') return {name:'UI test instance',gameVersion:'1.21.0',type:'Release',enableIsolation:true};
 if (name==='GetLocalVersionNames') return ['UI test instance'];
 if (name==='GetSunTimes') return {};
 if (name==='StartMsixvcDownload') return 'C:\\Fixture\\installers\\retry.msixvc';
 if (name.startsWith('List')||name.includes('Versions')||name.includes('VersionDB')) return [];
 if (name.startsWith('Is')||name.startsWith('Can')||name.startsWith('GetEnable')||name.startsWith('GetDisable')) return false;
 return '';
};
state.downloads = scenario==='uwp-downloads' ? {
  'C:\\Fixture\\installers\\Minecraft-UWP-Release-1.21.100.0.appx': {dest:'C:\\Fixture\\installers\\Minecraft-UWP-Release-1.21.100.0.appx',fileName:'Minecraft-UWP-Release-1.21.100.0.appx',status:'done',error:'',speed:0,progress:{downloaded:2048,total:2048},installer:{version:'1.21.100.0',type:'Release',packageType:'uwp',uuid:state.uwpVersions[0].uuid,isLeviLaminaSupported:false}},
  'C:\\Fixture\\installers\\Minecraft-UWP-Beta-1.21.99.0.appx': {dest:'C:\\Fixture\\installers\\Minecraft-UWP-Beta-1.21.99.0.appx',fileName:'Minecraft-UWP-Beta-1.21.99.0.appx',status:'error',error:'Fixture: network unavailable',speed:0,progress:null,url:'uwp:'+state.uwpVersions[1].uuid,installer:{version:'1.21.99.0',type:'Beta',packageType:'uwp',uuid:state.uwpVersions[1].uuid,isLeviLaminaSupported:false}}
} : scenario==='downloads' ? {
 'C:\\Fixture\\installers\\Release 1.21.0.msixvc': {dest:'C:\\Fixture\\installers\\Release 1.21.0.msixvc',fileName:'Release 1.21.0.msixvc',status:'done',error:'',speed:0,progress:{downloaded:2048,total:2048},installer:{version:'1.21.0',type:'Release',isLeviLaminaSupported:false}},
 'C:\\Fixture\\installers\\retry.msixvc': {dest:'C:\\Fixture\\installers\\retry.msixvc',fileName:'retry.msixvc',status:'error',error:'Fixture: network unavailable',speed:0,progress:null,url:'https://example.com/game.msixvc',md5sum:'fixture-checksum',installer:{version:'1.21.1',type:'Release',isLeviLaminaSupported:false}},
 'active.msixvc':{dest:'active.msixvc',fileName:'active.msixvc',status:'started',error:'',speed:1024,progress:{downloaded:512,total:4096}}
} : {};
document.addEventListener('DOMContentLoaded',()=>{const panel=document.createElement('aside');panel.style.cssText='position:fixed;bottom:0;right:0;z-index:20000;background:#fff;color:#111;border:1px solid #777;padding:4px;font:12px sans-serif;max-width:420px';panel.innerHTML='<b>UI 预览 · 模拟接口，不执行系统操作</b> <button id="audit-recover">恢复模拟接口</button><output id="audit-calls" style="display:block;overflow-wrap:anywhere"></output>';document.body.append(panel);panel.querySelector('button').onclick=()=>{state.recovered=true;document.getElementById('audit-calls').textContent='模拟接口已恢复';};});
})();
