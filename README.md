# CambiumDB

**A B+ tree database storage engine written from scratch in Go.**

[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

CambiumDB stores and retrieves key-value pairs on disk using a B+ tree, with no external database
dependencies. It implements the layers a real storage engine needs: a slotted page format, a buffer
pool with LRU eviction, Direct I/O that bypasses the kernel page cache, latch-based concurrency
control, and write-ahead logging for crash recovery. Clients talk to it over TCP using a small
binary protocol.

<p align="center">
  <img src="assets/Architecture/CambiumDB%20Architecture%20v2.png" alt="CambiumDB architecture" width="900"/>
</p>

## Features

- **B+ tree indexing** — supports key-based lookup and sequential iteration over all items. Multiple
  independent B+ trees can live in one database file.
- **Slotted page format** — B+ tree nodes are laid out as slotted pages, supporting variable-size
  records and in-page compaction to reclaim fragmented space.
- **Buffer pool manager** — caches 4 KB pages in a fixed set of frames, with an LRU replacer
  choosing which unpinned page to evict, and a dirty-flag check to avoid unnecessary writes.
- **Direct I/O** — reads and writes pages straight between disk and user-space memory, bypassing the
  kernel page cache to avoid double-buffering and give the engine full control over caching.
- **Concurrency control** — page-level read/write latches via guard objects allow multiple concurrent
  readers or a single exclusive writer per B+ tree.
- **Page checksums** — every page header carries a CRC32 that is recomputed on write and verified on
  read, so silent corruption is detected rather than propagated into the tree.
- **Crash recovery** — page-level modifications are appended to a
  [write-ahead log](https://github.com/Adarsh-Kmt/Lucario) before being persisted, and replayed on
  restart after an unclean shutdown.
- **TCP server** — a binary request/response protocol over TCP for `INSERT`, `GET`, `DELETE`, `PING`,
  `CLOSE`, and `SHUTDOWN`.

## Architecture

See [architecture.md](architecture.md) for a component-by-component breakdown of the database file
layout, disk manager, buffer pool manager, replacer, guards, codecs, and node readers/writers.

<p align="center">
  <img src="assets/Architecture/Buffer%20Pool%20Manager%20v2.png" alt="Buffer pool manager" width="900"/>
</p>

## Getting started

### Prerequisites

- Go 1.24 or higher
- A POSIX-compliant operating system (Linux or macOS) — Direct I/O uses platform-specific flags

### Build and run

```bash
git clone https://github.com/Sourav-Nath-01/CambiumDB.git
cd CambiumDB
go mod tidy
go build ./...
go run .
```

The server listens on `:8080`, stores its pages in `cambium.db`, and writes its log to
`lucario.wal`. Both files are created in the working directory on first run.

### Run the tests

```bash
go test ./...
```

The suite covers B+ tree insert/get/delete and iteration, buffer pool eviction, WAL recovery after a
simulated crash, and request decoding.

## Wire protocol

Each request begins with a single-byte op code. Requests that carry a body follow it with a
little-endian `uint32` body length, then the body itself. Length-prefixed fields inside the body use
the same encoding.

| Op code | Operation | Body |
|---|---|---|
| `I` | Insert | `uint32` key length, key, `uint32` value length, value |
| `G` | Get | `uint32` key length, key |
| `D` | Delete | `uint32` key length, key |
| `P` | Ping | — |
| `C` | Close connection | — |
| `S` | Shut down server | — |

Responses start with `O` on success or `E` on error. A successful `GET` returns the key and value as
length-prefixed fields; an error response returns the message as a length-prefixed string.

## Technical challenges solved

### 1. Duplicate page fetching

**Problem**: When multiple threads simultaneously try to read the same page from disk, duplicate
copies of the page end up in memory.

<p align="center">
  <img src="assets/Race%20Conditions/Fetch%20Page/1.%20Race%20Condition.png" alt="Race condition when fetching a page" width="900"/>
</p>

**Solution**: A double-checked locking pattern:

- Acquire the read lock and check whether the page is already in memory.
- If not, release the read lock and acquire the write lock.
- Re-check the condition under the write lock, to make sure another thread didn't load the page in
  the window between releasing the read lock and acquiring the write lock.
- Only one copy of the page ever exists in memory.

<p align="center">
  <img src="assets/Race%20Conditions/Fetch%20Page/3.%20Read-Write%20Lock%20Solution.png" alt="Read-write lock solution" width="900"/>
</p>

### 2. Resource leaks in error paths

**Problem**: A page allocation followed by an I/O error left the allocated page stranded — reachable
by nothing, but never returned to the free list.

**Solution**: Every allocation path cleans up on failure, deallocating the page and releasing its
frame before returning the error.

### 3. Direct I/O performance optimization

**Problem**: Buffered I/O suffered from double-buffering — every page existed once in the kernel page
cache and once in the buffer pool — and from unpredictable kernel cache eviction behaviour.

<p align="center">
  <img src="assets/Buffered%20IO/Buffered%20IO.png" alt="Buffered I/O" width="900"/>
</p>

**Solution**: A custom Direct I/O implementation transfers data from disk directly into user-space
memory, bypassing the kernel page cache. This required allocating page-aligned buffers by
over-allocating virtual memory and aligning the pointer into it, since Direct I/O requires the
destination buffer to be aligned to the block size.

<p align="center">
  <img src="assets/Direct%20IO/Direct%20IO.png" alt="Direct I/O" width="900"/>
</p>

### 4. Variable-size records in fixed-size pages

**Problem**: A naive sorted-list page layout requires shifting every subsequent record on each insert
or delete, and cannot handle variable-size keys and values without expensive compaction on every
write.

**Solution**: A slotted page format. A slot array at the front of the page holds offsets into a
record area that grows from the back, so inserts and deletes only move slot entries rather than
record bytes. Space from deleted records is reclaimed by compacting the page when it fills up.

<p align="center">
  <img src="assets/Slotted%20Page%20Format/5.%20B-Tree%20Node%20Slotted%20Page%20Format.png" alt="Slotted page format" width="900"/>
</p>

## Project layout

| Package | Responsibility |
|---|---|
| [bplustree/](bplustree/) | B+ tree insert, get, delete, iteration, and node split/merge |
| [bufferpoolmanager/](bufferpoolmanager/) | Frame management, LRU replacer, Direct I/O disk manager |
| [pagecodec/](pagecodec/) | Encoding and decoding slotted pages as internal and leaf nodes |
| [storageengine/](storageengine/) | Ties the layers together; WAL recovery |
| [server/](server/) | TCP server, request decoding, response encoding |

## License

Released under the [MIT License](LICENSE).
