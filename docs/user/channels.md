# Canales globales de chat

Los canales globales permiten hablar con **todo el mundo conectado**, sin
importar la sala donde estés. Se usan desde la consola de comandos.

> Implementación: `pkg/mudserver/game/commands/channels.go` (registro en
> `commandprocessor.go`) y `public/mud-client/src/game/chatTabs.js`.

---

## 1. Sintaxis

```
chat <canal> <mensaje>
```

También funcionan **atajos directos** para los canales cuyo nombre no está
usado por otro comando:

| Canal | Sintaxis válida | Atajo directo |
|---|---|---|
| `general` | `chat general hola` | `general hola` ✅ |
| `trade` | `chat trade vendo hierro` | ❌ solo con `chat` |
| `lfg` | `chat lfg falta sanador` | `lfg falta sanador` ✅ |
| `help` | `chat help ¿cómo hago X?` | ❌ solo con `chat` |
| `roleplay` | `roleplay mira la lluvia` | `roleplay mira la lluvia` ✅ |
| `admin` | `chat admin aviso` | `admin aviso` ✅ (solo admins) |

> `help` y `trade` **solo se alcanzan con el prefijo `chat`** porque `help` (ayuda)
> y `trade` (inventario del mercader) ya son comandos con otros usos; el atajo
> no los roba.

---

## 2. Quién puede publicar

| Canal | Requisito |
|---|---|
| `general`, `trade`, `lfg`, `help`, `roleplay` | cualquier usuario con **personaje seleccionado** |
| `admin` | solo cuentas con rol **admin**; el resto recibe `Admin channel is restricted.` |

* Sin personaje seleccionado: `You must select a character first.`
* Mensaje vacío (`chat lfg` sin texto) o `chat` solo: se muestra la línea de uso
  `Usage: chat <channel> <message>. Channels: general, trade, lfg, help, roleplay, admin`.
* Una palabra que **no** sea un canal conocido no se intercepta: el mensaje sigue
  su camino normal al chat de sala.

---

## 3. Formato y alcance

Los mensajes se emiten con **audiencia global** (todos los usuarios conectados)
con este formato de texto:

```
[General] Aster: ¿alguien por el templo?
```

`[<Título del canal>] <Nombre del personaje>: <texto>`, donde el título es
`General`, `Trade`, `LFG`, `Help`, `Roleplay` o `Admin`.

El mensaje también lleva la etiqueta estructurada `channel` (`general`,
`trade`…), que el cliente usa para enrutarlo a la pestaña **Ch** y para el
filtro por canal.

---

## 4. Interfaz (pestaña **Ch**)

* Todos los mensajes de canal caen en la pestaña **Ch** del chat, con contador
  de no leídos por pestaña.
* Sobre la lista hay una barra de filtros: **All** + un chip por canal
  (`General`, `Trade`, `LFG`, `Help`, `Roleplay`, `Admin`).
* Cada chip tiene un botón **↩** que rellena la consola con `chat <canal> ` para
  responder en ese canal.
* Al pulsar sobre el prefijo `[General]` de una línea se filtra por ese canal.

> La pestaña **Clan** sigue apareciendo deshabilitada con el aviso
> *"Clans coming soon"*: la chat de clan todavía no existe (los clanes se
> gestionan por panel, ver [`clans.md`](clans.md)).

---

## 5. Ejemplos

```
chat trade vendo 10 de hierro a 5 monedas
lfg ¿alguien para la mazmorra del templo?
roleplay Aster ajusta la mochila y sale al claro.
chat help ¿cómo equipo un arma?
chat admin alguien está farmeando en la sala 12
```

Salida que ven los demás:

```
[Trade] Aster: vendo 10 de hierro a 5 monedas
[LFG] Aster: ¿alguien para la mazmorra del templo?
[Roleplay] Aster: ajusta la mochila y sale al claro.
```

---

## 6. Diferencia con el resto del chat

| Comando | Alcance |
|---|---|
| `say` | solo la sala actual |
| `tell` / `whisper` / `pm` | un usuario concreto |
| `party <msg>` / `party say` | tu fiesta |
| `scream` / `shrug` | sala (con otro formato) |
| `chat <canal> <msg>` | **todo el servidor** |
