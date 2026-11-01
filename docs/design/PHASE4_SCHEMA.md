# PHASE 4 Schema Design: Sistema Social (Clanes + Mensajería Privada)

**Fecha**: 2026-10-12  
**Estado**: Implementado (Fase 4) — con dos huecos abiertos (ver abajo)  
**Base**: TalesMUD v0.5.0, SQLite vía `modernc.org/sqlite`  
**Patrón**: `(id TEXT PRIMARY KEY, data TEXT NOT NULL)` con contenido JSON

> **Actualizado 2026-10-15.** La cabecera original decía **"Diseño (no implementado)"**: ya no es
> así. Verificado con `grep`/lectura, el diseño de este documento existe en el código:
>
> - **Migraciones**: `migratePhase4SocialTables()` en `pkg/db/sqlite/sqlite.go` crea `clans`,
>   `clan_members`, `clan_invites`, `clan_bank_ledger`, `dm_channels` y `dm_messages` (con columnas
>   reales `channel_id`/`sent_at`/`from_char_id` + índices, tal como decide §2.1).
> - **Entidades / repos / servicios**: `pkg/entities/clan/`, `pkg/entities/dm/`,
>   `pkg/repository/clans_sqlite.go`, `pkg/repository/dm_sqlite.go` (+ tests), `pkg/service/clans.go`,
>   `pkg/service/dms.go` (+ tests), `pkg/server/handler/clans.go`, `pkg/server/handler/dms.go`
>   (10 rutas `/api/clans*` y 5 rutas `/api/dm/*`).
> - **UI**: `public/mud-client/src/game/ui/ClanPanel.svelte` (tecla `C`) y `DMsPanel.svelte`
>   (tecla `M`, sondeo 5 s), con `src/api/clans.js` y `src/api/dms.js`.
>
> **Huecos siguen abiertos (verificados 2026-10-15, no resueltos por la Db/Backend):**
>
> 1. **No existe comando MUD `clan …`**: no hay `ClansCommand` ni registro de la clave `"clan"` en
>    `pkg/mudserver/game/commands/` (solo aparece `"clan"` en el JSON del handler REST). Los clanes
>    se gestionan por REST + panel; el chat de clan tampoco existe.
> 2. **`tell` sigue en memoria**: `pkg/mudserver/game/commands/tell.go` sigue resolviendo al
>    destinatario con `game.FindOnlinePlayerByName` y descarta el mensaje si no está online; no usa
>    `dm_channels`/`dm_messages` ni `pkg/service/dms.go`. La persistencia existe solo en la vía
>    DM (API + panel), no en el comando `tell`.

---

## 1. Contexto y Estado Actual

### 1.1 Cómo funciona hoy los `tell`/DMs

El comando `tell` en `pkg/mudserver/game/commands/tell.go` es **completamente en memoria**:

1. Busca al destinatario con `game.FindOnlinePlayerByName(targetName)` (solo jugadores online)
2. Envía el mensaje vía `game.SendMessage() <- messages.Reply(targetPlayer.UserID, ...)`
3. Envía confirmación al remitente
4. **No hay persistencia**: si el target está offline, el mensaje se pierde. Si el servidor se reinicia, todo se pierde

> **2026-10-15**: sigue siendo cierto **para el comando `tell`** (verificado leyendo `tell.go`). Lo
> que ya no es cierto es que "los DMs" sean solo esto: existe una segunda vía persistente
> (`dm_channels`/`dm_messages` → `pkg/service/dms.go` → `/api/dm/*` → `DMsPanel.svelte`). El hueco
> es de integración: `tell` no está cableado a esa vía.

### 1.2 Qué falta para persistencia

- **Canales de conversación**: registro de quién habla con qué
- **Mensajes persistentes**: almacenamiento del cuerpo, timestamp, y estado de lectura
- **Consulta por canal+timestamp**: necesita índices reales (no solo json_extract)

> **Estado 2026-10-15: los tres puntos ya están implementados** en `dm_channels` / `dm_messages`
> (migración + índices reales en `pkg/db/sqlite/sqlite.go`, repo `pkg/repository/dm_sqlite.go`,
> servicio `pkg/service/dms.go`). Esta lista describe el punto de partida del diseño, no el trabajo
> pendiente. El pendiente real es otra cosa: **conectar el comando `tell` a esa persistencia**
> (ver cabecera).

### 1.3 Patrón actual del repo

```go
// Todas las tablas siguen este patrón:
CREATE TABLE IF NOT EXISTS <name> (id TEXT PRIMARY KEY, data TEXT NOT NULL);

// Los índices se crean sobre campos JSON:
CREATE INDEX IF NOT EXISTS idx_<table>_<field> ON <table>(json_extract(data, '$.<field>'));
```

**Limitación**: `json_extract` no es eficiente para consultas de rango ni para ordenamiento por timestamp. Para `dm_messages`, necesitamos columnas reales.

---

## 2. Diseño de Tablas

### 2.1 Decisión de Patrón: ¿Desviarse del JSON puro?

**Decisión: SÍ, desviarse parcialmente para `dm_messages`.**

Justificación:
- `dm_messages` requiere consultas frecuentes por rango de tiempo (últimos N mensajes desde timestamp X)
- `json_extract(data, '$.sentAt')` no usa índices eficientemente para `ORDER BY` y `BETWEEN`
- El patrón JSON puro funciona para entidades completas, no para colecciones de alta escritura
- SQLite soporta columnas generadas (STORED) que mantienen la indexación automática sin lógica extra

**Para las demás tablas (clanes, miembros, invites, bank_ledger)**: mantenemos el patrón puro `(id, data)` con índices `json_extract` ya que las consultas son por igualdad, no por rango.

### 2.2 DDL SQL Completo

```sql
-- ============================================================
-- FASE 4: Sistema Social - DDL
-- ============================================================

-- ---------- CLANES ----------

-- Información principal del clan
CREATE TABLE IF NOT EXISTS clans (
    id TEXT PRIMARY KEY,
    data TEXT NOT NULL
);

-- Miembros del clan con su rango y estadísticas
CREATE TABLE IF NOT EXISTS clan_members (
    id TEXT PRIMARY KEY,
    data TEXT NOT NULL
);

-- Invitaciones pendientes
CREATE TABLE IF NOT EXISTS clan_invites (
    id TEXT PRIMARY KEY,
    data TEXT NOT NULL
);

-- Ledger de auditoría del banco del clan
CREATE TABLE IF NOT EXISTS clan_bank_ledger (
    id TEXT PRIMARY KEY,
    data TEXT NOT NULL
);

-- ---------- MENSAJERÍA PRIVADA ----------

-- Canales de conversación entre jugadores
CREATE TABLE IF NOT EXISTS dm_channels (
    id TEXT PRIMARY KEY,
    data TEXT NOT NULL
);

-- Mensajes individuales (desviación controlada del patrón)
CREATE TABLE IF NOT EXISTS dm_messages (
    id TEXT PRIMARY KEY,
    data TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    sent_at INTEGER NOT NULL,
    from_char_id TEXT NOT NULL
);

-- Índices para dm_messages (columnas reales)
CREATE INDEX IF NOT EXISTS idx_dm_messages_channel_time 
    ON dm_messages(channel_id, sent_at DESC);
CREATE INDEX IF NOT EXISTS idx_dm_messages_channel_sender 
    ON dm_messages(channel_id, from_char_id);
CREATE INDEX IF NOT EXISTS idx_dm_messages_unread 
    ON dm_messages(channel_id, sent_at) 
    WHERE json_extract(data, '$.readAt') IS NULL;

-- Índices JSON para las demás tablas
CREATE INDEX IF NOT EXISTS idx_clans_name 
    ON clans(json_extract(data, '$.name'));
CREATE INDEX IF NOT EXISTS idx_clans_tag 
    ON clans(json_extract(data, '$.tag'));
CREATE INDEX IF NOT EXISTS idx_clans_leader 
    ON clans(json_extract(data, '$.leaderCharID'));
CREATE INDEX IF NOT EXISTS idx_clans_active 
    ON clans(json_extract(data, '$.active'));

CREATE INDEX IF NOT EXISTS idx_clan_members_char 
    ON clan_members(json_extract(data, '$.characterID'));
CREATE INDEX IF NOT EXISTS idx_clan_members_clan 
    ON clan_members(json_extract(data, '$.clanID'));
CREATE UNIQUE INDEX IF NOT EXISTS idx_clan_members_unique 
    ON clan_members(json_extract(data, '$.characterID'), json_extract(data, '$.clanID'));

CREATE INDEX IF NOT EXISTS idx_clan_invites_clan 
    ON clan_invites(json_extract(data, '$.clanID'));
CREATE INDEX IF NOT EXISTS idx_clan_invites_char 
    ON clan_invites(json_extract(data, '$.characterID'));
CREATE INDEX IF NOT EXISTS idx_clan_invites_status 
    ON clan_invites(json_extract(data, '$.status'));

CREATE INDEX IF NOT EXISTS idx_clan_bank_ledger_clan 
    ON clan_bank_ledger(json_extract(data, '$.clanID'));
CREATE INDEX IF NOT EXISTS idx_clan_bank_ledger_time 
    ON clan_bank_ledger(json_extract(data, '$.timestamp'));

CREATE INDEX IF NOT EXISTS idx_dm_channels_participant 
    ON dm_channels(json_extract(data, '$.participantA'));
CREATE INDEX IF NOT EXISTS idx_dm_channels_participant_b 
    ON dm_channels(json_extract(data, '$.participantB'));
```

---

## 3. Snippet Go para `runMigrations()`

Este snippet se añade al final de `runMigrations()` en `pkg/db/sqlite/sqlite.go`:

```go
// FASE 4: Sistema Social - Clanes y Mensajería
func (c *Client) migratePhase4SocialTables() error {
    stmts := []string{
        // Clanes
        `CREATE TABLE IF NOT EXISTS clans (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
        `CREATE TABLE IF NOT EXISTS clan_members (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
        `CREATE TABLE IF NOT EXISTS clan_invites (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
        `CREATE TABLE IF NOT EXISTS clan_bank_ledger (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
        
        // Mensajería privada
        `CREATE TABLE IF NOT EXISTS dm_channels (id TEXT PRIMARY KEY, data TEXT NOT NULL);`,
        `CREATE TABLE IF NOT EXISTS dm_messages (
            id TEXT PRIMARY KEY,
            data TEXT NOT NULL,
            channel_id TEXT NOT NULL,
            sent_at INTEGER NOT NULL,
            from_char_id TEXT NOT NULL
        );`,
        
        // Índices para dm_messages
        `CREATE INDEX IF NOT EXISTS idx_dm_messages_channel_time 
            ON dm_messages(channel_id, sent_at DESC);`,
        `CREATE INDEX IF NOT EXISTS idx_dm_messages_channel_sender 
            ON dm_messages(channel_id, from_char_id);`,
        `CREATE INDEX IF NOT EXISTS idx_dm_messages_unread 
            ON dm_messages(channel_id, sent_at) 
            WHERE json_extract(data, '$.readAt') IS NULL;`,
        
        // Índices JSON para clanes
        `CREATE INDEX IF NOT EXISTS idx_clans_name ON clans(json_extract(data, '$.name'));`,
        `CREATE INDEX IF NOT EXISTS idx_clans_tag ON clans(json_extract(data, '$.tag'));`,
        `CREATE INDEX IF NOT EXISTS idx_clans_leader ON clans(json_extract(data, '$.leaderCharID'));`,
        `CREATE INDEX IF NOT EXISTS idx_clans_active ON clans(json_extract(data, '$.active'));`,
        
        // Índices JSON para miembros
        `CREATE INDEX IF NOT EXISTS idx_clan_members_char ON clan_members(json_extract(data, '$.characterID'));`,
        `CREATE INDEX IF NOT EXISTS idx_clan_members_clan ON clan_members(json_extract(data, '$.clanID'));`,
        `CREATE UNIQUE INDEX IF NOT EXISTS idx_clan_members_unique 
            ON clan_members(json_extract(data, '$.characterID'), json_extract(data, '$.clanID'));`,
        
        // Índices JSON para invitaciones
        `CREATE INDEX IF NOT EXISTS idx_clan_invites_clan ON clan_invites(json_extract(data, '$.clanID'));`,
        `CREATE INDEX IF NOT EXISTS idx_clan_invites_char ON clan_invites(json_extract(data, '$.characterID'));`,
        `CREATE INDEX IF NOT EXISTS idx_clan_invites_status ON clan_invites(json_extract(data, '$.status'));`,
        
        // Índices JSON para ledger bancario
        `CREATE INDEX IF NOT EXISTS idx_clan_bank_ledger_clan ON clan_bank_ledger(json_extract(data, '$.clanID'));`,
        `CREATE INDEX IF NOT EXISTS idx_clan_bank_ledger_time ON clan_bank_ledger(json_extract(data, '$.timestamp'));`,
        
        // Índices JSON para canales DM
        `CREATE INDEX IF NOT EXISTS idx_dm_channels_participant ON dm_channels(json_extract(data, '$.participantA'));`,
        `CREATE INDEX IF NOT EXISTS idx_dm_channels_participant_b ON dm_channels(json_extract(data, '$.participantB'));`,
    }
    
    for _, stmt := range stmts {
        if _, err := c.db.Exec(stmt); err != nil {
            return fmt.Errorf("phase4 social table migration failed: %w", err)
        }
    }
    return nil
}
```

Y en `runMigrations()`, añadir la llamada:

```go
func (c *Client) runMigrations() error {
    if err := c.migrateItemTemplates(); err != nil {
        return fmt.Errorf("item templates migration failed: %w", err)
    }
    // FASE 4: Sistema Social
    if err := c.migratePhase4SocialTables(); err != nil {
        return fmt.Errorf("phase4 social migration failed: %w", err)
    }
    return nil
}
```

---

## 4. Entidades Go (Structs con tags JSON)

### 4.1 `pkg/entities/clan/clan.go`

```go
package clan

import (
    "time"
    "github.com/talesmud/talesmud/pkg/entities"
)

// Rank define el rango de un miembro en el clan
type Rank int

const (
    RankMember Rank = iota + 1
    RankOfficer
    RankLeader
)

func (r Rank) String() string {
    switch r {
    case RankMember:  return "member"
    case RankOfficer: return "officer"
    case RankLeader:  return "leader"
    default:          return "unknown"
    }
}

// Clan es la entidad principal del clan
type Clan struct {
    *entities.Entity `json:",inline"`
    
    Name           string    `json:"name"`
    Tag            string    `json:"tag"`            // Tag corto (ej: "DRK")
    LeaderCharID   string    `json:"leaderCharID"`
    FoundedAt      time.Time `json:"foundedAt"`
    Level          int       `json:"level"`
    XP             int64     `json:"xp"`
    Bank           int64     `json:"bank"`           // Oro del clan
    MOTD           string    `json:"motd"`           // Message of the day
    Description    string    `json:"description"`
    Active         bool      `json:"active"`          // false = disuelto
    MaxMembers     int       `json:"maxMembers"`     // Límite de miembros
    CreatedAt      time.Time `json:"createdAt"`
    UpdatedAt      time.Time `json:"updatedAt"`
}

// Member representa la membresía de un personaje en un clan
type Member struct {
    *entities.Entity `json:",inline"`
    
    CharacterID   string    `json:"characterID"`
    ClanID        string    `json:"clanID"`
    Rank          Rank      `json:"rank"`
    JoinedAt      time.Time `json:"joinedAt"`
    Contribution  int64     `json:"contribution"`  // Total contribuido al clan
    LastOnline    time.Time `json:"lastOnline"`
}

// Invite representa una invitación pendiente a un clan
type Invite struct {
    *entities.Entity `json:",inline"`
    
    ClanID      string    `json:"clanID"`
    CharacterID string    `json:"characterID"`
    InviterID   string    `json:"inviterID"`   // Quién invitó
    Status      string    `json:"status"`      // "pending" | "accepted" | "declined" | "expired"
    CreatedAt   time.Time `json:"createdAt"`
    ExpiresAt   time.Time `json:"expiresAt"`
}

// BankLedgerEntry registra una transacción en el banco del clan
type BankLedgerEntry struct {
    *entities.Entity `json:",inline"`
    
    ClanID      string    `json:"clanID"`
    CharacterID string    `json:"characterID"`  // Quién realizó la acción
    Type        string    `json:"type"`         // "deposit" | "withdraw"
    Amount      int64     `json:"amount"`
    Balance     int64     `json:"balance"`      // Balance después de la transacción
    Note        string    `json:"note"`         // Descripción opcional
    Timestamp   time.Time `json:"timestamp"`
}
```

### 4.2 `pkg/entities/dm/dm.go`

```go
package dm

import (
    "time"
    "github.com/talesmud/talesmud/pkg/entities"
)

// Channel representa un canal de mensajería privada entre dos jugadores
type Channel struct {
    *entities.Entity `json:",inline"`
    
    ParticipantA   string    `json:"participantA"`   // characterID
    ParticipantB   string    `json:"participantB"`   // characterID
    CreatedAt      time.Time `json:"createdAt"`
    LastMessageAt  time.Time `json:"lastMessageAt"`
    LastMessageID  string    `json:"lastMessageID"`
    UnreadA        int       `json:"unreadA"`        // Mensajes no leídos por A
    UnreadB        int       `json:"unreadB"`        // Mensajes no leídos por B
}

// Message representa un mensaje individual en un canal DM
type Message struct {
    *entities.Entity `json:",inline"`
    
    ChannelID   string    `json:"channelID"`
    FromCharID  string    `json:"fromCharID"`
    Body        string    `json:"body"`
    SentAt      time.Time `json:"sentAt"`
    ReadAt      *time.Time `json:"readAt,omitempty"`  // nil = no leído
}

// GetReadAt devuelve el timestamp de lectura o nil
func (m *Message) GetReadAt() *time.Time {
    return m.ReadAt
}

// IsRead indica si el mensaje ha sido leído
func (m *Message) IsRead() bool {
    return m.ReadAt != nil
}

// Marca el mensaje como leído
func (m *Message) MarkRead() {
    now := time.Now()
    m.ReadAt = &now
}
```

---

## 5. Interfaces de Repositorio

### 5.1 `pkg/repository/interfaces.go` (añadir)

```go
import (
    "time"
    "github.com/talesmud/talesmud/pkg/entities/clan"
    "github.com/talesmud/talesmud/pkg/entities/dm"
)

// ClanRepository provides access to clan data
type ClanRepository interface {
    // CRUD básico
    Drop() error
    FindByID(id string) (*clan.Clan, error)
    FindByName(name string) ([]*clan.Clan, error)
    FindByTag(tag string) ([]*clan.Clan, error)
    FindByLeader(leaderCharID string) ([]*clan.Clan, error)
    FindAll() ([]*clan.Clan, error)
    FindAllActive() ([]*clan.Clan, error)
    Store(cl *clan.Clan) (*clan.Clan, error)
    Update(id string, cl *clan.Clan) error
    Delete(id string) error
    
    // Gestión de miembros
    AddMember(m *clan.Member) error
    RemoveMember(clanID, characterID string) error
    UpdateMemberRank(clanID, characterID string, rank clan.Rank) error
    FindMember(clanID, characterID string) (*clan.Member, error)
    FindMembersByClan(clanID string) ([]*clan.Member, error)
    FindMembersByCharacter(characterID string) ([]*clan.Member, error)
    UpdateMemberContribution(clanID, characterID string, amount int64) error
    
    // Invitaciones
    CreateInvite(inv *clan.Invite) (*clan.Invite, error)
    FindInvite(clanID, characterID string) (*clan.Invite, error)
    FindInvitesByClan(clanID string) ([]*clan.Invite, error)
    FindInvitesByCharacter(characterID string) ([]*clan.Invite, error)
    UpdateInviteStatus(clanID, characterID string, status string) error
    DeleteInvite(clanID, characterID string) error
    
    // Banco del clan
    GetBankBalance(clanID string) (int64, error)
    Deposit(clanID, characterID string, amount int64, note string) error
    Withdraw(clanID, characterID string, amount int64, note string) error
    GetBankLedger(clanID string, limit int) ([]*clan.BankLedgerEntry, error)
    
    // Nivel y XP
    AddXP(clanID string, amount int64) error
    UpdateLevel(clanID string, level int) error
    
    // Utilidad
    IsCharacterInClan(characterID string) (bool, error)
    GetCharacterClan(characterID string) (*clan.Clan, error)
}

// DMChannelRepository provides access to DM channels
type DMChannelRepository interface {
    // Canales
    Drop() error
    FindByID(id string) (*dm.Channel, error)
    FindOrCreateChannel(participantA, participantB string) (*dm.Channel, error)
    FindChannelBetween(participantA, participantB string) (*dm.Channel, error)
    FindChannelsByCharacter(characterID string) ([]*dm.Channel, error)
    UpdateLastMessage(channelID, messageID string, timestamp time.Time) error
    IncrementUnread(channelID, recipient string) error
    MarkRead(channelID, reader string) error
    GetUnreadCount(channelID, reader string) (int, error)
    
    // Mensajes
    StoreMessage(msg *dm.Message) (*dm.Message, error)
    FindMessageByID(id string) (*dm.Message, error)
    FindMessagesByChannel(channelID string, limit int) ([]*dm.Message, error)
    FindMessagesSince(channelID string, since time.Time) ([]*dm.Message, error)
    MarkMessageRead(messageID string) error
    DeleteMessage(messageID string) error
}
```

### 5.2 `pkg/repository/clans_sqlite.go` (nuevo)

```go
package repository

import (
    "fmt"
    
    dbsqlite "github.com/talesmud/talesmud/pkg/db/sqlite"
    "github.com/talesmud/talesmud/pkg/entities/clan"
)

type sqliteClanRepository struct {
    *sqliteGenericRepo
}

// NewSQLiteClanRepository creates a new SQLite clan repository.
func NewSQLiteClanRepository(client *dbsqlite.Client) ClanRepository {
    return &sqliteClanRepository{
        sqliteGenericRepo: newSQLiteGenericRepo(client.DB(), "clans", func() interface{} {
            return &clan.Clan{}
        }),
    }
}

// Implementar todos los métodos de ClanRepository...
// (Ver firmas en interfaces.go)
```

### 5.3 `pkg/repository/dm_sqlite.go` (nuevo)

```go
package repository

import (
    "database/sql"
    "errors"
    "fmt"
    "strings"
    "time"
    
    dbsqlite "github.com/talesmud/talesmud/pkg/db/sqlite"
    "github.com/talesmud/talesmud/pkg/entities/dm"
    "github.com/talesmud/talesmud/pkg/entities"
)

type sqliteDMChannelRepository struct {
    db *sql.DB
}

// NewSQLiteDMChannelRepository creates a new SQLite DM channel repository.
func NewSQLiteDMChannelRepository(client *dbsqlite.Client) DMChannelRepository {
    return &sqliteDMChannelRepository{
        db: client.DB(),
    }
}

// StoreMessage inserta un mensaje usando las columnas reales para indexación
func (repo *sqliteDMChannelRepository) StoreMessage(msg *dm.Message) (*dm.Message, error) {
    if msg.Entity == nil {
        msg.Entity = entities.NewEntity()
    }
    
    payload, err := json.Marshal(msg)
    if err != nil {
        return nil, err
    }
    
    _, err = repo.db.Exec(
        `INSERT INTO dm_messages (id, data, channel_id, sent_at, from_char_id) 
         VALUES (?, ?, ?, ?, ?)`,
        msg.ID,
        string(payload),
        msg.ChannelID,
        msg.SentAt.UnixMilli(),
        msg.FromCharID,
    )
    if err != nil {
        return nil, fmt.Errorf("failed to store DM message: %w", err)
    }
    
    return msg, nil
}

// FindMessagesSince obtiene mensajes desde un timestamp usando el índice real
func (repo *sqliteDMChannelRepository) FindMessagesSince(channelID string, since time.Time) ([]*dm.Message, error) {
    rows, err := repo.db.Query(
        `SELECT data FROM dm_messages 
         WHERE channel_id = ? AND sent_at > ?
         ORDER BY sent_at ASC`,
        channelID,
        since.UnixMilli(),
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var messages []*dm.Message
    for rows.Next() {
        var payload string
        if err := rows.Scan(&payload); err != nil {
            continue
        }
        msg := &dm.Message{}
        if err := json.Unmarshal([]byte(payload), msg); err != nil {
            continue
        }
        messages = append(messages, msg)
    }
    return messages, rows.Err()
}

// ... resto de métodos
```

### 5.4 `pkg/repository/sqlite_factory.go` (añadir)

```go
func (f *SQLiteFactory) Clans() ClanRepository {
    return NewSQLiteClanRepository(f.client)
}

func (f *SQLiteFactory) DMChannels() DMChannelRepository {
    return NewSQLiteDMChannelRepository(f.client)
}
```

---

## 6. Índices Recomendados

### 6.1 Resumen de Índices

| Tabla | Índice | Campos | Propósito |
|-------|--------|--------|-----------|
| `clans` | `idx_clans_name` | `$.name` | Búsqueda por nombre |
| `clans` | `idx_clans_tag` | `$.tag` | Búsqueda por tag |
| `clans` | `idx_clans_leader` | `$.leaderCharID` | Clanes por líder |
| `clans` | `idx_clans_active` | `$.active` | Solo clanes activos |
| `clan_members` | `idx_clan_members_char` | `$.characterID` | Clanes de un personaje |
| `clan_members` | `idx_clan_members_clan` | `$.clanID` | Miembros de un clan |
| `clan_members` | `idx_clan_members_unique` | `$.characterID, $.clanID` | Unicidad de membresía |
| `clan_invites` | `idx_clan_invites_clan` | `$.clanID` | Invitaciones por clan |
| `clan_invites` | `idx_clan_invites_char` | `$.characterID` | Invitaciones por personaje |
| `clan_invites` | `idx_clan_invites_status` | `$.status` | Filtrar por estado |
| `clan_bank_ledger` | `idx_clan_bank_ledger_clan` | `$.clanID` | Historial por clan |
| `clan_bank_ledger` | `idx_clan_bank_ledger_time` | `$.timestamp` | Orden temporal |
| `dm_channels` | `idx_dm_channels_participant` | `$.participantA` | Canales de A |
| `dm_channels` | `idx_dm_channels_participant_b` | `$.participantB` | Canales de B |
| `dm_messages` | `idx_dm_messages_channel_time` | `channel_id, sent_at DESC` | Mensajes por canal+tiempo |
| `dm_messages` | `idx_dm_messages_channel_sender` | `channel_id, from_char_id` | Mensajes por remitente |
| `dm_messages` | `idx_dm_messages_unread` | `channel_id, sent_at` (partial) | No leídos |

### 6.2 Nota sobre Índices Parciales

`idx_dm_messages_unread` es un **índice parcial** (`WHERE json_extract(data, '$.readAt') IS NULL`). SQLite soporta índices parciales desde 3.8.0. Esto optimiza la consulta de mensajes no leídos sin indexar todos los mensajes.

---

## 7. Decisiones de Diseño y Trade-offs

### 7.1 Desviación del Patrón para `dm_messages`

**Decisión**: Añadir columnas reales `channel_id`, `sent_at`, `from_char_id` además de `data`.

**Pros**:
- Índices eficientes para consultas de rango y ordenamiento
- Posibilidad de usar índices parciales para no leídos
- Mejor rendimiento en `ORDER BY sent_at` y `BETWEEN`

**Contras**:
- Redundancia: los mismos datos existen en JSON y en columnas
- Riesgo de inconsistencia si no se mantienen sincronizados
- Desviación del patrón uniforme del repo

**Mitigación**:
- Siempre escribir las columnas junto con el JSON en el mismo INSERT
- Las columnas son "solo lectura" desde la perspectiva del ORM
- Documentar claramente que las columnas son de solo lectura para la lógica de negocio

### 7.2 Unicidad de Membresía

**Decisión**: Índice único en `(characterID, clanID)` en `clan_members`.

**Justificación**: Un personaje solo puede pertenecer a un clan a la vez (diseño de juego común en MUDs).

### 7.3 Canales DM Pre-creados vs. On-Demand

**Decisión**: Crear canales on-demand (primer mensaje entre dos jugadores crea el canal).

**Alternativa rechazada**: Pre-crear canales cuando un jugador se registra (complicado, requiere conocer todos los pares posibles).

### 7.4 Almacenamiento de Timestamp

**Decisión**: Usar `INTEGER` (Unix milisegundos) para `sent_at` en `dm_messages`, y `time.Time` en JSON.

**Justificación**: 
- `INTEGER` permite comparaciones numéricas eficientes
- `UnixMilli()` tiene suficiente resolución
- Mantiene compatibilidad con `time.Time` en JSON (serialización estándar)

### 7.5 Blanqueo de Ledger Bancario

**Decisión**: Append-only (solo INSERT, nunca UPDATE ni DELETE).

**Justificación**: Auditoría completa. Las correcciones se hacen con transacciones de ajuste.

### 7.6 Denormalización de Contadores en Canales

**Decisión**: Mantener `UnreadA`/`UnreadB` y `LastMessageAt` en el canal.

**Alternativa**: Calcular con COUNT/GROUP BY sobre `dm_messages`.

**Justificación**: Más simple y rápido; la complejidad de mantener sincronizados los contadores es menor que el coste de agregación en cada lectura.

### 7.7 Nomenclatura de Tablas

**Decisión**: 
- `clans` para la entidad principal (consistente con `parties`)
- `clan_members` para la relación (no `members` para evitar ambigüedad)
- `dm_channels` / `dm_messages` (prefijo `dm_` para claridad)

### 7.8 Soft Delete para Clanes

**Decisión**: Campo `active: false` en lugar de DELETE físico.

**Justificación**: Preservar datos históricos para auditoría y posibles restauraciones.

---

## 8. Roadmap de Implementación Sugerido

> **Estado real (verificado 2026-10-15): los pasos 1–5 están hechos**
> (migración `migratePhase4SocialTables`, `pkg/entities/clan/` + `pkg/entities/dm/`,
> interfaces `ClanRepository`/`DMRepository` en `interfaces.go`, `clans_sqlite.go`/`dm_sqlite.go`,
> factory `Clans()`/`DMChannels()` + rutas REST + paneles). **El paso 6 está a medias**: dentro de
> `pkg/mudserver/game/` **no hay ninguna integración** — `grep` no encuentra consumidores del DM
> service en ese paquete, **no existe comando `clan …`** y **`tell` no está conectado a la
> persistencia** (el DM service solo lo consumen los handlers REST y, por tanto, los paneles).

1. **Migraciones**: Añadir `migratePhase4SocialTables()` a `runMigrations()`
2. **Entidades**: Crear `pkg/entities/clan/clan.go` y `pkg/entities/dm/dm.go`
3. **Interfaces**: Añadir `ClanRepository` y `DMChannelRepository` a `interfaces.go`
4. **Repositorios**: 
   - `clans_sqlite.go` (usa patrón genérico)
   - `dm_sqlite.go` (implementación custom por desviación del patrón)
5. **Factory**: Registrar en `sqlite_factory.go`
6. **Game Logic**: Integrar en `pkg/mudserver/game/`

---

## 9. Referencias

- `pkg/db/sqlite/sqlite.go` — patrón de migraciones e índices
- `pkg/repository/sqlite_generic.go` — patrón de repositorio genérico
- `pkg/repository/parties_sqlite.go` — ejemplo de repositorio simple
- `pkg/entities/party.go` — ejemplo de entidad base
- `pkg/mudserver/game/commands/tell.go` — implementación actual de tell
- SQLite JSON1: https://www.sqlite.org/json1.html
- SQLite Partial Indexes: https://www.sqlite.org/partialindex.html
