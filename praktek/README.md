# Panduan Praktek

Satu file per modul. Setiap file berisi: tujuan, langkah berurutan, checklist (apa yang harus **kamu lihat sendiri**), cek pemahaman, dan tempat catatan.

## Aturan main

1. Kerjakan berurutan. Jangan lompat ke 6B sebelum 6A paham.
2. Centang `[x]` hanya setelah melihat hasilnya sendiri di terminal.
3. Untuk setiap latihan rusak-dan-perbaiki, **tulis prediksi dulu**, baru lihat hasilnya. Selisih antara prediksi dan kenyataan adalah bahan belajar.
4. Kalau macet, tempel pesan error ke Catatan dan lihat bagian "Pemecahan masalah" di [README utama](../README.md).
5. Selesai satu modul: centang barisnya di tabel di bawah, lalu `git commit` supaya progresmu tercatat.

## Progres

| Status | Modul | File | Estimasi |
| --- | --- | --- | --- |
| [ ] | Persiapan | [00-persiapan.md](00-persiapan.md) | 20 menit |
| [ ] | 4. Aplikasi dan container | [modul-04-container.md](modul-04-container.md) | 1 sampai 2 jam |
| [ ] | 5. Image ke registry (boleh ditunda) | [modul-05-registry.md](modul-05-registry.md) | 30 menit |
| [ ] | 6A. Cluster kind dan deploy | [modul-06a-cluster-dan-deploy.md](modul-06a-cluster-dan-deploy.md) | 1 sampai 2 jam |
| [ ] | 6B. Rusak dan perbaiki | [modul-06b-rusak-dan-perbaiki.md](modul-06b-rusak-dan-perbaiki.md) | 2 sampai 3 jam |
| [ ] | 7. Scaling dan ketahanan | [modul-07-scaling.md](modul-07-scaling.md) | 2 jam |

Estimasi hanya perkiraan kasar; sesuaikan dengan kecepatanmu.

## Cara membaca perintah di panduan (penting untuk pemula)

- **Semua terminal adalah terminal di laptopmu sendiri.** Tidak ada server lain dan tidak ada SSH. Cluster kind pun hanya container Docker di laptopmu.
- Buka terminal dari Terminal.app, iTerm, atau VS Code (menu Terminal, New Terminal). **Setiap terminal baru harus masuk ke root repo dulu:**

  ```bash
  cd /Users/hasbi.shiddieqy/Documents/Personal/Github/kubernetes-for-noob
  pwd
  ```
- Blok kode tanpa judul = satu terminal cukup; jalankan baris demi baris dari atas ke bawah, tunggu tiap perintah selesai.
- Blok berjudul **Terminal 1**, **Terminal 2**, dan seterusnya = jalankan di jendela/tab terminal terpisah sesuai nomornya. Nomor yang sama berarti terminal yang sama sepanjang satu latihan.
- **(biarkan berjalan)** = perintah itu tidak selesai sendiri (server, port-forward, `-w`, `-f`). Jangan tunggu selesai, jangan tutup, pindah ke terminal lain. Hentikan dengan `Ctrl+C` di terminal tersebut.
- `<seperti-ini>` = placeholder. Ganti dengan nilai sebenarnya, tanpa tanda `<` dan `>`.
- `less` (halaman penuh teks): tekan `q` untuk keluar.
- Teks setelah `#` pada baris perintah adalah komentar; tidak perlu diketik.

## Catatan tentang perintah

Panduan ini sengaja tidak memakai `make`, supaya kamu melihat dan memahami perintah aslinya (`docker`, `kind`, `kubectl`, `helm`, `k6`). Semua perintah dijalankan dari root repo. Kalau `kubectl` berikut `rollout undo` atau `rollout restart` dijalankan, tambahkan `kubectl -n tokoku rollout status deployment/api` untuk menunggu sampai selesai.

## Peta repo ke modul

| Yang kamu pelajari | File di repo |
| --- | --- |
| Perilaku aplikasi | [cmd/api/main.go](../cmd/api/main.go) |
| Image | [Dockerfile](../Dockerfile), [.dockerignore](../.dockerignore) |
| Cluster | [k8s/kind/](../k8s/kind/) |
| Deploy | [k8s/base/](../k8s/base/) |
| Scaling dan ketahanan | [k8s/extras/](../k8s/extras/), [loadtest/hpa.js](../loadtest/hpa.js) |
| Ringkasan perintah (hanya referensi) | [Makefile](../Makefile) |

## Jika ingin mengulang dari nol

```bash
kind delete cluster --name lab   # hapus cluster
# lalu ulangi 6A bagian A untuk membuat lagi
```

Lalu hapus centang di file modul yang ingin diulang (cari `[x]` dan ganti jadi `[ ]`).
