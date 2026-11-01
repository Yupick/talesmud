# Guía de arranque del fork `mi-mud`

Guía práctica para levantar el fork en local y desplegarlo con Docker.
La arquitectura detrás de todo esto está en [`docs/architecture.md`](architecture.md).

> Basado en el repositorio real (`Makefile`, `docker-compose.yml`,
> `.env.example`, `Dockerfile`, `config/gamemode.yaml`, `cmd/tales/main.go`,
> `pkg/server/server.go`).

---

## 1. Prerrequisitos

| Requisito | Versión mínima | Comprobación | Para qué |
|---|---|---|---|
| **Go** | **1.26** (`go.mod`: `go 1.26.0`, toolchain `go1.26.9`) | `go version` | Backend, `go test ./...` |
| **Node.js** | **22** (README: 22.18+) | `node -v` | Creator UI y game client |
| **npm** | incluido con Node | `npm -v` | Instalación/build de frontends |
| **Make** | cualquiera | `make -v` | Targets de desarrollo |
| **Docker + Docker Compose** | cualquiera | `docker compose version` | Despliegue contenedorizado |
| **Ollama** | opcional | `ollama --version` | LLM local (features de IA). Alternativa sin instalar nada: sidecar del compose (`docker compose --profile ollama up`, ver §5.1). |

Sin Make puedes ejecutar los comandos subyacentes a mano (ver §3 y §4).
Sin Go/Node puedes usar las imágenes de Docker indicadas en §9 (Troubleshooting).

---

## 2. Configuración inicial

```bash
cd /home/mkd/programacion/mmorpgweb/mi-mud
cp .env.example .env      # y editar valores
```

`.env` lo carga `godotenv` en `cmd/tales/main.go` **antes** de arrancar el
servidor (si no existe, el proceso sigue arrancando pero muestra un *warning*).
En el fork, aunque falte `.env`, `config/gamemode.yaml` fija igualmente `port`
y `sqlite_path`.

Mínimo para desarrollo local:

```dotenv
GIN_MODE=debug
PORT=8010
SQLITE_PATH=./talesmud.db
TRUSTED_PROXIES=127.0.0.1,::1
```

> Nota: si `config/gamemode.yaml` está presente, sus `port:`/`sqlite_path:`
> **pisan** este bloque (la base acaba en `data/talesmud.db`); `PORT=8010`
> solo importa si no hay fichero de gamemode. Ver la precedencia de §2.1.

### 2.1 Auth local por defecto

**El fork arranca con auth local** (`AUTH_MODE=local`): cuentas de
usuario/contraseña propias (hash Argon2id, JWT con issuer `talesmud-local`)
sin depender de ningún tenant de Auth0. Ni `.env` ni `docker compose` exigen
variables `AUTH0_*` para levantarse.

El perfil por defecto vive en **`config/gamemode.yaml`**:

```yaml
presentation: classic
auth: local
port: "8010"
sqlite_path: data/talesmud.db
timezone: UTC
title: "Mi MUD"
subtitle: "Dungeon crawler"
```

`cmd/tales/main.go` lo aplica **automáticamente** cuando no pasas `-config` y
el fichero existe (la imagen Docker lo copia a `/app/config/gamemode.yaml`, así
que el contenedor también arranca en local-auth). Si no hay fichero ni flag,
el default del código sigue siendo el de upstream: `auth: external` (Auth0).

Endpoints que habilita `auth: local`:

| Endpoint | Auth | Notas |
|---|---|---|
| `POST /api/auth/register` | pública | crea la cuenta (usuario + contraseña) |
| `POST /api/auth/login` | pública | devuelve el JWT local |
| `GET /api/auth/me` | Bearer | perfil de la cuenta |
| `POST /api/guest` | pública | sesión de invitado (HMAC), funciona en **cualquier** modo |
| `POST /api/ws-ticket` | Bearer | ticket de un solo uso para `/ws` |

Los tokens de reset de contraseña se guardan en el outbox local
(`data/auth-outbox.log`, configurable con `AUTH_OUTBOX_PATH`); los endpoints
`POST /api/auth/forgot` · `POST /api/auth/reset` solo se registran si el
mailer entrega el correo fuera de la máquina (SMTP).

**Precedencia de la configuración:**

* El **fichero** manda sobre el entorno para `PORT` y `SQLITE_PATH`
  (`gamemode.ApplyFile` los escribe en el proceso; la ruta relativa se
  resuelve contra el directorio de trabajo, p. ej. `data/talesmud.db`).
* Las variables de entorno **`AUTH_MODE`**, `PRESENTATION`, `WORLD_PACK`,
  `GAME_TIMEZONE` y `SESSION_SECRET` **sobrescriben** el fichero
  (`gamemode.ApplyEnv()` se ejecuta después).

#### Cambiar a Auth0 (opcional)

Solo si quieres mantener el login del upstream contra un tenant propio:

```dotenv
# .env
AUTH_MODE=auth0
# backend (obligatorias con AUTH_MODE=auth0)
AUTH0_AUDIENCE=http://localhost:8010/api
AUTH0_DOMAIN=https://tu-tenant.auth0.com/
AUTH0_WK_JWKS=https://tu-tenant.auth0.com/.well-known/jwks.json
# build-time del SPA (si recompilas los frontends)
VITE_AUTH0_DOMAIN=tu-tenant.auth0.com
VITE_AUTH0_CLIENT_ID=<client id>
VITE_AUTH0_AUDIENCE=http://localhost:8010/api
# opcional: sub de Auth0 que recibe rol Admin en cada login
MUD_ADMIN_OAUTHID=<sub de Auth0>
```

Con `AUTH_MODE=local` las `AUTH0_*` de backend se **ignoran** (en compose
llegan vacías gracias a `${AUTH0_*:-}`), así que no hace falta borrarlas del
`.env`: basta con dejar `AUTH_MODE=local`. Las `VITE_AUTH0_*` solo cuentan al
**construir** el frontend: con `VITE_AUTH0_CLIENT_ID` vacío el cliente no
inicializa Auth0 y se queda en el modo local/invitado.

> Ver también §5 (Docker), §6 (tablas de variables) y
> [`docs/architecture.md`](architecture.md) §3/§6.

---

## 3. Modo desarrollo

### 3.1 Arranque completo: `make dev`

```bash
make dev
```

`dev` es un alias de `${MAKE} -j4 run-server run-frontend run-mud-client` y
lanza **3 procesos en paralelo**:

| # | Target | Qué hace | URL |
|---|---|---|---|
| 1 | `run-server` | `go run ./cmd/tales` (prepara assets embebidos si faltan) — Gin + WebSocket + motor de juego | **http://localhost:8010** |
| 2 | `run-frontend` | `cd public/app && npm run dev` — Vite con hot reload; hace **proxy** de `/api`, `/ws` y `/admin` al backend | **http://localhost:5173** (Creator UI) |
| 3 | `run-mud-client` | `cd public/mud-client && npm install && npm run dev` — Rollup en watch + servidor estático (`sirv`) | **http://localhost:8080** (game client, según el comentario del `Makefile`) |

El **game client en `/play`** se puede abrir también contra el backend directo
(`http://localhost:8010/play`) una vez hecho `make build`; en desarrollo se usa
el servidor de Vite/sirv con hot reload.

> ⚠️ **CORS**: la lista *built-in* de `pkg/server/server.go` incluye
> `http://localhost:5173`, `http://localhost:8010` y `http://localhost:5000`,
> pero **no** `http://localhost:8080`. Si el game client te sirve en 8080 y el
> navegador bloquea las peticiones, añade en `.env`:
> ```dotenv
> CORS_ALLOWED_ORIGINS=http://localhost:8080
> ```
> (la variable admite varias URLs separadas por comas).

### 3.2 Procesos individuales

```bash
make run-server        # backend Go en :8010
make run-frontend      # Creator UI (Vite) en :5173
make run-mud-client    # game client (Rollup) en :8080
make run               # igual que make dev
```

Utilidades extra:

```bash
make run-dialogs-sandbox   # sandbox de dialogs (cmd/dialog_sandbox)
```

### 3.3 Importar un mundo (pack)

El motor viene **sin contenido**; el mundo se importa desde `import/<pack>/`:

```bash
./bin/tales -import <pack>            # importa y guarda respaldo
./bin/tales -import <pack> --dry-run  # solo valida
./bin/tales -check <pack>             # content health sin arrancar servidor
./bin/tales -check <pack> -json -fail-on error
```

Con `make run-server` puedes pasar flags igualmente:
`go run ./cmd/tales -import <pack>`.

### 3.4 Probar los sistemas sociales (dos clientes / dos personajes)

Clanes, DMs y party necesitan **dos personajes**; canales y party se ven mejor
con dos sesiones. Receta rápida:

1. Arranca el stack completo (`make dev`, §3.1).
2. Abre el game client en **dos navegadores** (uno normal y otro incógnito, o
   dos perfiles distintos) contra `http://localhost:8080` (o
   `http://localhost:8010/play`).
3. Registra una cuenta por ventana (`http://localhost:5173` → Creator UI, o
   `POST /api/auth/register`) y crea un personaje en cada una.
4. Para invitar o escribir DMs necesitas el **character id** del otro; sácalo
   con `curl` (o desde la consola del navegador con el token de sesión):

   ```bash
   TOKEN=$(curl -s -X POST http://localhost:8010/api/auth/login \
     -H 'Content-Type: application/json' \
     -d '{"username":"ana","password":"secreto"}' | jq -r '.token')

   # mis personajes: [{ "id": "…", "name": "Aster", … }, …]
   curl -s http://localhost:8010/api/my-characters \
     -H "Authorization: Bearer $TOKEN" | jq -r '.[] | "\(.id) \(.name)"'
   ```

Comprobaciones por sistema (detalle completo en [`user/`](user/)):

| Sistema | Qué probar |
|---|---|
| **Clanes** ([`user/clans.md`](user/clans.md)) | tecla `C` en la ventana A → *Create*; en B: invitar por `characterId` → `C` → **Accept**; deposit/withdraw, promote/demote, kick, disband |
| **DMs** ([`user/dms.md`](user/dms.md)) | tecla `M` en A → *New message — character id* (el de B) → mensaje → en B el badge amarillo; comprobar que el historial sobrevive al recargar |
| **Canales** ([`user/channels.md`](user/channels.md)) | en A: `chat general hola`; en B debe aparecer `[General] Aster: hola` en la pestaña **Ch** (sin importar sala); probar `trade`, `lfg`, `roleplay`, `help` y el filtro por chip |
| **Party** ([`user/party.md`](user/party.md)) | en A: `party create` → `party invite <nombre-en-B>`; en B: `party accept`; `party hola`; `party loot leader` + matar un NPC; `party readycheck` y responder `party ready` en 30 s |
| **Curación** ([`user/combat.md`](user/combat.md)) | Clérigo/Druida: `skills`, `skills equip Mend`, atacar un NPC y usar `cast Mend` (o el hueco 1–4 de la hotbar); comprobar `Not enough mana…` y `… is on cooldown…` |

> Nota: las invitaciones de clan y las DMs **no llegan por WebSocket**; si no
> ves nada, pulsa **Refresh** en el panel (las DMs además sondean cada 5 s).

---

## 4. Build embebido (modo producción local)

```bash
make build
```

`make build` ejecuta tres pasos y deja el binario listo:

| Paso | Comando | Salida |
|---|---|---|
| 1 | `cd public/app && npm run build` → copia a `pkg/webui/dist/` | **Creator UI** embebida, servida en `/` |
| 2 | `cd public/mud-client && npm install && npm run build` → copia a `pkg/webuiplay/dist/` | **Game client** embebido, servida en `/play` |
| 3 | `go build -buildvcs=false -o bin/tales ./cmd/tales` | binario único **`bin/tales`** |

> **Compilación por paquete:** todos los targets Go del `Makefile` compilan el
> paquete entero (`./cmd/tales`, `./cmd/dialog_sandbox`), nunca un fichero
> suelto: desde el Batch 20 el paquete incluye `cmd/tales/pprof.go` y
> `go build cmd/tales/main.go` falla con `undefined: startPprof` (Batch 22
> corrigió los targets, que llevaban rotos desde entonces). `-buildvcs=false`
> omite el sello de VCS y es **necesario** donde no haya `.git` disponible
> (Docker con bind-mount y propiedad dubious, contenedores, checkouts de root);
> en un clone normal puedes quitarlo si quieres la marca de versión.


```bash
./bin/tales                 # sirve /, /play, /api, /ws en :8010
./bin/tales -config <fichero.yaml>   # presentation/auth/puerto/ruta desde YAML
```

> **Config por defecto**: si **no** pasas `-config` y existe
> `config/gamemode.yaml`, el binario lo aplica solo (auth `local`,
> `presentation: classic`, `port: "8010"`, `sqlite_path: data/talesmud.db` —
> ver §2.1). Ese mismo fichero se empaqueta en la imagen Docker
> (`/app/config/gamemode.yaml`), por eso `docker compose up` arranca igual
> sin flags.
>
> ⚠️ Como el fichero **pisa** `PORT` y `SQLITE_PATH`, un
> `PORT=8030 ./bin/tales` queda anulado si `config/gamemode.yaml` está
> presente: para cambiar de puerto usa `-config <otro.yaml>` o edita el
> `port:` del fichero (`AUTH_MODE`/`PRESENTATION`/`SESSION_SECRET` sí se
> pueden sobrescribir por entorno).

Targets alternativos:

```bash
make build-frontend    # solo Creator UI → pkg/webui/dist
make build-mud-client  # solo game client → pkg/webuiplay/dist
make build-backend     # solo Go (usa prepare-embedded-assets)
```

`prepare-embedded-assets` (invocado por `build-backend` y `run-server`) crea
`pkg/webui/dist/index.html` y `pkg/webuiplay/dist/` **si no existen**, para que
el `go build` no falle por el `go:embed`; esos assets son un *placeholder* —
ejecuta `make build` para incrustar los frontends reales.

---

## 5. Docker

```bash
cp .env.example .env       # sin variables obligatorias: AUTH_MODE=local ya sirve
docker compose up -d --build
# estado / logs
docker compose ps
docker compose logs -f talesmud
# parar
docker compose down
```

**No hace falta ninguna variable de Auth0 para arrancar.** El arranque por
defecto es **auth local** (`AUTH_MODE=${AUTH_MODE:-local}`) y todas las
`AUTH0_*`/`VITE_AUTH0_*` llegan vacías (`${AUTH0_*:-}`); solo si cambias a
`AUTH_MODE=auth0` hay que rellenarlas (ver §2.1).

Datos del `docker-compose.yml` actual:

* **Variables obligatorias**: **ninguna**. Todas usan `${VAR:-}` /
  `${VAR:-default}`; el servicio arranca con `GIN_MODE=release` y el perfil de
  `config/gamemode.yaml` que trae la imagen (auth `local`).
* **Modo de auth**: `AUTH_MODE=${AUTH_MODE:-local}`; `AUTH0_AUDIENCE`,
  `AUTH0_DOMAIN` y `AUTH0_WK_JWKS` se propagan **solo como opcionales** (se
  ignoran salvo `AUTH_MODE=auth0`). Las build args `VITE_AUTH0_DOMAIN`,
  `VITE_AUTH0_CLIENT_ID`, `VITE_AUTH0_AUDIENCE` siguen siendo opcionales.
* **Puerto publicado**: `8010:8010` (único puerto del servicio `talesmud`).
* **Modo**: `GIN_MODE=release`, base en `/app/data/talesmud.db`; la imagen
  trae `config/gamemode.yaml` en `/app/config/gamemode.yaml` (se aplica solo).
* **Variables LLM** (se propagan con defaults):
  * `LLM_PROVIDER` → `ollama` (por defecto; `groq` o `none`/`off` alternativos)
  * `OLLAMA_BASE_URL` → `http://ollama:11434` (**dentro de la red compose**;
    en local fuera de compose usa `http://localhost:11434`)
  * `OLLAMA_MODEL` → `llama3.1`, `LLM_TIMEOUT_SECONDS` → `60`
  * `GROQ_API_KEY` → vacío (hace falta solo con `LLM_PROVIDER=groq`)
* **Variables opcionales restantes**: `ADMIN_USER`, `ADMIN_PASSWORD`,
  `GUEST_SECRET`, `SESSION_SECRET`, `TRUSTED_PROXIES` (por defecto
  `127.0.0.1`), `CORS_ALLOWED_ORIGINS`, `MUD_ADMIN_OAUTHID`.
* **Salud**: `healthcheck` del servicio `talesmud` → `wget` contra
  **`GET http://localhost:8010/health`** (intervalo 30 s, timeout 5 s,
  3 reintentos, `start_period` 20 s). `docker compose ps` marca
  `healthy` cuando el backend responde `API is up and running`.
* **Volúmenes**: `talesmud-data` (SQLite), `talesmud-uploads`
  (fondos/retratos), `talesmud-backups`, `ollama-models` (modelos del
  sidecar).

### 5.1 Ollama como servicio opcional (profile)

El servicio `ollama` (`ollama/ollama:latest`) está tras el profile **`ollama`**:
**no arranca por defecto** y `talesmud` no depende de él.

```bash
# backend + sidecar de LLM en la misma red
docker compose --profile ollama up -d --build

# dentro de la red compose el backend habla con http://ollama:11434
# (es el default de OLLAMA_BASE_URL en el compose); el host lo publica
# en http://localhost:11434 para gestionar modelos:
docker compose exec ollama ollama pull llama3.1
curl -s http://localhost:11434/api/tags
```

* El sidecar usa el volumen `ollama-models` (`/root/.ollama`) y publica
  `11434` en el host.
* Desarrollo **fuera** de compose (Ollama instalado a mano): pon en `.env`
  `OLLAMA_BASE_URL=http://localhost:11434` (es el default de
  `.env.example`).
* Para usar Groq en compose: `LLM_PROVIDER=groq` + `GROQ_API_KEY=...` (y no
  hace falta el profile `ollama`).

La imagen se construye en 4 etapas (`Dockerfile`): `node:22-alpine` para el
Creator UI, `node:22-alpine` para el game client, `golang:1.26-alpine` para el
binario (`CGO_ENABLED=0`) y `alpine:3.21` como runtime no-root (`tales`),
`EXPOSE 8010`, con `config/gamemode.yaml` copiado a `/app/config/`.

> El `README.md` upstream usa `docker-compose up -d` (con guion); el plugin
> moderno es `docker compose up -d`.

---

## 6. Variables de entorno clave

### Servidor y red

| Variable | Por defecto | Descripción |
|---|---|---|
| `PORT` | `8010` (`.env.example` y `config/gamemode.yaml`) | Puerto HTTP/WS. **Si queda vacía el servidor no arranca** (`0.0.0.0:` sin puerto). Con el `gamemode.yaml` por defecto presente, el fichero **pisa** la variable (ver §2.1). |
| `GIN_MODE` | `debug` (`debug`/`release`) | Modo Gin; el compose fija `release`. |
| `LOG_LEVEL` / `LOG_FILE` | `info` (código) / `logs/talesmud.log` (`.env.example` pone `debug`) | Nivel y fichero de log con rotación (retención 7 días). |
| `SQLITE_PATH` | `./talesmud.db` (`.env.example`); `data/talesmud.db` (`config/gamemode.yaml`) | Ruta del fichero SQLite (Docker: `/app/data/talesmud.db`). Prioridad: fichero de gamemode > entorno (§2.1). |
| `CORS_ALLOWED_ORIGINS` | vacío | Orígenes extra (separados por comas) además de los built-in: `talesmud.io`, `localhost:5000`, `localhost:5173`, `localhost:8010`. |
| `TRUSTED_PROXIES` | `127.0.0.1,::1` | Proxies de confianza para `X-Forwarded-For` (rate limit de invitados). **No usar `0.0.0.0/0`.** |
| `BACKGROUNDS_PATH` | `./uploads/backgrounds` | Imágenes de sala. |
| `LANDING_PATH` | vacío | Directorio con `index.html` para servir una landing en `/`. |

### Autenticación

| Variable | Por defecto | Descripción |
|---|---|---|
| `GUEST_SECRET` | aleatoria en cada arranque | Clave HMAC-SHA256 de los tokens de invitado. **Fijarla** para que las sesiones sobrevivan a un reinicio. |
| `SESSION_SECRET` | `data/session.key` (se crea con modo 0600) | Firma HS256 de las sesiones de cuenta (device login y `auth: local`). |
| `ADMIN_USER` / `ADMIN_PASSWORD` | vacíos (desactivados) | Basic auth de `/admin/export` y `/admin/import`. Mínimo 12 caracteres; `admin`/`password`/`changeme` se rechazan. |
| `AUTH_MODE` | **`local`** (fork: `.env.example`, `config/gamemode.yaml` y compose `${AUTH_MODE:-local}`) | `local` para login usuario/contraseña propio (sin Auth0); `auth0` para el proveedor externo. *Sin fichero de gamemode ni variable, el default del código es `auth0` (upstream).* |
| `AUTH0_AUDIENCE` / `AUTH0_DOMAIN` / `AUTH0_WK_JWKS` | vacías | Solo con `AUTH_MODE=auth0`: verificación RS256 (aud+iss obligatorios) y caché JWKS. Con `AUTH_MODE=local` se ignoran. |
| `VITE_AUTH0_DOMAIN` / `VITE_AUTH0_CLIENT_ID` / `VITE_AUTH0_AUDIENCE` | `VITE_AUTH0_DOMAIN=your-tenant.auth0.com` (placeholder), el resto vacío | **Build-time** del frontend (Docker build args / `netlify.toml`). Con `VITE_AUTH0_CLIENT_ID` vacío el cliente mantiene los placeholders y **no** inicializa Auth0 (`isAuth0Configured()` en `public/mud-client/src/authConfig.js`); solo rellénalas con `AUTH_MODE=auth0`. |
| `MUD_ADMIN_OAUTHID` | vacío | Solo `auth0`: `sub` de Auth0 que recibe el rol Admin en cada login. |
| `SSH_ACTIVATE_URL` | `http://localhost:8010/activate` | URL pública de confirmación de device login. |

> **Consejo:** para volver a Auth0 basta con `AUTH_MODE=auth0` + las tres
> `AUTH0_*` (y, si recompilas frontends, las `VITE_AUTH0_*`); no hay que tocar
> el código. El detalle está en §2.1.

### LLM (nueva sección del fork)

| Variable | Por defecto | Descripción |
|---|---|---|
| `LLM_PROVIDER` | `ollama` (`.env.example` y compose); **sin definir** → Groq si hay `GROQ_API_KEY`, si no Ollama | `ollama` · `groq` · `none`/`off` (o vacío = off). Proveedor desconocido = generación desactivada. El cliente se elige con `llm.NewFromEnv()` y se enchufa en `POST /api/generate/character`. |
| `OLLAMA_BASE_URL` | `http://localhost:11434` (`.env.example`) · `http://ollama:11434` (compose, sidecar) | Daemon de Ollama (API compatible con OpenAI: `/v1/chat/completions`). Fuera de compose usa `localhost`; dentro de la red compose, `ollama`. |
| `OLLAMA_MODEL` | `llama3.1` | Modelo a usar. |
| `LLM_TIMEOUT_SECONDS` | `60` | Timeout HTTP del cliente Ollama (la inferencia local es más lenta que Groq, 15 s). |
| `GROQ_API_KEY` | vacío | Clave de Groq (`llama-3.3-70b-versatile`). Sin ella, `LLM_PROVIDER=groq` devuelve *no configurado*. |

### Otras

| Variable | Descripción |
|---|---|
| `TALESMUD_ALLOW_JS_SCRIPTS=1` | Habilita scripts JavaScript (Otto); **Lua es el estándar**. |
| `CLASSKIT_DIR` | Directorio con las clases YAML del pack (si no, se prueban `import/mvp-rpg-1/data/classes`, `../talesmud-rpg-1/data/classes`, `data/classes`). |
| `PRESENTATION` / `WORLD_PACK` / `GAME_TIMEZONE` | Flags de modo (`classic` o `door_tui`), pack de mundo y zona horaria. Sobrescriben `config/gamemode.yaml`. |
| `AUTH_OUTBOX_PATH` | Outbox local de resets (`data/auth-outbox.log` por defecto). |

Fichero **`config/gamemode.yaml`** (no es una variable, pero configura el
arranque): `presentation`, `auth`, `port`, `sqlite_path`, `timezone`, `title`,
`subtitle`. Se aplica solo si no pasas `-config`; la copia del contenedor está
en `/app/config/gamemode.yaml`.

---

## 7. Ollama (LLM local) y proveedor LLM

```bash
# 1. Instalación (Linux/macOS): https://ollama.com/download
curl -fsSL https://ollama.com/install.sh | sh     # o: brew install ollama

# 2. Descargar un modelo (una sola vez)
ollama pull llama3.1

# 3. Arrancar el daemon (si no corre como servicio)
ollama serve
# → escucha en http://localhost:11434

# 4. Verificación rápida
ollama list
ollama run llama3.1 "Di hola en una palabra"
curl -s http://localhost:11434/api/tags
```

Configuración en `.env` (Ollama instalado a mano, fuera de compose):

```dotenv
LLM_PROVIDER=ollama
OLLAMA_BASE_URL=http://localhost:11434
OLLAMA_MODEL=llama3.1
LLM_TIMEOUT_SECONDS=60
```

**Alternativa en Docker:** no hace falta instalar Ollama en el host; el
compose trae un sidecar tras el profile `ollama` (ver §5.1):

```bash
docker compose --profile ollama up -d --build
# el backend ya apunta a http://ollama:11434 (default del compose);
# el host lo publica en http://localhost:11434
docker compose exec ollama ollama pull llama3.1
```

Proveedores (contrato único `llm.Client`):

| `LLM_PROVIDER` | Cliente | Requisitos |
|---|---|---|
| `ollama` (default del fork) | `pkg/service/ollama` → `OLLAMA_BASE_URL` | daemon arriba + modelo descargado; sin API key |
| `groq` | `pkg/service/groq` (15 s, `llama-3.3-70b-versatile`) | `GROQ_API_KEY` |
| `none` / `off` / vacío / desconocido | sin cliente | la generación responde `503 AI generation is not configured` |

Ejemplo de llamada al endpoint del motor (con tu token):

```bash
curl -sS -X POST http://localhost:8010/api/generate/character \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"generate":"both","archetype":"mage","race":"elf","class":"Mage"}'
```

> **Cableado (delta del fork, hecho):** `pkg/server/server.go` construye el
> handler con `llm.NewFromEnv()`, así que el proveedor se elige por
> `LLM_PROVIDER` (fallback: Groq si hay `GROQ_API_KEY`, si no Ollama) — ya
> **no** hace falta `GROQ_API_KEY` para que funcione con solo Ollama. Si la
> generación devuelve `503 AI generation is not configured`, es que el
> proveedor quedó desactivado (revisa §6 y el troubleshooting de §9).

---

## 8. Tests y verificación

```bash
# Backend (Go 1.26)
go build ./... && go vet ./... && go test ./...
# o por paquete:
go test ./pkg/mudserver/... ./pkg/server/...

# Creator UI (public/app)
cd public/app && npm ci --include=dev --legacy-peer-deps && npm test

# Game client (public/mud-client)
cd public/mud-client && npm ci --include=dev --legacy-peer-deps
node --test 'src/**/*_test.mjs'
```

Notas:

* Las instalaciones de frontend usan `--legacy-peer-deps` por los rangos de
  peer-dependencias de los plugins del bundler (política upstream).
* El test de imagen del game client necesita además:
  `npm install --no-save --package-lock=false --legacy-peer-deps canvas`
  (requiere librerías de sistema de Cairo).
* Sanidad del contenido: `./bin/tales -check <pack>` (con `-json` y
  `-fail-on error|warning`).
* Los tests usan bases de datos temporales y listeners locales: no tocan tu
  `talesmud.db`.

---

## 9. Troubleshooting

| Síntoma | Causa probable | Solución |
|---|---|---|
| `go: command not found` / versión < 1.26 | Go no instalado | Usar la imagen oficial (ver abajo) o instalar desde https://go.dev/dl/ |
| `node: command not found` | Node no instalado | Usar `node:22-alpine` o instalar Node 22 |
| `listen tcp: address 0.0.0.0: missing port` | `PORT` vacío | Asegurar `PORT=8010` en `.env` (o `port:` en `config/gamemode.yaml`) |
| `404` en `POST /api/auth/register` o `POST /api/auth/login` | El proceso no está en `auth: local` (`AUTH_MODE=auth0` o sin `config/gamemode.yaml`) | `AUTH_MODE=local` en `.env` y/o dejar `config/gamemode.yaml` (ver §2.1) |
| `set AUTH0_AUDIENCE` (o similar) al hacer `docker compose up` | Estás usando un `docker-compose.yml` viejo (upstream) que sí exigía Auth0 | Usar el `docker-compose.yml` de este fork: Auth0 es opcional (`${AUTH0_*:-}`) y arranca con `AUTH_MODE=local` |
| El contenedor queda `unhealthy` en `docker compose ps` | El backend no responde a `GET /health` (arranque lento o crash) | `docker compose logs talesmud`; el healthcheck espera 20 s y reintenta 3 veces (ver §5) |
| `Blocked by CORS policy` desde el game client | Puerto no permitido | `CORS_ALLOWED_ORIGINS=http://localhost:8080` (o el puerto real) |
| `401` al abrir `/ws` | Ticket caducado (60 s) o sin token | Reintentar: el cliente debe pedir de nuevo `POST /api/ws-ticket`; comprobar que no se pone `?access_token=` en la URL |
| `503 AI generation is not configured` | Proveedor desactivado: `LLM_PROVIDER` vacío/`none`/desconocido, o `groq` sin `GROQ_API_KEY` | Ver §7: `LLM_PROVIDER=ollama` (no requiere key) o definir `GROQ_API_KEY` |
| `500 Failed to generate name` o timeout con `LLM_PROVIDER=ollama` | Daemon parado, modelo no descargado o `OLLAMA_BASE_URL` mal | `ollama serve`, `ollama pull llama3.1`, revisar `OLLAMA_BASE_URL` (`localhost` fuera de compose, `ollama` dentro) y `LLM_TIMEOUT_SECONDS` |
| `database is locked` | Escrituras concurrentes | Dejar de editar el `.db` externamente; el motor usa WAL + 1 conexión |
| `npm ERR! peer dependency` | Plugins del bundler | Instalar con `npm ci --include=dev --legacy-peer-deps` |
| Puerto 8010 ocupado (o `PORT=8011` no hace efecto) | Proceso previo / el `gamemode.yaml` por defecto **pisa** `PORT` | Matar el proceso previo; para otro puerto edita `port:` en `config/gamemode.yaml` o pasa `-config <otro.yaml>` (no basta la variable de entorno) |
| La UI muestra "Run make build-frontend to embed the Creator UI" | Assets embebidos de *placeholder* | `make build` |
| El juego no tiene mundo / salas | No se importó contenido | `./bin/tales -import <pack>` (ver `import/README.md`) |

### Ejecutar las pruebas sin Go/Node instalados

```bash
# Backend completo dentro de golang:1.26 (-buildvcs=false: dentro del
# contenedor el .git montado puede no ser utilizable por propiedad)
docker run --rm -v "$PWD":/src -w /src \
  -v gomod:/go/pkg/mod -v gobuild:/root/.cache/go-build \
  golang:1.26 sh -c "go build -buildvcs=false ./... && go vet -buildvcs=false ./... && go test -buildvcs=false ./..."

# Frontends con Node 22
docker run --rm -v "$PWD":/src -w /src/public/app \
  node:22-alpine sh -c "npm ci --include=dev --legacy-peer-deps && npm test"
```

---

## 10. Checklist rápido de arranque

```bash
cp .env.example .env          # AUTH_MODE=local ya viene; PORT=8010
make dev                      # backend 8010 · Creator 5173 · game client 8080
# en otra terminal, si usas IA local:
ollama pull llama3.1 && ollama serve
# ...o, si prefieres Docker (auth local por defecto, sin variables Auth0):
docker compose up -d --build
docker compose --profile ollama up -d   # opcional: sidecar de LLM
```

1. `http://localhost:5173` → Creator UI (edición de contenido).
2. `http://localhost:8010/play` o `http://localhost:8080` → game client.
3. `http://localhost:8010/health` → `API is up and running` (también es el
   healthcheck del compose: `docker compose ps` → `healthy`).
