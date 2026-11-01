# Referencia de comandos del fork

Guía breve de referencia: **solo comandos que existen hoy en el código**. Cada fila se verificó con
`grep` sobre el registro de comandos (`pkg/mudserver/game/commands/commandprocessor.go`) y su archivo
de implementación el **2026-10-15**; el bloque `ask` (sección 5) se añadió y verificó el
**2026-10-22** (Batch 25 — FASE 6.3 corte 1), se amplió con el **2026-10-26** (Batch 26 — corte 2:
personalidad Big5, secretos anti-fuga, resumen LLM y config) y volvió a ampliarse el
**2026-10-30** (Batch 27 — corte 3: opciones LLM accionables sobre la conversación `__llm__`,
relaciones dinámicas y feedback de relación en el cliente) y se actualizó el **2026-11-01**
(Batch 29 — auto-saludo con `talk`, continuación por texto libre y opciones `quest:<id>` validadas).

> Implementación: `pkg/mudserver/game/commands/talents.go`, `prestige.go`, `party.go`,
> `party_loot_ready.go`, `channels.go`, `ask.go` (registro en `commandprocessor.go`),
> `llm_dialog.go` (opciones numéricas y texto libre de `__llm__`, Batches 27/29) y `dialog_select.go`
> (enruta `DialogID == "__llm__"` a ese manejador).

Para el catálogo completo del motor (combate, inventario, movimiento, mercaderes, quests…):
[`docs/player-guide/03-commands.md`](../player-guide/03-commands.md) y el comando `help` en juego.
Guías por sistema: [`channels.md`](channels.md), [`party.md`](party.md), [`clans.md`](clans.md),
[`dms.md`](dms.md).

---

## 1. Progresión — `talents` y `prestige`

| Comando | Alias | Qué hace |
|---|---|---|
| `talents` · `talents list` | `talent` | Puntos de talento sin gastar + talentos de tu clase (disponibles / no disponibles) |
| `talents spend <talent>` | — | Compra 1 rango de un talento que tu clase ofrece (consume 1 punto) |
| `prestige` | — | Estado: nivel actual, prestige actual y si ya puedes subir de prestige |
| `prestige confirm` | — | **Irreversible**: te deja a nivel 1 y te da +5 % XP permanente y +1 punto de talento por cada prestige |

Notas verificadas:

- Se gana **1 punto de talento cada 5 niveles** (`TalentMilestoneInterval = 5` en
  `pkg/mudserver/game/leveling/leveling.go`); el mensaje de level-up te lo recuerda.
- `prestige confirm` exige **nivel 50** (`leveling.MaxLevel`), no estar en combate y tu confirmación
  explícita. Conservas talents, oro, equipo, descubrimientos y amigos.
- Diseño completo: [`docs/design/PHASE2_4_PROGRESSION.md`](../design/PHASE2_4_PROGRESSION.md).

## 2. Party — botín y comprobación de disponibilidad

Subcomandos de `party` (alias `p`); detalle completo en [`party.md`](party.md).

| Comando | Quién | Qué hace |
|---|---|---|
| `party loot` | cualquiera de la party | Consulta la regla de botín vigente (`free` por defecto) |
| `party loot <regla>` | **solo leader** | Cambia la regla: `free`, `leader`, `roundrobin`, `needgreed` (también `need/greed`) |
| `party readycheck` | **solo leader** | Inicia una comprobación de disponibilidad: `[Party] Ready check!` |
| `party ready` | cualquiera de la party | Marca tu respuesta como lista |
| `party notready` | cualquiera de la party | Marca tu respuesta como no lista |

Notas verificadas:

- La comprobación **caduca a los 30 segundos** (`sessions_ready.go`: `timeout = 30 * time.Second`) y
  al cerrar se resumen los aciertos (`Ready check over: N/M ready.`).
- Cada respuesta actualiza a toda la party (`Ready: 1/3 · Not ready: 0/3`).

## 3. Canales globales — `chat` y sus atajos

| Canal | Con prefijo | Atajo directo |
|---|---|---|
| `general` | `chat general hola` | `general hola` ✅ |
| `trade` | `chat trade vendo hierro` | ❌ solo con `chat` |
| `lfg` | `chat lfg falta sanador` | `lfg falta sanador` ✅ |
| `help` | `chat help ¿cómo hago X?` | ❌ solo con `chat` |
| `roleplay` | `chat roleplay mira la lluvia` | `roleplay mira la lluvia` ✅ |
| `admin` | `chat admin aviso` | `admin aviso` ✅ (solo administradores) |

`trade` y `help` **no** tienen atajo: esos nombres ya son otros comandos (inventario del mercader y
ayuda). Un mensaje cuya primera palabra no sea un canal conocido no lo intercepta este comando: lo
procesa la charla de sala. Detalle y pestañas del cliente en [`channels.md`](channels.md).

## 4. Quests contextuales — `offer`

Comando nuevo (Batch 18), implementado en `pkg/mudserver/game/commands/offer.go`
(registro en `commandprocessor.go`).

| Comando | Qué hace |
|---|---|
| `offer <npc>` | Pide a un NPC quest-giver de la sala una quest contextual (nivel + área actuales); la generación corre fuera del bucle y te responde después |
| `offer accept` | Acepta la oferta: guarda la quest, muestra `questAccepted` y refresca el quest log |
| `offer decline` (alias `no`, `reject`, `pass`) | Descarta la oferta |

Notas verificadas:

- Solo responde el NPC quest-giver de la sala (mismo contrato `IsQuestGiver` que la UI).
- Cooldown de 60 s por personaje; si no hay proveedor LLM instalado o está
  deshabilitado (`LLM_PROVIDER=none` sin fallback) responde que no hay trabajo.
- La quest **no se persiste hasta** `offer accept`.

## 5. Diálogo libre con NPCs — `ask`

Comando nuevo (Batch 25, FASE 6.3 corte 1; ampliado en los Batches 26–27 y en el Batch 29 con texto
libre y opciones de quest), implementado en
`pkg/mudserver/game/commands/ask.go` (registro en `commandprocessor.go`, **sin alias**).

| Comando | Qué hace |
|---|---|
| `ask <npc>` | Saludo: el NPC LLM te responde en persona (sin pregunta) |
| `ask <npc> [pregunta]` | Pregunta libre; el NPC responde en carácter con su personalidad (Big5/`persona`/`voice`) |
| `ask to <npc> [pregunta]` | Igual (la preposición `to` se ignora) |
| `talk <npc>` | Diálogo autoral primero, quests disponibles después; si faltan ambos y `llm.enabled` está activo, inicia saludo LLM |

Notas verificadas:

- **Solo NPCs con `llm.enabled`.** Si el NPC no lo tiene (o no hay proveedor LLM instalado) te avisa
  — *"<name> has no free-form voice; you speak with them the usual way."* — y sigue el flujo
  **autoral** de `talk` (mismos diálogos y quests). Nadie se queda sin responder. Con el content
  pack de aventura importado, **Master Aldric** (`TUT_MENTOR`) viene con `llm.enabled: true`, así
  que `ask Master Aldric ¿qué debo saber?` en la sala R1001 ejercita el camino completo.
- **Cadencia:** cooldown por personaje+NPC (`llm.cooldownSeconds`, 3 s por defecto), máximo
  **2** generaciones en paralelo en el servidor y timeout de **20 s**; mientras piensa verás
  *"<name> considers your words..."* y la respuesta llega después como un mensaje del NPC
  (`Username` = nombre del NPC), solo para ti.
- **Personalidad (corte 2):** el prompt del NPC incluye su configuración estructurada —
  rasgos Big5 (`llm.personality`), objetivos (`llm.goals`), secretos (`llm.secrets`) y relaciones
  (`llm.relations`, p. ej. `player(mentor,0.70)`). La plantilla del prompt **ordena no revelar los
  secretos literalmente** (solo indirectas, y solo con alta confianza); si el modelo reproduce un
  secreto palabra por palabra en la respuesta, en una opción o en una nota de memoria, la respuesta
  se descarta y se reintenta — nunca verás el secreto tal cual.
- **Memoria:** cada turno se guarda en la tabla `npc_memories` (clave `<npcTemplateID>::<characterID>`)
  y la siguiente pregunta lleva ese contexto al prompt (resumen + últimos hechos + tu última
  pregunta). El resumen de base es determinista (hechos + última pregunta); a partir de 8 turnos
  el servidor genera además un **resumen con LLM** (asíncrono) y lo preserva como resumen de
  largo plazo (procedencia marcada con `summarySource`).
- **Opciones accionables (corte 3):** cuando el modelo devuelve opciones (máx. 4), el servidor las
  convierte en una **conversación real** con el `DialogID` centinela `__llm__` y te las muestra como
  una lista numerada (el overlay del diálogo o el terminal; eliges escribiendo/clicando el número).
  Las opciones LLM admiten estas acciones:
  1. **`end`** — cierra la conversación (*"<name> nods, and the conversation comes to a close."*).
  2. **Etiqueta que coincide con un nodo del diálogo autoral** — continúa el árbol escrito en ese
     nodo vía `talkToNPCAt` (la conversación pasa del centinela `__llm__` al `DialogID` real del
     NPC, con lo que las quests y los textos autorales vuelven a mandar). El mapeo compara la
     etiqueta con nombre/texto/textos alternativos/opciones de los nodos del diálogo del NPC
     (`matchAuthoredDialogLabel`).
  3. **`flavor` (seguimiento)** — el NPC vuelve a responder a la línea elegida: la petición sale con
     `Continuation=true`, que **salta el rechazo por cooldown** (aún refresca la ventana, para que no
     sirva de atajo al siguiente `ask` directo) y respeta el semáforo de 2.
  4. **`quest:<questId>`** — ejecuta una acción de quest que ya esté disponible para ese personaje
     y NPC (aceptar, entregar o consultar progreso, según corresponda). La opción se valida con la
     lista contextual enviada al modelo y vuelve a comprobarse al elegirla; el modelo no puede crear
     quests. Si ya no está disponible o el ID no existe, informa sin cambiar el estado de quest.
  Un número fuera de rango responde *"Invalid option. Please choose 1-N"* y **mantiene** las opciones;
  si la conversación ya se cerró, el número se ignora (una sesión NPC abierta no la reactiva).
- **Relaciones dinámicas (corte 3):** cada intercambio puede mover un poco la actitud del NPC hacia
  ti. El modelo sugiere un `relationDelta` acotado a **±0.05** por turno (si lo omite, +0.02 salvo
  que la opción sea `end`), el acumulado se guarda en la fila `npc_memories` (`relations`, objetivo
  `player`) **clampado a [-1, 1]** y la siguiente generación lo inyecta en el prompt como
  `player(dinámica,0.04)`. Es solo contexto de rol: **no es una stat del juego** y no abre ni cierra
  contenido por sí solo.
- **Feedback en el cliente (corte 3):** si el mensaje trae información de relación, el cliente pinta
  una línea sutil — en el overlay del diálogo una fila con ▲/▼/• y en el terminal entre corchetes
  (`[Master Aldric's regard for you warms.]`, si el modelo/sub-productor marca la dirección `up`). **Solo se pinta si el mensaje lo trae**: no hay nada que ver
  en un saludo normal (`relationFeedback.js` + `DialogOverlay.svelte`).
- **`talk` con NPC LLM (Batch 29):** el orden de selección no cambia: diálogo autoral → quests
  disponibles → saludo LLM. El saludo automático solo se programa si el NPC tiene `llm.enabled` y
  faltan las dos opciones previas. Si el proveedor está deshabilitado, se muestra el mensaje actual de
  NPC sin diálogo.
- **Texto libre durante la conversación (Batch 29):** con el diálogo `__llm__` activo, el overlay
  muestra un campo de texto que envía el contenido crudo, sin comando `ask`. También se intercepta
  entrada normal no reconocida si el NPC LLM sigue en la sala y la sesión dialog está abierta o su
  última interacción fue hace menos de 60 s. La petición se envía como continuación; al programarla
  se cierra la conversación anterior. Si está limitada (`RateLimited`), ocupada (`Busy`) o no hay
  proveedor (`Disabled`), las opciones/sesión no se cierran.
- **Alcance de seguridad:** el texto libre requiere un NPC LLM vivo en la misma sala, además de una
  sesión dialog abierta o reciente. Es un guardrail básico; no equivale a bloquear toda interacción
  con NPCs en combate. Generar opciones no crea ni muta quests; solo al elegir una opción válida se
  ejecuta la acción existente de quest, con su efecto de juego correspondiente.
- **Probarlo en el mundo:** con el content pack de aventura importado, **Master Aldric**
  (`TUT_MENTOR`, sala R1001) viene con `llm.enabled: true` **y** con el diálogo autoral
   `DLG_TUT_MENTOR`, así que `talk` conserva el diálogo autoral y quests antes de cualquier saludo
   LLM; `ask Master Aldric …` seguido de una opción cuya etiqueta coincida con un nodo de ese diálogo
   ejercita el camino `__llm__` → nodo autoral.
- **Config del NPC:** campo `llm` del JSON del NPC (`enabled`, `persona`, `voice`, `temperature`,
  `cooldownSeconds`, `memoryTurns`, `personality`, `goals`, `secrets`, `relations`). Tres vías:
  YAML del content pack (importer), plantilla de NPC por REST (`/api/npc-templates`, para
  creators) y el panel **"Dialogue / LLM"** del NPC factory del cliente (tecla `N`). Los NPCs
  creados por spawner heredan la config de la plantilla. Diseño y deuda:
  [`docs/design/PHASE6_LLM.md`](../design/PHASE6_LLM.md) §6.3.

## 6. Comandos que **no** existen hoy

Verificado con `grep` en `pkg/mudserver/game/commands/` (no documentar hasta que aparezcan):

- **`clan …`** — no hay `ClansCommand` ni clave `"clan"` registrada; los clanes se gestionan por
  REST + panel (tecla `C`). Ver [`clans.md`](clans.md).
- **`tell` persistente** — el comando `tell` existe, pero sigue **en memoria** (solo jugadores
  online, sin guardar nada). Para mensajes guardados usa el panel de DMs (tecla `M`); ver
  [`dms.md`](dms.md).
