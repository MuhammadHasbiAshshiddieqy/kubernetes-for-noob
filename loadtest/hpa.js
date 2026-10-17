// Uji beban untuk latihan HPA (Modul 7).
// Jalankan: make loadtest   (butuh k6 dan NodePort: make nodeport)
// Target memakai NodePort agar traffic terbagi ke semua pod.
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '1m', target: 20 },   // naik pelan
    { duration: '3m', target: 80 },   // beban tinggi: HPA seharusnya menambah pod
    { duration: '1m', target: 0 },    // turun
  ],
};

const BASE = __ENV.BASE_URL || 'http://localhost:8090';

export default function () {
  const res = http.get(`${BASE}/debug/cpu?ms=100`);
  check(res, { 'status 200': (r) => r.status === 200 });
  sleep(0.1);
}
