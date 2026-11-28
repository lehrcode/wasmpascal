program TotFehler;
{ Die Routine wird nie aufgerufen; der Typfehler in ihr muss trotzdem
  gemeldet werden (Typprüfung bleibt vollständig, PLAN.md Etappe 18). }
procedure WirdNieGerufen;
var X: Integer;
begin
  X := 'kein Integer'
end;
begin
  WriteLn('ok')
end.
