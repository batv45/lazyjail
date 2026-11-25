# lazyjail

Klavye odaklı terminal arayüzü (TUI) ile [fail2ban](https://github.com/fail2ban/fail2ban) yönetimi.

> Durum: **erken iskelet.** Bu sürümde yalnızca `--version` çalışır. TUI ve fail2ban
> entegrasyonu MVP işleriyle eklenecek. Plan: [docs/PLAN.md](docs/PLAN.md).

## Hedef

`fail2ban-client` komutlarını ezberlemeden, tek bir ekrandan: jail durumu, banlı IP'ler,
ban/unban, jail start/stop/reload, config görüntüleme, canlı log ve filtre regex testi.
Tek statik binary; sunucuda port açmaya gerek yok.

## Kurulum (Ubuntu / Debian, amd64 · arm64 · armv7)

Son sürümü indirir, mimariyi seçer, SHA-256 doğrular ve `/usr/local/bin/lazyjail`'e kurar:

```sh
curl -fsSL https://github.com/batv45/lazyjail/releases/latest/download/install.sh | sudo sh
```

Scripti çalıştırmadan önce okumak isterseniz:

```sh
curl -fsSLO https://github.com/batv45/lazyjail/releases/latest/download/install.sh
less install.sh && sudo sh install.sh
```

Script kullanmadan, aynı işi elle:

```sh
case "$(uname -m)" in x86_64) A=amd64;; aarch64) A=arm64;; armv7l) A=armv7;; *) echo "desteklenmeyen mimari"; exit 1;; esac
cd "$(mktemp -d)" && U=https://github.com/batv45/lazyjail/releases/latest/download \
  && curl -fsSLO "$U/lazyjail_linux_$A.tar.gz" -O "$U/checksums.txt" \
  && sha256sum -c --ignore-missing checksums.txt \
  && tar xzf "lazyjail_linux_$A.tar.gz" lazyjail \
  && sudo install -m 0755 lazyjail /usr/local/bin/lazyjail && lazyjail --version
```

Belirli bir sürüm: `curl -fsSL https://github.com/batv45/lazyjail/releases/latest/download/install.sh | sudo LAZYJAIL_VERSION=v0.1.0 sh`

**Güncelleme:** kurulum komutunu yeniden çalıştırın.
**Kaldırma:** `sudo rm /usr/local/bin/lazyjail`

### .deb paketi

```sh
A=$(dpkg --print-architecture); [ "$A" = armhf ] && A=armv7
curl -fsSLO "https://github.com/batv45/lazyjail/releases/latest/download/lazyjail_$A.deb"
sudo apt install "./lazyjail_$A.deb"     # güncelleme: aynı komutlar
sudo apt remove lazyjail                 # kaldırma
```

`.deb` binary'yi `/usr/bin/lazyjail`'e koyar. İki yöntemden yalnız birini kullanın; ikisi birden kuruluysa
`/usr/local/bin` önce gelir.

### Çalıştırma

```sh
sudo lazyjail          # fail2ban socket'i root ister
lazyjail --demo        # root'suz, sahte verilerle deneme
```

Kısayollar: [docs/keybindings.md](docs/keybindings.md) (tam tablo log/regex/ignoreip ekranlarıyla eklenecek).

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
- [docs/releasing.md](docs/releasing.md) — sürüm çıkarma ve release provası
- [docs/DEGISIKLIKLER.md](docs/DEGISIKLIKLER.md) — değişiklik günlüğü

## Katkı ve güvenlik

- Katkı: [CONTRIBUTING.md](CONTRIBUTING.md)
- Güvenlik açığı bildirimi: [SECURITY.md](SECURITY.md)

## Lisans

[MIT](LICENSE)
