// Memory-only test bundle for restricted browser environments. Not shipped at runtime.
import ts from 'typescript';
import {readFile,readdir} from 'node:fs/promises';
let out='const modules = {}, cache = {};\n';
for(const file of await readdir('internal/webui/dist/ui'))if(file.endsWith('.js')){
 const source=await readFile(`internal/webui/dist/ui/${file}`,'utf8');
 const js=ts.transpileModule(source,{compilerOptions:{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.CommonJS}}).outputText;
 out+=`modules[${JSON.stringify(file)}] = function(exports, require){\n${js}\n};\n`;
}
out+='function require(id){ id=id.replace(/^.*\\//, "");if(cache[id])return cache[id];const exports={};cache[id]=exports;modules[id](exports,require);return exports;}\nrequire("App.js");';
process.stdout.write(out);
