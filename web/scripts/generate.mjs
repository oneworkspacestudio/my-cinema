import {parse,compileReact} from '@codedia/parser';
import {readFile,mkdir,writeFile} from 'node:fs/promises';
await mkdir('src/generated',{recursive:true});
for(const name of ['Cinema','Profiles','Hero','Rails','Catalog','Details','Settings','AddVideos','Watch','Login']) {
 const result=compileReact(parse(await readFile(`src/${name}.aui`,'utf8')));
 const component=result.match(/export function (\w+)/)[1];
 await writeFile(`src/generated/${name}.tsx`,result+`\nexport default ${component};\n`);
}
