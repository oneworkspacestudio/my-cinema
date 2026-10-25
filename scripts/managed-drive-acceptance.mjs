// Fresh synthetic NTFS installation only. Never accepts an existing data folder.
import {spawn,execFileSync} from 'node:child_process';
import {createRequire} from 'node:module';
import {randomBytes,createHash} from 'node:crypto';
import {mkdir,mkdtemp,readFile,writeFile,copyFile,stat} from 'node:fs/promises';
import {resolve,join} from 'node:path';
const require=createRequire(new URL('../web/package.json',import.meta.url));
const {chromium,expect}=require('@playwright/test');
const installer=process.env.CINEMA_QA_INSTALLER,exe=process.env.CINEMA_QA_EXE,video=process.env.CINEMA_QA_VIDEO,artifacts=process.env.CINEMA_QA_ARTIFACTS;
if(!installer||!exe||!video||!artifacts)throw Error('Explicit synthetic installer, candidate, original video and artifact paths required');
const root=await mkdtemp('C:/Projects/.cache/cinema-managed5-'),data=join(root,'Private'),files=join(root,'Files'),programs=join(root,'Apps'),profile=join(root,'Profile');
execFileSync('icacls',[root,'/inheritance:r','/grant:r',process.env.USERDOMAIN+'\\'+process.env.USERNAME+':(OI)(CI)F','SYSTEM:(OI)(CI)F'],{windowsHide:true,stdio:'ignore'});
const origin='http://localhost:39300',password=randomBytes(24).toString('hex'),checks=[],errors=[];
let setup,runtime,browser,diagnostic='';
await mkdir(artifacts,{recursive:true});
const env={...process.env,APPDATA:profile,LOCALAPPDATA:profile};for(const key of Object.keys(env))if(key.startsWith('WORKSPACE_')||key.startsWith('MYCINEMA_'))delete env[key];
const check=s=>{checks.push(s);console.log('PASS',s)};
const wait=async(fn,ms=240000)=>{const end=Date.now()+ms;while(Date.now()<end){try{if(await fn())return}catch{}await new Promise(r=>setTimeout(r,500))}throw Error('Fresh managed acceptance timed out')};
const stopped=p=>!p||p.exitCode!==null;
async function record(done=false){await writeFile(join(artifacts,'process.json'),JSON.stringify({root,data,files,programs,pid:runtime?.pid||setup?.pid,synthetic:true,stopped:done},null,2))}
async function start(){runtime=spawn(join(data,'Workspace.exe'),['--data',data,'run'],{env,windowsHide:true,stdio:['ignore','pipe','pipe']});runtime.stdout.on('data',()=>{});runtime.stderr.on('data',b=>diagnostic+=b);await record();await wait(async()=>{const s=await(await fetch(origin+'/healthz')).json();return s.apps?.cinema==='running'})}
async function stop(){if(stopped(runtime))return;execFileSync(join(data,'Workspace.exe'),['--data',data,'stop'],{env,windowsHide:true,stdio:'ignore',timeout:45000});await wait(()=>stopped(runtime),60000)}
try{
 await mkdir(join(profile,'Microsoft/Windows/Start Menu/Programs'),{recursive:true});
 setup=spawn(resolve(installer),['--offline','--headless','--no-browser'],{env,windowsHide:true,stdio:['pipe','pipe','pipe']});setup.stdout.on('data',()=>{});setup.stderr.on('data',b=>diagnostic+=b);await record();
 setup.stdin.end(JSON.stringify({programs,data,files,name:'Cinema QA',email:'cinema-managed@example.test',password,confirm:password,port:39300,mode:'local',selectedApps:['accounts','drive','cinema'],autostart:false,noShortcuts:true}));
 await new Promise((resolve,reject)=>{setup.on('error',reject);setup.on('exit',c=>c===0?resolve():reject(Error('Synthetic setup failed '+c)))});
 // The baseline install is a fixture provider for real Accounts/Drive. Run the
 // exact accepted candidate with the production catalog environment contract.
 await copyFile(exe,join(programs,'MyCinema.exe'));
 const config=JSON.parse(await readFile(join(data,'workspace.json'),'utf8')),app=config.catalog.apps.find(a=>a.id==='cinema');app.version='0.1.0-preview.5';app.env={MYCINEMA_DRIVE_ROOT:'${FILES}'};
 config.catalog.packages.cinema.sha256=createHash('sha256').update(await readFile(exe)).digest('hex');config.catalog.packages.cinema.size=(await stat(exe)).size;
 await writeFile(join(data,'workspace.json'),JSON.stringify(config));await start();
 for(const name of ['Movies','TV Shows','Home Videos'])expect((await stat(join(files,'Cinema',name))).isDirectory()).toBe(true);
 check('Exact candidate creates all three Cinema folders in the actual shared Drive files root on native startup');
 browser=await chromium.launch({channel:'msedge',headless:true});const context=await browser.newContext({viewport:{width:1440,height:1000}}),page=await context.newPage();page.on('pageerror',e=>errors.push(e.message));
 const signin=async(p,email)=>{await p.getByRole('textbox',{name:'Email or username',exact:true}).fill(email);await p.getByRole('button',{name:'Next',exact:true}).click();await p.getByRole('textbox',{name:'Password',exact:true}).fill(password);await p.getByRole('button',{name:'Sign In',exact:true}).click()};
 const api=async(p,path,method='GET',body)=>p.evaluate(async({path,method,body})=>{const me=await(await fetch('/cinema/api/me')).json();const r=await fetch('/cinema'+path,{method,headers:{'Content-Type':'application/json','X-CSRF-Token':me.csrf},body:body===undefined?undefined:JSON.stringify(body)});return {status:r.status,data:await r.json()}},{path,method,body});
 await page.goto(origin);await signin(page,'cinema-managed@example.test');await expect(page.getByRole('heading',{name:'Your workspace.',exact:true})).toBeVisible();await page.goto(origin+'/_workspace/open/cinema');await expect(page.getByRole('heading',{name:"Who's watching?",exact:true})).toBeVisible({timeout:90000});
 expect((await api(page,'/api/me')).data.user.issuer).toBe('http://localhost:39301');expect((await api(page,'/api/me')).data.user.role).toBe('owner');expect((await api(page,'/api/profiles')).data.pinRequired).toBe(false);
 await expect(page.getByPlaceholder('Profile PIN',{exact:true})).toHaveCount(0);await page.getByPlaceholder('Profile name',{exact:true}).fill('Managed Cinema');await page.getByRole('button',{name:'Create profile',exact:true}).click();await expect(page.getByRole('button',{name:'Add videos',exact:true}).first()).toBeVisible();
 check('Actual MyAccounts code/PKCE sign-in creates and selects an adult viewing profile without any PIN');
 const profiles=(await api(page,'/api/profiles')).data.profiles,adult=profiles[0];
 await page.getByRole('button',{name:'Add videos',exact:true}).first().click();
 const libraries=(await api(page,'/api/admin/settings')).data.libraries;expect(libraries.length).toBe(3);
 await mkdir(join(files,'Another folder'),{recursive:true});
 await page.locator('.row--library').filter({has:page.getByText('Movies',{exact:true})}).getByRole('button',{name:'Change folder',exact:true}).click();const picker=page.getByRole('dialog',{name:'Choose a My Drive folder',exact:true});await picker.getByRole('button',{name:'Another folder',exact:true}).click();await picker.getByRole('button',{name:'Use this folder',exact:true}).click();await expect(picker).toBeHidden();
 await copyFile(video,join(files,'Another folder','Shared Video.mp4'));
 const drive=await context.newPage();await drive.goto(origin+'/_workspace/open/drive');await wait(async()=>drive.evaluate(async()=>(await fetch('/drive/api/users?username=self')).ok),60000);const info=await drive.evaluate(async()=>await(await fetch('/drive/api/users?username=self')).json());
 let shared=false;for(const source of info.scopes||[]){const out=await drive.evaluate(async name=>{const r=await fetch('/drive/api/resources/download?'+new URLSearchParams({source:name,file:'/Another folder/Shared Video.mp4'}),{headers:{Range:'bytes=0-1023'}});return {status:r.status,size:(await r.arrayBuffer()).byteLength}},source.name);if((out.status===200||out.status===206)&&out.size>0){shared=true;break}}
 expect(shared).toBe(true);await drive.close();check('Changed destination is the same physical folder authenticated My Drive reads, without duplicate media storage');
 await stop();await start();await page.reload();await expect(page.getByRole('button',{name:'Add videos',exact:true}).first()).toBeVisible();await page.getByRole('button',{name:'Add videos',exact:true}).first().click();
 expect((await api(page,'/api/admin/settings')).data.libraries.find(l=>l.kind==='movies').path).toBe(join(files,'Another folder'));
 check('Normal native stop/restart preserves the owner-selected folder and never resets it to the default');
 const second=await browser.newContext(),other=await second.newPage();await other.goto(origin);await signin(other,'cinema-managed@example.test');await expect(other.getByRole('heading',{name:'Your workspace.',exact:true})).toBeVisible();await other.goto(origin+'/_workspace/open/cinema');await expect(other.getByRole('button',{name:'Add videos',exact:true}).first()).toBeVisible();await expect(other.getByPlaceholder('Profile PIN',{exact:true})).toHaveCount(0);await second.close();
 check('Independent OneAccount browser session selects the existing single profile without a PIN');
 const child=await api(page,'/api/profiles','POST',{name:'Kids QA',kind:'child',rating:12,avatar:'mint'});expect(child.status).toBe(201);await page.reload();await page.getByRole('button',{name:'Switch profile',exact:true}).click();await page.getByRole('button',{name:/Kids QA/}).click();expect((await api(page,'/api/admin/folders')).status).toBe(403);await page.getByRole('button',{name:'Switch profile',exact:true}).click();await page.getByRole('button',{name:/Managed Cinema/}).click();await expect(page.getByRole('button',{name:'Add videos',exact:true}).first()).toBeVisible();
 check('Kids retain restricted viewing and cannot browse owner folders; adult OneAccount profile switching needs no PIN');
 const identity=await context.newPage();await identity.goto('http://localhost:39301');const created=await identity.evaluate(async({password})=>{const r=await fetch('/api/add-user',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({owner:'myaccounts',name:'cinema-member',displayName:'Cinema member',email:'cinema-member@example.test',password,passwordType:'plain',type:'normal-user',isAdmin:false,signupApplication:'myaccounts',createdTime:new Date().toISOString()})});return await r.json()},{password});expect(created.status).toBe('ok');await identity.close();
 const guestContext=await browser.newContext(),guest=await guestContext.newPage();await guest.goto(origin);await signin(guest,'cinema-member@example.test');await expect(guest.getByRole('heading',{name:'Your workspace.',exact:true})).toBeVisible();await guest.goto(origin+'/_workspace/open/cinema');await expect(guest.getByRole('heading',{name:"Who's watching?",exact:true})).toBeVisible();expect((await api(guest,'/api/profiles')).data.profiles).toEqual([]);expect((await api(guest,'/api/profiles','POST',{name:'Unauthorized',kind:'adult',rating:18})).status).toBe(403);expect((await api(guest,'/api/profiles/select','POST',{id:adult.id})).status).toBe(404);expect((await api(guest,'/api/admin/folders')).status).toBe(403);await guestContext.close();
 check('Different real MyAccounts subject cannot access the owner household or browse Drive');
 await page.getByRole('button',{name:'Add videos',exact:true}).first().click();await page.locator('.row--library').filter({has:page.getByText('Movies',{exact:true})}).getByRole('button',{name:'Change folder',exact:true}).click();await page.getByRole('dialog',{name:'Choose a My Drive folder',exact:true}).getByRole('button',{name:'Cinema',exact:true}).click();await page.screenshot({path:join(artifacts,'oneaccount-drive-desktop.png')});await page.setViewportSize({width:390,height:844});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);await page.screenshot({path:join(artifacts,'oneaccount-drive-phone.png')});expect(errors).toEqual([]);
 await writeFile(join(artifacts,'acceptance.json'),JSON.stringify({version:'0.1.0-preview.5',checks,errors,realOIDC:true,exactCandidateSHA256:config.catalog.packages.cinema.sha256,syntheticProtectedNTFS:true,baselineInstaller:'0.3.0-preview.85',candidateInstalledIntoFreshFixture:true,wholeCinema5InstallerTest:false,liveOwnerDataUsed:false,cleanHost:false},null,2));
}catch(e){await writeFile(join(artifacts,'failure.txt'),String(e.stack)+'\n'+diagnostic);throw e}finally{await browser?.close();try{await stop()}catch{if(runtime&&!stopped(runtime))runtime.kill()}if(setup&&!stopped(setup))setup.kill();await record(true)}
