# lazyjail

Klavye odaklı terminal arayüzü (TUI) ile [fail2ban](https://github.com/fail2ban/fail2ban) yönetimi.

> Durum: **erken iskelet.** Bu sürümde yalnızca `--version` çalışır. TUI ve fail2ban
> entegrasyonu MVP işleriyle eklenecek. Plan: [docs/PLAN.md](docs/PLAN.md).

## Hedef

`fail2ban-client` komutlarını ezberlemeden, tek bir ekrandan: jail durumu, banlı IP'ler,
ban/unban, jail start/stop/reload, config görüntüleme, canlı log ve filtre regex testi.
Tek statik binary; sunucuda port açmaya gerek yok.

## Geliştirme

Gereksinimler: Go (sürüm `go.mod` içinde), `make`, [golangci-lint](https://golangci-lint.run/).

```sh
make build      # bin/lazyjail üretir
make test       # go test ./...
make test-race  # go test -race ./... (race dedektörü destekleyen platformda)
make lint       # go vet + golangci-lint + gofmt kontrolü
./bin/lazyjail --version
```

## Dokümantasyon

- [docs/README.md](docs/README.md) — doküman indeksi
- [docs/PLAN.md](docs/PLAN.md) — mimari plan (dil, fail2ban entegrasyonu, MVP kapsamı)
- [docs/DEĞİŞİKLİKLER.md](docs/DEĞİŞİKLİKLER.md) — değişiklik günlüğü

## Katkı ve güvenlik

- Katkı: [CONTRIBUTING.md](CONTRIBUTING.md)
- Güvenlik açığı bildirimi: [SECURITY.md](SECURITY.md)

## Lisans

[MIT](LICENSE)
