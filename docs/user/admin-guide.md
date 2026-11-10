# Guía del admin

Operación del fork `mi-mud`: roles, panel Creator, herramientas del cliente de
juego, acciones en vivo, salud del contenido, LLM y despliegue.

> Verificado contra el código el 2026-10-17. Comandos de jugador en
> [`commands.md`](commands.md); arranque y Docker en [`../fork-setup.md`](../fork-setup.md).

---

## 1. Roles y acceso

| Rol | Cómo se obtiene | Acceso |
|---|---|---|
| `player` | registro normal | juego y API de jugador |
| `creator` | `PUT /api/admin/users/:id/role` (solo admin) | + rutas `/api/creator/*`, `/api/rooms|items|npcs|quests|skills…`, `GET /api/audit`, SPA `/creator` |
| `admin` | `MUD_ADMIN_OAUTHID` (solo `auth0`; la API no permite asignarlo) | + `POST /api/ops/:action`, `GET /api/live/characters`, `POST /api/audit/:id/undo`, `/api/admin/*`, canal `admin` |

* El middleware es `CreatorMiddleware` (creator **o** admin) y `AdminMiddleware`
  (`pkg/server/auth.go`); el cliente solo muestra el acceso a `/creator` a esas
  dos cuentas (`UserMenu.svelte`, `CharacterSwitcher.svelte`).
* `MUD_ADMIN_OAUTHID` solo se aplica con `AUTH_MODE=auth0`: el middleware de
  sesiones locales resuelve el usuario sin pasar por ese sync
  (`pkg/server/auth.go`). Con auth local, promover a admin exige editar la fila
  `users.data` (JSON, campo `role`) en SQLite; la API solo admite `player` y
  `creator`.
* Un rol distinto de `player`/`creator` no se puede fijar por API; banear,
  desbanear y borrar usuarios va por `/api/admin/users`.
* Las sesiones de invitado (guest) no abren rutas creator/admin.

---

## 2. Panel Creator (`/creator/*`)

SPA del repo (`public/app`), enlace **World Builder** desde el menú de cuenta.

| Grupo | Pestañas |
|---|---|
| World | Rooms, World/Zones, Spawners, Health |
| Actors | NPCs, Loot tables, Character Templates, Items (plantillas) |
| Narrative | Dialogs, Dialog Graph, Quests (+ depurador `/creator/quests/debug`) |
| Systems | Skills, Scripts, Settings |
| Operate | Players (**solo admin**), Audit log, Drift |

* Búsqueda global con **Ctrl+K**; los selectores de entidad usan
  `EntitySelectButton` + modal filtrable (nunca `<select>` para IDs).
* Validación en línea por entidad, **World Health** (referencias rotas,
  alcanzabilidad, contenido sospechoso) y **Drift** (antes/después por campo,
  export YAML por entidad o completo).
* Todo lo escrito pasa por `handler.AuditWrites`: cada escritura deja una fila
  en el audit log con instantáneas antes/después (deshacer con `UndoCRUD`).
* Catálogo de capacidades: [`../../FEATURES.md`](../../FEATURES.md) § *Creator UI Capabilities*.

---

## 3. Herramientas dentro del cliente de juego

| Tecla | Herramienta | Estado real |
|---|---|---|
| `L` | Editor de clases y balance (`ClassesEditor.svelte`) | **CRUD REST**: con token creator/admin carga y guarda en `/api/classes` (`GET/POST/PUT/DELETE`, `POST /api/classes/:id/duplicate`). Cada guardado valida y escribe el YAML de `data/classes/`, luego recarga el catálogo; solo cae a `localStorage` si la red no está disponible |
| `N` | Fábrica de NPCs / mobs (`NpcFactory.svelte`) | plantillas en `localStorage` (`mud-client.npc-factory.v1`, módulo puro `npcFactory.js` con validación de rangos); sincronización REST **best-effort** con token de sesión creador: pull con `GET /api/npcs/templates` y push con `POST /api/npcs` / `PUT /api/npcs/:id`; sin token, solo local |
| `D` | Dungeon Builder (`DungeonBuilder.svelte`) | **corte inicial FASE 5.1 con persistencia (Batch 19)**: grid clickable, salas, corredores, encuentros y loot tables, validación (entrada→salida por BFS, solapes) y modo de prueba; biblioteca con `localStorage` como fuente de verdad + sincronización **best-effort** a `GET/POST/PUT/DELETE /api/dungeons` con sesión creator (degrada a local con aviso si no hay rol/red) |
| `A` | Audit log (`AuditPanel.svelte`) | `GET /api/audit` (creator-gated, `limit` ≤ 500); **filtros, búsqueda y CSV se calculan en el navegador** (no hay filtros server-side: el endpoint devuelve las últimas escrituras y ya) |
| `C` / `M` | clan y DMs | paneles de jugador, ver [`clans.md`](clans.md), [`dms.md`](dms.md) |

* **Dungeon Builder (`D`)**: corte inicial de FASE 5.1; con token de creador
  sincroniza la biblioteca contra `/api/dungeons` (mejor esfuerzo) y con
  `localStorage` como respaldo; sin token o con error de red solo local (exporta
  JSON además). El editor de clases (`L`) sí persiste en el servidor cuando hay
  token creator/admin: valida y escribe los YAML de `data/classes/` vía
  `/api/classes` y recarga el catálogo en caliente. Contrato REST completo en
  [`../API.md`](../API.md).
* El atajo `A` abre el panel en cualquier cuenta, pero sin rol `creator` el
  servidor responde 403 y el panel muestra el error.
* Canal global `admin <msg>` (`chat admin …`): solo cuentas con rol admin
  (`pkg/mudserver/game/commands/channels.go`).
* **No comandos MUD de admin**: el registro de comandos no tiene `goto`,
  `spawn`, etc.; las acciones de operador son REST (§4). Detalle de comandos en
  [`commands.md`](commands.md).

---

## 4. Acciones en vivo y endpoints de administración

| Endpoint | Rol | Qué hace |
|---|---|---|
| `POST /api/ops/:action` + `{"confirm":true}` | admin | ejecuta en el game loop: `teleport`, `give-item`, `take-item`, `npc-heal`, `npc-respawn`, `npc-despawn`, `end-combat`, `instance-cleanup`, `quest-step`, `regrant-starter-kit` (`pkg/mudserver/game/ops.go`) |
| `GET /api/live/characters[/:id]` | admin | personajes e inventarios en vivo |
| `POST /api/audit/:id/undo` | admin | deshace una fila auditable |
| `GET /api/audit` | creator | últimas escrituras (`limit` ≤ 500); **sin filtros server-side** (el panel filtra en el navegador) |
| `GET/PUT /api/admin/users…` | admin | listar, cambiar rol (`player`/`creator`), ban/unban, borrar |
| `GET /api/admin/stats/guests[ /daily]` | admin | analítica de invitados |
| `GET /api/admin/llm/status`, `/api/admin/llm/stats` | admin | proveedor, modelo y contadores |
| `GET /admin/export`, `POST /admin/import` | basic auth | export/import legacy con `ADMIN_USER`/`ADMIN_PASSWORD` (mín. 12 caracteres) |

Todas las respuestas de ops devuelven `auditId` y `undoable`; el deshacer vive
en la pestaña **Audit log** de `/creator`.

---

## 5. Salud del contenido

```bash
go run ./cmd/tales -check game               # informe de texto
go run ./cmd/tales -check game -json         # informe completo
go run ./cmd/tales -check game -fail-on=warning
```

Reglas y endpoints (`GET /api/health`, mute, drift, alcance de quests) en
[`../content-health.md`](../content-health.md). En el panel: pestaña **Health**
y `/creator/drift`. `tales -check` no abre la base de datos y no corre la regla
`deploy-tree-dirty`.

---

## 6. Configuración LLM

Cliente único `pkg/service/llm` con proveedor elegido por entorno:

| Variable | Por defecto | Valores |
|---|---|---|
| `LLM_PROVIDER` | `ollama` en `.env.example` | `ollama`, `groq`, `none`/`off` (o vacío = off); sin definir → Groq si hay `GROQ_API_KEY`, si no Ollama |
| `OLLAMA_BASE_URL` | `http://localhost:11434` (`ollama` dentro de compose) | URL del daemon |
| `OLLAMA_MODEL` | `llama3.1` | modelo local descargado |
| `LLM_TIMEOUT_SECONDS` | `60` | timeout del cliente Ollama |
| `GROQ_API_KEY` | vacío | clave de Groq (modelo fijo `llama-3.3-70b-versatile`) |

* Sin proveedor, `POST /api/generate/character` responde **503** *AI generation is not configured*.
* Autoría asistida (creator): `POST /api/creator/llm/quests/generate` (dry run
  salvo `commit:true`), `POST /api/creator/llm/quests/contextual` (nueva),
  `POST /api/creator/llm/rooms/:id/description`,
  `GET|POST /api/creator/llm/prompts`, `POST /api/creator/llm/prompts/:id/test`.
* `POST /api/creator/llm/quests/contextual` genera **una** quest ajustada al
  contexto: body `{"playerLevel":N,"zoneId":"…","theme":"…"}`
  (`handler.GenerateContextualQuest` → `llmgen.GenerateContextualQuest`).
  Respuesta: `quest`, `issues`, `source` (`llm`/`fallback`), `attempts`,
  `cached`. Sin proveedor responde **503**; cae a un fallback determinista
  validado contra el catálogo del mundo.
* Narrador DM en combate: cableado en `pkg/server/llm.go` sobre
  `game.SetDMNarrator`; requiere proveedor y respeta el kill switch
  `LLM_DM_ENABLED` (por defecto activo). El modo B (`DMDecision`) está
  implementado pero **apagado por defecto** (`LLM_DM_DECISION_ENABLED=0`).
  El diálogo de NPC con LLM en tiempo de juego (`ask`, `talk` con prioridad
  autoral/quest) está implementado; ver `docs/user/commands.md`.
* Tablas y checklist: [`../fork-setup.md`](../fork-setup.md) §6 (LLM) y §7.

---

## 7. Docker y despliegue

```bash
cp .env.example .env
docker compose up -d --build     # sin variables obligatorias (AUTH_MODE=local)
docker compose ps                # healthcheck: GET /health
docker compose logs -f talesmud
```

Detalles (volúmenes, perfil `ollama`, puertos, troubleshooting) en
[`../fork-setup.md`](../fork-setup.md) §5. Desarrollo local: `make dev`.

Variables de rendimiento y seguridad (opcionales; las de timeout ya están en
`.env.example`):

| Variable | Por defecto | Qué controla |
|---|---|---|
| `DB_MAX_OPEN_CONNS` | `4` | tamaño del pool de SQLite (WAL y `busy_timeout` van en el DSN para todo el pool) — `pkg/db/sqlite/sqlite.go` |
| `HTTP_READ_HEADER_TIMEOUT` | `5s` | timeouts del `http.Server` (defensa slowloris) — `pkg/server/server.go` |
| `HTTP_READ_TIMEOUT` | `15s` | ídem |
| `HTTP_WRITE_TIMEOUT` | `30s` | ídem; las sesiones WebSocket se salen del servidor y quedan sin este límite |
| `HTTP_IDLE_TIMEOUT` | `60s` | ídem |
| `JWKS_TIMEOUT_SECONDS` | `10` | timeout del fetch JWKS de Auth0 (cuerpo acotado a 1 MiB) — `pkg/server/auth.go` |
| `LLM_DM_ENABLED` | activo | kill switch del narrador DM en prosa (modo A) — `pkg/server/llm.go` |
| `LLM_DM_DECISION_ENABLED` | `0` | kill switch del modo B (`DMDecision`: ambush/environmental/narrative); opt-in — `pkg/server/llm.go` |
| `LLM_DM_DECISION_COOLDOWN_SECONDS` | `60` | cooldown por sala entre decisiones DM |
| `LLM_DM_DECISION_TIMEOUT_SECONDS` | `8` | timeout de una decisión DM |
| `LLM_DM_DECISION_MAX_PER_MIN` | `10` | presupuesto global de decisiones DM por minuto |

Seguridad global: `SecurityHeadersMiddleware` añade `X-Content-Type-Options:
nosniff`, `X-Frame-Options: DENY` y `Referrer-Policy: no-referrer` a todas las
respuestas; la CSP restrictiva se aplica solo a `/activate`. La auditoría
estática completa (auth, SQLi, XSS, CORS/CSRF, WS) está en
[`../qa/REGRESSION_REPORT.md`](../qa/REGRESSION_REPORT.md) § B16 — sin
hallazgos de severidad alta; quedan condiciones abiertas (B16-F1…F8).

## 8. Documentación relacionada

- Índice general: [`../index.md`](../index.md)
- Arquitectura del fork: [`../architecture.md`](../architecture.md)
- Protocolo del cliente: [`../structured-messages.md`](../structured-messages.md)
- Acceso SSH / dispositivo: [`../ssh-access.md`](../ssh-access.md), [`../device-login.md`](../device-login.md)
- Reporte de regresión: [`../qa/REGRESSION_REPORT.md`](../qa/REGRESSION_REPORT.md)
