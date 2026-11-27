{ Exit-Prozeduren nach einem Laufzeitfehler dürfen anhalten (Delay, ReadLn) }
program ExitDelay;

uses Crt;

var
  P: ^Integer;

procedure Frage;
var
  S: string;
begin
  Delay(5);
  Write('Weiter? ');
  ReadLn(S);
  WriteLn('Antwort: ', S, ', ExitCode ', ExitCode)
end;

begin
  ExitProc := @Frage;
  P := nil;
  WriteLn(P^)
end.
