# Clanes

Los clanes son agrupaciones persistentes de personajes con roles, banco y
membresía única. Todo se gestiona desde el **panel de clan** (interfaz gráfica);
**no existe ningún comando de juego `clan …`**.

> Implementación: `pkg/service/clans.go` + `pkg/repository/clans_sqlite.go`
> (tablas `clans`, `clan_members`, `clan_invites`, `clan_bank_ledger`),
> `pkg/server/handler/clans.go` y `public/mud-client/src/game/ui/ClanPanel.svelte`.

---

## 1. Abrir el panel

| Cómo | Detalle |
|---|---|
| **Tecla `C`** | Alterna el panel. Solo funciona si no hay otro panel encima y no estás escribiendo en la consola. |
| **Botón de cabecera** | Icono de escudo, con la ayuda *Clan (C)*, en la barra superior del game client. |
| **Cerrar** | `Escape`, la `×` de la esquina o clic fuera del panel. |

Al abrir, el panel carga tu estado (`GET /api/clans/mine`) y hay un botón
**Refresh** para recargarlo. **No hay sondeo ni notificaciones push**: las
invitaciones nuevas solo aparecen cuando abres o refrescas el panel.

> Las sesiones de **invitado** no ven el botón de cabecera; la tecla `C` sigue
> respondiendo, pero para gestionar un clan necesitas una cuenta registrada.

---

## 2. Crear un clan

Sin clan, el panel muestra un formulario con tres campos y el botón dorado
**Create**:

| Campo | Obligatorio | Límite (interfaz) |
|---|---|---|
| **Clan name** | sí | 40 caracteres |
| **Tag** | sí | 5 caracteres (p. ej. `DRK`) |
| **Description** | no | 160 caracteres |

Reglas al crear:

* El nombre y la etiqueta son obligatorios también en el servidor.
* Tu personaje **no puede pertenecer ya a otro clan** (`character is already in a
  clan`).
* Quien crea el clan pasa a ser **leader** automáticamente.
* El clan nace con `level 1`, `xp 0`, `bank 0` y `maxMembers 50`.

---

## 3. Unirse a un clan

1. Un **leader u officer** introduce tu **character id** en el campo
   *Invite by character id* y pulsa **Invite**.
2. Tú abres el panel (`C`): si no tienes clan, verás la sección
   **Pending invitations** con el clan y quién lo invitó.
3. Pulsa **Accept** (entras como `member`) o **Decline**.

Restricciones del servidor:

* No puedes invitar a tu propio personaje ni a quien ya está en un clan.
* Solo puede haber **una invitación pendiente** por (clan, personaje).
* Un personaje solo puede pertenecer a **un clan a la vez** (se comprueba al
  crear, invitar y aceptar).
* La invitación guarda un `expiresAt` de **7 días**, pero hoy **no se aplica**:
  sigue `pending` hasta que la aceptes o la rechaces (no hay estado `expired`
  automático).

---

## 4. Roles y permisos

Hay tres rangos (`member`, `officer`, `leader`):

| Acción | member | officer | leader |
|---|:-:|:-:|:-:|
| Ver miembros y banco | ✅ | ✅ | ✅ |
| **Invitar** a otro personaje | ❌ | ✅ | ✅ |
| **Expulsar** (kick) a un miembro | ❌ | ✅ | ✅ |
| **Promover / degradar** rangos | ❌ | ❌ | ✅ |
| **Deposit** en el banco | ✅ | ✅ | ✅ |
| **Withdraw** del banco | ❌ | ❌ | ✅ |
| **Disband** (disolver el clan) | ❌ | ❌ | ✅ |

Garantías del servidor:

* **El leader no puede ser expulsado** por nadie (`cannot kick the clan leader`).
* **Nadie puede expulsarse a sí mismo** (`cannot kick yourself`).
* **El leader no se puede degradar a sí mismo** (`the leader cannot demote
  themselves`).
* Los oficiales solo pueden actuar sobre rangos `member` **en la interfaz**; el
  servidor, al expulsar, solo protege al leader (un officer podría expulsar a
  otro officer llamando a la API).
* El leader puede promover a cualquier rango, incluido `leader`; el sistema no
  degrada automáticamente al leader anterior, así que pueden coexistir dos
  `leader`.

---

## 5. Banco del clan

* **Deposit**: cualquier miembro, importe **positivo**. El saldo sube y se
  registra una línea en el libro (`clan_bank_ledger`).
* **Withdraw**: solo el **leader**, importe **positivo** y sin superar el saldo
  (`insufficient clan bank balance`).
* Ambas acciones devuelven el saldo nuevo; el panel lo muestra en la cabecera
  como `Bank <n>g`.

> **Nota:** el banco es un **contador propio del clan** con su libro de
> movimientos. Hoy **no descuenta ni abona el oro del personaje**: depositar o
> retirar solo mueve el saldo del clan. El campo `contribution` del miembro
> existe en el modelo pero aún no se incrementa.

---

## 6. Disolver el clan

Solo el **leader**, con el botón rojo **Disband** (la interfaz pide
confirmación). Se eliminan todos los miembros y el clan deja de existir
(`active` → borrado). No hay deshacer.

> **Limitación actual:** un miembro normal **no puede abandonar su clan** por su
> cuenta: la única salida es que el leader lo expulse o que disuelva el clan.

---

## 7. Botones del panel

| Estado del panel | Botón / control | Quién puede usarlo |
|---|---|---|
| Sin clan | **Create** (formulario name/tag/description) | cualquier personaje sin clan |
| Sin clan | **Accept** / **Decline** en *Pending invitations* | el invitado |
| Con clan | **Promote** / **Demote** (por fila) | leader |
| Con clan | **Kick** (por fila, con confirmación) | leader u officer |
| Con clan | **Invite** (campo *Invite by character id*) | leader u officer |
| Con clan | **Deposit** (campo numérico + botón dorado) | cualquier miembro |
| Con clan | **Withdraw** | solo leader |
| Con clan | **Disband** (con confirmación) | solo leader |
| Siempre | **Refresh**, `Escape`, `×` | cualquiera |

Cada fila de miembro muestra el nombre (con la etiqueta **You** para ti), su
`contribution`, el distintivo de rango y las acciones que te corresponden.

---

## 8. Errores frecuentes

| Mensaje | Causa |
|---|---|
| `character is already in a clan` | ya formas parte de otro clan |
| `clan name is required` / `clan tag is required` | faltan nombre o etiqueta |
| `insufficient rank to invite` / `to kick` | no eres leader ni officer |
| `only the leader can change ranks` / `withdraw` / `disband` | rango insuficiente |
| `cannot kick the clan leader` | intentas expulsar al leader |
| `pending invite already exists` | ya había una invitación abierta |
| `target is already in a clan` | el invitado ya tiene clan |
| `deposit amount must be positive` / `withdraw amount must be positive` | importe ≤ 0 |
| `insufficient clan bank balance` | el retiro supera el saldo del clan |
| `Select a character before managing a clan.` | no has elegido personaje |

---

## 9. Referencia API (equipo)

Todas las rutas están bajo `/api/`, requieren `Authorization: Bearer` y
identifican al personaje que actúa con `characterId` (query string en GET, body
en POST). Registradas en `pkg/server/server.go`.

| Método | Ruta | Body / query |
|---|---|---|
| `GET` | `/api/clans/mine` | `?characterId=` |
| `POST` | `/api/clans` | `{characterId, name, tag, description}` |
| `POST` | `/api/clans/:id/invite` | `{characterId, targetCharacterId}` |
| `POST` | `/api/clans/invites/:inviteId/accept` | `{characterId}` |
| `POST` | `/api/clans/invites/:inviteId/decline` | `{characterId}` |
| `POST` | `/api/clans/:id/kick` | `{characterId, targetCharacterId}` |
| `POST` | `/api/clans/:id/rank` | `{characterId, targetCharacterId, rank}` (`member`\|`officer`\|`leader`) |
| `POST` | `/api/clans/:id/deposit` | `{characterId, amount}` |
| `POST` | `/api/clans/:id/withdraw` | `{characterId, amount}` |
| `POST` | `/api/clans/:id/disband` | `{characterId}` |

Respuestas de estado: `200` OK · `400` validación · `401/403` sin permiso ·
`404` clan/personaje · `409` conflicto (ya existe / invitación pendiente).

> No hay endpoint de listado público de clanes ni de historial del banco
> (`GetBankLedger` existe en la capa de servicio pero no está expuesto).

### Cómo obtener un `characterId`

La interfaz invita y escribe DMs **por id de personaje**, no por nombre.

* Los tuyos: `GET /api/my-characters` (o `GET /api/characters`) con tu token.
* El de otra persona: **no hay endpoint de búsqueda por nombre**. El panel solo
  muestra el nombre de los miembros que están en tu sala (usa la lista de
  presencia del cliente); si no están a la vista, la fila muestra un id
  abreviado. La vía práctica es pedir el id por fuera (o mirarlo con un token de
  admin en `GET /api/characters`, que lista todos los personajes).
