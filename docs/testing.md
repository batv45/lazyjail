# Test

## Hızlı kontrol

```sh
make build test lint
go test -race ./...        # CI'da; Raspberry Pi'de race çalışmaz, orada `go test ./...`
go test -coverpkg=./internal/f2b/... -cover ./...   # f2b kapsaması (hedef >= %80; son ölçüm %93,5)
```

## Birim testleri (backend)

| Paket | Ne test edilir |
|---|---|
| `internal/f2b/proto` | Gerçek fail2ban yanıtlarının kayıtları (`testdata/2404_*.pickle` = 1.0.2, `2204_*.pickle` = 0.11.2) çözülür; komut gidiş-dönüşü; boyut sınırı; bozuk giriş |
| `internal/f2b/socket` | Tüm socket metotları `fake` sunucuya karşı; hata, zaman aşımı, bozuk yanıt, izin yok, çalışmıyor |
| `internal/f2b/execbk` | Sahte `fail2ban-client`/`fail2ban-regex` betikleri (argv kaydı + `testdata/regex_*.txt` gerçek çıktıları) |
| `internal/f2b/real` | Birleşik backend, `StartJail` → `reload` yedeği, "kurulu değil" ayrımı |
| `internal/validate` | IP/CIDR/jail adı kabul ve ret tabloları |

Kayıtları yenilemek: `docker build` ile imajı yap (aşağıda), konteyneri argümansız çalıştır,
`docker cp scripts/record-pickles.py <konteyner>:/rec.py && docker exec <konteyner> python3 /rec.py /out <etiket>`.

## Entegrasyon testi (gerçek fail2ban, Docker)

```sh
make integration                      # ubuntu:24.04 (fail2ban 1.0.2)
make integration BASE=ubuntu:22.04    # fail2ban 0.11.2
make integration BASE=debian:12       # fail2ban 1.0.2
```

`make integration`: test ikilisini derler (`-tags integration`), `test/integration/docker/Dockerfile` ile
fail2ban'li imajı kurar ve ikiliyi konteynerde **root** olarak çalıştırır. Konteynerde `banaction = dummy` ve
`backend = polling` olan bir `sshd` jail'i vardır (`/tmp/auth.log`); iptables gerekmez. Testler: ping/sürüm/jail
listesi, config okuma, ban/unban (IPv4, IPv6, tüm jail'lerden), log satırlarından gerçek ban, ignoreip ekle/sil,
stop → start → reload, regex testi (dosya + tek satır + bozuk regex), socket yok.

CI: `.github/workflows/integration.yml` aynı üç imajı matris olarak koşar. Konteyner dışında root olmayan
kullanıcıyla çalıştırılırsa testler atlanır.

## Yerel sonuçlar (TEK-449)

Üç imajın üçünde de `make integration` eşdeğeri 8/8 test geçti (ubuntu:24.04, ubuntu:22.04, debian:12).
