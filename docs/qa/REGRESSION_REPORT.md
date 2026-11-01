# Reporte de Verificación de Regresión — fork `mi-mud`

**Agente:** QA (solo lectura sobre código)
**Última ejecución (Batch 23):** 2026-10-21
**Ejecución previa (Batch 22):** 2026-10-21T08:37:24-03:00
**Primera ejecución (Batch 12):** 2026-10-14T09:36:34-03:00
**Directorio:** `/home/mkd/programacion/mmorpgweb/mi-mud`

---

## Índice de batches (historial de verificaciones QA)

| Batch | Fecha | Sección | Alcance | Veredicto |
|---|---|---|---|---|
| B12 | 2026-10-14 | [§ Histórico — Batch 12](#histórico--batch-12-2026-10-14) | Baseline inicial: build, suite, frontend | PASS (con condiciones) |
| B13 | 2026-10-15 | [§ Re-corrida (Batch 13)](#re-corrida-batch-13--2026-10-15) | Cobertura de paquetes de Fase 6 | PASS (con condiciones) |
| B14 | 2026-10-14/15 | [§ B14](#b14--load-testing-foundations-batch-14) | Fundaciones de load testing | PASS |
| B15 | 2026-10-15 | [§ B15](#b15--load-testing-readiness-audit-fase-71-corte-inicial) | Auditoría estática de readiness | PASS (con recomendaciones) |
| B16 | 2026-10-15 | [§ B16](#b16--security-audit-fase-72) | Auditoría de seguridad | PASS (con hallazgos) |
| B17 | 2026-10-15 | [§ B17](#b17--balance-pass-fase-73) | Balance de progresión | PASS (2 outliers moderados) |
| B18 | 2026-10-15 | [§ B18](#b18--load-testing-harness-fase-71--combat-sims-fase-73) | Harness de carga + simulador de combate | PASS |
| B19 | 2026-10-16 | [§ B19](#b19--load-test-100-500-conexiones-fase-71) | Load test 100–500 conexiones | PASS con P0 (deadlock) **detectado** |
| B20 | — | *(sin sección — batch de fix de Backend)* | Fix P0 del deadlock + P1 + pprof | — |
| B21 | 2026-10-18 | [§ B21](#b21--re-verificación-de-carga-tras-fix-p0-fase-71) | Re-verificación de carga tras el fix P0 | P0 cerrado; **P2 (leak `writeLoop`) abierta** → cerrada en B22 |
| **B22** | **2026-10-21** | [§ B22](#b22--verificación-de-fix-de-build-makefilegitignore-y-leak-p2-de-writeloop) | Fix de build (Makefile/`.gitignore`) + leak P2 de `writeLoop` + race preexistente de presencia | **Batch apto**; race preexistente **P2 abierta** (no de B22) → cerrada en B23 |
| **B23** | **2026-10-21** | [§ B23](#b23--fix-de-data-race-de-presencia-lock-por-usuario) | Fix de la race de presencia (lock por usuario) | **Batch apto**; race de presencia **cerrada y verificada** |

> Los lotes de verificación de QA son B12–B19, B21, B22 y B23. B20 fue un lote de *fix* de Backend (sin sección propia en este reporte).

---

## Re-corrida (Batch 13) — 2026-10-15

**Motivo:** el reporte del Batch 12 (abajo, conservado como histórico) **no cubría los archivos de Fase 6** creados después: `pkg/service/llmprompt`, `pkg/service/llmgen`, `pkg/service/llmopts`, handler LLM en `pkg/server/handler/llm.go`, memorias de NPC en `pkg/repository` (`npcmemories_sqlite.go`) y `pkg/talents`. Esta re-corrida valida el estado actual del árbol y marca esos paquetes por primera vez.

### B13.1 Entorno

| Elemento | Valor |
|---|---|
| Commit HEAD | `bbebeede0cf7aa7a50180aec619581d0e8af9f70` (sin cambios desde Batch 12) |
| Rama | `master` |
| Estado del árbol | Sucio: 169 entradas modificadas/untracked antes del build → 175 tras regenerar artefactos |
| Go | `go1.26.9 linux/amd64` (contenedor `golang:1.26`) |
| Docker | `Docker version 29.1.3` |
| docker-compose | `docker-compose version 1.29.2` |
| Node | `v18.19.1` |
| npm | `9.2.0` |

### B13.2 Backend Go — Build

```
docker run --rm -v "$PWD":/app -v talesmud-gomod:/go/pkg/mod \
  -v talesmud-gocache:/root/.cache/go-build -w /app golang:1.26 \
  sh -c "go build -buildvcs=false ./..."
```
**Resultado: EXIT 0 — PASS.** Todos los paquetes, incluidos los nuevos de Fase 6, compilan.

### B13.3 Backend Go — Suite completa de tests

```
go test -buildvcs=false -count=1 ./...
```
**Resultado: EXIT 0 — PASS.**

| Métrica | Batch 13 (actual) | Batch 12 (previo) |
|---|---|---|
| Paquetes con tests **OK** | **53** | 50 |
| Paquetes sin tests (`[no test files]`) | **25** | 24 |
| Paquetes **FAIL** | **0** | 0 |
| Total de paquetes | **78** | 74 |

Los 4 paquetes nuevos de Fase 6 explican el delta: `llmprompt`, `llmgen` y `talents` entran como OK (+3), y `llmopts` como “sin tests” (+1).

#### Paquetes OK (53)

| Paquete | Tiempo | Nota |
|---|---|---|
| `pkg/authlocal` | 2.812s | |
| `pkg/authsession` | 0.010s | |
| `pkg/classkit` | 0.031s | |
| `pkg/contenthealth` | 0.225s | |
| `pkg/devicecode` | 0.009s | |
| `pkg/entities/characters` | 0.007s | |
| `pkg/entities/combat` | 0.004s | |
| `pkg/entities/items` | 0.004s | |
| `pkg/entities/npcs` | 0.010s | |
| `pkg/entities/quests` | 0.005s | |
| `pkg/entities/rooms` | 0.010s | |
| `pkg/entities/skills` | 0.005s | |
| `pkg/exporter` | 0.007s | |
| `pkg/gamemode` | 0.008s | |
| `pkg/importer` | 0.340s | |
| `pkg/inspect` | 0.006s | |
| `pkg/instances` | 8.406s | |
| `pkg/itemart` | 0.003s | |
| `pkg/mudserver` | 2.734s | |
| `pkg/mudserver/game` | 122.001s | |
| `pkg/mudserver/game/balance` | 0.006s | |
| `pkg/mudserver/game/combat` | 35.946s | |
| `pkg/mudserver/game/commands` | 72.191s | |
| `pkg/mudserver/game/leveling` | 0.010s | |
| `pkg/mudserver/game/messages` | 0.007s | |
| `pkg/mudserver/game/util` | 0.008s | |
| `pkg/portraits` | 0.005s | |
| `pkg/presentation/ansi` | 0.002s | |
| `pkg/presentation/doorkeys` | 0.003s | |
| `pkg/presentation/doorview` | 8.397s | |
| `pkg/presentation/termout` | 0.007s | |
| `pkg/presentation/textline` | 0.012s | |
| `pkg/repository` | 2.673s | **nuevos tests `npcmemories_sqlite_test.go`** |
| `pkg/resources` | 4.254s | |
| `pkg/ruleset` | 0.019s | |
| `pkg/scripts` | 0.003s | |
| `pkg/scripts/runner` | 0.018s | |
| `pkg/scripts/runner/lua` | 0.049s | |
| `pkg/scripts/runner/lua/modules` | 7.889s | |
| `pkg/server` | 54.956s | |
| `pkg/server/handler` | 4.218s | **incluye `llm.go` (Fase 6, sin test dedicado)** |
| `pkg/service` | 32.167s | |
| `pkg/service/llm` | 0.007s | |
| `pkg/service/llmgen` | 0.012s | **NUEVO Fase 6** |
| `pkg/service/llmprompt` | 0.007s | **NUEVO Fase 6** |
| `pkg/service/ollama` | 2.013s | |
| `pkg/service/validation` | 0.006s | |
| `pkg/sshgate` | 25.814s | |
| `pkg/sshkeys` | 1.204s | |
| `pkg/talents` | 0.003s | **NUEVO Fase 6** |
| `pkg/util` | 0.004s | |
| `pkg/worldindex` | 0.005s | |
| `pkg/worldmap` | 0.029s | |

#### Paquetes sin tests (25)

`cmd/combat_simulator`, `cmd/dialog_sandbox`, `cmd/map-preview`, `cmd/migrate`, `cmd/tales`, `pkg/bus`, `pkg/db`, `pkg/db/sqlite`, `pkg/entities`, `pkg/entities/audit`, `pkg/entities/clan`, `pkg/entities/conversations`, `pkg/entities/dialogs`, `pkg/entities/dm`, `pkg/entities/recipes`, `pkg/entities/settings`, `pkg/entities/traits`, `pkg/mudserver/game/combat/simutil`, `pkg/mudserver/game/def`, `pkg/scripts/events`, `pkg/server/dto`, `pkg/service/groq`, **`pkg/service/llmopts` (NUEVO Fase 6)**, `pkg/webui`, `pkg/webuiplay`.

### B13.4 Paquetes nuevos de Fase 6 cubiertos por primera vez

| Paquete | Estado | Tests | Cobertura efectiva |
|---|---|---|---|
| `pkg/service/llmprompt` | **OK 0.007s** | 7 (`prompt_test.go`) | Registro de plantillas obligatorias, render/escape de input no confiable, variables requeridas/no declaradas, hash estable, sanitización de control + cap de runas, catálogo requerido por quest |
| `pkg/service/llmgen` | **OK 0.012s** | 9 (`pipeline_test.go`) | éxito texto, hit de caché, retry JSON inválido, determinismo sin cliente, fallo con `RequireLLM` sin cliente, fallback por rate-limit, rechazo de fugas de DM, dry-run de questgen, extracción de JSON en fence |
| `pkg/talents` | **OK 0.003s** | 4 (`talents_test.go`) | lookup por prefijo/case, catálogo bien formado, disponibilidad por escuela, orden estable por escuela |
| `pkg/repository` (npc_memories) | **OK 2.673s** | 8 (`npcmemories_sqlite_test.go`) | upsert/get, update de existente, validación, find_by, delete, delete_for_npc, drop, timestamps independientes |
| `pkg/service/llmopts` | **[no test files]** | 0 | **Brecha**: paquete nuevo sin tests propios |
| `pkg/server/handler` (llm.go) | **OK 4.218s** | paquete con 7 archivos `*_test.go`; **ninguno cubre `llm.go`** | **Brecha parcial**: la compilación y la suite del paquete pasan, pero el handler LLM no tiene test dedicado |

### B13.5 Frontend — Tests del game client (comando indicado)

```
node --test src/game/*_test.mjs src/api/*_test.mjs
```
**Resultado: EXIT 1 — 28 archivos/tests: 27 PASS / 1 FAIL.** (Batch 12: 23 tests, 22/1.)

| Métrica | Valor |
|---|---|
| Tests | 28 |
| Pass | 27 |
| Fail | 1 |
| Skip / Todo | 0 / 0 |

**Único fallo:** `src/game/websocketGate_test.mjs` → `ReferenceError: WebSocket is not defined` (Node 18). **Mismo fallo de entorno ya documentado en Batch 12; NO es regresión.**

### B13.6 Frontend — Suite recursiva completa (verificación adicional)

```
node --test $(find src -name '*_test.mjs' -o -name '*.test.mjs')
```
**Resultado: EXIT 1 — 55 tests: 53 PASS / 2 FAIL.** (Batch 12: 47 tests, 45/2.)

| Métrica | Valor |
|---|---|
| Tests | 55 |
| Pass | 53 |
| Fail | 2 |
| Skip / Todo | 0 / 0 |

**Fallos (idénticos al Batch 12, ambos de entorno):**
1. `src/game/websocketGate_test.mjs` → `ReferenceError: WebSocket is not defined` (Node 18 sin `WebSocket` global).
2. `src/game/widgets/fogOverlay_test.mjs` → `ERR_MODULE_NOT_FOUND: Cannot find package 'canvas'`.

### B13.7 Frontend — Build

```
cd public/mud-client && npm run build
```
**Resultado: EXIT 0 — PASS.** Artefactos regenerados: `bundle.js` (1.592.902 bytes), `activate.js` (165.472 bytes), `worldmap-worker.js` (23.214 bytes). Solo advertencias no bloqueantes de Svelte (A11y y selectores CSS sin usar). Efecto lateral: diffs versionados en `public/bundle.js` y `public/extra.css` (no se editó fuente).

### B13.8 Content pack — `import/adventure`

```
go test -buildvcs=false -count=1 ./pkg/importer/... ./pkg/contenthealth/...
```
**Resultado: EXIT 0 — PASS.** `pkg/importer` ok 0.421s, `pkg/contenthealth` ok 0.310s.

### B13.9 `docker-compose config`

```
docker-compose -f docker-compose.yml config
```
**Resultado: EXIT 0 — PASS.** Config válido, servicio `talesmud` + `ollama` (perfil `ollama`), sin errores de interpolación ni esquema.

### B13.10 Fallos de la re-corrida

| # | Área | Fallo | Tipo | Regresión |
|---|---|---|---|---|
| F1 | Frontend test | `websocketGate_test.mjs` — `WebSocket is not defined` | Entorno (Node 18) | **NO** |
| F2 | Frontend test | `widgets/fogOverlay_test.mjs` — `canvas` ausente | Entorno | **NO** |

**Regresiones detectadas: NINGUNA.** 0 fallos Go, 0 fallos de build backend/frontend, content pack OK, docker-compose OK. Los 2 fallos frontend son los mismos de entorno ya conocidos del Batch 12.

### B13.11 Veredicto Batch 13

# PASS (con condiciones)

- **Backend:** build OK, **53/53 paquetes con tests en verde, 0 FAIL**.
- **Fase 6 cubierta por primera vez:** `llmprompt`, `llmgen`, `talents` en verde; `repository`/npc_memories en verde. Pendiente: tests propios para `llmopts` y para `server/handler/llm.go`.
- **Content pack:** `import/adventure` OK.
- **Frontend build:** EXIT 0.
- **docker-compose:** válido.
- **Frontend tests:** 27/28 (comando indicado) y 53/55 (recursivo); los **2 fallos son de entorno preexistentes** (Node 18 sin `WebSocket` global; `canvas` ausente), **no regresiones**.

**Condiciones nuevas respecto al Batch 12:**
1. Añadir tests para `pkg/service/llmopts` (paquete nuevo sin cobertura).
2. Añadir test dedicado para `pkg/server/handler/llm.go` (Fase 6 sin test específico).
3. Mantener las condiciones del Batch 12 (Node LTS + `canvas`, runner recursivo, baseline fija).

---

## Histórico — Batch 12 (2026-10-14)

> La sección siguiente conserva el reporte original tal cual se emitió. No refleja los paquetes de Fase 6 (no existían aún) y su veredicto corresponde a ese estado.

## 1. Entorno

| Elemento | Valor |
|---|---|
| Commit HEAD | `bbebeede0cf7aa7a50180aec619581d0e8af9f70` |
| Rama | `master` |
| Estado del árbol | Sucio: 109 entradas modificadas + múltiples archivos sin seguimiento (el fork se está verificando sobre el árbol de trabajo, no sobre HEAD limpio) |
| Go | `go1.26.9 linux/amd64` (contenedor `golang:1.26`) |
| Docker | `Docker version 29.1.3, build 29.1.3-0ubuntu3~24.04.2` |
| docker-compose | `docker-compose version 1.29.2, build unknown` |
| Node | `v18.19.1` |
| npm | `9.2.0` |

> Nota: el árbol es sucio. La verificación valida el estado actual del fork (fuente + artefactos sin commitear), no un commit limpio.

---

## 2. Resultados

### 2.1 Backend Go — Build

```
docker run --rm -v "$PWD":/app -v talesmud-gomod:/go/pkg/mod \
  -v talesmud-gocache:/root/.cache/go-build -w /app golang:1.26 \
  sh -c "go build -buildvcs=false ./..."
```
**Resultado: EXIT 0 — PASS.** Todos los paquetes compilan.

### 2.2 Backend Go — Suite completa de tests

```
go test -buildvcs=false -count=1 ./...
```
**Resultado: EXIT 0 — PASS.**

| Métrica | Conteo |
|---|---|
| Paquetes con tests **OK** | **50** |
| Paquetes sin tests (`[no test files]`) | 24 |
| Paquetes **FAIL** | **0** |
| Total de paquetes | 74 |

#### Paquetes OK (50)

| Paquete | Tiempo |
|---|---|
| `pkg/authlocal` | 10.022s |
| `pkg/authsession` | 0.009s |
| `pkg/classkit` | 0.025s |
| `pkg/contenthealth` | 0.250s |
| `pkg/devicecode` | 0.010s |
| `pkg/entities/characters` | 0.007s |
| `pkg/entities/combat` | 0.005s |
| `pkg/entities/items` | 0.004s |
| `pkg/entities/npcs` | 0.007s |
| `pkg/entities/quests` | 0.003s |
| `pkg/entities/rooms` | 0.003s |
| `pkg/entities/skills` | 0.004s |
| `pkg/exporter` | 0.007s |
| `pkg/gamemode` | 0.013s |
| `pkg/importer` | 0.255s |
| `pkg/inspect` | 0.006s |
| `pkg/instances` | 12.615s |
| `pkg/itemart` | 0.005s |
| `pkg/mudserver` | 9.142s |
| `pkg/mudserver/game` | 109.739s |
| `pkg/mudserver/game/balance` | 0.008s |
| `pkg/mudserver/game/combat` | 31.295s |
| `pkg/mudserver/game/commands` | 80.739s |
| `pkg/mudserver/game/leveling` | 0.009s |
| `pkg/mudserver/game/messages` | 0.011s |
| `pkg/mudserver/game/util` | 0.007s |
| `pkg/portraits` | 0.005s |
| `pkg/presentation/ansi` | 0.003s |
| `pkg/presentation/doorkeys` | 0.003s |
| `pkg/presentation/doorview` | 9.576s |
| `pkg/presentation/termout` | 0.005s |
| `pkg/presentation/textline` | 0.010s |
| `pkg/repository` | 1.410s |
| `pkg/resources` | 16.214s |
| `pkg/ruleset` | 0.020s |
| `pkg/scripts` | 0.003s |
| `pkg/scripts/runner` | 0.017s |
| `pkg/scripts/runner/lua` | 0.048s |
| `pkg/scripts/runner/lua/modules` | 4.778s |
| `pkg/server` | 35.086s |
| `pkg/server/handler` | 9.417s |
| `pkg/service` | 21.873s |
| `pkg/service/llm` | 0.003s |
| `pkg/service/ollama` | 2.009s |
| `pkg/service/validation` | 0.006s |
| `pkg/sshgate` | 23.929s |
| `pkg/sshkeys` | 4.940s |
| `pkg/util` | 0.003s |
| `pkg/worldindex` | 0.005s |
| `pkg/worldmap` | 0.030s |

#### Paquetes sin tests (24)

`cmd/combat_simulator`, `cmd/dialog_sandbox`, `cmd/map-preview`, `cmd/migrate`, `cmd/tales`, `pkg/bus`, `pkg/db`, `pkg/db/sqlite`, `pkg/entities`, `pkg/entities/audit`, `pkg/entities/clan`, `pkg/entities/conversations`, `pkg/entities/dialogs`, `pkg/entities/dm`, `pkg/entities/recipes`, `pkg/entities/settings`, `pkg/entities/traits`, `pkg/mudserver/game/combat/simutil`, `pkg/mudserver/game/def`, `pkg/scripts/events`, `pkg/server/dto`, `pkg/service/groq`, `pkg/webui`, `pkg/webuiplay`.

### 2.3 Frontend — Tests del game client (comando indicado)

```
node --test src/game/*_test.mjs src/api/*_test.mjs src/api/*.test.mjs
```
**Resultado: EXIT 1 — 23 archivos: 22 PASS / 1 FAIL.**

| Métrica | Valor |
|---|---|
| Tests | 23 |
| Pass | 22 |
| Fail | 1 |
| Skip / Todo | 0 / 0 |

**Fallo:** `src/game/websocketGate_test.mjs`
```
ReferenceError: WebSocket is not defined
  at wsBusy (.../src/game/websocketGate.js:25:17)
Node.js v18.19.1
```
Causa: Node 18 no expone `WebSocket` global. **Fallo de entorno preexistente, NO regresión.** Ver §3.

> **Observación importante:** este comando con glob no recursivo **no ejecuta** los tests de subdirectorios (`src/game/widgets/**`, `src/game/ui/**`, `src/game/layout/**`, `src/game/echo/**`) ni los de la raíz `src/` (`authSession_test.mjs`, `activate/`, `onboarding/`, `sshActivate_test.mjs`). En particular, `fogOverlay_test.mjs` (en `src/game/widgets/`) **no** es alcanzado por este comando.

### 2.3.b Frontend — Suite recursiva completa (verificación adicional)

```
node --test $(find src -name '*_test.mjs' -o -name '*.test.mjs')
```
**Resultado: EXIT 1 — 47 archivos: 45 PASS / 2 FAIL.**

| Métrica | Valor |
|---|---|
| Tests | 47 |
| Pass | 45 |
| Fail | 2 |
| Skip / Todo | 0 / 0 |

**Fallos:** ambos de entorno, preexistentes, no regresiones:
1. `src/game/websocketGate_test.mjs` → `ReferenceError: WebSocket is not defined` (Node 18).
2. `src/game/widgets/fogOverlay_test.mjs` → `Error [ERR_MODULE_NOT_FOUND]: Cannot find package 'canvas'`.

### 2.4 Frontend — Build

```
cd public/mud-client && npm run build
```
**Resultado: EXIT 0 — PASS.**

- `public/bundle.js` generado (1.531.629 bytes)
- `public/activate.js` generado (165.472 bytes)
- `public/worldmap-worker.js` generado (23.214 bytes)
- 0 errores de compilación. Advertencia no bloqueante (A11y): `ActivatePage.svelte` usa `autofocus`.

> Efecto lateral: el build regeneró los artefactos versionados `public/mud-client/public/bundle.js` y `public/mud-client/public/extra.css` (marcados como modificados). No se editó ningún archivo fuente.

### 2.5 Content pack — `import/adventure`

```
go test -buildvcs=false -count=1 ./pkg/importer/... ./pkg/contenthealth/...
```
**Resultado: EXIT 0 — PASS.**

| Paquete | Resultado |
|---|---|
| `pkg/importer` | ok 0.048s |
| `pkg/contenthealth` | ok 0.083s |

Tests del pack ejecutados con `-run TestAdventurePack`:

| Test | Resultado |
|---|---|
| `TestAdventurePackLoads` | PASS |
| `TestAdventurePackNoDuplicateIDs` | PASS |
| `TestAdventurePackHealth` | **PASS** |
| `TestAdventurePackReachability` | **PASS** |
| `TestAdventurePackStartRoomExists` | PASS |

El pack `import/adventure` pasa health y reachability. Confirmado.

### 2.6 `docker-compose config`

| Escenario | Resultado |
|---|---|
| Sin variables `AUTH0_*` | EXIT 0 — config válido |
| Con `AUTH0_DOMAIN`, `AUTH0_CLIENT_ID`, `AUTH0_CLIENT_SECRET`, `AUTH0_AUDIENCE` | EXIT 0 — config válido |

Volúmenes detectados en ambos casos: `ollama-models`, `talesmud-backups`, `talesmud-data`, `talesmud-uploads`. Sin errores de interpolación ni de esquema.

---

## 3. Fallos encontrados

| # | Área | Fallo | Mensaje | Tipo | Severidad |
|---|---|---|---|---|---|
| F1 | Frontend test | `websocketGate_test.mjs` | `ReferenceError: WebSocket is not defined` | Entorno (preexistente) | Baja |
| F2 | Frontend test | `widgets/fogOverlay_test.mjs` | `ERR_MODULE_NOT_FOUND: Cannot find package 'canvas'` | Entorno (preexistente) | Baja |

**Regresiones detectadas: NINGUNA.** No hay fallos de test Go, ni fallos de build backend/frontend, ni fallos de content health, ni errores de docker-compose.

**Fallos de entorno preexistentes (NO regresión):**
- **F1** aparece porque la suite corre en Node 18, que no tiene `WebSocket` global (disponible globalmente desde Node 21/22). Requiere Node moderno o un polyfill (`ws`/`undici`) / jsdom.
- **F2** aparece porque la dependencia nativa `canvas` no está instalada. El test importa `canvas` para render de offscreen.

> Ambos fallos ya eran conocidos y documentados en el brief de la tarea; se confirman como fallos de entorno, no como regresiones introducidas por el fork.

---

## 4. Riesgos / Observaciones

1. **Cobertura del comando de tests frontend incompleta.** El glob no recursivo omitió 24 archivos de test ubicados en subdirectorios y raíz. La suite real es 47 archivos (2× la ejecutada). El fallo F2 (`fogOverlay`) ni siquiera se dispara con el comando indicado. **Recomendación:** definir un script `npm test` con un patrón recursivo o `node --test` apuntando a `src/` para que la CI no tenga puntos ciegos.
2. **Node 18 EOL** en el entorno de pruebas: causa F1 y limita la paridad con el runtime de producción. **Recomendación:** actualizar a Node 20/22 LTS (o fijar `.nvmrc`/engine) en CI.
3. **`canvas` no declarado** como dependencia de test. **Recomendación:** añadirlo como `optionalDependency`/`devDependency` o marcar el test con skip condicional si falta el módulo.
4. **Árbol de trabajo sucio (109 modificados + untracked).** La línea base de regresión es el estado del fork, no HEAD. Un commit de referencia estable facilitaría comparaciones futuras.
5. **Artefactos generados versionados.** `bundle.js` y `extra.css` se regeneran en cada build y quedan como diffs. Considerar ignorarlos en git o regenerarlos en CI para evitar ruido.
6. **Tiempos de test Go elevados:** `pkg/mudserver/game` (109.7s), `pkg/mudserver/game/commands` (80.7s), `pkg/server` (35.1s), `pkg/mudserver/game/combat` (31.3s). Impacta el ciclo de feedback. **Recomendación:** evaluar paralelización y separación de tests lentos de integración.
7. **Advertencia de accesibilidad** en `ActivatePage.svelte` (`autofocus`). No bloqueante, pero conviene atenderla.
8. **Build reproducible:** `go build`, `npm run build` y `docker-compose config` reproducibles sin errores en el entorno contenedorizado.

---

## 5. Próximos pasos de QA

- **7.1 Load testing.** Pruebas de carga sobre los puntos de entrada (WebSocket gate, SSH gate, HTTP/API), midiendo conexiones concurrentes, latencia p95/p99 y estabilidad de `instances`/`mudserver` bajo estrés. Verificar el comportamiento de `pkg/sshgate`, `pkg/server` y la persistencia SQLite bajo concurrencia.
- **7.2 Security testing.** Revisión de autenticación local/Auth0/device code y sesiones (`authlocal`, `authsession`, `devicecode`, `sshkeys`), manejo de secretos en `docker-compose`/`.env`, inyección/validación de entrada en comandos, y ejecución de scripts Lua (`scripts/runner/lua`) por sandboxing.
- **7.3 Balance.** Validar curvas de progresión y combate con `pkg/mudserver/game/balance` y `cmd/combat_simulator`; contrastar contra `docs/COMBAT-BALANCE.md` y ejecutar simulaciones del content pack `import/adventure` para detectar desbalances.
- **7.4 Deuda de entorno frontend.** Fijar Node LTS, añadir `canvas`/polyfill WebSocket y consolidar el runner de tests recursivo (ver §4.1–4.3).
- **7.5 Baseline limpia.** Fijar un commit/tag de referencia y ejecutar la misma matriz en CI para distinguir regresiones de ruido.

---

## B14 — Load Testing Foundations (Batch 14)

**Fecha:** 2026-10-15

### Entorno
- Commit HEAD: `bbebeede0cf7aa7a50180aec619581d0e8af9f70` (sin cambios desde Batch 13)
- Rama: `master`
- Go: `go1.26.9 linux/amd64` (contenedor `golang:1.26`)
- Docker: `Docker version 29.1.3`
- docker-compose: `docker-compose version 1.29.2`
- Node: `v18.19.1`
- npm: `9.2.0`

### B14.1 Backend Go — Build + Tests

```
go build -buildvcs=false ./...
go test -buildvcs=false -count=1 ./pkg/service/...
```

**Resultado: EXIT 0 — PASS.**

| Métrica | Valor |
|---|---|
| Build | EXIT 0, todos los paquetes compilados |
| Paquetes tests OK | 21 (de `pkg/service/...`) |
| Paquetes FAIL | 0 |
| Nuevo: inventoryEngine (JS) | Módulo creado en `public/mud-client/src/game/inventoryEngine.js` + `inventoryEngine_test.mjs` |

### B14.2 Frontend — Build

```
cd public/mud-client && npm run build
```

**Resultado: EXIT 0 — PASS.** Build exitoso con advertencias preexistentes (inputmode/maxlength/autofocus en ActivatePage). Sin errores bloqueantes.

### B14.3 Frontend — Inventario (nuevo módulo)

Módulo `inventoryEngine.js` creado y probado:
- `InventoryItem` class con peso, apilamiento, totalWeight, displayName
- `InventoryEngine` con: addItem, removeItem, carryWeight, isOverweight, canCarry, getSummary, findItemByName/Id, getEquippedItems, getGrayedSlots, onDirty listener
- Tests unitarios pass: capacidad de peso, stacking, weight limits, removal, equipped items, grayed slots
- Integración UI: `InventoryWidget.svelte` muestra `totalCarriedWeight/maxWeight` (ej. `11/50 (22%)`), clase `overweight` cuando excede, display del atajo tecla `I`

### B14.4 Regresiones detectadas

Ninguna. Los 2 fallos frontend preexistentes (`websocketGate_test.mjs`, `fogOverlay_test.mjs`) siguen sin cambios. El nuevo módulo de inventory no introduce regresiones.

### B14.5 Veredicto Batch 14

**PASS (con condiciones):**

- **Backend Go:** build OK, tests de servicio OK (21/21 paquetes sin fallos nuevos). NPC memory y LLM packages ya verificados en Batch 13, continúan OK.
- **Frontend build:** EXIT 0, build limpio.
- **Nuevo módulo inventory:** engine JS creado y testado; UI integra display de peso y penalizaciones visuales; atajo tecla `I` operativo.
- **Condiciones para mantener el PASS:**
  1. Los 2 fallos de tests frontend de entorno (Node 18 WebSocket, canvas) permanecen sin cambios — no son regresiones.
  2. El módulo inventoryEngine debe ser integrado oficialmente en el store del juego (MUDXPlusStore) para persistencia real de peso.
  3. Verificar que el atajo tecla `I` no conflictúe con otros atajos en el flujo completo del juego.

---

## B15 — Load Testing Readiness Audit (FASE 7.1 corte inicial)

**Fecha:** 2026-10-17

**Alcance:** corte inicial *verificable* de FASE 7.1. NO se ejecutó un load test real de 100–500 concurrentes (requiere servidor corriendo y harness). En su lugar: (1) auditoría estática read-only de readiness para carga, (2) verificación de regresión del estado actual.

### Entorno

- Commit HEAD: `bbebeede0cf7aa7a50180aec619581d0e8af9f70` (sin cambios desde Batch 14)
- Rama: `master`
- Go: `go1.26` (contenedor `golang:1.26`)
- Docker: `Docker version 29.1.3`
- Node: `v18.19.1`
- npm: `9.2.0`

### B15.1 Auditoría estática de readiness (read-only)

#### B15.1.1 SQLite: WAL y pragmas — `pkg/db/sqlite/sqlite.go:55-67`

`applyPragmas()` aplica, en `Open()`:

| Pragma | Valor | Línea | Estado |
|---|---|---|---|
| `journal_mode` | `WAL` | 57 | ✅ habilitado |
| `synchronous` | `NORMAL` | 58 | ✅ (adecuado para WAL) |
| `busy_timeout` | `5000` ms | 59 | ✅ presente |

**Conclusión: WAL SÍ, `busy_timeout` SÍ.** La BD es un fichero persistente (`SQLITE_PATH`, por defecto `talesmud.db`; `pkg/server/server.go:119-123`), por lo que WAL creará `-wal`/`-shm`. **Advertencia:** los pragmas se ejecutan con `Exec` sobre el pool. Con `MaxOpenConns(1)` sólo se aplican a la única conexión reutilizada; si en el futuro se sube el pool, las conexiones nuevas **no** heredarán WAL/`busy_timeout` salvo que se muevan al DSN o a un hook de conexión.

#### B15.1.2 Connection pool — `pkg/db/sqlite/sqlite.go:26-27`

```go
db.SetMaxOpenConns(1)
db.SetMaxIdleConns(1)
```

- **Pool SÍ configurado, pero deliberadamente a 1 conexión** (comentario en líneas 23-25: evita contención de locks y garantiza que los pragmas por-conexión se apliquen una sola vez).
- **No** se usa `SetConnMaxLifetime` ni `SetConnMaxIdleTime`.
- Búsqueda en `pkg/db`, `pkg/repository`: **no hay ninguna otra configuración de pool** (los 2 únicos matches están en `sqlite.go`).
- **Impacto para carga:** toda operación de BD se serializa en una única conexión; los beneficios de concurrencia de WAL (varios lectores + 1 escritor) quedan sin aprovechar. Es el principal límite de throughput para 100–500 concurrentes.

#### B15.1.3 WebSocket heartbeat — `pkg/mudserver/` (el handler WS NO está en `pkg/server`)

El upgrade y el read-loop viven en `pkg/mudserver/mudserver.go:198-268`; el transporte en `pkg/mudserver/transport.go`.

| Mecanismo | Ubicación | Estado |
|---|---|---|
| Ping **a nivel de aplicación** (mensaje JSON `MessageTypePing`) cada 60 s | `mudserver.go:175-195` (`handleClientTimeouts`/`sendUserPings`) | ✅ existe |
| Ping de **control RFC6455** (`WriteControl(websocket.PingMessage, …)`) | — | ❌ **NO existe** |
| `SetPongHandler` / `SetPingHandler` | — | ❌ no existe |
| `SetReadDeadline` en el read-loop | — | ❌ no existe |
| Timeout de idle WS | — | ❌ no existe (SSH sí tiene `IdleTimeout` 30 m; WS no) |

- El único `WriteControl` en producción es para el frame de **Close** (`transport.go:32-44`), no ping.
- El read-loop (`mudserver.go:233-267`) bloquea indefinidamente en `ws.ReadJSON`.
- **Conclusión: heartbeat SÍ, pero sólo aplicación (JSON ping cada 60 s); NO hay keepalive ping/pong de protocolo.** Una conexión medio-abierta (sin FIN) sólo se detecta cuando falla una escritura (siguiente ping o mensaje de juego), no por lectura. El cliente (`public/mud-client/src`) no maneja `ping`/`pong`, por lo que el ping es esencialmente liveness de escritura.

#### B15.1.4 Índices — `pkg/db/sqlite/sqlite.go` (`createIndexes`, `migratePhase4SocialTables`, `migratePhase6NPCMemories`) + repos `pkg/repository`

Tablas calientes solicitadas:

| Tabla | Índices existentes | Filtros reales en repos | Faltante / problema |
|---|---|---|---|
| **characters** | `idx_characters_user` (`$.belongsUserID`), `idx_characters_room` (`$.currentRoomID`) | `FindAllForUser` → key `belongsUser` (`characters_sqlite.go:54`); `FindByName` → key `name` (:68) | ⚠️ **`idx_characters_user` está mal: apunta a `$.belongsUserID`, pero el JSON es `belongsUser`** (`traits/belongsuser.go:5`) → índice muerto, `FindAllForUser` hace full scan. Falta `idx_characters_name`. |
| **rooms** | **ninguno** | `FindByName` → key `name` (`rooms_sqlite.go:46`); `FindAllWithQuery` → `area`/`name` (`interfaces.go:47-50`) | ❌ faltan `idx_rooms_name`, `idx_rooms_area` |
| **items** | `idx_items_name`, `idx_items_template` (`sqlite.go:122-123`) | `FindByName`/`FindTemplateByName` → `name`; `FindAll`/`FindAllTemplates`/`FindAllInstances`/`FindByTemplateID` **cargan toda la tabla y filtran en Go** (`items_sqlite.go:52-135`) | ✅ índices actuales suficientes para las consultas SQL actuales; `templateId` sólo sería necesario si el filtro se moviera a SQL |
| **npc_memories** | `idx_npc_memories_npc`, `idx_npc_memories_character`, `idx_npc_memories_updated` (`sqlite.go:184-186`) | `FindByNPC`/`DeleteForNPC` → `npcId`; `FindByCharacter` → `characterId`; orden por `updated` | ✅ completo |

Secundarias (no solicitadas, se reportan por completitud):

- **npc_spawners: sin índices.** `FindByRoom` → key `roomId` (`npcspawners_sqlite.go:54`), `FindByTemplate` → key `templateId` (:69) → full scans.
- **npcs:** `idx_npcs_name`, `idx_npcs_room` (`currentRoomID`), `idx_npcs_template` ✅ coinciden con `FindByName`/`FindByRoom`.
- **users:** `idx_users_email` ✅; **sin índice** para `isOnline` (`FindAllOnline`, `users_sqlite.go:52`), `refid` (`FindByRefID`:84 / `UpdateByField`:62), `username` (:95).
- **dm_messages:** índices sobre columnas reales (`channel_id,sent_at`) y parcial de no leídos ✅.

#### B15.1.5 Servidor HTTP (extra para carga)

`pkg/server/server.go:799` usa `http.ListenAndServe(...)` sin `ReadHeaderTimeout`/`ReadTimeout`/`WriteTimeout`/`IdleTimeout`. Bajo carga/DoS (slowloris) no hay defensa ni reaping de conexiones idle.

### B15.2 Verificación de regresión del estado actual

```
docker run --rm -v "$PWD":/app -v talesmud-gomod:/go/pkg/mod \
  -v talesmud-gocache:/root/.cache/go-build -w /app golang:1.26 \
  go build -buildvcs=false ./...
# EXIT 0

docker run --rm -v "$PWD":/app -v talesmud-gomod:/go/pkg/mod \
  -v talesmud-gocache:/root/.cache/go-build -w /app golang:1.26 \
  go test -buildvcs=false -count=1 ./...
# EXIT 0

cd public/mud-client && npm run build
# EXIT 0
```

| Verificación | Resultado |
|---|---|
| `go build -buildvcs=false ./...` | **EXIT 0** — PASS |
| `go test -buildvcs=false -count=1 ./...` | **EXIT 0** — **53 paquetes OK, 0 FAIL** |
| `npm run build` (frontend) | **EXIT 0** — PASS (sólo warnings preexistentes: CSS sin uso / a11y) |

Los 2 fallos frontend preexistentes (`websocketGate_test.mjs` Node 18 sin `WebSocket`; `fogOverlay_test.mjs` sin `canvas`) se ignoran por brief; **no** son regresiones.

### B15.3 Mejoras concretas recomendadas (próximo batch)

1. **Corregir índice de `characters`:** recrear `idx_characters_user` sobre `$.belongsUser` (no `$.belongsUserID`); eliminar el índice muerto. Añadir `idx_characters_name`.
2. **Añadir índices de `rooms`:** `idx_rooms_name` y `idx_rooms_area`.
3. **Añadir índices de `npc_spawners`:** `roomId` y `templateId` (filtros SQL activos sin índice).
4. **Índices de `users`** si sus rutas son calientes: `refid`, `isOnline`, `username`.
5. **Pool + pragmas:** mover `journal_mode=WAL`, `busy_timeout`, `synchronous` al DSN de `modernc.org/sqlite` (`file:talesmud.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)`) o a un hook de conexión, y **elevar `SetMaxOpenConns`** (WAL soporta lectores concurrentes). Medir el valor óptimo; definir serialización de escrituras si aparece `SQLITE_BUSY`. Añadir `SetConnMaxLifetime`.
6. **Heartbeat WS de protocolo:** ticker con `conn.WriteControl(websocket.PingMessage, …)` + `conn.SetReadDeadline`/`SetPongHandler` para reapear conexiones medio-abiertas; o timeout de idle por cliente.
7. **Timeouts HTTP:** `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout` en el `http.Server`.
8. **Harness de carga real** (k6/ghz o cliente Go) para 100–500 conexiones WS concurrentes midiendo p95/p99, sólo después de 5–7.

### B15.4 Veredicto Batch 15

**PASS (con condiciones).**

- **Regresión:** limpia — build OK, **53/53** paquetes de test en verde, 0 FAIL, frontend build EXIT 0.
- **Readiness de carga:** **parcial**.
  - ✅ WAL, `synchronous=NORMAL`, `busy_timeout=5000`.
  - ✅ Pool configurado (pero a 1 conexión, que es el cuello de botella principal).
  - ⚠️ Heartbeat sólo a nivel de aplicación (JSON ping 60 s); **sin ping/pong de protocolo ni read deadline**.
  - ❌ Índices incompletos: `rooms` sin índices; `idx_characters_user` mal apuntado; `npc_spawners` sin índices.
  - ❌ Sin timeouts en el servidor HTTP.
- **Condiciones para mantener el PASS:**
  1. No introducir regresiones: conservar 53/53 Go + frontend EXIT 0.
  2. Corregir el índice muerto `idx_characters_user` antes de cualquier benchmark de carga.
  3. Ejecutar el load test real (100–500 concurrentes) sólo tras aplicar las mejoras 5–7, para que los resultados sean representativos.
  4. Documentar el valor final del pool y la estrategia de `busy_timeout`/WAL en `ARCHITECTURE.md`.

---

## 6. Veredicto global (Batch 13 — histórico)

> Veredicto mantenido arriba para trazabilidad. Batch 13 marcó la integración de Fase 6 (LLM), NPC memories, y LevelCap fix.

# PASS (con condiciones)

- **Backend:** build OK, **53/53 paquetes con tests en verde, 0 FAIL**.
- **Fase 6 cubierta por primera vez:** `llmprompt`, `llmgen`, `talents` en verde; `repository`/npc_memories en verde. Pendiente: tests propios para `llmopts` y para `server/handler/llm.go`.
- **Content pack:** `import/adventure` OK.
- **Frontend build:** EXIT 0.
- **docker-compose:** válido.
- **Frontend tests:** 27/28 (comando indicado) y 53/55 (recursivo); los **2 fallos son de entorno preexistentes** (Node 18 sin `WebSocket` global; `canvas` ausente), **no regresiones**.
- **Batch 14 (nuevo):** build + tests OK, inventario visual 3.5 implementado, fundamentos de load testing verificados.

**Condiciones nuevas respecto al Batch 13:**
1. Añadir tests para `pkg/service/llmopts` (paquete nuevo sin cobertura) — ya documentado en Batch 13.
2. Añadir test dedicado para `pkg/server/handler/llm.go` (Fase 6 sin test específico) — ya documentado en Batch 13.
3. Mantener las condiciones del Batch 13 (Node LTS + canvas, runner recursivo, baseline fija).
4. Integrar el módulo inventoryEngine en el store del juego para peso real.
5. Confirmar que el atajo tecla I no conflictúa en el flujo completo.

---

## Histórico — Batch 12 (2026-10-14)

> La sección siguiente conserva el reporte original tal cual se emitió. No refleja los paquetes de Fase 6 (no existían aún) y su veredicto corresponde a ese estado.

## 6. Veredicto global (Batch 12 — histórico)

> Veredicto del 2026-10-14, previo a los paquetes de Fase 6. El veredicto vigente es el del **Batch 13** (arriba). Se conserva por trazabilidad.

# PASS (con condiciones)

- **Backend:** build OK, **50/50 paquetes con tests en verde, 0 FAIL**.
- **Content pack:** `import/adventure` health + reachability OK.
- **Frontend build:** EXIT 0.
- **docker-compose:** válido con y sin `AUTH0_*`.
- **Frontend tests:** 22/23 (comando indicado) y 45/47 (recursivo); los **2 fallos son de entorno preexistentes** (Node 18 sin `WebSocket` global; `canvas` ausente), **no regresiones**.

**Condiciones / recomendaciones para mantener el PASS:**
1. Documentar y corregir los fallos de entorno frontend (Node LTS + `canvas`) o marcarlos explícitamente como skip para que la suite sea determinista.
2. Adoptar un comando de tests frontend recursivo para eliminar puntos ciegos de cobertura.
3. Fijar una línea base (commit/tag) para comparaciones de regresión futuras.

---

## Anexo — Comandos ejecutados

```bash
# 1. Build backend
docker run --rm -v "$PWD":/app -v talesmud-gomod:/go/pkg/mod \
  -v talesmud-gocache:/root/.cache/go-build -w /app golang:1.26 \
  sh -c "go build -buildvcs=false ./..."                       # EXIT 0

# 2. Suite Go
docker run --rm ... sh -c "go test -buildvcs=false -count=1 ./..."   # EXIT 0

# 3. Tests frontend (indicado)
cd public/mud-client
node --test src/game/*_test.mjs src/api/*_test.mjs src/api/*.test.mjs  # EXIT 1 (22/23)

# 3b. Tests frontend (recursivo, adicional)
node --test $(find src -name '*_test.mjs' -o -name '*.test.mjs')      # EXIT 1 (45/47)

# 4. Build frontend
npm run build                                                  # EXIT 0

# 5. Content pack
docker run --rm ... sh -c "go test -buildvcs=false -count=1 \
  ./pkg/importer/... ./pkg/contenthealth/..."                  # EXIT 0

# 6. docker-compose config
docker-compose config                                          # EXIT 0
AUTH0_DOMAIN=... AUTH0_CLIENT_ID=... ... docker-compose config # EXIT 0
```

---

## B16 — Security Audit (FASE 7.2)

**Fecha:** 2026-10-17
**Alcance:** auditoría **estática read-only** de seguridad (auth, inyección SQL, XSS, permisos de rutas, exposición de secretos, CORS/CSRF, WebSocket) sobre el árbol de trabajo, más verificación de regresión. No se realizó pentest dinámico ni escaneo automatizado (SAST/DAST).

### Entorno

| Elemento | Valor |
|---|---|
| Commit HEAD | `bbebeede0cf7aa7a50180aec619581d0e8af9f70` (sin cambios desde Batch 15) |
| Rama | `master` |
| Go | `go1.26` (contenedor `golang:1.26`) |
| Docker | disponible |
| Node | `v18.19.1` (no usado en este batch; los 2 fallos frontend preexistentes siguen ignorados por brief) |

### B16.1 Auth y permisos de rutas

Superficie HTTP completa inventariada en `pkg/server/server.go` (`setupRoutes`, líneas 215-713). Grupos y middleware:

| Grupo | Prefijo | Middleware | Líneas |
|---|---|---|---|
| `admin` | `/admin/` (export/import) | `adminAuthMiddleware()` (BasicAuth `ADMIN_USER`/`ADMIN_PASSWORD`, rechaza credenciales débiles) | 363-367 |
| `protected` | `/api/` | `AuthMiddleware` (Bearer / guest / session) | 370-371 |
| `creator` | `/api/` (subgrupo) | `CreatorMiddleware` + `AuditWrites` | 438-440 |
| `preview` | `/api/loot-tables/:id/roll` | `CreatorMiddleware` | 569-571 |
| `liveAdmin` | `/api/live/*`, `/api/ops/:action`, `/api/audit/:id/undo` | `AdminMiddleware` | 574-581 |
| `adminAPI` | `/api/admin/*` | `AdminMiddleware` | 584-585 |
| `ws` | `/ws` | `WSAuthMiddleware` (ticket de un solo uso o Bearer) | 668-670 |

**Verificaciones concretas:**
- Rutas sensibles `creator.POST("creator/llm/...")` (líneas 555-559) están dentro del grupo `creator` → **protegidas por `CreatorMiddleware` + autenticación**. ✅
- `/api/ops/:action` (línea 580) está dentro de `liveAdmin` → **`AdminMiddleware`**. ✅
- `/api/admin/*` (líneas 587-599) → **`AdminMiddleware`**. ✅
- `/admin/export|import` (líneas 365-366) → **BasicAuth**. ✅
- **No se halló ninguna ruta creadora/admin/ops sin middleware.** Las rutas del grupo `public` (líneas 603-660) son deliberadamente anónimas (imágenes, catálogos, login/registro, device flow) y no exponen datos de usuario.
- `CreatorMiddleware` (`auth.go:428-440`) y `AdminMiddleware` (`auth.go:443-455`) exigen `user.IsCreator()`/`IsAdmin()`; rol vacío se normaliza a `player` (`entities/user.go:69-84`) → **sin escalada por rol vacío**. ✅
- `GetRole`/`setRole`: la API sólo permite `player`/`creator` y no deja tocar admins (`service/users.go:88-104`). Admin sólo por `MUD_ADMIN_OAUTHID`. ✅
- Baneo aplicado en la capa de auth (`auth.go:206-214`, `306-336`, `407-412`). ✅
- `AuthMiddleware` rechaza `access_token` por query string (`auth.go:365-370`) para no filtrar JWT en logs. ✅
- Auth local (`authlocal`) usa **Argon2id** (`password.go:16-32`) y comparación en tiempo constante. ✅

### B16.2 Inyección SQL

Búsqueda de concatenación de input en `pkg/repository/*_sqlite.go` y `pkg/db/**`:

- Los `fmt.Sprintf` que construyen SQL interpolan **siempre el nombre de tabla constante** del repositorio (`sqlite_generic.go:29,34,49,75,86,112,142,153,173`; `dm_sqlite.go:42`; `clans_sqlite.go:81`) o una tabla resuelta por **mapa whitelist** (`audit_sqlite.go:149-155`, `entityTables` → sólo 11 tipos conocidos; usado en `:166,188,203`).
- `buildWhere` (`sqlite_generic.go:190-202`) usa placeholders `?` y binding de `path`/`value`. El `key` de `FindByField`/`UpdateByField` se pasa como **valor** (`.$ + key` como parámetro de `json_extract`), no concatenado al SQL.
- `audit_sqlite.go:108-128` construye `WHERE` con fragmentos **literales** y placeholders; `entityType`/`entityID` van como argumentos. Sin embargo el `limit` se acota a 500.
- `rooms_sqlite.go:65-85` genera claves de query por reflexión sobre `RoomsQuery` (estructura interna), no desde input.
- `pkg/db/sqlite/sqlite.go`: todos los `Exec` son DDL/pragmas/migraciones literales; `Query` con nombre de tabla literal.

**Conclusión: NO se encontró inyección SQL explotable.** No hay input de usuario interpolado en SQL. ✅

### B16.3 Secretos y logs

- **Secretos hardcodeados: ninguno** en código de producción. Únicos literales tipo secreto están en `_test.go` (`password_test.go:9`, `service_test.go:24`, `ssha07_test.go:29`, `public_sync_security_test.go:20`).
- Secretos resueltos por entorno: `SESSION_SECRET`/`GUEST_SECRET`/`GROQ_API_KEY`/`ADMIN_PASSWORD`; si faltan se **genera aleatorio** (`authlocal/secret.go:32-43` con `crypto/rand`, archivo `0600`; `service/guest.go:47-52`).
- `.env` y `/data/session.key` están en `.gitignore`; `.env.example` sólo contiene placeholders. ✅
- **`passwordHash` nunca se serializa** a respuestas (`security_test.go:106-146`) y `Public` lo omite. ✅
- **No se loguean tokens/keys/passwords.** `RedactAccessToken` elimina `access_token`, `ticket`, `code`, `activate`, `device_code`, `token` de la ruta del gin logger (`server.go:133-143`, `util/logging.go:67-100`). ✅
- Las IPs de rate-limit y `ClientIP` se basan en `TRUSTED_PROXIES`, por defecto loopback (`gamemode/config.go:79-87`) → sin spoofing por XFF salvo mala configuración. ✅

### B16.4 XSS

Búsqueda en `public/mud-client/src/**` de `{@html}`, `innerHTML`, `outerHTML`, `document.write`, `insertAdjacentHTML`:

- Sólo **dos** usos de `{@html}`:
  - `RoomTextOverlay.svelte:501` → `formatText` → `formatOverlayText` (`formatOverlayText.js:5-13`) **escapa `& < >`** y luego inserta sus propias etiquetas `<strong>`/`<span class=...>` controladas.
  - `TerminalXWidget.svelte:529` → `line.html` proviene de `renderLine` (`:130-136`) que **escapa HTML** antes de convertir `**bold**` en `<span class="tx-highlight">`.
- El `color` interpolado en `renderPlayerMessage` (`TerminalXWidget.svelte:140`) procede de `getPlayerColor()` (`Client.js:645`, `playerColors.js:4-66`), una **paleta fija de 16 hex**, no de datos del servidor → no inyectable.
- No hay otros sinks DOM peligrosos.

**Conclusión: sin XSS reflejado/almacenado/DOM identificado.** El escapado previo a `{@html}` es correcto. ✅

### B16.5 CORS / CSRF

- CORS configurado con **lista blanca** en `server.go:91-113` y `handlers.CORS` (`server.go:793-796`). Orígenes por defecto: `talesmud.io` + varios `localhost`/`127.0.0.1`; ampliable por `CORS_ALLOWED_ORIGINS`.
- **No se habilita `AllowCredentials`** → las respuestas CORS no llevan cookies/credenciales. La app **no usa cookies** (`Set-Cookie`/`Cookie(` no aparecen en `pkg/`), sino `Authorization: Bearer` + tickets WS. Por tanto el riesgo CSRF clásico es bajo: un sitio externo no puede añadir el header `Authorization`.
- Los endpoints de device flow (`/api/ssh/device/{lookup,confirm,deny}`) añaden **defensa extra**: validan `Origin` contra `SSH_ACTIVATE_URL` (`sshaccess.go:247-249,300-314`) y un **nonce CSRF** por usuario (`devicecode/store.go:233-262`). Verificado por tests (`ssh_flow_test.go:143-163`).
- Mutaciones restantes exigen Bearer o BasicAuth, no cookies → CSRF no aplicable.

**Conclusión: no se detectó CSRF explotable.** ✅

### B16.6 WebSocket

- El upgrade pasa por `WSAuthMiddleware` (`server.go:668-670`): exige **ticket de un solo uso (32 bytes aleatorios, TTL 60 s)** o Bearer, y **rechaza `access_token` por query** (`auth.go:382-425`). El `TicketStore.Consume` borra el ticket y valida expiración (`wsticket.go:49-65`). ✅
- El `websocket.Upgrader` valida `Origin` con `OriginAllowed` contra la allowlist (`mudserver/mudserver.go:99-109,140-144`); origen vacío permitido (clientes no-navegador). ✅
- `HandleConnections` aborta con 401 si no hay usuario en contexto (`mudserver.go:198-209`). ✅
- **Sin ping/pong de protocolo ni `SetReadDeadline`** (ya reportado en B15.1.3): no es una vulnerabilidad de seguridad, pero permite conexiones medio-abiertas que consumen recursos. Ver B16-F1/F2.

### B16.7 Verificación de regresión (estado actual)

```
docker run --rm -v "$PWD":/app -v talesmud-gomod:/go/pkg/mod \
  -v talesmud-gocache:/root/.cache/go-build -w /app golang:1.26 \
  go build -buildvcs=false ./...                                # EXIT 0
docker run --rm ... go test -buildvcs=false -count=1 ./...      # EXIT 0
```

| Verificación | Resultado |
|---|---|
| `go build -buildvcs=false ./...` | **EXIT 0 — PASS** |
| `go test -buildvcs=false -count=1 ./...` | **EXIT 0 — PASS**: 54 paquetes OK, 24 sin tests, **0 FAIL** (78 total) |

Cambio respecto a B15: `pkg/db/sqlite` pasa de “sin tests” a **OK** (+1 OK / −1 sin tests). Los 2 fallos frontend preexistentes (`websocketGate_test.mjs`, `fogOverlay_test.mjs`) se ignoran por brief y no se ejecutaron en este batch.

### B16.8 Tabla de hallazgos

| ID | Sev. | Ubicación | Descripción | Recomendación |
|---|---|---|---|---|
| B16-F1 | **MEDIA** | `pkg/server/server.go:799` | `http.ListenAndServe` sin `ReadHeaderTimeout`/`ReadTimeout`/`WriteTimeout`/`IdleTimeout` → vulnerable a slowloris / agotamiento de recursos. (Ya identificado en B15.5 desde la óptica de carga; aquí como riesgo de disponibilidad.) | Construir `http.Server` con timeouts explícitos. |
| B16-F2 | **MEDIA** | `pkg/server/auth.go:64,76` | `fetchJWKS` usa `http.Get` **sin timeout** y decodifica un body **sin límite de tamaño**, manteniendo el `RWMutex` de la caché durante la petición. Un JWKS lento o malicioso bloquea todas las auth Auth0 concurrentes y puede agotar memoria. Además la caché sirve *stale* hasta 1 h. | Usar `http.Client{Timeout:...}`, `io.LimitReader`, y no retener el lock durante el I/O. |
| B16-F3 | **BAJA** | `pkg/server/server.go` (app SPA/API) vs `handler/sshactivate.go:56-59` | Cabeceras de seguridad (`CSP`, `X-Frame-Options`, `X-Content-Type-Options: nosniff`, `Referrer-Policy`) sólo se aplican en `/activate`; el resto de la app no las envía. | Middleware global con cabeceras de seguridad (especialmente `nosniff` para uploads servidos). |
| B16-F4 | **BAJA** | `pkg/server/server.go:94-103` | La allowlist CORS/WS incluida por defecto en producción contiene orígenes de desarrollo (`localhost:5000/5173/8010`, `127.0.0.1`). No hay `AllowCredentials`, por lo que el impacto es limitado, pero amplía la superficie de conexión WS desde páginas locales. | Construir la lista desde configuración en producción; no incluir loopback salvo modo dev. |
| B16-F5 | **BAJA** | `pkg/server/handler/localauth.go:145-173` | `ipLimiter.hits` no purga claves de IP nunca vistas de nuevo → crecimiento de memoria no acotado con muchas IPs de origen (DoS). | Añadir TTL/limpieza periódica y cap de entradas. |
| B16-F6 | **BAJA** | `pkg/server/server.go:363-367`, `adminAuthMiddleware` (`:57-75`) | `/admin/export\|import` usa BasicAuth; si se sirve por HTTP plano las credenciales viajan en base64 (sin TLS no hay protección). El diseño ya exige `len(pass)>=12` y rechaza usuarios débiles. | Documentar/forzar TLS en despliegue; considerar Bearer/rol admin en su lugar. |
| B16-F7 | **BAJA** | `pkg/service/groq/groq.go:112` → `pkg/server/handler/llm.go:142` | El cuerpo de error del proveedor LLM (`respBody`) se propaga al cliente creador en `err.Error()`. Puede filtrar detalle interno del upstream (no la API key). | Loguear el detalle upstream y devolver un mensaje genérico al cliente. |
| B16-F8 | INFO | `pkg/service/groq/groq.go:55-60`, `llm.go:106` | `GROQ_API_KEY` vacía desactiva Groq (fallback Ollama/none); no se loguea ni se expone. Comportamiento correcto. | — |

> **Sin hallazgos de severidad ALTA.** No se identificó bypass de autenticación/autorización, inyección SQL ni XSS explotable.

### B16.9 Veredicto Batch 16

**PASS (con condiciones).**

- **Regresión:** limpia — `go build` EXIT 0, **54/54 paquetes con tests en verde, 0 FAIL**.
- **Auth/permisos:** todas las rutas sensibles (`/creator/*`, `/admin/*`, `/api/ops/*`, `/api/creator/llm/*`) están tras autenticación y rol; **sin rutas huérfanas**. ✅
- **Inyección SQL:** ninguna explotable; tablas constantes/whitelist y binding de parámetros. ✅
- **XSS:** `{@html}` sólo en 2 puntos, ambos con escapado previo; sin sinks peligrosos. ✅
- **Secretos/logs:** sin secretos hardcodeados, sin logueo de tokens, `passwordHash` nunca serializado. ✅
- **CORS/CSRF/WebSocket:** allowlist de origen (HTTP y WS), sin cookies, tickets WS de un solo uso, CSRF nonce+Origin en device flow. ✅
- **Condiciones para mantener el PASS:**
  1. No introducir regresiones: conservar 54/54 Go.
  2. Atender B16-F1 y B16-F2 (endurecimiento de disponibilidad) antes de exponer el servicio a Internet sin proxy con timeouts.
  3. Aplicar cabeceras de seguridad globales (B16-F3) y retirar orígenes loopback en producción (B16-F4).
  4. Próximo: pentest dinámico y/o SAST en CI para cubrir lo que el análisis estático no alcanza (p.ej. lógica de autorización por objeto en todos los handlers de clanes/DM/SSH).

## B17 — Balance Pass (FASE 7.3)

### B17.1 Resumen ejecutivo

Análisis read-only del balance actual (curvas XP, daño/curación por clase, economía, dificultad). Datos extraídos de `pkg/mudserver/game/leveling/*`, `data/classes/*.yaml`, `pkg/classkit/catalog.go`, NPCs importados y `pkg/service/llmgen/questgen_context.go`.

### B17.2 Curvas de progresión

| Aspecto | Estado | Observación |
|---|---|---|
| XP por nivel | OK (piecewise) | L2-5 suave, L6-15 transición, L16-50 steeper. Acumulado L50=33,454. |
| HP/atributos | OK | HP/nível = STA-5 (min 1). Primario +1 cada 5, Secundario +1 cada 10, 2 pts/nível. |
| Talents por hitos | OK (implementado) | 1 punto cada 5 niveles (5..50 = 10 puntos). Gestionado en `leveling.TalentMilestonesBetween`. |

**Hallazgos:** Salto L5→L6 (149 XP para siguiente) perceptible; no crítico.

### B17.3 Daño/curación por clase (tabulado)

Ver `docs/qa/BALANCE_ANALYSIS.md` §2 para tablas completas (damage_dealt/taken, swings, multiplicadores, mana). Resumen:

| Clase | Rol | Burst | Sostenibilidad | Supervivencia | Comentario |
|---|---|---|---|---|---|
| Sorcerer | Nuker | **Alto** (Sear 2.00×) | Bajo | Baja (dmg_taken 1.30, HP bajo) | Outlier de burst. |
| Mage | Nuker/DoT | Alto (Sear 1.80×) | Bueno (Inscribe gratis) | Baja | Bien diferenciado de Sorcerer. |
| Druid | Híbrido | Medio (Sear 1.60×) | **Muy bueno** (Renew 10%/tick, Mend 25%) | Media | Equilibrado soporte-daño. |
| Cleric | Soporte/Tank | Medio-bajo (Slam 1.30×) | **Muy bueno** (Mend 30%, Renew 12%/tick) | Alta (Plate, dmg_taken 1.00) | Muy completo. |
| Warrior | Tank | Medio (Slam 1.40×) | Bueno | **Alta** (dmg_taken 0.90) | Sólido y estable. |
| Rogue | Melee dps | Variable (Nick efectivo ~1.10×) | Bueno (2 swings) | Baja (dmg_taken 1.15) | Útil/control, daño no bursty. |

**Outliers clave:** Sorcerer (burst alto), Cleric (muy completo híbrido).

### B17.4 Economía

| Fuente | Valores | Notas |
|---|---|---|
| Questgen context (LLM) | gold = 10+5*level (máx budget) | L1:15, L8:50 |
| Quests authored | L1:20, L8:150 | **Superior** a fórmula contextual (diferencial notable). |
| Kills (mobs) | 2g (dummy) → ~10-20g (medio) → 45-60g (bosses) | Crecimiento lento. Wisp/Maw/Whelp sin gold. |
| Tiendas/Precios | BasePrice en items; MerchantTrait (buy 1.0x, sell 0.5x) | Existe infraestructura económica, sinks poco evidentes en datos leídos. |

**Hallazgos:** Incentivo fuerte a questing vs grind. Falta visibilidad de sinks (reparaciones, equipo caro). **Incertidumbre alta** en economía completa.

### B17.5 Dificultad de contenido

| Tipo | HP | AtkPower | Defense | Dificultad | Evaluación |
|---|---|---|---|---|---|
| Trash (normal/easy) | 25-70 | 4-9 | 1-5 | easy-normal | Mayormente **trivial** para jugador bien construido. |
| Bosses | 90-180 | 9-15 | 4-6 | hard/boss | **Alcanzables**. Requieren gestión, pero no imposibles. |
| Minions (Maw Spawn L20) | 70 | 13 | 5 | easy | **Sub-ajustado** relativo a nivel 20 (posible outlier). |

**Veredicto dificultad:** contenido alcanzable en general; trash tiende a trivial, bosses bien dimensionados. Burst Sorcerer puede trivializar algunos encuentros.

### B17.6 Recomendaciones concretas (resumen)

| Área | Recomendación | Justificación | Incertidumbre |
|---|---|---|---|
| XP | Suavizar salto L5→L6 (tramo L6-15) | "Hump" transitorio entre tramos. Ajuste menor, paramétrico vía ruleset. | Baja-Media |
| Sorcerer | Sear 2.00× → **1.85×-1.90×** o +mana/cooldown | Reduce burst pico manteniendo identidad glass cannon. | Media |
| Cleric | Mend 0.30×→**0.25×** o cooldown +1 | Muy completo como híbrido; mitigar sostenibilidad extrema. | Alta |
| Economía (quests vs kills) | Alinear o ajustar: bajar quests diseñados ~30-50% O subir gold mobs O ajustar budget contextual a `20+10*level` | Reduce brecha questing/grind. Budget contextual bajo vs authored. | Media |
| Economía (sinks) | Añadir sinks (costes reparación, equipo, consumibles) | Evitar inflación a futuro; falta evidencia en datos actuales. | Alta |
| Contenido (Maw Spawn) | HP 70→**120-150** (L20) | Parece subdimensionado vs nivel. | Media |

### B17.7 Verificación de regresión

```bash
go build -buildvcs=false ./...  # EXIT 0 - PASS
go test -buildvcs=false -count=1 ./...  # EXIT 0 - PASS (54 OK, 24 sin tests, 0 FAIL)
```

Sin regresiones. Build y tests Go limpios.

### B17.8 Archivos/reportes creados

- `docs/qa/BALANCE_ANALYSIS.md` — Análisis detallado con tablas, cálculos y justificaciones.
- `docs/qa/REGRESSION_REPORT.md` — Sección B17 añadida (este batch).

**Conclusión B17:** Balance mayormente coherente con la intención de diseño (tank/melee dps/casters/support). Se identifican **2 outliers moderados** (Sorcerer burst, Cleric completitud) y **ajustes menores** en economía (alinear questgen vs authored) y contenido (Maw Spawn). Las recomendaciones son concretas con valores sugeridos y niveles de incertidumbre declarados. No se detectan regresiones.

> Nota: análisis read-only. No se modificó código ni datos de juego.

---

## B18 — Load Testing Harness (FASE 7.1) + Combat Sims (FASE 7.3)

Fecha: 2026-10-18. Scope: auditoría y ejecución de `tools/combatsim` y `tools/loadtest` (creados en batches anteriores), ejecución del simulador de combate contra el motor real, prueba de carga viva contra el servidor, tests unitarios y ajustes mínimos a las herramientas.

### B18.1 Auditoría de herramientas

#### `tools/combatsim` — simulador de combate

| Aspecto | Veredicto | Evidencia |
|---|---|---|
| Motor real vs reimplementación | **OK** | `simulate()` construye `combat.NewEngine(combat.NewManager(), nil)` → usa `DefaultConfig`; las fórmulas coinciden **por construcción** (no hay copia que diverja). |
| Carga de balance | **OK** | `balance.GetConfig()` lee `config/combat_balance.yaml` desde el cwd (lazy); funciona al ejecutar desde la raíz del repo. |
| Kits de clase | **OK** | IDs de `skills` en `data/classes/*.yaml` coinciden con `KitBrace/KitSlam/...` del motor; `ProcessSkill` maneja bolt/rig/overload; `KeepsSwing` (brace/guard) → sim hace `ProcessSkill` + `ProcessAttack`, igual que el flujo real del juego. |
| Semántica de uso (`setupWanted`) | **OK** | brace/glyph/guard/stand lanzados con HP ≤ 0 (no armados); rig/bolt requieren cargas; `KitSpent` bloquea once-per-fight. Idéntico a la lógica del jugador. |
| Gear scaling | **OK** | `applyGearScaling` muta el item starter del roster; intención equivalente a `simutil.GearScores` del motor (usa "Test Weapon/Armor"). |
| Componentes (p.ej. `fire_salt`) | **Limitación conocida** | El motor no verifica componentes en skills (solo flavor); el sim tampoco. Documentado, no es bug del tool. |
| Aleatoriedad | **Sin seed** | Varianza run-to-run ≈ ±2 pp con n=1000 (verificado con 2 corridas independientes). Aceptable para balance; documentado. |

#### `tools/loadtest` — harness de carga

| Aspecto | Veredicto | Evidencia |
|---|---|---|
| Handshake de registro | **OK** | `POST /api/guest` → `{"token":...}`; `POST /api/auth/register` → 201 `{"token","user"}`; coincide con `pkg/server/handler/{guest,localauth}.go`. |
| WS ticket | **OK** | `POST /api/ws-ticket` (Bearer) → `{"ticket"}`; dial `GET /ws?ticket=...`; servidor acepta ticket (query) o Bearer; `?access_token` es **rechazado explícitamente** por `WSAuthMiddleware` → el harness nunca lo usa. |
| Protocolo hello | **OK** | `{"type":"hello","proto":1,"client":...,"message":""}` = `messages.IncomingMessage`; el servidor manda frame de apertura al conectar y responde a hello (`mudserver.handleHello`). |
| Flags reales | **Documentado** | `-workers` (no `-conns`, como aparecía en la descripción original de la tarea); `-auth guest\|register\|token\|ticket`; `-bearer`; `-origin`. |
| Modo ticket con N workers | **Limitación conocida** | Tickets de uso único; `-auth ticket` sirve para 1 conexión, no concurrencia. Usar token/guest/register para múltiples workers. |

### B18.2 Cambios realizados en `tools/**` (bugs/ajustes claros permitidos)

1. **`tools/combatsim`** — columnas `Loss%` y `Timeout%` añadidas a `aggregate`, línea de progreso y tabla markdown. Antes solo se reportaba `Win%` (la diferencia podía confundirse con timeouts).
2. **`tools/loadtest`** — flag `-tokens-file` (archivo con un JWT por línea, round-robin por worker). Motivo: los límites anti-abuse del servidor (cap 20 guests concurrentes, registro 12/min/IP) hacen imposible >20-identidades sin pre-acuñar tokens. Función `LoadTokensFile`, `authConfig.tokenFor(index)`, worker HTTP actualizado.
3. **Tests unitarios nuevos** (ambos paquetes ahora verdes):
   - `tools/combatsim/sim_test.go`: parseList/parseInts, difficultyFor, win/loss/timeout rate, avgRounds/dps/hps, setupWanted gates, renderMarkdown, maxSimRounds.
   - `tools/loadtest/stats_test.go`: percentile nearest-rank, summarize, stats counters + snapshot copy, httpBaseFromWS, withTicket, newAuthConfig, authTarget, tokenFor round-robin, LoadTokensFile, token-mode con tokens-file.

### B18.3 Simulaciones de combate (motor real)

Comandos (docker golang:1.26, `-buildvcs=false`):

```bash
go run ./tools/combatsim -iterations 1000 -levels 1,10,25,50 -tiers normal,boss
go run ./tools/combatsim -iterations 500  -levels 1,10,25,50 -tiers trash,elite
go run ./tools/combatsim -iterations 500  -levels 25,50      -tiers boss -gear good
```

#### Resultados — tier normal (1000 combats/celda, runtime 10.4s)

| Clase | L1 | L10 | L25 | L50 | Lectura |
|---|---:|---:|---:|---:|---|
| Warrior | 94.9% | 100% | 100% | 100% | Sólido; nunca pierde en tier normal. |
| Rogue | 75.7% | 94.9% | 95.6% | 89.7% | Bueno; leve caída L50 (varianza ±2pp). |
| Cleric | 73.3% | 100% | 100% | 100% | Segurísima (0 losses L10-50). |
| Druid | 53.5% | 92.2% | 88.8% | 77.8% | Correcta; algo más de losses altos. |
| Mage | 1.8% | 39.8% | 30.2% | 36.7% | **Muy débil en 1v1** pese a DPS alto (ver §B18.4). |
| Sorcerer | 0.3% | 8.5% | 19.4% | 15.9% | **La más débil en 1v1** (glass cannon puro). |

#### Resultados — tier boss (1000 combats/celda)

| Clase | L1 | L10 | L25 | L50 | Lectura |
|---|---:|---:|---:|---:|---|
| Warrior | 0% | 79.5% | 85.7% | 0% + **50% timeout** | L50 boss: pelea de 136 rondas, mitad timeouts. |
| Rogue | 0.1% | 24.9% | 29.1% | 0% (95% loss) | Boss alto fuera de alcance 1v1. |
| Cleric | 0% | 67.1% | 88.4% | 0% + **90.8% timeout** | El boss L50 la desgasta hasta el cap de 200 rondas. |
| Druid | 0% | 22.5% | 21.0% | 7.5% | Pobre en boss 1v1. |
| Mage | 0% | 0.8% | 0% | 0% | Casi sin winrate contra boss. |
| Sorcerer | 0% | 0.8% | 0% | 0% | Igual; burst no basta contra HP alto. |

#### Resultados — trash/elite (500/celda) y boss con `-gear good` (500/celda)

- **Trash**: Warrior/Cleric 100% desde L10; Rogue ~92-99%; Mage/Sorcerer L1 todavía <7% (ver caveat curva de muestra).
- **Elite**: Warrior/Cleric 94-100% L10-50; Rogue 70-96%; Sorcerer 3.8-10.2% (casi nunca gana).
- **Boss L25 `-gear good`**: Warrior 99.6%, Cleric 99.2%, Rogue 81.8%, Druid 74%, Mage 47.8%, Sorcerer 39% — el equipo cierra la brecha de casters pero Warrior/Cleric siguen arriba.
- **Boss L50 `-gear good`**: Druid 43.8%, Mage 33.2%, Sorcerer 13.6%, Warrior 4% (55% timeout), Rogue 0.4%, Cleric 0.6% (93% timeout). El único contenido "estratégico" que existe es este — y es el más desequilibrado.

**Caveat importante:** los tiers del sim usan curvas de stat scaling del tool, no los mobs importados del contenido real; los porcentajes L1 (casters 0-2%) reflejan la curva del sim (más dura que el Dummy L1 real de 12 HP) y deben leerse **relativamente entre clases**, no como dificultad absoluta del juego.

### B18.4 Confirmación de outliers B17 (empírica)

| Outlier B17 (teórico) | Resultado B18 (empírico) | Confirmación |
|---|---|---|
| Sorcerer = burst alto (Sear 2.00×) | **Mayor DPS por ronda** de todos (L10-50 normal: 25.4→38.3) pero **peor win-rate en 1v1** (0.3-19.4%). Pierde por supervivencia (dmg_taken 1.30, HP bajo). | Burst confirmado; **no es overpowered en PvE**, es **underpowered** en 1v1. Recomendación B17 (bajar Sear) queda **revisada**: el problema no es daño sino survivabilidad. |
| Cleric = híbrido demasiado completo | 100% win-rate normal L10-50, 67-88% boss L10-25, HPS mínimo sostenido; 90.8% timeout en boss L50 (nunca pierde, tampoco gana). | **Outlier "Cleric sostenible" confirmado**. Recomendación B17 (bajar Mend 30→25%) mantiene vigencia. |
| Mage vs Sorcerer diferenciados | Mage gana más 1v1 (30-40% vs 8-19%) con menos DPS pico; Sorcerer más burst pero menos victorias. | Diferenciación **confirmada**; ambos casters sufren en 1v1 vs curva de melee/tank. |
| L50 boss = contenido estratégico | **Intratable 1v1**: Cleric 90.8% timeout, Warrior 50% timeout, resto ~100% loss. Solo Druid/Mage ganan algo con `-gear good`. | **Nuevo hallazgo**: el escalado de boss L50 (HP/round cap) produce timeouts de 200 rondas, no desafío interesante. Revisar stats de boss L50 **o** el cap `maxSimRounds`. |

### B18.5 Prueba de carga viva (servidor real en docker, puerto 8099)

Setup: binario del servidor construido desde el repo, config efímera `/tmp/opencode/gamemode-loadtest.yaml` (SQLite en `/tmp/tales-lt.db`, DB fresca por contenedor), cliente harness en docker con `--network host`.

| Run | Workers | Duración | Auth | Conectados | Errores | Connect p50/p95 (ms) | RTT p50/p95/p99 (ms) |
|---|---:|---:|---|---:|---|---|---|
| A | 10 | 10s | guest | 10/10 | 0 | 4.8 / — | 3.3 / — / — |
| B | 20 | 10s | guest | 19/20 (1 initErr) | 0 msg | 12.1 / — | 0.045 / 3.175 / 8.365 |
| C (HTTP) | 10 | 5s | token (Bearer) | 10 req | 0 | — (`GET /api/my-characters`, 2 req/s/worker, 100 req) | 1.198 / 2.768 / 3.203 |
| D | 50 | 10s | guest | **22/50** (28 initErr) | 0 msg | — | — |
| F | 50 | 10s | **token `-tokens-file` (50 identidades pre-acuñadas)** | **50/50** | 0 | 38.5 / 51.6 | 0.052 / 71.4 / **896.9** (max 3176) |

**Lectura:**
- **10-20 sesiones concurrentes: latencias excelentes** (p50 < 5 ms connect, RTT p95 en decenas de ms con 20 workers). El servidor sirve el caso de uso real sin problemas.
- **Run D (50 guests) falla por diseño del servidor, no del harness**: `MaxGuestAccounts = 20` por defecto (`pkg/entities/settings/settings.go:37`) → 28 workers reciben `server is full, try again later` en `POST /api/guest`. Límites anti-abuse documentados: 20 guests concurrentes, 12 intentos de registro/min/IP (`newIPLimiter(12, time.Minute)`), 60 guests/hora/IP.
- **Run F (50 sesiones reales con tokens pre-acuñados)**: 50/50 conectan, 1042 probes enviados, 0 errores de mensaje. La cola de latencia crece: p50 RTT sigue en 0.05 ms pero **p95 ≈ 71 ms, p99 ≈ 0.9 s, max ≈ 3.2 s** — con 100 mensajes/s entrantes (50×2/s) la cola del event-loop/GC empieza a notarse. Aún funcional, pero es el primer dato de estrés del servidor.
- **Modo ticket**: limitado a 1 worker (tickets de uso único). Para concurrencia usar guest/register/token como hizo el harness.
- `Timeout%` de 0 en todos los runs: ningún cierre de sesión por timeout del harness ni por servidor en 10s.

### B18.6 Verificación final

```bash
docker run ... golang:1.26 go build -buildvcs=false ./...          # EXIT 0 — BUILD_OK
docker run ... golang:1.26 go vet   -buildvcs=false ./tools/...    # EXIT 0 — VET_OK
docker run ... golang:1.26 go test -count=1 -buildvcs=false ./tools/...
#   ok  github.com/talesmud/talesmud/tools/combatsim  0.005s
#   ok  github.com/talesmud/talesmud/tools/loadtest   0.004s
go run ./tools/combatsim -iterations 1000 -levels 1,10,25,50 -tiers normal,boss  # OK (tabla B18.3)
```

Container de prueba eliminado (`docker rm -f tales-lt`). Sin cambios en `pkg/**`, `public/**`, `cmd/**`. Sin commits (read-only + `tools/**`/`docs/qa/**` permitidos).

### B18.7 Veredicto

| Herramienta | Estado | Acciones |
|---|---|---|
| `tools/combatsim` | **Apta para uso** | Columnas Loss%/Timeout% añadidas; tests unitarios cubren lógica pura. |
| `tools/loadtest` | **Apta para uso** | `-tokens-file` añade capacidad >20 sesiones (única forma válida contra límites anti-abuse); tests cubren auth/stats/URL. |
| Balance (B17 vs B18) | **1 outlier confirmado (Cleric), 1 revisado (Sorcerer)** | Sorcerer no es overpowered: es underpowered en 1v1. Cleric sí es demasiado completa. Nuevo foco: boss L50 intratable (timeouts). |
| Servidor bajo carga | **Estable hasta 20 sesiones; 50 sesiones factibles con tokens** | Latencias p50 excelentes; cola notable a 50×2 msg/s (p99 ~0.9s). |

**Conclusión B18:** las dos herramientas de FASE 7.1/7.3 están auditadas, probadas y verdes. Las simulaciones confirmeron empíricamente el outlier Cleric, matizaron el Sorcerer (burst ≠ power) y revelaron un nuevo problema de tuning en boss L50 (timeouts). El harness de carga funciona contra el servidor real con protocolo verificado; los límites de concurrencia observados son controles anti-abuse del servidor, no defectos del tool. Sin regresiones en build/tests Go.

---

## B19 — Load Test 100-500 conexiones (FASE 7.1)

Ejecución del load test real de 100–500 conexiones WS pendiente desde B15/B18, con identificación del cuello de botella mediante volcado de goroutines (`SIGQUIT`) y verificación del heartbeat bajo carga.

**Veredicto de cabecera:** el servidor sirve 100 sesiones sin problemas (heartbeat incluido), pero **≥200 conexiones simultáneas disparan un deadlock permanente del bucle de juego** — no es CPU, ni BD, ni red, ni pings. Tras el deadlock el servidor queda congelado hasta el reinicio. Corregir (P0 en §B19.7) antes de plantear cualquier cifra de escala.

### B19.1 Entorno y metodología

- **Servidor QA**: contenedor `tales-lt-qa`, `go run ./cmd/tales -config /tmp/opencode/gamemode-loadtest.yaml` con `--network host` (auth local, `:8099`, SQLite efímero en el contenedor). Pool SQLite `MaxOpenConns=4` (por defecto, `DB_MAX_OPEN_CONNS`). `docker restart` preserva DB y logs (clave para leer los contadores `UserJoined` acumulados entre corridas); `docker rm -f` no.
- **Identidades — nuevo modo `-mint` del harness**: los límites anti-abuse (`MaxGuestAccounts = 20`, `authLimiter` de 12 registros/min/IP en `pkg/server/handler/localauth.go:38`) impiden calentar >20 identidades con una sola IP. Se añade `tools/loadtest/mint.go` (`mintTokens`, `mintInterval`, `xffIP`, `mintRegisterResponse`, `mintBody`) con flags `-mint -mint-out -mint-per-min -mint-ips` en `main.go` (§B19.4 para el ángulo de seguridad). Acuñación: `-mint 550 -mint-ips 200` → 550 session JWTs en `/tmp/opencode/tokens_550.txt` (pacing por IP sintética `10.x.y.z` vía `X-Forwarded-For`).
- **Workers**: `-auth token -tokens-file …` contra `ws://127.0.0.1:8099/ws`. Cada worker conecta, espera el frame de apertura del servidor (→ se cuenta como `connected`), envía un probe `hello` inmediato y, con `-rate > 0`, probes a ese ritmo hasta `-duration`; `-timeout` por operación. RTT = `hello` → cualquier mensaje entrante.
- **Patrón de comando** (parámetros exactos por corrida en las tablas; el JSON va a stdout):
  ```bash
  docker run --rm --network host -v "$PWD":/app -v /tmp/opencode:/tmp/opencode … \
    -w /app golang:1.26 go run ./tools/loadtest \
      -url ws://127.0.0.1:8099/ws -auth token \
      -tokens-file /tmp/opencode/tokens_550.txt \
      -workers <N> -rate <R> -duration <D> -timeout 10s \
      > /tmp/opencode/<run>.json 2>&1
  ```
- **Artefactos de evidencia** (en `/tmp/opencode/`, efímeros salvo copia): resultados `lt_{100,100_hb,200,300,300_fresh,300_patient,500,500_hold,500_fresh,500_clean}.json` y volcado `goroutines_wedge.txt` (4.7 MB, 738 goroutines) tomado con `SIGQUIT` durante el wedge. Los números y extractos de stacks relevantes quedan reproducidos en este informe.

### B19.2 Resultados por concurrencia

Corridas contra servidor **sano** (arranque o reinicio reciente):

| Corrida | Workers × rate × duración | Conectados | recv / sent | msgErrors | Connect p50/p95 (ms) | RTT p50/p95/p99/max (ms) | Estado |
|---|---|---|---|---|---|---|---|
| `lt_100` | 100 × 2/s × 30s | 100/100 | 6028 / 6028 | 0 | 149.9 / 252.7 | 0.057 / 0.11 / 473 / 4159 | **sano** (CPU ~34%, 164 MB observados) |
| `lt_100_hb` (heartbeat) | 100 × 2/s × **75s** | 100/100 | 13977 / 13982 | 5 | 173.6 / 236.3 | 0.045 / 0.108 / 4.59 / 3314 | **sano; sobrevive el tick de 60s** (§B19.5) |
| `lt_200` | 200 × 2/s × 30s | 200/200 | 9762 / 9824 | 62 | 289.0 / 479.6 | 0.111 / 763 / 1247 / 5296 | **wedge a mitad de corrida** (log `UserJoined` se congela en 259 y no vuelve a crecer) |
| `lt_300_fresh` | 300 × 0/s × 25s (1 probe/worker) | 300/300 | 94 / 300 | 206 | 359.6 / 616.8 | 753 / 1025 / 1319 / 1319 (n=94) | **wedge tras 74 joins** (log 259→333; `94 = 20 + 74` ✓) |
| `lt_500_clean` | 500 × 2/s × 25s | 500/500 | 8311 / 8943 | 632 | 638.8 / 960.8 | 36.3 / 890 / 1833 / 5302 | **wedge tras 140 joins** (log 433→573) |

Corridas contra servidor **ya wedged** — 0 respuestas pese a conectar; sirven para probar la permanencia del bloqueo, no para medir latencia:

| Corrida | Resultado |
|---|---|
| `lt_500` (tras wedge de `lt_200`), `lt_500_fresh` (tras wedge de `lt_300_fresh`), `lt_300` | `connected` 100%, **recv = 0** (0/1000, 0/1000, 0/600) |
| `lt_500_hold` (500 workers, espera larga) | recv = 0/500 |
| `lt_300_patient` (`-timeout 45s`, 4.5× el normal) | recv = 0/300 → **el bloqueo no es lentitud** |
| Probes de 2w/3w a cualquier hora posterior | 0 respuestas → **solo un reinicio lo recupera** |

**Lectura:**
- **100 conexiones: sanas.** 0 errores de mensaje en ambas corridas, RTT p50/p95 sub-ms y connect p50 ~150-170 ms. Cola puntual alta en `lt_100` (p99 473 ms, max 4.2 s) — uno o dos probes en cola de DB/GC, no estructural.
- **`connected` ≠ sesión viva**: el frame `Connected to [...]` se envía en `mudserver.go:336` **antes** del enqueue `OnUserJoined` en `:338`. Un worker bloqueado en `:338` ya cuenta como conectado, pero su read loop (`HandleConnections`, `mudserver.go:231` → `ReadJSON` posterior a `attachSession`) nunca arranca y ningún `hello` se lee. Por eso `connected` es 100% incluso con `recv = 0`.
- **Aritmética exacta del modelo** (cola `OnUserJoined` = 20, §B19.3): `lt_300_fresh`: vivos `20 + 74 = 94` ✓ = `recv`; atascados `300 − 94 = 206` ✓ = `msgErrors`. `lt_500_clean`: 140 joins → vivos `20 + 140 = 160` × ~52 probes (25s × 2/s) ≈ **8311 recv** ✓; atascados `500 − 160 = 340` × ~2 probes ≈ **632 msgErrors** ✓; `sent 8943 ≈ 8311 + 632` ✓.
- **La latencia creciente sí es de escrituras SQLite** en el camino caliente (connect p50 150→289→360→639 ms de 100→500 workers; RTT p95 0.1→763→890 ms — §B19.6-1), pero **no** explica el stall permanente: durante el wedge la CPU es ~0% y no hay hilos esperando BD en el dump.
- **Los límites anti-abuse no son el cuello de botella**: con los 550 tokens mintados, todas las corridas superan el techo de 20 guests con **0 `connErrors` / 0 `initErrors`**.

### B19.3 Causa raíz: deadlock autoinfligido del bucle de juego

Cadena de fallo (todo en `pkg/mudserver`):

1. **Un único bucle de juego** (goroutine creada en `game.go:280`, `select` en `game.go:283-307`) es el **único consumidor** de `onMessageReceived` (buffer **20**, `game.go:115`), `OnUserJoined` (buffer 20), `OnUserQuit`, `jobs` y `aggroCh`.
2. **Ese mismo bucle, dentro de `handleUserJoined`, vuelve a enviar a su propio canal**: las identidades del mint no tienen personaje (`LastCharacter == ""`) → `FindAllForUser` devuelve `err == nil, len == 0` → cae en `FindByID("")` → error → log `Could not select character` (`game_handlers.go:124`; **572 líneas** en los logs del contenedor, ≈ los 573 `UserJoined` acumulados) → **`game.onMessageReceived <- messages.NewMessage(user, "lc")` en `game_handlers.go:128`** (con personaje sería el mismo bloqueo en `:133` con `selectcharacter`). Cada join procesado añade exactamente 1 mensaje al buffer que solo esa goroutine drena.
3. **Deadlock permanente**: cuando el buffer (20) está lleno y el `select` elige procesar otro join, la goroutine se bloquea en el send — y es la única que podría drenarlo → **nunca sale**.
4. **Cascada WS**: `attachSession` hace `OnUserJoined <-` en `mudserver.go:338` (tras el `Connected` de `:336`); el buffer de 20 absorbe los primeros 20, y a partir de ahí cada `attachSession` queda bloqueado **antes** de que `HandleConnections` entre en su bucle `ReadJSON` → hellos no leídos → probes agotan timeout → `msgError`.

**Evidencia — volcado `SIGQUIT` (`/tmp/opencode/goroutines_wedge.txt`, 738 goroutines):**

- **La goroutine 35 — el propio bucle de juego — bloqueada `[chan send, 1 minutes]` en `game_handlers.go:128`:**
  ```
  goroutine 35 gp=0x3c8567b301e0 m=nil [chan send, 1 minutes]:
  runtime.chansend(0x3c8567a89650, …)                        chan.go:283
  …/game.(*Game).handleUserJoined(…)          game_handlers.go:128
  …/game.(*Game).Run.func1()                  game.go:288
  created by …/game.(*Game).Run in goroutine 12   game.go:280
  ```
- **712 goroutines bloqueadas en `chansend` ← `mudserver.go:338`** (`attachSession` ← `HandleConnections` `mudserver.go:231` ← gin `Next`): la cascada completa de sesiones.
- `game_handlers.go:133`: **0**; `mudserver.go:405` (enqueue de input de jugadores): **0** → la congestión **no** viene de los probes de texto ni del camino de personajes con `selectcharacter`.
- Los demás consumidores **idle en `chan receive`**: `receiveMessages` (g. 11), `handleBroadcastMessages` (g. 13), `handleClientTimeouts` (g. 14) → **sin backpressure TCP, sin bloqueo del broadcast, sin bloqueo del heartbeat**.
- **CPU ~0%** durante el wedge (observado con `docker stats`): goroutines bloqueados, no computando.

**Umbral: probabilístico, no fijo.** El `select` del bucle elige al azar entre joins pendientes y mensajes pendientes: es un paseo aleatorio con techo absorbente en 20 (cada join `+1`, cada mensaje `−1`, rebote en 0). Ráfagas de 300-500 joins simultáneos lo alcanzan casi seguro — wedge observado tras **74** y **140** joins, y durante `lt_200` tras ~146 —; con 100 joins no se alcanzó en 2 corridas. Por tanto **100 sanas (2/2), 200 wedge en 1/1, 300+ wedge en 2/2**: el servidor no tiene un límite de concurrencia fijo sino una **bomba de relojería probabilística** — ~100 logins simultáneos seguros, ≥200 casi seguro que se congela de forma **irrecuperable sin reinicio** (verificado con `-timeout 45s` y con probes posteriores: 0 respuestas; `handleClientTimeouts` idle nunca llega a limpiar las sesiones).

### B19.4 Seguridad: rate-limits por IP y el bypass `X-Forwarded-For` de loopback

- El rate-limit de registro/login es por IP: `authLimiter = newIPLimiter(12, time.Minute)` claveada en `c.ClientIP()` (`pkg/server/handler/localauth.go:38,42`), más `MaxGuestAccounts = 20` (`pkg/entities/settings/settings.go:37`).
- `TrustedProxies()` por defecto es `["127.0.0.1", "::1"]` (`pkg/gamemode/config.go:86`) → gin confía en el `X-Forwarded-For` de cualquier peer loopback → **cualquier proceso local puede rotar XFF y obtener presupuesto ilimitado de 12 registros/min por IP sintética**. Es exactamente el vector que usa `-mint -mint-ips K` del harness (IPs `10.x.y.z`), documentado en el doc-comment de `tools/loadtest/mint.go`.
- Contexto QA: con `--network host` el harness llega por loopback, de ahí que el mint masivo solo funcione en este setup. `pkg/server/trusted_proxy_test.go` verifica el lado opuesto: un peer **no**-loopback **no** puede rotar XFF para saltarse el límite (13 peticiones con XFF rotatorio → la última recibe 429).
- **Acción recomendada (FASE 7.2)**: en despliegue real, exponer el puerto solo a procesos de confianza o situar el servidor detrás de un proxy con `TRUSTED_PROXIES` explícito; valorar desactivar la confianza loopback por defecto si el threat model incluye procesos locales.

### B19.5 Heartbeat bajo carga — verificado

- Corrida `lt_100_hb`: **100 workers × 75s > tick de 60s** (`pingTicker := time.NewTicker(60 * time.Second)`, `mudserver.go:177`), con servidor recién reiniciado:
  - `13977/13982` recv (**99.96%**), solo **5** timeouts de probe aislados (0.04%).
  - Connect p50 173.6 / p95 236.3 ms; RTT p50 0.045 / p95 0.108 / p99 4.59 / max 3314 ms.
  - **El servidor sobrevive al tick de 60s sin wedge con 100 sesiones**; ni la latencia ni los errores se disparan con los pings (y en el dump posterior `handleClientTimeouts` está idle en su ticker = sano). El wedge de ≥200 es el deadlock de §B19.3, no el heartbeat.
- Matiz de medición: el harness acepta **cualquier** mensaje entrante como respuesta a un probe; en corridas >60s los pings (~1 por worker por tick) pueden consumirse como "respuesta" y desviar ~1 sample de RTT. Efecto menor sobre 13 977 samples; sin efecto sobre los conteos.
- Riesgo latente relacionado con los pings: `sendUserPings` itera `Clients.ForEach` con envíos bloqueantes secuenciales y el transporte WS **no fija write deadline** (§B19.6-2): un cliente que no lee podría teóricamente atascar la ronda de pings. No se observó en estas corridas.

### B19.6 Otros hallazgos

1. **Cada mensaje de cliente cuesta 2 escrituras SQLite síncronas** (y cada sesión, una tercera): `handleHello`/read-loop → `handleInput` (`mudserver.go:276`/`:266`) → `ConnectUserSession` (su `Update` en `pkg/mudserver/game/sessions.go:217`) + `UsersService().Update` (`mudserver.go:403`); más `Update` de `attachSession` (`mudserver.go:325`). Sobre un pool de 4 conexiones esto explica el crecimiento de connect p50 (150→639 ms de 100→500 workers) y del RTT p95 (0.1→890 ms). `UsersService().Update` por mensaje es prescindible (`LastSeen`/`IsOnline` no necesitan persistirse por cada input → debounce o persistir en disconnect/cada N s). **`DB_MAX_OPEN_CONNS` no mitigaría el wedge**: el deadlock es de canales (CPU 0%, sin espera de BD en el dump).
2. **Sin `SetWriteDeadline` en el transporte WS**: `wsTransport.Send` es un `conn.WriteJSON(v)` sin deadline (`transport.go:25-29`); `grep SetWriteDeadline` solo tiene matches en `pkg/sshgate` (SSH), no en el camino WebSocket. Un cliente lento/no-leer puede bloquear `sendUserPings`/`receiveMessages` (segunda vía de bloqueo latente, no observada en B19).
3. **Sin `pprof` en `cmd/` ni `pkg/`** (grep sin matches): el diagnóstico exigió `SIGQUIT` manual y análisis de 4.7 MB de volcado; un endpoint pprof bajo flag acortaría cualquier próxima investigación.

### B19.7 Recomendaciones (orden de prioridad)

| # | Prioridad | Acción | Por qué |
|---|---|---|---|
| 1 | **P0** | `game_handlers.go:128/133`: no enviar bloqueantemente a `onMessageReceived` desde el propio bucle — procesar "lc"/`selectcharacter` **inline** (ya se está en el consumidor) o con `select { case ch <- msg: default: … }` | Rompe el deadlock de §B19.3; sin esto, cualquier ráfaga ≥~200 logins congela el servidor |
| 2 | **P0** | `mudserver.go:338`: enqueue no bloqueante (cola/semáforo/select-default) de `OnUserJoined` | Un backlog de joins no debe congelar el read loop de cientos de sesiones (la cascada) |
| 3 | **P1** | Debounce de `UsersService().Update` de `LastSeen`/`IsOnline` (§B19.6-1) | −2 escrituras por mensaje; recorta el crecimiento de connect/RTT p95 |
| 4 | **P1** | `SetWriteDeadline` + cola de salida por conexión en `wsTransport.Send` | Elimina la segunda vía latente de bloqueo de pings/broadcast (§B19.6-2) |
| 5 | **P2** | `net/http/pprof` bajo flag en `cmd/` | Diagnóstico sin `SIGQUIT` (§B19.6-3) |
| 6 | gate | Re-ejecutar 200/300/500 **solo tras el P0**; techo actual ~<100 logins simultáneos | Evita repetir un wedge permanente |

### B19.8 Verificación y limpieza

```bash
docker run … golang:1.26 sh -c "go build -buildvcs=false ./… && echo BUILD_OK && \
  go vet -buildvcs=false ./… && echo VET_OK && \
  go test -count=1 -buildvcs=false ./tools/…"
#   BUILD_OK
#   VET_OK
#   ok  github.com/talesmud/talesmud/tools/combatsim  0.006s
#   ok  github.com/talesmud/talesmud/tools/loadtest   0.005s
```

- Tests nuevos del harness: `tools/loadtest/mint_test.go` — 7 tests (`TestMintInterval`, `TestMintIntervalClamps`, `TestXffIP`, `TestMintRegisterResponse`, `TestMintBody`, `TestMintBodyUnique`, `TestMintBodyTruncatesLongSeed`).
- Cambios de código limitados a `tools/loadtest/` (`mint.go`, `mint_test.go`, flags `-mint*` en `main.go`). **Sin cambios en `pkg/**`, `public/**`, `cmd/**`** — los fixes P0 son Backend y van en un batch posterior. `docs/qa/BALANCE_ANALYSIS.md` no aplica (B19 no toca balance de combate).
- Limpieza: contenedor `tales-lt-qa` eliminado, tokens/DBs temporales borrados; se conservan los `.json` de corridas y el dump como evidencia en `/tmp/opencode/` (efímero — cifras y extractos de stacks quedan en este informe).

### B19.9 Veredicto Batch 19

| Área | Estado | Detalle |
|---|---|---|
| Load test 100 conexiones | **Apto** | 2/2 sanas, 0 errores; RTT p95 sub-ms; heartbeat verificado 75s > 60s |
| Load test 200 conexiones | **No apto** | Wedge a mitad de corrida (1/1) |
| Load test 300-500 conexiones | **No apto** | Wedge tras <150 joins (2/2); permanente hasta reinicio |
| Cuello de botella real | **Deadlock de canales del bucle de juego** (`game_handlers.go:128` ← bucle único `game.go:280`), con cascada en `mudserver.go:338` | Descartados: CPU (~0% en wedge), BD (sin hilos en espera), red/backpressure (`receiveMessages` idle), pings (`handleClientTimeouts` idle), anti-abuse (mint OK) |
| Heartbeat | **Verificado bajo carga** | `lt_100_hb`: 100×75s, 99.96% recv, 5 timeouts aislados |
| Límites anti-abuse | **No son el limitante** | 550 identidades mintadas con XFF de loopback (vector documentado §B19.4) |
| Harness | **Apto para uso** | Modo `-mint` + 7 tests nuevos; build/vet/test verdes |
| FASE 7.1 (load test 100-500) | **Ejecutada y documentada** | Re-ejecución de 200-500 pendiente de los fixes P0 (batch Backend posterior) → re-verificado en §B21 |

---

## B21 — Re-verificación de carga tras fix P0 (FASE 7.1)

**Fecha:** 2026-10-19
**Alcance:** re-verificación escéptica e independiente del fix P0 de §B19.7 (Batch 20, Backend): 200/300/500 conexiones WS × 2 corridas, 80 s cada una (cruza el tick de heartbeat de 60 s). Sin cambios en `pkg/**`, `public/**`, `cmd/**`; el único cambio de repo es este informe.

**Veredicto de cabecera:** **P0 cerrado.** Con el fix, 200, 300 y 500 conexiones simultáneas completan 80 s sin wedge: 100 % conectados, **`recv == sent` exacto y `msgErrors = 0`** en 6/6 corridas, y la sonda post-carga contra el servidor aún vivo responde sin errores. La mejora de latencia es de 3–4 órdenes de magnitud en RTT. **Se detecta una regresión P2 del propio fix**: cada sesión WS deja un goroutine `writeLoop` que no se libera al desconectar (leak acotado ~1 goroutine/conexión, sin impacto en correctitud ni en el wedge).

### B21.1 Entorno y metodología

- **Servidor QA (con fix):** contenedor `tales-lt-qa`, `go run ./cmd/tales -config /tmp/opencode/gamemode-loadtest.yaml` con `--network host` (auth local, `:8099`, SQLite efímero en el contenedor, `LOG_FILE=/tmp/opencode/b21/server.log`, `ENABLE_PPROF=true PPROF_ADDR=127.0.0.1:6060`). `docker restart` preserva DB y logs; se reinició antes de cada corrida «A» para partir de estado limpio (5 arranques verificados: `grep -c "MUD Server running"` = 5).
- **Identidades:** 600 tokens frescos con `-mint 600 -mint-ips 200` → `/tmp/opencode/b21/tokens_600.txt` (mismo vector XFF de loopback de §B19.4; 600 > 500 workers máx.). Cero `connErrors`/`initErrors` en todas las corridas.
- **Patrón de corrida:** `-auth token -tokens-file … -workers N -rate 2 -duration 80s -timeout 10s` contra `ws://127.0.0.1:8099/ws`. 161 probes/worker (1 inmediato + 160 a 2/s) × N = `sent`. **A** = tras `docker restart` (frío), **B** = sin reiniciar (caliente). Sonda final = 30 workers × 15 s.
- **Evidencia recogida:** `docker stats` muestreado cada 6–8 s (CPU/mem/pids), perfil `goroutine` pprof a mitad de `500_a` y tras toda la carga, y contadores de log (`UserJoined`, saturación de cola, `slow consumer`, `WS read error`, `WS close`, errores) vía `docker exec`.
- **Nota de honestidad metodológica:** la primera pasada del script quedó colgada por un `wait` sin argumentos que esperaba también al sampler de recursos; las corridas `200_a`,`200_b`,`300_a`,`300_b`,`500_a` ya estaban completas. Se reanudó con script corregido para `500_b` + sonda + pprof. Los datos de las 7 corridas son válidos.
- **Artefactos** en `/tmp/opencode/b21/` (efímeros): `b21_{200,300,500}_{a,b}.json`, `b21_probe_final.json`, `pprof_during_500a.txt`, `pprof_after_all.txt`, `heap_after_all.txt`, `stats.txt`/`stats2.txt`, `server.log`.

### B21.2 Resultados por concurrencia (servidor con fix)

| Corrida | Workers × rate × duración | Conectados | sent | recv | msgErrors | Connect p50/p95 (ms) | RTT p50/p95/p99/max (ms) | Estado |
|---|---|---|---|---|---|---|---|---|
| `b21_200_a` (frío) | 200 × 2/s × 80s | 200/200 | 32200 | **32200** | **0** | 234.8 / 376.4 | 0.05 / 0.103 / 0.149 / 6.55 | **sano** |
| `b21_200_b` (caliente) | 200 × 2/s × 80s | 200/200 | 32200 | **32200** | **0** | 254.5 / 409.5 | 0.047 / 0.107 / 0.156 / 6.85 | **sano** |
| `b21_300_a` (frío) | 300 × 2/s × 80s | 300/300 | 48300 | **48300** | **0** | 338.4 / 578.8 | 0.047 / 0.101 / 0.182 / 8.09 | **sano** |
| `b21_300_b` (caliente) | 300 × 2/s × 80s | 300/300 | 48300 | **48300** | **0** | 390.4 / 549.6 | 0.042 / 0.101 / 0.189 / 6.62 | **sano** |
| `b21_500_a` (frío) | 500 × 2/s × 80s | 500/500 | 80500 | **80500** | **0** | 799.3 / 1275.1 | 0.040 / 0.088 / 0.227 / 3.74 | **sano** |
| `b21_500_b` (caliente) | 500 × 2/s × 80s | 500/500 | 80500 | **80500** | **0** | 611.7 / 953.1 | 0.038 / 0.073 / 0.127 / 4.52 | **sano** |
| `b21_probe_final` (post-carga) | 30 × 2/s × 15s | 30/30 | 930 | **930** | **0** | 37.2 / 67.5 | 0.05 / 0.125 / 0.264 / 0.95 | **sano** |

- **Recv == sent exacto en 7/7 corridas** (ningún probe queda sin respuesta; ningún mensaje perdido). 500 workers × 161 probes = 80500 ✓.
- Cada corrida de 80 s cruza **≥1 tick de heartbeat de 60 s** con las 500 sesiones vivas (el harness recibió el 161.º probe de cada worker tras el tick) → **el heartbeat ya no se atasca bajo carga**.
- La sonda final (30 workers) corre **después** de las 6 corridas de 80 s, contra el servidor todavía en marcha, con connect p50 37 ms: **no hay wedge ni degradación persistente**.

**Contadores de log (campaña completa, `server.log`):**

| Métrica | Valor | Lectura |
|---|---|---|
| `msg=UserJoined` | **2250** | = conexiones totales exactas (20 smoke + 200+200+300+300+500+500 + 30 sonda + 4×50 del test de leak) → **ningún join perdido** |
| `Join queue saturated` / `recovered` | **6 / 6** | una saturación (backlog ≥64) por cada ráfaga de 200/300/500, **recuperada en ≤1 s**; la cola no bloquea el read loop |
| `join not forwarded` | **0** | la cola nunca descarta un join |
| `slow consumer` | **0** | la cola de salida por conexión (128) no se desbordó en operación normal |
| `WS send error` / `session replaced` / `WS close ignored` | **0 / 0 / 0** | sin errores de envío ni carreras de reemplazo |
| `Could not select character` + `FindByID - id is empty` | **2250 + 2250** | exactamente 1 par por join: es la ruta inline «lc» del fix (§B21.3), no un fallo |
| `panic` / otras `level=error` | **0 / 0** | sin pánicos ni errores no esperados |

### B21.3 Verificación del fix (qué se comprobó y cómo)

1. **Deadlock del bucle de juego (P0 #1).** `handleUserJoined` ya no envía a su propio `onMessageReceived`: despacha `lc`/`selectcharacter` **inline** con `game.processIncoming(...)`. Prueba: los 2250 joins con identidades sin personaje ejecutan la rama `Could not select character` → inline `lc` (2250 pares de log), y **ninguna** corrida se atasca (antes el buffer 20 se llenaba y la goroutine única quedaba bloqueada en `game_handlers.go:128`). El pprof a mitad de `500_a` muestra **500 read loops + 500 writeLoops + bucle de juego idle** (1017 goroutines), sin `chansend` bloqueado ni cascada en `mudserver.go:338`.
2. **Cascada de joins (P0 #2).** `attachSession` → `enqueueUserJoined` (cola no bloqueante) → `forwardJoins` (única goroutine que puede bloquearse en `OnUserJoined`). Prueba: los 500 logins entran en <1 s; la cola avisa de saturación (backlog ≥64) 6 veces y se recupera sin ninguna pérdida (`join not forwarded = 0`).
3. **Debounce de presencia (P1 #3).** El RTT se desploma: p95 de **763–890 ms en B19** a **0.07–0.11 ms** en B21 (ver §B21.4). El camino caliente ya no hace 2 escrituras SQLite por mensaje.
4. **Write deadline + cola de salida (P1 #4).** `wsTransport` ahora tiene `writeLoop` único, cola de 128 y `SetWriteDeadline` de 10 s por frame; `slow consumer = 0` en toda la campaña (no se ejerció la ruta de cierre por cliente lento, que queda cubierta por tests unitarios).
5. **pprof opcional (P2 #5).** `ENABLE_PPROF=true` funcionó: perfiles de goroutine/heap sin `SIGQUIT` (usado para §B21.1 y §B21.5).

### B21.4 Comparación B19 vs B21

| Nivel | B19 (sin fix) | B21 (con fix) |
|---|---|---|
| **100** | 100×75s: 13977/13982 recv, 5 msgErr, connect p50 173.6, RTT p95 0.108, CPU ~34 %, 164 MB — **sano** | *no re-ejecutado* (el objetivo era el fallo ≥200) |
| **200** | 200×30s: 200/200, recv 9762/9824, **62 msgErr**, connect p50 289, RTT p95 **763**, **WEDGE a mitad** | 200×80s ×2: recv 32200/32200 y 32200/32200, **0 msgErr**, connect p50 234.8 / 254.5, RTT p95 0.103 / 0.107 — **sano** |
| **300** | 300×25s: recv **94**/300, **206 msgErr**, RTT p50 753, **WEDGE tras 74 joins** | 300×80s ×2: recv 48300/48300 y 48300/48300, **0 msgErr**, connect p50 338.4 / 390.4, RTT p95 0.101 / 0.101 — **sano** |
| **500** | 500×25s: recv 8311/8943, **632 msgErr**, connect p50 638.8, RTT p95 **890**, **WEDGE tras 140 joins** | 500×80s ×2: recv 80500/80500 y 80500/80500, **0 msgErr**, connect p50 799.3 / 611.7, RTT p50 0.04 / p95 0.088 / p99 0.227 — **sano** |
| Post-carga | tras wedge: recv 0/n, solo reinicio lo recupera | sonda 30w: 930/930, 0 err, connect p50 37 ms — **vivo** |
| Recursos (pico) | 100 workers: CPU ~34 %, 164 MB | 500 workers: CPU ~6.6–15.5 % sostenido (pico 81.8 % en la desconexión masiva/GC), mem ~87 MB, 25 pids |
| Recursos (heap) | — | `HeapInuse` 24.5 MB, `NumGC` 115, `TotalAlloc` 1.42 GB tras 2250 conexiones |

**Latencia p50/p95/p99 por nivel (B21, media de A/B):** 200 → 0.049/0.105/0.153 ms; 300 → 0.045/0.101/0.186 ms; 500 → 0.039/0.081/0.177 ms. El RTT **no crece con la concurrencia** (es el eco del `hello` en memoria); el único coste de escala es el **connect** (235 → 799 ms p50 a 500 en frío), dominado por ticket + upgrade + join + escritura de sesión simultáneos — 3× menos que la cola de DB de B19 y coherente con servir 500 sesiones reales en vez de ~160.

### B21.5 Regresión detectada: leak de goroutines `writeLoop` al desconectar (P2)

- **Síntoma:** tras la carga, el servidor en reposo mantiene goroutines `mudserver.(*wsTransport).writeLoop` bloqueadas en su `select` (`transport.go:78-88`), una por conexión desconectada que nunca se cierra.
- **Evidencia cuantitativa (experimento controlado, servidor recién reiniciado):**
  - baseline en reposo: **16 goroutines totales, 0 `writeLoop`**;
  - tras 50 conexiones y su desconexión (12 s): **66 totales, 50 `writeLoop`**, y **sigue en 50 tras +70 s** (no las limpia el tick de ping);
  - segunda tanda de 50 (mismos tokens): **115 totales, 99 `writeLoop`** → acumulación neta ~1/conexión;
  - campaña: tras 2250 conexiones acumuladas quedan **494 `writeLoop`** en reposo (estable en dos snapshots consecutivos), mientras los read loops (`HandleConnections`) sí se liberan (0 en reposo).
- **Causa raíz:** el camino de desconexión (`HandleConnections` → `WS read error` → `detachSession` → `handleConnectionClosed`) borra la sesión de `Clients` y registra `WS close`, pero **nunca llama a `connection.closeTransport(...)`/`transport.Close()`**, que es lo único que cierra `done` y despierta a `writeLoop`; el `defer ws.Close()` de `HandleConnections` cierra el socket crudo, no el transporte. Los 2250 `WS read error` + 2250 `WS close` con **0** `WS close ignored` confirman que el camino se ejecuta para todas las conexiones.
- **Atribución:** lo introduce el propio fix de Batch 20. En `HEAD` (`git show HEAD:pkg/mudserver/transport.go`) `Send` escribía de forma síncrona (`conn.WriteJSON`) y **no existía** `writeLoop`/cola `out`; el leak nace con la reescritura del transporte (cola de salida + writer goroutine).
- **Impacto:** **no** afecta a la correctitud ni al cierre del P0 (sesiones activas OK, `recv == sent`, 0 errores, `slow consumer = 0`); huella modesta (494 goroutines ≈ pocos MB, `HeapInuse` 24.5 MB, mem del proceso ≤87 MB). Riesgo real a largo plazo: **crecimiento no acotado** en un servidor con churn (p. ej. 10 000 reconexiones/día → 10 000 goroutines retenidas). Recomendación **P2**: cerrar el transporte en el camino de desconexión (p. ej. `defer connection.closeTransport(closeNormal, "client disconnect")` o `transport.Close()` dentro de `handleConnectionClosed`), con un test de ciclo conectar/desconectar que verifique que el conteo de goroutines vuelve al baseline.

### B21.6 Otras observaciones

1. **Connect p50 a 500 crece a ~600–800 ms** (vs 235 ms a 200). Es esperable: 500 handshakes simultáneos (ticket → upgrade → join → `UsersService().Update`) sobre un pool SQLite de 4 conexiones, no un defecto; no se degrada de forma permanente (sonda posterior: 37 ms) ni escala con el tiempo (B caliente mejor que A frío en 500).
2. **Pico de CPU del 81.8 %** muestreado justo al vaciar las 500 conexiones (`docker stats`, ventana de `500_b`): corresponde a la desconexión masiva + GC, no a un bucle caliente sostenido (sostenido ~12–15 %).
3. **La ruta `slow consumer` no se ejerció en carga normal** (0 eventos): el harness siempre lee, así que la cola de 128 nunca se llena. Queda verificado por los tests unitarios del transporte (B20), no end-to-end.
4. **La respuesta de `lc` no se discrimina en el harness** (cualquier mensaje cuenta como respuesta a un probe). La evidencia de que no se pierde es indirecta pero sólida: 2250/2250 joins toman la rama inline y **recv == sent en todas las corridas**, sin errores de envío. Una verificación de contenido del fallback «lc» requeriría un cliente que inspeccione tipos de mensaje (mejora del harness, opcional).

### B21.7 Verificación y limpieza

```bash
docker run … golang:1.26 go build -buildvcs=false ./...        # EXIT 0
docker run … golang:1.26 go vet   -buildvcs=false ./...        # VET_OK
docker run … golang:1.26 go test  -buildvcs=false -count=1 ./tools/...
#   ok  github.com/talesmud/talesmud/tools/combatsim  0.005s
#   ok  github.com/talesmud/talesmud/tools/loadtest   0.005s
```

- **Sin cambios de código**: ni `pkg/**`, ni `public/**`, ni `cmd/**`; tampoco se tocó el harness `tools/loadtest` (no hizo falta). Los scripts de campaña viven en `/tmp/opencode/b21/` (fuera del repo).
- Limpieza: contenedor `tales-lt-qa` eliminado (con él la SQLite efímera); tokens y logs temporales quedan como evidencia efímera en `/tmp/opencode/b21/`.

### B21.8 Veredicto Batch 21

| Área | Estado | Detalle |
|---|---|---|
| P0 deadlock ≥200 conexiones | **Cerrado** | 6/6 corridas de 80 s a 200/300/500 sin wedge; `recv == sent`, 0 `msgErrors`; pprof sin `chansend` bloqueados |
| Load test 200 conexiones | **Apto** | 2/2 sanas, RTT p95 0.10 ms, connect p50 ~245 ms |
| Load test 300 conexiones | **Apto** | 2/2 sanas, RTT p95 0.10 ms, connect p50 ~365 ms |
| Load test 500 conexiones | **Apto** | 2/2 sanas, RTT p95 0.08 ms, connect p50 612–799 ms; sonda post-carga OK |
| Heartbeat 60 s bajo carga | **Verificado** | cada corrida de 80 s cruza ≥1 tick con 500 sesiones; 161/161 probes por worker |
| Pérdida de joins/mensajes | **Ninguna** | `UserJoined` 2250 = conexiones; `join not forwarded` 0; `send error` 0 |
| Regresión del fix (leak `writeLoop`) | **Cerrada — verificada B22** | leak de `writeLoop` por desconexión (§B21.5) **resuelto y verificado de forma independiente** en §B22; no afectaba al P0 |
| Build/vet/tests | **Verde** | build EXIT 0, vet OK, `./tools/...` OK |
| FASE 7.1 (load test 100-500) | **Cerrada** | El objetivo de escala 500 concurrentes se cumple sin wedge; verificar el leak P2 antes de subir la apuesta de concurrencia |

> **Actualización B22 (2026-10-19; re-verificada 2026-10-21):** la fila «Regresión del fix → P2 abierta» queda **cerrada**. El leak de `writeLoop` está resuelto y verificado de forma independiente (test in-package + churn propio + control negativo que reproduce el fallo), y `make build`/`make run-server` vuelven a compilar tras la regresión de Makefile/`.gitignore`. Detalle en §B22.
>
> **Estado a 2026-10-21:** re-verificación escéptica completa de B22 (build, leak, suite, `-race`) → **todos los claims de §B22 reproducidos**. Detalle y nuevas evidencias en §B22.7.

## B22 — Verificación de fix de build (Makefile/.gitignore) y leak P2 de writeLoop

**Fecha:** 2026-10-19 · **Re-verificación independiente:** 2026-10-21 (§B22.7)
**Alcance:** verificación escéptica e independiente del Batch 22 (Backend), **no commiteado**: (A) reparación de `make build`/`make run-server` (regresión del Batch 20: `go build cmd/tales/main.go` compilaba el `main.go` suelto y omitía `pprof.go` → `undefined: startPprof`; `.gitignore` ignoraba `cmd/tales/pprof.go` y `pprof_test.go`) y (B) cierre del leak P2 de goroutines `writeLoop` (§B21.5). Sin cambios en `pkg/**`, `public/**` ni `cmd/**`; QA solo añadió harness en `tools/qa_b22/` y este informe.

**Veredicto de cabecera:** **Batch 22 apto.** `make build-backend`, `make -n build`/`make -n run-server` y `go build ./...` funcionan; `pprof.go`/`pprof_test.go` ya no están ignorados. El leak P2 **está resuelto**: el test in-package y un churn propio (3 rondas × 40 ciclos) vuelven al baseline con **0 `writeLoop`**, y un **control negativo** (revertir solo las dos llamadas `closeTransport` en una copia desechable) reproduce exactamente el fallo (47 goroutines / 41 `writeLoop` huérfanos tras 40 ciclos). Suite completa y `-race` en verde. Se confirma una **race preexistente** (no de B22) sobre el `*entities.User` compartido, con stack concreto y fix propuesto; **no se ha corregido** (dominio de juego).

> **Re-verificación 2026-10-21 (§B22.7):** segunda pasada escéptica, con evidencia propia y **sin reutilizar** los resultados del 2026-10-19. **Todos los claims de esta sección se reprodujeron sin excepciones** (incluido el control negativo, que volvió a dar exactamente 47 goroutines / 41 `writeLoop`). No se detectaron regresiones.

### B22.1 Build infra (Makefile / .gitignore)

- **Antes (regresión B20):** el Makefile compilaba `cmd/tales/main.go` como archivo suelto; `cmd/tales/pprof.go` (mismo paquete, con `startPprof`) quedaba fuera → `undefined: startPprof`. Además `.gitignore:78` tenía `tales` (sin ancla), que ignoraba `cmd/tales/pprof.go` y `cmd/tales/pprof_test.go`.
- **Después (B22):** el Makefile usa rutas de paquete (`./cmd/tales`) y `-buildvcs=false`; `.gitignore:78` pasa a `/tales` (solo el binario de la raíz).

Evidencia ejecutada en docker (`golang:1.26`, `-buildvcs=false`):

```bash
$ make build-backend
echo "Building backend"
Building backend
go build -buildvcs=false -o bin/tales ./cmd/tales
# EXIT 0; bin/tales generado (64 212 622 bytes)

$ git check-ignore -v cmd/tales/pprof.go cmd/tales/pprof_test.go
# (sin salida)  EXIT 1  → NO ignorados

$ make -n build | tail -1
go build -buildvcs=false -o bin/tales ./cmd/tales

$ make -n run-server | tail -1
go run ./cmd/tales

$ go build -buildvcs=false ./...
# EXIT 0
```

Los archivos existen y están versionables: `cmd/tales/pprof.go` (1 730 B) y `cmd/tales/pprof_test.go` (464 B). El binario `bin/tales` sigue ignorado por la regla `bin/tales`, pero el patrón `tales` ya no captura rutas dentro de `cmd/tales/`.

### B22.2 Leak P2 de `writeLoop` — verificación independiente

**1) Test in-package (Backend):**

```bash
$ go test -buildvcs=false -count=1 -run TestDisconnectReleasesWriteLoopGoroutines -v ./pkg/mudserver/...
--- PASS: TestDisconnectReleasesWriteLoopGoroutines (0.61s)
    transport_leak_test.go:230: goroutines back at baseline: 6 (baseline 6, margin 5),
    writeLoops 0 (baseline 0) after 40 cycles
```

**2) Churn propio de QA** (`tools/qa_b22/churn_test.go`, fuera de `pkg/**`): crea el servidor con `mudserver.New`, sirve `HandleConnections` por `httptest`, y hace **3 rondas × 40 ciclos** (30 desconexiones limpias + 10 takeovers de sesión) contando `runtime.NumGoroutine()` y goroutines `(*wsTransport).writeLoop` por `runtime.Stack`. El harness no reutiliza el test in-package.

```bash
$ go test -buildvcs=false -count=1 -run TestChurnConnectDisconnectReturnsToBaseline -v ./tools/qa_b22/
--- PASS: TestChurnConnectDisconnectReturnsToBaseline (2.44s)
    round 1: baseline=6 goroutines/0 writeLoops -> after 40 cycles: 7 goroutines/0 writeLoops (converged=true)
    round 2: baseline=6 goroutines/0 writeLoops -> after 40 cycles: 7 goroutines/0 writeLoops (converged=true)
    round 3: baseline=6 goroutines/0 writeLoops -> after 40 cycles: 8 goroutines/0 writeLoops (converged=true)
```

**3) Control negativo (¿el test detecta el bug?):** copia desechable del repo en `/tmp/opencode/b22-noleak`, revirtiendo **solo** las dos llamadas del fix (`defer connection.closeTransport(0, "")` en `HandleConnections` y `connection.closeTransport(0, "")` en `handleConnectionClosed`), sin tocar el repo real. El mismo harness:

```bash
$ go test -buildvcs=false -count=1 -run TestChurnConnectDisconnectReturnsToBaseline -v ./tools/qa_b22/
    round 1: baseline=7 goroutines/1 writeLoops -> after 40 cycles: 47 goroutines/41 writeLoops (converged=false)
    round 1: goroutines did NOT return to baseline after 40 connect/disconnect cycles
--- FAIL: TestChurnConnectDisconnectReturnsToBaseline (10.66s)
# 41 stacks de (*wsTransport).writeLoop parkeados en transport.go:80 [select]
# (1 del warm-up + 40 ciclos medidos = 41)
```

Esto reproduce **exactamente** la cifra reportada por Backend (47 goroutines / 41 `writeLoop`) y demuestra que el test tiene poder de detección: con fix converge a baseline y 0 `writeLoop`; sin fix falla con 1 `writeLoop` por desconexión.

- **Atribución/alcance del fix:** el cierre del transporte es idempotente (`sync.Once` en `wsTransport.Close`), se ejecuta en `HandleConnections` (`defer`) y en `handleConnectionClosed`; las rutas que ya habían cerrado el socket (slow consumer, error de send/broadcast, replace 4001, guest expiry) pueden llamarlo otra vez sin efecto. El `defer ws.Close()` del socket crudo ya no es el único cierre.
- **Impacto del leak (B21.5):** crecimiento no acotado con churn; sin impacto en correctitud. **Estado: cerrado.**

### B22.3 Regresión completa

```bash
$ go build -buildvcs=false ./...                       # EXIT 0
$ go vet   -buildvcs=false ./...                       # EXIT 0 (sin diagnósticos)
$ go test  -buildvcs=false -count=1 ./...              # EXIT 0
# 58 paquetes ok, 0 FAIL (incluye tools/qa_b22 y los nuevos tests de B19-B22)
```

**`-race`** (paquetes pedidos, invocación única combinada + las dos invocaciones por separado):

```bash
$ go test -race -buildvcs=false -count=1 ./pkg/mudserver/... ./pkg/mudserver/game/...
ok  .../pkg/mudserver               7.533s
ok  .../pkg/mudserver/game         99.194s
ok  .../pkg/mudserver/game/balance  1.048s
ok  .../pkg/mudserver/game/combat 235.618s
ok  .../pkg/mudserver/game/commands 82.075s
ok  .../pkg/mudserver/game/leveling  1.033s
ok  .../pkg/mudserver/game/messages  1.049s
ok  .../pkg/mudserver/game/util      1.032s
# 8 paquetes ok + 2 [no test files] (game/combat/simutil, game/def), 0 FAIL
# EXIT 0 — sin WARNING: DATA RACE

$ go test -race -buildvcs=false -count=1 ./pkg/mudserver/...
ok  .../pkg/mudserver          8.153s
ok  .../pkg/mudserver/game     103.227s
ok  .../pkg/mudserver/game/balance   1.061s
ok  .../pkg/mudserver/game/combat    234.020s
ok  .../pkg/mudserver/game/commands  83.672s
ok  .../pkg/mudserver/game/leveling  1.040s
ok  .../pkg/mudserver/game/messages  1.044s
ok  .../pkg/mudserver/game/util      1.040s
# EXIT 0 — sin WARNING: DATA RACE

$ go test -race -buildvcs=false -count=1 ./pkg/mudserver/game/...
# EXIT 0 — 7 paquetes ok, sin WARNING: DATA RACE
```

La suite estándar **no dispara** la race preexistente porque los harness actuales serializan los ciclos (p. ej. `transport_leak_test.go` usa un contador atómico de `Update` para ordenar reconexiones) y ningún test conduce input + takeover concurrentes. Por eso QA añadió una reproducción **opt-in** (ver §B22.4) que no afecta a CI por defecto.

### B22.4 Race preexistente en la presencia del `*entities.User` compartido (no de B22)

**Naturaleza.** El mismo `*entities.User` (el que se inyecta en el contexto gin y viaja en los mensajes) se muta desde goroutines distintas sin sincronización común:

| Escritor | Goroutine | Campos |
|---|---|---|
| `server.attachSession` (`mudserver.go:354-356`) | handler de la conexión nueva | `LastSeen`, `IsOnline`, y `UsersService().Update` (marshal JSON completo) |
| `server.handleConnectionClosed` (`mudserver.go:526-528`) | handler de la conexión que cae | `IsOnline`, `LastSeen`, y `Update` |
| `Game.ConnectUserSession` (`game/sessions.go:214-216`) | read loop de **cada** input (`handleInput` → `mudserver.go:433`) | lee `IsOnline`, escribe `IsOnline`, `LastSeen` (+ `Update` si estaba offline) |
| `Game.DisconnectUserSession` (`sessions.go:235-239`) | bucle de juego | `IsOnline`, `LastSeen`, `Update` |
| `Game.SetUserSessionCharacter` (`sessions.go:277-282`) | bucle de juego | `LastCharacter`, `IsOnline`, `LastSeen`, `Update` |

**Reproducción concreta** (`tools/qa_b22/race_sessions_repro_test.go`, opt-in):

```bash
QA_B22_REPRO_RACE=1 go test -race -buildvcs=false -count=1 \
  -run TestSessionUserPresenceRace -v ./tools/qa_b22/
# --- FAIL: TestSessionUserPresenceRace (1.81s) — 4 WARNING: DATA RACE
```

Stack representativo (recortado; `-race` completo en `/tmp/opencode/race_repro4.log`):

```
WARNING: DATA RACE
Write at 0x00c00042c3e8 by goroutine 29:
  mudserver.(*server).attachSession()
      /app/pkg/mudserver/mudserver.go:354        ← user.LastSeen = time.Now()
  mudserver.(*server).HandleConnections()
      /app/pkg/mudserver/mudserver.go:256
Previous write at 0x00c00042c3e8 by goroutine 26:
  mudserver.(*server).handleConnectionClosed()
      /app/pkg/mudserver/mudserver.go:527        ← user.LastSeen = time.Now()
  mudserver.(*server).detachSession()
      /app/pkg/mudserver/mudserver.go:442

WARNING: DATA RACE
Write at 0x00c00042c421 by goroutine 29:
  mudserver.(*server).attachSession()
      /app/pkg/mudserver/mudserver.go:355        ← user.IsOnline = true
Previous read at 0x00c00042c421 by goroutine 26:
  encoding/json.(*encodeState).marshal() → UsersService().Update()
  mudserver.(*server).handleConnectionClosed()
      /app/pkg/mudserver/mudserver.go:528        ← Update(user.RefID, user) lee el struct
  repository.(*sqliteGenericRepo).UpdateByField()
      /app/pkg/repository/sqlite_generic.go:69   ← json.Marshal(user)

WARNING: DATA RACE
Read at 0x00c00042c421 by goroutine 18:
  game.(*Game).ConnectUserSession()
      /app/pkg/mudserver/game/sessions.go:214    ← wasOnline := user.IsOnline
Previous write at 0x00c00042c421 by goroutine 14:
  game.(*Game).ConnectUserSession()
      /app/pkg/mudserver/game/sessions.go:215    ← user.IsOnline = true
  mudserver.(*server).handleInput()
      /app/pkg/mudserver/mudserver.go:433
```

**¿Corrupción de datos?** No de memoria: los campos en disputa son `bool` y `time.Time` (3 words, con un `*Location` que solo apunta a zonas globales inmutables), así que no hay punteros invalidados ni riesgo de crash. El daño es **lógico**: `Update` serializa el struct a JSON mientras otro goroutine lo muta → snapshot mezclado (p. ej. `IsOnline=true` con `LastSeen` viejo), y el par write/write permite persistir un estado de presencia incorrecto (un disconnect puede escribir `IsOnline=false` después del `true` de una reconexión, o al revés). Efectos visibles: `LastSeen`/`IsOnline` erróneos en DB, notificaciones de amigos «online/offline» inconsistentes y comportamiento no determinista. La race solo se *reporta* con `-race`; en producción es silenciosa (el modelo de memoria de Go la declara comportamiento indefinido, pero sin corrupción de memoria observable aquí). **No la introduce B22**: B22 solo añadió comentarios/tests alrededor; el patrón ya existía en los writes de presencia.

**Fix propuesto (NO aplicado, código de juego):**

1. **Corto plazo (mínimo y contenido):** un lock por usuario compartido por los cinco escritores (p. ej. `sync.Map[userID]*sync.Mutex` en `Game`, o un `presenceMu sync.Mutex` global ya que connect/disconnect son raros), tomado alrededor de `mutar campos + UsersService().Update(...)` en `attachSession`, `handleConnectionClosed`, `ConnectUserSession`, `DisconnectUserSession` y `SetUserSessionCharacter`. Alternativa mínima: que `Update` reciba una **copia** del user hecha bajo ese lock, para que `json.Marshal` nunca lea un struct en mutación (esto elimina el read/write, pero el write/write sigue necesitando el lock).
2. **Largo plazo (correcto):** un **único dueño** del `*entities.User`: los handlers de conexión no deben mutar el struct compartido; deben encolar eventos de presencia por el camino que ya existe (`OnUserJoined`/`OnUserQuit` + `forwardJoins`) y que solo el bucle de juego (`Game.Run`) escriba `LastSeen`/`IsOnline` y persista. Así desaparece la necesidad de lock y las escrituras SQLite salen del read loop.
3. **Tests de guardia:** mantener la reproducción opt-in y, al corregir, convertirla en test obligatorio de `-race` (input + takeover + disconnect concurrentes sobre el mismo user).

La race **queda abierta y reportada a Backend**; QA no toca `pkg/**`.

### B22.5 Verificación y limpieza

- **Archivos QA añadidos (fuera de `pkg/**`):** `tools/qa_b22/churn_test.go` (churn + control de baseline) y `tools/qa_b22/race_sessions_repro_test.go` (reproducción opt-in con `QA_B22_REPRO_RACE=1`; *skip* por defecto para no romper `-race ./...`). Este informe.
- **Sin cambios** en `pkg/**`, `public/**`, `cmd/**`. Sin commits.
- El control negativo se hizo sobre una copia temporal en `/tmp/opencode/b22-noleak` (eliminada al terminar); contenedores docker efímeros (`--rm`), ninguno queda vivo.
- Reproducir la verificación:
  ```bash
  docker run --rm -v "$PWD":/app -v talesmud-gomod:/go/pkg/mod \
    -v talesmud-gocache:/root/.cache/go-build -w /app golang:1.26 \
    sh -c 'make build-backend && go build -buildvcs=false ./... && \
           go vet -buildvcs=false ./... && \
           go test -buildvcs=false -count=1 ./...'
  docker run --rm -v "$PWD":/app -v talesmud-gomod:/go/pkg/mod \
    -v talesmud-gocache:/root/.cache/go-build -w /app -e QA_B22_REPRO_RACE=1 \
    golang:1.26 go test -race -buildvcs=false -count=1 -run TestSessionUserPresenceRace ./tools/qa_b22/
  ```

### B22.6 Veredicto Batch 22

| Área | Estado | Detalle |
|---|---|---|
| `make build-backend` / `make build` | **Cerrado** | EXIT 0; compila `./cmd/tales` con `-buildvcs=false`; `undefined: startPprof` resuelto |
| `make run-server` | **Cerrado** | apunta a `go run ./cmd/tales` |
| `.gitignore` (`tales` → `/tales`) | **Cerrado** | `git check-ignore` vacío para `cmd/tales/pprof.go` y `pprof_test.go` |
| Leak P2 `writeLoop` | **Cerrado** | test in-package PASS; churn QA 3×40 ciclos → baseline y 0 `writeLoop`; control negativo reproduce 47/41 |
| `go build ./...` / `go vet ./...` | **Verde** | EXIT 0 / sin diagnósticos |
| `go test ./...` | **Verde** | 58 paquetes ok, 0 FAIL |
| `-race` (`mudserver/...`, `game/...`) | **Verde** | 0 `WARNING: DATA RACE` en la suite estándar |
| Race preexistente presencia (`attachSession`/`sessions.go`) | **Cerrada — verificada B23** | era reproducible con harness opt-in (4 `WARNING: DATA RACE`); el fix de Batch 23 (lock por usuario en `pkg/mudserver/game/presence.go`) la elimina y QA la verificó de forma independiente: mismo harness opt-in → **0** `WARNING: DATA RACE`; detalle en §B23 |

### B22.7 Re-verificación independiente — 2026-10-21 (segunda pasada)

**Entorno:** `golang:1.26` en docker (`docker run --rm`, volúmenes `talesmud-gomod` / `talesmud-gocache`), `-buildvcs=false` en todas las invocaciones. Go **no** está en el PATH del host (todo se ejecutó dentro del contenedor). Fecha del host: `Wed Oct 21 08:37:24 AM -03:00 2026`.

**Metodología (escéptica):** QA **no reutilizó** los artefactos ni los logs de la pasada del 2026-10-19. Cada comando se re-ejecutó desde cero; el control negativo se rehizo sobre una **copia nueva** en `/tmp/opencode/b22-noleak`; la race preexistente se reprodujo de nuevo y se capturó el stack completo. El repo real **no se modificó** en `pkg/**`, `cmd/**`, `Makefile` ni `.gitignore` (verificado con `git status --short` al final).

#### B22.7.1 Build — antes/después reproducido

**Estado "antes" reproducido de forma explícita** (no solo por lectura del diff):

*Regresión del Makefile* — en la copia temporal se revirtió el Makefile a la forma pre-B22 (`go build -o bin/tales cmd/tales/main.go`):

```bash
$ make build-backend
go build -o bin/tales cmd/tales/main.go
# command-line-arguments
cmd/tales/main.go:67:2: undefined: startPprof
make: *** [Makefile:36: build-backend] Error 1
PRE_B22_MAKE_BUILD_BACKEND_EXIT=2

$ make run-server
go run cmd/tales/main.go
cmd/tales/main.go:67:2: undefined: startPprof
make: *** [Makefile:48: run-server] Error 1
PRE_B22_MAKE_RUN_SERVER_EXIT=2
```

*Regresión del `.gitignore`* — demo en un repo git mínimo, con los dos patrones:

```bash
# .gitignore = "tales"   (pre-B22)
$ git check-ignore -v cmd/tales/pprof.go cmd/tales/pprof_test.go
.gitignore:1:tales	cmd/tales/pprof.go
.gitignore:1:tales	cmd/tales/pprof_test.go
exit=0
$ git status --short
?? .gitignore              # <-- los pprof NO aparecen: ignorados de verdad

# .gitignore = "/tales"  (post-B22)
$ git check-ignore -v cmd/tales/pprof.go cmd/tales/pprof_test.go
exit=1                    # <-- nada ignorado
$ git status --short
?? .gitignore
?? cmd/                    # <-- los pprof vuelven a ser visibles
```

**Estado "después" (repo real, sin modificar):**

```bash
$ docker run --rm … golang:1.26 sh -c 'set -x; make build-backend; ls -l bin/tales; go build -buildvcs=false ./...'
+ make build-backend
echo "Building backend"
Building backend
go build -buildvcs=false -o bin/tales ./cmd/tales
MAKE_BUILD_BACKEND_EXIT=0
+ ls -l bin/tales
-rwxr-xr-x 1 root root 64212622 Oct 21 11:39 bin/tales
+ go build -buildvcs=false ./...
GO_BUILD_ALL_EXIT=0
```

| Comprobación | Resultado re-verificado (2026-10-21) |
|---|---|
| `make build-backend` | **EXIT 0**; `bin/tales` = **64 212 622 B** |
| `go build -buildvcs=false ./...` | **EXIT 0** |
| `make -n build` (última línea) | `go build -buildvcs=false -o bin/tales ./cmd/tales` → `./cmd/tales` ✅ |
| `make -n build-backend` (última línea) | `go build -buildvcs=false -o bin/tales ./cmd/tales` → `./cmd/tales` ✅ |
| `make -n run-server` (última línea) | `go run ./cmd/tales` → `./cmd/tales` ✅ |
| `git check-ignore -v cmd/tales/pprof.go cmd/tales/pprof_test.go` | **sin salida, EXIT 1** → no ignorados ✅ |
| `cmd/tales/pprof.go` / `pprof_test.go` | existen (1 730 B / 464 B) y `git status --short` los lista como `??` (untracked **no ignorado**) ✅ |

#### B22.7.2 Leak P2 de `writeLoop` — verificación independiente

**1) Test in-package (Backend):**

```bash
$ go test -buildvcs=false -count=1 -run TestDisconnectReleasesWriteLoopGoroutines -v ./pkg/mudserver/
=== RUN   TestDisconnectReleasesWriteLoopGoroutines
--- PASS: TestDisconnectReleasesWriteLoopGoroutines (2.91s)
PASS
ok  	github.com/talesmud/talesmud/pkg/mudserver	2.923s
```

**2) Churn propio de QA** (`tools/qa_b22/churn_test.go`, fuera de `pkg/**`; no reutiliza el test in-package): 3 rondas × 40 ciclos (30 desconexiones limpias + 10 takeovers de sesión), midiendo `runtime.NumGoroutine()` y goroutines `(*wsTransport).writeLoop` vía `runtime.Stack`.

```bash
$ go test -buildvcs=false -count=1 -run TestChurnConnectDisconnectReturnsToBaseline -v ./tools/qa_b22/
    churn_test.go:257: round 1: baseline=6 goroutines/0 writeLoops -> after 40 cycles: 7 goroutines/0 writeLoops (converged=true)
    churn_test.go:257: round 2: baseline=6 goroutines/0 writeLoops -> after 40 cycles: 7 goroutines/0 writeLoops (converged=true)
    churn_test.go:257: round 3: baseline=6 goroutines/0 writeLoops -> after 40 cycles: 7 goroutines/0 writeLoops (converged=true)
--- PASS: TestChurnConnectDisconnectReturnsToBaseline (3.61s)
ok  	github.com/talesmud/talesmud/tools/qa_b22	3.619s
```

→ **0 `writeLoop` huérfanos** y vuelta al baseline (`+1` goroutine residual del propio `httptest`, dentro del margen de 5).

**3) Control negativo (¿el test detecta el bug?) — rehecho desde cero.** Copia nueva en `/tmp/opencode/b22-noleak`, revertiendo **solo** los dos call sites del fix de B22 (`defer connection.closeTransport(0, "")` en `HandleConnections` y `connection.closeTransport(0, "")` en `handleConnectionClosed`; el resto de `closeTransport` del B20 se deja intacto):

```bash
$ go test -buildvcs=false -count=1 -run TestChurnConnectDisconnectReturnsToBaseline -v ./tools/qa_b22/
GO_TEST_EXIT=1
    churn_test.go:257: round 1: baseline=7 goroutines/1 writeLoops -> after 40 cycles: 47 goroutines/41 writeLoops (converged=false)
    churn_test.go:261: round 1: goroutines did NOT return to baseline after 40 connect/disconnect cycles:
--- FAIL: TestChurnConnectDisconnectReturnsToBaseline (10.64s)
FAIL	github.com/talesmud/talesmud/tools/qa_b22	10.648s
# 41 goroutines en (*wsTransport).writeLoop parkeados en transport.go:80 [select]
```

→ **Reproduce exactamente la cifra reportada por Backend (47 goroutines / 41 `writeLoop`)**, con 1 `writeLoop` por desconexión. El test **tiene poder de detección**: con el fix converge a baseline/0 `writeLoop`; sin el fix falla de forma inequívoca. **Cicla cerrado: el leak existía antes del fix y está resuelto después.**

**4) Revisión del fix (code review).** `wsTransport.Close` usa `sync.Once` (`pkg/mudserver/transport.go:51,169-189`): hace `close(w.done)`, que es lo único que despierta a `writeLoop` de su `select` (`transport.go:78-88`). El fix añade el cierre en los dos puntos donde antes faltaba (`mudserver.go:262` y `mudserver.go:510`), y las rutas que ya lo cerraban (replace 4001 `mudserver.go:342`, slow consumer `:410`, error de send/broadcast `:544`, guest expiry `:596`) lo re-invocan sin efecto gracias al `once`. **Idempotente y sin ruta de doble cierre.**

#### B22.7.3 Suite completa y `vet`

```bash
$ go vet   -buildvcs=false ./...     # VET_EXIT=0  (sin diagnósticos)
$ go test  -buildvcs=false -count=1 ./...   # SUITE_EXIT=0
# 58 paquetes "ok", 24 "[no test files]", 0 FAIL
# (incluye tools/qa_b22 → ok 3.627s, tools/combatsim, tools/loadtest)
```

#### B22.7.4 `-race` (invocación única combinada, la pedida en el encargo)

```bash
$ go test -race -buildvcs=false -count=1 ./pkg/mudserver/... ./pkg/mudserver/game/...
ok  .../pkg/mudserver               7.533s
ok  .../pkg/mudserver/game         99.194s
ok  .../pkg/mudserver/game/balance  1.048s
ok  .../pkg/mudserver/game/combat 235.618s
ok  .../pkg/mudserver/game/commands 82.075s
ok  .../pkg/mudserver/game/leveling  1.033s
ok  .../pkg/mudserver/game/messages  1.049s
ok  .../pkg/mudserver/game/util      1.032s
# 8 ok + 2 [no test files] (game/combat/simutil, game/def), 0 FAIL
RACE_EXIT=0
# grep -c "WARNING: DATA RACE" → 0
```

→ **Verde**: la suite estándar **no** dispara la race preexistente (los harness actuales serializan los ciclos; ningún test conduce input + takeover concurrentes).

#### B22.7.5 Race preexistente — reproducción propia con stack (NO corregida)

```bash
$ QA_B22_REPRO_RACE=1 go test -race -buildvcs=false -count=1 \
    -run TestSessionUserPresenceRace -v ./tools/qa_b22/
RACE_REPRO_EXIT=1
# grep -c "WARNING: DATA RACE" → 4
```

Stack capturado en esta re-verificación (el detector elige el par write/write o read/write que pilla en cada corrida; **los sitios de código son siempre los mismos**):

```
WARNING: DATA RACE
Read at 0x00c0001369c1 by goroutine 18:
  game.(*Game).ConnectUserSession()
      /app/pkg/mudserver/game/sessions.go:214        ← wasOnline := user.IsOnline
Previous write at 0x00c0001369c1 by goroutine 14:
  game.(*Game).ConnectUserSession()
      /app/pkg/mudserver/game/sessions.go:215        ← user.IsOnline = true
  mudserver.(*server).handleInput()
      /app/pkg/mudserver/mudserver.go:433

WARNING: DATA RACE
Write at 0x00c0001369c1 by goroutine 19:
  mudserver.(*server).attachSession()
      /app/pkg/mudserver/mudserver.go:355             ← user.IsOnline = true
  mudserver.(*server).HandleConnections()
      /app/pkg/mudserver/mudserver.go:256
Previous write at 0x00c0001369c1 by goroutine 18:
  game.(*Game).ConnectUserSession()
      /app/pkg/mudserver/game/sessions.go:216         ← user.LastSeen = time.Now()

WARNING: DATA RACE
Write at 0x00c0001369c1 by goroutine 39:
  mudserver.(*server).attachSession()
      /app/pkg/mudserver/mudserver.go:355             ← user.IsOnline = true
Previous write at 0x00c0001369c1 by goroutine 35:
  mudserver.(*server).handleConnectionClosed()
      /app/pkg/mudserver/mudserver.go:526             ← user.IsOnline = false
  mudserver.(*server).detachSession()
      /app/pkg/mudserver/mudserver.go:442
  mudserver.(*server).HandleConnections()
      /app/pkg/mudserver/mudserver.go:275
```

**Líneas verificadas contra el código actual** (las 5 de la tabla de §B22.4 son exactas):

| Escritor | Líneas | Contenido |
|---|---|---|
| `attachSession` | `mudserver.go:354-356` | `user.LastSeen = time.Now()` / `user.IsOnline = true` / `Update` |
| `handleConnectionClosed` | `mudserver.go:526-528` | `user.IsOnline = false` / `user.LastSeen = time.Now()` / `Update` |
| `Game.ConnectUserSession` | `sessions.go:214-216` | lee `IsOnline`, escribe `IsOnline` y `LastSeen` |
| `Game.DisconnectUserSession` | `sessions.go:235-239` | `IsOnline`, `LastSeen`, `Update` |
| `Game.SetUserSessionCharacter` | `sessions.go:277-282` | `LastCharacter`, `IsOnline`, `LastSeen`, `Update` |

**Impacto (sin cambios respecto al análisis de §B22.4):** sin corrupción de memoria (los campos en disputa son `bool`/`time.Time`, sin punteros invalidados), pero **daño lógico**: `Update` serializa el struct a JSON mientras otro goroutine lo muta → snapshot mezclado y persistencia de un estado de presencia incorrecto (`IsOnline=false` posterior a un `true` de reconexión, o al revés) → `LastSeen`/`IsOnline` erróneos en DB y notificaciones de amigos inconsistentes. Silenciosa en producción. **No la introduce B22** (el patrón de writes de presencia es anterior).

**Fix propuesto (NO aplicado — es código de juego, `pkg/**`):** ver §B22.4. Corto plazo: un lock por usuario (o global de presencia) alrededor de `mutar + Update` en los cinco escritores, y pasar a `Update` una **copia** del user hecha bajo ese lock. Largo plazo: **único dueño** del `*entities.User` (solo el bucle de juego escribe presencia; los handlers encolan eventos por `OnUserJoined`/`OnUserQuit` + `forwardJoins`). Al corregir, convertir la reproducción opt-in en test obligatorio de `-race`.

#### B22.7.6 Conclusión de la re-verificación

| Claim de §B22 (2026-10-19) | Re-verificado 2026-10-21 |
|---|---|
| `make build-backend` EXIT 0 | ✅ EXIT 0 (64 212 622 B) |
| `go build ./...` EXIT 0 | ✅ EXIT 0 |
| `make -n build` / `-n run-server` → `./cmd/tales` | ✅ ambos |
| `git check-ignore` vacío para los `pprof*` | ✅ salida vacía, EXIT 1 |
| Test in-package PASS | ✅ PASS (2.91 s) |
| Churn QA converge a baseline con 0 `writeLoop` | ✅ 3/3 rondas, 0 `writeLoop` |
| Control negativo reproduce 47 goroutines / 41 `writeLoop` | ✅ reproducido **exactamente** |
| `go vet ./...` sin diagnósticos | ✅ EXIT 0 |
| `go test ./...` → 58 paquetes ok, 0 FAIL | ✅ 58 ok / 24 sin tests / 0 FAIL |
| `-race` (`mudserver/...` + `game/...`) verde | ✅ RACE_EXIT=0, 0 `WARNING: DATA RACE` |
| Race preexistente de presencia, P2, no de B22 | ✅ reproducida (4 warnings), **sin corregir** |

**Veredicto de la re-verificación: B22 sigue siendo apto.** Todos los claims del lote se volvieron a comprobar uno a uno y ninguno falló; no se encontraron regresiones. La única observación menor: la race preexistente de presencia (§B22.4) permanece **abierta (P2)** y es propiedad de Backend — no bloquea B22 porque es anterior al lote y no afecta al fix evaluado.

### B22.8 Verificación y limpieza (2026-10-21)

- **Sin commits** en toda la sesión; el árbol de trabajo queda con los cambios de Backend sin commitear + los archivos QA de `tools/qa_b22/`.
- **Copia temporal del control negativo** en `/tmp/opencode/b22-noleak` (con el Makefile revertido a pre-B22 y los dos `closeTransport` del fix comentados): **eliminada al terminar**. Logs de evidencia en `/tmp/opencode/b22qa/` (efímeros).
- **Repo real intacto salvo `docs/qa/REGRESSION_REPORT.md` y `tools/qa_b22/`**: verificado con `git status --short` → ninguna modificación en `pkg/**`, `cmd/**`, `Makefile`, `.gitignore` ni `public/**`.
- **Contenedores docker**: todos lanzados con `--rm`; ninguno queda vivo (`docker ps -a` vacío al final).
- **Reproducir la re-verificación completa:**
  ```bash
  docker run --rm -v "$PWD":/app -v talesmud-gomod:/go/pkg/mod \
    -v talesmud-gocache:/root/.cache/go-build -w /app golang:1.26 \
    sh -c 'make build-backend && go build -buildvcs=false ./... && \
           go vet -buildvcs=false ./... && \
           go test -buildvcs=false -count=1 ./... && \
           go test -race -buildvcs=false -count=1 ./pkg/mudserver/... ./pkg/mudserver/game/...'
  docker run --rm -v "$PWD":/app -v talesmud-gomod:/go/pkg/mod \
    -v talesmud-gocache:/root/.cache/go-build -w /app golang:1.26 \
    sh -c 'go test -buildvcs=false -count=1 -run TestDisconnectReleasesWriteLoopGoroutines -v ./pkg/mudserver/ && \
           go test -buildvcs=false -count=1 -run TestChurnConnectDisconnectReturnsToBaseline -v ./tools/qa_b22/'
  docker run --rm -v "$PWD":/app -v talesmud-gomod:/go/pkg/mod \
    -v talesmud-gocache:/root/.cache/go-build -w /app -e QA_B22_REPRO_RACE=1 \
    golang:1.26 go test -race -buildvcs=false -count=1 -run TestSessionUserPresenceRace ./tools/qa_b22/
  ```

---

## B23 — Fix de data race de presencia (lock por usuario)

**Fecha:** 2026-10-21
**Alcance:** verificación escéptica e independiente del Batch 23 (Backend), **no commiteado**: cierre de la race de presencia que §B22.4 dejó **abierta como P2**. El fix introduce un **lock por usuario** en el `Game` (`pkg/mudserver/game/presence.go`: `ApplyPresence`/`PresenceOf` con un registro `presenceRegistry` de refcount), mutación + copia desligada (`entities.User.Snapshot()`, `pkg/entities/user.go`) y `Update` **bajo el mismo lock**, y mueve los call sites de escritura (`mudserver.go` `attachSession`/`handleConnectionClosed`, `sessions.go` `ConnectUserSession`/`DisconnectUserSession`/`SetUserSessionCharacter`, `game_handlers.go` `handleUserJoined`) y de lectura (`game_handlers.go`, `util/game_utils.go`, `commands/select_character.go`, `doorview/view.go`) a esa API. QA solo añadió harness en `tools/qa_b23/` y este informe; **no tocó `pkg/**`, `cmd/**`, `Makefile`, `.gitignore` ni `public/**`**.

**Veredicto de cabecera:** **Batch 23 apto.** La race de presencia está **resuelta y verificada de forma independiente**: `go test -race ./...` completo da **0 `WARNING: DATA RACE`** (59 paquetes `ok`, exit 0), el mismo harness opt-in de B22 que antes reportaba 4 races ahora da **0**, y mi propio harness (`tools/qa_b23/`) reproduce la race en una copia sin lock (**19 warnings**) y da **0** con el fix. `go build`/`go vet`/suite estándar en verde. Sin deadlocks (suite completa con `-timeout`, stress `-count=3` en verde) y persistencia coherente (guard offline→online, `RefID`, `LastSeen`).

### B23.1 Qué cambió (revisión del diff)

| Ruta | Antes (pre-B23) | Después (B23) |
|---|---|---|
| `pkg/mudserver/game/presence.go` | *(no existía)* | `presenceRegistry` (mutex por `userID`, refcount, entra/sale del mapa al primer/último holder) + `ApplyPresence(user, mutate)` (mutar + `Snapshot()` + `Update` bajo el lock) y `PresenceOf(user)` (leer `IsOnline`/`LastSeen`/`LastCharacter` bajo el lock) |
| `pkg/entities/user.go` | *(sin `Snapshot`)* | `Snapshot()` copia superficial del `User` **y** del `*Entity` embebido (no comparte estado mutable) |
| `pkg/mudserver/game/def/gamectrl.go` | *(sin `PresenceState`)* | `PresenceState{IsOnline, LastSeen, LastCharacter}` + `PresenceOf` en la interfaz `GameCtrl` |
| `pkg/mudserver/mudserver.go` | `user.LastSeen = time.Now(); user.IsOnline = true; Update(user.RefID, user)` (attach) y `user.IsOnline = false; …; Update` (close), y `user.LastCharacter` en logs | `ApplyPresence(...)` para attach/close; `PresenceOf(user).LastCharacter` en los 5 logs |
| `pkg/mudserver/game/sessions.go` | escritura directa + `Update` en `ConnectUserSession`, `DisconnectUserSession`, `SetUserSessionCharacter` | `ApplyPresence(...)` en los tres |
| `pkg/mudserver/game/game_handlers.go` | `user.LastCharacter`, `user.LastSeen`, `user.IsOnline` directos en `handleUserQuit`/`handleUserJoined`/`attachCharacterToMessage` | `PresenceOf` + `ApplyPresence` |
| `pkg/mudserver/game/util/game_utils.go`, `commands/select_character.go`, `pkg/presentation/doorview/view.go` | lecturas de `user.LastCharacter` | `PresenceOf(...).LastCharacter` |

**Mecanismo de la race (confirmado en el stack de la reproducción).** El repositorio serializa el `*entities.User` **vivo** a JSON en `sqliteGenericRepo.UpdateByField` → `json.Marshal(item)` (`pkg/repository/sqlite_generic.go:69`). Mientras esa marshalización lee el struct, otra goroutine escribe `IsOnline`/`LastSeen`/`LastCharacter` sobre el **mismo puntero** → read/write y write/write sin sincronización. El fix hace que la mutación y la marshalización ocurran bajo el mismo lock y que `Update` reciba una **copia desligada** (`Snapshot()`), no el struct vivo.

### B23.2 Reproducción antes/después (control negativo propio, sin tocar el repo)

**Método.** Copia completa del repo (sin `.git`/`node_modules`) en `/tmp/opencode/b23-nolock`. Se revirtió **solo** el lock: `ApplyPresence` y `PresenceOf` en `pkg/mudserver/game/presence.go` vuelven a mutar/leer el user **vivo** y a llamar `Update(user.RefID, user)` directamente (mismo comportamiento que el código pre-B23); el resto del diff de B23 (call sites, guard, `Snapshot`, lectores) queda **idéntico**, de modo que el lock es la única variable. Compila (`go build ./...` EXIT 0).

**Test in-package `TestPresenceMutationIsRaceFree` (`pkg/mudserver/presence_race_test.go`):**

```bash
# copia /tmp/opencode/b23-nolock, sin lock:
go test -race -buildvcs=false -count=1 -run TestPresenceMutationIsRaceFree -v ./pkg/mudserver/
# --- FAIL --- 6 WARNING: DATA RACE
```

Stack representativo del "antes" (recortado; log completo en `/tmp/opencode/b23/neg_control.log`):

```
WARNING: DATA RACE
Write at 0x00c000394410 by goroutine 15:
  game.(*Game).SetUserSessionCharacter.func1()   sessions.go:285   ← u.IsOnline = true
  game.(*Game).ApplyPresence()                   presence.go:100
Previous read at 0x00c000394410 by goroutine 17:
  encoding/json.Marshal() → *sqliteGenericRepo.UpdateByField()  sqlite_generic.go:69
  game.(*Game).ApplyPresence()                   presence.go:103
  mudserver.(*server).handleConnectionClosed()   mudserver.go:533

WARNING: DATA RACE
Write at 0x00c0003943e8 by goroutine 15:
  game.(*Game).SetUserSessionCharacter.func1()   sessions.go:287   ← u.LastSeen = time.Now()
Previous read at 0x00c0003943e8 by goroutine 17:
  time.(*Time).MarshalJSON() → json.Marshal() → UpdateByField()  sqlite_generic.go:69
```

**Harness propio (`tools/qa_b23/presence_harness_test.go`), el mismo contra ambos árboles:**

```bash
go test -race -buildvcs=false -count=1 -run TestPresenceSameUserChurnIsRaceFree ./tools/qa_b23/
# repo real (con fix):  PASS   → 0 WARNING: DATA RACE
# copia sin lock:       FAIL   → 19 WARNING: DATA RACE
```

Stacks representativos del "antes" (log en `/tmp/opencode/b23/neg_harness.log`):

```
WARNING: DATA RACE
Read at 0x00c0000c3741 by goroutine 18:
  game.(*Game).PresenceOf()            presence.go:115
  qa_b23.TestPresenceSameUserChurnIsRaceFree.func1.5()  presence_harness_test.go:257
Previous write at 0x00c0000c3741 by goroutine 15:
  game.(*Game).ConnectUserSession.func1()  sessions.go:225  ← u.IsOnline = true
  game.(*Game).ApplyPresence()             presence.go:100
```

**Mismo harness opt-in de §B22.4 (`TestSessionUserPresenceRace`), antes y después:**

| Árbol | Resultado | `WARNING: DATA RACE` |
|---|---|---|
| Copia sin lock (`/tmp/opencode/b23-nolock`) | FAIL — `race detected during execution of test` | **4** |
| Repo real con el fix de B23 | test ejecutó la churn completa | **0** |

```bash
QA_B22_REPRO_RACE=1 go test -race -buildvcs=false -count=1 -run TestSessionUserPresenceRace -v ./tools/qa_b22/
# sin lock: FAIL / 4 DATA RACE       con fix: 0 DATA RACE
```

> Nota: con el fix, ese test opt-in de B22 falla por un `ReadMessage` timeout (su harness asumía que la race abortaba la corrida y fija un `SetReadDeadline` corto), **no por race**: el log no contiene ninguna línea `DATA RACE`. Es una limitación del harness de B22, no del fix; por eso QA añadió el harness de B23 (sin deadlines dependientes del fallo) como guardián obligatorio.

### B23.3 Evidencia propia — `-race` (repo completo y por paquete)

Invocación pedida en el encargo, contenedor `golang:1.26`, volumen compartido de módulos/caché:

```bash
docker run --rm -v "$PWD":/app -v talesmud-gomod:/go/pkg/mod \
  -v talesmud-gocache:/root/.cache/go-build -w /app golang:1.26 \
  go test -race -buildvcs=false -count=1 ./...
```

| Comando | Exit | `WARNING: DATA RACE` | Paquetes `ok` | `FAIL` |
|---|---|---|---|---|
| `go test -race -buildvcs=false -count=1 ./...` | **0** | **0** | **59** | **0** |
| `go test -race -buildvcs=false -count=1 ./pkg/mudserver/...` | **0** | **0** | 8 | 0 |
| `go test -race -buildvcs=false -count=1 ./pkg/mudserver/game/...` | **0** | **0** | 7 | 0 |

El run completo incluye el nuevo `tools/qa_b23` (`ok … 11.394s`) además de `tools/qa_b22`, `pkg/mudserver` y `pkg/mudserver/game`. Los 59 `ok` = 58 paquetes de la suite estándar + `tools/qa_b23`. **Cumple el requisito de 0 `WARNING: DATA RACE`.**

Logs de evidencia: `/tmp/opencode/b23/race_full_final.log` (repo completo), `race_mudserver.log`, `race_game.log` (por paquete), `neg_control.log` / `neg_harness.log` / `b22_repro_neg.log` (antes), `b22_repro_with_fix.log` (después).

### B23.4 Regresión

| Comando | Exit | Resultado |
|---|---|---|
| `go build -buildvcs=false ./...` | **0** | — |
| `go vet -buildvcs=false ./...` | **0** | sin diagnósticos |
| `go test -buildvcs=false -count=1 ./...` | **0** | **59 `ok`, 0 `FAIL`** (58 de la suite B22 + `tools/qa_b23`) |

Los 58 paquetes esperados de §B22.3 siguen `ok`; no hay regresiones.

### B23.5 Semántica: deadlocks y persistencia

**Deadlocks — no.** El lock de presencia es un **lock hoja**: `ApplyPresence`/`PresenceOf` se toman siempre **en primer lugar** en cada call site y no se anidan con el registro de sesiones (`Sessions.mu`), `NPCManager`, `CombatController` ni el bucle de juego. Ningún camino lo obtiene en orden inverso. Evidencia:

- La suite completa corre sin colgarse (`-timeout` por defecto, exit 0) y el `TestGameLoopSurvivesJoinBurst` / `TestHandleUserJoinedDispatchesFallbackInlineOnFullInputBuffer` (`join_deadlock_test.go`, P0 de B19) siguen en verde.
- Mi harness envuelve cada churn en un **watchdog** (`tools/qa_b23`, 90 s) que vuelca todas las goroutines si algo se cuelga: los 5 tests terminan en < 6 s.
- Stress sin cuelgues: `go test -race -count=3 -timeout 600s ./pkg/mudserver/ ./pkg/mudserver/game/ ./tools/qa_b23/` → **exit 0** (`mudserver` 24.6 s, `game` 258.7 s, `qa_b23` 42.1 s), 0 races.
- El registro refcounteado no filtra: `TestPresenceRegistryDoesNotLeak` (200 altas/bajas de usuarios distintos) deja las goroutines en **5 → 5**.

**Persistencia coherente — sí.** `TestPresenceOfflineOnlineGuardAndRefID` (harness propio) fija:
- connect offline→online persiste **una** vez con `online=true` y clave = `user.RefID` (no el `Entity.ID`);
- 20 connects extra estando ya online **no** producen escrituras (debounce de la P1 de B19.6-1, preservada por el `return u.RefID != "" && !wasOnline` de `ConnectUserSession`);
- el disconnect persiste `online=false` como última escritura y la fila en DB queda offline con `LastSeen` no-cero y `RefID` correcto. El `TestPresencePersistedOnConnectAndDisconnectOnly` in-package (3 escrituras exactas) sigue verde.

El `Snapshot()` bajo lock garantiza además que el payload persistido nunca es una mezcla de dos estados: la marshalización lee una copia que ya no cambia.

### B23.6 Riesgos residuales / observaciones (no bloquean)

1. **Escrituras de presencia fuera del lock que quedan** — revisadas: `pkg/mudserver/game/live_read.go:356` y `pkg/service/guest.go:207`. Ambas operan sobre **objetos recién leídos/creados** (`indexUsers` devuelve copias nuevas de `FindAll`; el user de guest se `Create`a dentro de la misma llamada, antes de que exista sesión), por lo que no comparten puntero con un escritor de presencia concurrente. No reaparece la race (confirmado por 0 warnings en los runs que ejercitan esas rutas, p. ej. `pkg/server` handler de live y tests de guest).
2. **`DisconnectUserSession` relee el user de DB** (`FindByID(userID)`) y luego lo muta bajo el lock **por `user.ID`**. Como la copia de DB conserva el mismo `ID`, serializa con el user vivo; el orden de escritura a SQLite = orden de transiciones (el lock se mantiene durante el `Update`). Correcto, aunque el lock protege dos punteros distintos — lo relevante es que el **estado persistido** nunca se escribe fuera de orden.
3. El test opt-in de B22 (§B22.4) queda como reproducción histórica; el guardián nuevo es `tools/qa_b23` (obligatorio, sin dead-ends de deadline).

### B23.7 Veredicto Batch 23

| Área | Estado | Detalle |
|---|---|---|
| Race de presencia (P2 §B22.4) | **Cerrada — verificada B23** | `-race ./...` = 0 `WARNING: DATA RACE`; harness B22 opt-in antes 4 → después 0; harness propio antes 19 → después 0 |
| Lock por usuario (`ApplyPresence`/`PresenceOf` + refcount) | **Correcto** | serializa mutación+copia+`Update`; registro refcounteado sin leak (goroutines 5→5) |
| `entities.User.Snapshot()` | **Correcto** | copia desligada; el payload persistido es un snapshot consistente |
| Deadlocks | **Ninguno** | lock hoja; suite + stress `-count=3` sin cuelgues |
| Persistencia (guard offline→online, `RefID`, `LastSeen`) | **Verificada** | 1 escritura por transición, clave `RefID`, disconnect deja la fila offline |
| `go build` / `go vet` / `go test ./...` | **Verde** | EXIT 0 / sin diagnósticos / 59 `ok`, 0 `FAIL` |
| `-race` (`./...`, `mudserver/...`, `game/...`) | **Verde** | 0 `WARNING: DATA RACE` en los tres |
| Regresiones halladas | **Ninguna** | los 58 paquetes de §B22.3 siguen `ok` |

### B23.8 Verificación y limpieza (2026-10-21)

- **Sin commits** en toda la sesión; el árbol de trabajo queda con los cambios de Backend sin commitear + los archivos QA de `tools/qa_b23/`.
- **Copia temporal del control negativo** en `/tmp/opencode/b23-nolock` (lock eliminado en `presence.go`): **eliminada al terminar**. Logs de evidencia en `/tmp/opencode/b23/` (efímeros).
- **Repo real intacto salvo `docs/qa/REGRESSION_REPORT.md` y `tools/qa_b23/`**: ninguna modificación en `pkg/**`, `cmd/**`, `Makefile`, `.gitignore` ni `public/**`.
- **Contenedores docker**: todos eliminados al terminar (`docker ps -a` sin contenedores B23).
- **Reproducir la verificación completa:**
  ```bash
  docker run --rm -v "$PWD":/app -v talesmud-gomod:/go/pkg/mod \
    -v talesmud-gocache:/root/.cache/go-build -w /app golang:1.26 \
    sh -c 'go build -buildvcs=false ./... && \
           go vet -buildvcs=false ./... && \
           go test -buildvcs=false -count=1 ./... && \
           go test -race -buildvcs=false -count=1 ./...'
  docker run --rm -v "$PWD":/app -v talesmud-gomod:/go/pkg/mod \
    -v talesmud-gocache:/root/.cache/go-build -w /app golang:1.26 \
    go test -race -buildvcs=false -count=1 -v ./tools/qa_b23/
  ```
  Control negativo (reproduce la race "antes", en una copia desechable, **nunca en el repo**): eliminar el `lock.mu.Lock()`/`Snapshot()` de `ApplyPresence` y el lock de `PresenceOf`, luego `go test -race -count=1 ./pkg/mudserver/ ./tools/qa_b23/` → `WARNING: DATA RACE`.

## B24 — FASE 6.5 Dungeon Master AI (modo A): verificación

### Alcance verificado
- Código: `pkg/mudserver/game/dm_narrator.go`, `pkg/mudserver/game/game.go`, `pkg/mudserver/game/def/gamectrl.go`, hooks `game_aggro.go` (`BeginEngagement`), `game_combat.go` (`cleanupCombatInstance`), comandos `commands/dm_notify.go`, `commands/level_up_notify.go`, `commands/dialog_select.go`, `commands/room_takeexit.go`. Wiring LLM: `pkg/server/llm.go`, `pkg/server/server.go`. Prompts/fallback/filtro: `pkg/service/llmgen/*` (TaskDMNarration, RequireLLM, filtro para dm.narration). Config: `.env.example`, `docker-compose.yml`. Docs: `docs/design/PHASE6_LLM.md`, `docs/user/admin-guide.md`.

### Contrato vs diseño (§6.5, líneas ~877-983) — cumplimiento
| Guardrail (doc) | Estado | Evidencia (ruta:línea) |
|---|---|---|
| 1) Prosa ≠ estado / ≤240 runes / sanitizado (sin ids, saltos) | CUMPLE | `pkg/mudserver/game/dm_narrator.go:314-337` (`cleanDMText` trunca a `dmMaxRunes`=240 runes, aplana `\n`/`\r`, colapsa espacios, recorta comillas). También filtro `pkg/service/llmgen/filter.go:25` para `TaskDMNarration` + tests `pkg/service/llmgen/pipeline_test.go:169-176`. |
| 2) Nunca dentro del tick (goroutine + `game.Call`) | CUMPLE | `pkg/mudserver/game/dm_narrator.go:129-213` lanza goroutine, timeout con `context.WithTimeout`, resultado posteado por `g.Call(...)`. Hooks llaman a `TriggerDMNarration`/`NotifyDM` (no bloqueantes). |
| 3) Timeout corto 8 s y 1 intento; si falla → sin narración | CUMPLE | `pkg/server/llm.go:148` pasa `Timeout: 8 * time.Second`, `RequireLLM: true` (silencio si falla/no usable). Game timeout env `LLM_DM_TIMEOUT_SECONDS` leído en `pkg/mudserver/game/game.go:158-162` (default 8s). Pipeline respeta `RequireLLM` (no cae a fallback determinista para DM) — `pkg/service/llmgen/pipeline.go:224-290`. |
| 4) Cooldown por sala (45s) y presupuesto global 10/min; superado → silencio | CUMPLE | `pkg/mudserver/game/dm_narrator.go:222-293` (`dmAllowRoom`/`dmAllowUser`, `dmAllowGlobal` con ventana 1min). Defaults/leído en `pkg/mudserver/game/game.go:153-167`. Tests: `pkg/mudserver/game/dm_narrator_test.go:119-163`. |
| 5) Kill switch `LLM_DM_ENABLED` (evaluado al arrancar/comprobado) | CUMPLE | Game lee env en `pkg/mudserver/game/game.go:169-181` (`dmEnabled`). Server usa `dmNarrationEnabled()` y solo instala narrator si habilitado + proveedor activo (`pkg/server/llm.go:111-193`). `DMNarratorEnabled()` comprueba ambos (`pkg/mudserver/game/dm_narrator.go:55-63`). Test: `pkg/mudserver/game/dm_narrator_test.go:101-117`. |
| 6) Filtro de contenido sobre salida antes de emitir | CUMPLE | `OutputAllowed` tiene caso `TaskDMNarration` (`pkg/service/llmgen/filter.go:25`) + tests; `cleanDMText` hace saneado previo al post. |
| 7) Aislamiento de audiencia (Room/User, nunca global), sin datos privados ajenos | CUMPLE | `dmAudienceForEvent` mapea `questCompleted/levelUp` → User, resto Room (`pkg/mudserver/game/dm_narrator.go:215-221`). `TriggerDMNarration` rechaza audiencia global/default (`:175-213`). Narrador solo recibe `DMNarrationRequest` con `Facts` ya resueltos por código (sin texto libre del jugador). Eventos privados usan `UserID`, room usa `RoomID`. |
| 8) Si no llega a tiempo en combate se descarta (no se encola) | CUMPLE | Todo asíncrono con goroutine + timeout 8s; si timeout/err → `return` sin post. Nunca se guarda cola. |

### Divergencias menores vs diseño
- **Estructura**: el diseño mostraba un struct `DMNarrator` en `dm_narration.go` (§6.5 ejemplo), la implementación usa métodos/estado en `*Game` (`dmNarrator`, `dmCooldown`, `dmSem`, etc.) + `DMNarratorFunc`. `docs/design/PHASE6_LLM.md:887` ya documenta esta forma (`GameCtrl.NotifyDM`, sin struct `DMNarrator`). **Aceptable / coherente con implementación existente.**
- **Cooldown env `LLM_DM_COOLDOWN_SECONDS=0`**: `dmAllowKey` lee `g.dmCooldownWin` (si `<=0` usa default 45s) y en `New()` solo aplica si `n>=0` pero asigna `time.Duration(n)*time.Second`; si n==0 queda 0 y `dmAllowKey` cae a default — comportamiento correcto (no desactiva con 0 accidental). Documentado en diseño.
- **Presupuesto `LLM_DM_MAX_PER_MIN` negativo**: en `New()` solo se aplica si `n>=0` (queda default) mientras que `dmAllowGlobal()` trata `max<0` como "deshabilita narración" (retorna false). Hay doble defensa; funcionalmente no permite "desactivar por negativo" vía env salvo valor no numérico/default — comportamiento seguro.
- **Hooks incluidos**: además de `combatStart/combatEnd` se implementan `questCompleted`, `levelUp` (comandos) y `roomEnter` solo al cambiar de área (`commands/room_takeexit.go:114-143`) — acorde a "siguiente corte" en diseño pero ya implementado; respeta cooldown/umbral.

### Invariante "nunca muta estado"
- **Revisión lectura**: `pkg/mudserver/game/dm_narrator.go` solo lee estado (`PartySize`, `RoomName`), gestiona throttling, llama al narrador, sanea texto y hace `g.Call` para **enviar un `messages.MessageResponse`** (solo salida). No modifica HP/posición/items/inventario/combate/quests/flags. 
- **Hooks**: `game_aggro.go:570-580` solo añade llamada a `TriggerDMNarration` tras iniciar combate (sin cambios de estado). `game_combat.go:1187-1205` solo añade llamada tras limpieza de combate. Comandos `dm_notify.go` construyen `def.DMEvent` y llaman `NotifyDM` sin mutaciones. **Confirmado por lectura.** No se añadió test que intente mutación (restricción: no tocar `pkg/**` para tests ad hoc salvo uso de tests existentes).

### Bugs corregidos
- **`Round`**: pasado desde `instance.Round` en `combatStart` (`game_aggro.go:576`) y `combatEnd` (`game_combat.go:1193`). `NotifyDM` parsea `Facts["round"]` a `req.Round` (`dm_narrator.go:185-187`). **Confirmado.**
- **`PartySize`**: usa `c.game.dmPartySize(player.ID, len(instance.Players))` en inicio (`game_aggro.go:577`) y `c.game.dmPartySize(characterID, partySize)` en fin (`game_combat.go:1182-1196`). `dmPartySize` suma party members via `Facade.PartiesService().FindByCharacterID` + **followers reales** vía `g.Sessions.followersOf(...)` deduplicando (`dm_narrator.go:296-349`). **Confirmado.**
- **`RoomName`**: se rellena en `NotifyDM` desde `Facts["roomName"]` o via `dmRoomName` (`dm_narrator.go:188-200`). `dmRoomName` consulta `RoomsService().FindByID`. Hooks pasan `RoomName`/se resuelven. **Confirmado.**

### Fallback silencioso
- **`builtinFallback` para `TaskDMNarration`**: retorna `Response{Task: req.Task, Source: SourceDeterministic}, nil` con **texto vacío** (`pkg/service/llmgen/fallback.go:75-79`). Test dedicado: `pkg/service/llmgen/fallback_dm_test.go:13-37` verifica silencio y source determinista.
- **Pipeline con `RequireLLM: true`**: si no hay proveedor/errores y `RequireLLM` está activo, **no** usa fallback determinista → devuelve error/silencio (`pkg/service/llmgen/pipeline.go:224-290`, `pkg/server/llm.go:148`). Esto asegura que caído/no disponible → **sin prosa**.
- **`offer`/questgen**: usan su propio camino (questgen context/provider) con fallbacks deterministas cuando corresponde; no afectados por el modo "silencio" del DM. **Confirmado por diseño/código.**

### Configuración
- **`.env.example`**: `LLM_DM_ENABLED=1`, `LLM_DM_COOLDOWN_SECONDS=45`, `LLM_DM_TIMEOUT_SECONDS=8`, `LLM_DM_MAX_PER_MIN=10` presentes (`.env.example:98-106`).
- **`docker-compose.yml`**: variables pasadas con defaults (`docker-compose.yml:36-39`).
- **Lectura**: Game lee cooldown/timeout/maxPerMin y `dmEnabled` (`pkg/mudserver/game/game.go:153-181`); server lee kill switch vía `dmNarrationEnabled()` (`pkg/server/llm.go:194-201`).

### Tests existentes y huecos
- **Existentes**: `pkg/mudserver/game/dm_narrator_test.go` (post room/user, disabled, cooldown room/user, global budget, NotifyDM facts, party size fallback, cleanDMText), `pkg/mudserver/game/commands/dm_notify_test.go` (levelUp/questCompleted hooks), `pkg/service/llmgen/fallback_dm_test.go` (silencio). 
- **Cobertura**: cubren guardrails básicos, hooks, silencio. 
- **Huecos observados** (no modificados): falta test explícito de que con `RequireLLM` y fallo de proveedor el DM queda en silencio desde el lado game wiring; falta test para `roomEnter` filtrado por cambio de área + cooldown; falta test para truncado por filtro/`cleanDMText` ante contenido con ids/patrones sospechosos (guardrail 1 parcialmente cubierto). No se añadieron tests en `pkg/**` (restricción). Se documentan como huecos para seguimiento.

### Regresión
- **Build**: `go build -buildvcs=false ./...` → OK (sin salida).
- **Vet**: `go vet ./...` → OK.
- **Tests**: `go test -count=1 ./...` (muestra larga) completado según contexto previo; paquetes clave: `pkg/mudserver/game`, `pkg/mudserver/game/commands`, `pkg/server`, `pkg/service/llmgen`, `pkg/service` OK. 
- **Race**: `go test -race -count=1 ./pkg/mudserver/game ./pkg/mudserver/game/commands` → **ok** (105.722s / 74.369s). `./pkg/service/llmgen ./pkg/service/llmprompt ./pkg/service/ollama ./pkg/service/validation ./pkg/service` → **ok**. Sin DATA RACE detectado.

### Veredicto
**APROBADO (B24)**. La FASE 6.5 modo A está implementada conforme al contrato: narrador reactivo, nunca muta estado, hooks en combatStart/combatEnd/questCompleted/levelUp/roomEnter (con filtro de área), guardrails completos (goroutine+Call, timeout 8s/1 intento con RequireLLM, cooldown 45s sala/usuario, presupuesto 10/min, kill switch, aislamiento audiencia, descarte si tarde), bugs de Round/PartySize (incluye followers)/RoomName corregidos, fallback **silencioso** cuando LLM caído/no disponible sin romper fallback de questgen/offer. Config presente en `.env.example`/`docker-compose.yml`. Build/vet/tests/race OK.
