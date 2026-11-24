# Abweichungen von Turbo Pascal 4.0

Bewusste Abweichungen und getroffene Annahmen. Die grundsätzlichen Abweichungen
(Typabbildung, Speichermodell, UTF-8, Erweiterungen) stehen in `PLAN.md`; hier
kommen die Einzelentscheidungen dazu, die während der Umsetzung fallen.

| Etappe | Thema | Entscheidung |
| --- | --- | --- |
| 0 | Laufzeitfehlermeldung | Fehlernummer ohne führende Nullen: `Runtime error 2 at …` (`// TP-ANNAHME`). |
