# Skill Codex — local-infra-projects

Se creó un skill instalable en:

```txt
~/.codex/skills/local-infra-projects
```

Úsalo en futuras sesiones cuando quieras que Codex recuerde cómo agregar proyectos al Local Infra Control Panel editando archivos.

Ejemplos de prompts que deberían activar el skill:

```txt
Usa local-infra-projects para agregar este backend y frontend al panel.
Agrega un nuevo proyecto a ~/local-infra/projects.yml.
Configura Traefik para mi proyecto local con Colima.
Registra este repo clonado en local-infra editando archivos.
```

El skill contiene:

```txt
local-infra-projects/
├── SKILL.md
├── agents/openai.yaml
└── references/project-configuration.md
```

La referencia principal del proyecto para humanos está en:

```txt
~/local-infra/docs/agregar-proyectos-configuracion.md
```
