export type UploadControl={cancelled:boolean,xhr?:XMLHttpRequest};

// Small same-origin requests work through the Workspace proxy and its request
// timeout. The server's acknowledged offset is authoritative after a lost reply.
export async function uploadVideo(file:File,library:string,base:string,csrf:string,profile:string,control:UploadControl,onProgress:(bytes:number)=>void){
 const headers={'X-CSRF-Token':csrf,'X-Cinema-Profile':profile};
 const json=async(path:string,method='GET',body?:unknown)=>{
  const r=await fetch(base+path,{method,headers:{...headers,'Content-Type':'application/json'},credentials:'same-origin',body:body===undefined?undefined:JSON.stringify(body)});
  const out=await r.json();if(!r.ok)throw new Error(out.error||'The upload could not finish.');return out;
 };
 let id='';
 try{
  if(control.cancelled)throw new Error('Upload cancelled.');
  const job=await json('/api/admin/uploads','POST',{library,name:file.name,size:file.size});id=job.id;
  let offset=job.offset,failures=0;
  while(offset<file.size){
   if(control.cancelled)throw new Error('Upload cancelled.');
   const start=offset,end=Math.min(file.size,start+job.chunkSize);
   try{
    const result:any=await new Promise((resolve,reject)=>{
     const xhr=new XMLHttpRequest();control.xhr=xhr;xhr.open('PUT',`${base}/api/admin/uploads/${id}?offset=${start}`);xhr.responseType='json';xhr.timeout=110000;
     for(const [key,value] of Object.entries(headers))xhr.setRequestHeader(key,value);
     xhr.setRequestHeader('Content-Type','application/octet-stream');
     xhr.upload.onprogress=e=>onProgress(start+e.loaded);
     xhr.onload=()=>xhr.status>=200&&xhr.status<300?resolve(xhr.response):reject(new Error(xhr.response?.error||'Upload connection interrupted.'));
     xhr.onerror=()=>reject(new Error('Upload connection interrupted.'));xhr.ontimeout=()=>reject(new Error('Upload timed out.'));xhr.onabort=()=>reject(new Error('Upload cancelled.'));
     xhr.send(file.slice(start,end));if(control.cancelled)xhr.abort();
    });
    offset=result.offset;failures=0;
   }catch(e){
    if(control.cancelled||++failures>2)throw e;
    const state=await json(`/api/admin/uploads/${id}`);offset=state.offset;
   }
   onProgress(offset);
  }
  if(control.cancelled)throw new Error('Upload cancelled.');
  let result;
  try{result=await json(`/api/admin/uploads/${id}/complete`,'POST',{})}
  catch(e){const state=await json(`/api/admin/uploads/${id}`).catch(()=>null);if(!state?.completed)throw e;result={uploaded:true,name:state.name,scanning:true}}
  id='';return result;
 }finally{
  control.xhr=undefined;
  // Only this unfinished job is removed; a completed video is never deleted.
  if(id)await json(`/api/admin/uploads/${id}`,'DELETE').catch(()=>{});
 }
}
