# Modul 4: Aplikasi dan Container

Tujuan: memahami aplikasi yang akan kamu pakai sepanjang kursus, lalu membungkusnya jadi image container. Belum ada Kubernetes di modul ini.

Yang akan kamu pahami di akhir:
- apa itu liveness dan readiness (di level aplikasi, sebelum Kubernetes ikut campur)
- kenapa aplikasi harus berhenti rapi saat menerima SIGTERM
- kenapa image dibuat multi-stage dan berjalan sebagai non-root

File yang dibaca: [cmd/api/main.go](../cmd/api/main.go), [Dockerfile](../Dockerfile), [.dockerignore](../.dockerignore).

## Sebelum mulai (baca sekali)

- **Semua perintah di panduan ini dijalankan di terminal laptopmu sendiri** (Terminal.app, iTerm, atau terminal di VS Code: menu Terminal, New Terminal). Tidak ada server lain dan tidak ada SSH.
- Setiap terminal baru harus berada di **folder root repo**. Di terminal baru, jalankan dulu:

  ```bash
  cd /Users/hasbi.shiddieqy/Documents/Personal/Github/kubernetes-for-noob
  pwd        # harus menampilkan path di atas
  ```
- Kalau satu langkah butuh dua terminal, blok perintahnya diberi judul **Terminal 1** dan **Terminal 2**. Buka dua jendela/tab terminal dan pakai sesuai judul. Blok tanpa judul = dijalankan di terminal mana pun (satu terminal cukup).
- Tanda **(biarkan berjalan)** berarti perintah itu tidak selesai sendiri (server terus hidup). Jangan tutup, jangan tunggu keluarannya selesai; pindah ke terminal lain. Untuk menghentikannya tekan `Ctrl+C` di terminal itu.

---

## A. Aplikasi berjalan di laptop

### Langkah

Satu terminal, uji dulu kodenya:

```bash
go test ./... -race -count=1
```

**Terminal 1** (biarkan berjalan): menjalankan aplikasi langsung di laptopmu.

```bash
PORT=8080 APP_ENV=local go run ./cmd/api
```

Keluarannya berupa satu baris log JSON `server dimulai`. Setelah itu terminal tampak "menggantung"; itu normal karena server sedang melayani.

**Terminal 2**: memanggil aplikasi tadi (`localhost:8080` = laptopmu sendiri).

```bash
curl -s localhost:8080/                    # JSON: message, version, env, hostname, time
curl -s localhost:8080/healthz             # ok
curl -s localhost:8080/readyz              # ready
curl -s localhost:8080/metrics | head -30
```

Masih di **Terminal 2**, rusak-rusakkan status probe:

```bash
curl -i -X POST "localhost:8080/debug/ready?state=false"
curl -i localhost:8080/readyz              # sekarang 503
curl -i localhost:8080/healthz             # masih 200
curl -X POST "localhost:8080/debug/ready?state=true"

curl -X POST "localhost:8080/debug/health?state=false"
curl -i localhost:8080/healthz             # 503
curl -X POST "localhost:8080/debug/health?state=true"
```

Lihat **Terminal 1**: setiap request ke `/` dan `/debug/*` tercatat sebagai log JSON, tetapi `/healthz`, `/readyz`, dan `/metrics` tidak (disengaja, supaya probe tidak membanjiri log).

Selesai bagian A: di **Terminal 1** tekan `Ctrl+C` untuk menghentikan aplikasi.

### Checklist A

- [ ] `go test ./... -race -count=1` lulus semua
- [ ] `/` menampilkan `"version":"dev"`, `"env":"local"`, dan `hostname` berisi nama laptopmu
- [ ] `/healthz` membalas `ok`, `/readyz` membalas `ready`
- [ ] Setelah `ready?state=false`: `/readyz` jadi **503** sedangkan `/healthz` tetap **200**
- [ ] Setelah `health?state=false`: `/healthz` jadi **503**
- [ ] `/metrics` memuat `http_requests_total{...path="/"...}` setelah kamu memanggil `/` beberapa kali
- [ ] Kamu paham kenapa `/healthz` tidak muncul di `http_requests_total`
- [ ] `curl -s "localhost:8080/debug/slow?ms=2000"` benar-benar tertahan 2 detik

---

## B. Perilaku start dan berhenti

Kita memakai binary hasil build (bukan `go run`) supaya kode keluar proses terlihat. Pastikan aplikasi dari bagian A sudah dihentikan (port 8080 harus bebas).

### Langkah

**Terminal 1**: build, lalu jalankan dengan jeda start 10 detik (biarkan berjalan).

```bash
go build -o bin/api ./cmd/api
STARTUP_DELAY=10s ./bin/api
```

**Terminal 2**: lakukan **segera** setelah Terminal 1 menampilkan log `server dimulai`.

```bash
curl -i localhost:8080/readyz           # 503 selama 10 detik pertama
sleep 10; curl -i localhost:8080/readyz # 200
```

Berhenti rapi: di **Terminal 1** tekan `Ctrl+C` dan baca lognya.

Crash disengaja:

**Terminal 1** (biarkan berjalan sampai prosesnya mati sendiri):

```bash
./bin/api
```

**Terminal 2**:

```bash
curl -X POST localhost:8080/debug/crash
```

Kembali ke **Terminal 1**: proses sudah berhenti dan prompt muncul lagi. Lalu ketik:

```bash
echo $?        # kode keluar proses terakhir
```

### Checklist B

- [ ] Dengan `STARTUP_DELAY=10s`, `/readyz` 503 sekitar 10 detik lalu berubah 200, sementara `/` sudah bisa diakses sejak awal
- [ ] Ctrl+C menghasilkan log `sinyal berhenti diterima, mulai graceful shutdown` lalu `server berhenti dengan rapi`
- [ ] Setelah `/debug/crash`, proses mati dan `echo $?` menampilkan **1**
- [ ] Kamu bisa menjelaskan: kenapa `/readyz` dijadikan gagal lebih dulu saat shutdown, sebelum server berhenti? (lihat komentar di `run()` pada main.go)

---

## C. Build image

Semua langkah di bawah memakai Docker di laptopmu. Docker Desktop harus sedang berjalan.

### Langkah

Satu terminal:

```bash
docker build --build-arg VERSION=dev -t tokoku-api:dev .   # titik di akhir = folder saat ini sebagai konteks build
docker images tokoku-api
docker history tokoku-api:dev
docker inspect -f '{{.Config.User}}' tokoku-api:dev
```

Jalankan image sebagai container:

**Terminal 1** (biarkan berjalan):

```bash
docker run --rm -p 8080:8080 --name tokoku-api tokoku-api:dev
```

**Terminal 2**:

```bash
curl -s localhost:8080/                 # perhatikan hostname = ID container, bukan nama laptop
docker exec -it tokoku-api sh           # akan GAGAL, dan itu disengaja
```

Uji berhenti rapi di container. Masih di **Terminal 2**:

```bash
docker stop tokoku-api
```

Lihat **Terminal 1**: harus muncul log `sinyal berhenti diterima` dan `server berhenti dengan rapi`, lalu perintah `docker run` selesai sendiri.

Ubah konfigurasi lewat environment variable (tanpa build ulang):

**Terminal 1** (biarkan berjalan; port 8080 harus sudah bebas dari langkah sebelumnya):

```bash
docker run --rm -p 8080:8080 -e GREETING="Halo Docker" -e APP_ENV=docker tokoku-api:dev
```

**Terminal 2**:

```bash
curl -s localhost:8080/                 # message dan env berubah
```

Hentikan dengan `Ctrl+C` di **Terminal 1**.

Uji build-arg versi:

**Terminal 1** (satu terminal cukup untuk build, lalu biarkan `run` berjalan):

```bash
docker build --build-arg VERSION=v0.0.1 -t tokoku-api:v0.0.1 .
docker run --rm -p 8080:8080 tokoku-api:v0.0.1
```

**Terminal 2**: `curl -s localhost:8080/` (cek `version`). Hentikan dengan `Ctrl+C` di Terminal 1.

Uji cache layer: ubah satu komentar di [cmd/api/main.go](../cmd/api/main.go) lalu, di satu terminal:

```bash
docker build -t tokoku-api:dev .        # perhatikan langkah mana yang CACHED
```

Kembalikan komentar yang kamu ubah (cek dengan `git status`).

### Checklist C

- [ ] Image `tokoku-api:dev` ada dan ukurannya jauh lebih kecil dari image `golang` (cek: `docker images golang`; kalau belum ada, bandingkan dengan perkiraan ratusan MB)
- [ ] `docker inspect` menunjukkan user `65532:65532` (bukan root)
- [ ] `docker exec ... sh` gagal karena distroless tidak punya shell; kamu paham itu kelebihan (permukaan serangan kecil) sekaligus konsekuensinya (debugging lebih sulit)
- [ ] `-e GREETING=...` mengubah `message` di `/` tanpa build ulang
- [ ] `--build-arg VERSION=v0.0.1` membuat `/` menampilkan `"version":"v0.0.1"`
- [ ] Setelah mengubah `main.go`, langkah `go mod download` pada build berstatus **CACHED** dan hanya langkah compile yang jalan ulang
- [ ] `docker stop` membuat aplikasi berhenti dengan log graceful shutdown (bukan dimatikan paksa setelah 10 detik)
- [ ] Kamu bisa menjelaskan fungsi dua tahap di [Dockerfile](../Dockerfile): tahap `build` (compiler, besar) dan tahap akhir (hanya binary)
- [ ] Kamu paham kenapa `.dockerignore` mengecualikan `k8s`, `*.md`, dan `.git`

## Cek pemahaman

1. Apa beda liveness dan readiness? Apa akibatnya masing-masing kalau gagal di Kubernetes?
   <details><summary>Jawaban</summary>Liveness: "proses ini macet?" Gagal berarti container di-restart. Readiness: "siap menerima traffic?" Gagal berarti pod dikeluarkan dari Service tetapi tidak di-restart. (Dibuktikan di Modul 6B, latihan 7 dan 8.)</details>
2. Kenapa konfigurasi dibaca dari environment variable, bukan ditulis di kode?
   <details><summary>Jawaban</summary>Image yang sama bisa dipakai di lingkungan berbeda (lokal, staging, production) hanya dengan mengganti konfigurasi. Di Kubernetes nilainya datang dari ConfigMap.</details>
3. Apa yang terjadi kalau aplikasi mengabaikan SIGTERM?
   <details><summary>Jawaban</summary>Kubernetes menunggu `terminationGracePeriodSeconds` lalu mematikan paksa (SIGKILL), sehingga request yang sedang berjalan bisa terputus.</details>

## Catatan

-

## Selesai?

- [ ] Semua checklist A, B, C tercentang
- [ ] Kamu bisa menjelaskan ulang isi `main.go` (config, probe, debug, shutdown) tanpa membaca contekan
