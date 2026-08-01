# Decision record 0001: licencia open source de NearProd

- **Jira:** NPROD-5
- **Estado:** Propuesta — pendiente de aprobación humana
- **Fecha:** 2026-08-01
- **Responsable de la decisión:** Jehicoob López
- **Alcance:** código fuente y documentación originales publicados en este repositorio

## Contexto

NearProd combina scripts de automatización, configuración de Docker Compose y Traefik, un backend local en Node.js y una interfaz React/Vite. El proyecto necesita una licencia explícita antes de distribuirse como software open source.

La licencia del repositorio no reemplaza las licencias de dependencias, imágenes, marcas, ejemplos o contribuciones de terceros. Cada tercero conserva sus propios avisos y condiciones.

## Criterios de decisión

1. Permitir uso personal y comercial, modificación y redistribución.
2. Facilitar adopción e integración por personas y organizaciones.
3. Mantener obligaciones de cumplimiento comprensibles para distribuidores.
4. Proteger a mantenedores y contribuyentes mediante exclusión de garantías y responsabilidad.
5. Tratar de forma explícita las concesiones de patentes cuando sea práctico.
6. Ser compatible con las dependencias actuales y no atribuir al proyecto derechos sobre componentes de terceros.

## Alternativas

| Alternativa | Uso comercial | Redistribución | Contribuciones y derivados | Patentes | Compatibilidad y riesgos relevantes |
| --- | --- | --- | --- | --- | --- |
| **MIT** | Permitido | Permitida conservando copyright y licencia | Los derivados pueden usar otra licencia | Sin concesión expresa de patentes | Muy simple y ampliamente compatible. Su tratamiento implícito de patentes ofrece menos claridad que Apache-2.0. |
| **Apache-2.0** | Permitido | Permitida conservando licencia, avisos, cambios relevantes y `NOTICE` cuando aplique | Los derivados pueden usar otra licencia, respetando las condiciones sobre material Apache | Concesión expresa y cláusula de terminación por litigio de patentes | Compatible con las dependencias permisivas detectadas. Requiere más disciplina de avisos que MIT. Es incompatible con GPL-2.0-only sin excepciones, aunque es compatible con GPLv3. |
| **MPL-2.0** | Permitido | Permitida | Copyleft a nivel de archivo: los archivos cubiertos modificados deben permanecer bajo MPL; puede combinarse con archivos de otras licencias | Concesión expresa | Adecuada para exigir reciprocidad limitada. Añade obligaciones de publicación de fuentes por archivo y mayor complejidad de cumplimiento. |
| **GPL-3.0-only** | Permitido | Permitida si se entrega el código fuente correspondiente y se mantienen las mismas libertades | Copyleft fuerte sobre trabajos derivados distribuidos | Concesión expresa y protecciones frente a restricciones adicionales | Puede reducir adopción e integración propietaria. Obliga a analizar con cuidado qué constituye un trabajo derivado o combinado. |
| **AGPL-3.0-only** | Permitido | Condiciones equivalentes a GPLv3 | Copyleft fuerte, incluida una obligación de ofrecer el código fuente correspondiente a usuarios que interactúan con una versión modificada por red | Concesión expresa | La obligación de interacción por red puede ser innecesaria para una herramienta local y reducir adopción. No evita por sí sola ofrecer el software como servicio sin modificaciones. |

## Compatibilidad con el árbol actual de dependencias

Se revisó `ui/package-lock.json` en la revisión base `869b954fa52d62926a5aab03fadbb66bc5a8a093`:

- Las 226 entradas de paquetes de dependencias declaran licencias identificadas. La entrada raíz de metadatos del proyecto no declara una licencia porque NPROD-6 todavía no ha aplicado la opción aprobada.
- Distribución observada entre dependencias: MIT (206), ISC (9), Apache-2.0 (5), BSD-2-Clause (2), BSD-3-Clause (2), MIT-0 (1) y CC-BY-4.0 (1).
- Las dependencias directas usan MIT, ISC o Apache-2.0.
- `caniuse-lite` declara CC-BY-4.0 para sus datos. Esto exige conservar la atribución correspondiente cuando esos datos se redistribuyan; no convierte el código original de NearProd en CC-BY-4.0.
- No se detectaron dependencias directas o transitivas declaradas bajo licencias copyleft en el lockfile actual.

Este inventario es una fotografía y debe repetirse cuando cambien dependencias. La compatibilidad de licencia no elimina obligaciones individuales de copyright, atribución o avisos de terceros.

## Recomendación

**Adoptar Apache License 2.0 para el código y la documentación originales de NearProd.**

Rationale:

- conserva un modelo permisivo apropiado para adopción personal y comercial;
- añade una concesión expresa de patentes y una defensa clara frente a litigios de patentes;
- es compatible con las dependencias permisivas actualmente declaradas;
- permite contribuciones y derivados sin imponer copyleft sobre integraciones;
- sus obligaciones adicionales de avisos y marcado de cambios son manejables para el tamaño y distribución previstos del proyecto.

MIT es la segunda opción si se prioriza simplicidad máxima sobre claridad explícita en patentes. MPL-2.0, GPL-3.0-only y AGPL-3.0-only solo deberían elegirse si el responsable decide que la reciprocidad de código es un objetivo de producto superior a la adopción permisiva.

## Riesgos y mitigaciones

- **No es asesoría legal:** una persona responsable debe aprobar la decisión y solicitar revisión jurídica externa si existen requisitos corporativos, marcas, patentes conocidas o distribución regulada.
- **Avisos de terceros:** mantener un inventario reproducible y añadir atribuciones/avisos cuando la distribución los requiera.
- **Contribuciones:** documentar en `CONTRIBUTING.md` que las contribuciones se ofrecen bajo la licencia del proyecto; evaluar un DCO o CLA solo si aparece una necesidad real.
- **Imágenes y herramientas externas:** Docker, Traefik, Colima y demás componentes no quedan relicenciados por este documento.
- **Contenido no software:** revisar por separado marcas, logotipos, capturas, datasets y otros activos si se incorporan.

## Decisión humana requerida

Marcar exactamente una opción durante la revisión:

- [ ] **Aprobar Apache-2.0** (recomendación)
- [ ] Aprobar MIT
- [ ] Aprobar MPL-2.0
- [ ] Aprobar GPL-3.0-only
- [ ] Aprobar AGPL-3.0-only
- [ ] Bloquear la decisión y registrar responsable, motivo y siguiente acción

Una vez aprobada la opción, NPROD-6 debe aplicar el texto oficial de la licencia, el identificador SPDX y los avisos necesarios en una rama y PR separados.

## Fuentes primarias

- SPDX — MIT: <https://spdx.org/licenses/MIT.html>
- SPDX — Apache-2.0: <https://spdx.org/licenses/Apache-2.0.html>
- SPDX — MPL-2.0: <https://spdx.org/licenses/MPL-2.0.html>
- SPDX — GPL-3.0-only: <https://spdx.org/licenses/GPL-3.0-only.html>
- SPDX — AGPL-3.0-only: <https://spdx.org/licenses/AGPL-3.0-only.html>
