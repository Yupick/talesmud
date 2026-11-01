/**
 * Dungeon Builder model (FASE 5.1 - Batch 17, initial cut).
 *
 * Pure module: grid model, operations, validation, serialization.
 * All operations are deterministic and testable with node --test.
 * No DOM dependencies.
 */

export const CELL_TYPES = [
  'empty',
  'room',
  'corridor',
  'door',
  'entrance',
  'exit',
];

export const DUNGEON_STORAGE_KEY = 'mud-client.dungeon-builder.v1';

export const DEFAULT_GRID = {
  width: 16,
  height: 16,
};

export function clone(value) {
  return JSON.parse(JSON.stringify(value));
}

function intOr(value, fallback) {
  const n = typeof value === 'number' ? value : Number.parseFloat(value);
  return Number.isFinite(n) ? Math.trunc(n) : fallback;
}

export function createCell(type = 'empty') {
  const t = CELL_TYPES.includes(type) ? type : 'empty';
  return {
    type: t,
    roomId: null,
    encounter: null,
    loot: null,
  };
}

export function createGrid(width = DEFAULT_GRID.width, height = DEFAULT_GRID.height) {
  const w = intOr(width, DEFAULT_GRID.width);
  const h = intOr(height, DEFAULT_GRID.height);
  const cells = Array(h)
    .fill()
    .map(() =>
      Array(w)
        .fill()
        .map(() => createCell('empty'))
    );
  return {
    width: w,
    height: h,
    cells,
    rooms: [],
    corridors: [],
    entrances: [],
    exits: [],
    encounters: [],
    lootTables: [],
    meta: {},
  };
}

export function isInBounds(grid, x, y) {
  return x >= 0 && x < grid.width && y >= 0 && y < grid.height;
}

export function getCell(grid, x, y) {
  if (!isInBounds(grid, x, y)) return null;
  return grid.cells[y][x];
}

export function setCell(grid, x, y, type = 'empty', opts = {}) {
  if (!isInBounds(grid, x, y)) return false;
  const cell = grid.cells[y][x];
  const t = CELL_TYPES.includes(type) ? type : cell.type;
  cell.type = t;
  if (opts.roomId !== undefined) cell.roomId = opts.roomId;
  if (opts.encounter !== undefined) cell.encounter = opts.encounter;
  if (opts.loot !== undefined) cell.loot = opts.loot;
  return true;
}

export function paintCell(grid, x, y, type) {
  if (!isInBounds(grid, x, y)) return 0;
  const startType = grid.cells[y][x].type;
  const newType = CELL_TYPES.includes(type) ? type : startType;
  if (startType === newType) return 0;
  const visited = new Set();
  const queue = [[x, y]];
  visited.add(`${x},${y}`);
  let count = 0;
  let head = 0;
  while (head < queue.length) {
    const [cx, cy] = queue[head++];
    grid.cells[cy][cx].type = newType;
    count++;
    const neighbors = [
      [cx + 1, cy],
      [cx - 1, cy],
      [cx, cy + 1],
      [cx, cy - 1],
    ];
    for (const [nx, ny] of neighbors) {
      if (isInBounds(grid, nx, ny) && !visited.has(`${nx},${ny}`)) {
        if (grid.cells[ny][nx].type === startType) {
          visited.add(`${nx},${ny}`);
          queue.push([nx, ny]);
        }
      }
    }
  }
  return count;
}

export function addRoom(grid, { x, y, w, h }) {
  const rx = intOr(x, 0);
  const ry = intOr(y, 0);
  const rw = intOr(w, 1);
  const rh = intOr(h, 1);
  const roomId = `room_${grid.rooms.length + 1}`;
  grid.rooms.push({ id: roomId, x: rx, y: ry, w: rw, h: rh });
  for (let dy = 0; dy < rh; dy++) {
    for (let dx = 0; dx < rw; dx++) {
      const cx = rx + dx;
      const cy = ry + dy;
      if (isInBounds(grid, cx, cy)) {
        grid.cells[cy][cx].type = 'room';
        grid.cells[cy][cx].roomId = roomId;
      }
    }
  }
  return roomId;
}

export function addCorridor(grid, points = []) {
  if (!Array.isArray(points) || points.length < 1) return false;
  const corridor = { points: [] };
  for (const p of points) {
    if (Array.isArray(p) && p.length >= 2) {
      const x = intOr(p[0], -1);
      const y = intOr(p[1], -1);
      if (isInBounds(grid, x, y)) {
        const cell = grid.cells[y][x];
        if (cell.type !== 'entrance' && cell.type !== 'exit' && cell.type !== 'door') {
          grid.cells[y][x].type = 'corridor';
        }
        corridor.points.push([x, y]);
      }
    }
  }
  if (corridor.points.length > 0) {
    grid.corridors.push(corridor);
    return true;
  }
  return false;
}

export function addEncounter(grid, { x, y, npcTemplateId = '', count = 1, level = 1 }) {
  if (!isInBounds(grid, x, y)) return null;
  const enc = {
    id: `enc_${grid.encounters.length + 1}`,
    cell: { x, y },
    npcTemplateId: String(npcTemplateId).trim(),
    count: intOr(count, 1),
    level: intOr(level, 1),
  };
  grid.encounters.push(enc);
  grid.cells[y][x].encounter = {
    npcTemplateId: enc.npcTemplateId,
    count: enc.count,
    level: enc.level,
  };
  return enc;
}

export function addLootTable(grid, lootTable = {}) {
  const table = {
    name: String(lootTable.name || `table_${grid.lootTables.length + 1}`),
    entries: Array.isArray(lootTable.entries)
      ? lootTable.entries.map((e) => ({
          itemId: String(e.itemId || '').trim(),
          chance: intOr(e.chance, 100),
          qty: intOr(e.qty || e.quantity, 1),
        }))
      : [],
  };
  grid.lootTables.push(table);
  return table;
}

function isPassable(cell) {
  if (!cell) return false;
  const type = cell.type;
  if (type === 'empty') return false;
  return true;
}

export function findPathBFS(grid, start, end) {
  if (!isInBounds(grid, start.x, start.y) || !isInBounds(grid, end.x, end.y)) return null;
  const startKey = `${start.x},${start.y}`;
  const endKey = `${end.x},${end.y}`;
  if (startKey === endKey) return [{ x: start.x, y: start.y }];
  const visited = new Set([startKey]);
  const cameFrom = new Map();
  const queue = [[start.x, start.y]];
  let head = 0;
  while (head < queue.length) {
    const [cx, cy] = queue[head++];
    if (cx === end.x && cy === end.y) {
      break;
    }
    const neighbors = [
      [cx + 1, cy],
      [cx - 1, cy],
      [cx, cy + 1],
      [cx, cy - 1],
    ];
    for (const [nx, ny] of neighbors) {
      const key = `${nx},${ny}`;
      if (!visited.has(key) && isInBounds(grid, nx, ny)) {
        if (isPassable(grid.cells[ny][nx])) {
          visited.add(key);
          cameFrom.set(key, `${cx},${cy}`);
          queue.push([nx, ny]);
        }
      }
    }
  }
  if (!visited.has(endKey)) return null;
  const pathKeys = [];
  let curKey = endKey;
  const seen = new Set();
  while (curKey !== startKey) {
    if (seen.has(curKey)) break;
    seen.add(curKey);
    pathKeys.push(curKey);
    const prev = cameFrom.get(curKey);
    if (!prev) break;
    curKey = prev;
  }
  pathKeys.push(startKey);
  pathKeys.reverse();
  if (pathKeys[0] !== startKey || pathKeys[pathKeys.length - 1] !== endKey) {
    return null;
  }
  return pathKeys.map((k) => {
    const [x, y] = k.split(',').map(Number);
    return { x, y };
  });
}

export function autoValidate(grid) {
  const issues = [];
  let entrances = [];
  let exits = [];
  for (let y = 0; y < grid.height; y++) {
    for (let x = 0; x < grid.width; x++) {
      if (grid.cells[y][x].type === 'entrance') entrances.push({ x, y });
      if (grid.cells[y][x].type === 'exit') exits.push({ x, y });
    }
  }
  grid.entrances = entrances;
  grid.exits = exits;
  if (entrances.length < 1) {
    issues.push({ level: 'error', msg: 'At least 1 entrance required' });
  }
  if (exits.length < 1) {
    issues.push({ level: 'error', msg: 'At least 1 exit required' });
  }
  if (entrances.length > 0 && exits.length > 0) {
    let connected = false;
    for (const ent of entrances) {
      for (const ex of exits) {
        const path = findPathBFS(grid, ent, ex);
        if (path !== null) {
          connected = true;
          break;
        }
      }
      if (connected) break;
    }
    if (!connected) {
      issues.push({ level: 'error', msg: 'No valid path from entrance to exit' });
    }
  }
  for (let i = 0; i < grid.rooms.length; i++) {
    const r1 = grid.rooms[i];
    for (let j = i + 1; j < grid.rooms.length; j++) {
      const r2 = grid.rooms[j];
      if (
        r1.x < r2.x + r2.w &&
        r1.x + r1.w > r2.x &&
        r1.y < r2.y + r2.h &&
        r1.y + r1.h > r2.y
      ) {
        issues.push({ level: 'error', msg: `Rooms ${r1.id} and ${r2.id} overlap` });
      }
    }
  }
  return issues;
}

export function toJSON(grid) {
  return JSON.stringify(grid);
}

export function fromJSON(json) {
  try {
    const parsed = typeof json === 'string' ? JSON.parse(json) : json;
    if (!parsed || typeof parsed !== 'object') return null;
    const grid = createGrid(parsed.width, parsed.height);
    if (Array.isArray(parsed.cells)) {
      for (let y = 0; y < Math.min(grid.height, parsed.cells.length); y++) {
        if (Array.isArray(parsed.cells[y])) {
          for (let x = 0; x < Math.min(grid.width, parsed.cells[y].length); x++) {
            const c = parsed.cells[y][x];
            if (c && typeof c === 'object') {
              grid.cells[y][x] = createCell(c.type);
              if (c.roomId !== undefined) grid.cells[y][x].roomId = c.roomId;
              if (c.encounter !== undefined) grid.cells[y][x].encounter = c.encounter;
              if (c.loot !== undefined) grid.cells[y][x].loot = c.loot;
            }
          }
        }
      }
    }
    grid.rooms = Array.isArray(parsed.rooms) ? clone(parsed.rooms) : [];
    grid.corridors = Array.isArray(parsed.corridors) ? clone(parsed.corridors) : [];
    grid.entrances = Array.isArray(parsed.entrances) ? clone(parsed.entrances) : [];
    grid.exits = Array.isArray(parsed.exits) ? clone(parsed.exits) : [];
    grid.encounters = Array.isArray(parsed.encounters) ? clone(parsed.encounters) : [];
    grid.lootTables = Array.isArray(parsed.lootTables) ? clone(parsed.lootTables) : [];
    grid.meta = parsed.meta || {};
    return grid;
  } catch (e) {
    return null;
  }
}

export function serialize(grid) {
  return toJSON(grid);
}

export function deserialize(str) {
  return fromJSON(str);
}

export function exportDefinition(grid) {
  const def = {
    version: 1,
    grid: {
      width: grid.width,
      height: grid.height,
    },
    cells: grid.cells.map((row) => row.map((c) => ({
      type: c.type,
      roomId: c.roomId,
      encounter: c.encounter,
      loot: c.loot,
    }))),
    rooms: grid.rooms,
    corridors: grid.corridors,
    entrances: grid.entrances,
    exits: grid.exits,
    encounters: grid.encounters,
    lootTables: grid.lootTables,
    meta: grid.meta,
  };
  return def;
}

export default {
  createGrid,
  createCell,
  getCell,
  setCell,
  paintCell,
  addRoom,
  addCorridor,
  addEncounter,
  addLootTable,
  autoValidate,
  findPathBFS,
  toJSON,
  fromJSON,
  serialize,
  deserialize,
  exportDefinition,
  CELL_TYPES,
};
