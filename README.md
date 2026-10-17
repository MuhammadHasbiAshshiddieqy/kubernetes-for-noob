# kubernetes-for-noob

Repo latihan Kubernetes memakai aplikasi Go kecil bernama **Tokoku API**. Dipakai bersama kurikulum DevOps (Modul 4 sampai 7): container, cluster kind, probes, rolling update, dan scaling.

Aplikasinya sengaja punya endpoint `/debug/*` untuk merusak dirinya sendiri (crash, kebocoran memori, probe gagal, beban CPU), supaya setiap latihan "rusak dan perbaiki" bisa dikerjakan tanpa mengubah kode.

Mau belajar langkah demi langkah dengan checklist? Mulai dari [praktek/README.md](praktek/README.md).

## Isi repo

```
.
├── cmd/api/                  # aplikasi Go (hanya standard library) + unit test
├── Dockerfile                # multi-stage, distroless, bisa multi-arch
├── Makefile                  # semua perintah umum; jalankan `make`
├── k8s/
│   ├── base/                 # manifest utama (Kustomize): Namespace, ConfigMap, Deployment, Service
│   ├── extras/               # HPA, PDB, NodePort (dipakai mulai Modul 7)
│   └── kind/                 # konfigurasi cluster kind (biasa dan tanpa CNI untuk Cilium)
└── loadtest/hpa.js           # skrip k6 untuk latihan autoscaling
```

## Prasyarat

Docker Desktop harus berjalan, lalu pasang tools (macOS):

```bash
brew install kind kubectl
# opsional, tapi berguna sejak awal:
brew install k9s kubectx stern
# dibutuhkan mulai Modul 7:
brew install helm k6
```

`kubectl` mungkin sudah dipasang Docker Desktop; cek dengan `kubectl version --client` dan jangan pasang dua kali. Untuk menjalankan aplikasi langsung di laptop (`make run`, `make test`) kamu juga butuh Go.

Jika fitur Kubernetes bawaan Docker Desktop aktif, matikan dulu saat memakai kind, supaya dua cluster tidak berebut memori.

## Mulai cepat

```bash
make test          # 1. pastikan kode lolos tes
make up            # 2. buat cluster kind 3 node, build image, muat ke cluster, deploy
make context       # 3. harus menampilkan: kind-lab
make status        # 4. 2 pod api harus Running dan READY 1/1

make port-forward  # 5. terminal 1: buka http://localhost:8081
make smoke         #    terminal 2: cek endpoint
```

Membersihkan semuanya: `make down`.

Perintah lain: `make help`.

## Port yang dipakai

| Port laptop | Dari mana | Catatan |
| --- | --- | --- |
| 8080 | `make run` atau `make docker-run` | Aplikasi di luar cluster |
| 8081 | `make port-forward` | Menembus **satu** pod saja, jadi tidak membagi beban |
| 8090 | `make nodeport` | NodePort yang dipetakan kind; membagi traffic ke **semua** pod |

## Endpoint aplikasi

| Endpoint | Fungsi |
| --- | --- |
| `GET /` | JSON berisi pesan, versi, env, dan nama pod (`hostname`) |
| `GET /healthz` | Liveness probe |
| `GET /readyz` | Readiness probe |
| `GET /metrics` | Metrik format Prometheus (request, durasi, goroutine, memori) |
| `GET /debug/slow?ms=500` | Membalas setelah jeda |
| `GET /debug/cpu?ms=100` | Memakai 1 core selama sekian milidetik |
| `POST /debug/mem?mb=50` | Menahan memori; `DELETE /debug/mem` melepasnya |
| `POST /debug/ready?state=false` | Membuat `/readyz` gagal |
| `POST /debug/health?state=false` | Membuat `/healthz` gagal |
| `POST /debug/crash` | Proses keluar dengan kode 1 |

Endpoint `/debug/*` dikendalikan variabel `ENABLE_DEBUG` di ConfigMap. Jangan aktifkan di production.

## Latihan Modul 6: rusak dan perbaiki

Jalankan `make port-forward` di satu terminal untuk latihan yang memanggil `localhost:8081`, dan `make events` serta `make status` di terminal lain. Ingat: port-forward menembus satu pod, jadi perubahan keadaan hanya terjadi di pod itu.

| # | Latihan | Perintah | Yang diamati | Perbaikan |
| --- | --- | --- | --- | --- |
| 1 | Load balancing antar pod | `make nodeport`, lalu `for i in $(seq 10); do curl -s localhost:8090/ \| grep -o '"hostname":"[^"]*"'; done` | Nama pod bergantian | - |
| 2 | Rolling update | `make rollout VERSION=v0.2.0`, sambil `kubectl -n tokoku get pods -w` | Pod baru siap dulu, baru yang lama dihapus | `make rollback` |
| 3 | `ImagePullBackOff` | `kubectl -n tokoku set image deployment/api api=tokoku-api:tidak-ada` | Pod baru gagal, pod lama tetap melayani | `make rollback` |
| 4 | `CrashLoopBackOff` | `kubectl -n tokoku set env deployment/api PORT=abc` | Pod baru crash berulang; baca `kubectl -n tokoku logs <pod>` | `kubectl -n tokoku set env deployment/api PORT-` |
| 5 | `OOMKilled` | `curl -X POST "localhost:8081/debug/mem?mb=200"` | `RESTARTS` naik; `describe pod` menampilkan `OOMKilled` (limit memori 128Mi) | Naikkan limit atau perbaiki kebocoran |
| 6 | `Pending` | `kubectl -n tokoku set resources deployment/api --requests=memory=50Gi --limits=memory=50Gi` | Pod baru `Pending`; `make events` menyebut memori tidak cukup | `make rollback` |
| 7 | Readiness gagal | `curl -X POST "localhost:8081/debug/ready?state=false"` | Pod jadi `0/1`, keluar dari `kubectl -n tokoku get endpoints api`, **tidak** di-restart | `.../debug/ready?state=true` |
| 8 | Liveness gagal | `curl -X POST "localhost:8081/debug/health?state=false"` | Sekitar 30 detik kemudian container di-restart | Otomatis |
| 9 | Aplikasi lambat start | Ubah `STARTUP_DELAY` jadi `20s` di `k8s/base/configmap.yaml`, lalu `make deploy restart` | Pod `0/1` selama 20 detik; traffic tidak masuk sebelum siap | Kembalikan ke `0s` |
| 10 | Node mati | `docker stop lab-worker` | `kubectl get nodes -w` menampilkan `NotReady`; pod dipindah setelah sekitar 5 menit | `docker start lab-worker` |
| 11 | Graceful shutdown | Jalankan loop `curl` ke `localhost:8090`, lalu `kubectl -n tokoku delete pod <nama>` | Tidak ada request gagal, berkat `SHUTDOWN_DELAY` dan `maxUnavailable: 0` | - |

Perubahan ConfigMap tidak otomatis me-restart pod. Setelah `make deploy`, jalankan `make restart`.

Latihan 2 mengubah image lewat `kubectl set image`, jadi isi cluster berbeda dari isi `deployment.yaml` di Git. Itu sengaja untuk tahap ini; di Modul 8 (GitOps) semua perubahan hanya lewat Git.

## Latihan Modul 7: scaling dan ketahanan

```bash
make metrics-server   # dibutuhkan HPA dan `kubectl top`
make nodeport         # jalur beban yang terbagi ke semua pod
make hpa
make watch-hpa        # terminal 1: pantau
make loadtest         # terminal 2: beban naik 20 -> 80 pengguna selama 5 menit
```

Amati pod bertambah saat beban naik dan berkurang setelah beban berhenti (jeda penurunan diatur 60 detik di `hpa.yaml`). Catatan: `make deploy` mengembalikan jumlah replika ke nilai di `deployment.yaml` (2), jadi jangan jalankan saat HPA sedang diamati.

Latihan lanjutan:

```bash
make pdb
kubectl drain lab-worker --ignore-daemonsets --delete-emptydir-data   # sambil loop curl ke :8090
kubectl uncordon lab-worker
```

Untuk latihan Cilium (Modul 7), gunakan cluster tanpa CNI bawaan:

```bash
make down
make up-cilium
```

## Image ke registry (GitHub Container Registry)

Cluster kind memakai image yang dimuat langsung (`make kind-load`), jadi push ke registry belum wajib. Dipakai mulai Modul 5 (CI/CD) dan 8 (GitOps).

```bash
make ghcr-login                 # petunjuk membuat token dan login (tanpa kartu)
make push VERSION=v0.1.0        # build amd64 + arm64, lalu push ke ghcr.io
```

Image dibangun untuk **dua arsitektur**: chip Apple Silicon menghasilkan arm64, sedangkan server production umumnya amd64. Ganti `GHCR_USER` di Makefile (atau `make push GHCR_USER=...`) jika username GitHub-mu berbeda. Nama image di GHCR harus huruf kecil semua.

## Alternatif: Kubernetes bawaan Docker Desktop

Untuk Modul 6 kamu bisa memakai cluster dari Docker Desktop (Settings, Kubernetes, metode kind, 3 node) sebagai pengganti `make kind-create`:

```bash
kubectl config use-context docker-desktop   # cek nama context dengan: kubectl config get-contexts
make docker-build deploy
```

Sebagian latihan butuh kind CLI (Cilium, `docker stop lab-worker`, port 8090), jadi gunakan kind CLI mulai Modul 7.

## Pemecahan masalah

| Gejala | Penyebab umum | Solusi |
| --- | --- | --- |
| `ErrImagePull` / `ImagePullBackOff` untuk `tokoku-api:dev` | Image belum dimuat ke cluster | `make kind-load` |
| `kind-load` gagal | Cluster belum ada atau namanya berbeda | `make kind-create`; pastikan `CLUSTER` sesuai |
| `make smoke` gagal koneksi | `make port-forward` belum berjalan | Jalankan di terminal lain |
| Perintah `kubectl` ke cluster yang salah | Context aktif bukan `kind-lab` | `make context`, lalu `kubectl config use-context kind-lab` |
| Port 8080/8081/8090 sudah dipakai | Proses lain di laptop | Hentikan prosesnya atau ganti port |
| HPA menampilkan `<unknown>` | metrics-server belum terpasang atau `requests` tidak ada | `make metrics-server`; tunggu satu menit |

## Status pengujian

Sudah diuji: unit test (`go test -race`), `go vet`, `gofmt`, menjalankan binary (probe, endpoint debug, metrik, graceful shutdown dengan SIGTERM), serta validasi sintaks dan konsistensi manifest YAML.

Belum diuji karena membutuhkan Docker dan cluster: build image, `make up`, `make push`, Helm (metrics-server dan Cilium), dan k6. Jika ada perintah yang gagal, tempel pesan errornya agar bisa diperbaiki.
