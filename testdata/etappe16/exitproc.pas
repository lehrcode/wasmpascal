{ Verkettete Exit-Prozeduren am normalen Programmende }
program ExitKette;

var
  AlteExit: Pointer;

procedure Zweite;
begin
  WriteLn('Zweite: ExitCode ', ExitCode, ', ErrorAddr nil: ', ErrorAddr = nil)
end;

procedure Erste;
begin
  WriteLn('Erste');
  ExitProc := AlteExit   { Kette fortsetzen }
end;

begin
  WriteLn('ExitProc nil: ', ExitProc = nil);
  ExitProc := @Zweite;
  AlteExit := ExitProc;
  ExitProc := @Erste;
  WriteLn('Hauptprogramm')
end.
