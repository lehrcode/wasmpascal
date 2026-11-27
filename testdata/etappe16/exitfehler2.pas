{ Ein Laufzeitfehler in einer Exit-Prozedur: die Kette läuft weiter, gemeldet
  wird der letzte Fehler }
program ExitFehler2;

var
  Alt: Pointer;
  N: Integer;

procedure Letzte;
begin
  WriteLn('Letzte: ExitCode ', ExitCode)
end;

procedure Fehlerhaft;
begin
  WriteLn('Fehlerhaft: ExitCode ', ExitCode);
  ExitProc := Alt;
  N := 0;
  N := 1 div N
end;

begin
  ExitProc := @Letzte;
  Alt := ExitProc;
  ExitProc := @Fehlerhaft;
  RunError(42)
end.
