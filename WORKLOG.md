## Что готово
- generator -> collector -> Redis -> worker -> PostgreSQL работает
- миграции применяются
- результаты прогонов сохраняются в results/*.json
- Prometheus и Grafana подняты
- collector и worker отдают /metrics
- в Grafana создан первый dashboard

## Что проверить при следующем запуске
- docker compose -f deploy/docker-compose.yml up -d
- запустить collector
- запустить worker
- открыть Prometheus и Grafana
- прогнать generator

## Следующий шаг
- доделать dashboard
- перейти к подключению ClickHouse