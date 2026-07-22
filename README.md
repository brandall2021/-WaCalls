<div align="center">

# 📞 WaCalls

**Llamadas de voz nativas de WhatsApp desde el navegador.**
VoIP nativo, multi-cuenta, multi-sesión, con cliente moderno en React.

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black)](https://react.dev)
[![whatsmeow](https://img.shields.io/badge/whatsmeow-VoIP-25D366?logo=whatsapp&logoColor=white)](https://github.com/tulir/whatsmeow)
[![pion](https://img.shields.io/badge/pion-WebRTC-FF6B6B)](https://github.com/pion/webrtc)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](#licencia)

</div>

---

## Resumen

WaCalls vincula una o más cuentas de WhatsApp mediante **código QR** y permite
**realizar y recibir llamadas de voz 1:1** desde cualquier navegador en la red local.
El micrófono del navegador se envía como **PCM raw de 16 kHz por un canal de datos
WebRTC** al servidor Go, que lo codifica con el códec **MLow** de Meta e inyecta
el medio en el **relé SRTP** de WhatsApp — y el camino inverso devuelve el audio
del interlocutor al navegador.

Todo el stack VoIP corre **de forma nativa en Go puro**: el códec de voz MLow,
**RTP/SRTP**, **STUN**, el transporte **WebRTC/SCTP** y la señalización `<call>`,
integrados con [**whatsmeow**](https://github.com/tulir/whatsmeow) y servidos a
un cliente **React 19**. No hay **cgo ni DLLs nativas** — el códec MLow es un
paquete Go puro vendoreado, así que un `go build` simple produce un binario
autocontenido con audio en vivo.

Se pueden vincular y operar múltiples cuentas de WhatsApp en paralelo, cada una con su
propio QR de vinculación, estado de conexión e historial. Una sola cuenta también puede
mantener **varias llamadas 1:1 simultáneas** — una por cada operador del navegador —
enrutadas independientemente por ID de llamada.

> **Estado:** estable. Llamadas salientes y entrantes 1:1 alcanzan `ACTIVE` con audio
> bidireccional, y una sola cuenta puede mantener varias de ellas concurrentemente.
> Las sesiones persisten en **PostgreSQL**. Autenticación JWT para acceso protegido.
> **Grabación server-side** WAV 16 kHz mono con descarga HTTP.
> **Webhooks** con firma HMAC-SHA256 y reintentos.

---

## Funcionalidades

### 🎙️ Grabación server-side
- Grabación automática al inicio de cada llamada (both inbound y outbound)
- WAV 16 kHz mono PCM, guardado en disco (`/data/recordings`)
- Tap PCM en ambas direcciones: micrófono del navegador + audio del interlocutor
- Tabla `recordings` en PostgreSQL con metadata (id, session_id, call_id, duration, file_path, file_size)
- API: `GET /api/sessions/{sid}/recordings`, `GET /api/recordings/{id}/download`
- Frontend: página Recordings con lista y descarga de archivos WAV

### 🔔 Webhooks
- Configuración de webhooks por sesión (URL, eventos, secret)
- Firma HMAC-SHA256 en cada payload (`X-Webhook-Signature`)
- Headers: `X-Webhook-Event`, `X-Webhook-ID`
- Reintentos con backoff exponencial (hasta 5 intentos)
- Eventos disponibles: `call.incoming`, `call.started`, `call.ended`, `recording.ready`
- API CRUD: `GET/POST/DELETE /api/sessions/{sid}/webhooks`
- Frontend: página Webhooks con crear/listar/eliminar/copy-secret

### 📞 Llamadas VoIP
- Llamadas salientes y entrantes 1:1 con audio bidireccional
- WebRTC data channel para PCM 16 kHz
- Códec MLow nativo en Go puro (sin cgo)
- Buffer de audio durante conexión del relay
- Métricas de calidad en tiempo real (latencia, jitter, packet loss, bitrate)

### 👥 Contactos
- ABM completo de contactos (alta, baja, modificación)
- Campos: nombre, teléfono, email, notas
- Marcar contactos como favoritos con estrella
- Búsqueda por nombre, teléfono o email
- Persistencia en localStorage

### 📅 Agenda de llamadas
- Programar llamadas para fecha y hora específica
- Selección de contacto de la agenda
- Duración estimada en minutos
- Estados: pendiente, completada, cancelada
- Persistencia en localStorage

### 📝 Notas por llamada
- Agregar notas a cualquier llamada
- Rating con estrellas (1-5) y tags personalizados
- Vista historial de todas las notas
- Persistencia en localStorage

### 🌐 Multiidioma
- Soporte completo para **Español (ES)**, **Portugués (PT)** e **Inglés (EN)**

### 🔊 Sonidos de llamada
- Tono de llamada entrante, ocupado y desconexión (Web Audio API, sin archivos externos)

### 📊 Métricas de calidad
- Latencia (RTT), jitter, pérdida de paquetes, bitrate
- Indicador visual de calidad (señal alta/media/baja)

### 🔍 Diagnóstico de audio
- Indicadores visuales: **Mic OK** / **Sin mic** y **Par OK** / **Sin audio par**
- Buffer de PCM mientras el relay o SRTP se conectan (máx. 2 segundos)
- Logs detallados en el servidor con `-debug`

---

## Arquitectura

```
┌──────────────────────────────────────────────────────────────────────────┐
│                    NAVEGADOR (React client)                              │
│   mic + speaker · WebRTC data channel (16 kHz PCM) · HTTP + SSE         │
│   + Contactos · Agenda · Notas (localStorage)                           │
│   + Auth (JWT token en localStorage)                                    │
└───────────────────────────────┬──────────────────────────────────────────┘
                                │  Authorization: Bearer <token>
                                │  POST /api/sessions/{sid}/calls/{id}/webrtc
                                │  GET  /api/events?token=<token>
                                ▼
┌────────────────────────── SERVIDOR GO (cmd/server) ──────────────────────┐
│  AuthStore       registro/login, JWT, bcrypt password hashing           │
│  SessionManager  registro de cuentas (client + CallManager + bridge)    │
│  Broker          hub SSE (sesiones, auth, ciclo de vida de llamadas)    │
│  Bridge          puente pion WebRTC (PCM 16 kHz ⇄ call core)          │
│  RecordingStore  grabaciones WAV en PostgreSQL + disco                  │
│  WebhookStore    configs de webhooks por sesión                         │
│  WebhookDispatcher  envío async con HMAC-SHA256 + reintentos            │
│                                                                           │
│  internal/wa     adaptador VoipSocket sobre whatsmeow                   │
│  internal/voip   call · signaling · media · transport · core · wanode   │
│  internal/recording  WAV writer (16 kHz mono PCM) + Recorder           │
└───────┬──────────────────────────────────────────────────┬──────────────┘
        │ señalización <call> (Signal/USync)               │ SRTP media
        ▼                                                   ▼
┌───────────────┐                                ┌──────────────────────┐
│  WhatsApp WS  │                                │   Relé WhatsApp      │
│  (whatsmeow)  │                                │  (SRTP over SCTP/DC) │
└───────────────┘                                └──────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────┐
│  PostgreSQL                                           │
│  users · sessions · recordings · webhook_configs      │
│  webhook_deliveries · whatsmeow_*                     │
└───────────────────────────────────────────────────────┘
```

### Estructura de archivos

| Ruta | Responsabilidad |
|---|---|
| `cmd/server` | Broker HTTP/SSE, gestor de sesiones, puente WebRTC, auth JWT, grabación, webhooks |
| `internal/wa` | `VoipSocket` — envía/recibe stanzas `<call>` vía whatsmeow |
| `internal/voip/core` | Tipos de dominio, constantes, interfaz `VoipSocket` |
| `internal/voip/wanode` | Helpers compartidos de nodo WhatsApp y JID |
| `internal/voip/media` | Códec MLow (Go puro), RTP, SRTP, SSRC, PCM, derivación de claves |
| `internal/voip/transport` | Relé SCTP, STUN, codificación de suscripciones |
| `internal/voip/signaling` | Build/parse de stanzas `<call>`, crypto de claves de llamada |
| `internal/voip/call` | `CallManager` — orquesta una llamada de principio a fin |
| `internal/recording` | WAV writer (16 kHz mono PCM) + `Recorder` struct |
| `client/` | React 19 + Vite + Tailwind v4 + shadcn/ui |

### Stores del cliente (Zustand + localStorage)

| Store | Datos |
|---|---|
| `stores/auth.ts` | Autenticación JWT (login, registro, token) |
| `stores/contacts.ts` | Contactos con CRUD, favoritos, búsqueda |
| `stores/schedule.ts` | Llamadas programadas con estados |
| `stores/callNotes.ts` | Notas por llamada con rating y tags |
| `stores/calls.ts` | Estado de llamadas activas (del servidor) |
| `stores/sessions.ts` | Sesiones de WhatsApp (del servidor) |
| `stores/devices.ts` | Dispositivos de audio del navegador |
| `stores/theme.ts` | Tema claro/oscuro |

---

## Flujo de una llamada

El núcleo es `internal/voip/call.CallManager`, que maneja una llamada de
principio a fin. Secuencia de llamada saliente:

```
1. POST .../calls            → CallManager.StartCall(peerJid)
                               genera un callID, construye la oferta <call>, la envía

2. Navegador abre WebRTC     → POST .../calls/{id}/webrtc (oferta SDP)
                               el puente responde con una respuesta SDP (pion)

3. Par acepta               → events.CallAccept → HandleCallAccept
                               servidor recibe <relay> + claves hop-by-hop

4. Transporte relay          → binding/allocate STUN en relés de WhatsApp
                               ICE + DTLS + SCTP DataChannel conectan (pion)

5. SRTP fluyendo             → el estado pasa a ACTIVE
   ├── subida   (vos → par): PCM 16 kHz del navegador (data channel) → MLow encode → SRTP → relay
   └── bajada   (par → vos): relay → SRTP → MLow decode → PCM 16 kHz (data channel) → navegador

6. Grabación server-side     → WAV 16 kHz mono → disco + tabla recordings
                               webhook "recording.ready" al finalizar

7. Webhooks                  → call.started / call.ended / recording.ready
                               HMAC-SHA256 + reintentos exponenciales

8. Teardown                  → DELETE .../calls/{id} o events.CallTerminate
                               CallManager.EndCall + limpieza del puente
```

---

## Requisitos

- **Go 1.26+**
- **Node 22+** y **npm** (solo para compilar/ejecutar el cliente React)
- **PostgreSQL** (almacena usuarios, sesiones, grabaciones, webhooks)

No se necesita compilador C, cgo ni bibliotecas nativas — el códec MLow es Go
puro vendoreado (`internal/voip/media/mlow`).

---

## Inicio rápido

### Con Docker Compose (recomendado)

```bash
git clone <url-del-repo> wacalls
cd wacalls

# Levantar PostgreSQL + WaCalls
docker compose up -d
```

Esto levanta PostgreSQL y WaCalls conectado automáticamente. Abrí `http://localhost:8080`.

### Instalación manual

```bash
# clonar y entrar al proyecto
git clone <url-del-repo> wacalls
cd wacalls

# dependencias de Go
go mod download

# dependencias del cliente React
cd client && npm install && cd ..

# configurar PostgreSQL
export DATABASE_URL="postgresql://user:pass@localhost:5432/wacall"
export JWT_SECRET="tu-secreto-seguro"
```

### Ejecutar

```bash
go run ./cmd/server -database-url "$DATABASE_URL" -addr :8080
# agregar -debug para logs verbosos
```

El audio en vivo funciona directamente — el códec MLow es Go puro, así que una
compilación simple lo incluye. Sin build tags, sin `CGO_ENABLED`, sin DLLs.

### Primer uso

1. Abrí `http://localhost:8080`
2. Iniciá sesión con una de las cuentas de ejemplo (se crean automáticamente al primer arranque):

| Email | Password | Nombre |
|---|---|---|
| `admin@wacalls.com` | `admin123` | Administrador |
| `operador@wacalls.com` | `operador123` | Operador |
| `demo@wacalls.com` | `demo123` | Demo |

3. Creá una sesión de WhatsApp y escaneá el QR
4. ¡Listo para llamadas!

> **Nota:** Las cuentas de ejemplo solo se crean si la tabla `users` está vacía.
> Podés registrar usuarios adicionales desde la aplicación.

### Cliente React en modo desarrollo

```bash
cd client
npm run dev      # Vite en :5173, proxea /api → http://localhost:8080
```

Para producción, compilá el cliente estático y servilo desde el servidor Go:

```bash
cd client && npm run build && cd ..
go run ./cmd/server -database-url "$DATABASE_URL" -static client/dist -addr :8080
```

### Flags del servidor

| Flag | Valor por defecto | Descripción |
|---|---|---|
| `-addr` | `:8080` | Dirección de escucha HTTP |
| `-database-url` | *(requerido)* | URL de conexión PostgreSQL (o env `DATABASE_URL`) |
| `-static` | `client/dist` | Directorio del cliente estático (opcional) |
| `-debug` | `false` | Logging verboso (incluye el log interno de whatsmeow) |
| `-max-calls-per-session` | `8` | Máximo de llamadas concurrentes por sesión (`0` = sin límite) |

### Variables de entorno

| Variable | Descripción |
|---|---|
| `DATABASE_URL` | URL de conexión PostgreSQL (alternativa al flag `-database-url`) |
| `JWT_SECRET` | Secreto para firmar tokens JWT (default: `wacalls-default-secret-change-me`) |

---

## API

### Autenticación (rutas públicas)

| Método | Ruta | Propósito |
|---|---|---|
| `POST` | `/api/auth/register` | Crear cuenta (`{ email, password, name }`) |
| `POST` | `/api/auth/login` | Iniciar sesión (`{ email, password }`) |
| `GET` | `/api/auth/me` | Obtener usuario actual (requiere `Authorization: Bearer <token>`) |

### Sesiones y llamadas (requieren auth)

Todas las rutas requieren header `Authorization: Bearer <token>` o query param `?token=<token>`.

| Método | Ruta | Propósito |
|---|---|---|
| `GET` | `/api/sessions` | Listar cuentas (id, nombre, jid, estado, vinculada) |
| `POST` | `/api/sessions` | Crear una cuenta e iniciar vinculación por QR |
| `DELETE` | `/api/sessions/{sid}` | Cerrar sesión y eliminar una cuenta |
| `POST` | `/api/sessions/{sid}/logout` | Desconectar una cuenta (mantener para re-vinculación) |
| `POST` | `/api/sessions/{sid}/pair` | Re-vincular una cuenta (emitir QR nuevo) |
| `POST` | `/api/sessions/{sid}/calls` | Iniciar llamada saliente (`{ phone, duration_ms?, record? }`) |
| `POST` | `/api/sessions/{sid}/calls/{id}/webrtc` | Intercambiar SDP WebRTC del navegador |
| `POST` | `/api/sessions/{sid}/calls/{id}/accept` | Aceptar llamada entrante |
| `POST` | `/api/sessions/{sid}/calls/{id}/reject` | Rechazar llamada entrante |
| `DELETE` | `/api/sessions/{sid}/calls/{id}` | Finalizar llamada activa |
| `GET` | `/api/sessions/{sid}/history` | Historial de llamadas recientes (hasta 50 registros) |
| `GET` | `/api/events` | Eventos server-sent (sesiones, auth, ciclo de llamadas) |

### Grabaciones

| Método | Ruta | Propósito |
|---|---|---|
| `GET` | `/api/sessions/{sid}/recordings` | Listar grabaciones de la sesión |
| `GET` | `/api/recordings/{id}/download` | Descargar archivo WAV |

### Webhooks

| Método | Ruta | Propósito |
|---|---|---|
| `GET` | `/api/sessions/{sid}/webhooks` | Listar webhooks configurados |
| `POST` | `/api/sessions/{sid}/webhooks` | Crear webhook (`{ url, events }`) |
| `DELETE` | `/api/sessions/{sid}/webhooks/{wid}` | Eliminar webhook |

**Eventos de webhook:**

| Evento | Descripción | Data |
|---|---|---|
| `call.incoming` | Llamada entrante recibida | `call_id, peer, direction` |
| `call.started` | Llamada activa (conectada) | `call_id, peer, direction` |
| `call.ended` | Llamada finalizada | `call_id, reason` |
| `recording.ready` | Grabación guardada en disco | `recording_id, call_id, duration_ms, file_size` |

**Headers en cada webhook:**

| Header | Descripción |
|---|---|
| `X-Webhook-Signature` | Firma HMAC-SHA256 del payload |
| `X-Webhook-Event` | Tipo de evento |
| `X-Webhook-ID` | ID único de la entrega |

---

## Navegación del cliente

El cliente tiene 6 secciones accesibles desde la barra lateral:

| Sección | Ícono | Descripción |
|---|---|---|
| **Calls** | 📞 | Marcador, llamadas activas, calidad, notas |
| **Contacts** | 👥 | ABM de contactos con favoritos y búsqueda |
| **Recordings** | 🎙️ | Lista de grabaciones WAV con descarga |
| **Webhooks** | 🔔 | Configurar URLs de webhook y gestionar secrets |
| **Schedule** | 📅 | Agenda de llamadas programadas |
| **Notes** | 📝 | Historial de notas con rating y tags |

---

## Tests

```bash
go test ./...                 # stack de media: SRTP, STUN, RTP, relay-ack, códec, estado
cd client && npm run build    # type-check del cliente + build de producción
```

---

## Seguridad

La API usa **autenticación JWT** — todas las rutas `/api/*` (excepto login y registro)
requieren un token válido en el header `Authorization: Bearer <token>` o como query
parameter `?token=<token>`.

Los webhooks usan **firma HMAC-SHA256** — cada entrega incluye un header
`X-Webhook-Signature` con la firma del payload, verificable con el secret
asociado al webhook.

### Configuración

```bash
# Variables de entorno (requeridas para producción)
export DATABASE_URL="postgresql://user:pass@host:5432/wacall"
export JWT_SECRET="tu-secreto-seguro-aqui"
```

**IMPORTANTE:** Cambiá el `JWT_SECRET` por defecto en producción. Los tokens duran 72 horas.

### Base de datos

- **PostgreSQL** almacena usuarios, sesiones de WhatsApp, grabaciones y webhooks
- Las credenciales de sesión de WhatsApp (secretos) se almacenan en las tablas `whatsmeow_*`
- Las grabaciones WAV se almacenan en `/data/recordings` (volumen Docker)

---

## Solución de problemas de audio

Si el otro teléfono no escucha audio, revisá los logs del servidor con `-debug`:

| Mensaje de log | Significado | Solución |
|---|---|---|
| `codec nil` | El códec MLow no se inicializó | Verificar que `internal/voip/media/mlow/` esté compilado |
| `srtpSession nil, buffering` | Las claves SRTP no se derivaron | Revisar que la llamada tenga `EncryptionKey` y `ParticipantJids` |
| `relay not connected, buffering` | El relay de WhatsApp no conectó | Verificar conectividad de red (ICE/DTLS a puertos 3478) |
| `srtp protect error` | Error al encriptar el paquete | Las claves SRTP no coinciden con el peer |
| `audio frame sent` | Audio fluyendo correctamente | Todo funciona, revisar del lado del peer |
| `flushing buffered audio` | Buffer liberado tras conectar relay | Funcionando, el delay inicial es normal |

En el navegador, los indicadores muestran:
- **Mic OK** (verde) = el micrófono está capturando audio
- **Sin mic** (amarillo) = el navegador no tiene permiso de micrófono o no hay dispositivo
- **Par OK** (verde) = se está recibiendo audio del interlocutor
- **Sin audio par** (amarillo) = no se recibe audio del relay

---

## Contribuidores

Este proyecto se construye sobre el trabajo de:

<div align="center">

<a href="https://github.com/jotadev66"><img src="https://github.com/jotadev66.png" width="72" height="72" style="border-radius:50%" alt="jotadev66"/></a>
<a href="https://github.com/jobasfernandes"><img src="https://github.com/jobasfernandes.png" width="72" height="72" style="border-radius:50%" alt="jobasfernandes"/></a>
<a href="https://github.com/edgardmessias"><img src="https://github.com/edgardmessias.png" width="72" height="72" style="border-radius:50%" alt="edgardmessias"/></a>
<a href="https://github.com/w3nder"><img src="https://github.com/w3nder.png" width="72" height="72" style="border-radius:50%" alt="w3nder"/></a>

[**@jotadev66**](https://github.com/jotadev66) · [**@jobasfernandes**](https://github.com/jobasfernandes) · [**@edgardmessias**](https://github.com/edgardmessias) · [**@w3nder**](https://github.com/w3nder)

</div>

---

## Agradecimientos

- [**whatsmeow**](https://github.com/tulir/whatsmeow) — Librería Go para el protocolo WhatsApp Web
- [**pion/webrtc**](https://github.com/pion/webrtc) — Stack WebRTC en Go puro (ICE + DTLS + SCTP)
- [**whatsapp-rust**](https://github.com/oxidezap/whatsapp-rust) — Implementación de referencia del códec MLow (portado al `internal/voip/media/mlow` Go puro)
- [**zapo**](https://github.com/w3nder/zapo) — Referencia del stack de media VoIP

---

## Licencia

[MIT](./LICENSE)
