// Development build only. Node never participates in the installed controller.
import {cp,mkdir} from 'node:fs/promises';
await mkdir('internal/webui/dist',{recursive:true});
await cp('ui/public','internal/webui/dist',{recursive:true});
console.log('UI estática lista para go:embed; se conservan vendor React y licencias.');
