type Source = {tracks?:{type:string;language?:string;index:number}[]};
const aliases:Record<string,string>={eng:'en',fra:'fr',fre:'fr',deu:'de',ger:'de',spa:'es',ita:'it',por:'pt',jpn:'ja',kor:'ko',zho:'zh',chi:'zh',ara:'ar',hin:'hi',ben:'bn',rus:'ru',nld:'nl',dut:'nl'};
function languageCode(raw:string){const code=raw.trim().toLowerCase().split(/[-_]/)[0];return aliases[code]||code}
export function preferredTrack(source:Source|undefined,type:string,language:string|undefined){
 if(!language||language.toLowerCase()==='off')return -1;
 const code=languageCode(language);
 return source?.tracks?.find(t=>t.type===type&&languageCode(t.language||'')===code)?.index??-1;
}
