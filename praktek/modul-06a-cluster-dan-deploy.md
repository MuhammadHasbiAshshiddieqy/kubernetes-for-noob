# Modul 6A: Cluster kind dan Deploy

Tujuan: membuat cluster 3 node di laptop, menjalankan Tokoku API di dalamnya, dan **memahami setiap manifest** sebelum merusaknya di Modul 6B.

Prasyarat: Modul 4 selesai (image `tokoku-api:dev` bisa dibuild), Docker Desktop jalan, Kubernetes bawaan Docker Desktop mati.

File yang dibaca, sambil mengerjakan:
- [k8s/kind/kind-3node.yaml](../k8s/kind/kind-3node.yaml)
- [k8s/base/namespace.yaml](../k8s/base/namespace.yaml), [configmap.yaml](../k8s/base/configmap.yaml), [deployment.yaml](../k8s/base/deployment.yaml), [service.yaml](../k8s/base/service.yaml), [kustomization.yaml](../k8s/base/kustomization.yaml)

## Sebelum mulai (baca sekali)

- **Semua perintah dijalankan di terminal laptopmu sendiri.** Cluster kind tidak berada di server lain: ia berupa tiga container Docker di laptopmu. Perintah `kubectl` di terminalmu berbicara ke cluster itu lewat jaringan lokal.
- Setiap terminal baru harus berada di **folder root repo**:

  ```bash
  cd /Users/hasbi.shiddieqy/Documents/Personal/Github/kubernetes-for-noob
  pwd        # harus menampilkan path di atas
  ```
- Blok berjudul **Terminal 1** dan **Terminal 2** berarti kamu butuh dua jendela/tab terminal. Blok tanpa judul = jalankan di terminal mana pun (satu terminal cukup), baris demi baris dari atas ke bawah.
- **(biarkan berjalan)** berarti perintah itu tidak selesai sendiri. Jangan tutup, pindah ke terminal lain. Hentikan dengan `Ctrl+C` di terminal itu.
- Perintah yang menampilkan teks panjang lewat `less`: tekan `q` untuk keluar.

---

## A. Buat cluster dan deploy

### Langkah

Satu terminal. Jalankan berurutan, tunggu tiap perintah selesai sebelum lanjut (langkah 1 butuh 1 sampai 2 menit).

```bash
go test ./... -race -count=1

# 1. buat cluster 3 node bernama "lab"
kind create cluster --name lab --config k8s/kind/kind-3node.yaml

# 2. build image
docker build --build-arg VERSION=dev -t tokoku-api:dev .

# 3. salin image ke semua node kind
docker save tokoku-api:dev -o image.tar
kind load image-archive image.tar --name lab
rm image.tar

# 4. deploy manifest, tunggu sampai siap
kubectl apply -k k8s/base
kubectl -n tokoku rollout status deployment/api --timeout=120s
```

Empat langkah itu berurutan dan masing-masing bergantung pada sebelumnya. Pastikan kamu tahu untuk apa tiap langkah: cluster harus ada dulu, image harus ada di node sebelum pod memakainya (`imagePullPolicy: IfNotPresent`, tidak ada registry), baru manifest diterapkan. `kind load` dijalankan lewat `docker save` agar aman di Docker Desktop dengan containerd image store.

Lalu periksa hasilnya (satu terminal):

```bash
kubectl config current-context                # harus: kind-lab
kubectl get nodes -o wide                     # 3 node
docker ps --format '{{.Names}}'               # node kind = container Docker
kubectl -n tokoku get deploy,rs,pods,svc,hpa -o wide
```

### Checklist A

- [ ] `kubectl config current-context` menampilkan **kind-lab**
- [ ] `kubectl get nodes` menampilkan 3 node `Ready`: `lab-control-plane`, `lab-worker`, `lab-worker2`
- [ ] `docker ps` menampilkan tiga container dengan nama yang sama dengan node (kamu paham: node kind = container)
- [ ] `kubectl -n tokoku get deploy,rs,pods,svc,hpa -o wide` menampilkan Deployment `api` 2/2, dua pod `Running` `1/1`, dan Service `api`
- [ ] Kamu mencatat dua pod ada di node mana (kolom NODE) dan bisa menjelaskan siapa yang memutuskan itu (scheduler)

---

## B. Akses aplikasi

Service `api` hanya bisa dijangkau dari dalam cluster. `port-forward` membuat terowongan dari laptopmu ke sana.

### Langkah

**Terminal 1** (biarkan berjalan): membuka terowongan `localhost:8081` di laptopmu ke Service `api`.

```bash
kubectl -n tokoku port-forward svc/api 8081:80
```

Keluarannya `Forwarding from 127.0.0.1:8081 -> 8080`, lalu terminal tampak menggantung. Itu normal.

**Terminal 2**: memanggil aplikasi lewat terowongan.

```bash
for p in / /healthz /readyz; do printf "%-10s " $p; curl -s -o /dev/null -w "%{http_code}\n" localhost:8081$p; done
curl -s localhost:8081/
```

Ulangi `curl -s localhost:8081/` beberapa kali di **Terminal 2** dan perhatikan `hostname`.

Biarkan **Terminal 1** tetap berjalan; bagian C dan D bisa memakainya atau terminal baru.

### Checklist B

- [ ] Loop `curl` di atas menampilkan `200` untuk `/`, `/healthz`, dan `/readyz`
- [ ] `hostname` di respons adalah nama pod (contoh `api-5d8c...-abcde`), cocok dengan `kubectl -n tokoku get pods`
- [ ] Kamu mengamati bahwa `hostname` **selalu sama** lewat port-forward, dan paham alasannya: port-forward menembus satu pod, tidak membagi beban (nanti dibuktikan di 6B latihan 1)

---

## C. Membaca manifest (jangan dilewati)

Kerjakan sambil membuka file. Untuk tiap pertanyaan, cari jawabannya di file lalu buktikan dengan perintah.

Satu terminal (**Terminal 2** atau yang baru; `less` memerlukan `q` untuk keluar):

```bash
kubectl -n tokoku get deploy api -o yaml | less
kubectl -n tokoku describe pod -l app=api | less
kubectl -n tokoku get endpoints api
kubectl -n tokoku get configmap api-config -o yaml
```

### Checklist C

- [ ] **Namespace**: kamu tahu kenapa semua perintah memakai `-n tokoku` (resource ada di namespace itu, dipasang oleh `namespace.yaml` dan `namespace:` di kustomization)
- [ ] **ConfigMap**: kamu bisa menyebut 5 key-nya (`APP_ENV`, `GREETING`, `ENABLE_DEBUG`, `STARTUP_DELAY`, `SHUTDOWN_DELAY`) dan tahu bagaimana masuk ke pod (`envFrom`)
- [ ] **Deployment, replicas**: `replicas: 2`, dan kamu tahu Deployment mengelola ReplicaSet yang mengelola Pod (`kubectl -n tokoku get deploy,rs,pods`)
- [ ] **Strategy**: `maxSurge: 1`, `maxUnavailable: 0` artinya pod baru dinyalakan dulu dan harus siap sebelum yang lama dihapus
- [ ] **Resources**: `requests` (50m CPU, 64Mi) dipakai scheduler dan HPA; `limits` (250m, 128Mi) adalah batas keras. Melewati limit memori = `OOMKilled`
- [ ] **Tiga probe** dan kapan masing-masing berlaku:
  - [ ] `startupProbe`: memberi waktu start (2 dtk x 30 = sampai 60 dtk), liveness belum jalan sebelum ini lulus
  - [ ] `livenessProbe`: gagal 3 kali (periode 10 dtk) = container di-restart
  - [ ] `readinessProbe`: gagal 2 kali (periode 5 dtk) = keluar dari Service, tanpa restart
- [ ] **Security**: `runAsNonRoot`, `readOnlyRootFilesystem`, `capabilities.drop: ALL`; kamu tahu ini cocok dengan image distroless non-root dari Modul 4
- [ ] **Service**: `selector: app=api` memilih pod berlabel sama; `port: 80` meneruskan ke `targetPort: http` (8080). `kubectl get endpoints api` menampilkan IP dua pod
- [ ] **Hubungan label**: kamu bisa menunjukkan tiga tempat label `app: api` harus cocok (selector Deployment, label template pod, selector Service)
- [ ] **imagePullPolicy: IfNotPresent** dengan `image: tokoku-api:dev`: kamu tahu image itu tidak ada di registry, melainkan dimuat oleh `kind load`
- [ ] **kind-3node.yaml**: `extraPortMappings` memetakan port 8090 laptop ke 30080 di node (dipakai di 6B latihan 1)

---

## D. Operasi harian

### Langkah

**Terminal 2** (biarkan berjalan): mengikuti log semua pod. `Ctrl+C` untuk berhenti.

```bash
kubectl -n tokoku logs -l app=api --tail=30 -f --max-log-requests=10
```

**Terminal 3** (terminal baru, ingat `cd` ke root repo): kirim beberapa request, lalu lihat Terminal 2.

```bash
curl -s localhost:8081/ > /dev/null
```

Hentikan log di **Terminal 2** dengan `Ctrl+C`, lalu di **Terminal 3** (atau Terminal 2):

```bash
kubectl -n tokoku scale deployment/api --replicas=4
kubectl -n tokoku get pods
kubectl -n tokoku scale deployment/api --replicas=2
kubectl -n tokoku rollout restart deployment/api
kubectl -n tokoku rollout status deployment/api
kubectl -n tokoku rollout history deployment/api
kubectl -n tokoku get events --sort-by=.lastTimestamp
```

Agar lebih jelas, saat `rollout restart` jalankan di terminal lain (**Terminal 2**, biarkan berjalan lalu `Ctrl+C`):

```bash
kubectl -n tokoku get pods -w
```

### Checklist D

- [ ] `kubectl logs ... -f` menampilkan log JSON dari pod; request lewat port-forward muncul di **satu** pod saja
- [ ] `scale --replicas=4` menghasilkan 4 pod, `--replicas=2` kembali ke 2
- [ ] `rollout restart` menggilir pod satu per satu (terlihat di `get pods -w`)
- [ ] Port-forward di Terminal 1 bisa putus karena pod yang ditembus diganti. Kalau `curl localhost:8081` gagal, jalankan ulang perintah port-forward di Terminal 1
- [ ] `rollout history` menampilkan lebih dari satu revisi setelah restart
- [ ] Kamu tahu `get events --sort-by=.lastTimestamp` adalah tempat pertama mencari penyebab masalah

---

## E. Idempoten dan bersih-bersih

### Langkah

Satu terminal:

```bash
kubectl apply -k k8s/base        # kedua kali: tidak ada perubahan (unchanged)
kubectl delete -k k8s/base       # hapus semua
kubectl get ns                   # namespace tokoku hilang
kubectl apply -k k8s/base        # kembali
kubectl -n tokoku rollout status deployment/api --timeout=120s
```

Opsional: hapus seluruh cluster (ulangi bagian A untuk membuat lagi):

```bash
kind delete cluster --name lab
```

### Checklist E

- [ ] `kubectl apply -k k8s/base` ulang melaporkan `unchanged` (kamu paham ini sifat deklaratif `kubectl apply`)
- [ ] Setelah `kubectl delete -k k8s/base`, namespace `tokoku` beserta isinya hilang
- [ ] (Opsional) menghapus cluster lalu mengulang bagian A berhasil membangun ulang dari nol: kamu bisa mengulang lab kapan pun

## Cek pemahaman

1. Pod `ImagePullBackOff` untuk `tokoku-api:dev`. Apa dugaan pertamamu?
   <details><summary>Jawaban</summary>Image belum dimuat ke cluster. Muat dengan `docker save tokoku-api:dev -o image.tar` lalu `kind load image-archive image.tar --name lab`. Image hanya ada di Docker laptop sampai dimuat ke node.</details>
2. Kamu mengubah `GREETING` di ConfigMap lalu `kubectl apply -k k8s/base`, tetapi `/` masih menampilkan pesan lama. Kenapa?
   <details><summary>Jawaban</summary>ConfigMap yang dipasang lewat env var hanya dibaca saat pod start. Jalankan `kubectl -n tokoku rollout restart deployment/api`.</details>
3. Apa bedanya `kubectl apply` dengan `kubectl set image`, dan kenapa README menyebut yang kedua "membuat cluster berbeda dari Git"?
   <details><summary>Jawaban</summary>`apply` mengikuti isi file manifest. `set image` mengubah langsung objek di cluster tanpa mengubah file, sehingga keadaan cluster dan Git tidak lagi sama. Modul 8 (GitOps) memperbaikinya.</details>

## Catatan

-

## Selesai?

- [ ] Checklist A sampai E tercentang
- [ ] Kamu bisa menggambar alur: `kubectl apply` ke Deployment ke ReplicaSet ke Pod, dan Service ke Endpoints ke Pod
