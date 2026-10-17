# Modul 7: Scaling dan Ketahanan

Tujuan: membuat aplikasi menambah dan mengurangi pod otomatis (HPA), bertahan saat node dikosongkan (PDB), dan mengganti jaringan cluster dengan Cilium.

Prasyarat: Modul 6A selesai, cluster `kind-lab` berjalan dari `kind-3node.yaml` (ada port 8090), `helm` dan `k6` terpasang (`brew install helm k6`; saat ini `k6` belum ada di laptopmu).

File yang dibaca: [k8s/extras/hpa.yaml](../k8s/extras/hpa.yaml), [pdb.yaml](../k8s/extras/pdb.yaml), [service-nodeport.yaml](../k8s/extras/service-nodeport.yaml), [loadtest/hpa.js](../loadtest/hpa.js), [kind-3node-cilium.yaml](../k8s/kind/kind-3node-cilium.yaml).

## Sebelum mulai (baca sekali)

- **Semua perintah dijalankan di terminal laptopmu sendiri.** Cluster kind berupa container Docker di laptopmu; `kubectl`, `helm`, dan `k6` di terminalmu berbicara ke cluster itu lewat `localhost`.
- Setiap terminal baru harus berada di **folder root repo**:

  ```bash
  cd /Users/hasbi.shiddieqy/Documents/Personal/Github/kubernetes-for-noob
  pwd        # harus menampilkan path di atas
  kubectl config current-context   # harus: kind-lab
  ```
- Blok berjudul **Terminal 1/2/3** berarti butuh beberapa jendela/tab terminal; blok tanpa judul = satu terminal, dijalankan baris demi baris dari atas ke bawah.
- **(biarkan berjalan)** berarti perintah itu tidak selesai sendiri. Hentikan dengan `Ctrl+C` di terminal itu.

---

## A. Autoscaling dengan HPA

### Langkah

Persiapan, satu terminal (tunggu tiap perintah selesai):

```bash
kubectl config current-context                  # kind-lab                  # kind-lab
kubectl -n tokoku get deploy,rs,pods,svc,hpa -o wide                   # 2 pod 1/1
helm repo add metrics-server https://kubernetes-sigs.github.io/metrics-server/
helm repo update metrics-server
helm upgrade --install metrics-server metrics-server/metrics-server \
  --namespace kube-system --set 'args={--kubelet-insecure-tls}'
# tunggu sekitar satu menit
kubectl top nodes
kubectl top pods -n tokoku
kubectl apply -f k8s/extras/service-nodeport.yaml                 # jalur beban ke SEMUA pod
kubectl apply -f k8s/extras/hpa.yaml
kubectl -n tokoku get hpa api
```

Mulai pengujian. Buka **tiga** terminal (masing-masing `cd` ke root repo):

**Terminal 1** (biarkan berjalan): memantau HPA. Baris baru muncul saat nilainya berubah.

```bash
kubectl -n tokoku get hpa api -w
```

**Terminal 2** (biarkan berjalan): memantau pod.

```bash
kubectl -n tokoku get pods -w
```

**Terminal 3**: menjalankan beban (20 lalu 80 pengguna virtual, total 5 menit; perintah selesai sendiri).

```bash
k6 run -e BASE_URL=http://localhost:8090 loadtest/hpa.js
```

Setelah k6 di Terminal 3 selesai, **tetap pantau Terminal 1 dan 2** sekitar 2 sampai 3 menit. Setelah itu tekan `Ctrl+C` di keduanya.

### Checklist A

- [ ] `kubectl top nodes` dan `kubectl top pods` menampilkan angka (artinya metrics-server hidup). Jika `<unknown>` atau error, tunggu satu menit, lalu cek `kubectl -n kube-system get pods`
- [ ] `get hpa` menampilkan target berbentuk `cpu: <angka>%/70%` (bukan `<unknown>`), `MINPODS 2`, `MAXPODS 10`
- [ ] Kamu paham dari mana persen CPU dihitung: **terhadap `requests.cpu` (50m)**, bukan terhadap limit atau CPU node. Jadi 70% = rata-rata sekitar 35m per pod
- [ ] Saat beban naik, `REPLICAS` bertambah (di atas 2) dan pod baru muncul di Terminal 2
- [ ] Pod baru yang `Running` baru menerima traffic setelah `1/1` (readiness dari Modul 6)
- [ ] Setelah beban berhenti, `REPLICAS` turun kembali ke 2, namun **tidak langsung**: ada jeda sekitar 60 detik (`stabilizationWindowSeconds: 60`, bawaan Kubernetes 300)
- [ ] Kamu bisa menjelaskan kenapa memakai NodePort (8090) dan bukan port-forward untuk uji beban: port-forward hanya menembus satu pod, sehingga CPU pod lain tidak naik dan HPA tidak bereaksi sebagaimana mestinya
- [ ] Kamu bisa menjelaskan apa yang dilakukan `/debug/cpu?ms=100` di skrip k6 dan kenapa CPU pod cepat naik (satu core terpakai penuh selama 100 ms, tapi limit hanya 250m sehingga di-throttle)
- [ ] Kamu bisa menjelaskan tiga tahap di [hpa.js](../loadtest/hpa.js) (naik pelan, tinggi, turun)

Catatan penting:
- **Jangan** menjalankan `kubectl apply -k k8s/base` saat HPA diamati: ia mengembalikan replika ke 2 (nilai `deployment.yaml`).
- Hasil jumlah replika puncak bisa berbeda antar mesin; catat angkamu di bagian Catatan.

### Pertanyaan eksperimen (opsional)

- [ ] Ubah `averageUtilization` ke 30, `kubectl apply -f k8s/extras/hpa.yaml`, ulangi `loadtest`: apakah replika puncak lebih banyak? Kembalikan ke 70
- [ ] Hapus `requests` di Deployment (jangan commit) dan lihat HPA menjadi `<unknown>`; kembalikan lagi. Pelajaran: HPA butuh `requests`

---

## B. PodDisruptionBudget dan drain node

### Langkah

Pastikan beban HPA sudah selesai dan replika kembali ke 2 (`kubectl -n tokoku get hpa api`).

Satu terminal:

```bash
kubectl apply -f k8s/extras/pdb.yaml
kubectl -n tokoku get pdb
kubectl -n tokoku get pods -o wide
```

**Terminal 1**: jalankan loop (sekitar 60 detik, selesai sendiri, cukup lama untuk meliputi drain). Mulai ini dulu:

```bash
for i in $(seq 300); do curl -s -o /dev/null -w "%{http_code}\n" localhost:8090/; sleep 0.2; done | sort | uniq -c
```

Segera setelah loop mulai, di **Terminal 2** kosongkan node (`drain` bisa makan waktu sampai satu menit; tunggu sampai prompt kembali):

```bash
kubectl drain lab-worker --ignore-daemonsets --delete-emptydir-data
kubectl -n tokoku get pods -o wide
kubectl uncordon lab-worker
kubectl get nodes
```

Setelah loop di Terminal 1 selesai, bacalah hasil hitungannya (jumlah tiap kode status).

### Checklist B

- [ ] `get pdb` menampilkan `MIN AVAILABLE 1` dan `ALLOWED DISRUPTIONS 1` (dengan 2 pod sehat)
- [ ] Saat `drain`, pod di `lab-worker` dipindah ke node lain satu per satu (drain menghormati PDB)
- [ ] Hasil loop curl hanya `200` (tidak ada kegagalan)
- [ ] Kamu paham beda `cordon` (node ditandai tidak bisa dijadwali), `drain` (cordon + mengusir pod), dan `uncordon` (membuka kembali)
- [ ] Setelah `uncordon`, node `Ready` tanpa `SchedulingDisabled`. Catatan: pod **tidak** otomatis kembali ke node itu
- [ ] Kamu bisa menjelaskan apa jadinya tanpa PDB: drain bisa mengusir kedua pod sekaligus sehingga layanan sempat mati
- [ ] (Opsional) Ubah `minAvailable` jadi 2 lalu `kubectl apply -f k8s/extras/pdb.yaml`, coba `drain` lagi dengan 2 replika: drain akan **tertahan** karena tidak boleh ada pod yang diganggu. Kembalikan ke 1 (`Ctrl+C` untuk membatalkan drain, lalu `uncordon`)

---

## C. Cilium (ganti CNI)

CNI adalah plugin jaringan pod. Cluster bawaan kind memakai `kindnet`; di sini kamu membuat cluster tanpa CNI lalu memasang Cilium sendiri.

### Langkah

```bash
kind delete cluster --name lab
kind create cluster --name lab --config k8s/kind/kind-3node-cilium.yaml

helm repo add cilium https://helm.cilium.io/
helm repo update cilium
helm upgrade --install cilium cilium/cilium --namespace kube-system \
  --set image.pullPolicy=IfNotPresent --set ipam.mode=kubernetes
kubectl wait --for=condition=Ready nodes --all --timeout=300s

# cluster baru = image dan manifest harus dipasang lagi
docker save tokoku-api:dev -o image.tar
kind load image-archive image.tar --name lab
rm image.tar
kubectl apply -k k8s/base
kubectl -n tokoku rollout status deployment/api --timeout=120s
```

Catatan: `--set 'args={--kubelet-insecure-tls}'` pada metrics-server diperlukan karena sertifikat kubelet di kind tidak ditandatangani CA yang dikenali.

Selama pembuatan dan pemasangan Cilium berjalan, kamu bisa membuka **terminal kedua** dan menjalankan `kubectl get nodes -w` (awalnya `NotReady`, itu normal karena belum ada CNI; `Ctrl+C` untuk berhenti).

Setelah semua perintah di atas selesai, periksa di satu terminal:

```bash
kubectl config current-context
kubectl get nodes
kubectl -n kube-system get pods -l k8s-app=cilium -o wide
kubectl -n tokoku get deploy,rs,pods,svc,hpa -o wide
```

Perlu autoscaling lagi di cluster baru? Ulangi bagian A (metrics-server tidak ikut terpasang).

### Checklist C

- [ ] Sebelum Cilium terpasang, node berstatus `NotReady` dan kamu paham alasannya (tidak ada CNI, pod tidak punya jaringan)
- [ ] Setelah `up-cilium`, ketiga node `Ready`
- [ ] Ada tiga pod `cilium-...` `Running` (satu per node, karena DaemonSet) di namespace `kube-system`
- [ ] `kubectl -n tokoku get deploy,rs,pods,svc,hpa -o wide` menampilkan 2 pod `1/1`, dan `curl localhost:8081/` berhasil (setelah menjalankan `kubectl -n tokoku port-forward svc/api 8081:80` di terminal lain dan membiarkannya berjalan)
- [ ] `kubectl apply -f k8s/extras/service-nodeport.yaml` lalu `curl localhost:8090/` berhasil di cluster Cilium
- [ ] Kamu bisa menjelaskan: CNI bertugas memberi IP dan konektivitas antar pod; Cilium adalah satu implementasi (berbasis eBPF)
- [ ] (Opsional) Pasang `cilium` CLI (`brew install cilium-cli`) dan jalankan `cilium status`

Kalau gagal: baca pesan Helm; `kubectl -n kube-system describe pod -l k8s-app=cilium`; cek `docker ps` apakah node masih jalan.

---

## D. Bersih-bersih

```bash
kind delete cluster --name lab
# (opsional) kembali ke cluster standar: ulangi 6A bagian A
```

- [ ] Kamu memutuskan mau memakai cluster yang mana untuk modul selanjutnya dan menuliskannya di Catatan

## Cek pemahaman

1. HPA menampilkan `cpu: <unknown>/70%`. Dua penyebab umum?
   <details><summary>Jawaban</summary>metrics-server belum terpasang atau belum siap, atau Deployment tidak punya `resources.requests.cpu`.</details>
2. Kamu menaikkan beban tetapi hanya satu pod yang CPU-nya tinggi, HPA lambat bereaksi. Apa dugaanmu?
   <details><summary>Jawaban</summary>Traffic lewat `port-forward` (satu pod), bukan lewat Service/NodePort yang membagi ke semua pod.</details>
3. Apa fungsi PDB, dan apakah ia melindungi dari node yang mati mendadak?
   <details><summary>Jawaban</summary>PDB membatasi gangguan yang disengaja (drain, upgrade node). Ia tidak melindungi dari kegagalan tiba-tiba seperti node mati (latihan 10 di 6B).</details>
4. Kenapa `kubectl apply -k k8s/base` saat HPA aktif bermasalah?
   <details><summary>Jawaban</summary>`apply` mengembalikan `replicas` ke nilai di manifest (2), menimpa hasil kerja HPA. Di praktik, `replicas` biasanya dihapus dari Deployment jika HPA dipakai.</details>

## Catatan

-

## Selesai?

- [ ] Checklist A, B, C tercentang
- [ ] Kamu bisa menggambarkan siklus HPA: metrik, hitung rata-rata terhadap target, ubah `replicas`, pod baru, jeda penurunan
- [ ] Kamu mencatat replika puncak dan lamanya turun di Catatan
