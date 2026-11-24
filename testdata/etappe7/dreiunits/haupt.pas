program Haupt;
{ Abnahme Etappe 7: drei eigene Units mit gegenseitigen uses im
  Implementation-Teil (Protokoll -> Werkzeug -> Zaehler -> Protokoll). }
uses Protokoll, Zaehler;
begin
  WriteLn('Programm');
  Zaehle(5);
  Zaehle(2);
  WriteLn(Zaehler.Stand, ' ', Stand, ' ', Protokoll.Anzahl, ' ', Ursprung.x)
end.
