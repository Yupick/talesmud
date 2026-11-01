import assert from 'assert';
import {
  DEFAULT_PAGE_SIZE,
  CSV_COLUMNS,
  filterEvents,
  summarizeEvent,
  eventTimestamp,
  dayStart,
  dayEnd,
  uniqueActions,
  uniqueEntityTypes,
  paginate,
  csvCell,
  toCSV,
} from './auditView.js';

const rows = [
  {
    id: 'a1',
    time: '2024-03-01T10:00:00Z',
    actorUserId: 'u1',
    actorName: 'Alice',
    action: 'create',
    entityType: 'rooms',
    entityId: 'room-1',
    source: 'creator',
  },
  {
    id: 'a2',
    time: '2024-03-02T12:30:00Z',
    actorUserId: 'u2',
    actorName: 'Bob',
    action: 'update',
    entityType: 'items',
    entityId: 'sword',
    summary: 'update items sword',
    source: 'creator',
  },
  {
    id: 'a3',
    time: '2024-03-03T08:00:00Z',
    actorUserId: 'u1',
    actorName: 'Alice',
    action: 'delete',
    entityType: 'rooms',
    entityId: 'room-2',
    source: 'ops',
  },
];

// --- summarizeEvent -------------------------------------------------------
assert.equal(summarizeEvent(rows[0]), 'create rooms room-1');
assert.equal(summarizeEvent(rows[1]), 'update items sword', 'explicit summary wins');
assert.equal(summarizeEvent(null), '');

// --- eventTimestamp -------------------------------------------------------
assert.equal(eventTimestamp(rows[0]), Date.parse('2024-03-01T10:00:00Z'));
assert.equal(eventTimestamp({ time: 'not-a-date' }), 0);
assert.equal(eventTimestamp({}), 0);
assert.equal(eventTimestamp({ time: new Date('2024-03-01T10:00:00Z') }), Date.parse('2024-03-01T10:00:00Z'));

// --- day helpers ----------------------------------------------------------
assert.equal(dayStart('2024-03-01'), new Date(2024, 2, 1, 0, 0, 0, 0).getTime());
assert.equal(dayEnd('2024-03-01'), new Date(2024, 2, 1, 23, 59, 59, 999).getTime());
assert.equal(dayStart(''), null);
assert.equal(dayStart('03/01/2024'), null);

// --- filterEvents: no criteria returns everything -------------------------
assert.deepEqual(filterEvents(rows), rows);
assert.deepEqual(filterEvents(rows, {}), rows);
assert.deepEqual(filterEvents(null, {}), []);

// --- filterEvents: action exact, case-insensitive -------------------------
assert.deepEqual(
  filterEvents(rows, { action: 'CREATE' }).map((r) => r.id),
  ['a1'],
);
assert.deepEqual(filterEvents(rows, { action: 'missing' }), []);

// --- filterEvents: actor substring over name and id -----------------------
assert.deepEqual(
  filterEvents(rows, { actor: 'ali' }).map((r) => r.id),
  ['a1', 'a3'],
);
assert.deepEqual(
  filterEvents(rows, { actor: 'u2' }).map((r) => r.id),
  ['a2'],
);

// --- filterEvents: entity substring over type + id ------------------------
assert.deepEqual(
  filterEvents(rows, { entity: 'rooms' }).map((r) => r.id),
  ['a1', 'a3'],
);
assert.deepEqual(
  filterEvents(rows, { entity: 'room-2' }).map((r) => r.id),
  ['a3'],
);

// --- filterEvents: free-text query ----------------------------------------
assert.deepEqual(
  filterEvents(rows, { query: 'sword' }).map((r) => r.id),
  ['a2'],
);
assert.deepEqual(
  filterEvents(rows, { query: 'update items' }).map((r) => r.id),
  ['a2'],
);
assert.deepEqual(filterEvents(rows, { query: 'nope' }), []);

// --- filterEvents: date range (TZ-robust: far-apart years) ----------------
{
  const dated = [
    { ...rows[0], id: 'y2020', time: '2020-06-15T12:00:00Z' },
    { ...rows[0], id: 'y2030', time: '2030-06-15T12:00:00Z' },
  ];
  assert.deepEqual(
    filterEvents(dated, { from: '2019-01-01', to: '2025-01-01' }).map((r) => r.id),
    ['y2020'],
  );
  assert.deepEqual(
    filterEvents(dated, { from: '2025-01-01' }).map((r) => r.id),
    ['y2030'],
  );
  assert.deepEqual(
    filterEvents(dated, { to: '2025-01-01' }).map((r) => r.id),
    ['y2020'],
  );
}

// --- filterEvents: combined criteria AND together -------------------------
assert.deepEqual(
  filterEvents(rows, { actor: 'alice', entity: 'rooms', action: 'delete' }).map((r) => r.id),
  ['a3'],
);

// --- uniqueActions / uniqueEntityTypes ------------------------------------
assert.deepEqual(uniqueActions(rows), ['create', 'delete', 'update']);
assert.deepEqual(uniqueEntityTypes(rows), ['items', 'rooms']);
assert.deepEqual(uniqueActions(null), []);

// --- paginate -------------------------------------------------------------
{
  const items = Array.from({ length: 12 }, (_, i) => i);
  const first = paginate(items, 1, 5);
  assert.deepEqual(first.items, [0, 1, 2, 3, 4]);
  assert.equal(first.page, 1);
  assert.equal(first.pages, 3);
  assert.equal(first.total, 12);
  assert.equal(first.hasPrev, false);
  assert.equal(first.hasNext, true);

  const last = paginate(items, 3, 5);
  assert.deepEqual(last.items, [10, 11]);
  assert.equal(last.hasNext, false);
  assert.equal(last.hasPrev, true);

  // Out-of-range pages clamp instead of returning nothing.
  assert.equal(paginate(items, 99, 5).page, 3);
  assert.equal(paginate(items, -4, 5).page, 1);

  // Bad page size falls back to the default.
  assert.equal(paginate(items, 1, 0).pageSize, DEFAULT_PAGE_SIZE);
  assert.equal(paginate(items, 1).pageSize, DEFAULT_PAGE_SIZE);

  const empty = paginate([], 1, 5);
  assert.deepEqual(empty.items, []);
  assert.equal(empty.pages, 1);
  assert.equal(empty.total, 0);
}

// --- csvCell --------------------------------------------------------------
assert.equal(csvCell('plain'), 'plain');
assert.equal(csvCell('a,b'), '"a,b"');
assert.equal(csvCell('say "hi"'), '"say ""hi"""');
assert.equal(csvCell('line\nbreak'), '"line\nbreak"');
assert.equal(csvCell(null), '');

// --- toCSV ----------------------------------------------------------------
{
  const csv = toCSV(rows);
  const lines = csv.split('\n');
  assert.equal(lines[0], CSV_COLUMNS.map((c) => c.label).join(','));
  assert.equal(lines.length, rows.length + 1);
  assert.ok(lines[1].startsWith('2024-03-01T10:00:00Z,Alice,create,rooms,room-1,creator'));

  // Empty data still yields the header row.
  assert.equal(toCSV([]), CSV_COLUMNS.map((c) => c.label).join(','));
  assert.equal(toCSV(null), CSV_COLUMNS.map((c) => c.label).join(','));

  // A comma in the summary is quoted.
  const tricky = toCSV([{ id: 'x', summary: 'a,b', action: 'create' }]);
  assert.ok(tricky.includes('"a,b"'));
}

console.log('auditView_test ok');
