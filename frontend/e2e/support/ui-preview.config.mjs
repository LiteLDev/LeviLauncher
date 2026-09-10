import baseConfig from '../../vite.config.js';
import { readFileSync } from 'node:fs';
export default (env) => {
 if (env.command !== "serve") throw new Error("UI fixtures are only available in the development preview.");
 const config=baseConfig(env);
 return {...config,plugins:[{
  name:'ui-audit-fixtures',enforce:'pre',
  transformIndexHtml(){return [{tag:'script',children:readFileSync(new URL('./ui-fixture.js',import.meta.url),'utf8'),injectTo:'head-prepend'}];},
  transform(code,id){
   if(id.replaceAll('\\','/').includes('/bindings/')) return code.replace(/export function (\w+)\([^)]*\) \{[\s\S]*?\n\}/g,(body,name)=>body.replace(/\$Call\.ByID\(\d+/, 'window.__audit.call('+JSON.stringify(name)));
   if(id.replaceAll('\\','/').endsWith('/utils/DownloadsContext.tsx')) return code.replace(/const \[downloadsMap, setDownloadsMap\] = useState<[\s\S]*?>\(\{\}\);/,'const [downloadsMap, setDownloadsMap] = useState<Record<string, DownloadItem>>(window.__audit.downloads);');
  }
 },...config.plugins],server:{host:'127.0.0.1',port:5174,strictPort:true}};
};