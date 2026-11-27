{ ErrorAddr := nil unterdrückt die Meldung; der Exit-Code bleibt }
program ExitStumm;

type
  PInt = ^Integer;

var
  P: PInt;

procedure Abfangen;
begin
  if ErrorAddr <> nil then
  begin
    WriteLn('Fehler ', ExitCode, ' abgefangen');
    ErrorAddr := nil;
    ExitCode := 1
  end
end;

begin
  ExitProc := @Abfangen;
  P := nil;
  P^ := 5;
  WriteLn('nie')
end.
