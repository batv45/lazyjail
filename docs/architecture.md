# Mimari: fail2ban backend

Bu belge `internal/f2b` altındaki backend'i anlatır. UI'nın (`internal/ui`) tek bağımlılığı
`f2b.Backend` arayüzüdür ([PLAN.md §3.2](PLAN.md)); gerçek, sahte (`fake`) ve bellek içi (`mock`)
uygulamalar aynı arayüzü sağlar.

## Paketler

| Paket | Görev |
|---|---|
| `internal/f2b` | `Backend` arayüzü ve tipleri, `errors.go` (ortak hatalar), `caps.go` (sürüm yetenekleri) |
| `internal/f2b/proto` | Socket tel biçimi: komut kodlama, çerçeveleme, yanıt çözme (`kisielk/og-rek`) |
| `internal/f2b/socket` | `Backend`'in socket yarısı (ban, unban, durum, config okuma, ignoreip, stop) |
| `internal/f2b/execbk` | `fail2ban-client` (`start`, `reload`) ve `fail2ban-regex` çağrıları |
| `internal/f2b/real` | Üretim `Backend`'i: socket + exec birleşimi |
| `internal/f2b/fake` | Gerçek protokolü konuşan bellek içi sahte sunucu (testler için) |
| `internal/f2b/mock` | UI/demo için bellek içi `Backend` (socket yok) |
| `internal/validate` | IP/CIDR ve jail adı doğrulama |
| `test/integration` | Gerçek fail2ban'e karşı testler (`-tags integration`) |

## Hangi işlem hangi yoldan gider

| İşlem | Yol |
|---|---|
| Ping, Version, Jails, JailStatus, BannedWithTime, JailConfig, Ban, Unban, StopJail, Add/DelIgnoreIP, LogTarget | Socket |
| StartJail, Reload | `fail2ban-client` (config dosyalarını istemci okur) |
| TestRegex | `fail2ban-regex` |

### Socket protokolü (doğrulandı)

- İstek: komut listesi (`["status","sshd"]`) pickle (protokol 2) + `<F2B_END_COMMAND>`. Her komut için yeni
  bağlantı açılır; kapatırken ayrıca CLOSE gönderilmez (sunucu EOF'u kabul eder).
- Yanıt: pickle protokol 5 `(kod, sonuç)` + aynı sonlandırıcı. `kod != 0` ise sonuç bir istisna nesnesidir
  (`builtins.Exception(...)`, `fail2ban.exceptions.UnknownJailException(...)`); `CommandError` metnine çevrilir.
- `og-rek` protokol 5'i, `STACK_GLOBAL`/`REDUCE` ile gelen istisnaları ve `str` alt sınıflarını
  (`ignoreip` girdileri `builtins.str("...")` olarak gelir) çözer. Spike ubuntu:24.04 (fail2ban 1.0.2) ve
  ubuntu:22.04 (0.11.2) ile yapıldı; ham yanıtlar `internal/f2b/proto/testdata/` altında
  (`scripts/record-pickles.py` ile kaydedilir). Yedek yola (exec + parse) gerek kalmadı.
- Yanıt üst sınırı 16 MB, varsayılan zaman aşımı 5 sn (çağıranın bağlamında süre varsa o geçerli).
  `exec` çağrılarında varsayılan 30 sn (reload tüm config'i okur).
- Gelen pickle yalnızca veri olarak çözülür; Python'daki gibi kod çalıştırma riski yoktur.

### Hatalar

`errors.Is` ile ayırt edilir: `ErrNotInstalled` (socket yok ve `fail2ban-client` PATH'te yok), `ErrNotRunning`
(socket yok / bağlantı reddedildi), `ErrPermission` (socket'e erişim yok), `ErrUnsupported` (sunucu
"Invalid command" dedi). fail2ban'in kendi hata yanıtı `*f2b.CommandError` olarak gelir (`errors.As`).

### Sürüm yetenekleri

`caps.go`: sürüm 0.10'dan eskiyse `--with-time` yoktur; `BannedWithTime` bu durumda (ya da sunucu bayrağı
reddederse) `status <jail>` içindeki IP listesini sıfır zamanlarla döndürür. Sürüm ayrıştırılamazsa tüm
özellikler açık varsayılır. Gerçek sunucuda doğrulananlar: 0.11.2 ve 1.0.2. 0.10 ve 1.1 doğrulanmadı.

### Gerçek sunucudan öğrenilen davranışlar

- **`stop <jail>` jail'i sunucudan siler.** Sonrasında `start <jail>` "jail does not exist" ile başarısız olur;
  `reload <jail>` da. Geri getirmenin tek yolu parametresiz `reload`'dur. `real.Backend.StartJail` bunu uygular:
  önce `start`, "does not exist" ise tam `reload`, sonra jail listede mi diye bakar (config'te kapalıysa açık
  hata verir). Tam reload diğer jail'lerin banlarını silmez.
- `fail2ban-client` seçenek ayrıştırıcısı için `--` komuttan **önce** konur (`-s SOCK -- reload sshd`);
  komuttan sonra konursa komutun parçası olur.
- `get <jail> banip --with-time` satırları `IP \t2026-11-25 20:14:16 + 600 = 2026-11-25 20:24:16` biçimindedir;
  zamanlar sunucunun yerel saatindedir (istemci `time.Local` ile okur). `bantime = -1` (kalıcı) için
  `ExpiresAt` sıfırdır.
- `fail2ban-client` ve `fail2ban-regex` hata metni stdout/stderr karışık gelir; `ERROR` satırları (ve devam
  satırları) `CommandError` metni olur.

## Güvenlik

- Shell yok: `exec.CommandContext(name, args...)`; `LC_ALL=C`.
- IP `net/netip` ile, jail adı `^[A-Za-z0-9._-]+$` (başta `-` yok) ile doğrulanır; geçersiz değer sunucuya ya da
  sürece hiç ulaşmaz (testle doğrulanır).
- `fail2ban-regex` argümanları `--`'dan sonra tek tek argv öğesidir; seçenek enjeksiyonu yok.
