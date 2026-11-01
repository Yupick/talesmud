# Guía del jugador

Punto de entrada para jugar en el fork `mi-mud`. Resume el flujo esencial y
enlaza la referencia de cada sistema en lugar de duplicarla.

> Verificado contra el código el 2026-10-17. Comandos reales del fork en
> [`commands.md`](commands.md).

---

## 1. Creación de personaje

Registro (o invitado) → asistente de 3 pasos:

1. **Plantilla + raza** — eliges una de las 6 clases jugables y una de las 4 razas.
2. **Nombre y descripción** (mínimo 2 caracteres para el nombre).
3. **Confirmación** — empiezas a nivel 1 en la sala inicial.

**Clases y escuelas** (catálogo `data/classes/*.yaml`, servido en `GET /api/classes`):

| Clase | Escuela | Estilo |
|---|---|---|
| Warrior (Guerrero) | `martial` | frente de placa, golpe pesado |
| Rogue (Pícaro) | `martial` | dos golpes ligeros, huida |
| Mage (Mago) | `arcane` | lanzador de tela, quemadura por marcas |
| Cleric (Clérigo) | `divine` | curación (`mend` / `renew`) y guardia |
| Druid (Druida) | `nature` | HoT de naturaleza y escudo de corteza |
| Sorcerer (Hechicero) | `fire` | daño puro, alto riesgo |

El motor registra las escuelas `martial`, `fire`, `ice`, `arcane`, `divine`,
`nature` y `shadow` (`pkg/classkit/schools.go`): la escuela de tu clase decide
qué **talents** puedes aprender. Los nombres en español son alias de búsqueda.

**Razas**: Human (sin bono, algo más de oro inicial), Dwarf (+10 % con armas
contundentes), Elf (+10 % con arcos), Construct (el veneno nunca se pega).

Para más personajes: `newcharacter` (`nc`); para cambiar de uno ya creado:
`sc <nombre>`. Cuentas de invitado: tope de **nivel 5**. Detalle:
[`../player-guide/04-character.md`](../player-guide/04-character.md).

---

## 2. Atajos de teclado

| Tecla | Acción |
|---|---|
| `C` | panel de clan |
| `M` | panel de DMs |
| `K` | spellbook (carga de la barra de acciones) |
| `L` | editor de clases (herramienta de equipo, experimental/local) |
| `I` | inventario (alternar el panel) |
| `N` | fábrica de NPCs / mobs (herramienta de equipo, local) |
| `A` | audit log (equipo) |
| `1`–`9` | dispara el hueco correspondiente de la hotbar |
| `Tab` / `Shift+Tab` | cambia de objetivo en combate (solo durante un combate activo) |
| `?` | hoja de atajos del cliente |
| `Escape` | cierra el panel que esté encima |

Ninguna se activa mientras escribes en la consola. Fuente:
`public/mud-client/src/game/Game.svelte` (`onShortcutKey`). La hoja `?` del
cliente (`ShortcutSheet.svelte`) lista C/M/K/L/I/N/D/A, `1`–`9`, Tab, Escape y
`?`. `I` abre/cierra el inventario y `D` el Dungeon Builder (FASE 5.1, corte
inicial: grid, salas, encuentros, loot y modo de prueba; con creador conectado
guarda la biblioteca en `/api/dungeons` y si no, solo local). `N` abre la
fábrica de NPCs/mobs.

---

## 3. Progresión (XP, talentos, prestige)

- **XP → nivel**: curva acumulativa hasta **nivel 50**; al subir se restauran
  PS y maná al máximo.
- **Atributos**: +2 puntos por nivel para gastar con `spend <atributo> [n]`;
  además tu atributo principal sube +1 cada 5 niveles y el secundario +1 cada 10.
- **Talents**: +1 punto cada 5 niveles (5, 10, … 50). `talents` lista los
  tuyos y los disponibles; `talents spend <talent>` gasta 1 punto. Solo aparecen
  los talents propios de la escuela de tu clase.
- **Prestige**: en nivel 50, `prestige confirm` (irreversible, no en combate)
  te devuelve a nivel 1 y otorga **+5 % XP permanente** y +1 punto de talento
  por prestige. Conservas talents, oro, equipo, descubrimientos y amigos.

Diseño completo: [`../design/PHASE2_4_PROGRESSION.md`](../design/PHASE2_4_PROGRESSION.md).

---

## 4. Inventario y equipo

- Abrir: tecla `I` o comando `i`. `equipment` (`eq`) enseña lo equipado;
  `equip <obj>` / `unequip <hueco>` cambian equipo; `get`/`drop`/`use` mueven o
  usan objetos.
- El panel dibuja un contador de **peso** `total/máx (%)` y lo marca en rojo al
  excederse. Es **solo un aviso visual del cliente**, no hay encumbrance real:
  el tope está fijo en `maxWeight = 100` (`InventoryWidget.svelte`, nada lo
  sobreescribe), los objetos que llegan del servidor **no incluyen `weight`**
  (el contador se queda normalmente en `0/100`) y el servidor **no rechaza**
  `get` ni ningún otro comando por peso.
- Sí aplica el servidor: requisitos de clase, **entrenamiento de armadura**
  (`Requires plate armor training.`) y hueco/tipo de objeto.
- La tarjeta del objeto compara con lo equipado y ofrece Equip, Use, Examine,
  Sell y Drop.

Detalle: [`../player-guide/05-inventory-equipment.md`](../player-guide/05-inventory-equipment.md).

---

## 5. Social

| Sistema | Cómo | Doc |
|---|---|---|
| Clanes | tecla `C` (no existe comando `clan`) | [`clans.md`](clans.md) |
| DMs | tecla `M` (persisten en base de datos) | [`dms.md`](dms.md) |
| Canales globales | `chat general hola`; atajos `general`, `lfg`, `roleplay` | [`channels.md`](channels.md) |
| Party | `party create/invite/…` (alias `p`) o el overlay | [`party.md`](party.md) |
| Sala / privado | `say`, `scream`, `tell` (solo jugadores online) | [`../player-guide/03-commands.md`](../player-guide/03-commands.md) |

---

## 6. Combate

- Entrar: `attack` (alias `a`); `attack!` confirma contra un rival mucho más
  fuerte. En el cliente: botones, hotbar y BattleStage.
- En tu turno: `cast <skill> [objetivo]` o directamente `1`–`4`; `defend`/`d`,
  `focus <enemigo>` y `flee`.
- Las skills **solo funcionan en combate**: `skills` las lista y
  `skills equip <nombre>` ocupa 1 de tus **4 huecos**; cada una se desbloquea a
  un nivel y entra sola en la hotbar cuando hay sitio.
- Recompensa: XP + botín (regla de la party con `party loot`).

Detalle: [`combat.md`](combat.md) y [`../player-guide/06-combat.md`](../player-guide/06-combat.md).

---

## 7. Comandos y ayuda

En el juego: `help` (alias `h`). Referencia corta de lo que existe hoy:
[`commands.md`](commands.md). Catálogo completo del motor:
[`../player-guide/03-commands.md`](../player-guide/03-commands.md) y hoja rápida
[`../player-guide/12-quick-reference.md`](../player-guide/12-quick-reference.md).

## 8. Más documentación

- Índice de toda la documentación: [`../index.md`](../index.md)
- Guía del jugador por capítulos: [`../player-guide/README.md`](../player-guide/README.md)
- Arranque local, Docker y problemas: [`../fork-setup.md`](../fork-setup.md)
