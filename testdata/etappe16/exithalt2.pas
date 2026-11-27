{ Halt in einer Exit-Prozedur nach einem Laufzeitfehler: keine Meldung,
  der Exit-Code kommt von Halt }
program ExitHalt2;

uses Crt;

var
  Alt: Pointer;
  N: Integer;

procedure Spaeter;
begin
  WriteLn('nie: ExitProc wurde nicht weitergegeben')
end;

procedure Beenden;
begin
  WriteLn('Fehler ', ExitCode, ', warte');
  Delay(5);
  WriteLn('Halt(7)');
  Halt(7)
end;

begin
  ExitProc := @Spaeter;
  Alt := ExitProc;
  ExitProc := @Beenden;
  N := 0;
  WriteLn(10 mod N)
end.
