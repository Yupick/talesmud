# Índice de documentación — fork `mi-mud`

Índice navegable de **todo** lo que existe hoy en `docs/`, más los documentos de
raíz que lo referencian.

**Leyenda de estados**

| Estado | Significado |
|--------|-------------|
| **hecho** | Describe código implementado hoy (guía operativa o referencia de sistemas que ya existen) |
| **diseño** | Plan, PRD o especificación; **no** está implementado en el código |
| **mixto** | Mezcla de sistema descrito y backlog pendiente |
| **diseño → hecho** | El doc de diseño quedó desactualizado: el código **ya** lo implementa (revisar FEATURES.md/PROJECT.md) |
| **en curso** | El diseño está cerrado y la implementación está llegando al árbol de trabajo, pero todavía no está conectada/cableada |

> Última revisión: 2026-10-26 (Batch 26 — FASE 6.3 corte 2: personalidad Big5/goals/secrets/relations, resumen LLM `npc.memory_summary` + `SummarySource`, config YAML/REST/NPC factory, spawn hereda `llm`, guard anti-fuga y redacción de secretos; Batch 25 — FASE 6.3 corte 1: comando `ask` + memoria NPC `npc_memories`; Batch 24 — FASE 6.5 DM narrator modo A; sincronía de docs raíz con Batches 14–26).
> Fase 6 sigue **en curso**: cut A + endpoints creator + questgen contextual +
> **narrador DM (FASE 6.5 modo A) completo** + **diálogo de NPC con LLM en runtime
> (FASE 6.3 cortes 1+2: comando `ask`, memoria `npc_memories` con resumen LLM y
> personalidad estructurada) cableado**; pendiente el **modo B del DM**
> (`DMDecision` + ambush/environmental) y los mecanismos B/C del diálogo
> (auto-apertura en `talk`, intercepción de texto libre).
> FASE 5.1 (Dungeon Builder) es **hecho parcial**: API REST `/api/dungeons` +
> panel cliente (tecla `D`), sin spawn de instancias en juego. QA B14–B24 en
> [`qa/REGRESSION_REPORT.md`](qa/REGRESSION_REPORT.md) (**§B24 — FASE 6.5 —
> APROBADO**; el Batch 25 — FASE 6.3 corte 1 — y el Batch 26 — corte 2, con
> observaciones menores ya cerradas — fueron verificados por QA sin sección
> propia en el reporte) y análisis de balance B17
> en [`qa/BALANCE_ANALYSIS.md`](qa/BALANCE_ANALYSIS.md). **P0 de carga cerrado —
> verificado por QA en B21** (§B19.3 / §B19.7: fix en Batch 20; §B21: 500
> concurrentes sin wedge, 0 `msgErrors`); el leak de `writeLoop` que abrió §B21.5
> se corrigió en Batch 22 y la **race de presencia** (§B22.4) se cerró en
> Batch 23 (race de presencia cerrada y verificada por QA en §B23). Contrato HTTP/WS en [`API.md`](API.md).

---

## Documentos de raíz

| Documento | Contenido | Estado |
|-----------|-----------|--------|
| [`../README.md`](../README.md) | Presentación del proyecto, arranque rápido y capturas | hecho |
| [`../PROJECT.md`](../PROJECT.md) | Visión global: stack, estructura, comandos (incl. build por paquete), **estado de fases** (5.1/5.3/6/7.1/7.2), pendientes y **P0 de carga (cerrado, verificado en B21)** | hecho |
| [`../FEATURES.md`](../FEATURES.md) | Catálogo de sistemas, entidades y APIs (atributos derivados 2.3, progresión 2.4, UX 3.x, Dungeon Builder 5.1, `offer`/contextual quests, FASE 6.5 DM narrator modo A, FASE 6.3 cortes 1+2 — comando `ask` + memoria NPC + personalidad/resumen LLM, estado de Fase 6, batches 14–26) | hecho |
| [`../ARCHITECTURE.md`](../ARCHITECTURE.md) | Arquitectura del motor: capas, entidades (incl. `Dungeon`/`NPCTemplate`), repos/tablas nuevos, progresión, concurrencia (**nota del P0 de deadlock, cerrado en B21**; **lock de presencia por usuario, cerrado en B23**), estado LLM | hecho |
| [`../MISSING_MVP_ITEMS.md`](../MISSING_MVP_ITEMS.md) | Lista de huecos MVP pendientes | diseño (backlog) |
| [`../AGENTS.md`](../AGENTS.md), [`../CLAUDE.md`](../CLAUDE.md) | Instrucciones para agentes de código y convenciones de documentación | hecho |

---

## `docs/` — guías del fork

| Documento | Contenido | Estado |
|-----------|-----------|--------|
| [`API.md`](API.md) | API REST y WebSocket: convenciones, auth/sesiones, personajes, social, `/api/dungeons`, creator, ops/admin, audit y protocolo WS (`/ws`) | hecho |
| [`architecture.md`](architecture.md) | Arquitectura **del fork** (`mi-mud`): componentes, mapa de módulos `pkg/*`, flujos de datos, deltas vs upstream, despliegue | hecho |
| [`fork-setup.md`](fork-setup.md) | Arranque en local y con Docker: prerrequisitos, `.env`, auth local, Ollama, tests, troubleshooting | hecho |
| [`structured-messages.md`](structured-messages.md) | Protocolo WebSocket (mensajes nuevos con `omitempty`, `hello` de capacidades) | hecho |
| [`content-health.md`](content-health.md) | `tales -check`: reglas de salud del contenido y API `GET /api/health` | hecho |
| [`device-login.md`](device-login.md) | Login por dispositivo para clientes instalables (`/api/device/start|poll`) | hecho |
| [`ssh-access.md`](ssh-access.md) | SSH como segundo transporte (clásico y Door), claves y política de device/guest | hecho |
| [`DOOR-ON-MUD.md`](DOOR-ON-MUD.md) | Diseño de correr un segundo mundo sobre el motor compartido | diseño |
| [`COMBAT-BALANCE.md`](COMBAT-BALANCE.md) | Tablas de balance por hueco de nivel (trash/elite/boss) y objetivos de win-rate | hecho |
| [`combat-battle-stage.md`](combat-battle-stage.md) | BattleStage: diseño y cortes C0–C7 (algunos polish fuera del slice) | mixto |

---

## `docs/design/` — diseño y planes

Índice propio del folder: [`design/README.md`](design/README.md) (estado: **hecho**, es un índice).

| Documento | Contenido | Estado |
|-----------|-----------|--------|
| [`design/GAME_DESIGN.md`](design/GAME_DESIGN.md) | Design doc + backlog MVP (epics, principios, specs de datos) | mixto |
| [`design/SCRIPTING.md`](design/SCRIPTING.md) | Sistema de scripts Lua: arquitectura, API `tales.*`, eventos, sandbox | hecho |
| [`design/CHARACTER_TEMPLATES.md`](design/CHARACTER_TEMPLATES.md) | Plantillas de personaje: estructura, razas/clases, atributos, API REST | hecho |
| [`design/DIALOG_SYSTEM.md`](design/DIALOG_SYSTEM.md) | Árboles de diálogo, estado de conversación, variables, comandos `talk` | hecho |
| [`design/WORLD_MAP_IMPLEMENTATION.md`](design/WORLD_MAP_IMPLEMENTATION.md) | Mapa mundial interactivo: endpoint, layout, componentes | hecho |
| [`design/QUEST_AUTHORING.md`](design/QUEST_AUTHORING.md) | Guía de autoría de quests (objetivos, rewards, validación) | hecho |
| [`design/PHASE2_4_PROGRESSION.md`](design/PHASE2_4_PROGRESSION.md) | Diseño de progresión: curva XP, talents cada 5 niveles, prestige. **El código ya lo implementa** (ver FEATURES.md); la cabecera del doc sigue diciendo "Diseño" | diseño (doc) / hecho (código) |
| [`design/PHASE4_SCHEMA.md`](design/PHASE4_SCHEMA.md) | Esquema propuesto para fases sociales (patrón `(id, data)` + migraciones) | **diseño → hecho**: tablas `clans*`/`dm_*`, repos `clans_sqlite`/`dm_sqlite`, `pkg/service/clans.go|dms.go`, rutas REST y `ClanPanel.svelte` existen; **queda corto**: sin comando MUD `clan` y `tell` sigue en memoria |
| [`design/PHASE5_DUNGEONS.md`](design/PHASE5_DUNGEONS.md) | Diseño y contenido del pack `import/adventure/` (4 zonas, raid, loot) | hecho (implementado y validado) |
| [`design/PHASE6_LLM.md`](design/PHASE6_LLM.md) | **Fase 6 — integración LLM (§0–6.7)**: `llmprompt`, `llmgen`, NPC LLM, quest gen, DM AI, guardrails, admin tools | **mixto** (el doc sigue marcado como diseño, con notas "Implementado"): existen `llmopts`, `llmprompt` (+6 prompts), `llmgen` (pipeline + tests + questgen contextual), la tabla `npc_memories` y su repo, `handler/llm.go` (generate, quests generate/contextual, room description, prompts), el **narrador DM completo en modo A** (Batch 24; nota "Implementado" en §6.5) y el **diálogo de NPC en runtime (FASE 6.3 cortes 1+2: comando `ask`, memoria `npc_memories` con resumen LLM y personalidad estructurada Big5/goals/secrets/relations)** (Batches 25–26; nota "Implementado (Batches 25–26, cortes 1+2)" en §6.3). **Pendiente:** modo B del DM (`DMDecision` + ambush/environmental) y mecanismos B/C del diálogo. Verificado 2026-10-26 |
| [`design/PRD_COMBAT_SYSTEM.md`](design/PRD_COMBAT_SYSTEM.md) | PRD del sistema de combate | diseño (draft) |
| [`design/PRD_ITEMIZATION.md`](design/PRD_ITEMIZATION.md) | PRD de itemización y loot | diseño (draft) |
| [`design/PRD_NPC_SYSTEM.md`](design/PRD_NPC_SYSTEM.md) | PRD del sistema de NPCs | hecho (implemented) |
| [`design/PRD_WORLD_IMPORT.md`](design/PRD_WORLD_IMPORT.md) | PRD de reset/importación de mundo | diseño |
| [`design/WorldBuilderAgentPRD.md`](design/WorldBuilderAgentPRD.md) | PRD de agentes para construcción de mundo | diseño |

---

## `docs/user/` — guías de jugador y de admin

| Documento | Contenido | Estado |
|-----------|-----------|--------|
| [`user/player-guide.md`](user/player-guide.md) | Guía del jugador en una página: creación (6 clases, escuelas, razas), atajos de teclado (`1`–`9`, Tab, C, M, K, L, I, N, D, A, Escape, `?`), progresión, inventario (peso = aviso visual) y combate; enlaza el resto de `user/` y `player-guide/` | hecho |
| [`user/admin-guide.md`](user/admin-guide.md) | Guía del admin: roles, panel Creator (`/creator`), herramientas del cliente (editor de clases `L` con CRUD REST, NPC factory `N`, audit `A` con filtros client-side), ops en vivo, `tales -check`, env vars LLM/de rendimiento, seguridad y Docker | hecho |
| [`user/commands.md`](user/commands.md) | Referencia breve de comandos realmente implementados (`talents`, `prestige`, `party loot`, `party readycheck/ready/notready`, `chat` + atajos, `offer` de quests contextuales, `ask` de diálogo libre con NPCs LLM) y de los que **no** existen (`clan`, `tell` persistente) | hecho |
| [`user/clans.md`](user/clans.md) | Clanes: roles, invitaciones, banco compartido, panel (sin comando `clan`) | hecho |
| [`user/dms.md`](user/dms.md) | Mensajería privada persistente (`DMChannel`/`DMMessage`, panel) | hecho |
| [`user/channels.md`](user/channels.md) | Canales globales (`chat`, shortcuts, tabs del cliente) | hecho |
| [`user/party.md`](user/party.md) | Parties: crear/invitar/roster, loot rule, ready check, follow | hecho |
| [`user/combat.md`](user/combat.md) | Skills de clase y curación (`mend` / `renew`), equipar y usar | hecho |

---

## `docs/qa/` — verificación

| Documento | Contenido | Estado |
|-----------|-----------|--------|
| [`qa/REGRESSION_REPORT.md`](qa/REGRESSION_REPORT.md) | Reportes de regresión y auditorías: B14 (load testing foundations), B15 (readiness de carga FASE 7.1), B16 (seguridad FASE 7.2, sin hallazgos altos), B17 (balance FASE 7.3, ver también `BALANCE_ANALYSIS.md`), B18 (harness de carga + combat sims), **B19** (load test 100–500: **P0 deadlock ≥200** — registro histórico, corregido en B20), **B21** (**re-verificación: P0 cerrado** + leak de `writeLoop` P2, corregido en B22; veredicto PASS con condiciones), **B22** (fix de build/`.gitignore` y leak de `writeLoop`), **B23** (**race de presencia §B22.4 cerrada y verificada**: lock por usuario, `-race` completo con 0 `DATA RACE`), **B24** (**FASE 6.5 DM narrator modo A verificada: APROBADO** — contrato vs diseño §6.5, invariante "nunca muta estado", guardrails, fallback silencio) | hecho |
| [`qa/BALANCE_ANALYSIS.md`](qa/BALANCE_ANALYSIS.md) | B17: análisis cuantitativo de balance (curva XP, atributos, combate, loot, win-rate objetivo); solo lectura | hecho |

---

## `docs/development/`, `docs/player-guide/`, `docs/superpowers/`

| Documento | Contenido | Estado |
|-----------|-----------|--------|
| [`development/AGENTS.md`](development/AGENTS.md) | Convenciones de agentes para desarrollo | hecho |
| [`development/COMBAT_BALANCE.md`](development/COMBAT_BALANCE.md) | Notas de balance de combate | hecho |
| [`development/LOGGING.md`](development/LOGGING.md) | Política de logging | hecho |
| [`player-guide/README.md`](player-guide/README.md) + `01`–`13` | Guía del jugador en 13 capítulos (inicio, UI, comandos, personaje, inventario, combate, quests, exploración, NPCs, ajustes, móvil, referencia, glosario) | hecho |
| [`superpowers/specs/`](superpowers/specs/) | Especificaciones UX (creator quality, quest delivery, combat reliability) | diseño |
| [`superpowers/plans/`](superpowers/plans/) | Planes de trabajo (roadmap de seis sistemas, cortes) | diseño |

---

## Carpetas sin documentos de texto

| Carpeta | Contenido |
|---------|-----------|
| [`combat-mocks/`](combat-mocks/) | PNG del BattleStage (desktop / mobile) |
| [`map-mocks/`](map-mocks/) | PNG de polish de combate y mapa |

---

## Relación con las fases

| Fase | Documento de diseño | Estado del código |
|------|--------------------|-------------------|
| 2.3 Atributos derivados | — (ver `FEATURES.md`) | hecho |
| 2.4 Progresión (XP, talents, prestige) | [`design/PHASE2_4_PROGRESSION.md`](design/PHASE2_4_PROGRESSION.md) | hecho |
| 3.2 / 3.4 / 3.7 UX de cliente | — (ver `FEATURES.md`) | hecho |
| 4 Social (clans, DMs, canales) | [`design/PHASE4_SCHEMA.md`](design/PHASE4_SCHEMA.md) | hecho (clanes/DM/canales); el doc de esquema sigue marcado como diseño |
| 5 Mazmorras | [`design/PHASE5_DUNGEONS.md`](design/PHASE5_DUNGEONS.md) | hecho |
| 5.1 Dungeon Builder | — | **hecho parcial**: API REST `/api/dungeons` + panel cliente (tecla `D`, localStorage + sync creator); sin spawn/instancias en juego (ver [`API.md`](API.md) §5) |
| 6 LLM | [`design/PHASE6_LLM.md`](design/PHASE6_LLM.md) | **en curso**: cut A + endpoints creator + questgen contextual + **narrador DM FASE 6.5 modo A (Batch 24)** + **diálogo de NPC FASE 6.3 cortes 1+2 — comando `ask` + memoria `npc_memories` + personalidad/resumen LLM (Batches 25–26)**; pendiente modo B del DM y mecanismos B/C del diálogo |
| 7.1 Carga | — (ver [`qa/REGRESSION_REPORT.md`](qa/REGRESSION_REPORT.md) §B15, §B18, §B19, §B21) | **cerrado — verificado por QA en B21**: pool SQLite 4 + índices (Batch 16), harness `tools/loadtest` con modo `-mint` (Batch 18) y corrida real 100–500 (B19): 100 conexiones sanas, **≥200 → deadlock P0** (§B19.3/§B19.7) **corregido en Batch 20 (Backend)**; re-verificación **§B21**: 200/300/500 × 2 corridas × 80 s, `recv == sent`, **0 `msgErrors`**, sin wedge; leak `writeLoop` (§B21.5) corregido en Batch 22 |
| 7.2 Seguridad | — (ver [`qa/REGRESSION_REPORT.md`](qa/REGRESSION_REPORT.md) §B16, §B19.4) | auditoría estática PASS; cabeceras globales y timeouts HTTP en el árbol; hallazgo medio: bypass `X-Forwarded-For` de loopback (§B19.4) |
| 7.3 Balance | [`design/PRD_COMBAT_SYSTEM.md`](design/PRD_COMBAT_SYSTEM.md) (draft) y [`COMBAT-BALANCE.md`](COMBAT-BALANCE.md) | auditoría B17 en [`qa/BALANCE_ANALYSIS.md`](qa/BALANCE_ANALYSIS.md) + combat sims B18; solo lectura, sin cambios de balance aplicados |
