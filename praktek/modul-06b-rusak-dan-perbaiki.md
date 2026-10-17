# Modul 6B: Rusak dan Perbaiki (11 latihan)

Tujuan: merusak aplikasi dengan sengaja, membaca gejalanya, lalu memperbaikinya. Setelah modul ini kamu harus bisa menjawab "pod saya kenapa?" dengan membaca `get`, `describe`, `logs`, dan `events`, bukan menebak.

Prasyarat: Modul 6A selesai dan cluster `kind-lab` berjalan dengan 2 pod `1/1`.

## Setup tiga terminal (sekali di awal)

**Semua terminal adalah terminal di laptopmu sendiri** (bukan server lain). Cluster kind berjalan sebagai container Docker di laptopmu, dan `kubectl` di setiap terminal berbicara ke cluster itu.

Buka **tiga** jendela/tab terminal. Di masing-masing, masuk ke root repo dulu:

```bash
cd /Users/hasbi.shiddieqy/Documents/Personal/Github/kubernetes-for-noob
kubectl config current-context        # harus: kind-lab
```

Lalu:

| Terminal | Jalankan | Sifat | Fungsi |
| --- | --- | --- | --- |
| **Terminal 1** | `kubectl -n tokoku port-forward svc/api 8081:80` | (biarkan berjalan) | Terowongan `localhost:8081` ke **satu** pod |
| **Terminal 2** | `kubectl -n tokoku get pods -w` | (biarkan berjalan) | Memantau pod secara live; baris baru muncul saat ada perubahan |
| **Terminal 3** | tempat menjalankan semua latihan | interaktif | `curl`, `kubectl`, dan perintah lain |

**Aturan di seluruh file ini:** perintah di blok kode **dijalankan di Terminal 3**, kecuali blok itu berjudul terminal lain. Terminal 1 dan 2 jangan dipakai mengetik perintah baru; hanya dilihat.

Perintah yang berisi `<...>` (misalnya `<nama-pod>`) harus kamu ganti dengan nilai sebenarnya, **tanpa tanda `<` dan `>`**. Nama pod diambil dari kolom NAME di Terminal 2 (contoh: `api-6d9f7c8b5-x2k4q`).

Ingat: perubahan keadaan lewat `localhost:8081` hanya terjadi di pod yang kena port-forward. Jika Terminal 1 berhenti atau tampil error `lost connection to pod`, tekan `Ctrl+C` lalu jalankan ulang perintahnya di Terminal 1.

## Pola kerja untuk setiap latihan

Biasakan urutan ini; inilah keterampilan inti modul ini:

1. **Cek sehat dulu** (Terminal 3): `kubectl -n tokoku get deploy,rs,pods,svc,hpa -o wide` (semua `1/1`).
2. **Rusak**: jalankan perintahnya.
3. **Amati**: `get pods`, lalu `describe pod <nama>`, `logs <nama>`, `kubectl -n tokoku get events --sort-by=.lastTimestamp`. **Tulis dulu prediksimu sebelum melihat.**
4. **Pahami**: kenapa begitu?
5. **Perbaiki** dan **pastikan pulih**: `kubectl -n tokoku get deploy,rs,pods,svc,hpa -o wide` kembali `1/1`.

Sebelum latihan berikutnya, pastikan keadaan sudah pulih.

---

## Latihan 1: Load balancing antar pod

**Konsep:** Service membagi traffic ke semua pod yang ready; port-forward tidak.

Di **Terminal 3**:

```bash
kubectl apply -f k8s/extras/service-nodeport.yaml
for i in $(seq 10); do curl -s localhost:8090/ | grep -o '"hostname":"[^"]*"'; done
```

- [ ] Output memuat **dua nama pod berbeda** (urutannya acak, bukan selalu bergantian)
- [ ] Kamu paham kenapa lewat `localhost:8081` hasilnya selalu satu nama
- [ ] Kamu bisa menjelaskan jalur: laptop 8090 ke node 30080 (NodePort) ke Service ke salah satu pod
- [ ] (Bukti) `kubectl -n tokoku get svc` menampilkan dua Service: `api` (ClusterIP) dan `api-nodeport`

Jika `localhost:8090` tidak tersambung: cluster dibuat dari config yang tidak punya `extraPortMappings` (yaitu cluster tidak dibuat dari [kind-3node.yaml](../k8s/kind/kind-3node.yaml)); buat ulang dengan `kind delete cluster --name lab` lalu ulangi langkah pembuatan di 6A bagian A.

---

## Latihan 2: Rolling update dan rollback

**Konsep:** `maxSurge: 1`, `maxUnavailable: 0` mengganti pod tanpa downtime.

Perhatikan **Terminal 2** selama langkah ini. Jalankan di **Terminal 3**:

```bash
docker build --build-arg VERSION=v0.2.0 -t tokoku-api:v0.2.0 .
docker save tokoku-api:v0.2.0 -o image.tar
kind load image-archive image.tar --name lab
rm image.tar
kubectl -n tokoku set image deployment/api api=tokoku-api:v0.2.0
kubectl -n tokoku rollout status deployment/api --timeout=120s
```

Masih di **Terminal 3**:

```bash
curl -s localhost:8090/ | grep -o '"version":"[^"]*"'    # pod baru melapor v0.2.0
kubectl -n tokoku rollout history deployment/api
kubectl -n tokoku rollout undo deployment/api
kubectl -n tokoku rollout history deployment/api
```

- [ ] Di Terminal 2 terlihat pod baru muncul `0/1` lalu `1/1` **sebelum** pod lama `Terminating`
- [ ] Selama rollout tidak pernah ada momen tanpa pod `1/1`
- [ ] Sesudah rollout, `version` menjadi `v0.2.0`
- [ ] `kubectl -n tokoku rollout undo deployment/api` mengembalikan `version` ke `dev`
- [ ] `kubectl -n tokoku rollout history deployment/api` menampilkan revisi bertambah (rollback membuat revisi baru, bukan menghapus)
- [ ] Kamu paham: image `v0.2.0` ada di node karena kamu membuild dan memuatnya dengan `kind load` sebelum `set image`

---

## Latihan 3: `ImagePullBackOff`

**Konsep:** rollout yang gagal tidak menjatuhkan layanan.

Di **Terminal 3** (lihat juga Terminal 2). `<pod-yang-baru>` = pod yang statusnya bukan `Running`:

```bash
kubectl -n tokoku set image deployment/api api=tokoku-api:tidak-ada
kubectl -n tokoku get pods
kubectl -n tokoku describe pod <pod-yang-baru>
kubectl -n tokoku get events --sort-by=.lastTimestamp
```

- [ ] Satu pod baru berstatus `ErrImagePull` lalu `ImagePullBackOff`
- [ ] Dua pod lama **tetap** `1/1` dan `curl localhost:8090/` terus menjawab
- [ ] `describe` / `events` menyebut gagal menarik image `tokoku-api:tidak-ada` (dari registry, karena tidak ada di node)
- [ ] Kamu bisa menjelaskan kenapa layanan tidak terganggu (`maxUnavailable: 0`: pod lama baru dihapus setelah pod baru ready, yang tidak pernah terjadi)
- [ ] `kubectl -n tokoku rollout undo deployment/api` memulihkan; `kubectl -n tokoku get deploy,rs,pods,svc,hpa -o wide` kembali bersih

---

## Latihan 4: `CrashLoopBackOff`

**Konsep:** proses langsung mati saat start; Kubernetes terus mencoba dengan jeda yang makin lama.

Di **Terminal 3**. `<pod-yang-crash>` = pod baru yang statusnya `Error` atau `CrashLoopBackOff`:

```bash
kubectl -n tokoku set env deployment/api PORT=abc
kubectl -n tokoku get pods
kubectl -n tokoku logs <pod-yang-crash>
kubectl -n tokoku logs <pod-yang-crash> --previous   # jika log kosong
kubectl -n tokoku describe pod <pod-yang-crash>
```

- [ ] Pod baru masuk `Error` lalu `CrashLoopBackOff`, `RESTARTS` terus naik
- [ ] Log menunjukkan server gagal start karena port `abc` tidak valid
- [ ] Pod lama tetap melayani (alasannya sama seperti latihan 3)
- [ ] Di `describe`: `Last State: Terminated`, `Exit Code: 1`
- [ ] Perbaiki: `kubectl -n tokoku set env deployment/api PORT-` (tanda minus menghapus variabel), lalu `kubectl -n tokoku rollout status deployment/api`
- [ ] Kamu paham urutan diagnosis: **logs dulu** untuk crash, **events/describe** untuk masalah penjadwalan atau image

---

## Latihan 5: `OOMKilled`

**Konsep:** melewati limit memori membuat kernel mematikan container.

Di **Terminal 3**. Perintah `curl` memakai port-forward di Terminal 1. `<pod-yang-kena>` = pod yang `RESTARTS`-nya naik:

```bash
curl -X POST "localhost:8081/debug/mem?mb=200"      # limit pod = 128Mi
kubectl -n tokoku get pods
kubectl -n tokoku describe pod <pod-yang-kena> | grep -A8 "Last State"
```

- [ ] Request terputus atau gagal, bukan membalas sukses
- [ ] `RESTARTS` pod bertambah menjadi 1
- [ ] `describe` menampilkan `Reason: OOMKilled` dan `Exit Code: 137`
- [ ] Kamu paham beda `requests` (jaminan untuk scheduler) dan `limits` (batas keras)
- [ ] Setelah restart, memori yang ditahan hilang (proses baru dimulai bersih); kamu paham kenapa `DELETE /debug/mem` tidak diperlukan di sini
- [ ] Terminal 1 (port-forward) mungkin putus; jalankan ulang `kubectl -n tokoku port-forward svc/api 8081:80` di Terminal 1
- [ ] Eksperimen (opsional): `kubectl -n tokoku set resources deployment/api --limits=memory=512Mi`, tunggu rollout, ulangi `mb=200`: sekarang berhasil. Kembalikan dengan `kubectl -n tokoku rollout undo deployment/api`

Perbaikan di dunia nyata: naikkan limit **jika** kebutuhannya wajar, atau perbaiki kebocoran memori di kode. Menaikkan limit untuk kebocoran hanya menunda.

---

## Latihan 6: `Pending`

**Konsep:** scheduler tidak menemukan node yang cukup resource.

Di **Terminal 3**:

```bash
kubectl -n tokoku set resources deployment/api --requests=memory=50Gi --limits=memory=50Gi
kubectl -n tokoku get pods
kubectl -n tokoku get events --sort-by=.lastTimestamp
```

- [ ] Pod baru berstatus `Pending` (belum ditempatkan di node mana pun: kolom NODE `<none>`)
- [ ] `events` memuat `FailedScheduling` dengan alasan memori tidak cukup (`Insufficient memory`)
- [ ] Pod lama tetap `1/1`
- [ ] Kamu bisa membedakan `Pending` (belum dijadwalkan) dari `ContainerCreating` dan `CrashLoopBackOff` (sudah jalan lalu mati)
- [ ] `kubectl -n tokoku rollout undo deployment/api` memulihkan

---

## Latihan 7: Readiness gagal

**Konsep:** pod yang tidak siap dikeluarkan dari Service, tetapi tidak di-restart.

Di **Terminal 3**. Sebelum mulai, pastikan Terminal 1 (port-forward) berjalan:

```bash
kubectl -n tokoku get endpoints api                # catat dua IP
curl -X POST "localhost:8081/debug/ready?state=false"
sleep 12
kubectl -n tokoku get pods
kubectl -n tokoku get endpoints api
curl -s localhost:8090/ | grep -o '"hostname":"[^"]*"'   # perlu NodePort dari latihan 1
```

Catatan: yang jadi `0/1` adalah pod yang kena port-forward. Hostname-nya ada di respons `curl` perintah `POST` tadi.

- [ ] Satu pod menjadi `0/1`, `RESTARTS` tetap `0`
- [ ] `endpoints api` tinggal **satu** IP
- [ ] Lewat `localhost:8090`, semua respons datang dari pod yang masih ready
- [ ] `describe pod` memuat event `Readiness probe failed: HTTP probe failed with statuscode: 503`
- [ ] Waktu sampai terdeteksi sekitar 10 detik (periode 5 dtk x 2 kali gagal): kamu tahu angkanya dari `deployment.yaml`
- [ ] Pulihkan: `curl -X POST "localhost:8081/debug/ready?state=true"`; pod kembali `1/1` dan IP kembali dua
- [ ] Kamu bisa menjawab: kapan readiness dipakai di dunia nyata? (sedang warm-up, sedang kehilangan koneksi database, sedang overload)

---

## Latihan 8: Liveness gagal

**Konsep:** liveness gagal berarti container di-restart.

Di **Terminal 3**:

```bash
curl -X POST "localhost:8081/debug/health?state=false"
```

Pindah lihat **Terminal 2** (jangan ketik apa-apa di sana) dan tunggu sekitar 30 detik sampai kolom `RESTARTS` salah satu pod naik. Lalu di **Terminal 3** (`<pod-yang-kena>` = pod itu):

```bash
kubectl -n tokoku describe pod <pod-yang-kena> | tail -20
```

- [ ] Setelah sekitar 30 detik (periode 10 dtk x 3 kali gagal), `RESTARTS` naik menjadi 1
- [ ] `describe` memuat `Liveness probe failed` lalu `Container api failed liveness probe, will be restarted`
- [ ] Setelah restart, pod sehat lagi tanpa kamu berbuat apa-apa (status `healthy` kembali default `true`)
- [ ] Kamu bisa membandingkan dengan latihan 7 dalam satu kalimat: readiness = berhenti dikirimi traffic; liveness = dimatikan dan dinyalakan ulang
- [ ] Kamu paham bahaya liveness yang terlalu agresif (container sehat tapi lambat bisa di-restart terus)

---

## Latihan 9: Aplikasi lambat start

**Konsep:** readiness (dan startup probe) menahan traffic sampai aplikasi siap.

Buka [k8s/base/configmap.yaml](../k8s/base/configmap.yaml) di editor (VS Code), ubah `STARTUP_DELAY: "0s"` menjadi `"20s"`, lalu simpan (`Cmd+S`).

Di **Terminal 3**:

```bash
kubectl apply -k k8s/base
kubectl -n tokoku rollout restart deployment/api
```

Segera setelah itu, di **Terminal 3** juga (loop ini memanggil aplikasi 40 kali selama 40 detik), sambil melihat **Terminal 2**:

```bash
for i in $(seq 40); do curl -s -o /dev/null -w "%{http_code} " localhost:8090/; sleep 1; done; echo
```

- [ ] Pod baru `Running` tetapi `0/1` sekitar 20 detik, lalu `1/1`
- [ ] Selama itu pod baru tidak menerima traffic dan loop `curl` tetap `200` semua (pod lama masih melayani)
- [ ] Kamu paham kenapa `kubectl apply -k k8s/base` saja tidak cukup (ConfigMap tidak otomatis me-restart pod)
- [ ] **Kembalikan** `STARTUP_DELAY` ke `"0s"` di editor, simpan, lalu di Terminal 3 jalankan `kubectl apply -k k8s/base` dan `kubectl -n tokoku rollout restart deployment/api`. Pastikan `git status` tidak menampilkan `k8s/base/configmap.yaml` sebagai berubah
- [ ] Kamu bisa menjelaskan beda "container berjalan" dan "pod siap" (`Running` tidak berarti `Ready`)

---

## Latihan 10: Node mati

**Konsep:** Kubernetes mendeteksi node mati dan memindahkan pod, tetapi tidak instan.

Di **Terminal 3**:

```bash
kubectl -n tokoku get pods -o wide                 # catat pod ada di node mana
docker stop lab-worker
```

Selanjutnya, di **Terminal 3** (biarkan berjalan; `Ctrl+C` untuk berhenti) pantau node sampai `lab-worker` menjadi `NotReady` (sekitar 40 detik):

```bash
kubectl get nodes -w
```

Lalu `Ctrl+C`, dan pantau pod (Terminal 2 juga memperlihatkannya, tapi tanpa kolom NODE):

```bash
kubectl -n tokoku get pods -o wide
```

- [ ] `lab-worker` berubah `NotReady`
- [ ] Pod di node itu tidak langsung pindah; sekitar 5 menit kemudian pod dievaluasi dan digantikan di node lain (waktu tunggu bawaan taint `NoExecute`)
- [ ] Selama menunggu, layanan tetap melayani lewat pod di node yang sehat (cek `curl localhost:8090/` dan `get endpoints api`)
- [ ] Kamu bisa menjelaskan kenapa 2 replika di 2 node lebih tahan daripada 2 replika di 1 node
- [ ] Pulihkan di Terminal 3: `docker start lab-worker`; node kembali `Ready`
- [ ] (Cek akhir) `kubectl -n tokoku get deploy,rs,pods,svc,hpa -o wide` bersih; kalau ada pod `Terminating` tersisa, tunggu sampai hilang

Jika 5 menit terasa lama, boleh tidak ditunggu; yang penting kamu mengamati `NotReady` dan memahami angka 5 menit itu.

---

## Latihan 11: Graceful shutdown

**Konsep:** pod dihapus tanpa satu request pun gagal.

**Terminal 3**: jalankan 300 request berurutan dan hitung kode statusnya (butuh sekitar 35 detik; biarkan sampai selesai):

```bash
for i in $(seq 300); do curl -s -o /dev/null -w "%{http_code}\n" localhost:8090/; sleep 0.1; done | sort | uniq -c
```

Setelah loop berjalan beberapa detik, di **Terminal 4** (buka terminal baru, `cd` ke root repo; Terminal 1 dan 2 jangan dipakai mengetik) hapus satu pod. `<nama-salah-satu-pod>` diambil dari Terminal 2:

```bash
kubectl -n tokoku delete pod <nama-salah-satu-pod>
```

- [ ] Hasil akhir hanya `300 200` (tidak ada `000`, `502`, atau `503`)
- [ ] Pod pengganti muncul otomatis (Deployment menjaga jumlah replika)
- [ ] Kamu bisa menjelaskan urutan saat pod dihapus: pod ditandai Terminating, Service berhenti mengirim traffic, SIGTERM dikirim, aplikasi menunggu `SHUTDOWN_DELAY` (5 dtk) sambil `readyz` gagal, lalu menyelesaikan request yang berjalan, lalu keluar
- [ ] Kamu paham kenapa jeda `SHUTDOWN_DELAY` perlu: pembaruan Service dan SIGTERM terjadi hampir bersamaan, sehingga ada celah waktu request masih tiba di pod yang sudah berhenti
- [ ] (Opsional) Ulangi dengan `SHUTDOWN_DELAY: "0s"` di ConfigMap (edit di editor, lalu di Terminal 3 jalankan `kubectl apply -k k8s/base` dan `kubectl -n tokoku rollout restart deployment/api`). Hasil bisa berbeda (kadang ada kegagalan), tapi tidak selalu muncul; jangan menyimpulkan dari satu kali coba. Kembalikan ke `5s`

---

## Rangkuman: tabel diagnosis

Isi sendiri dari hasil latihanmu (tanpa menyalin README), lalu bandingkan dengan README.

| Gejala | Penyebab | Perintah pertama untuk mengecek | Perbaikan |
| --- | --- | --- | --- |
| `ImagePullBackOff` |  |  |  |
| `CrashLoopBackOff` |  |  |  |
| `OOMKilled` |  |  |  |
| `Pending` |  |  |  |
| `0/1` tapi `Running` |  |  |  |
| `RESTARTS` naik tanpa crash |  |  |  |

## Catatan

-

## Selesai?

- [ ] Latihan 1 sampai 11 tercentang
- [ ] Tabel diagnosis terisi dari ingatanmu
- [ ] `kubectl -n tokoku get deploy,rs,pods,svc,hpa -o wide` bersih (semua `1/1`), `git diff` tidak menunjukkan sisa perubahan di `k8s/`
- [ ] Minta seseorang (atau dirimu besok) merusak satu hal secara acak, dan kamu bisa menemukan penyebabnya hanya dari `get`, `describe`, `logs`, `events`
