# Güvenlik politikası

lazyjail, fail2ban'i yönettiği için çoğu kurulumda root yetkisiyle çalışır. Güvenlik
açığı bulduğunuzu düşünüyorsanız lütfen herkese açık bir issue açmayın.

## Bildirim

GitHub'da [Security Advisories](https://github.com/batv45/lazyjail/security/advisories/new)
üzerinden özel bildirim yapın. Bildirimde şunları ekleyin:

- Etkilenen sürüm ya da commit
- Yeniden üretme adımları
- Olası etki (ör. yetki yükseltme, komut enjeksiyonu, dosya yazma)

Bildirimi aldıktan sonra mümkün olan en kısa sürede dönüş yapılmaya çalışılır.

## Kapsam

Özellikle ilgilendiğimiz konular: exec çağrılarında komut enjeksiyonu, socket
yanıtlarının işlenmesi, root olarak çalışan TUI'nin dosya/terminal davranışları.
