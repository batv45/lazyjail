# Katkı rehberi

## Başlamadan önce

- Büyük değişikliklerde önce bir issue açın, ne yapılacağını birlikte netleştirelim.
- Mimari kararlar [docs/PLAN.md](docs/PLAN.md) içinde. Özellikle fail2ban'e erişim
  `internal/f2b.Backend` arayüzü üzerinden yapılır; UI doğrudan socket'e ya da
  `fail2ban-client` çıktısına bağlanmaz.

## Geliştirme akışı

1. Dal açın (`feature/...`, `fix/...`).
2. Değişikliği yapın ve testleri ekleyin.
3. Çalıştırın:

   ```sh
   make fmt
   make lint
   make test
   ```

4. Pull request açın. Açıklamada ne değiştiğini ve nasıl doğrulandığını yazın.

## Kodlama kuralları

- `gofmt -s` ve `go vet` temiz olmalı; `golangci-lint` uyarısı bırakılmaz.
- Shell çağrısı yok: harici komutlar `exec.Command` ile argüman dizisi olarak çalışır.
- Kullanıcıdan gelen IP, CIDR ve jail adları çağrılmadan önce doğrulanır.
- Yeni özellik ya da mimari değişiklik `docs/` altındaki ilgili dosyayı günceller.

## Dokümantasyon

Kod değiştiren her katkı `docs/DEGISIKLIKLER.md` dosyasına tarihli bir madde ekler.
