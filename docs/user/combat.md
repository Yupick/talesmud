# Habilidades de clase y curación

Guía breve de las **skills** de clase: cómo equiparlas y usarlas, y qué hacen
los efectos de curación `mend` y `renew`.

> Implementación: `pkg/classkit/catalog.go` + `data/classes/*.yaml` (definición),
> `pkg/entities/skills/classkit.go` (traducción al motor),
> `pkg/mudserver/game/combat/classkit.go` (ejecución) y
> `pkg/mudserver/game/commands/cast.go` / `skills.go` (comandos).

---

## 1. Básico: equipar y usar

| Paso | Cómo |
|---|---|
| Ver tus skills | `skills` (alias `spells`, `abilities`): lista equipadas y desbloqueadas con su coste |
| Equipar | `skills equip <nombre>` — hasta **4 huecos** (clases con `hotbar_cap: 4`) |
| Quitar | `skills unequip <nombre>` |
| Usar | `cast <nombre> [objetivo]` o el **número del hueco** (`1`–`4`) en combate |
| En el cliente | la hotbar se rellena sola al subir de nivel; los botones envían el mismo `cast` |

* Las skills **solo funcionan en combate** (`You can only use skills in combat.`).
* Cada skill se desbloquea a un **nivel** (`level`) y entra sola en la hotbar
  cuando hay hueco libre.
* El costo se muestra al equipar/usar: `(10 mana)` si consume maná, o
  `(4 round CD)` si es solo de enfriamiento.

---

## 2. `effect`: `mend` y `renew`

El campo `effect` de cada skill indica qué primitiva ejecuta el motor. Las dos
relacionadas con la curación son:

| `effect` | Tipo | Qué hace |
|---|---|---|
| `mend` | curación directa | **cura al instante** un porcentaje (`multiplier`) de los **PS máximos del lanzador** |
| `renew` | HoT (*heal over time*) | aplica un efecto que cura ese porcentaje **por ronda** durante 3 rondas |

* El porcentaje se calcula **sobre el lanzador**, no sobre el objetivo:
  `cura = round(PS_máx_del_lanzador × multiplier)` (mínimo 1).
* La curación **nunca supera los PS máximos** del objetivo.
* En `renew`, el tick se aplica al **inicio de la ronda**, baja la duración en 1
  y desaparece al llegar a 0 (`X's Renew effect has worn off.`).
* Un fallo de lanzamiento (sin objetivo, maná insuficiente, en enfriamiento)
  **no consume maná**.

### Skills de curación actuales

| Skill | Clase | Nivel | `effect` | Curación | Maná | Enfriamiento | Componente |
|---|---|:-:|---|---|:-:|:-:|---|
| **Mend** (`cleric_mend`) | Clérigo | 2 | `mend` | 30 % de tus PS máx. | 10 | 4 rondas | `divine_wax` |
| **Renew** (`cleric_renew`) | Clérigo | 7 | `renew` | 12 % por ronda × 3 | 12 | 6 rondas | `divine_wax` |
| **Rejuvenate** (`druid_renew`) | Druida | 5 | `renew` | 10 % por ronda × 3 | 10 | 5 rondas | `nature_herb` |
| **Verdant Mend** (`druid_mend`) | Druida | 9 | `mend` | 25 % de tus PS máx. | 12 | 4 rondas | `nature_herb` |

Ejemplo: un Clérigo con 400 PS máximos que lanza *Mend* recupera
`round(400 × 0.30) = 120` PS; *Renew* cura `48` PS al inicio de cada una de las
próximas 3 rondas.

---

## 3. Objetivo: aliado o uno mismo

Las skills de curación se declaran con `target: ally` y el motor resuelve el
objetivo así:

1. Si se indica un **objetivo aliado vivo** en la misma facción → cura a ese
   aliado.
2. Si el objetivo no existe, ya es cadáver o es un **enemigo** → se cura
   **uno mismo** (fallback). Un `mend` nunca puede usarse para dañar.
3. Sin ningún objetivo → se cura uno mismo.

Mensajes:

```
Aster casts Mend on Brom, restoring 120 HP! (320/400 HP) (mana -10)
Aster casts Mend, restoring 120 HP! (400/400 HP) (mana -10)
Aster casts Renew on Brom! (Healing 48 per round for 3 rounds) (mana -12)
```

Sin objetivo válido el motor responde `Nobody to mend.` / `Nobody to renew.`.

> **Limitación actual:** el comando `cast <skill> [objetivo]` solo resuelve
> **nombres de enemigos**; para `target: ally` no traduce el nombre a un id, así
> que en la práctica un `cast Mend …` termina curándote a ti. El motor sí
> acepta el id de un aliado si el llamante lo pasa (lo cubren los tests
> `classkit_heal_test.go`).

---

## 4. Maná, enfriamiento y componente

| Concepto | Regla |
|---|---|
| **Maná** (`mana_cost`) | se comprueba antes de lanzar: `Not enough mana for Mend (need 10, have 4).`; si falta, la skill no se ejecuta |
| **Enfriamiento** (`cooldown`) | en rondas; mientras dura: `Mend is on cooldown (3 rounds remaining)` |
| **Ambos a la vez** | si la skill tiene `mana_cost` **y** `cooldown`, el motor paga maná **y** arranca el enfriamiento |
| **`once_per_fight`** | habilidades de cargas (p. ej. `Ward`, `Retribution`): un uso por combate en lugar de maná/enfriamiento |
| **Componente** (`component`) | reagente por escuela (`divine_wax`, `nature_herb`, `arcane_dust`…) declarado en el YAML; **hoy el motor no lo consume**, solo queda en el esquema |

Ejemplos de los YAML de clase (`data/classes/cleric.yaml`, `druid.yaml`):

```yaml
- id: cleric_mend
  name: Mend
  effect: mend
  level: 2
  mana_cost: 10
  cooldown: 4
  component: divine_wax
  multiplier: 0.30      # 30 % de mis PS máximos
  target: ally
```

---

## 5. Resumen de flujo

```
cast Mend
   │  ¿en combate?  ── no ──► "You can only use skills in combat."
   │  sí
   ├─ ¿está equipada? ── no ──► "You don't have a skill matching …"
   ├─ ¿maná suficiente? ── no ──► "Not enough mana for Mend …"
   ├─ ¿en enfriamiento? ── sí ──► "Mend is on cooldown (n rounds remaining)"
   └─ OK → cola de skill → en tu turno: paga maná, arranca cooldown,
           cura round(PS_máx × multiplier) al aliado (o a ti) y registra la línea
```

Para ver todos los valores vigentes de tu clase: `skills` en el juego, o
`GET /api/classes` / `GET /api/skills` (nivel `creator`) para el equipo.
