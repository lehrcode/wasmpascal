{ Auch ein WASM-Trap ohne Prüfung (207 bei Trunc) läuft über die Exit-Prozeduren }
program ExitTrap;

var
  R: Real;
  I: LongInt;

procedure Melde;
begin
  WriteLn('Melde: ExitCode ', ExitCode)
end;

begin
  ExitProc := @Melde;
  R := 1E30;
  I := Trunc(R);
  WriteLn(I)
end.
