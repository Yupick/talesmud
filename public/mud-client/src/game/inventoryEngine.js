/**
 * InventoryEngine — motor puro de inventario sin dependencias Svelte.
 * Proporciona: capacidad de peso, añadir/quitar items, stacking, validación.
 * 
 * NOTA: La UI (InventoryWidget.svelte) se conecta a este motor mediante el store.
 * Este módulo NO importará de Svelte para poder ser testeado independientemente.
 */

// --- Tipos / interfaces ---

/** Un item individual en el inventario. */
export class InventoryItem {
  /**
   * @param {Object} data
   * @param {string} data.id        -- identificador único (UUID o nombre+suffix)
   * @param {string} data.name      -- nombre visible
   * @param {number} [data.weight=1] -- peso unitario
   * @param {number} [data.maxStack=1] -- máximo apilable (1 = no apilable)
   * @param {number} [data.quantity=1] -- cantidad actual (1..maxStack)
   * @param {string} [data.type='other'] -- tipo: weapon, armor, consumable, quest, other
   * @param {string} [data.slot='inventory'] -- slot donde está equipado/usado
   * @param {string} [data.quality='normal'] -- calidad: normal, magic, rare, legendary, mythic
   * @param {boolean} [data.stackable=false] -- ¿se pueden apilar unidades?
   * @param {string} [data.instanceSuffix=''] -- suffix para items instanciados (name-suffix)
   */
  constructor(data = {}) {
    this.id = data.id || `${data.name}-${Date.now()}-${Math.random().toString(36).slice(2)}`;
    this.name = data.name || 'Unnamed Item';
    this.weight = Number(data.weight) || 1;
    this.maxStack = Number(data.maxStack) || 1;
    this.quantity = Math.min(Math.max(Number(data.quantity) || 1, 1), this.maxStack);
    this.type = data.type || 'other';
    this.slot = data.slot || 'inventory';
    this.quality = String(data.quality || 'normal').toLowerCase();
    this.stackable = data.stackable !== false; // default: true a menos que maxStack===1
    this.instanceSuffix = String(data.instanceSuffix || '');
  }

  /** Nombre para display: "Name (suffix)" o solo "Name" */
  get displayName() {
    if (this.instanceSuffix) return `${this.name}-${this.instanceSuffix}`;
    return this.name;
  }

  /** Peso total de esta instancia (quantity * weight) */
  get totalWeight() {
    return this.quantity * this.weight;
  }

  /** ¿Es apilable y ya está al máximo? */
  get isMaxStack() {
    return this.stackable && this.quantity >= this.maxStack;
  }

  /** ¿Es apilable y hay espacio para más? */
  get hasRoomForMore() {
    return this.stackable && this.quantity < this.maxStack;
  }

  /** Copia superficial */
  clone() {
    return new InventoryItem({
      id: this.id,
      name: this.name,
      weight: this.weight,
      maxStack: this.maxStack,
      quantity: this.quantity,
      type: this.type,
      slot: this.slot,
      quality: this.quality,
      stackable: this.stackable,
      instanceSuffix: this.instanceSuffix,
    });
  }
}

/** Motor de inventario completo. */
export class InventoryEngine {
  /**
   * @param {Object} opts
   * @param {InventoryItem[]} [opts.initialItems=[]] — items iniciales
   * @param {number} [opts.maxWeight=1000] — peso máximo que el personaje puede cargar
   */
  constructor(opts = {}) {
    this.items = (opts.initialItems || []).map(i => new InventoryItem(i));
    this.maxWeight = Number(opts.maxWeight) || 1000;
    this._dirtyListeners = [];
  }

  /** Registrar callback que se llama cuando el inventario cambia. */
  onDirty(callback) {
    this._dirtyListeners.push(callback);
    return () => {
      this._dirtyListeners = this._dirtyListeners.filter(c => c !== callback);
    };
  }

  /** Notificar a todos los listeners de cambios. */
  _notifyDirty() {
    for (const cb of this._dirtyListeners) {
      try { cb(this.getSummary()); } catch (e) { /* ignore */ }
    }
  }

  /** Resumen rápido: { totalWeight, maxWeight, carryPercent, canCarryMore } */
  getSummary() {
    const total = this.carryWeight();
    return {
      totalWeight: total,
      maxWeight: this.maxWeight,
      carryPercent: this.maxWeight > 0 ? Math.round((total / this.maxWeight) * 100) : 0,
      canCarryMore: total < this.maxWeight,
    };
  }

  /** Peso total actualmente cargado. */
  carryWeight() {
    return this.items.reduce((sum, it) => sum + it.totalWeight, 0);
  }

  /** ¿Lleva más peso del permitido? */
  isOverweight() {
    return this.carryWeight() > this.maxWeight;
  }

  /** ¿Puede cargar peso adicional? */
  canCarry(additionalWeight) {
    return this.carryWeight() + additionalWeight <= this.maxWeight;
  }

  /** -----------------------------
   *  Operaciones de items
   *  ----------------------------- */

  /** Añadir un item (o incrementar cantidad si ya existe y es apilable). */
  addItem(itemData) {
    const item = new InventoryItem(itemData);

    // Buscar si ya existe un item apilable idéntico
    const existingIdx = this.items.findIndex(
      it => it.id === item.id && it.isMaxStack === false ? false : it.hasRoomForMore
    );

    if (existingIdx >= 0 && this.items[existingIdx].hasRoomForMore) {
      const newQty = Math.min(this.items[existingIdx].quantity + item.quantity, this.items[existingIdx].maxStack);
      this.items[existingIdx].quantity = newQty;
    } else {
      // Buscar slot vacío o nuevo item
      const insertIdx = this.items.findIndex(it => it.id === item.id && it.isMaxStack);
      if (insertIdx >= 0) {
        // Reemplazar item existente (por ejemplo, upgraded)
        this.items[insertIdx] = item;
      } else {
        // Añadir nuevo item al final
        this.items.push(item);
      }
    }

    this._notifyDirty();
    return this.getSummary();
  }

  /** Remover items por slot/ID. Devuelve { removed, remainingItem o null }.
   *  @param {Object} params
   *  @param {string} params.id - ID del item a remover
   *  @param {number} [params.quantity=1] - cantidad a remover (1..item.quantity)
   *  @param {boolean} [params.keepID=false] - si true, remueve el slot pero deja el ID para reuso
   *  @returns {{removed: InventoryItem, remaining: InventoryItem|null}} */
  removeItem({ id, quantity = 1, keepID = false }) {
    const idx = this.items.findIndex(it => it.id === id);
    if (idx < 0) return { removed: null, remaining: null };

    const target = this.items[idx];
    const removeQty = Math.min(quantity, target.quantity);

    if (removeQty >= target.quantity) {
      // Remover completamente
      const removed = target;
      this.items.splice(idx, 1);
      this._notifyDirty();
      return { removed, remaining: null };
    } else {
      // Reducir cantidad
      target.quantity -= removeQty;
      this._notifyDirty();
      return { removed: null, remaining: target.clone() };
    }
  }

  /** Remover un slot específico (por índice en el array de UI). */
  removeSlot(index) {
    if (index < 0 || index >= this.items.length) return false;
    const removed = this.items.splice(index, 1)[0];
    this._notifyDirty();
    return removed;
  }

  /** -----------------------------
   *  Consulta y filtrado
   *  ----------------------------- */

  /** Items de un tipo/slot específico. */
  getItemsByType(type) {
    return this.items.filter(it => it.type === type);
  }

  /** Items equipados (slot !== 'inventory' y slot !== 'container' y slot !== 'purse'). */
  getEquippedItems() {
    return this.items.filter(it => it.slot && it.slot !== 'inventory' && it.slot !== 'container' && it.slot !== 'purse');
  }

  /** Items en slots de equipo específicos. */
  getItemsBySlot(slot) {
    return this.items.filter(it => it.slot === slot);
  }

  /** Buscar item por nombre (case-insensitive, parcial). */
  findItemByName(query) {
    const q = String(query || '').toLowerCase().trim();
    if (!q) return [];
    return this.items.filter(it => it.name.toLowerCase().includes(q));
  }

  /** Buscar item por exactitud de ID. */
  findItemById(id) {
    return this.items.find(it => it.id === id) || null;
  }

  /** -----------------------------
   *  Validación / guardrails
   *  ----------------------------- */

  /** ¿El personaje puede cargar este item añadido a lo actual? */
  canAddItem(itemData) {
    const item = new InventoryItem(itemData);
    const newTotal = this.carryWeight() + item.totalWeight;
    return newTotal <= this.maxWeight;
  }

  /** Peso restante disponible. */
  getRemainingWeight() {
    return this.maxWeight - this.carryWeight();
  }

  /** ¿Lleva exactamente el máximo? */
  isMaxWeightReached() {
    return this.carryWeight() >= this.maxWeight;
  }

  /** Lista de slots grisesados (items que no pueden recibir más). */
  getGrayedSlots() {
    return this.items
      .filter(it => it.stackable && it.isMaxStack)
      .map(it => it.id);
  }
}

// --- Exportar para uso en módulos browser ---
// Los globals se asignan abajo solo si estamos en navegador (type=module o browser).
// Las clases InventoryItem e InventoryEngine ya están exportadas arriba (lines 12, 78).
// Si estuvieras en navegador, window.InventoryEngine / window.InventoryItem se poblarían
// por el bloque conditional al final de este archivo (si estuviera presente).