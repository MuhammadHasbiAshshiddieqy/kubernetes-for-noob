# Persiapan (kerjakan sekali, sebelum Modul 4)

Tujuan: memastikan laptop siap, supaya kalau nanti ada yang gagal kamu tahu itu masalah modul, bukan masalah tools.

Cara pakai file praktek: kerjakan berurutan, centang `[x]` setiap item setelah **kamu lihat sendiri hasilnya**. Jangan centang kalau hanya "kayaknya benar". Kolom "Catatan" di bagian bawah tiap file untuk menulis apa yang kamu temukan atau yang membingungkan.

## Langkah

Semua perintah dijalankan di terminal laptopmu sendiri (Terminal.app, iTerm, atau terminal VS Code). Konvensi lengkap (Terminal 1/2, "biarkan berjalan", `cd` ke root repo) ada di [README praktek](README.md).

### 1. Pasang tools

```bash
brew install kind kubectl          # kubectl mungkin sudah ada dari Docker Desktop
brew install k9s kubectx stern     # opsional
brew install helm k6               # dibutuhkan Modul 7
```

### 2. Cek versi

```bash
docker version          # harus menampilkan Client DAN Server (Docker Desktop harus jalan)
go version
kind version
kubectl version --client
helm version            # untuk Modul 7
k6 version              # untuk Modul 7
```

### 3. Matikan Kubernetes bawaan Docker Desktop

Docker Desktop, Settings, Kubernetes: pastikan **tidak aktif** (supaya tidak berebut memori dengan kind).

```bash
kubectl config get-contexts     # sekarang boleh ada docker-desktop, tapi jangan dipakai
```

### 4. Kenali repo

```bash
ls cmd/api k8s/base k8s/extras k8s/kind
cat Makefile            # hanya dibaca untuk referensi; panduan praktek tidak memakai make
```

## Checklist

- [ ] `docker version` menampilkan bagian **Server** (bukan error "Cannot connect to the Docker daemon")
- [ ] `go version` berhasil (minimal 1.22, karena kode memakai pola routing `GET /path` yang baru)
- [ ] `kind version` berhasil
- [ ] `kubectl version --client` berhasil (dan kamu hanya punya satu `kubectl`: `which -a kubectl`)
- [ ] `helm version` berhasil (boleh ditunda sampai Modul 7)
- [ ] `k6 version` berhasil (boleh ditunda sampai Modul 7; saat ini belum terpasang di laptopmu)
- [ ] Kubernetes bawaan Docker Desktop mati
- [ ] Kamu sudah membuka [Makefile](../Makefile) dan melihat bahwa setiap target hanyalah kumpulan perintah biasa (di panduan ini semuanya ditulis langsung)

## Cek pemahaman

1. Kenapa Docker Desktop harus berjalan walaupun kita memakai `kind`?
   <details><summary>Jawaban</summary>kind menjalankan setiap "node" Kubernetes sebagai container Docker. Tanpa Docker, tidak ada node.</details>
2. Apa bedanya `kind` dan `kubectl`?
   <details><summary>Jawaban</summary>kind membuat dan menghapus cluster (alat untuk lab). kubectl berbicara ke cluster mana pun yang sedang aktif di context-nya (alat untuk mengoperasikan isi cluster).</details>

## Catatan

-
