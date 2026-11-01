# Mensajería privada (DMs)

Las DMs (*direct messages*) son conversaciones **privadas y persistentes** entre
dos personajes. A diferencia de `tell`, el historial se guarda en base de datos
y sobrevive a la desconexión.

> Implementación: `pkg/service/dms.go` + `pkg/repository/dm_sqlite.go`
> (tablas `dm_channels`, `dm_messages`), `pkg/server/handler/dms.go` y
> `public/mud-client/src/game/ui/DMsPanel.svelte`.

---

## 1. Abrir el panel

| Cómo | Detalle |
|---|---|
| **Tecla `M`** | Alterna el panel (si no hay otro panel encima y no estás escribiendo en la consola). |
| **Botón de cabecera** | Icono de correo, con la ayuda *Messages (M)*. |
| **Cerrar** | `Escape`, la `×` o clic fuera del panel. |

El panel se organiza en tres zonas:

* **Izquierda**: lista de conversaciones (nombre del otro personaje y badge
  amarillo con el contador de no leídos).
* **Centro**: hilo con los mensajes (los tuyos alineados a la derecha, como
  **You**) y el campo de redacción con el botón dorado **Send**.
* **Inferior**: campo *New message — character id* con el botón **New** para
  abrir una conversación nueva. En la cabecera hay además un botón **Refresh**.

> Las sesiones de **invitado** no ven el botón de cabecera; la tecla `M` sigue
> respondiendo.

---

## 2. Iniciar una conversación

1. Escribe el **character id** del otro personaje en el campo inferior
   (*New message — character id*, máximo 64 caracteres).
2. Pulsa **New**.
3. El servidor crea **o reutiliza** el canal de la pareja: para la misma pareja
   de personajes **siempre existe un único canal** (*find-or-create*), así que
   repetir la operación no duplica conversaciones.
4. El canal se selecciona solo y puedes empezar a escribir.

Restricciones:

* No puedes escribirte a ti mismo (`cannot dm self`).
* El id debe corresponder a un personaje existente (`target character not found`).
* No hace falta que el otro esté conectado: los mensajes quedan guardados.

---

## 3. No leídos

* Cada mensaje que llega **incrementa el contador** del receptor
  (`unreadA`/`unreadB` del canal).
* El contador se muestra como badge junto al nombre de la conversación.
* Al seleccionar o refrescar un canal se envía `POST …/read`, que:
  * pone el contador a **0**,
  * marca como leídos los mensajes que **no has escrito** (`readAt`).
* No hay badge de no leídos fuera del panel: si lo tienes cerrado, no verás que
  hay mensajes nuevos hasta abrirlo.

---

## 4. Actualización y límites

| Aspecto | Valor |
|---|---|
| Refresco automático | **sondeo cada 5 s** mientras el panel está abierto; se detiene al cerrarlo |
| Refresco manual | botón **Refresh** |
| Push por WebSocket | **no existe aún** (las DMs no viajan por `/ws`) |
| Longitud del mensaje | cliente: `maxlength="1000"`; servidor: **no vacío** (sin límite de servidor) |
| Mensajes por petición | por defecto `50`; el panel pide `100`; el servidor acepta como máximo `500` |
| Filtro incremental | parámetro `since` (marca de tiempo **RFC3339**) |
| Orden | canal: por `lastMessageAt` descendente; mensajes: del más antiguo al más reciente |
| Persistencia | SQLite (`dm_messages` con índice por canal y fecha) |
| Seguridad | solo los dos participantes pueden leer o escribir el canal (anti-spoof) |

> Los mensajes no se editan ni se borran desde la interfaz.

---

## 5. DMs vs. `tell`

| | `tell` / `whisper` / `pm` | DMs (este panel) |
|---|---|---|
| Destino | **usuario** conectado | **personaje** (par fijo) |
| Historial | no (línea suelta en la consola) | sí, persistente |
| Requiere que esté online | sí | no |
| Unread | no | sí |
| Interfaz | comando de texto | panel con hilos |

---

## 6. Errores frecuentes

| Mensaje | Causa |
|---|---|
| `Select a character before sending messages.` | no has elegido personaje |
| `Enter the character id to message.` | campo *New* vacío |
| `cannot dm self` | id igual al tuyo |
| `target character not found` | el id no existe |
| `not a participant of this channel` | no formas parte de ese canal |
| `message body is required` | mensaje vacío |
| `limit must be a positive integer` | parámetro `limit` inválido |
| `since must be an RFC3339 timestamp` | parámetro `since` mal formado |

---

## 7. Referencia API (equipo)

Rutas bajo `/api/` con `Authorization: Bearer` (registradas en
`pkg/server/server.go`); el personaje actúa se envía por query string en GET y
por body en POST.

| Método | Ruta | Body / query |
|---|---|---|
| `GET` | `/api/dm/channels` | `?characterId=` |
| `POST` | `/api/dm/channels` | `{characterId, targetCharacterId}` (find-or-create) |
| `GET` | `/api/dm/channels/:channelId/messages` | `?characterId=&limit=&since=` |
| `POST` | `/api/dm/channels/:channelId/messages` | `{characterId, body}` |
| `POST` | `/api/dm/channels/:channelId/read` | `{characterId}` |

Ver cómo obtener un `characterId` en [`clans.md`](clans.md) §9.
