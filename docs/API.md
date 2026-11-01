# API REST y WebSocket — `mi-mud`

Referencia del contrato HTTP/WS servido por `pkg/server` (Gin) y `pkg/mudserver`.
Generada leyendo el código real (rutas registradas en `pkg/server/server.go:274-788`);
los shapes de respuesta provienen de los handlers citados. Los clientes deben
ignorar campos y tipos de mensaje desconocidos.

**Leyenda de auth**

| Nivel | Significado |
|-------|-------------|
| pública | sin token |
| sesión | `Authorization: Bearer <token>` (JWT local/Auth0, token de sesión de dispositivo o token de guest) |
| **creator** | sesión con rol `creator` o `admin` (`CreatorMiddleware`, `pkg/server/auth.go:485`) |
| **admin** | sesión con rol `admin` (`AdminMiddleware`, `pkg/server/auth.go:500`) |
| básica (env) | HTTP Basic con `ADMIN_USER`/`ADMIN_PASSWORD` (solo `/admin/*`) |

---

## 1. Convenciones

- **Prefijo**: todo lo de juego cuelga de `/api/`. Errores: JSON `{"error": "..."}` salvo donde se indique.
- **Auth REST**: solo por cabecera `Authorization: Bearer …`. El parámetro de query
  `access_token` se rechaza con 401 (`pkg/server/auth.go:420-434`).
- **Tokens aceptados** (`authenticateToken`, `pkg/server/auth.go:335-416`): token de
  guest (validado por `GuestService`, con expiración), token de sesión de dispositivo
  (`authsession`), token local (`authlocal`) o JWT Auth0 (RS256 + JWKS + audience/issuer).
  Usuario baneado → 403 `{"error":"Your account has been banned"}`.
- **Roles** (`pkg/entities/user.go:5-9`): `player` (por defecto), `creator`, `admin`.
- **CORS**: allowlist por defecto (talesmud.io + dev servers) ampliable con
  `CORS_ALLOWED_ORIGINS` (`server.go:96-118`). Cabeceras de seguridad globales
  (`X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`) vía
  `SecurityHeadersMiddleware`.
- **Auditoría**: las escrituras creator de CRUD (rooms, items, npcs, loottables,
  spawners, dialogs, quests, scripts, skills, character-templates, settings) dejan
  fila de audit con snapshots antes/después (`handler.AuditWrites`,
  `pkg/server/handler/audit.go:25-57`) y invalidan el índice de búsqueda.
- **Rate limits**: auth local 12/min por IP (`handler/localauth.go:38`);
  `generate/character` 10/hora por usuario (`handler/generate.go:21-26`); creación
  de guest y device-flow con límites propios (429).

---

## 2. Autenticación y sesión

### Públicos (sin token)

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/health` | `200` texto plano "API is up and running" (`server.go:421`) |
| POST | `/api/auth/register` | Registro local: `{username, email, password}` → `201 {token, user}` |
| POST | `/api/auth/login` | Login local: `{username, password}` → `200 {token, user}` |
| POST | `/api/auth/forgot` | `{email}` → `200 {ok:true}` (solo si el outbox entrega correo) |
| POST | `/api/auth/reset` | `{token, password}` → `200 {ok:true}` |
| POST | `/api/guest` | Crea sesión de guest: body opcional `{templateId, race}` → `200 {token, expiresIn:1800}` |
| POST | `/api/device/start` | Inicio de login por dispositivo (proto 1): `{client, client_version, proto:1}` → `200 {user_code, device_code, verification_uri, verification_uri_complete, expires_in, interval}` |
| POST | `/api/device/poll` | `{device_code}` → `200 {token, expires_in}` o 400 con `error` (`pending`/`slow_down`/`denied`/`expired`) |
| GET | `/api/ssh/info` | Info pública del gate SSH: `{"enabled": false}` o detalle |
| GET | `/api/server-info` | `200 {serverName, envLabel, host}` |
| GET | `/api/classes` | Catálogo público de razas/clases (sin números de balance) |
| GET | `/api/templates/characters` | Plantillas legacy de creación (hardcoded) |
| GET | `/api/item-slots` · `/api/item-qualities` · `/api/item-types` · `/api/item-subtypes` | Enums de items |
| GET | `/api/room-of-the-day` | Sala destacada del día |
| GET | `/api/backgrounds/:filename` · `/api/portraits/:filename` · `/api/item-art/:filename` · `/api/map-tiles/:filename` | Assets públicos (PNG/JPG/WebP; `HEAD` en item-art y map-tiles) |

Los endpoints de registro/login solo existen cuando el servidor corre con auth
local (`gamemode.LocalAuth()`). `forgot`/`reset` solo si el outbox envía correo
externo (`app.resetByMail`). `device/start`/`device/poll` solo si el servicio de
dispositivo está construido (`app.deviceAPI != nil`, `server.go:716`). En modo
ANSI/Door existe además `GET /api/door/config`
(`{title, subtitle, tokenKey, forgotEnabled}`, `server.go:761-769`).

Errores auth local (`handler/localauth.go:130-143`): 400 validación, 404/401
credenciales (`ErrCredentials` → 401), 409 cuenta existente, 403 baneado,
429 demasiados intentos.

### Con sesión

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/auth/me` | Usuario actual sin secretos: `{user}` (solo auth local) |
| GET | `/api/user` | `User.RedactedCopy()` |
| PUT | `/api/user` | Actualiza `name`, `email`, `nickname`, `picture`, `isNewUser`; guests rechazados (403) |
| POST | `/api/ws-ticket` | Emite ticket WS de un solo uso: `200 {ticket, expiresIn}` (TTL 60 s, `pkg/server/wsticket.go:14,82-102`) |

### SSH (con sesión)

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/ssh/keys` | Lista claves vinculadas: `[{id, fingerprint, key_type, label, created_at, created_via, last_used_at}]` |
| POST | `/api/ssh/keys` | **No implementado aún como upload**: siempre responde 403 "link the key from an SSH sign-in" (el body no se almacena ni se ecoa) |
| DELETE | `/api/ssh/keys/:id` | Revoca una clave propia |
| POST | `/api/ssh/device/lookup` | `{user_code, csrf}` → estado del código device |
| POST | `/api/ssh/device/confirm` | Confirma el login device pendiente |
| POST | `/api/ssh/device/deny` | Deniega el login device pendiente |

---

## 3. Personajes, plantillas y quests (jugador)

| Método | Ruta | Auth | Descripción |
|--------|------|------|-------------|
| GET | `/api/characters` | sesión | Personajes del usuario (admin: todos) |
| GET | `/api/my-characters` | sesión | Solo los del usuario autenticado |
| GET | `/api/characters/:id` | sesión (dueño o admin) | Ficha; otros → 403 `character access denied` |
| PUT | `/api/characters/:id` | sesión (dueño o admin) | Solo campos allowlist `{name?, description?}` → `{"status":"updated character"}` |
| DELETE | `/api/characters/:id` | sesión (dueño o admin) | → `"deleted"` |
| GET | `/api/characters/:id/map` | sesión (dueño o admin) | Atlas fog-of-war del personaje |
| POST | `/api/newcharacter` | sesión | Crea desde plantilla: `{templateId, name, description, race}` (el `userId` se toma del token); 400 si la raza no está permitida |
| POST | `/api/characters` | **creator** | Crea un personaje crudo (JSON `characters.Character`); `belongsUserID` forzado al usuario salvo admin |
| GET | `/api/character-templates` | sesión | Lista plantillas |
| GET | `/api/character-templates/:id` | sesión | Detalle |
| GET | `/api/character-templates/presets` | sesión | Presets semilla |
| POST | `/api/character-templates` | **creator** | Crea plantilla |
| PUT | `/api/character-templates/:id` | **creator** | Actualiza plantilla |
| DELETE | `/api/character-templates/:id` | **creator** | Borra plantilla |
| POST | `/api/character-templates/seed` | **creator** | Siembra presets; `{"status":"skipped",…}` si ya existen |
| GET | `/api/quest-progress/:characterId` | sesión (dueño o admin) | Quest log enriquecido: `[{questId, questName, status, description, objectives:[{objectiveId, description, current, required, completed}], readyToTurnIn, rewards, …}]` |
| POST | `/api/quest-progress/:characterId/accept/:questId` | sesión (dueño o admin) | Acepta → progreso |
| POST | `/api/quest-progress/:characterId/abandon/:questId` | sesión (dueño o admin) | Abandona → `{"status":"quest abandoned"}` |
| POST | `/api/quest-progress/:characterId/complete/:questId` | sesión (dueño o admin) | Turn-in: exige objetivos completos; `200 {progress, rewards:{xp, gold, items}}`, 400 si faltan objetivos |
| POST | `/api/generate/character` | sesión (no guests) | LLM: `{generate:"name"|"description"|"both", templateName?, archetype?, race?, class?, description?, backstory?, originArea?, currentName?}` → `{name?, description?}`; 429 rate limit, 503 sin pipeline |

---

## 4. Social — clanes y mensajes privados

Los endpoints de clanes y DMs reciben el personaje actuante en el body (o
`?characterId=` en GET) y verifican propiedad antes de tocar el servicio
(`handler/clans.go:12-14`, `handler/dms.go:12-14`).

### Clanes

| Método | Ruta | Body | Respuesta |
|--------|------|------|-----------|
| GET | `/api/clans/mine` | — (`?characterId=` o body) | `{clan, members, invites}` (clan `null` si no está en ninguno) |
| POST | `/api/clans` | `{characterId, name, tag, description}` | clan creado |
| POST | `/api/clans/:id/invite` | `{characterId, targetCharacterId}` | invite |
| POST | `/api/clans/invites/:inviteId/accept` | `{characterId}` | `{"status":"accepted"}` |
| POST | `/api/clans/invites/:inviteId/decline` | `{characterId}` | `{"status":"declined"}` |
| POST | `/api/clans/:id/kick` | `{characterId, targetCharacterId}` | `{"status":"kicked"}` |
| POST | `/api/clans/:id/rank` | `{characterId, targetCharacterId, rank}` | `{"status":"rank updated"}` |
| POST | `/api/clans/:id/deposit` | `{characterId, amount}` | `{"status":"deposited", balance}` |
| POST | `/api/clans/:id/withdraw` | `{characterId, amount}` | `{"status":"withdrawn", balance}` |
| POST | `/api/clans/:id/disband` | `{characterId}` | `{"status":"disbanded"}` |

Errores (`writeServiceError`, `handler/clans.go:104-126`): 404 no encontrado /
personaje inexistente, 409 ya existe o invite pendiente, 403 rango insuficiente,
solo líder, no miembro, no autorizado; 400 resto. `characterId` ausente → 400.

### Mensajes privados (DM)

| Método | Ruta | Body / query | Respuesta |
|--------|------|--------------|-----------|
| GET | `/api/dm/channels` | `?characterId=` | array de canales |
| POST | `/api/dm/channels` | `{characterId, targetCharacterId}` | canal (find-or-create) |
| GET | `/api/dm/channels/:channelId/messages` | `?characterId=&limit=50&since=<RFC3339>` | array de mensajes |
| POST | `/api/dm/channels/:channelId/messages` | `{characterId, body}` | mensaje guardado |
| POST | `/api/dm/channels/:channelId/read` | `{characterId}` | `{"status":"read"}` |

`limit` debe ser entero positivo; `since` RFC3339 (400 en caso contrario).
Solo participantes del canal (403 `not a participant`).

### Canales globales de chat

**No hay endpoints REST de chat.** `general`, `trade`, `lfg`, `help`, `roleplay`
y `admin` se envían por WebSocket como comandos de texto
(`chat <canal> <msg>` o atajos; ver `docs/user/channels.md` y
`pkg/mudserver/game/commands/channels.go`).

---

## 5. Mazmorras (Dungeon Builder) — `/api/dungeons`

Contrato nuevo (Batch 19 / FASE 5.1), handler `pkg/server/handler/dungeons.go`.
Lecturas para cualquier usuario autenticado; escrituras **creator**. El campo
`definition` es el JSON opaco exportado por el builder del cliente
(`exportDefinition`, `public/mud-client/src/game/dungeonLibrary.js`).

| Método | Ruta | Auth | Códigos |
|--------|------|------|---------|
| GET | `/api/dungeons?name=&author=&limit=` | sesión | 200 array de resumen (sin `definition`); 400 si `limit` no es positivo |
| GET | `/api/dungeons/:id` | sesión | 200 resumen + `definition`; 404 `dungeon not found` |
| POST | `/api/dungeons` | **creator** | 201 resumen creado; 400 validación; 403 sin rol |
| PUT | `/api/dungeons/:id` | **creator** | 200 resumen actualizado; 400; 404 id desconocido |
| DELETE | `/api/dungeons/:id` | **creator** | 204 sin contenido; 404 |

Resumen (`dungeonSummaryDTO`):

```json
{
  "id": "…", "name": "…", "description": "…",
  "gridWidth": 20, "gridHeight": 15,
  "author": "…", "tags": ["…"],
  "updatedAt": "2026-10-18T12:00:00Z"
}
```

Detalle: mismo objeto + `"definition": { … }` (blob opaco).

Body de create/update (`dungeonUpsertRequest`):

```json
{
  "name": "Mazmorra de prueba",
  "description": "…",
  "gridWidth": 20,
  "gridHeight": 15,
  "definition": { "version": 1, "grid": {"width": 20, "height": 15}, "cells": [], "rooms": [], "corridors": [], "entrances": [], "exits": [], "encounters": [], "lootTables": [], "meta": {} },
  "author": "…",
  "tags": ["…"]
}
```

Validación de servicio: `name` y rejilla requeridas (`must be…`/`required` →
400), `definition` debe ser JSON válido. En update se preserva `CreatedAt`.
El detalle del cliente (`public/mud-client/src/api/dungeons.js`) usa esta misma
ruta y degrada a `localStorage` ante 401/403/404 o fallo de red.

**Pendiente/no implementado aún**: no existe endpoint de "compile/run" de una
mazmorra ni spawn de instancias desde estas definiciones; la persistencia es
solo la biblioteca de diseños.

---

## 6. Creator — definiciones de mundo

Grupo `creator` (`server.go:507-640`): requiere rol creator/admin; las
escrituras CRUD listadas en §1 dejan audit. Formato genérico: GET lista →
array; GET `:id` → entidad (404 si falta); POST → entidad creada; PUT `:id` →
`{"status":"updated <entidad>"}`; DELETE `:id` → `{"status":"deleted…"}` o
`"deleted"`. Errores habituales: 400 bind/validación, 404 no encontrado,
500 servicio.

| Método | Ruta | Notas |
|--------|------|-------|
| GET/POST/PUT/DELETE | `/api/rooms` · `/api/rooms/:id` | `GET` acepta query de `RoomsQuery`; `PUT /api/world/rooms-coords` batch `{id,x,y,z}[]` → `{updated}` |
| GET | `/api/rooms/:id/inspect` | Inspector estático (resuelve copia de instancia a su template) |
| GET | `/api/rooms-vh` | Value-help de campos de sala |
| GET/POST/PUT/DELETE | `/api/items` · `/api/items/:id` | `GET` filtra por `isTemplate`; `POST /api/items/from-template/:templateId` instancia |
| GET/POST/PUT/DELETE | `/api/scripts` · `/api/scripts/:id` | Solo Lua (400 en otro lenguaje) |
| GET | `/api/script-types` | Tipos soportados |
| POST | `/api/run-script/:id` | Ejecuta con ctx JSON opcional → `{success, result, error, duration, durationMs}` |
| GET/POST/PUT/DELETE | `/api/npcs` · `/api/npcs/:id` | `GET` filtra `roomID`, `isTemplate`; `GET /api/npcs/templates`; `GET /api/npcs/:id/inspect`; `POST /api/npcs/:id/spawn` → instancia viva |
| GET/POST/PUT/DELETE | `/api/spawners` · `/api/spawners/:id` | Spawners de NPC |
| GET/POST/PUT/DELETE | `/api/dialogs` · `/api/dialogs/:id` | Árboles de diálogo |
| GET/POST/PUT/DELETE | `/api/loottables` · `/api/loottables/:id` | Tablas de botín |
| POST | `/api/loottables/:id/roll` | Roll real (escribe loot): query `playerLevel`, `baseGold` |
| POST | `/api/loot-tables/:id/roll` | **Preview** de frecuencias (creator, sin audit ni escritura): query `n` (1–10000, default 1000), `seed`, `playerLevel`, `boss` (0/1) |
| GET/POST/PUT/DELETE | `/api/quests` · `/api/quests/:id` | Validación previa al guardado (400 con issues) |
| GET | `/api/quests/:id/debug` | Vista de debug (personajes con la quest) |
| GET/POST/PUT/DELETE | `/api/skills` · `/api/skills/:id` | Habilidades |
| GET/PUT | `/api/settings` | Ajustes del servidor (`ServerSettings`) |
| GET/POST/DELETE | `/api/backgrounds` · `/api/backgrounds/upload` · `/api/backgrounds/:filename` | Upload multipart, máx 10 MB, png/jpg/jpeg/webp |
| GET | `/api/world/graph` · `/api/world/rooms-minimal` | Datos del grafo para el mapa del editor |
| GET | `/api/balance/enemy-scaling` | Tablas de escalado de dificultad: `{tiers, namedOverrides}` |

### Diagnóstico y salud de contenido (creator)

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/diagnostics/world` | Diagnósticos del mundo |
| GET | `/api/world/validation` | Validación world-wide |
| GET | `/api/world/reachability?from=` | Capa de alcanzabilidad (404 si la sala inicial no existe) |
| GET | `/api/world/overlays` | Capas del mapa; nombres de jugador solo para admin |
| GET | `/api/health` | Reporte content-health |
| PUT | `/api/health/mute` | `{ruleId, muted}` → silencia una regla en los contadores |
| GET | `/api/health/drift` | `{contentCommit, changes[], importedAt?}` |
| GET | `/api/health/drift/export?type=&id=` | Entidad drifted como YAML (attachment) |
| POST | `/api/validate/:entityType` | `entityType` ∈ room, npc, dialog, quest, loottable, item, script, spawner; body JSON de la entidad → resultado de validación (400 si errores) |
| POST | `/api/preview/dialog` | Preview de nodo de diálogo → `{name, nodeId, text, optionsCount, issues}` |
| POST | `/api/preview/quest` | Preview de quest → `{name, source, objectivesCount, rewards, issues}` |
| POST | `/api/preview/room` | Preview de sala → contadores + `issues` |
| POST | `/api/preview/merchant` | Preview de stock de mercader → `{name, stock[], issues}` |
| GET | `/api/search?q=&types=&limit=` | Búsqueda en el índice de contenido → `{query, hits[]}` |
| GET | `/api/refs/:type/:id` | Referencias entrantes/salientes de una entidad (404 si no está indexada) |

### LLM del creator — `/api/creator/llm/*`

| Método | Ruta | Body | Respuesta / errores |
|--------|------|------|---------------------|
| POST | `/api/creator/llm/quests/generate` | `{area?, level?, sourceNpcId?, theme?, commit?}` | `{quests[], issues[], dryRun, source, attempts, cached}`; dry-run por defecto (`commit:true` persiste); 503 sin questgen, 502 provider, 400 con `commit` si falla |
| POST | `/api/creator/llm/quests/contextual` | `{playerLevel, zoneId, theme?}` | `{quest, issues[], source, attempts, cached}`; 503 sin questgen/LLM unavailable, 400 `ErrGeneration` |
| POST | `/api/creator/llm/rooms/:id/description` | `{theme?}` opcional | `{roomId, description, current, source, cached}`; preview **no persiste**; 404 sala |
| GET | `/api/creator/llm/prompts` | — | `{prompts[]}` (id, version, task, variables, system, user, hash…) |
| POST | `/api/creator/llm/prompts` | template YAML/JSON | Guarda override; 400 si `LLM_PROMPT_DIR` no está configurado |
| POST | `/api/creator/llm/prompts/:id/test` | `{variables?, execute?}` | Render (y opcional ejecución) del prompt → `{id, version, hash, system, user, …, response?|error?}` |

### Mundo vivo y audit (creator)

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/audit?entityType=&entityId=&limit=` | Últimas filas de audit (default 100, tope 500). Cada fila: `{id, time, actorUserId, actorName, action, entityType, entityId, before?, after?, source, undoable, undoneBy?, summary?, inverse?}` |
| GET | `/api/live/npcs?templateId=&roomId=` | Instancias de NPC vivas |
| GET | `/api/live/instances` | Copias de instancia activas |

---

## 7. Ops y administración

### Admin (rol `admin`)

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/api/live/characters?online=&guest=&inCombat=&zone=&all=` | Personajes online + recién offline |
| GET | `/api/live/characters/:id` | Inventario, quests y combate actual |
| POST | `/api/ops/:action` | Ejecuta una op en vivo; body **debe** incluir `confirm:true` → `{summary, auditId, undoable, result}` |
| POST | `/api/audit/:id/undo` | Deshace una entrada de audit (CRUD inverso u ops inversa) → `{summary, auditId, undoable, result?}`; 404 si no existe, 400 si ya se deshizo o no es deshacible |
| GET | `/api/admin/users` | Todos los usuarios (redactados) |
| PUT | `/api/admin/users/:id/role` | `{role}` (`player`\|`creator`\|`admin`) |
| POST | `/api/admin/users/:id/ban` | Banea y cierra sesiones SSH vivas |
| POST | `/api/admin/users/:id/unban` | Levanta el baneo |
| DELETE | `/api/admin/users/:id` | Borra usuario (400 si es el admin) |
| GET | `/api/admin/stats/guests` | `{totalAllTime, today, thisWeek, activeNow, daily[]}` |
| GET | `/api/admin/stats/guests/daily?days=` | Serie diaria |
| GET | `/api/admin/llm/status` | `{enabled, provider, stats}` |
| GET | `/api/admin/llm/stats` | Solo contadores del pipeline |

**Ops disponibles** (`Game.RunOp`, `pkg/mudserver/game/ops.go:70-133`):

| `:action` | Body (además de `confirm:true`) |
|-----------|----------------------------------|
| `teleport` | `{characterId, roomId, force?}` |
| `give-item` | `{characterId, itemTemplateId, quantity?}` |
| `take-item` | `{characterId, itemInstanceId?, itemTemplateId?, quantity?}` |
| `npc-heal` / `npc-despawn` | `{npcInstanceId}` |
| `npc-respawn` | `{npcTemplateId, roomId}` |
| `end-combat` | `{combatInstanceId?, characterId?, npcInstanceId?}` |
| `instance-cleanup` | `{roomCopyId, allEmpty?}` |
| `quest-step` | `{characterId, questId, objectiveId, op}` |
| `regrant-starter-kit` | `{characterId}` |

Respuestas de error: 400 JSON inválido o `confirm:true` ausente, 404 op
desconocida, 503 juego no disponible; el código puede venir del `OpError`
codificado (`writeCoded`). Si la operación succeeds pero el audit falla → 500
"the operation succeeded but the audit log failed".

### Export/import (Basic Auth de entorno)

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | `/admin/export` | JSON con rooms, characters, items, scripts, NPCs, dialogs, parties (users solo con `?includeUsers=1`, redactados) |
| POST | `/admin/import` | Importa el mismo blob (borra y rellena colecciones) → `{"status":"Import successful"}` |

Sin `ADMIN_USER`/`ADMIN_PASSWORD`, o con credenciales inseguras (password < 12
car. o débiles), el middleware responde 503 (`server.go:62-89`).

---

## 8. WebSocket de juego — `GET /ws`

### Conexión y autenticación

- Upgrade en `/ws` con `WSAuthMiddleware` (`pkg/server/auth.go:439-482`): ticket
  de un solo uso (`?ticket=` obtenido con `POST /api/ws-ticket`) **o** Bearer en
  cabecera. `access_token` en query → 401.
- Origen validado contra la misma allowlist CORS (`OriginAllowed`,
  `pkg/mudserver/mudserver.go:99-109`); origen vacío permitido (no-browser).
- Una sesión nueva por usuario **reemplaza** la anterior con código de cierre
  **4001** ("session replaced"); los clientes no deben reconectar automática en
  ese código (`mudserver.go:38-43`).
- El servidor envía `ping` (`{"type":"ping"}`) cada 60 s (`mudserver.go:175-195`).
- Sesiones guest: aviso a los 5 min de caducar y cierre al expirar.

### Mensajes cliente → servidor

Payload JSON (`messages.IncomingMessage`, `pkg/mudserver/game/messages/incoming.go`):

```json
{ "message": "look", "type": "", "key": "", "proto": 0, "client": "" }
```

- **Comandos de juego**: `{"message": "<línea de comando>"}` — se despacha al
  procesador de comandos del MUD (movimiento, `offer <npc>`, `chat`, `talk`,
  combate, etc.). Ejemplo de comando nuevo (Batch 18):
  `{"message": "offer Guardia"}`, `{"message": "offer accept"}`,
  `{"message": "offer decline"}` (`pkg/mudserver/game/commands/offer.go`).
- **Handshake opcional** (protocolo 1): `{"type":"hello","proto":1,"client":"name/semver","message":""}`.
  El servidor responde `type:"hello"`, `proto:1`, `server:"talesmud/<sha>"` y
  `status:"ok" | "too-old" | "upgrade-recommended"` (advisory, no cierra el
  socket). Un hello nunca entra al bucle de comandos aunque lleve `message`.
  Detalle: `docs/structured-messages.md`.
- **Door/ANSI**: `{"type":"door_key","key":"n"}` para hotkeys (solo en sesión
  ANSI; en WS clásico se ignora).

### Mensajes servidor → cliente

Todos los payloads llevan el envelope base `MessageResponse`
(`type`, `username`, `message`; `channel`, `style`, `hook`, `source` opcionales,
`pkg/mudserver/game/messages/responses.go:63-85`) más campos estructurados
según tipo (`messagetypes.go:6-65`):

| `type` | Payload extra (resumen) |
|--------|-------------------------|
| `message` | Línea de texto (chat, respuestas de comandos) |
| `enterRoom` | `room`, `npcs[]`, `items[]`, `players[]` (render completo) |
| `roomUpdate` | Mismo shape, refresh silencioso sin re-render de descripción |
| `roomPresence` | `roomId`, `players[]` |
| `createCharacter` / `selectCharacter` / `characterSelected` | `character`, `xpForNextLevel`, `guest?` |
| `hello` | `proto`, `server`, `status` |
| `ping` | — (keepalive) |
| `dialog` / `dialogEnd` | `npcName`, `npcText`, `options[]`, `conversationID` |
| `combatStart` | `enemies[]`, `players[]` (combatants con retrato/HP), `targetId?` |
| `combatTurn` | `actorId`, `actorName`, `round`, `deadlineMs?`, cola y cooldowns |
| `combatAction` | Resolución estructurada: `action`, `result` (hit/miss/crit/…), `damage?`, `heal?`, `combatants?`, cola |
| `combatStatus` | Snapshot de cola/cooldowns/ronda (reenganche en combate) |
| `combatEnd` | `outcome` (victory/defeat/fled/timeout), `rewards?`, `loot?`, `levelUp?`, `defeat?` |
| `inventoryUpdate` | `inventory`, `equippedItems`, `gold` |
| `examineResult` | Ficha de item inspeccionado (`name`, `slot`, `quality`, `stats`, `value`, `effects[]`…) |
| `characterUpdate` | Stats vivos: HP/mana/XP/nivel/oro, atributos, derivados de combate, talents |
| `levelUp` | Ganancias de nivel (HP/mana/atributos/puntos) |
| `questAccepted` / `questProgress` / `questReady` / `questCompleted` / `questAbandoned` | `questId`, `questName`, `status`, `objectives[]`, `changedObjective?` |
| `questLog` | `quests[]` (log completo enriquecido) |
| `atlas` | `atlas` (mapa descubierto del personaje) |
| `shop` | `merchantName`, `gold`, `stock[]`, reglas de compra/venta |
| `recipes` | `recipes[]`, `roomTags?` |
| `friends` | `friends[]` (`{id, name, online}`) |
| `party` | `inParty`, `partyId?`, `leaderId?`, `members[]`, `lootRule?`, `following?` |
| `party_invite` | `pending`, `inviterName?`, `partyId?`, `expiresAt?` |

Audiencias (interna): origin/user/room/global/system — el server enruta
(`mudserver.go:566-611`). Los clientes deben ignorar tipos y campos
desconocidos. El comando `offer` responde por prosa y, al aceptar, emite
`questAccepted` + `questLog` (`offer.go:167-188`).

### Comandos de juego relacionados (no REST)

- `offer <npc>` / `offer accept|decline|no|reject|pass`: pide una quest
  contextual al NPC quest-giver de la sala (nivel + zona actuales). El servidor
  genera de forma asíncrona (rate-limit 60 s por personaje); solo se persiste
  al aceptar. Sin trabajo disponible / busy / disabled → respuestas prosaicas
  (`pkg/mudserver/game/commands/offer.go:110-119`).
- Parties, clanes (lectura de estado), canales y DMs "tell" en memoria: ver
  `docs/user/{party,clans,dms,channels,commands}.md`.

---

## 9. Estado de implementación / pendientes

- `POST /api/ssh/keys`: rechazado a propósito (el vinculado se hace por
  sign-in SSH + confirmación); no implementado como upload directo.
- No existen endpoints REST de chat global, parties ni amigos: esos sistemas
  viven en el socket (§8) y en paneles del cliente.
- Las definiciones de mazmorra (`/api/dungeons`) se persisten pero **no** se
  instancian en juego vía API (spawn/compile: no implementado aún).
- `tell` persistente entre personajes offline: no implementado (ver
  `docs/user/commands.md`).
- El editor de clases del cliente no tiene CRUD REST (`/api/classes` es solo
  GET público); ver `docs/user/admin-guide.md`.
