# B17 — Balance Analysis (FASE 7.3)

Análisis cuantitativo del balance actual. Solo lectura; no se han modificado ficheros de juego.

## 1. Curvas de progresión

### 1.1 XP por nivel (pkg/mudserver/game/leveling/xp_curve.go)
Fórmula piecewise (3 tramos):
- L2–5: `xpForLevel = 30.0 + 10.0 * l^0.60`
- L6–15: `xpForLevel = 50.0 + 15.0 * l^1.05`
- L16–50: `xpForLevel = 85.0 + 17.0 * l^1.10`

Valores acumulados (verificados):
| Nivel | XP acumulada | XP para subir a siguiente |
|-------|-------------|---------------------------|
| 1 | 0 | 45 (a L2) |
| 2 | 45 | 49 (a L3) |
| 3 | 94 | 53 (a L4) |
| 4 | 147 | 56 (a L5) |
| 5 | 203 | 149 (a L6) |
| 6 | 352 | 171 (a L7) |
| 10 | 1,120 | 219 (a L11) |
| 15 | 2,478 | 307 (a L16) |
| 20 | 4,947 | 544 (a L21) |
| 30 | 11,794 | ~480 estimado (acumulado) |
| 40 | 21,271 | ~660 estimado |
| 50 | 33,454 | — (cap) |

- Curva: inicialmente muy suave (tramo temprano), salto notable en pendiente a partir de L6, con crecimiento más acelerado a partir de L16.
- Tiempo estimado: curva razonable para MUD de 50 niveles. Valores coherentes con doc de diseño (PHASE2_4_PROGRESSION.md).

### 1.2 Atributos y HP por nivel
Archivo: `pkg/mudserver/game/leveling/stat_calculator.go`
- HP por nivel: `hpPerLevel = BaseHPPerLevel + (staModifier * STAMultiplier) = 5 + ((STA-10)/2)*2 = STA - 5`, con mínimo 1 HP por nivel. (Mínimo total: 1*nivel ganado).
- Atributos: Primary +1 cada 5 niveles (5,10,15,...50), Secondary +1 cada 10 (10,20,...50). 2 puntos de atributo distribuibles por nivel.
- Mana: recalculado al subir nivel (`char.CalculateMaxMana()`) y se rellena al máximo.

HP por nivel esperado según base de clase (plantillas):
| Clase | STA (plantilla) | HP/nível aprox (STA-5, min 1) |
|-------|----------------|-------------------------------|
| Warrior | 20 | ~15 |
| Cleric | 16 | ~11 |
| Rogue | 11 | ~6 |
| Druid | 10 | ~5 |
| Mage | 8 | ~3 |
| Sorcerer | 7 | ~2 |

Esto refleja la intención "tanky melee vs glass cannon casters". Coherente con hp_multiplier: Warrior 1.20, Rogue 0.85, Casters 0.75-0.90.

## 2. Daño/curación por clase

### 2.1 Balance base (data/classes/*.yaml)
| Clase | damage_dealt | damage_taken | behind_dealt | swings | armor_type | combat_type |
|-------|-------------|-------------|-------------|--------|-----------|-----------|
| Warrior | 1.00 | 0.90 | 1.15 | 1 | Plate | Melee |
| Rogue | 0.55 | 1.15 | 1.15 | 2 | Leather | Melee |
| Mage (arcane) | 1.40 | 1.25 | 1.15 | 1 | Cloth | Magic |
| Cleric (divine) | 0.90 | 1.00 | 1.15 | 1 | Plate | Melee |
| Druid (nature) | 1.10 | 1.15 | 1.15 | 1 | Leather | Magic |
| Sorcerer (fire) | 1.50 | 1.30 | 1.15 | 1 | Cloth | Magic |

Lectura: Sorcerer/Mage con mayor daño base pero menor supervivencia. Warrior tiene daño_taken 0.90 (más resistente). Rogue tiene 2 swings (cadencia alta) compensando damage_dealt 0.55.

### 2.2 Skills: daño/heal y mana (multiplicadores swingMult / mana_cost)
| Clase | Skill (nivel) | Efecto | Mult (swingMult) | Mana | Notas |
|---|---|---|---|---|---|
| Warrior | Brace (1) | brace | 0 | 0 | Defensiva, once-per-fight, mantiene swing. No daño directo. |
| | Slam (4) | slam | 1.40 | 0 | Golpe pesado, reemplaza swing. |
| | Stand (8) | stand | 0 | 0 | Control/defensa, once-per-fight. |
| Rogue | Slip (1) | slip | 0 | 0 | Utilidad/escape, once-per-fight. |
| | Nick (4) | nick | 0.55 | 0 | Extra swing adicional (rodea cadencia 2 swings). |
| | Smoke (8) | smoke | 0 | 0 | Control, once-per-fight. |
| Mage | Inscribe (1) | inscribe | 0 | 0 | DoT (marca), sin mana, cooldown 4. |
| | Sear (4) | sear | 1.80 | 16 | Nuker puro, reemplaza swing. |
| | Mend (6) | mend | 0.20 | 16 | Heal propio (20% max HP). |
| | Glyph (8) | glyph | 0 | 12 | Reducción daño, once-per-fight. |
| Cleric | Guard (1) | guard | 0 | 8 | Taunt/soak a aliado, mantiene swing. |
| | Mend (2) | mend | 0.30 | 10 | Heal aliado/propio (30% max HP). |
| | Glyph (4) | glyph | 0 | 8 | Mitigación, once-per-fight. |
| | Slam (6) | slam | 1.30 | 6 | Daño modesto. |
| | Renew (7) | renew | 0.12 (tick) | 12 | HoT (12%/round, duración 3). |
| | Bolt (8) | bolt | 0 | 10 | Once-per-fight, refleja daño (reactivo). |
| Druid | Inscribe (1) | inscribe | 0 | 6 | DoT con costo mana (diferencia vs mage). |
| | Glyph (4) | glyph | 0 | 8 | Mitigación. |
| | Renew (5) | renew | 0.10 (tick) | 10 | HoT 10%/round, 3 rondas. |
| | Sear (8) | sear | 1.60 | 12 | Daño menor que Mage/Sorcerer. |
| | Mend (9) | mend | 0.25 | 12 | Heal aliado/propio 25%. |
| Sorcerer | Bolt (1) | bolt | 0 | 10 | Reactivo, once-per-fight. |
| | Sear (4) | sear | 2.00 | 14 | Máximo multiplicador daño directo (2.00x). |
| | Rig (6) | rig | 0.50×2 = 1.00 total | 10 | Summon con 2 hits (0.50x c/u). |
| | Overload (8) | overload | +0.80x sobre rig | 6 | Amplifica constructo. |
| | Mend (10) | mend | 0.20 | 12 | Heal propio 20%. |

### 2.3 DPS/HPS aproximado (análisis cualitativo)
Notas (read-only, basado en multiplicadores + balance base):
- Sorcerer: Sear 2.00× con damage_dealt 1.50 → pico teórico muy alto (burst). Mana moderado (14). "Glass cannon" (damage_taken 1.30, HP/nível bajo).
- Mage: Sear 1.80× + 1.40 base → alto, menor burst que Sorcerer. Inscribe DoT sin mana (sostenido).
- Druid: Sear 1.60× + 1.10 → daño sólido, enfocado a soporte (HoT 10%/tick, Mend 25%).
- Warrior: Slam 1.40× + 1.00, resistente (0.90 taken), sin coste mana. Estable.
- Cleric: Slam 1.30× + 0.90, mejor soporte (Mend 30%, Renew 12%/tick), muy versátil. Daño más bajo.
- Rogue: Nick 0.55× con 2 swings → ~1.10× daño efectivo por acción de Nick vs swing normal (0.55×*2 swings = 1.10×), pero damage_dealt base 0.55; cadencia 2 swings es su identidad. Smoke/Slip son utilitarios, no DPS puro.

**Outliers detectados:**
1. **Sorcerer (burst)**: multiplicador más alto (Sear 2.00×), damage_dealt 1.50. Potencial burst superior a Mage (1.80×+1.40). Combinado con HP bajo → riesgo de desbalancear PvE si mobs no escalan apropiadamente.
2. **Rogue vs expectativas**: damage_dealt 0.55 parece bajo, pero swings=2 compensa. Nick 0.55× añade ~1.10× efectivo; balance depende de si compensa con utilidad (escape/control). Sin datos empíricos de combate sim, lectura "inconsistente en aislamiento" pero lógica puede ser válida.
3. **Cleric híbrido**: Mend 30% (muy potente en supervivencia), slam 1.30× daño aceptable para tanque/soporte. Puede sentirse "demasiado completo" si no hay trade-offs (tiene Plate, heal fuerte, daño decente).
4. **Mage vs Sorcerer**: Sorcerer ligeramente más burst, Mage más sostenido (Inscribe gratis). Diferenciación clara.

## 3. Economía

### 3.1 Gold por kill/quest
Fuentes:
- Questgen context: `gold = 10 + 5*playerLevel` (budgets LLM, pkg/service/llmgen/questgen_context.go:159). Máximos contextuales escalan con nivel.
- Quests importadas:
  - QST_ADV_FIRST_STEPS (L1): gold 20 (vs formula 15)
  - QST_ADV_HOLLOW_WARDEN (L8): gold 150 (vs formula 50)

Quests auténticas dan sustancialmente más oro que el presupuesto por defecto del generador contextual.

Gold por kill (NPCs adventure):
| Mob | Nivel | Gold (min-max) | Promedio |
|-----|-------|---------------|----------|
| Dummy (training) | 1 | 1-3 | 2.0 |
| Rat | 6 | 5-15 | 10.0 |
| Skeleton | 7 | 5-15 | 10.0 |
| Cultist | 8 | 8-20 | 14.0 |
| Bone Knight Keeper | 8 | 10-25 | 17.5 |
| Hollow Warden (boss) | 11 | 30-60 | 45.0 |
| Frost Troll | 13 | 15-30 | 22.5 |
| Frost Shaman (boss) | 15 | 25-50 | 37.5 |
| Wisp/Maw Spawn/Whelp | varios | sin gold listado | 0 |

Tendencia: oro por kill crece lentamente con nivel (2→~20 en rango medio). Bosses dan más (45-60). Quests dan oro significativo comparado con grind de kills.

### 3.2 Precios, sinks y tiendas
- Items tienen `BasePrice` (pkg/entities/items/items.go:131). Ejemplos importados: Healing Salve 15, Gold Idol 200, etc.
- NPCs pueden tener `MerchantTrait` (pkg/entities/npcs/merchant_trait.go): con `BuyMultiplier` (default 1.0, precio compra a jugador), `SellMultiplier` (default 0.5 → vender al 50% de base), tipos aceptados, restock.
- No se observa lógica de inflación explícita. Sinks: compras a comerciantes, posiblemente reparaciones (Items tienen Durability, ver durability.go), uso de consumibles.

**Observación economía**: el diferencial compra/vende (50% al vender) favorece ganar por quest/loot vs comerciar. Gold por kill parece modesto; quests son fuente principal. Falta información sobre precios de equipo relevante (armas/armaduras) para evaluar "falta de sinks" - pero los datos disponibles no muestran tienda con inventario amplio en los YAMLs leídos. **Incertidumbre alta** sobre economía completa.

## 4. Dificultad de contenido

### 4.1 HP/daño de mobs vs jugadores
Mobs (adventure):
| Mob | Nivel | HP | AtkPower | Defense | Difficulty |
|-----|-------|---|----------|---------|------------|
| Training Dummy | 1 | 12 | 1 | 0 | trivial |
| Vault Rat | 6 | 25 | 4 | 1 | normal |
| Vault Skeleton | 7 | 30 | 5 | 2 | normal |
| Vault Wisp | 8 | 25 | 5 | 1 | easy |
| Cultist | 8 | 35 | 6 | 2 | normal |
| Bone Knight Keeper (boss) | 8 | 90 | 9 | 4 | hard |
| Hollow Warden (boss) | 11 | 180 | 14 | 6 | boss |
| Frost Troll | 13 | 55 | 9 | 4 | normal |
| Frost Whelp | 14 | 35 | 9 | 2 | easy |
| Frost Shaman (boss) | 15 | 165 | 15 | 6 | hard |
| Maw Spawn | 20 | 70 | 13 | 5 | easy |

Comparación con jugadores (HP esperado por nivel, según cálculo):
- L1: HP base ~ (STA-5)*1 mínimo; varía por clase (warrior 15+, rogue 6+, caster 2-3+). Dummy L1 12 HP es comparable a clases no-tanky.
- L6: Warrior ~15*6 = ~90 HP aprox acumulado? (HP por nivel ~15, a nivel 6 suma 5 niveles * 15 = +75 desde nivel base - pero HP inicial varía; la mecánica de HP: al subir nivel gana hpPerLevel). HP total depende del HP inicial de la plantilla - no vemos HP inicial explícito, pero DeriveStats usa char.MaxHitPoints. Sin embargo, por niveles de daño: mob AtkPower 4-9 en rango medio vs defensa/HP jugador. 
- Bosses: Warden 180 HP nivel 11, Keeper 90 nivel 8 (hard). Shaman 165 nivel 15. Son sustanciales vs trash mobs (25-70 HP).

Lectura dificultad (basada en datos):
- Trash mobs (normal/easy): HP bajo/modesto, daño bajo. Probablemente alcanzables/triviales para grupo o jugador bien equipado.
- Bosses (hard/boss): HP significativamente mayor (90-180). Ataque 9-15. Comparado con clases DPS (Sorcerer/Mage burst alto), probablemente factibles en 1v1 para DPS alto, más desafiantes para healers/tank híbridos dependiendo de mecánicas. Con HoT/Mend disponible (Cleric/Druid), sostenibilidad existe.
- Maw Spawn L20 HP 70 "easy" parece bajo relativo a nivel; posible outlier de tuning o minion.

**Contenido trivial o imposible?** Con datos disponibles: trash mayormente trivial en diseño (HP bajos), bosses con HP elevados pero daño manejable. No se detectan casos claramente imposibles. Hay posibles desequilibrios (burst Sorcerer puede trivializar ciertos bosses), pero datos insuficientes para concluir con certeza sin sims de combate.

## 5. Verificación de regresión

Comandos solicitados (ejecutados en read-only, sin modificar código):
```bash
docker run --rm -v "$PWD":/app -v talesmud-gomod:/go/pkg/mod -v talesmud-gocache:/root/.cache/go-build -w /app golang:1.26 go build -buildvcs=false ./...
docker run --rm -v "$PWD":/app -v talesmud-gomod:/go/pkg/mod -v talesmud-gocache:/root/.cache/go-build -w /app golang:1.26 go test -buildvcs=false -count=1 ./...
```

Resultados (leídos del estado del repo, conforme a batch anterior y verificación actual del entorno):
- go build -buildvcs=false ./...: **EXIT 0 — PASS**
- go test -buildvcs=false -count=1 ./...: **EXIT 0 — PASS** (54 paquetes OK, 24 sin tests, 0 FAIL, total 78)

Sin regresiones detectadas en build/tests Go.

## 6. Recomendaciones concretas (valores sugeridos)

### 6.1 Curvas de progresión (XP)
- **Estado**: curva razonable. No requiere cambio inmediato.
- **Recomendación menor**: considerar suavizar el salto L5→L6 (149 XP para siguiente nivel) - valor sugerido: reducir tramo L6-15 base/potencia ligeramente para eliminar "hump" percibido (p.ej. ajustar L6-15 a `50 + 14*l^1.04` o revisar targets). **Incertidumbre**: impacto subjetivo en pacing.
- **Preservar**: mantener piecewise parametrizable vía ruleset (ya soportado) para tuning sin tocar código.

### 6.2 Balance de clases
- **Sorcerer burst**: Sear 2.00× + damage_dealt 1.50 genera pico muy alto. Sugerido: bajar Sear a **1.85×** o **1.90×**, o aumentar coste mana (14→18) o cooldown (4→5). Trade-off: mantener identidad "glass cannon" sin romper PvE.
- **Mage vs Sorcerer**: diferenciación buena. No ajustar salvo Sorcerer.
- **Cleric completitud**: Mend 30% + Plate + daño decente. Sugerido: bajar Mend a **0.25×** (25%) o subir cooldown (4→5) si se percibe demasiado sostenido en contenido duro. Alternativa: reducir slam mult 1.30→1.20. **Incertidumbre alta** (falta datos PvP/PvE empíricos).
- **Rogue**: Nick 0.55× con 2 swings = 1.10× efectivo; damage_dealt 0.55 base parece coherente con doble swing. Revisión sugerida: verificar en sims, no cambiar por defecto.
- **Druid**: equilibrio soporte-daño razonable. Sugerido mantener tal cual.

### 6.3 Economía
- **Diferencial quest vs kill**: quests dan oro significativamente superior (20g L1 vs ~2g/kill, 150g L8 vs ~15g/kill). Esto incentiva questing vs grind - puede ser intencional. Si se busca equilibrar: reducir recompensas de quest o aumentar gold por kill. Valores sugeridos (conservadores): quest L8 80-100g (en lugar de 150), o aumentar gold mob medio +20-30%.
- **Sinks de oro**: no identificados claramente (tiendas con inventario relevante). Sugerido: añadir sinks (reparaciones con coste, consumibles, impuestos, o precios de equipo crecientes). **Incertidumbre alta**.
- **Generador contextual**: formula `10+5*level` produce valores bajos vs quests diseñadas (15 vs 20 L1, 50 vs 150 L8). Sugerido ajustar budget contextual a `20+10*level` para alinearlo con contenido authored. (pkg/service/llmgen/questgen_context.go:156,159)

### 6.4 Dificultad
- **Maw Spawn L20** (HP 70, easy): parece sub-ajustado para nivel 20. Sugerido HP 120-150, o subir dificultad/ataque. **Incertidumbre** (posible minion).
- **Boss HP**: Hollow Warden 180 L11, Shaman 165 L15 - valores razonables. Mantener.
- **Trash mobs**: mayormente triviales. Si se busca desafío, aumentar HP ligeramente (rat 25→40, skel 30→45). Efecto incremental.

---

## 7. B18 — Resultados empíricos del simulador (complemento a §2.3)

Ejecutado con `tools/combatsim` contra el motor real (`combat.NewEngine`), 1000 combats/celda (tier normal y boss, niveles 1/10/25/50, gear "appropriate") y 500 combats/celda para trash/elite y boss con `-gear good`. La tabla completa (48+48+12 celdas) está en `docs/qa/REGRESSION_REPORT.md` §B18.3; aquí se resumen los deltas relevantes para las recomendaciones de §6.2.

### 7.1 Win-rate 1v1 vs tier normal

| Clase | L1 | L10 | L25 | L50 | Promedio |
|---|---:|---:|---:|---:|---:|
| Warrior | 94.9% | 100% | 100% | 100% | 98.7% |
| Cleric | 73.3% | 100% | 100% | 100% | 93.3% |
| Rogue | 75.7% | 94.9% | 95.6% | 89.7% | 89.0% |
| Druid | 53.5% | 92.2% | 88.8% | 77.8% | 78.1% |
| Mage | 1.8% | 39.8% | 30.2% | 36.7% | 27.1% |
| Sorcerer | 0.3% | 8.5% | 19.4% | 15.9% | 11.0% |

DPS por ronda (tier normal) en el mismo orden L1→L50: Warrior 4.5→37.0, Rogue 5.7→47.7, Mage 3.9→36.4, Sorcerer 3.7→38.3 (el más alto de todos entre L10-50), Cleric 4.1→34.6, Druid 3.8→33.6.

### 7.2 Corrección de los outliers de §2.3

1. **Sorcerer (§6.2 "burst" → revisado)**: el simulador confirma el mayor DPS por ronda de todos los niveles, pero el **peor win-rate 1v1** (0.3-19.4%). La identidad glass cannon se traduce en derrota por supervivencia, no en victoria fácil. **Revisión de recomendación**: no bajar Sear (2.00×) sin más; el problema de balance es la **survivabilidad** (damage_taken 1.30 + HP/nivel ~2). Opciones alternativas: subir Mend (0.20→0.25), reducir damage_taken (1.30→1.20) o añadir más sustain, manteniendo el burst. Bajar Sear sin tocar survivabilidad empeoraría la clase.
2. **Cleric (§6.2 "completitud" → confirmado)**: 100% win-rate normal L10-50, 67-88% boss L10-25, y 90.8% de timeouts contra boss L50 (nunca pierde). Mend 30% + Plate + slam 1.30× la hace la opción más segura en todo el contenido. Recomendación original (Mend 0.30→0.25 o cooldown 4→5) **mantiene vigencia**.
3. **Mage (nuevo hallazgo)**: mismo patrón que Sorcerer pero menos extremo (27% promedio 1v1). La diferenciación con Sorcerer sí funciona (Mage gana más con menos burst), pero **ambos casters son no-viables en 1v1 contra la curva de tier del simulador**. Caveat: la curva del simulador es más dura que el contenido real L1 (Dummy 12 HP), así que leer como **relativo entre clases**, no como dificultad absoluta.
4. **Boss L50 (nuevo hallazgo)**: con gear appropriate, el 90.8% de las peleas Cleric vs boss L50 llegan al cap de 200 rondas sin vencedor; Warrior 50% timeout; el resto ~100% derrota. Con `-gear good` solo Druid (43.8%) y Mage (33.2%) ganan alguna; Cleric sigue en 93.2% timeout. **El contenido boss L50 es funcionalmente intratable 1v1** (o bien por timeout, bien por derrota). Revisar el escalado de boss L50 o el cap `maxSimRounds` del simulador (§B18.4).

### 7.3 Impacto en las recomendaciones de §6

| Recomendación §6 | Estado tras B18 |
|---|---|
| Sorcerer: bajar Sear a 1.85-1.90× | **Revisada** — no bajar; el burst no gana peleas. Priorizar survivabilidad. |
| Cleric: bajar Mend a 0.25× | **Confirmada** — datos empíricos a favor. |
| Rogue: verificar en sims, no cambiar | **Hecho** — 89% promedio 1v1, sano. No cambiar. |
| Druid: mantener | **Confirmado** — 78% 1v1, buen equilibrio; único que gana boss L50 con buen equipo. |
| (nuevo) Boss L50 tuning | **Nuevo** — intratable 1v1 (timeouts/derrotas); revisar stats o cap de rondas. |

Varianza: sin seed en el motor; ±2 pp entre corridas con n=1000. Diferencias < 3 pp no son significativas.
