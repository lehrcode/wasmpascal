{ Halt läuft über die Exit-Prozeduren; sie dürfen ExitCode ändern }
program ExitHalt;

var
  Alt: Pointer;

procedure Aufraeumen;
begin
  WriteLn('Aufraeumen: ExitCode ', ExitCode);
  ExitCode := ExitCode + 1;
  ExitProc := Alt
end;

procedure Letzte;
begin
  WriteLn('Letzte: ExitCode ', ExitCode)
end;

procedure Tief(N: Integer);
begin
  if N = 0 then
    Halt(3);
  Tief(N - 1)
end;

begin
  ExitProc := @Letzte;
  Alt := ExitProc;
  ExitProc := @Aufraeumen;
  Tief(5);
  WriteLn('nie')
end.
