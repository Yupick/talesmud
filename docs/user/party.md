# Party (fiestas)

Las parties son grupos de hasta **5 personajes** para luchar, hablar y repartir
botín juntos. Se gestionan con el comando `party` (alias `p`) y con el
**overlay de party**.

> Implementación: `pkg/mudserver/game/commands/party.go`,
> `party_loot_ready.go`, `pkg/mudserver/game/loot_party.go`,
> `pkg/entities/party.go` y `public/mud-client/src/game/ui/PartyOverlay.svelte`.

---

## 1. Comandos

| Comando | Qué hace |
|---|---|
| `party` / `party list` | lista miembros y estado de seguimiento |
| `party create` | crea una party (eres leader) |
| `party invite <jugador>` | invita a un jugador **online**; crea la party si no existe |
| `party accept` / `party decline` | acepta / rechaza la invitación pendiente |
| `party leave` | sales de la party |
| `party kick <jugador>` | expulsa (solo **leader**) |
| `party promote <jugador>` | cambia el leader (solo **leader**) |
| `party say <mensaje>` · `party <mensaje>` | chat de fiesta: `[Party] Aster: hola` |
| `party follow` / `party unfollow` | sigues al leader (no en combate) |
| `party loot [regla]` | consulta o cambia la regla de botín (ver §3) |
| `party readycheck` | inicia una comprobación de disponibilidad (solo **leader**) |
| `party ready` · `party notready` | respuesta a la comprobación |

Sinónimo corto: `p` (`p invite Aster`, `p ready`…).

Notas:

* **Máximo 5 miembros**: al llegar al límite el servidor responde
  `Your party is full (5/5).`
* La invitación solo funciona con el otro **conectado**
  (`Player 'X' is not online.`).
* El overlay se abre con el botón **Party** de la cabecera (no tiene tecla
  propia). Dentro hay campo *Invite player by name*, campo *Party say…* con su
  botón **Say**, botón **Follow/Unfollow** (para no-leaders) y los controles de
  botín y disponibilidad de §3–§4.

---

## 2. Chat de fiesta

Todo lo que ocurre dentro de la party se envía con el prefijo `[Party]`:

```
[Party] Aster: ¿preparados?
[Party] Brom joined the party.
[Party] Cira left the party.
[Party] Cira was kicked from the party.
[Party] Brom is now the party leader.
```

El chat de party aparece en la pestaña **Party** del chat y solo lo ven los
miembros conectados.

---

## 3. Reglas de botín (`party loot`)

| Comando | Quién |
|---|---|
| `party loot` | cualquiera: muestra la regla actual (`[Party] Loot rule: free`) |
| `party loot <regla>` | solo el **leader**: la cambia y avisa a toda la party |

Reglas válidas: `free` · `leader` · `roundrobin` · `needgreed` (también se
acepta `need/greed`). Cualquier otra cadena muestra
`Usage: party loot <free|leader|roundrobin|needgreed>`.

La regla se guarda en la party y se refleja en el overlay: los leaders ven un
selector de radio con las cuatro opciones; el resto ve solo lectura
(`Loot: Free-for-all`).

### Qué hace cada regla al ganar un combate

| Regla | Etiqueta en la UI | Comportamiento |
|---|---|---|
| `free` | Free-for-all | **Los objetos quedan en el suelo de la sala** (por defecto). Cualquiera los recoge con `pickup`. |
| `leader` | Leader | Cada objeto va **directo al inventario del leader**. Motivo en el mensaje: `(leader loot)`. |
| `roundrobin` | Round Robin | Reparto **en rotación** entre los miembros vivos de la tanda: 1.º → 2.º → 3.º… Motivo: `(round robin)`. |
| `needgreed` | Need/Greed | **Sorteo aleatorio** entre los miembros vivos de la tanda. Motivo: `(need/greed)`. |

Al repartir, la party ve una línea por objeto, con el formato
`[Party] <objeto> → <personaje> (<motivo>)` (una línea por regla, a modo de
ejemplo):

```
[Party] Wolf Pelt → Aster (leader loot)
[Party] Iron Fang → Brom (round robin)
[Party] Holy Symbol → Cira (need/greed)
```

Ejemplo con una party de tres (Aster *leader*, Brom, Cira) que mata a un lobo y
suelta *Wolf Pelt* + *Iron Fang*:

| Regla | Resultado |
|---|---|
| `free` | ambos objetos aparecen en la sala; quien llegue primero los recoge |
| `leader` | los dos van a Aster |
| `roundrobin` | *Wolf Pelt* → Aster, *Iron Fang* → Brom (la rotación empieza en el primer elegido **en cada reparto**) |
| `needgreed` | cada objeto se asigna a un miembro al azar (pueden repetir) |

Detalles importantes:

* Solo se aplica si estás en una party; **en solitario el botín sigue siendo
  `free`** (objetos en el suelo).
* Se reparte entre los **personajes vivos** al ganar el combate.
* Si la asignación falla (p. ej. inventario lleno), el objeto vuelve al suelo de
  la sala.
* **`needgreed` hoy no es una tirada need/greed real**: reparte al azar, sin
  ventana de tiradas ni prioridad de *need* sobre *greed*.

---

## 4. Comprobación de disponibilidad (`ready check`)

```
party readycheck      # solo el leader
party ready           # cualquier miembro
party notready        # cualquier miembro
```

1. El leader inicia la comprobación; **toda la party** recibe
   `[Party] Ready check! Type 'party ready' or 'party notready'.`
2. Cada respuesta actualiza a todos el marcador
   `[Party] Ready: 2/5 · Not ready: 1/5`.
3. A los **30 segundos** la comprobación caduca; la siguiente respuesta muestra
   el resumen `[Party] Ready check over: 3/5 ready.` y la cierra.
4. Si alguien responde sin comprobación activa, su respuesta **abre** una nueva.

En el overlay:

* **Ready Check** (solo leaders) lanza la comprobación.
* Con una comprobación activa aparecen **Ready** y **Not Ready**, más el aviso
  *"Respond with 'party ready' or 'party notready'."*
* Al terminar se muestra `Ready check complete`.

---

## 5. Errores frecuentes

| Mensaje | Causa |
|---|---|
| `You are not in a party.` | no formas parte de ninguna |
| `You are already in a party.` | `party create` estando ya en una |
| `You have no pending party invite.` | `party accept` sin invitación |
| `That party is full (5/5).` | la party destino está llena |
| `Player 'X' is not online.` | invitado desconectado |
| `You can't invite yourself.` | autoinvitación |
| `Only the party leader can kick members.` / `promote members.` | no eres leader |
| `Only the party leader can change the loot rule.` | cambio de regla sin ser leader |
| `Only the party leader can start a ready check.` | `party readycheck` sin ser leader |
| `You can't follow while in combat.` | `party follow` en combate |
| `Usage: party loot <free\|leader\|roundrobin\|needgreed>` | regla desconocida |
