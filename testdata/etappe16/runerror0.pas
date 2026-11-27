{ RunError ohne Parameter ergibt Runtime error 0 }
program RunErr0;

procedure Melde;
begin
  WriteLn('ExitCode ', ExitCode, ', Fehler: ', ErrorAddr <> nil)
end;

begin
  ExitProc := @Melde;
  RunError
end.
