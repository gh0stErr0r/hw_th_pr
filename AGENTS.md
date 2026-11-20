# genc

Требования к проекту описаны в spec.md, уточнения к первоначальным схемам — в docs/DECISIONS.md. Реализация и листинги находятся в 04-coding/. Разделы 5 и 6 подключаются из своих папок в docs/report/main.typ. Тесты Go хранятся в 05-structural-testing/testdata и подключаются через overlay.json. Проверки перед коммитом: gofmt -l ., make vet test.
