# ADR-005: Модель raw данных для Binance USDⓈ-M Perpetual

## Статус
Принято (Этап 2)

## Контекст
Для perpetual futures нужны три слоя raw данных: trade klines, mark price klines, funding rates. Спот-схема из устава недостаточна.

## Решение

### Источники (нативный Binance Futures API, без CCXT)
- `GET /fapi/v1/exchangeInfo` — contract metadata
- `GET /fapi/v1/klines` — trade klines (limit 1500)
- `GET /fapi/v1/markPriceKlines` — mark price klines (limit 1500)
- `GET /fapi/v1/fundingRate` — funding history (limit 1000)

### Пути в S3/MinIO
```
raw/trade_klines/exchange=binance_usdm/symbol=BTCUSDT/interval=1m/year=YYYY/month=MM/
raw/mark_price_klines/exchange=binance_usdm/symbol=BTCUSDT/interval=1m/year=YYYY/month=MM/
raw/funding_rates/exchange=binance_usdm/symbol=BTCUSDT/year=YYYY/month=MM/
```

### Правила
- Все цены и объёмы — fixed-point int64
- Период backfill: from = max(now - 3y, onboardDate), to = now
- Валидация: уникальность open_time, монотонность, проверка дыр

## Ссылки
- [TODO.md](../../TODO.md) — этап 2
- Binance Futures API: https://binance-docs.github.io/apidocs/futures/en/
