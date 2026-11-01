/**
 * inventoryEngine_test.mjs — Tests unitarios para InventoryEngine e InventoryItem.
 *
 * Ejecutar: node src/game/inventoryEngine_test.mjs
 *
 * Estas pruebas describen el comportamiento REAL del motor tal y como está
 * implementado (incluidas sus particularidades de normalización y stacking).
 */

import { strict as assert } from 'assert';
import { InventoryEngine, InventoryItem } from './inventoryEngine.js';

// --- Helper: crear engine con peso max ----------
function makeEngine(items = [], maxWeight = 100) {
  return new InventoryEngine({ initialItems: items, maxWeight });
}

// --- InventoryItem tests ---

console.log('=== Testing InventoryItem ===');

// Creación básica con defaults.
const sword = new InventoryItem({ name: 'Sword', weight: 3, type: 'weapon', quality: 'normal' });
assert.strictEqual(sword.name, 'Sword');
assert.strictEqual(sword.weight, 3);
assert.strictEqual(sword.type, 'weapon');
assert.strictEqual(sword.quality, 'normal');
assert.strictEqual(sword.maxStack, 1, 'maxStack default es 1');
assert.strictEqual(sword.quantity, 1, 'quantity default es 1');
assert.strictEqual(sword.stackable, true, 'stackable default es true');
assert.strictEqual(sword.id.length > 0, true);
console.log('✓ InventoryItem creation OK');

// displayName: sin suffix → name; con suffix → `${name}-${suffix}`.
assert.strictEqual(sword.displayName, 'Sword');
const namedSword = new InventoryItem({ name: 'Sword', instanceSuffix: '+1' });
assert.strictEqual(namedSword.displayName, 'Sword-+1');
console.log('✓ InventoryItem displayName OK');

// totalWeight = quantity * weight (quantity se normaliza al maxStack).
const potion = new InventoryItem({ name: 'Potion', weight: 1, maxStack: 5, quantity: 5 });
assert.strictEqual(potion.quantity, 5);
assert.strictEqual(potion.totalWeight, 5);
assert.strictEqual(sword.totalWeight, 3);
console.log('✓ InventoryItem totalWeight OK');

// La cantidad se recorta a [1, maxStack].
const clamped = new InventoryItem({ name: 'Arrow', weight: 1, maxStack: 10, quantity: 99 });
assert.strictEqual(clamped.quantity, 10);
const floored = new InventoryItem({ name: 'Arrow', weight: 1, maxStack: 10, quantity: 0 });
assert.strictEqual(floored.quantity, 1, 'quantity 0 cae al mínimo 1');
console.log('✓ InventoryItem quantity clamp OK');

// isMaxStack / hasRoomForMore con maxStack=1 (no apilable en la práctica).
assert.strictEqual(sword.isMaxStack, true);
assert.strictEqual(sword.hasRoomForMore, false);
assert.strictEqual(potion.isMaxStack, true);
assert.strictEqual(potion.hasRoomForMore, false);
const stackableItem = new InventoryItem({ name: 'Arrow', weight: 1, maxStack: 100, quantity: 10, stackable: true });
assert.strictEqual(stackableItem.hasRoomForMore, true);
assert.strictEqual(stackableItem.isMaxStack, false);
const unstackable = new InventoryItem({ name: 'Rock', weight: 5, maxStack: 5, quantity: 3, stackable: false });
assert.strictEqual(unstackable.isMaxStack, false);
assert.strictEqual(unstackable.hasRoomForMore, false);
console.log('✓ InventoryItem isMaxStack/hasRoomForMore OK');

// clone conserva el mismo id (copia 1:1 de los campos).
const cloned = sword.clone();
assert.strictEqual(cloned.name, 'Sword');
assert.strictEqual(cloned.id, sword.id, 'clone conserva el id');
assert.notStrictEqual(cloned, sword, 'clone es un objeto distinto');
cloned.quantity = 2;
assert.strictEqual(sword.quantity, 1, 'mutar el clon no afecta al original');
console.log('✓ InventoryItem clone OK');

// --- InventoryEngine tests ---

console.log('\n=== Testing InventoryEngine ===');

// Creación.
const eng = makeEngine();
assert.strictEqual(eng.items.length, 0);
assert.strictEqual(eng.maxWeight, 100);
assert.strictEqual(eng.carryWeight(), 0);
const emptySummary = eng.getSummary();
assert.strictEqual(emptySummary.totalWeight, 0);
assert.strictEqual(emptySummary.carryPercent, 0);
assert.strictEqual(emptySummary.canCarryMore, true);
console.log('✓ InventoryEngine creation OK');

// addItem: ids autogenerados distintos → se empujan items separados.
const pushEng = makeEngine([], 100);
pushEng.addItem({ name: 'Apple', weight: 1, type: 'consumable' });
assert.strictEqual(pushEng.items.length, 1);
assert.strictEqual(pushEng.items[0].name, 'Apple');
assert.strictEqual(pushEng.carryWeight(), 1);
pushEng.addItem({ name: 'Pear', weight: 2 });
assert.strictEqual(pushEng.items.length, 2);
assert.strictEqual(pushEng.carryWeight(), 3);
assert.strictEqual(pushEng.getSummary().totalWeight, 3);
console.log('✓ InventoryEngine addItem OK');

// addItem reemplaza un item del mismo id que ya está al máximo.
const replaceEng = makeEngine([], 100);
replaceEng.addItem({ id: 'potion', name: 'Potion', weight: 1, maxStack: 5, quantity: 5 });
assert.strictEqual(replaceEng.items.length, 1);
assert.strictEqual(replaceEng.items[0].quantity, 5);
replaceEng.addItem({ id: 'potion', name: 'Potion', weight: 1, maxStack: 5, quantity: 1 });
assert.strictEqual(replaceEng.items.length, 1, 'mismo id y stack lleno → reemplaza');
assert.strictEqual(replaceEng.items[0].quantity, 1);
console.log('✓ InventoryEngine addItem replace maxed OK');

// removeItem completo.
const remEng = makeEngine([{ name: 'Sword', weight: 3, type: 'weapon' }], 100);
const rem = remEng.removeItem({ id: remEng.items[0].id, quantity: 1 });
assert.strictEqual(rem.removed.name, 'Sword');
assert.strictEqual(rem.remaining, null);
assert.strictEqual(remEng.items.length, 0);
assert.strictEqual(remEng.carryWeight(), 0);
console.log('✓ InventoryEngine removeItem complete OK');

// removeItem parcial (con maxStack que permite quantity > 1).
const partialEng = makeEngine([{ name: 'Apple', weight: 1, maxStack: 10, quantity: 5 }], 100);
assert.strictEqual(partialEng.carryWeight(), 5);
const remPartial = partialEng.removeItem({ id: partialEng.items[0].id, quantity: 2 });
assert.strictEqual(remPartial.removed, null);
assert.strictEqual(remPartial.remaining.quantity, 3);
assert.strictEqual(partialEng.items[0].quantity, 3);
assert.strictEqual(partialEng.carryWeight(), 3);
console.log('✓ InventoryEngine removeItem partial OK');

// removeItem con cantidad mayor que la disponible → remueve por completo.
const excessEng = makeEngine([{ name: 'Apple', weight: 1, maxStack: 10, quantity: 5 }], 100);
const remExcess = excessEng.removeItem({ id: excessEng.items[0].id, quantity: 100 });
assert.notStrictEqual(remExcess.removed, null);
assert.strictEqual(remExcess.remaining, null);
assert.strictEqual(excessEng.items.length, 0);
console.log('✓ InventoryEngine removeItem excess OK');

// removeItem inexistente.
const noRemEng = makeEngine([], 100);
const noRem = noRemEng.removeItem({ id: 'nonexistent-id', quantity: 1 });
assert.strictEqual(noRem.removed, null);
assert.strictEqual(noRem.remaining, null);
console.log('✓ InventoryEngine removeItem non-existent OK');

// removeSlot por índice.
const slotEng = makeEngine([{ name: 'A', weight: 1 }, { name: 'B', weight: 2 }], 100);
const removedSlot = slotEng.removeSlot(0);
assert.strictEqual(removedSlot.name, 'A');
assert.strictEqual(slotEng.items.length, 1);
assert.strictEqual(slotEng.items[0].name, 'B');
assert.strictEqual(slotEng.removeSlot(5), false);
assert.strictEqual(slotEng.removeSlot(-1), false);
console.log('✓ InventoryEngine removeSlot OK');

// getEquippedItems: excluye inventory/container/purse.
const equipEng = makeEngine([
  { name: 'Apple', weight: 1, type: 'consumable', slot: 'inventory' },
  { name: 'Shield', weight: 5, type: 'armor', slot: 'shield' },
  { name: 'Ring', weight: 1, type: 'armor', slot: 'ring' },
  { name: 'Bag', weight: 2, type: 'other', slot: 'container' },
  { name: 'Coins', weight: 1, type: 'other', slot: 'purse' },
], 100);
const equipped = equipEng.getEquippedItems();
assert.strictEqual(equipped.length, 2);
assert.deepStrictEqual(equipped.map(i => i.name), ['Shield', 'Ring']);
console.log('✓ InventoryEngine getEquippedItems OK');

// getItemsByType / getItemsBySlot.
assert.strictEqual(equipEng.getItemsByType('armor').length, 2);
assert.strictEqual(equipEng.getItemsBySlot('shield').length, 1);
assert.strictEqual(equipEng.getItemsBySlot('shield')[0].name, 'Shield');
assert.strictEqual(equipEng.getItemsBySlot('missing').length, 0);
console.log('✓ InventoryEngine getItemsByType/getItemsBySlot OK');

// findItemByName (case-insensitive, parcial).
const searchEng = makeEngine([
  { name: 'Health Potion', weight: 2, type: 'consumable' },
  { name: 'Mana Potion', weight: 2, type: 'consumable' },
  { name: 'Apple', weight: 1, type: 'consumable' },
], 100);
assert.strictEqual(searchEng.findItemByName('potion').length, 2);
assert.strictEqual(searchEng.findItemByName('HEALTH').length, 1);
assert.strictEqual(searchEng.findItemByName('').length, 0);
assert.strictEqual(searchEng.findItemByName('zzz').length, 0);
console.log('✓ InventoryEngine findItemByName OK');

// findItemById.
const byId = searchEng.findItemById(searchEng.items[0].id);
assert.strictEqual(byId.name, 'Health Potion');
assert.strictEqual(searchEng.findItemById('nope'), null);
console.log('✓ InventoryEngine findItemById OK');

// canAddItem es una consulta pura: no muta el engine.
const lightEng = makeEngine([], 10);
assert.strictEqual(lightEng.canAddItem({ name: 'Light', weight: 5 }), true);
assert.strictEqual(lightEng.canAddItem({ name: 'Heavy', weight: 10 }), true);
assert.strictEqual(lightEng.canAddItem({ name: 'Too Heavy', weight: 11 }), false);
assert.strictEqual(lightEng.carryWeight(), 0, 'canAddItem no añade nada');
console.log('✓ InventoryEngine canAddItem OK');

// canCarry / getRemainingWeight / isMaxWeightReached.
const weightEng = makeEngine([{ name: 'Rock', weight: 3 }], 10);
assert.strictEqual(weightEng.carryWeight(), 3);
assert.strictEqual(weightEng.canCarry(7), true);
assert.strictEqual(weightEng.canCarry(8), false);
assert.strictEqual(weightEng.getRemainingWeight(), 7);
assert.strictEqual(weightEng.isMaxWeightReached(), false);
weightEng.addItem({ name: 'Boulder', weight: 7 });
assert.strictEqual(weightEng.carryWeight(), 10);
assert.strictEqual(weightEng.getRemainingWeight(), 0);
assert.strictEqual(weightEng.isMaxWeightReached(), true);
console.log('✓ InventoryEngine weight helpers OK');

// isOverweight.
const owEng = makeEngine([], 5);
owEng.addItem({ name: 'Heavy', weight: 3 });
assert.strictEqual(owEng.isOverweight(), false);
owEng.addItem({ name: 'Another', weight: 3 });
assert.strictEqual(owEng.carryWeight(), 6);
assert.strictEqual(owEng.isOverweight(), true);
console.log('✓ InventoryEngine isOverweight OK');

// getGrayedSlots: solo items apilables que ya están al máximo.
const grayEng = makeEngine([
  { name: 'Apple', weight: 1, maxStack: 5, quantity: 5, stackable: true },
  { name: 'Pear', weight: 1, maxStack: 5, quantity: 3, stackable: true },
  { name: 'Rock', weight: 5, maxStack: 5, quantity: 5, stackable: false },
], 100);
const grayed = grayEng.getGrayedSlots();
assert.strictEqual(grayed.length, 1);
assert.strictEqual(grayed[0], grayEng.items[0].id);
console.log('✓ InventoryEngine getGrayedSlots OK');

// getSummary.carryPercent redondeado.
const pctEng = makeEngine([], 100);
pctEng.addItem({ name: 'Load', weight: 50 });
assert.strictEqual(pctEng.getSummary().carryPercent, 50);
assert.strictEqual(pctEng.getSummary().canCarryMore, true);
console.log('✓ InventoryEngine getSummary OK');

// initialItems: se normalizan a InventoryItem y suman peso.
const starterEng = makeEngine([
  { name: 'Rusty Sword', weight: 5, type: 'weapon' },
  { name: 'Apple', weight: 1, maxStack: 10, quantity: 3 },
], 50);
assert.strictEqual(starterEng.items.length, 2);
assert.strictEqual(starterEng.carryWeight(), 8);
assert.ok(starterEng.items.every(i => i instanceof InventoryItem));
console.log('✓ InventoryEngine with initial items OK');

// onDirty: notifica en cada cambio y el unsubscribe detiene las llamadas.
let calls = 0;
let captured = null;
const unsub = starterEng.onDirty((sum) => { calls += 1; captured = sum; });
starterEng.addItem({ name: 'Test', weight: 1 });
assert.strictEqual(calls, 1);
assert.strictEqual(captured.canCarryMore, true);
assert.strictEqual(captured.totalWeight, 9);
starterEng.removeSlot(0);
assert.strictEqual(calls, 2, 'removeSlot también notifica');
unsub();
starterEng.addItem({ name: 'After Unsub', weight: 1 });
assert.strictEqual(calls, 2, 'tras unsub no hay más notificaciones');
console.log('✓ InventoryEngine onDirty OK');

// --- Resumen ---
console.log('\n✅ All inventoryEngine tests passed!');
console.log('='.repeat(40));

// Exit code 0 para CI
process.exit(0);
