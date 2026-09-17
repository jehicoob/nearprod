# Dependencias de terceros

Código del proyecto privado según LICENSE. Los terceros conservan sus licencias.

- Backend/CLI: biblioteca estándar de Go. No paquetes de terceros en go.mod. Binarios de revisión: Go1.23.2, licencia BSD de Go incluida en internal/webui/dist/vendor/go-LICENSE.txt. Consultar estado de soporte y seguridad en COMPILAR.md; no es un compilador vigente.
- React18.3.1 y ReactDOM18.3.1, MIT. UMD oficiales de producción y licencias conservadas en internal/webui/dist/vendor; incorporados en binario para servir el panel sin CDN.
- TypeScript5.9.3, Apache2.0; @types/react18.3.31, @types/react-dom18.3.7, @types/prop-types15.7.15, csstype3.2.3, MIT: desarrollo únicamente. Lockfile/integridades derivados de los archivos anteriores, no auditoría online nueva.
- Playwright/Python: dependencias de desarrollo y evidencia, no parte del runtime instalado.

No se empaqueta yaml/npm/Node como backend. No se distribuyen fuentes tipográficas ni node_modules. Docker, Colima y las imágenes de aplicaciones/motores no vienen en estos binarios; sus licencias/versiones corresponden a sus proveedores.
