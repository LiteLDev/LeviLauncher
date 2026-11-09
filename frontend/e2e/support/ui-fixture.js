(() => {
  const scenario = new URLSearchParams(location.search).get('scenario') || 'normal';
  const isUWP = scenario.startsWith('uwp');
  const isEditorMinimum = scenario === 'uwp-editor-minimum';
  const state = window.__audit = { recovered: false, baseRoot: 'C:\\Fixture', scenario, processes: [{ pid: 4242, exePath: 'C:\\Fixture\\Minecraft.Windows.exe', isLauncher: true, versionName: 'UI test instance' }] };
  const sponsorParams = new URLSearchParams(location.search);
  state.openedSponsorURLs = [];
  state.openSponsorURL = async (url) => {
    if (scenario === 'sponsor-link-error' && !state.recovered) throw new Error('Fixture: browser unavailable');
    state.openedSponsorURLs.push(url);
  };
  localStorage.setItem('i18nextLng', 'zh_CN');
  if (scenario.startsWith('sponsor')) {
    localStorage.setItem('i18nextLng', sponsorParams.get('locale') || 'zh_CN');
    localStorage.setItem('app.themeMode', sponsorParams.get('theme') || 'light');
  }
  localStorage.setItem('ll.clarity.enabled', 'false');
  localStorage.setItem('ll.clarity.choiceMade', 'fixture');
  localStorage.setItem('ll.termsAccepted', 'fixture');
  if (scenario === 'sponsor-terms') localStorage.removeItem('ll.termsAccepted');
  if (scenario === 'sponsor-analytics') localStorage.removeItem('ll.clarity.choiceMade');
  if (scenario === 'instance-backup') localStorage.setItem('app.experimental.instanceBackup', 'true');
  localStorage.setItem('ll.currentVersionName', isUWP ? 'UWP release instance' : 'UI test instance');
  if (isUWP) localStorage.setItem('download.filters', JSON.stringify({ packageType: 'uwp', type: 'all', status: 'all', loader: 'all' }));
  localStorage.setItem('app.backgroundImage', scenario === 'wallpaper' ? 'C:\\Fixture\\wallpapers' : '');
  if (scenario === 'root-error') localStorage.removeItem('ll.onboarded'); else localStorage.setItem('ll.onboarded', 'fixture');
  state.catalogPackages = Array.from({ length: 45 }, (_, index) => ({
    id: `github.com/fixture/package-${index + 1}`, identifier: `github.com/fixture/package-${index + 1}`,
    name: `Preview package ${String(index + 1).padStart(2, '0')}`,
    description: 'A sample package used to check search, scrolling and pagination in a small launcher window.',
    owner: 'fixture', author: 'fixture', logo: '', avatarUrl: '', projectUrl: '', stars: 100 - index,
    updated: '2026-09-08', tags: ['platform:levilamina', 'type:mod', index % 3 ? 'utility' : 'economy'],
    side: ['client'], platform: ['win-x64'], gameVersions: ['1.21.0'], deps: { 'github.com/LiteLDev/LeviLamina': '>=0.1.0' },
    license: index % 2 ? 'MIT' : 'GPL-3.0', url: '', json: '', requiredBy: 0, order: index,
    versions: ['1.0.0'], llDependencyRanges: ['>=0.1.0'], variants: [{ key: 'client', packageIdentifier: `github.com/fixture/package-${index + 1}#client` }], preferredVariantKey: 'client',
  }));
  state.uwpVersions = Array.from({ length: 24 }, (_, index) => ({ version: `1.21.${100 - index}.0`, uuid: `00000000-0000-4000-8000-${String(index).padStart(12, '0')}`, type: ['release', 'beta', 'preview'][index % 3], packageType: 'uwp' }));
  if (scenario.startsWith('uwp-isolation-min')) state.uwpVersions[0].version = new URLSearchParams(location.search).get('version') || '';
  state.uwpMetas = ['release', 'beta', 'preview'].map((type, index) => ({ name: `UWP ${type} instance`, gameVersion: state.uwpVersions[index].version, type, packageType: 'uwp', enableIsolation: false, registered: index === 0 }));
  if (isEditorMinimum) {
    const params = new URLSearchParams(location.search);
    Object.assign(state.uwpMetas[0], { gameVersion: params.get('version') || '', type: params.get('channel') || 'release', packageType: params.get('packageType') || 'uwp', enableEditorMode: params.get('enabled') === 'true' });
  }
  if (scenario === 'uwp-isolation-min-settings') state.uwpMetas[0].enableIsolation = true;
  if (['uwp-unregistered', 'uwp-register-fail', 'uwp-register-unconfirmed', 'uwp-stale-register'].includes(scenario)) state.uwpMetas.forEach(meta => { meta.registered = false; });
  state.uwpRoots = { base: 'C:\\Fixture\\Packages\\Microsoft.MinecraftUWP_8wekyb3d8bbwe\\LocalState\\games\\com.mojang', usersRoot: '', isIsolation: false, isPreview: false, packageType: 'uwp' };
  Object.assign(state.uwpRoots, { comMojangRoot: state.uwpRoots.base, worlds: state.uwpRoots.base + '\\minecraftWorlds', resourcePacks: state.uwpRoots.base + '\\resource_packs', behaviorPacks: state.uwpRoots.base + '\\behavior_packs', skinPacks: state.uwpRoots.base + '\\skin_packs', screenshots: state.uwpRoots.base + '\\Screenshots' });
  state.call = async (name, ...args) => {
    state.calls = [...(state.calls || []), { name, args }];
    if (scenario === 'licenses') {
      const xuid = state.licenseAccount || '101';
      if (name === 'GetLocalUserId') return xuid;
      if (name === 'GetLocalUserGamertag') return xuid === '101' ? 'License account A' : 'License account B';
      if (name === 'XUserGetState') return 0;
      if (name === 'ResetSession') return '';
      if (name === 'SignIn') { state.licenseAccount = '202'; return ''; }
      if (name === 'CheckGameLicenses' || name === 'RefreshGameLicenses') return {
        xuid,
        release: xuid === '202' ? 'not_entitled' : name === 'RefreshGameLicenses' ? 'trial' : 'authorized',
        preview: 'authorized',
      };
    }
    if (/^(Set|Reset|Kill|Open|Start|Install|Register)/.test(name)) {
      const output = document.getElementById('audit-calls');
      if (output) output.textContent = name + ': ' + JSON.stringify(args);
    }
    await new Promise(resolve => setTimeout(resolve, 80));
    if (scenario.startsWith('lip-progress') && name === 'GetMods') return (state.fixtureMods = state.fixtureMods || ['Fixture mod A', 'Fixture mod B', 'Fixture mod C']).map(mod => ({ name: mod, entry: mod + '.dll', version: '1.0.0', type: 'native', author: 'Fixture', folder: mod }));
    // lip-progress-delete: only Fixture mod A is lip-managed (with a late final
    // update of its task, as Wails may deliver it); the others are deleted
    // directly, running no lip task.
    if (scenario === 'lip-progress-delete' && name === 'DeleteMod') {
      if (args[1] === 'Fixture mod A') { const result = await state.runLipTask('UninstallLIPPackage', args); const lateEmit = state.lipLastEmit; setTimeout(() => lateEmit('lip_task_progress', { progressId: 'task', stage: 'task', current: 1, total: 1, unit: 'packages', item: 'Fixture mod A@1.0.0', package: '', message: '', percentage: 100, done: true, indeterminate: false }), 30); state.fixtureMods = state.fixtureMods.filter(mod => mod !== args[1]); return result; }
      state.lipPaused++; await new Promise(resolve => state.lipReleases.push(resolve)); state.fixtureMods = state.fixtureMods.filter(mod => mod !== args[1]); return '';
    }
    if (scenario.startsWith('lip-progress') && name === 'DeleteMod') { const result = await state.runLipTask('UninstallLIPPackage', args); state.fixtureMods = state.fixtureMods.filter(mod => mod !== args[1]); return result; }
    if (scenario.startsWith('lip-progress') && name === 'GetLIPPackageInstallStates') return (args[1] || []).map(identifierKey => { const installed = !!state.llInstalled && /levilamina/i.test(identifierKey); return { identifierKey, state: { installed, explicitInstalled: installed, installedVersion: installed ? '0.2.0' : '', error: '' } }; });
    if (scenario.startsWith('lip-progress') && ['InstallLeviLamina', 'UninstallLeviLamina', 'InstallLIPPackage', 'UninstallLIPPackage', 'CacheClean'].includes(name)) return state.runLipTask(name, args);
    if (name === 'TakeSponsorPrompt') {
      const count = Number(sponsorParams.get('count') || '100');
      const milestone = scenario.startsWith('sponsor') && count > 0 && count % 100 === 0 && !state.sponsorTaken;
      state.sponsorTaken = true;
      return milestone ? count : 0;
    }
    if (scenario === 'sponsor-update' && name === 'CheckUpdate') return { isUpdate: true, version: '99.0.0', body: 'Fixture update notes' };
    if (scenario === 'sponsor-check-error' && name === 'CheckUpdate') throw new Error('Fixture: update service unavailable');
    if (scenario === 'mods-import') {
      if (name === 'OpenModFiles') return state.modFiles || ['C:\\Fixture\\example.mcmod'];
      if (name === 'ImportModZipPath' || name === 'ImportModDllPath') {
        if (state.holdImport) await new Promise(resolve => { state.releaseImport = resolve; });
        if (state.importDuplicate && !args.at(-1)) return 'ERR_DUPLICATE_FOLDER';
        return state.importError || '';
      }
    }
    if (scenario === 'instance-backup') {
      const gameData = { key: 'gameData', label: 'Game data', path: 'C:\\Fixture\\gameData', size: 52428800, selectable: true, exists: true, shared: false, modes: [], defaultMode: '', warnings: [] };
      if (name === 'GetInstanceBackupInfo') return { name: 'UI test instance', backupDir: 'C:\\Fixture\\backups', scopes: [gameData] };
      if (name === 'OpenBackupArchive') return ['C:\\Fixture\\backups\\UI_test_instance.zip'];
      if (name === 'InspectInstanceBackupArchive') return { formatVersion: 1, name: 'UI test instance', archivePath: args[0], archiveName: 'UI_test_instance.zip', gameVersion: '1.21.0', type: 'Release', createdAt: '2026-11-01T10:00:00Z', includedScopes: ['gameData'], scopes: [{ key: 'gameData', label: 'Game data', mode: '' }] };
      if (/^(BackupInstance|PreviewInstanceBackupRestoreConflicts|RestoreInstanceBackup)$/.test(name)) {
        await new Promise(resolve => { state.release = { ...state.release, [name]: resolve }; });
        if (name === 'BackupInstance') return { archivePath: 'C:\\Fixture\\backups\\UI_test_instance.zip', backupDir: 'C:\\Fixture\\backups', includedScopes: ['gameData'], errorCode: state.backupResultError || '' };
        if (name === 'PreviewInstanceBackupRestoreConflicts') return { archivePath: args[1].archivePath, includedScopes: ['gameData'], conflicts: [] };
        return { status: 'success', includedScopes: ['gameData'], scopeResults: [{ key: 'gameData', label: 'Game data', status: 'success' }] };
      }
    }
    if (scenario.includes('content-import')) {
      if (name === 'OpenContentFiles') return state.importFiles || ['C:\\Fixture\\example.mcworld'];
      if (name === 'ListDir' && String(args[0]).endsWith('users')) {
        return ['player-one', 'player-two', 'Shared'].map(player => ({ name: player, path: args[0] + '\\' + player, isDir: true }));
      }
      if (name === 'GetUserGamertagMap') return { 'player-one': 'Preview player', 'player-two': 'Second player' };
      if (name === 'IsMcpackSkinPackPath') return String(args[0]).endsWith('skin.mcpack');
      if (name.startsWith('Import')) {
        if (state.importError) return state.importError;
        if (state.importDuplicate && !args.at(-1)) return 'ERR_DUPLICATE_FOLDER';
        return '';
      }
    }
    if (isUWP) {
      if (name === 'IsVcRuntimeInstalled') return true;
      if (name === 'SaveVersionMeta') {
        if (scenario === 'uwp-isolation-save-error') return 'ERR_WRITE_META';
        const target = state.uwpMetas.find(meta => meta.name === args[0]);
        if (target) Object.assign(target, { gameVersion: args[1], type: args[2], enableIsolation: args[3], enableConsole: args[4], enableEditorMode: args[5], launchArgs: args[6], envVars: args[7] });
        return '';
      }
      if (name === 'RegisterVersionWithWdapp') {
        await new Promise(resolve => setTimeout(resolve, 900));
        if (scenario === 'uwp-register-fail' && !state.recovered) return 'ERR_UWP_QUERY: Fixture registration query failed';
        if (scenario !== 'uwp-register-unconfirmed') {
          const target = state.uwpMetas.find(meta => meta.name === args[0]);
          state.uwpMetas.forEach(meta => { if ((meta.type === 'preview') === (target?.type === 'preview')) meta.registered = meta.name === args[0]; });
        }
        state.registrationChanged = true;
        return 'success';
      }
      if (name === 'UnregisterVersionByName') { state.uwpMetas.forEach(meta => { if (meta.name === args[0]) meta.registered = false; }); return ''; }
      if (name === 'GetVersionMenuDetails') {
        const details = state.uwpMetas.map(meta => ({ name: meta.name, registered: meta.registered, leviLaminaInstalled: false, logoDataUrl: '' }));
        if (scenario === 'uwp-stale-register') await new Promise(resolve => setTimeout(resolve, 5000));
        return details;
      }
      if (name === 'LaunchVersionByName') {
        await new Promise(resolve => setTimeout(resolve, 700));
        const registered = state.uwpMetas.find(meta => meta.name === args[0])?.registered;
        return scenario === 'uwp-registration-lost' || !registered ? 'ERR_UWP_NOT_REGISTERED: register the selected UWP instance first' : '';
      }
      if (name === 'FetchUWPVersions') { if (scenario === 'uwp-error' && !state.recovered) throw new Error('Fixture: UWP version database unavailable'); return state.uwpVersions; }
      if (name === 'FetchHistoricalVersions') return { releaseVersions: [{ version: 'Release 1.26.0', urls: ['https://example.com/game.msixvc'] }], previewVersions: [] };
      if (name === 'GetContentRoots') {
        const meta = state.uwpMetas.find(meta => meta.name === args[0]);
        const isPreview = meta?.type === 'preview';
        const folder = isPreview ? 'Minecraft Bedrock Preview' : meta?.type === 'beta' ? 'Minecraft Bedrock Beta' : 'Minecraft Bedrock';
        const base = meta?.enableIsolation ? `C:\\Fixture\\versions\\${meta.name}\\${folder}` : `C:\\Fixture\\Packages\\${isPreview ? 'Microsoft.MinecraftWindowsBeta' : 'Microsoft.MinecraftUWP'}_8wekyb3d8bbwe\\LocalState`;
        const comMojangRoot = base + '\\games\\com.mojang';
        return { base, comMojangRoot, usersRoot: '', isIsolation: !!meta?.enableIsolation, isPreview, packageType: 'uwp', worlds: comMojangRoot + '\\minecraftWorlds', resourcePacks: comMojangRoot + '\\resource_packs', behaviorPacks: comMojangRoot + '\\behavior_packs', skinPacks: comMojangRoot + '\\skin_packs', screenshots: comMojangRoot + '\\Screenshots' };
      }
      if (name === 'ListVersionMetas' || name === 'ListVersionMetasWithRegistered') {
        if (name === 'ListVersionMetasWithRegistered' && state.registrationChanged) await new Promise(resolve => setTimeout(resolve, 600));
        return state.uwpMetas.map(meta => ({ ...meta }));
      }
      if (name === 'GetVersionMeta') {
        const meta = { ...(state.uwpMetas.find(meta => meta.name === args[0]) || state.uwpMetas[0]) };
        if (scenario === 'uwp-stale-register') await new Promise(resolve => setTimeout(resolve, 5000));
        return meta;
      }
      if (name === 'GetLocalVersionNames') return state.uwpMetas.map(meta => meta.name);
      if (name === 'GetAllVersionsStatus') return args[0].map(item => ({ version: item.short, type: item.type, packageType: item.packageType, isDownloaded: item.short === state.uwpVersions[0].version, isInstalled: false }));
      if (name === 'GetVersionStatusForPackage') return { version: args[0], type: args[1], packageType: args[2], isDownloaded: false, isInstalled: false };
      if (name === 'ResolveDownloadedUWP') return args[0] === state.uwpVersions[0].version ? 'C:\\Fixture\\installers\\Minecraft-UWP-Release-' + args[0] + '.appx' : '';
      if (name === 'StartUWPDownload') { if (scenario === 'uwp-download-error' && !state.recovered) throw new Error('ERR_UWP_DOWNLOAD_URL: Fixture: Windows Update link unavailable'); const channel = String(args[2]); return 'C:\\Fixture\\installers\\Minecraft-UWP-' + channel[0].toUpperCase() + channel.slice(1) + '-' + args[0] + '.appx'; }
      if (name === 'InstallExtractAppx') {
        if (scenario === 'uwp-install-error' && !state.recovered) return 'ERR_UWP_REGISTER: Fixture registration failure';
        const local = scenario === 'uwp-isolation-local';
        state.uwpMetas.push({ name: args[1], gameVersion: local ? '1.21.80.3' : state.uwpVersions[0].version, type: local ? 'preview' : args[2], packageType: 'uwp', enableIsolation: false, registered: true });
        return '';
      }
      if (name === 'ValidateVersionFolderName') return '';
    }
    if (scenario === 'no-instances' && /^(IsGameInputInstalled|IsGamingServicesInstalled|IsVcRuntimeInstalled)$/.test(name)) return true;
    if (scenario === 'launch-error') {
      if (name === 'IsGameInputInstalled' || name === 'IsGamingServicesInstalled' || name === 'IsVcRuntimeInstalled') return true;
      if (name === 'GetVersionMenuDetails') return [{ name: 'UI test instance', registered: false, leviLaminaInstalled: false, logoDataUrl: '' }];
      if (name === 'LaunchVersionByName') { await new Promise(resolve => setTimeout(resolve, 300)); return new URLSearchParams(location.search).get('result') || 'ERR_LAUNCH_GAME'; }
      if (name === 'GetLogsDir') return 'C:\\Fixture\\Logs';
    }
    if (scenario === 'gdk-launch') {
      if (name === 'IsGameInputInstalled' || name === 'IsGamingServicesInstalled' || name === 'IsVcRuntimeInstalled') return true;
      if (name === 'GetVersionMenuDetails') return [{ name: 'UI test instance', registered: false, leviLaminaInstalled: false, logoDataUrl: '' }];
      if (name === 'LaunchVersionByName') { await new Promise(resolve => setTimeout(resolve, 700)); return ''; }
    }
    if (/^(Set|Reset|Kill|Open|Start)/.test(name)) {
      const output = document.getElementById('audit-calls');
      if (output) output.textContent = name + ': ' + JSON.stringify(args);
    }
    if (name === 'GetBaseRoot') { if (scenario === 'root-error' && !state.recovered) throw new Error('Fixture: folder unavailable'); return state.baseRoot; }
    if (name === 'CanWriteToDir') return !String(args[0]).includes('invalid');
    if (name === 'SetBaseRoot') { state.baseRoot = args[0]; return ''; }
    if (name === 'ResetBaseRoot') { state.baseRoot = 'C:\\Fixture'; return ''; }
    if (name === 'GetInstallerDir') return 'C:\\Fixture\\installers';
    if (name === 'GetVersionsDir') return 'C:\\Fixture\\versions';
    if (name === 'GetLanguageNames') return [{ code: 'zh_CN', language: '简体中文' }, { code: 'en_US', language: 'English' }];
    if (name === 'GetAppVersion') return '1.0.0-preview';
    if (name === 'GetContentRoots') return { base: state.baseRoot, usersRoot: 'C:\\Fixture\\users', resourcePacks: 'C:\\Fixture\\resource_packs', behaviorPacks: 'C:\\Fixture\\behavior_packs', isIsolation: true, isPreview: false };
    if (name === 'ListDir') {
      const path = String(args[0]);
      if (path.endsWith('users')) { if (scenario === 'players-error' && !state.recovered) throw new Error('Fixture: player directory access denied'); return [{ name: 'player-one', path: path + '\\player-one', isDir: true }]; }
      if (path.endsWith('minecraftWorlds')) { if (scenario === 'worlds-error' && !state.recovered) throw new Error('Fixture: world directory access denied'); return scenario === 'empty' ? [] : [{ name: 'healthy', path: path + '\\healthy', isDir: true }, { name: 'broken', path: path + '\\broken', isDir: true }]; }
      return [];
    }
    if (name === 'GetWorldLevelName') { if (scenario === 'partial' && !state.recovered && String(args[0]).endsWith('broken')) throw new Error('Fixture: unreadable world'); return String(args[0]).endsWith('broken') ? 'Recovered world' : 'Healthy world'; }
    if (name === 'GetPathSize') return 1048576;
    if (name === 'GetPathModTime') return 1700000000;
    if (name === 'GetUserGamertagMap') return { 'player-one': 'Preview player' };
    if (name === 'GetLocalUserGamertag') return 'Preview player';
    if (name === 'ListServers') { if (scenario === 'servers-error' && !state.recovered) throw new Error('Fixture: servers unavailable'); return []; }
    if (name === 'ListMinecraftProcesses') return state.processes;
    if (name === 'KillProcess' || name === 'KillAllMinecraftProcesses') { if (scenario === 'process-error' && !state.recovered) return 'Fixture: Access is denied'; state.processes = []; return ''; }
    if (name === 'CheckUpdate') return { isUpdate: false };
    if (name === 'GetLipGithubProxy') return state.lipGithubProxy || 'https://gh-proxy.org/https://github.com';
    if (name === 'SetLipGithubProxy') { state.lipGithubProxy = args[0] || 'https://gh-proxy.org/https://github.com'; return ''; }
    if ((scenario.startsWith('catalog') || scenario.startsWith('lip-progress')) && name === 'FetchLeviLaminaVersionDB') return { '1.21.0': ['0.1.0', '0.2.0'] };
    if (scenario.startsWith('catalog') && name === 'GetCurseForgeGameVersions') return [{ id: 1, name: '1.21.0' }, { id: 2, name: '1.20.0' }];
    if (scenario.startsWith('catalog') && name === 'GetCurseForgeCategories') return [];
    if (scenario.startsWith('catalog') && name === 'SearchCurseForgeMods') {
      const matches = state.catalogPackages.filter(pkg => pkg.name.toLowerCase().includes(String(args[4] || '').toLowerCase()));
      const index = Number(args[8] || 0), pageSize = Number(args[7] || 20);
      return { data: matches.slice(index, index + pageSize).map(pkg => ({ id: Number(pkg.identifier.split('-').pop()), name: pkg.name, summary: pkg.description, authors: [{ name: pkg.author }], logo: { thumbnailUrl: '/src/assets/images/LeviLamina.png', url: '/src/assets/images/LeviLamina.png' }, downloadCount: pkg.stars, dateModified: pkg.updated, dateCreated: pkg.updated, categories: [], latestFiles: [], latestFilesIndexes: [] })), pagination: { index, pageSize, totalCount: matches.length } };
    }
    if (name === 'ListVersionMetas') return scenario === 'no-instances' ? [] : [{ name: 'UI test instance', gameVersion: '1.21.0', type: 'Release', enableIsolation: true }];
    if (name === 'ListPacksForVersion' && scenario === 'resource-packs') return Array.from({ length: 24 }, (_, index) => ({ name: 'Resource pack ' + String(index + 1).padStart(2, '0'), path: 'C:/Fixture/resource_packs/pack-' + (index + 1), manifest: { pack_type: 6, name: 'Resource pack ' + String(index + 1).padStart(2, '0'), description: 'A sample resource pack.', identity: { version: { major: 1, minor: 0, patch: 0 } } } }));
    if (name === 'ListPackDetailsForVersion' && scenario === 'resource-packs' && args[2] === 6) return Array.from({ length: 24 }, (_, index) => ({
      name: 'Resource pack ' + String(index + 1).padStart(2, '0'), path: 'C:/Fixture/resource_packs/pack-' + (index + 1),
      description: 'A sample resource pack for checking search, sorting, and pagination.', version: '1.0.0', modTime: 1700000000,
    }));
    if (name === 'GetVersionMeta') return { name: 'UI test instance', gameVersion: '1.21.0', type: 'Release', enableIsolation: true };
    if (name === 'GetLocalVersionNames') return ['UI test instance'];
    if (name === 'GetSunTimes') return {};
    if (name === 'StartMsixvcDownload') return 'C:\\Fixture\\installers\\retry.msixvc';
    if (name.startsWith('List') || name.includes('Versions') || name.includes('VersionDB')) return [];
    if (name.startsWith('Is') || name.startsWith('Can') || name.startsWith('GetEnable') || name.startsWith('GetDisable')) return false;
    return '';
  };
  // lip-progress scenarios replay measured lip task events. Each task stops at
  // its pauses until the test calls window.__audit.lipContinue(), which resumes
  // the oldest paused task; with state.lipFail set, the task fails after its
  // last pause.
  state.lipPaused = 0;
  state.lipReleases = [];
  state.lipContinue = () => { const release = state.lipReleases.shift(); if (release) release(); };
  state.runLipTask = async (name, args) => {
    const ll = 'github.com/LiteLDev/LeviLamina';
    const plans = {
      InstallLeviLamina: {
        method: 'Install', target: args[1], packages: [ll + '@' + args[2]], phases: [
          [{ progressId: 'resolve-1', stage: 'resolve', current: 1, total: 1, unit: 'packages', done: true },
          { progressId: 'resolve-2', stage: 'resolve', current: 7, total: 0, unit: 'packages', item: 'github.com/LiteLDev/bedrock-runtime-data@1.21.0', indeterminate: true },
          { progressId: 'dl-1', stage: 'download', package: ll + '@' + args[2], current: 1153434, total: 0, unit: 'bytes', item: 'https://github.com/LiteLDev/LeviLamina/releases/download/v' + args[2] + '/LeviLamina-windows-x64.zip', indeterminate: true }],
          [{ progressId: 'resolve-2', stage: 'resolve', current: 9, total: 0, unit: 'packages', done: true },
          { progressId: 'dl-1', stage: 'download', package: ll + '@' + args[2], current: 1153434, total: 0, unit: 'bytes', done: true },
          { progressId: 'task', stage: 'task', current: 1, total: 3, unit: 'packages', item: ll + '@' + args[2] },
          { progressId: 'git-1', stage: 'git_fetch', package: 'github.com/LiteLDev/bedrock-runtime-data@1.21.0', current: 212, total: 480, unit: 'objects' },
          { progressId: 'dl-2', stage: 'download', package: ll + '@' + args[2], current: 3565158, total: 9542041, unit: 'bytes', item: 'https://github.com/LiteLDev/LeviLamina/releases/download/v' + args[2] + '/LeviLamina-windows-x64.zip' }],
          [{ progressId: 'git-1', stage: 'git_fetch', current: 480, total: 480, unit: 'objects', done: true },
          { progressId: 'dl-2', stage: 'download', current: 9542041, total: 9542041, unit: 'bytes', done: true },
          { progressId: 'place-1', stage: 'place', package: ll + '@' + args[2], current: 9542041, total: 9542041, unit: 'bytes', done: true },
          { progressId: 'task', stage: 'task', current: 3, total: 3, unit: 'packages', done: true }]]
      },
      UninstallLeviLamina: {
        method: 'Uninstall', target: args[0], packages: [ll], phases: [
          [{ progressId: 'task', stage: 'task', current: 0, total: 2, unit: 'packages', item: ll + '@0.2.0' },
          { progressId: 'rm-1', stage: 'remove', package: ll + '@0.2.0', current: 152, total: 0, unit: 'files', item: 'plugins/LeviLamina/LeviLamina.dll', indeterminate: true },
          { progressId: 'sc-1', stage: 'scripts', package: ll + '@0.2.0', current: 1, total: 3, unit: 'commands', item: 'post-uninstall' }],
          [{ progressId: 'rm-1', stage: 'remove', current: 240, total: 240, unit: 'files', done: true },
          { progressId: 'sc-1', stage: 'scripts', current: 3, total: 3, unit: 'commands', done: true },
          { progressId: 'task', stage: 'task', current: 2, total: 2, unit: 'packages', done: true }]]
      },
      UninstallLIPPackage: {
        method: 'Uninstall', target: args[0], packages: [String(args[1] || '')], phases: [
          [{ progressId: 'task', stage: 'task', current: 0, total: 1, unit: 'packages', item: String(args[1] || '') + '@1.0.0' },
          { progressId: 'rm-1', stage: 'remove', package: String(args[1] || '') + '@1.0.0', current: 30, total: 120, unit: 'files', item: 'mods/' + String(args[1] || '') + '/' + String(args[1] || '') + '.dll' }],
          [{ progressId: 'rm-1', stage: 'remove', current: 120, total: 120, unit: 'files', done: true },
          { progressId: 'task', stage: 'task', current: 1, total: 1, unit: 'packages', done: true }]]
      },
      CacheClean: {
        method: 'CacheClean', target: '', packages: [], phases: [
          [{ progressId: 'cache_clean', stage: 'cache_clean', current: 412, total: 1380, unit: 'files' }],
          [{ progressId: 'cache_clean', stage: 'cache_clean', current: 1380, total: 1380, unit: 'files', done: true }]]
      },
    };
    const plan = plans[name] || { method: 'Install', target: args[0], packages: [String(args[1] || '')], phases: [[{ progressId: 'task', stage: 'task', current: 1, total: 1, unit: 'packages', done: true }]] };
    const taskId = 'lip-fixture-' + (state.lipTaskSeq = (state.lipTaskSeq || 0) + 1);
    const base = () => ({ taskId, method: plan.method, target: plan.target, packages: plan.packages, timestamp: Date.now() });
    const emit = (event, data) => window._wails.dispatchWailsEvent({ name: event, data: { ...base(), ...data } });
    state.lipLastEmit = emit;
    emit('lip_task_started', {});
    emit('lip_task_log', { level: 'info', message: 'Fixture task ' + plan.method, raw: '' });
    for (const [index, phase] of plan.phases.entries()) {
      for (const update of phase) emit('lip_task_progress', { message: '', percentage: update.total > 0 ? update.current * 100 / update.total : 0, package: '', item: '', done: false, indeterminate: false, ...update });
      if (index < plan.phases.length - 1) {
        state.lipPaused++;
        await new Promise(resolve => { state.lipReleases.push(resolve); });
      }
    }
    if (state.lipFail) {
      const error = 'Fixture: failed to remove plugins/LeviLamina (file in use)';
      emit('lip_task_log', { level: 'error', message: error, raw: error });
      emit('lip_task_finished', { success: false, error, stderr: '', durationMs: 1 });
      return 'ERR_LIP_PACKAGE_UNINSTALL_FAILED';
    }
    if (name === 'InstallLeviLamina') state.llInstalled = true;
    if (name === 'UninstallLeviLamina') state.llInstalled = false;
    emit('lip_task_finished', { success: true, error: '', stderr: '', durationMs: 1 });
    return '';
  };
  state.downloads = scenario === 'uwp-downloads' ? {
    'C:\\Fixture\\installers\\Minecraft-UWP-Release-1.21.100.0.appx': { dest: 'C:\\Fixture\\installers\\Minecraft-UWP-Release-1.21.100.0.appx', fileName: 'Minecraft-UWP-Release-1.21.100.0.appx', status: 'done', error: '', speed: 0, progress: { downloaded: 2048, total: 2048 }, installer: { version: '1.21.100.0', type: 'Release', packageType: 'uwp', uuid: state.uwpVersions[0].uuid, isLeviLaminaSupported: false } },
    'C:\\Fixture\\installers\\Minecraft-UWP-Beta-1.21.99.0.appx': { dest: 'C:\\Fixture\\installers\\Minecraft-UWP-Beta-1.21.99.0.appx', fileName: 'Minecraft-UWP-Beta-1.21.99.0.appx', status: 'error', error: 'Fixture: network unavailable', speed: 0, progress: null, url: 'uwp:' + state.uwpVersions[1].uuid, installer: { version: '1.21.99.0', type: 'Beta', packageType: 'uwp', uuid: state.uwpVersions[1].uuid, isLeviLaminaSupported: false } }
  } : scenario === 'downloads' ? {
    'C:\\Fixture\\installers\\Release 1.21.0.msixvc': { dest: 'C:\\Fixture\\installers\\Release 1.21.0.msixvc', fileName: 'Release 1.21.0.msixvc', status: 'done', error: '', speed: 0, progress: { downloaded: 2048, total: 2048 }, installer: { version: '1.21.0', type: 'Release', isLeviLaminaSupported: false } },
    'C:\\Fixture\\installers\\retry.msixvc': { dest: 'C:\\Fixture\\installers\\retry.msixvc', fileName: 'retry.msixvc', status: 'error', error: 'Fixture: network unavailable', speed: 0, progress: null, url: 'https://example.com/game.msixvc', md5sum: 'fixture-checksum', installer: { version: '1.21.1', type: 'Release', isLeviLaminaSupported: false } },
    'active.msixvc': { dest: 'active.msixvc', fileName: 'active.msixvc', status: 'started', error: '', speed: 1024, progress: { downloaded: 512, total: 4096 } }
  } : {};
  document.addEventListener('DOMContentLoaded', () => { const panel = document.createElement('aside'); panel.style.cssText = 'position:fixed;bottom:0;right:0;z-index:20000;background:#fff;color:#111;border:1px solid #777;padding:4px;font:12px sans-serif;max-width:420px'; panel.innerHTML = '<b>UI 预览 · 模拟接口，不执行系统操作</b> <button id="audit-recover">恢复模拟接口</button><output id="audit-calls" style="display:block;overflow-wrap:anywhere"></output>'; document.body.append(panel); panel.querySelector('button').onclick = () => { state.recovered = true; document.getElementById('audit-calls').textContent = '模拟接口已恢复'; }; });
})();
