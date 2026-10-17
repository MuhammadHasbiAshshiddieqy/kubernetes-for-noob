# Modul 5: Image ke Registry (GHCR)

Tujuan: mengirim image ke registry publik/privat supaya bisa dipakai di luar laptop (CI/CD di Modul 5, GitOps di Modul 8).

Catatan: untuk cluster kind di Modul 6 dan 7, push **belum wajib**, karena image dimuat langsung dengan `kind load`. Kalau kamu belum sampai ke CI/CD, kamu boleh melewati file ini dan kembali nanti. Satu hal yang tetap berguna sekarang: memahami perbedaan image lokal dan image di registry.

Prasyarat: Modul 4 selesai, akun GitHub.

File yang dibaca: [Dockerfile](../Dockerfile) (argumen `VERSION`, `TARGETOS`, `TARGETARCH` dipakai saat build multi-arsitektur).

---

## Langkah

Semua perintah dijalankan di **satu terminal di laptopmu**, dari root repo (`cd /Users/hasbi.shiddieqy/Documents/Personal/Github/kubernetes-for-noob`). Dijalankan berurutan; tunggu tiap perintah selesai. Perintah `export GITHUB_TOKEN=...` hanya berlaku di terminal tempat kamu mengetiknya.

### 1. Buat token

1. GitHub, Settings, Developer settings, Personal access tokens (classic), **Generate new token**.
2. Centang scope `write:packages` saja.
3. Simpan token di password manager. Jangan tulis di file repo dan jangan di-commit.

```bash
export GITHUB_TOKEN=<token kamu>
echo $GITHUB_TOKEN | docker login ghcr.io -u <username-github> --password-stdin
```

### 2. Sesuaikan username

Nama image di GHCR berbentuk `ghcr.io/<username-github>/tokoku-api:<tag>` dan harus **huruf kecil semua**. Di contoh bawah, ganti `<username>` dengan username GitHub-mu (huruf kecil).

### 3. Push multi-arsitektur

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --build-arg VERSION=v0.1.0 \
  -t ghcr.io/<username>/tokoku-api:v0.1.0 \
  --push .
```

Perintah ini membuat image untuk `linux/amd64` dan `linux/arm64` sekaligus lalu langsung mengirimnya (`--push`). Sengaja memakai tag versi, bukan `dev`.

Kalau muncul error soal multi-platform (tidak didukung driver), buat builder dulu:

```bash
docker buildx create --use
```

### 4. Verifikasi

```bash
docker buildx imagetools inspect ghcr.io/<username>/tokoku-api:v0.1.0
```

Cek juga di GitHub, tab Packages pada profilmu.

## Checklist

- [ ] Token dibuat dengan scope `write:packages` saja dan **tidak** tersimpan di file repo (`git status` bersih dari token)
- [ ] `docker login ghcr.io` menampilkan `Login Succeeded`
- [ ] Kamu bisa menjelaskan setiap flag pada `docker buildx build` di atas (`--platform`, `--build-arg`, `-t`, `--push`)
- [ ] Perintah `buildx` selesai tanpa error
- [ ] `imagetools inspect` menampilkan **dua** platform: `linux/amd64` dan `linux/arm64`
- [ ] Package muncul di halaman Packages GitHub
- [ ] Kamu bisa menjelaskan kenapa image dibangun untuk dua arsitektur (laptop Apple Silicon = arm64, server umumnya amd64)
- [ ] Kamu paham kenapa tag `dev` / `latest` buruk untuk production dan kenapa kita memakai `v0.1.0`

## Cek pemahaman

1. Apa bedanya `kind load` dan `docker buildx build --push`?
   <details><summary>Jawaban</summary>`kind load` menyalin image dari Docker laptop langsung ke node kind (tanpa registry). `--push` mengirim ke registry sehingga bisa ditarik oleh cluster mana pun.</details>
2. Kenapa token tidak boleh di-commit?
   <details><summary>Jawaban</summary>Siapa pun yang memegang token bisa menulis ke package-mu. Riwayat Git bersifat permanen dan bocor walaupun file dihapus kemudian.</details>

## Catatan

-

## Selesai?

- [ ] Semua checklist tercentang (atau modul ini sengaja ditunda; tulis alasannya di Catatan)
