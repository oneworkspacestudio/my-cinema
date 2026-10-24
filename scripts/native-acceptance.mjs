// Developer-only acceptance. Always creates a new synthetic installation.
import {spawn,execFileSync} from 'node:child_process';
import {randomBytes} from 'node:crypto';import {mkdtemp,mkdir,writeFile,readFile,copyFile,unlink} from 'node:fs/promises';
import {tmpdir} from 'node:os';import {dirname,join,resolve} from 'node:path';import {fileURLToPath} from 'node:url';
const repo=resolve(dirname(fileURLToPath(import.meta.url)),'..'),fixture=await mkdtemp(join(process.env.CINEMA_QA_PARENT||tmpdir(),'mycinema-qa-')),data=join(fixture,'data'),exe=resolve(process.env.CINEMA_QA_EXE||join(repo,'dist','MyCinema.exe')),password=randomBytes(24).toString('hex'),port=Number(process.env.CINEMA_QA_PORT||29320),origin=`http://localhost:${port}`;
await writeFile(join(fixture,'fixture-password.txt'),password,{mode:0o600});
const env={...process.env,WORKSPACE_BOOTSTRAP_PASSWORD:password};for(const k of ['WORKSPACE_OIDC_ISSUER','WORKSPACE_OIDC_CLIENT_ID','WORKSPACE_OIDC_CLIENT_SECRET','WORKSPACE_OWNER_SUBJECT','WORKSPACE_HOME_URL','MYCINEMA_RUNTIME'])delete env[k];
const proc=spawn(exe,['--data',data,'--listen',`127.0.0.1:${port}`,'--bootstrap-email','cinema@example.test','--no-browser'],{env,windowsHide:true,stdio:'ignore'});
const wait=async(fn,ms=180000)=>{const end=Date.now()+ms;while(Date.now()<end){if(proc.exitCode!==null)throw Error('Synthetic app exited during startup');try{if(await fn())return}catch{}await new Promise(r=>setTimeout(r,500))}throw Error('Synthetic app startup timed out')};
try{
 await wait(async()=>(await fetch(origin+'/healthz')).ok);
 const ff=join(data,'runtime','jellyfin','ffmpeg.exe');await wait(async()=>{await readFile(ff);return true});
 // Three originally generated films and two episodes; no third-party artwork.
 const media=join(fixture,'media');await mkdir(media,{recursive:true});const base=join(media,'source.mp4');
 const ffmpeg=args=>execFileSync(ff,['-hide_banner','-loglevel','error',...args],{windowsHide:true,stdio:'ignore'});
 ffmpeg(['-f','lavfi','-i','testsrc2=size=320x180:rate=10','-f','lavfi','-i','sine=frequency=440:sample_rate=48000','-t','360','-c:v','libx264','-preset','ultrafast','-crf','34','-pix_fmt','yuv420p','-c:a','aac','-b:a','32k','-movflags','+faststart',base]);
 for(const [name,rating] of [['Blue Horizon','GB-U'],['Midnight Signal','GB-18'],['Unrated Journey','']]){const dir=join(media,'Movies',name);await mkdir(dir,{recursive:true});await copyFile(base,join(dir,name+'.mp4'));await writeFile(join(dir,'movie.nfo'),`<movie><title>${name}</title><year>2026</year><plot>Original synthetic My Cinema acceptance film.</plot><mpaa>${rating}</mpaa><genre>Adventure</genre></movie>`);ffmpeg(['-f','lavfi','-i','color=c=0x234773:size=640x360','-frames:v','1','-update','1',join(dir,'poster.jpg')])}
 const series=join(media,'TV','The Little Observatory'),season=join(series,'Season 01');await mkdir(season,{recursive:true});await writeFile(join(series,'tvshow.nfo'),'<tvshow><title>The Little Observatory</title><year>2026</year><plot>Original synthetic series.</plot><mpaa>GB-U</mpaa></tvshow>');
 for(const [n,title] of [[1,'First Light'],[2,'Beyond the Clouds']]){const name=`The Little Observatory S01E0${n}`;ffmpeg(['-i',base,'-c','copy',join(season,name+'.mkv')]);await writeFile(join(season,name+'.nfo'),`<episodedetails><title>${title}</title><season>1</season><episode>${n}</episode><plot>Original synthetic episode ${n}.</plot><mpaa>GB-U</mpaa></episodedetails>`)}
 const runner=spawn(process.execPath,[join(repo,'web','scripts','acceptance.mjs')],{env:{...process.env,CINEMA_QA_ORIGIN:origin,CINEMA_QA_ROOT:fixture,CINEMA_QA_ARTIFACTS:join(fixture,'artifacts')},stdio:'inherit',windowsHide:true});await new Promise((resolve,reject)=>{runner.on('error',reject);runner.on('exit',code=>code===0?resolve():reject(Error('Native acceptance failed: '+code)))});
 console.log('Synthetic evidence retained at '+fixture);
}finally{if(proc.exitCode===null){proc.kill();await new Promise(r=>proc.once('exit',r))};await unlink(join(fixture,'fixture-password.txt')).catch(()=>{});console.log('Synthetic process stopped; original installation was not used.')}
