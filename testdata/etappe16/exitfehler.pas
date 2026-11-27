{ Exit-Prozeduren laufen auch nach einem Laufzeitfehler }
program ExitFehler;

var
  A, B: Integer;
  Alt: Pointer;

procedure Melde;
begin
  WriteLn('Melde: ExitCode ', ExitCode, ', Fehler: ', ErrorAddr <> nil);
  ExitProc := Alt
end;

procedure Zweite;
begin
  WriteLn('Zweite');
  Write('ohne Zeilenende')
end;

procedure Teile;
begin
  B := 0;
  A := 10 div B
end;

begin
  ExitProc := @Zweite;
  Alt := ExitProc;
  ExitProc := @Melde;
  WriteLn('vorher');
  Teile;
  WriteLn('nie')
end.
