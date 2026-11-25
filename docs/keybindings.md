# Klavye kısayolları

Tek kaynak: `internal/ui/keys.go`. Alt çubuk ve `?` ekranı buradan üretilir. Yazma tuşları `--read-only` modunda kapalıdır.

| Tuş | Eylem |
|---|---|
| `↑` `k` / `↓` `j` | Gezin |
| `tab`, `←` `→`, `h` `l` | Jail / IP paneli arası geçiş |
| `/` | Odaklı listede ara (`enter` tut, `esc` temizle) |
| `space` | IP'yi çoklu seçime ekle/çıkar |
| `b` | Seçili jail'de IP banla (SSH IP'niz ise iki onay) |
| `u` | Seçili/işaretli IP'leri bu jail'den unban (onaylı) |
| `U` | Seçili/işaretli IP'leri tüm jail'lerden unban (iki onay) |
| `s` | Jail durdur (onaylı) / durmuşsa başlat |
| `r` / `R` | Jail reload / tüm jail'ler reload (onaylı) |
| `F5`, `ctrl+r` | Hemen yenile |
| `?` | Yardım |
| `q`, `ctrl+c` | Çık |

Log (`l`), regex testi (`t`) ve ignoreip (`i`) ekranları TEK-452 ile gelir.
